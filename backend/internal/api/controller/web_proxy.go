package controller

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/samber/lo"
	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/service"
	"github.com/veops/oneterm/internal/service/web_proxy"
	"github.com/veops/oneterm/pkg/logger"
)

type WebProxyController struct{}

func NewWebProxyController() *WebProxyController {
	return &WebProxyController{}
}

// 使用service层的结构体
type StartWebSessionRequest = web_proxy.StartWebSessionRequest
type StartWebSessionResponse = web_proxy.StartWebSessionResponse

// 使用service层的全局变量和结构体
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

// GetWebAssetConfig get web asset configuration
// @Summary Get web asset configuration
// @Description Get web asset configuration by asset ID
// @Tags WebProxy
// @Accept json
// @Produce json
// @Param asset_id path int true "Asset ID"
// @Success 200 {object} model.WebConfig
// @Failure 400 {object} map[string]interface{} "Invalid asset ID"
// @Failure 404 {object} map[string]interface{} "Asset not found"
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
// @Accept json
// @Produce json
// @Param request body web_proxy.StartWebSessionRequest true "Start session request"
// @Success 200 {object} web_proxy.StartWebSessionResponse
// @Failure 400 {object} map[string]interface{} "Invalid request"
// @Failure 403 {object} map[string]interface{} "No permission"
// @Failure 404 {object} map[string]interface{} "Asset not found"
// @Failure 429 {object} map[string]interface{} "Maximum concurrent connections exceeded"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /web_proxy/start [post]
func (c *WebProxyController) StartWebSession(ctx *gin.Context) {
	var req StartWebSessionRequest
	if err := ctx.ShouldBindBodyWithJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := web_proxy.StartWebSession(ctx, req)
	if err != nil {
		// Return appropriate HTTP status code based on error type
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
// @Accept */*
// @Produce */*
// @Param Host header string true "Asset subdomain (asset-123.domain.com)"
// @Param session_id query string false "Session ID (alternative to cookie)"
// @Success 200 "Proxied content"
// @Failure 400 {object} map[string]interface{} "Invalid subdomain format"
// @Failure 401 "Session expired page"
// @Failure 403 {object} map[string]interface{} "Access denied"
// @Router /proxy [get]
func (c *WebProxyController) ProxyWebRequest(ctx *gin.Context) {
	host := ctx.Request.Host

	// Try to get session_id from multiple sources (priority order)
	sessionID := ctx.Query("session_id")

	// 1. Try from Cookie (preferred method)
	if sessionID == "" {
		if cookie, err := ctx.Cookie("oneterm_session_id"); err == nil && cookie != "" {
			sessionID = cookie
			logger.L().Debug("Extracted session_id from cookie", zap.String("sessionID", sessionID))
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
	assetID, err := c.extractAssetIDFromHost(host)
	if err != nil {
		logger.L().Error("Invalid subdomain format", zap.String("host", host), zap.Error(err))
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid subdomain format"})
		return
	}

	logger.L().Debug("Extracted asset ID", zap.Int("assetID", assetID))

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
			allSessions := web_proxy.GetAllSessions()
			for sid, session := range allSessions {
				if session.AssetId == assetID {
					sessionID = sid
					break
				}
			}
		}
	}

	if sessionID == "" {
		logger.L().Error("Missing session ID", zap.String("host", host))
		c.renderSessionExpiredPage(ctx, "Session ID required - please start a new web session")
		return
	}

	// Validate session ID and get session information
	session, exists := web_proxy.GetSession(sessionID)
	if !exists {
		c.renderSessionExpiredPage(ctx, "Invalid or expired session")
		return
	}

	// Check session timeout using system config (same as other protocols)
	now := time.Now()
	maxInactiveTime := time.Duration(model.GlobalConfig.Load().Timeout) * time.Second
	if now.Sub(session.LastActivity) > maxInactiveTime {
		web_proxy.CloseWebSession(sessionID)
		c.renderSessionExpiredPage(ctx, "Session expired due to inactivity")
		return
	}

	// Update last activity time and auto-renew cookie
	web_proxy.UpdateSessionActivity(sessionID)
	cookieMaxAge := int(model.GlobalConfig.Load().Timeout)
	ctx.SetCookie("oneterm_session_id", sessionID, cookieMaxAge, "/", "", false, true)

	// Update last activity
	web_proxy.UpdateSessionActivity(sessionID)

	// Check Web-specific access controls
	if err := c.checkWebAccessControls(ctx, session); err != nil {
		ctx.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	if session.AssetId != assetID {
		ctx.JSON(http.StatusForbidden, gin.H{"error": "Asset ID mismatch"})
		return
	}

	targetURL := c.buildTargetURLWithHost(session.Asset, session.CurrentHost)
	target, err := url.Parse(targetURL)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid target URL"})
		return
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
		contentType := resp.Header.Get("Content-Type")
		if resp.StatusCode == 200 && strings.Contains(contentType, "text/html") {
			c.processHTMLResponse(resp, assetID, currentScheme, host, session)
		}

		// Record activity if enabled
		if session.WebConfig != nil && session.WebConfig.ProxySettings != nil && session.WebConfig.ProxySettings.RecordingEnabled {
			c.recordWebActivity(session, ctx.Request)
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
					baseDomain := lo.Ternary(strings.HasPrefix(host, "asset-"),
						func() string {
							parts := strings.SplitN(host, ".", 2)
							return lo.Ternary(len(parts) > 1, parts[1], host)
						}(),
						host)

					if c.isSameDomainOrSubdomain(target.Host, redirectURL.Host) {
						web_proxy.UpdateSessionHost(sessionID, redirectURL.Host)
						newProxyURL := fmt.Sprintf("%s://asset-%d.%s%s", currentScheme, assetID, baseDomain, redirectURL.Path)
						if redirectURL.RawQuery != "" {
							newProxyURL += "?" + redirectURL.RawQuery
						}
						resp.Header.Set("Location", newProxyURL)
					} else {
						newLocation := fmt.Sprintf("%s://asset-%d.%s/external?url=%s",
							currentScheme, assetID, baseDomain, url.QueryEscape(redirectURL.String()))
						resp.Header.Set("Location", newLocation)
					}
				} else {
					resp.Header.Set("Location", redirectURL.String())
				}
			}
		}

		if cookies := resp.Header["Set-Cookie"]; len(cookies) > 0 {
			proxyDomain := strings.Split(host, ":")[0]

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

	ctx.Header("Cache-Control", "no-cache")

	proxy.ServeHTTP(ctx.Writer, ctx.Request)
}

// HandleExternalRedirect shows a page when an external redirect is blocked
// @Summary Handle external redirect
// @Description Show a page when an external redirect is blocked by the proxy
// @Tags WebProxy
// @Accept html
// @Produce html
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
// @Accept json
// @Produce json
// @Param request body map[string]string true "Session close request" example({"session_id": "web_123_456_1640000000"})
// @Success 200 {object} map[string]string "Session closed successfully"
// @Failure 400 {object} map[string]interface{} "Invalid request"
// @Failure 404 {object} map[string]interface{} "Session not found"
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
// @Accept json
// @Produce json
// @Param asset_id path int true "Asset ID"
// @Success 200 {array} map[string]interface{} "List of active sessions"
// @Failure 400 {object} map[string]interface{} "Invalid asset ID"
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
// @Accept json
// @Produce json
// @Param request body map[string]string true "Heartbeat request" example({"session_id": "web_123_456_1640000000"})
// @Success 200 {object} map[string]string "Heartbeat updated"
// @Failure 400 {object} map[string]interface{} "Invalid request"
// @Failure 404 {object} map[string]interface{} "Session not found"
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
		session.LastActivity = time.Now()
		ctx.JSON(http.StatusOK, gin.H{"status": "updated"})
	} else {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
	}
}

// CleanupWebSession handles browser tab close cleanup
// @Summary Cleanup web session
// @Description Clean up web session when browser tab is closed
// @Tags WebProxy
// @Accept json
// @Produce json
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
