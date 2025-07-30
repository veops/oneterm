package controller

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/samber/lo"
	"github.com/veops/oneterm/internal/model"
	"github.com/veops/oneterm/internal/service"
	"github.com/veops/oneterm/pkg/logger"
)

type WebProxyController struct{}

func NewWebProxyController() *WebProxyController {
	return &WebProxyController{}
}

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

var webProxySessions = make(map[string]*WebProxySession)

type WebProxySession struct {
	SessionId    string
	AssetId      int
	Asset        *model.Asset
	CreatedAt    time.Time
	LastActivity time.Time
	CurrentHost  string
}

func cleanupExpiredSessions(maxInactiveTime time.Duration) {
	now := time.Now()
	for sessionID, session := range webProxySessions {
		if now.Sub(session.LastActivity) > maxInactiveTime {
			delete(webProxySessions, sessionID)
			logger.L().Info("Cleaned up expired web session",
				zap.String("sessionID", sessionID))
		}
	}
}

func StartSessionCleanupRoutine() {
	ticker := time.NewTicker(10 * time.Minute)
	go func() {
		for range ticker.C {
			cleanupExpiredSessions(8 * time.Hour)
		}
	}()
}

func (c *WebProxyController) renderSessionExpiredPage(ctx *gin.Context, reason string) {
	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>Session Expired - OneTerm</title>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
        body { 
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%);
            min-height: 100vh;
            margin: 0;
            display: flex;
            align-items: center;
            justify-content: center;
        }
        .container {
            background: white;
            padding: 40px;
            border-radius: 12px;
            box-shadow: 0 10px 25px rgba(0,0,0,0.1);
            text-align: center;
            width: 100%%;
            max-width: 400px;
        }
        .icon { font-size: 4rem; margin-bottom: 20px; display: block; }
        .title { color: #333; font-size: 1.5rem; font-weight: 600; margin-bottom: 16px; }
        .message { color: #666; font-size: 1rem; line-height: 1.5; margin-bottom: 24px; }
        .reason {
            background: #f8f9fa;
            border-left: 4px solid #ffa726;
            padding: 12px 16px;
            margin: 20px 0;
            font-size: 0.9rem;
            color: #555;
            border-radius: 4px;
        }
        .button {
            background: #667eea;
            color: white;
            padding: 12px 24px;
            border: none;
            border-radius: 6px;
            font-size: 1rem;
            font-weight: 500;
            cursor: pointer;
            text-decoration: none;
            display: inline-block;
            transition: all 0.2s;
        }
        .button:hover { background: #5a6fd8; transform: translateY(-1px); }
        .footer { margin-top: 24px; font-size: 0.8rem; color: #999; }
    </style>
</head>
<body>
    <div class="container">
        <span class="icon">⏰</span>
        <div class="title">Session Expired</div>
        <div class="message">Your web proxy session has expired and you need to reconnect.</div>
        <div class="reason">Reason: %s</div>
        <a href="javascript:history.back()" class="button">← Go Back</a>
        <div class="footer">OneTerm Bastion Host</div>
    </div>
</body>
</html>`, reason)

	ctx.SetCookie("oneterm_session_id", "", -1, "/", "", false, true)
	ctx.Header("Content-Type", "text/html; charset=utf-8")
	ctx.String(http.StatusUnauthorized, html)
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
// @Accept json
// @Produce json
// @Param request body StartWebSessionRequest true "Start session request"
// @Success 200 {object} StartWebSessionResponse
// @Router /web_proxy/start [post]
func (c *WebProxyController) StartWebSession(ctx *gin.Context) {
	var req StartWebSessionRequest
	if err := ctx.ShouldBindBodyWithJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	assetService := service.NewAssetService()
	asset, err := assetService.GetById(ctx, req.AssetId)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Asset not found"})
		return
	}

	// Check if asset is web asset
	if !asset.IsWebAsset() {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Asset is not a web asset"})
		return
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

	// Create and store web proxy session
	now := time.Now()

	// Get initial target host from asset
	initialHost := c.getAssetHost(asset)

	webSession := &WebProxySession{
		SessionId:    sessionId,
		AssetId:      asset.Id,
		Asset:        asset,
		CreatedAt:    now,
		LastActivity: now,
		CurrentHost:  initialHost,
	}
	webProxySessions[sessionId] = webSession

	// Generate subdomain-based proxy URL
	scheme := "https"
	if ctx.Request.TLS == nil {
		scheme = "http"
	}

	// Extract base domain and port from current host
	currentHost := ctx.Request.Host
	var baseDomain string
	var portSuffix string

	if strings.Contains(currentHost, ":") {
		hostParts := strings.Split(currentHost, ":")
		baseDomain = hostParts[0]
		port := hostParts[1]

		// Keep port unless it's default
		isDefaultPort := (scheme == "http" && port == "80") || (scheme == "https" && port == "443")
		if !isDefaultPort {
			portSuffix = ":" + port
		}
	} else {
		baseDomain = currentHost
	}

	// Create subdomain URL with session_id for first access (cookie will handle subsequent requests)
	subdomainHost := fmt.Sprintf("asset-%d.%s%s", req.AssetId, baseDomain, portSuffix)
	proxyURL := fmt.Sprintf("%s://%s/?session_id=%s", scheme, subdomainHost, sessionId)

	ctx.JSON(http.StatusOK, StartWebSessionResponse{
		SessionId: sessionId,
		ProxyURL:  proxyURL,
		Message:   "Web session started successfully",
	})
}

// ProxyWebRequest handles subdomain-based web proxy requests
// Extract asset ID from Host header like: asset-123.oneterm.com
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
			for sid, session := range webProxySessions {
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

	// Get session from simple session store
	webSession, exists := webProxySessions[sessionID]
	if !exists {
		c.renderSessionExpiredPage(ctx, "Session not found")
		return
	}

	// Check session timeout (8 hours of inactivity)
	now := time.Now()
	maxInactiveTime := 8 * time.Hour
	if now.Sub(webSession.LastActivity) > maxInactiveTime {
		// Remove expired session
		delete(webProxySessions, sessionID)
		c.renderSessionExpiredPage(ctx, "Session expired due to inactivity")
		return
	}

	// Update last activity time and auto-renew cookie
	webSession.LastActivity = now
	ctx.SetCookie("oneterm_session_id", sessionID, 8*3600, "/", "", false, true)

	// Verify asset ID matches session
	if webSession.AssetId != assetID {
		ctx.JSON(http.StatusForbidden, gin.H{"error": "Asset ID mismatch"})
		return
	}

	// Build target URL using current host (may have been updated by redirects)
	targetURL := c.buildTargetURLWithHost(webSession.Asset, webSession.CurrentHost)
	target, err := url.Parse(targetURL)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid target URL"})
		return
	}

	// Set session_id cookie with smart expiration management
	// Use a longer cookie duration (8 hours) but validate session on each request
	cookieMaxAge := 8 * 3600 // 8 hours
	ctx.SetCookie("oneterm_session_id", sessionID, cookieMaxAge, "/", "", false, true)

	// Determine current request scheme for redirect rewriting
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
			c.rewriteHTMLContent(resp, assetID, currentScheme, host)
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
						webSession.CurrentHost = redirectURL.Host
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

// HandleExternalRedirect handles redirects to external domains through proxy
func (c *WebProxyController) HandleExternalRedirect(ctx *gin.Context) {
	targetURL := ctx.Query("url")

	// Get session_id from cookie instead of URL parameter
	sessionID, err := ctx.Cookie("oneterm_session_id")
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Session required"})
		return
	}

	if targetURL == "" || sessionID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Missing required parameters"})
		return
	}

	// Validate session
	webSession, exists := webProxySessions[sessionID]
	if !exists {
		c.renderSessionExpiredPage(ctx, "Session not found")
		return
	}

	// Check session timeout (8 hours of inactivity)
	now := time.Now()
	maxInactiveTime := 8 * time.Hour
	if now.Sub(webSession.LastActivity) > maxInactiveTime {
		// Remove expired session
		delete(webProxySessions, sessionID)
		c.renderSessionExpiredPage(ctx, "Session expired due to inactivity")
		return
	}

	// Update last activity time
	webSession.LastActivity = now

	// Return a simple page explaining the redirect was blocked
	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>External Redirect Blocked</title>
    <style>
        body { 
            font-family: Arial, sans-serif; 
            max-width: 600px; 
            margin: 50px auto; 
            padding: 20px;
            background: #f5f5f5;
        }
        .container {
            background: white;
            padding: 30px;
            border-radius: 8px;
            box-shadow: 0 2px 10px rgba(0,0,0,0.1);
        }
        .blocked { color: #e74c3c; }
        .info { color: #666; margin: 20px 0; }
        .target { 
            background: #f8f9fa; 
            padding: 10px; 
            border-radius: 4px; 
            word-break: break-all;
            font-family: monospace;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1 class="blocked">🛡️ External Redirect Blocked</h1>
        <div class="info">
            The target website attempted to redirect you to an external domain, 
            which has been blocked by the bastion host for security reasons.
        </div>
        <div class="info"><strong>Target URL:</strong></div>
        <div class="target">%s</div>
        <div class="info">
            All web access must go through the bastion host to maintain security 
            and audit compliance. External redirects are not permitted.
        </div>
        <div class="info">
            <a href="javascript:history.back()">← Go Back</a>
        </div>
    </div>
</body>
</html>`, targetURL)

	ctx.Header("Content-Type", "text/html; charset=utf-8")
	ctx.String(http.StatusOK, html)
}

// isSameDomainOrSubdomain checks if two hosts belong to the same domain or subdomain
// Examples:
// - baidu.com & www.baidu.com → true (subdomain)
// - baidu.com & m.baidu.com → true (subdomain)
// - baidu.com & google.com → false (different domain)
// - sub.example.com & other.example.com → true (same domain)
func (c *WebProxyController) isSameDomainOrSubdomain(host1, host2 string) bool {
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

// getAssetHost extracts the host from asset configuration
func (c *WebProxyController) getAssetHost(asset *model.Asset) string {
	targetURL := c.buildTargetURL(asset)
	if u, err := url.Parse(targetURL); err == nil {
		return u.Host
	}
	return "localhost" // fallback
}

// buildTargetURLWithHost builds target URL with specific host
func (c *WebProxyController) buildTargetURLWithHost(asset *model.Asset, host string) string {
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

// buildTargetURL builds the target URL from asset information
func (c *WebProxyController) buildTargetURL(asset *model.Asset) string {
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

// rewriteHTMLContent rewrites HTML content to redirect external links through proxy
func (c *WebProxyController) rewriteHTMLContent(resp *http.Response, assetID int, scheme, proxyHost string) {
	if resp.Body == nil {
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}
	resp.Body.Close()

	baseDomain := lo.Ternary(strings.HasPrefix(proxyHost, "asset-"),
		func() string {
			parts := strings.SplitN(proxyHost, ".", 2)
			return lo.Ternary(len(parts) > 1, parts[1], proxyHost)
		}(),
		proxyHost)

	content := string(body)

	// Universal URL rewriting patterns - catch ALL external URLs
	patterns := []struct {
		pattern string
		rewrite func(matches []string) string
	}{
		// JavaScript location assignments: window.location = "http://example.com/path"
		{
			`(window\.location(?:\.href)?\s*=\s*["'])https?://([^/'"]+)(/[^"']*)?["']`,
			func(matches []string) string {
				path := lo.Ternary(len(matches) > 3 && matches[3] != "", matches[3], "")
				return fmt.Sprintf(`%s%s://asset-%d.%s%s"`, matches[1], scheme, assetID, baseDomain, path)
			},
		},
		// Form actions: <form action="http://example.com/path"
		{
			`(action\s*=\s*["'])https?://([^/'"]+)(/[^"']*)?["']`,
			func(matches []string) string {
				path := lo.Ternary(len(matches) > 3 && matches[3] != "", matches[3], "")
				return fmt.Sprintf(`%s%s://asset-%d.%s%s"`, matches[1], scheme, assetID, baseDomain, path)
			},
		},
		// Link hrefs: <a href="http://example.com/path"
		{
			`(href\s*=\s*["'])https?://([^/'"]+)(/[^"']*)?["']`,
			func(matches []string) string {
				path := lo.Ternary(len(matches) > 3 && matches[3] != "", matches[3], "")
				return fmt.Sprintf(`%s%s://asset-%d.%s%s"`, matches[1], scheme, assetID, baseDomain, path)
			},
		},
	}

	for _, p := range patterns {
		re := regexp.MustCompile(p.pattern)
		content = re.ReplaceAllStringFunc(content, func(match string) string {
			matches := re.FindStringSubmatch(match)
			if len(matches) >= 4 {
				return p.rewrite(matches)
			}
			return match
		})
	}

	newBody := bytes.NewReader([]byte(content))
	resp.Body = io.NopCloser(newBody)
	resp.ContentLength = int64(len(content))
	resp.Header.Set("Content-Length", fmt.Sprintf("%d", len(content)))
}

// extractAssetIDFromHost extracts asset ID from subdomain host
// Examples: asset-123.oneterm.com -> 123, asset-456.localhost:8080 -> 456
func (c *WebProxyController) extractAssetIDFromHost(host string) (int, error) {
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
