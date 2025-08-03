package web_proxy

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"go.uber.org/zap"

	"github.com/veops/oneterm/internal/acl"
	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/service"
	gsession "github.com/veops/oneterm/internal/session"
	"github.com/veops/oneterm/pkg/logger"
)

// StartWebSessionRequest represents the request to start a web session
type StartWebSessionRequest struct {
	AssetId   int    `json:"asset_id" binding:"required"`
	AssetName string `json:"asset_name"`
	AuthMode  string `json:"auth_mode"`
	AccountId int    `json:"account_id"`
}

// StartWebSessionResponse represents the response from starting a web session
type StartWebSessionResponse struct {
	SessionId string `json:"session_id"`
	ProxyURL  string `json:"proxy_url"`
	Message   string `json:"message"`
}

// StartWebSession creates a new web proxy session
func StartWebSession(ctx *gin.Context, req StartWebSessionRequest) (*StartWebSessionResponse, error) {
	assetService := service.NewAssetService()
	asset, err := assetService.GetById(ctx, req.AssetId)
	if err != nil {
		return nil, fmt.Errorf("Asset not found")
	}

	// Check if asset is web asset
	if !asset.IsWebAsset() {
		return nil, fmt.Errorf("Asset is not a web asset")
	}

	// Auto-detect auth_mode from asset.WebConfig if not provided
	authMode := req.AuthMode
	if authMode == "" && asset.WebConfig != nil {
		authMode = asset.WebConfig.AuthMode
		if authMode == "" {
			authMode = "none" // default
		}
	}

	// Generate unique session ID
	sessionId := fmt.Sprintf("web_%d_%d_%d", req.AssetId, req.AccountId, time.Now().Unix())

	// Create session for permission checking (following standard pattern)
	tempSession := &gsession.Session{
		Session: &model.Session{
			SessionId: sessionId,
			AssetId:   req.AssetId,
			AccountId: req.AccountId,
			Protocol:  "http", // Web assets use http/https protocol
		},
	}

	requiredActions := []model.AuthAction{
		model.ActionConnect,
		model.ActionFileDownload,
		model.ActionCopy,
		model.ActionPaste,
		model.ActionShare,
	}

	result, err := service.DefaultAuthService.HasAuthorizationV2(ctx, tempSession, requiredActions...)
	if err != nil {
		return nil, fmt.Errorf("Authorization check failed")
	}

	// Check connect permission (required for all protocols)
	if !result.IsAllowed(model.ActionConnect) {
		return nil, fmt.Errorf("No permission to connect to this asset")
	}

	// Build permissions object from authorization result (same as other protocols)
	permissions := &model.AuthPermissions{
		Connect:      result.IsAllowed(model.ActionConnect),
		FileDownload: result.IsAllowed(model.ActionFileDownload),
		Copy:         result.IsAllowed(model.ActionCopy),
		Paste:        result.IsAllowed(model.ActionPaste),
		Share:        result.IsAllowed(model.ActionShare),
	}

	// Check max concurrent connections (only when creating new session)
	if asset.WebConfig != nil && asset.WebConfig.ProxySettings != nil && asset.WebConfig.ProxySettings.MaxConcurrent > 0 {
		activeCount := GetActiveSessionsForAsset(req.AssetId)
		if activeCount >= asset.WebConfig.ProxySettings.MaxConcurrent {
			logger.L().Warn("Maximum concurrent connections exceeded",
				zap.Int("assetID", req.AssetId),
				zap.Int("activeCount", activeCount),
				zap.Int("maxConcurrent", asset.WebConfig.ProxySettings.MaxConcurrent))
			return nil, fmt.Errorf("maximum concurrent connections (%d) exceeded", asset.WebConfig.ProxySettings.MaxConcurrent)
		}
	}

	now := time.Now()

	// Get initial target host from asset
	initialHost := GetAssetHost(asset)

	webSession := &WebProxySession{
		SessionId:     sessionId,
		AssetId:       asset.Id,
		AccountId:     req.AccountId,
		Asset:         asset,
		CreatedAt:     now,
		LastActivity:  now,
		LastHeartbeat: now,  // Initialize heartbeat timestamp
		IsActive:      true, // Initially active
		CurrentHost:   initialHost,
		Permissions:   permissions,
		WebConfig:     asset.WebConfig,
	}
	StoreSession(sessionId, webSession)

	// Generate subdomain-based proxy URL
	baseDomain := strings.Split(ctx.Request.Host, ":")[0]
	if strings.Contains(baseDomain, ".") {
		parts := strings.Split(baseDomain, ".")
		if len(parts) > 2 {
			baseDomain = strings.Join(parts[1:], ".")
		}
	}

	// Determine proxy scheme based on current request only (not asset protocol)
	scheme := lo.Ternary(ctx.Request.TLS != nil, "https", "http")

	portSuffix := ""
	if strings.Contains(ctx.Request.Host, ":") {
		portSuffix = ":" + strings.Split(ctx.Request.Host, ":")[1]
	}

	// Create subdomain URL with session_id for first access (cookie will handle subsequent requests)
	subdomainHost := fmt.Sprintf("asset-%d.%s%s", req.AssetId, baseDomain, portSuffix)
	proxyURL := fmt.Sprintf("%s://%s/?session_id=%s", scheme, subdomainHost, sessionId)

	// Create database session record for history (same as other protocols)
	currentUser, _ := acl.GetSessionFromCtx(ctx)

	// Get actual protocol from asset
	protocol, port := asset.GetWebProtocol()
	if protocol == "" {
		protocol = "http"
		port = 80
	}

	// Format protocol as "protocol:port"
	protocolStr := fmt.Sprintf("%s:%d", protocol, port)

	dbSession := &model.Session{
		SessionType: model.SESSIONTYPE_WEB,
		SessionId:   sessionId,
		Uid:         currentUser.GetUid(),
		UserName:    currentUser.GetUserName(),
		AssetId:     asset.Id,
		AssetInfo:   fmt.Sprintf("%s(%s)", asset.Name, asset.Ip),
		AccountId:   req.AccountId,
		AccountInfo: "", // Web assets don't have named accounts
		GatewayId:   asset.GatewayId,
		GatewayInfo: "",
		ClientIp:    ctx.ClientIP(),
		Protocol:    protocolStr, // Now shows "http:80" or "https:443" etc.
		Status:      model.SESSIONSTATUS_ONLINE,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// Set gateway info if exists
	if asset.GatewayId > 0 {
		dbSession.GatewayInfo = fmt.Sprintf("Gateway_%d", asset.GatewayId)
	}

	// Save session to database using gsession
	fullSession := &gsession.Session{Session: dbSession}
	if err := gsession.UpsertSession(fullSession); err != nil {
		logger.L().Error("Failed to save web session to database",
			zap.String("sessionId", sessionId), zap.Error(err))
		// Don't fail the request, just log the error
	}

	logger.L().Info("Web session started", zap.String("sessionId", sessionId), zap.String("proxyURL", proxyURL), zap.String("authMode", authMode))

	return &StartWebSessionResponse{
		SessionId: sessionId,
		ProxyURL:  proxyURL,
		Message:   "Web session started successfully",
	}, nil
}

// GetAssetHost extracts the host from asset configuration
func GetAssetHost(asset *model.Asset) string {
	targetURL := BuildTargetURL(asset)
	if u, err := url.Parse(targetURL); err == nil {
		return u.Host
	}
	return "localhost" // fallback
}

// BuildTargetURL builds the target URL from asset information
func BuildTargetURL(asset *model.Asset) string {
	protocol, port := asset.GetWebProtocol()
	if protocol == "" {
		protocol = "http"
		port = 80
	}

	// Check if asset.Ip already includes port (case 1: 127.0.0.1:8000)
	if strings.Contains(asset.Ip, ":") {
		// IP already has port, use as-is
		return fmt.Sprintf("%s://%s", protocol, asset.Ip)
	}

	// Case 2: IP without port (127.0.0.1), use port from protocol
	// If port is default port for protocol, don't include it
	if (protocol == "http" && port == 80) || (protocol == "https" && port == 443) {
		return fmt.Sprintf("%s://%s", protocol, asset.Ip)
	}

	return fmt.Sprintf("%s://%s:%d", protocol, asset.Ip, port)
}

// BuildTargetURLWithHost builds target URL with specific host
func BuildTargetURLWithHost(asset *model.Asset, host string) string {
	protocol, port := asset.GetWebProtocol()
	if protocol == "" {
		protocol = "http"
		port = 80
	}

	// Check if host already includes port
	if strings.Contains(host, ":") {
		// Host already has port, use as-is
		return fmt.Sprintf("%s://%s", protocol, host)
	}

	// Use custom host instead of asset's original host
	if (port == 80 && protocol == "http") || (port == 443 && protocol == "https") {
		return fmt.Sprintf("%s://%s", protocol, host)
	}
	return fmt.Sprintf("%s://%s:%d", protocol, host, port)
}

// ExtractAssetIDFromHost extracts asset ID from subdomain host
func ExtractAssetIDFromHost(host string) (int, error) {
	// Remove port if present
	hostParts := strings.Split(host, ":")
	hostname := hostParts[0]

	// Check for asset- prefix
	if !strings.HasPrefix(hostname, "asset-") {
		return 0, fmt.Errorf("host does not start with asset- prefix: %s", hostname)
	}

	// Extract asset ID: asset-123.domain.com -> 123
	parts := strings.Split(hostname, ".")
	if len(parts) == 0 {
		return 0, fmt.Errorf("invalid hostname format: %s", hostname)
	}

	assetPart := parts[0] // asset-123
	assetIDStr := strings.TrimPrefix(assetPart, "asset-")
	if assetIDStr == assetPart {
		return 0, fmt.Errorf("failed to extract asset ID from: %s", assetPart)
	}

	assetID, err := strconv.Atoi(assetIDStr)
	if err != nil {
		return 0, fmt.Errorf("invalid asset ID format: %s", assetIDStr)
	}

	return assetID, nil
}

// IsSameDomainOrSubdomain checks if two hosts belong to the same domain or subdomain
func IsSameDomainOrSubdomain(host1, host2 string) bool {
	if host1 == host2 {
		return true
	}

	// Remove port if present
	host1 = strings.Split(host1, ":")[0]
	host2 = strings.Split(host2, ":")[0]

	// Get domain parts
	parts1 := strings.Split(host1, ".")
	parts2 := strings.Split(host2, ".")

	// Need at least domain.tld (2 parts)
	if len(parts1) < 2 || len(parts2) < 2 {
		return false
	}

	// Compare the last two parts (domain.tld)
	domain1 := strings.Join(parts1[len(parts1)-2:], ".")
	domain2 := strings.Join(parts2[len(parts2)-2:], ".")

	return domain1 == domain2
}

// CheckWebAccessControls validates web-specific access controls
func CheckWebAccessControls(ctx *gin.Context, session *WebProxySession) error {
	// Check access policy (read-only mode)
	if session.WebConfig != nil && session.WebConfig.AccessPolicy == "read_only" {
		method := strings.ToUpper(ctx.Request.Method)
		if method != "GET" && method != "HEAD" && method != "OPTIONS" {
			return fmt.Errorf("read-only access mode - %s method not allowed", method)
		}
	}

	// Check blocked paths
	if session.WebConfig != nil && session.WebConfig.ProxySettings != nil && len(session.WebConfig.ProxySettings.BlockedPaths) > 0 {
		requestPath := ctx.Request.URL.Path
		for _, blockedPath := range session.WebConfig.ProxySettings.BlockedPaths {
			if strings.Contains(requestPath, blockedPath) {
				return fmt.Errorf("access to path '%s' is blocked", requestPath)
			}
		}
	}

	// Check file download permissions
	if session.Permissions != nil && !session.Permissions.FileDownload {
		if IsDownloadRequest(ctx) {
			return fmt.Errorf("file download not permitted")
		}
	}

	return nil
}

// IsDownloadRequest checks if the request is a file download
func IsDownloadRequest(ctx *gin.Context) bool {
	// Check Content-Disposition header for downloads
	contentDisposition := ctx.GetHeader("Content-Disposition")
	if strings.Contains(contentDisposition, "attachment") {
		return true
	}

	// Check common download file extensions in URL path
	path := ctx.Request.URL.Path
	downloadExts := []string{".pdf", ".doc", ".docx", ".xls", ".xlsx", ".zip", ".rar", ".tar", ".gz", ".csv", ".txt"}
	for _, ext := range downloadExts {
		if strings.HasSuffix(strings.ToLower(path), ext) {
			return true
		}
	}

	// Check query parameters that indicate download intent
	if ctx.Query("download") != "" || ctx.Query("export") != "" || ctx.Query("attachment") != "" {
		return true
	}

	// Check Accept header for file download types
	accept := ctx.GetHeader("Accept")
	downloadMimeTypes := []string{
		"application/vnd.ms-excel",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		"application/pdf",
		"application/zip",
		"application/octet-stream",
	}
	for _, mimeType := range downloadMimeTypes {
		if strings.Contains(accept, mimeType) {
			return true
		}
	}

	return false
}

// RecordWebActivity records web session activity for auditing
func RecordWebActivity(sessionId string, ctx *gin.Context) {
	// Activity recording logic would go here
	// This is a placeholder to maintain API compatibility
}

// ProxyRequestContext holds the context for a proxy request
type ProxyRequestContext struct {
	SessionID        string
	AssetID          int
	Session          *WebProxySession
	Host             string
	IsStaticResource bool
}

// ExtractSessionAndAssetInfo extracts session ID and asset ID from the request
func ExtractSessionAndAssetInfo(ctx *gin.Context, extractAssetIDFromHost func(string) (int, error)) (*ProxyRequestContext, error) {
	host := ctx.Request.Host

	// Try to get session_id from multiple sources (priority order)
	sessionID := ctx.Query("session_id")

	// 1. Try from Cookie (preferred method)
	if sessionID == "" {
		if cookie, err := ctx.Cookie("oneterm_session_id"); err == nil && cookie != "" {
			sessionID = cookie
		}
	}

	// 2. Try from redirect parameter (for login redirects)
	if sessionID == "" {
		if redirect := ctx.Query("redirect"); redirect != "" {
			if decoded, err := url.QueryUnescape(redirect); err == nil {
				if decodedURL, err := url.Parse(decoded); err == nil {
					sessionID = decodedURL.Query().Get("session_id")
				}
			}
		}
	}

	// Extract asset ID from Host header: asset-11.oneterm.com -> 11
	assetID, err := extractAssetIDFromHost(host)
	if err != nil {
		return nil, fmt.Errorf("invalid subdomain format: %w", err)
	}

	// Try to get session_id from Referer header as fallback
	if sessionID == "" {
		referer := ctx.GetHeader("Referer")
		if referer != "" {
			if refererURL, err := url.Parse(referer); err == nil {
				sessionID = refererURL.Query().Get("session_id")
				// Also try to extract from fragment/hash part if URL encoded
				if sessionID == "" && strings.Contains(refererURL.RawQuery, "session_id") {
					// Handle URL encoded session_id in redirect parameter
					if redirect := refererURL.Query().Get("redirect"); redirect != "" {
						if decoded, err := url.QueryUnescape(redirect); err == nil {
							if decodedURL, err := url.Parse(decoded); err == nil {
								sessionID = decodedURL.Query().Get("session_id")
							}
						}
					}
				}
			}
		}
	}

	// For static resources, try harder to find session_id
	if sessionID == "" {
		// Check if this looks like a static resource
		isStaticResource := strings.Contains(ctx.Request.URL.Path, "/img/") ||
			strings.Contains(ctx.Request.URL.Path, "/css/") ||
			strings.Contains(ctx.Request.URL.Path, "/js/") ||
			strings.Contains(ctx.Request.URL.Path, "/assets/") ||
			strings.HasSuffix(ctx.Request.URL.Path, ".png") ||
			strings.HasSuffix(ctx.Request.URL.Path, ".jpg") ||
			strings.HasSuffix(ctx.Request.URL.Path, ".gif") ||
			strings.HasSuffix(ctx.Request.URL.Path, ".css") ||
			strings.HasSuffix(ctx.Request.URL.Path, ".js") ||
			strings.HasSuffix(ctx.Request.URL.Path, ".ico")

		if isStaticResource {
			// For static resources, find any valid session for this asset
			allSessions := GetAllSessions()
			for sid, session := range allSessions {
				if session.AssetId == assetID {
					sessionID = sid
					break
				}
			}
		}
	}

	if sessionID == "" {
		return nil, fmt.Errorf("session ID required - please start a new web session")
	}

	// Determine if this is a static resource request
	isStaticResource := strings.Contains(ctx.Request.URL.Path, "/img/") ||
		strings.Contains(ctx.Request.URL.Path, "/css/") ||
		strings.Contains(ctx.Request.URL.Path, "/js/") ||
		strings.Contains(ctx.Request.URL.Path, "/assets/") ||
		strings.HasSuffix(ctx.Request.URL.Path, ".png") ||
		strings.HasSuffix(ctx.Request.URL.Path, ".jpg") ||
		strings.HasSuffix(ctx.Request.URL.Path, ".gif") ||
		strings.HasSuffix(ctx.Request.URL.Path, ".css") ||
		strings.HasSuffix(ctx.Request.URL.Path, ".js") ||
		strings.HasSuffix(ctx.Request.URL.Path, ".ico") ||
		strings.HasSuffix(ctx.Request.URL.Path, ".woff") ||
		strings.HasSuffix(ctx.Request.URL.Path, ".woff2") ||
		strings.HasSuffix(ctx.Request.URL.Path, ".ttf") ||
		strings.HasSuffix(ctx.Request.URL.Path, ".svg")

	return &ProxyRequestContext{
		SessionID:        sessionID,
		AssetID:          assetID,
		Host:             host,
		IsStaticResource: isStaticResource,
	}, nil
}

// ValidateSessionAndPermissions validates the session and checks permissions
func ValidateSessionAndPermissions(ctx *gin.Context, proxyCtx *ProxyRequestContext, checkWebAccessControls func(*gin.Context, *WebProxySession) error) error {
	// Validate session ID and get session information
	session, exists := GetSession(proxyCtx.SessionID)
	if !exists {
		return fmt.Errorf("invalid or expired session")
	}

	// Check session timeout using system config (same as other protocols)
	now := time.Now()
	maxInactiveTime := time.Duration(model.GlobalConfig.Load().Timeout) * time.Second
	if now.Sub(session.LastActivity) > maxInactiveTime {
		CloseWebSession(proxyCtx.SessionID)
		return fmt.Errorf("session expired due to inactivity")
	}

	// Only update LastActivity for real user operations (not static resources)
	if !proxyCtx.IsStaticResource {
		UpdateSessionActivity(proxyCtx.SessionID)

		// Auto-renew cookie for user operations
		cookieMaxAge := int(model.GlobalConfig.Load().Timeout)
		ctx.SetCookie("oneterm_session_id", proxyCtx.SessionID, cookieMaxAge, "/", "", false, true)
	}

	// Check Web-specific access controls
	if err := checkWebAccessControls(ctx, session); err != nil {
		return err
	}

	if session.AssetId != proxyCtx.AssetID {
		return fmt.Errorf("asset ID mismatch")
	}

	// Store session in context
	proxyCtx.Session = session
	return nil
}

// SetupReverseProxy creates and configures a reverse proxy
func SetupReverseProxy(ctx *gin.Context, proxyCtx *ProxyRequestContext, buildTargetURLWithHost func(*model.Asset, string) string, processHTMLResponse func(*http.Response, int, string, string, *WebProxySession), recordWebActivity func(*WebProxySession, *http.Request), isSameDomainOrSubdomain func(string, string) bool) (*httputil.ReverseProxy, error) {
	targetURL := buildTargetURLWithHost(proxyCtx.Session.Asset, proxyCtx.Session.CurrentHost)
	target, err := url.Parse(targetURL)
	if err != nil {
		return nil, fmt.Errorf("invalid target URL")
	}

	currentScheme := lo.Ternary(ctx.Request.TLS != nil, "https", "http")

	// Create transparent reverse proxy
	proxy := httputil.NewSingleHostReverseProxy(target)

	// Configure proxy director for transparent proxying
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)

		req.Host = target.Host
		req.Header.Set("Host", target.Host)

		if origin := req.Header.Get("Origin"); origin != "" {
			req.Header.Set("Origin", target.Scheme+"://"+target.Host)
		}

		if referer := req.Header.Get("Referer"); referer != "" {
			if refererURL, err := url.Parse(referer); err == nil {
				refererURL.Scheme = target.Scheme
				refererURL.Host = target.Host
				req.Header.Set("Referer", refererURL.String())
			}
		}

		q := req.URL.Query()
		q.Del("session_id")
		req.URL.RawQuery = q.Encode()
	}

	// Redirect interception for bastion control
	proxy.ModifyResponse = func(resp *http.Response) error {
		// Check file download permissions based on response headers
		contentDisposition := resp.Header.Get("Content-Disposition")
		contentType := resp.Header.Get("Content-Type")

		// Check if this is a file download response
		isDownload := strings.Contains(contentDisposition, "attachment") ||
			strings.Contains(contentType, "application/octet-stream") ||
			strings.Contains(contentType, "application/vnd.ms-excel") ||
			strings.Contains(contentType, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet") ||
			strings.Contains(contentType, "application/pdf") ||
			strings.Contains(contentType, "application/zip")

		if isDownload && proxyCtx.Session.Permissions != nil && !proxyCtx.Session.Permissions.FileDownload {
			// Replace the response with a 403 error
			resp.StatusCode = http.StatusForbidden
			resp.Status = "403 Forbidden"
			resp.Header.Set("Content-Type", "application/json")
			resp.Header.Del("Content-Disposition")

			errorMsg := `{"error":"File download not permitted"}`
			resp.Body = io.NopCloser(strings.NewReader(errorMsg))
			resp.ContentLength = int64(len(errorMsg))
			resp.Header.Set("Content-Length", fmt.Sprintf("%d", len(errorMsg)))

			return nil
		}

		// Process HTML content for injection
		if resp.StatusCode == 200 && strings.Contains(contentType, "text/html") {
			processHTMLResponse(resp, proxyCtx.AssetID, currentScheme, proxyCtx.Host, proxyCtx.Session)
		}

		// Record activity if enabled
		if proxyCtx.Session.WebConfig != nil && proxyCtx.Session.WebConfig.ProxySettings != nil && proxyCtx.Session.WebConfig.ProxySettings.RecordingEnabled {
			recordWebActivity(proxyCtx.Session, ctx.Request)
		}

		if resp.StatusCode >= 300 && resp.StatusCode < 400 {
			location := resp.Header.Get("Location")
			if location != "" {
				redirectURL, err := url.Parse(location)
				if err != nil {
					return nil
				}

				shouldIntercept := redirectURL.IsAbs()

				if shouldIntercept {
					baseDomain := lo.Ternary(strings.HasPrefix(proxyCtx.Host, "asset-"),
						func() string {
							parts := strings.SplitN(proxyCtx.Host, ".", 2)
							return lo.Ternary(len(parts) > 1, parts[1], proxyCtx.Host)
						}(),
						proxyCtx.Host)

					if isSameDomainOrSubdomain(target.Host, redirectURL.Host) {
						UpdateSessionHost(proxyCtx.SessionID, redirectURL.Host)
						newProxyURL := fmt.Sprintf("%s://asset-%d.%s%s", currentScheme, proxyCtx.AssetID, baseDomain, redirectURL.Path)
						if redirectURL.RawQuery != "" {
							newProxyURL += "?" + redirectURL.RawQuery
						}
						resp.Header.Set("Location", newProxyURL)
					} else {
						newLocation := fmt.Sprintf("%s://asset-%d.%s/external?url=%s",
							currentScheme, proxyCtx.AssetID, baseDomain, url.QueryEscape(redirectURL.String()))
						resp.Header.Set("Location", newLocation)
					}
				} else {
					resp.Header.Set("Location", redirectURL.String())
				}
			}
		}

		if cookies := resp.Header["Set-Cookie"]; len(cookies) > 0 {
			proxyDomain := strings.Split(proxyCtx.Host, ":")[0]

			newCookies := lo.Map(cookies, func(cookie string, _ int) string {
				if strings.Contains(cookie, "Domain="+target.Host) {
					return strings.Replace(cookie, "Domain="+target.Host, "Domain="+proxyDomain, 1)
				}
				return cookie
			})
			resp.Header["Set-Cookie"] = newCookies
		}

		return nil
	}

	return proxy, nil
}
