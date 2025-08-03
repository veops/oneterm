package controller

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/service"
	"github.com/veops/oneterm/internal/service/web_proxy"
	"github.com/veops/oneterm/pkg/logger"
)

type WebProxyController struct{}

func NewWebProxyController() *WebProxyController {
	return &WebProxyController{}
}

type StartWebSessionRequest = web_proxy.StartWebSessionRequest
type StartWebSessionResponse = web_proxy.StartWebSessionResponse

type WebProxySession = web_proxy.WebProxySession

func StartSessionCleanupRoutine() {
	web_proxy.StartSessionCleanupRoutine()
}

func (c *WebProxyController) renderSessionExpiredPage(ctx *gin.Context, reason string) {
	html := web_proxy.RenderSessionExpiredPage(reason)
	ctx.SetCookie("oneterm_session_id", "", -1, "/", "", false, true)
	ctx.Header("Content-Type", "text/html; charset=utf-8")
	ctx.String(http.StatusUnauthorized, html)
}

func (c *WebProxyController) renderErrorPage(ctx *gin.Context, errorType, title, reason, details string) {
	html := web_proxy.RenderErrorPage(errorType, title, reason, details)
	ctx.Header("Content-Type", "text/html; charset=utf-8")

	// Set appropriate HTTP status code based on error type
	var statusCode int
	switch errorType {
	case "access_denied":
		statusCode = http.StatusForbidden
	case "session_expired":
		statusCode = http.StatusUnauthorized
	case "connection_error":
		statusCode = http.StatusBadGateway
	case "concurrent_limit":
		statusCode = http.StatusTooManyRequests
	case "server_error":
		statusCode = http.StatusInternalServerError
	default:
		statusCode = http.StatusInternalServerError
	}

	ctx.String(statusCode, html)
}

// GetWebAssetConfig get web asset configuration
// @Summary Get web asset configuration
// @Description Get web asset configuration by asset ID
// @Tags WebProxy
// @Param asset_id path int true "Asset ID"
// @Success 200 {object} model.WebConfig
// @Router /web_proxy/config/{asset_id} [get]
func (c *WebProxyController) GetWebAssetConfig(ctx *gin.Context) {
	assetIdStr := ctx.Param("asset_id")
	assetId, err := strconv.Atoi(assetIdStr)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid asset ID"})
		return
	}

	assetService := service.NewAssetService()
	asset, err := assetService.GetById(ctx, assetId)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Asset not found"})
		return
	}

	if !asset.IsWebAsset() {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Asset is not a web asset"})
		return
	}

	ctx.JSON(http.StatusOK, asset.WebConfig)
}

// StartWebSession start a new web session
// @Summary Start web session
// @Description Start a new web session for the specified asset
// @Tags WebProxy
// @Param request body web_proxy.StartWebSessionRequest true "Start session request"
// @Success 200 {object} web_proxy.StartWebSessionResponse
// @Router /web_proxy/start [post]
func (c *WebProxyController) StartWebSession(ctx *gin.Context) {
	var req StartWebSessionRequest
	if err := ctx.ShouldBindBodyWithJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := web_proxy.StartWebSession(ctx, req)
	if err != nil {
		// Return appropriate HTTP status code and JSON error for API
		if strings.Contains(err.Error(), "not found") {
			ctx.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else if strings.Contains(err.Error(), "not a web asset") {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		} else if strings.Contains(err.Error(), "No permission") {
			ctx.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		} else if strings.Contains(err.Error(), "maximum concurrent") {
			ctx.JSON(http.StatusTooManyRequests, gin.H{"error": err.Error()})
		} else {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	ctx.JSON(http.StatusOK, resp)
}

// ProxyWebRequest handles subdomain-based web proxy requests
// @Summary Proxy web requests
// @Description Handle web proxy requests for subdomain-based assets
// @Tags WebProxy
// @Param Host header string true "Asset subdomain (asset-123.domain.com)"
// @Param session_id query string false "Session ID (alternative to cookie)"
// @Success 200 "Proxied content"
// @Router /proxy [get]
func (c *WebProxyController) ProxyWebRequest(ctx *gin.Context) {
	// Extract session ID and asset ID from request
	proxyCtx, err := web_proxy.ExtractSessionAndAssetInfo(ctx, c.extractAssetIDFromHost)
	if err != nil {
		logger.L().Error("Failed to extract session/asset info", zap.Error(err))
		if strings.Contains(err.Error(), "invalid subdomain format") {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid subdomain format"})
		} else {
			c.renderSessionExpiredPage(ctx, err.Error())
		}
		return
	}

	// Validate session and check permissions
	if err := web_proxy.ValidateSessionAndPermissions(ctx, proxyCtx, c.checkWebAccessControls); err != nil {
		if strings.Contains(err.Error(), "invalid or expired session") || strings.Contains(err.Error(), "session expired") {
			c.renderSessionExpiredPage(ctx, err.Error())
		} else {
			c.renderErrorPage(ctx, "access_denied", "Access Denied", err.Error(), "Your request was blocked by the security policy.")
		}
		return
	}

	// Setup reverse proxy
	proxy, err := web_proxy.SetupReverseProxy(ctx, proxyCtx, c.buildTargetURLWithHost, c.processHTMLResponse, c.recordWebActivity, c.isSameDomainOrSubdomain)
	if err != nil {
		c.renderErrorPage(ctx, "server_error", "Proxy Setup Failed", err.Error(), "Failed to establish connection to the target server.")
		return
	}

	ctx.Header("Cache-Control", "no-cache")

	// Add panic recovery for proxy requests
	defer func() {
		if r := recover(); r != nil {
			logger.L().Error("Proxy request panic recovered",
				zap.String("url", ctx.Request.URL.String()),
				zap.String("host", ctx.Request.Host),
				zap.Any("panic", r))

			// Return appropriate error response instead of crashing
			if !ctx.Writer.Written() {
				ctx.JSON(http.StatusBadGateway, gin.H{
					"error":   "Proxy request failed",
					"details": "The target server is not responding properly",
				})
			}
		}
	}()

	proxy.ServeHTTP(ctx.Writer, ctx.Request)
}

// HandleExternalRedirect shows a page when an external redirect is blocked
// @Summary Handle external redirect
// @Description Show a page when an external redirect is blocked by the proxy
// @Tags WebProxy
// @Param url query string true "Target URL that was blocked"
// @Success 200 "External redirect blocked page"
// @Router /web_proxy/external_redirect [get]
func (c *WebProxyController) HandleExternalRedirect(ctx *gin.Context) {
	targetURL := ctx.Query("url")
	if targetURL == "" {
		targetURL = "Unknown URL"
	}

	html := web_proxy.RenderExternalRedirectPage(targetURL)
	ctx.Header("Content-Type", "text/html; charset=utf-8")
	ctx.String(http.StatusOK, html)
}

// isSameDomainOrSubdomain checks if two hosts belong to the same domain or subdomain
func (c *WebProxyController) isSameDomainOrSubdomain(host1, host2 string) bool {
	return web_proxy.IsSameDomainOrSubdomain(host1, host2)
}

// buildTargetURLWithHost builds target URL with specific host
func (c *WebProxyController) buildTargetURLWithHost(asset *model.Asset, host string) string {
	return web_proxy.BuildTargetURLWithHost(asset, host)
}

// processHTMLResponse processes HTML response for content rewriting and injection
func (c *WebProxyController) processHTMLResponse(resp *http.Response, assetID int, scheme, proxyHost string, session *WebProxySession) {
	web_proxy.ProcessHTMLResponse(resp, assetID, scheme, proxyHost, session)
}

// checkWebAccessControls validates web-specific access controls
func (c *WebProxyController) checkWebAccessControls(ctx *gin.Context, session *WebProxySession) error {
	return web_proxy.CheckWebAccessControls(ctx, session)
}

// getActiveSessionsForAsset returns detailed info about active sessions for an asset
func (c *WebProxyController) getActiveSessionsForAsset(assetId int) []map[string]interface{} {
	sessions := make([]map[string]interface{}, 0)
	for sessionId, session := range web_proxy.GetAllSessions() {
		if session.AssetId == assetId {
			sessions = append(sessions, map[string]interface{}{
				"session_id":    sessionId,
				"asset_id":      session.AssetId,
				"account_id":    session.AccountId,
				"created_at":    session.CreatedAt,
				"last_activity": session.LastActivity,
				"current_host":  session.CurrentHost,
			})
		}
	}
	return sessions
}

// CloseWebSession closes an active web session
// @Summary Close web session
// @Description Close an active web session and clean up resources
// @Tags WebProxy
// @Param request body map[string]string true "Session close request" example({"session_id": "web_123_456_1640000000"})
// @Success 200 {object} map[string]string "Session closed successfully"
// @Router /web_proxy/close [post]
func (c *WebProxyController) CloseWebSession(ctx *gin.Context) {
	var req struct {
		SessionID string `json:"session_id" binding:"required"`
	}

	if err := ctx.ShouldBindBodyWithJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	web_proxy.CloseWebSession(req.SessionID)
	ctx.JSON(http.StatusOK, gin.H{"message": "Session closed successfully"})
}

// GetActiveWebSessions gets active sessions for an asset
// @Summary Get active web sessions
// @Description Get list of active web sessions for a specific asset
// @Tags WebProxy
// @Param asset_id path int true "Asset ID"
// @Success 200 {array} map[string]interface{} "List of active sessions"
// @Router /web_proxy/sessions/{asset_id} [get]
func (c *WebProxyController) GetActiveWebSessions(ctx *gin.Context) {
	assetIdStr := ctx.Param("asset_id")
	assetId, err := strconv.Atoi(assetIdStr)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid asset ID"})
		return
	}

	sessions := c.getActiveSessionsForAsset(assetId)
	ctx.JSON(http.StatusOK, sessions)
}

// UpdateWebSessionHeartbeat updates session heartbeat
// @Summary Update session heartbeat
// @Description Update the last activity time for a web session (heartbeat)
// @Tags WebProxy
// @Param request body map[string]string true "Heartbeat request" example({"session_id": "web_123_456_1640000000"})
// @Success 200 {object} map[string]string "Heartbeat updated"
// @Router /web_proxy/heartbeat [post]
func (c *WebProxyController) UpdateWebSessionHeartbeat(ctx *gin.Context) {
	var req struct {
		SessionId string `json:"session_id" binding:"required"`
	}

	if err := ctx.ShouldBindBodyWithJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if session, exists := web_proxy.GetSession(req.SessionId); exists {
		// Update heartbeat - this extends session life and indicates user is still viewing
		web_proxy.UpdateSessionHeartbeat(req.SessionId)
		_ = session // Use the session variable to avoid unused warning
		ctx.JSON(http.StatusOK, gin.H{"status": "alive"})
	} else {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
	}
}

// CleanupWebSession handles browser tab close cleanup
// @Summary Cleanup web session
// @Description Clean up web session when browser tab is closed
// @Tags WebProxy
// @Param request body map[string]string true "Cleanup request" example({"session_id": "web_123_456_1640000000"})
// @Success 200 {object} map[string]string "Session cleaned up"
// @Router /web_proxy/cleanup [post]
func (c *WebProxyController) CleanupWebSession(ctx *gin.Context) {
	var req struct {
		SessionId string `json:"session_id"`
	}

	// Best effort parsing - browser might send malformed data on page unload
	ctx.ShouldBindBodyWithJSON(&req)

	if req.SessionId != "" {
		web_proxy.CloseWebSession(req.SessionId)
		logger.L().Info("Web session cleaned up by browser",
			zap.String("sessionId", req.SessionId))
	}

	ctx.Status(http.StatusOK)
}

// recordWebActivity records web session activity for audit
func (c *WebProxyController) recordWebActivity(session *WebProxySession, req *http.Request) {
	web_proxy.RecordWebActivity(session.SessionId, &gin.Context{Request: req})
}

// extractAssetIDFromHost extracts asset ID from subdomain host
func (c *WebProxyController) extractAssetIDFromHost(host string) (int, error) {
	return web_proxy.ExtractAssetIDFromHost(host)
}
