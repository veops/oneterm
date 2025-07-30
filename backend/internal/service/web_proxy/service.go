package web_proxy

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"go.uber.org/zap"

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

	// Use standard V2 authorization check (same as other asset types)
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
			return nil, fmt.Errorf("maximum concurrent connections (%d) exceeded", asset.WebConfig.ProxySettings.MaxConcurrent)
		}
	}

	now := time.Now()

	// Get initial target host from asset
	initialHost := GetAssetHost(asset)

	webSession := &WebProxySession{
		SessionId:    sessionId,
		AssetId:      asset.Id,
		AccountId:    req.AccountId,
		Asset:        asset,
		CreatedAt:    now,
		LastActivity: now,
		CurrentHost:  initialHost,
		Permissions:  permissions,
		WebConfig:    asset.WebConfig,
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

	// Use custom host instead of asset's original host
	if port == 80 && protocol == "http" || port == 443 && protocol == "https" {
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

	// Check common download file extensions
	path := ctx.Request.URL.Path
	downloadExts := []string{".pdf", ".doc", ".docx", ".xls", ".xlsx", ".zip", ".rar", ".tar", ".gz"}
	return lo.SomeBy(downloadExts, func(ext string) bool {
		return strings.HasSuffix(strings.ToLower(path), ext)
	})
}

// RecordWebActivity records web session activity for auditing
func RecordWebActivity(sessionId string, ctx *gin.Context) {
	// Activity recording logic would go here
	// This is a placeholder to maintain API compatibility
	logger.L().Debug("Recording web activity", zap.String("sessionId", sessionId), zap.String("path", ctx.Request.URL.Path))
}
