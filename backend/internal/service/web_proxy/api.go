package web_proxy

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/veops/oneterm/internal/acl"
	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/service"
	gsession "github.com/veops/oneterm/internal/session"
	"github.com/veops/oneterm/pkg/logger"
)

type StartWebSessionRequest struct {
	AssetId   int    `json:"asset_id" binding:"required"`
	AssetName string `json:"asset_name"`
	AuthMode  string `json:"auth_mode"`
	AccountId int    `json:"account_id"`
}

type StartWebSessionResponse struct {
	SessionId string `json:"session_id"`
	ProxyURL  string `json:"proxy_url"`
	Message   string `json:"message"`
}

type ProxyRequestContext struct {
	SessionID        string
	AssetID          int
	Session          *WebProxySession
	Host             string
	IsStaticResource bool
}

// StartWebSession starts a web session - compatible with existing API
func StartWebSession(ctx *gin.Context, req StartWebSessionRequest) (*StartWebSessionResponse, error) {
	assetService := service.NewAssetService()
	asset, err := assetService.GetById(ctx, req.AssetId)
	if err != nil {
		return nil, fmt.Errorf("asset not found")
	}

	if !asset.IsWebAsset() {
		return nil, fmt.Errorf("asset is not a web asset")
	}

	currentUser, err := acl.GetSessionFromCtx(ctx)
	if err != nil {
		return nil, fmt.Errorf("authentication required")
	}

	authService := service.NewAuthorizationV2Service()
	authResult, err := authService.GetAssetPermissions(ctx, req.AssetId, req.AccountId)
	if err != nil {
		return nil, fmt.Errorf("failed to check permissions: %v", err)
	}

	// Check if user has connect permission
	connectResult, exists := authResult.Results[model.ActionConnect]
	if !exists || !connectResult.Allowed {
		reason := "access denied"
		if exists && connectResult.Reason != "" {
			reason = connectResult.Reason
		}
		return nil, fmt.Errorf("connection not allowed: %s", reason)
	}

	permissions := &SessionPermissions{
		CanRead:     true,
		CanWrite:    true,
		CanDownload: false,
		CanUpload:   false,
	}

	// Check specific action permissions
	if downloadResult, exists := authResult.Results[model.ActionFileDownload]; exists && downloadResult.Allowed {
		permissions.CanDownload = true
	}
	if uploadResult, exists := authResult.Results[model.ActionFileUpload]; exists && uploadResult.Allowed {
		permissions.CanUpload = true
	}

	// Apply access policy restrictions from asset configuration
	if asset.WebConfig != nil && asset.WebConfig.AccessPolicy == "read_only" {
		permissions.CanWrite = false
		permissions.CanUpload = false
	}

	// Check concurrent connections limit
	if asset.WebConfig != nil && asset.WebConfig.ProxySettings != nil && asset.WebConfig.ProxySettings.MaxConcurrent > 0 {
		activeCount := GetActiveSessionsForAsset(req.AssetId)
		if activeCount >= asset.WebConfig.ProxySettings.MaxConcurrent {
			return nil, fmt.Errorf("maximum concurrent connections (%d) exceeded", asset.WebConfig.ProxySettings.MaxConcurrent)
		}
	}

	targetHost := getAssetHost(asset)

	protocol, port := asset.GetWebProtocol()
	if protocol == "" {
		protocol = "http"
		port = 80
	}

	session, err := GetCore().CreateSessionWithProtocol(req.AssetId, targetHost, currentUser.GetUserName(), permissions, protocol, port)
	if err != nil {
		return nil, err
	}

	// Generate proxy URL
	baseDomain := strings.Split(ctx.Request.Host, ":")[0]
	scheme := "http"
	if ctx.GetHeader("X-Forwarded-Proto") == "https" || ctx.Request.TLS != nil {
		scheme = "https"
	}

	portSuffix := ""
	if strings.Contains(ctx.Request.Host, ":") {
		portSuffix = ":" + strings.Split(ctx.Request.Host, ":")[1]
	}

	webproxyHost := fmt.Sprintf("webproxy.%s%s", baseDomain, portSuffix)
	proxyURL := fmt.Sprintf("%s://%s/?asset_id=%d&session_id=%s", scheme, webproxyHost, req.AssetId, session.ID)

	protocolStr := fmt.Sprintf("%s:%d", protocol, port)
	dbSession := &model.Session{
		SessionType: model.SESSIONTYPE_WEB,
		SessionId:   session.ID,
		Uid:         currentUser.GetUid(),
		UserName:    currentUser.GetUserName(),
		AssetId:     asset.Id,
		AssetInfo:   fmt.Sprintf("%s(%s)", asset.Name, asset.Ip),
		AccountId:   req.AccountId,
		AccountInfo: "",
		GatewayId:   asset.GatewayId,
		GatewayInfo: "",
		ClientIp:    ctx.ClientIP(),
		Protocol:    protocolStr,
		Status:      model.SESSIONSTATUS_ONLINE,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if asset.GatewayId > 0 {
		dbSession.GatewayInfo = fmt.Sprintf("Gateway_%d", asset.GatewayId)
	}

	fullSession := &gsession.Session{Session: dbSession}
	if err := gsession.UpsertSession(fullSession); err != nil {
		logger.L().Error("Failed to save web session to database", zap.String("sessionId", session.ID), zap.Error(err))
	}

	return &StartWebSessionResponse{
		SessionId: session.ID,
		ProxyURL:  proxyURL,
		Message:   "Web session started successfully",
	}, nil
}

// ExtractSessionAndAssetInfo compatible with existing controller
func ExtractSessionAndAssetInfo(ctx *gin.Context, extractAssetIDFromHost func(string) (int, error)) (*ProxyRequestContext, error) {
	reqCtx, err := GetCore().ParseRequestContext(ctx)
	if err != nil {
		return nil, err
	}

	// For static resources, if session is found, use it directly
	if reqCtx.IsStatic && reqCtx.Session != nil {
		assetService := service.NewAssetService()
		asset, err := assetService.GetById(ctx, reqCtx.Session.AssetID)
		if err == nil {
			webProxySession := &WebProxySession{
				SessionId:    reqCtx.Session.ID,
				AssetId:      reqCtx.Session.AssetID,
				AccountId:    -1,
				Asset:        asset,
				CreatedAt:    reqCtx.Session.CreatedAt,
				LastActivity: reqCtx.Session.LastActivity,
				IsActive:     reqCtx.Session.IsActive,
				CurrentHost:  reqCtx.Session.CurrentHost,
				SessionPerms: reqCtx.Session.Permissions,
			}

			return &ProxyRequestContext{
				SessionID:        reqCtx.Session.ID,
				AssetID:          reqCtx.Session.AssetID,
				Session:          webProxySession,
				Host:             reqCtx.ProxyHost,
				IsStaticResource: reqCtx.IsStatic,
			}, nil
		}
	}

	if reqCtx.IsStatic && reqCtx.Session == nil {
		return &ProxyRequestContext{
			SessionID:        reqCtx.SessionID,
			AssetID:          reqCtx.AssetID,
			Session:          nil,
			Host:             reqCtx.ProxyHost,
			IsStaticResource: reqCtx.IsStatic,
		}, nil
	}

	// Get asset information
	assetService := service.NewAssetService()
	asset, err := assetService.GetById(ctx, reqCtx.AssetID)
	if err != nil {
		return nil, fmt.Errorf("failed to get asset info: %v", err)
	}

	// Convert WebSession to WebProxySession (compatibility layer)
	webProxySession := &WebProxySession{
		SessionId:    reqCtx.Session.ID,
		AssetId:      reqCtx.Session.AssetID,
		AccountId:    -1,
		Asset:        asset, // Set asset information
		CreatedAt:    reqCtx.Session.CreatedAt,
		LastActivity: reqCtx.Session.LastActivity,
		IsActive:     reqCtx.Session.IsActive,
		CurrentHost:  reqCtx.Session.CurrentHost,
		SessionPerms: reqCtx.Session.Permissions,
	}

	return &ProxyRequestContext{
		SessionID:        reqCtx.SessionID,
		AssetID:          reqCtx.AssetID,
		Session:          webProxySession,
		Host:             reqCtx.ProxyHost,
		IsStaticResource: reqCtx.IsStatic,
	}, nil
}

// ValidateSessionAndPermissions compatible with existing controller
func ValidateSessionAndPermissions(ctx *gin.Context, proxyCtx *ProxyRequestContext, checkWebAccessControls func(*gin.Context, *WebProxySession) error) error {
	if !proxyCtx.IsStaticResource {
		GetCore().UpdateSessionActivity(proxyCtx.SessionID)
	}

	if err := checkWebAccessControls(ctx, proxyCtx.Session); err != nil {
		return err
	}

	return nil
}

// SetupReverseProxy compatible with existing controller
func SetupReverseProxy(ctx *gin.Context, proxyCtx *ProxyRequestContext, buildTargetURLWithHost func(*model.Asset, string) string, processHTMLResponse func(*http.Response, int, string, string, *WebProxySession), recordWebActivity func(*WebProxySession, *http.Request), isSameDomainOrSubdomain func(string, string) bool) (*httputil.ReverseProxy, error) {

	if proxyCtx.IsStaticResource && proxyCtx.Session == nil {
		ctx.Status(404)
		ctx.String(404, "Static resource not available")
		return nil, nil
	}

	// Get asset information to determine protocol and port
	var targetScheme string = "http"
	var targetPort int = 80

	if proxyCtx.Session != nil && proxyCtx.Session.Asset != nil {
		protocol, port := proxyCtx.Session.Asset.GetWebProtocol()
		if protocol != "" {
			targetScheme = protocol
			targetPort = port
		}
	}

	// Use cached permissions from session (set during Start phase for performance)
	permissions := proxyCtx.Session.SessionPerms
	if permissions == nil {
		return nil, fmt.Errorf("session permissions not initialized - please restart session")
	}

	webSession := &WebSession{
		ID:           proxyCtx.Session.SessionId,
		AssetID:      proxyCtx.Session.AssetId,
		AssetHost:    proxyCtx.Session.CurrentHost,
		UserID:       "webproxy_user",
		CreatedAt:    proxyCtx.Session.CreatedAt,
		LastActivity: proxyCtx.Session.LastActivity,
		IsActive:     proxyCtx.Session.IsActive,
		CurrentHost:  proxyCtx.Session.CurrentHost,
		TargetScheme: targetScheme,
		TargetPort:   targetPort,
		Permissions:  permissions,
	}

	reqCtx := &RequestContext{
		SessionID:   proxyCtx.SessionID,
		AssetID:     proxyCtx.AssetID,
		Session:     webSession,
		IsStatic:    proxyCtx.IsStaticResource,
		OriginalURL: ctx.Request.URL.String(),
		ProxyHost:   proxyCtx.Host,
	}

	return GetCore().CreateReverseProxy(reqCtx)
}

func getAssetHost(asset *model.Asset) string {
	protocol, port := asset.GetWebProtocol()
	if protocol == "" {
		protocol = "http"
		port = 80
	}

	if strings.Contains(asset.Ip, ":") {
		return asset.Ip
	}

	if (protocol == "http" && port == 80) || (protocol == "https" && port == 443) {
		return asset.Ip
	}

	return fmt.Sprintf("%s:%d", asset.Ip, port)
}

func ExtractAssetIDFromHost(host string) (int, error) {
	return 0, fmt.Errorf("not supported in fixed webproxy subdomain approach")
}

func BuildTargetURLWithHost(asset *model.Asset, host string) string {
	protocol, port := asset.GetWebProtocol()
	if protocol == "" {
		protocol = "http"
		port = 80
	}

	if strings.Contains(host, ":") {
		return fmt.Sprintf("%s://%s", protocol, host)
	}

	if (port == 80 && protocol == "http") || (port == 443 && protocol == "https") {
		return fmt.Sprintf("%s://%s", protocol, host)
	}
	return fmt.Sprintf("%s://%s:%d", protocol, host, port)
}

// isSameDomain checks if two hosts belong to the same domain
func isSameDomain(host1, host2 string) bool {
	if host1 == host2 {
		return true
	}

	host1 = strings.Split(host1, ":")[0]
	host2 = strings.Split(host2, ":")[0]

	parts1 := strings.Split(host1, ".")
	parts2 := strings.Split(host2, ".")

	if len(parts1) < 2 || len(parts2) < 2 {
		return false
	}

	// Compare the last two parts (domain.tld)
	domain1 := strings.Join(parts1[len(parts1)-2:], ".")
	domain2 := strings.Join(parts2[len(parts2)-2:], ".")

	return domain1 == domain2
}

// IsSameDomainOrSubdomain compatible with existing controller
func IsSameDomainOrSubdomain(host1, host2 string) bool {
	return isSameDomain(host1, host2)
}

func CheckWebAccessControls(ctx *gin.Context, session *WebProxySession) error {
	if session == nil || session.Asset == nil {
		return fmt.Errorf("invalid session or asset")
	}

	method := strings.ToUpper(ctx.Request.Method)
	requestPath := ctx.Request.URL.Path

	// Check access policy restrictions
	if session.Asset.WebConfig != nil {
		if session.Asset.WebConfig.AccessPolicy == "read_only" {
			// Only allow safe HTTP methods for read-only access
			allowedReadOnlyMethods := []string{"GET", "HEAD", "OPTIONS"}
			allowed := false
			for _, allowedMethod := range allowedReadOnlyMethods {
				if method == allowedMethod {
					allowed = true
					break
				}
			}
			if !allowed {
				return fmt.Errorf("method %s not allowed in read-only mode", method)
			}
		}

		// Check proxy settings if available
		if session.Asset.WebConfig.ProxySettings != nil {
			proxySettings := session.Asset.WebConfig.ProxySettings

			// Check allowed HTTP methods
			if len(proxySettings.AllowedMethods) > 0 {
				methodAllowed := false
				for _, allowedMethod := range proxySettings.AllowedMethods {
					if strings.ToUpper(allowedMethod) == method {
						methodAllowed = true
						break
					}
				}
				if !methodAllowed {
					return fmt.Errorf("HTTP method %s is not allowed", method)
				}
			}

			// Check blocked paths
			if len(proxySettings.BlockedPaths) > 0 {
				for _, blockedPath := range proxySettings.BlockedPaths {
					if strings.HasPrefix(requestPath, blockedPath) {
						return fmt.Errorf("access to path %s is blocked", requestPath)
					}
				}
			}
		}
	}

	return nil
}

// RecordWebActivity compatible with existing controller
func RecordWebActivity(sessionId string, ctx *gin.Context) {
	// Placeholder implementation for recording activity
}

// ProcessHTMLResponse compatible with existing controller
func ProcessHTMLResponse(resp *http.Response, assetID int, scheme, proxyHost string, session *WebProxySession) {
	webSession := &WebSession{
		ID:           session.SessionId,
		AssetID:      session.AssetId,
		CreatedAt:    session.CreatedAt,
		LastActivity: session.LastActivity,
		IsActive:     session.IsActive,
		CurrentHost:  session.CurrentHost,
		// Simplified permission conversion
		Permissions: &SessionPermissions{
			CanRead:     true, // Default allow read
			CanWrite:    true, // Default allow write
			CanDownload: true, // Default allow download
			CanUpload:   true, // Default allow upload
		},
	}

	reqCtx := &RequestContext{
		AssetID:     assetID,
		SessionID:   session.SessionId,
		Session:     webSession,
		ProxyHost:   proxyHost,
		OriginalURL: fmt.Sprintf("%s://%s", scheme, proxyHost),
	}
	processHTMLContent(resp, reqCtx)
}

// Render functions
func RenderSessionExpiredPage(reason string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>Session Expired - OneTerm</title>
    <meta charset="utf-8">
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; background: #f5f5f5; }
        .container { max-width: 500px; margin: 0 auto; background: white; padding: 40px; border-radius: 8px; }
        .title { color: #d32f2f; font-size: 24px; margin-bottom: 20px; }
        .message { color: #666; line-height: 1.6; }
        .btn { background: #1976d2; color: white; padding: 10px 20px; text-decoration: none; border-radius: 4px; }
    </style>
</head>
<body>
    <div class="container">
        <h1 class="title">🕐 Session Expired</h1>
        <div class="message">%s</div>
        <p><a href="javascript:history.back()" class="btn">← Go Back</a></p>
    </div>
</body>
</html>`, reason)
}

func RenderErrorPage(errorType, title, reason, details string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>%s - OneTerm</title>
    <meta charset="utf-8">
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; background: #f5f5f5; }
        .container { max-width: 600px; margin: 0 auto; background: white; padding: 40px; border-radius: 8px; }
        .title { color: #d32f2f; font-size: 24px; margin-bottom: 20px; }
        .message { color: #666; line-height: 1.6; margin-bottom: 20px; }
        .details { background: #f8f9fa; padding: 15px; border-left: 4px solid #d32f2f; font-family: monospace; }
        .btn { background: #1976d2; color: white; padding: 10px 20px; text-decoration: none; border-radius: 4px; margin-right: 10px; }
    </style>
</head>
<body>
    <div class="container">
        <h1 class="title">❌ %s</h1>
        <div class="message">%s</div>
        %s
        <p>
            <a href="javascript:history.back()" class="btn">← Go Back</a>
            <a href="javascript:location.reload()" class="btn">🔄 Refresh</a>
        </p>
    </div>
</body>
</html>`, title, title, reason,
		func() string {
			if details != "" {
				return fmt.Sprintf(`<div class="details">%s</div>`, details)
			}
			return ""
		}())
}

func RenderAccessDeniedPage(reason, details string) string {
	return RenderErrorPage("access_denied", "Access Denied", reason, details)
}

func RenderExternalRedirectPage(targetURL string) string {
	return RenderErrorPage("external_redirect", "External Redirect Blocked",
		fmt.Sprintf("Target URL: %s", targetURL),
		"External redirects are blocked for security reasons")
}
