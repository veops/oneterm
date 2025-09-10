package web_proxy

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/veops/oneterm/pkg/logger"
)

type WebProxyCore struct {
	sessions sync.Map // session storage
	config   *ProxyConfig
}

type ProxyConfig struct {
	SessionTimeout time.Duration
	MaxSessions    int
}

type WebSession struct {
	ID           string
	AssetID      int
	AssetHost    string
	UserID       string
	CreatedAt    time.Time
	LastActivity time.Time
	IsActive     bool

	// Proxy state
	CurrentHost  string
	TargetScheme string
	TargetPort   int

	// Permissions
	Permissions *SessionPermissions
}

type SessionPermissions struct {
	CanRead     bool
	CanWrite    bool
	CanDownload bool
	CanUpload   bool
}

type RequestContext struct {
	SessionID   string
	AssetID     int
	Session     *WebSession
	IsStatic    bool
	OriginalURL string
	ProxyHost   string
	TargetURL   string
}

var globalCore *WebProxyCore

func init() {
	globalCore = &WebProxyCore{
		config: &ProxyConfig{
			SessionTimeout: 30 * time.Minute,
			MaxSessions:    100,
		},
	}
}

// GetCore returns the global proxy core instance
func GetCore() *WebProxyCore {
	return globalCore
}

// CreateSession creates a new web session with default HTTP protocol
func (c *WebProxyCore) CreateSession(assetID int, assetHost, userID string, permissions *SessionPermissions) (*WebSession, error) {
	return c.CreateSessionWithProtocol(assetID, assetHost, userID, permissions, "http", 80)
}

// CreateSessionWithProtocol creates a new web session with specified protocol and port
func (c *WebProxyCore) CreateSessionWithProtocol(assetID int, assetHost, userID string, permissions *SessionPermissions, scheme string, port int) (*WebSession, error) {
	// Generate unique session ID: web_{assetId}_{timestamp}
	sessionID := fmt.Sprintf("web_%d_%d", assetID, time.Now().UnixMicro())

	session := &WebSession{
		ID:           sessionID,
		AssetID:      assetID,
		AssetHost:    assetHost,
		UserID:       userID,
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
		IsActive:     true,
		CurrentHost:  assetHost,
		TargetScheme: scheme,
		TargetPort:   port,
		Permissions:  permissions,
	}

	c.sessions.Store(sessionID, session)

	// Store in legacy global session storage for compatibility
	webProxySession := &WebProxySession{
		SessionId:     sessionID,
		AssetId:       assetID,
		CreatedAt:     session.CreatedAt,
		LastActivity:  session.LastActivity,
		LastHeartbeat: time.Now(),
		IsActive:      true,
		CurrentHost:   assetHost,
		SessionPerms:  permissions, // Cache permissions for proxy phase
	}
	StoreSession(sessionID, webProxySession)

	return session, nil
}

// GetSession retrieves a session by ID and checks if it's expired
func (c *WebProxyCore) GetSession(sessionID string) (*WebSession, bool) {
	if val, ok := c.sessions.Load(sessionID); ok {
		session := val.(*WebSession)

		if time.Since(session.LastActivity) > c.config.SessionTimeout {
			c.CloseSession(sessionID)
			return nil, false
		}

		return session, true
	}
	return nil, false
}

// UpdateSessionActivity updates the last activity time for a session
func (c *WebProxyCore) UpdateSessionActivity(sessionID string) {
	if val, ok := c.sessions.Load(sessionID); ok {
		session := val.(*WebSession)
		session.LastActivity = time.Now()
		c.sessions.Store(sessionID, session)

		if oldSession, exists := GetSession(sessionID); exists {
			oldSession.LastActivity = time.Now()
		}
	}
}

// UpdateSessionHost updates the current host for a session to handle redirects
func (c *WebProxyCore) UpdateSessionHost(sessionID string, newHost string) {
	if val, ok := c.sessions.Load(sessionID); ok {
		session := val.(*WebSession)
		session.CurrentHost = newHost
		c.sessions.Store(sessionID, session)

		if oldSession, exists := GetSession(sessionID); exists {
			oldSession.CurrentHost = newHost
		}
	}
}

// CloseSession closes and removes a session
func (c *WebProxyCore) CloseSession(sessionID string) {
	if val, ok := c.sessions.Load(sessionID); ok {
		session := val.(*WebSession)
		session.IsActive = false
		c.sessions.Delete(sessionID)

		CloseWebSession(sessionID)

		logger.L().Info("Web session closed", zap.String("sessionId", sessionID))
	}
}

// GetActiveSessionsForAsset returns the number of active sessions for an asset
func (c *WebProxyCore) GetActiveSessionsForAsset(assetID int) int {
	count := 0
	c.sessions.Range(func(key, value any) bool {
		session := value.(*WebSession)
		if session.AssetID == assetID && session.IsActive {
			count++
		}
		return true
	})
	return count
}

// ParseRequestContext extracts session and asset information from request
func (c *WebProxyCore) ParseRequestContext(ctx *gin.Context) (*RequestContext, error) {
	var sessionID string
	var assetID int

	isStatic := c.isStaticResource(ctx.Request.URL.Path)

	// 1. Extract from URL parameters first (supports both static and non-static resources)
	sessionID = ctx.Query("session_id")
	assetIDStr := ctx.Query("asset_id")
	targetHost := ctx.Query("target_host")

	if sessionID != "" && assetIDStr != "" {
		if id, err := strconv.Atoi(assetIDStr); err == nil {
			assetID = id
		} else {
			logger.L().Warn("Failed to parse asset_id", zap.String("assetIDStr", assetIDStr), zap.Error(err))
		}
	}

	// For static resources, prioritize Referer over all other fallbacks
	if isStatic && (sessionID == "" || assetID == 0) {
		refererSessionID, refererAssetID := c.extractFromReferer(ctx.GetHeader("Referer"))

		// For static resources, Referer is the most reliable source - use it if available
		if refererSessionID != "" && refererAssetID != 0 {
			sessionID = refererSessionID
			assetID = refererAssetID
		} else {
			// If Referer extraction fails completely, return empty context
			// This prevents random session mixing which causes wrong asset_id usage
			return &RequestContext{
				SessionID:   "",
				AssetID:     0,
				Session:     nil,
				IsStatic:    true,
				OriginalURL: ctx.Request.URL.String(),
				ProxyHost:   ctx.Request.Host,
			}, nil
		}
	}

	// 2. If URL parameters are incomplete, try to parse asset ID from session ID
	if assetID == 0 && sessionID != "" {
		assetID = c.extractAssetIDFromSession(sessionID)
	}

	// 3. If still missing, try to extract from Referer (non-static resources)
	if !isStatic && (sessionID == "" || assetID == 0) {
		refererSessionID, refererAssetID := c.extractFromReferer(ctx.GetHeader("Referer"))
		if refererSessionID != "" {
			sessionID = refererSessionID
		}
		if refererAssetID != 0 {
			assetID = refererAssetID
		}
	}

	// 4. Static resources without proper session context should be handled by fallback
	// Removed: dangerous random session selection that causes cross-session pollution

	// 5. For non-static resources, validate parameter completeness
	if !isStatic && (sessionID == "" || assetID == 0) {
		logger.L().Error("Missing parameters after all extraction attempts",
			zap.String("sessionID", sessionID),
			zap.Int("assetID", assetID))
		return nil, fmt.Errorf("missing session_id or asset_id parameters")
	}

	// 5. Get session (static resources may not have session)
	var session *WebSession
	if sessionID != "" {
		if sess, exists := c.GetSession(sessionID); exists {
			session = sess
			// 6. Validate asset matching (only when session exists)
			if session.AssetID != assetID {
				return nil, fmt.Errorf("asset mismatch: session=%d, request=%d", session.AssetID, assetID)
			}

			// 7. If target_host is specified in URL, update session's CurrentHost (for redirect handling)
			if targetHost != "" {
				session.CurrentHost = targetHost
			}
		} else if !isStatic {
			return nil, fmt.Errorf("invalid or expired session: %s", sessionID)
		}
	}

	return &RequestContext{
		SessionID:   sessionID,
		AssetID:     assetID,
		Session:     session,
		IsStatic:    isStatic,
		OriginalURL: ctx.Request.URL.String(),
		ProxyHost:   ctx.Request.Host,
	}, nil
}

// extractAssetIDFromSession extracts asset ID from session ID format
func (c *WebProxyCore) extractAssetIDFromSession(sessionID string) int {
	parts := strings.Split(sessionID, "_")
	if len(parts) >= 2 && parts[0] == "web" {
		if assetID, err := strconv.Atoi(parts[1]); err == nil {
			return assetID
		}
	}
	return 0
}

// extractFromReferer extracts session information from Referer header
func (c *WebProxyCore) extractFromReferer(referer string) (string, int) {
	if referer == "" {
		return "", 0
	}

	if refURL, err := url.Parse(referer); err == nil {
		sessionID := refURL.Query().Get("session_id")
		assetIDStr := refURL.Query().Get("asset_id")

		assetID := 0
		if assetIDStr != "" {
			assetID, _ = strconv.Atoi(assetIDStr)
		} else if sessionID != "" {
			assetID = c.extractAssetIDFromSession(sessionID)
		}

		return sessionID, assetID
	}

	return "", 0
}

// isStaticResource checks if the path refers to a static resource
func (c *WebProxyCore) isStaticResource(path string) bool {
	staticExts := []string{
		".css", ".js", ".png", ".jpg", ".jpeg", ".gif", ".svg", ".ico",
		".woff", ".woff2", ".ttf", ".eot", ".mp3", ".mp4", ".pdf",
		".zip", ".rar", ".doc", ".docx", ".xls", ".xlsx",
	}

	lowerPath := strings.ToLower(path)
	for _, ext := range staticExts {
		if strings.HasSuffix(lowerPath, ext) {
			return true
		}
	}

	staticPaths := []string{"/static/", "/assets/", "/css/", "/js/", "/img/", "/images/", "/fonts/"}
	for _, staticPath := range staticPaths {
		if strings.Contains(lowerPath, staticPath) {
			return true
		}
	}

	return false
}

// CreateReverseProxy creates a reverse proxy for the session
func (c *WebProxyCore) CreateReverseProxy(reqCtx *RequestContext) (*httputil.ReverseProxy, error) {
	if reqCtx.IsStatic && reqCtx.Session == nil {
		return nil, fmt.Errorf("static resource request without valid session context")
	}

	session := reqCtx.Session

	if session.TargetScheme == "" {
		session.TargetScheme = "http"
		session.TargetPort = 80
		logger.L().Warn("Session missing target scheme, using default",
			zap.String("sessionId", session.ID))
	}

	if session.CurrentHost == "" {
		return nil, fmt.Errorf("session has no target host configured")
	}

	targetHost := session.CurrentHost
	if targetHost == "" {
		targetHost = session.AssetHost
	}

	_, _, err := net.SplitHostPort(targetHost)
	hasPort := err == nil

	var targetURL string
	if hasPort {
		targetURL = fmt.Sprintf("%s://%s", session.TargetScheme, targetHost)
	} else {
		targetURL = fmt.Sprintf("%s://%s", session.TargetScheme, targetHost)
		if (session.TargetScheme == "http" && session.TargetPort != 80) ||
			(session.TargetScheme == "https" && session.TargetPort != 443) {
			targetURL = fmt.Sprintf("%s://%s:%d", session.TargetScheme, targetHost, session.TargetPort)
		}
	}

	target, err := url.Parse(targetURL)
	if err != nil {
		return nil, fmt.Errorf("invalid target URL: %s", targetURL)
	}

	reqCtx.TargetURL = targetURL

	proxy := httputil.NewSingleHostReverseProxy(target)

	// Custom request handler
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)

		req.Host = target.Host
		req.Header.Set("Host", target.Host)

		if req.Header.Get("User-Agent") == "" {
			req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
		}

		req.Header.Del("X-Forwarded-For")
		req.Header.Del("X-Forwarded-Proto")
		req.Header.Del("X-Forwarded-Host")
		req.Header.Del("X-Real-IP")
		req.Header.Del("X-Proxy-Authorization")
		req.Header.Del("Proxy-Authorization")
		req.Header.Del("Proxy-Connection")
		req.Header.Del("Via")
		req.Header.Del("X-Proxy-Connection")
		req.Header.Del("Proxy-Authenticate")
		req.Header.Del("X-Forwarded-Server")

		// Don't add any IP-related headers to make request look direct
		// Server will see proxy's own IP, which looks more like CDN

		// Add common browser headers for enhanced stealth
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
		req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
		req.Header.Set("Accept-Encoding", "gzip, deflate, br")
		req.Header.Set("DNT", "1")
		req.Header.Set("Connection", "keep-alive")
		req.Header.Set("Upgrade-Insecure-Requests", "1")

		// Add modern browser client hints
		req.Header.Set("sec-ch-ua", `"Not_A Brand";v="8", "Chromium";v="120", "Google Chrome";v="120"`)
		req.Header.Set("sec-ch-ua-mobile", "?0")
		req.Header.Set("sec-ch-ua-platform", `"Windows"`)

		// Simulate real browser cache control
		if req.Header.Get("Cache-Control") == "" {
			req.Header.Set("Cache-Control", "max-age=0")
		}

		// Add standard browser Sec-Fetch headers (no special handling)
		req.Header.Set("Sec-Fetch-Site", "same-origin")
		req.Header.Set("Sec-Fetch-Mode", "navigate")
		req.Header.Set("Sec-Fetch-User", "?1")
		req.Header.Set("Sec-Fetch-Dest", "document")

		req.Header.Del("X-Forwarded-Host")

		// Rewrite Origin header to match target host
		if origin := req.Header.Get("Origin"); origin != "" {
			req.Header.Set("Origin", target.Scheme+"://"+target.Host)
		}

		// Rewrite Referer header - critical: convert to target server URL
		if referer := req.Header.Get("Referer"); referer != "" {
			if refererURL, err := url.Parse(referer); err == nil {
				// Convert proxy URL to target URL
				refererURL.Scheme = target.Scheme
				refererURL.Host = target.Host
				req.Header.Set("Referer", refererURL.String())
			}
		} else {
			// Smart Referer setting - enhanced anti-detection
			if req.URL.Path == "/" {
				// Homepage request: set reasonable external referrer or leave empty
				if req.Method == "GET" {
					// Simulate direct access or from search engine, no Referer is more natural
					// req.Header.Set("Referer", "https://www.google.com/")
				}
			} else if strings.Contains(req.URL.Path, "/s") && strings.Contains(req.URL.RawQuery, "wd=") {
				// Search request: set homepage as Referer, normal user behavior
				req.Header.Set("Referer", target.Scheme+"://"+target.Host+"/")
			} else if req.URL.RawQuery != "" {
				// Other requests with parameters: set homepage as Referer
				req.Header.Set("Referer", target.Scheme+"://"+target.Host+"/")
			}
		}

		// Force CSS files to return new content instead of 304 cache, so we can process URLs in CSS
		if strings.Contains(req.URL.Path, ".css") {
			req.Header.Set("Cache-Control", "no-cache")
			req.Header.Set("If-None-Match", "")
			req.Header.Set("If-Modified-Since", "")
		}

		// Remove session parameters, don't send to target server - hybrid method: precise removal while keeping other parameter encoding
		if req.URL.RawQuery != "" {
			q := req.URL.Query()
			// Check if we have internal parameters to remove
			if q.Has("session_id") || q.Has("asset_id") || q.Has("target_host") {
				// Only rebuild query string when parameters need to be removed
				// Use manual method to keep original encoding of non-internal parameters
				var newParts []string

				// Split original query string
				parts := strings.Split(req.URL.RawQuery, "&")
				for _, part := range parts {
					if part == "" {
						continue
					}
					// Check if this parameter is an internal parameter we need to remove
					if strings.HasPrefix(part, "session_id=") ||
						strings.HasPrefix(part, "asset_id=") ||
						strings.HasPrefix(part, "target_host=") {
						// Skip internal parameters
						continue
					}
					newParts = append(newParts, part)
				}

				req.URL.RawQuery = strings.Join(newParts, "&")
			}
			// If no session parameters, keep original RawQuery completely unchanged
		}
	}

	// Custom response handler
	proxy.ModifyResponse = func(resp *http.Response) error {
		return c.processResponse(resp, reqCtx)
	}

	// Custom error handler for network connection issues
	proxy.ErrorHandler = func(rw http.ResponseWriter, req *http.Request, err error) {
		logger.L().Error("Web proxy connection error",
			zap.String("sessionId", reqCtx.SessionID),
			zap.String("targetHost", req.Host),
			zap.String("error", err.Error()))

		// Analyze error type and provide appropriate user-friendly message
		var errorTitle, errorReason, errorDetails string

		if strings.Contains(err.Error(), "tls: failed to verify certificate") {
			errorTitle = "SSL Certificate Error"
			errorReason = fmt.Sprintf("The target website (%s) has an invalid SSL certificate.", req.Host)
			errorDetails = "This could be due to: certificate expired/untrusted, hostname mismatch, or self-signed certificate."
		} else if strings.Contains(err.Error(), "no such host") || strings.Contains(err.Error(), "dns") {
			errorTitle = "DNS Resolution Failed"
			errorReason = fmt.Sprintf("Cannot resolve hostname: %s", req.Host)
			errorDetails = "Check if the domain name is correct. The website may be temporarily unavailable."
		} else if strings.Contains(err.Error(), "connection refused") || strings.Contains(err.Error(), "connect: connection refused") {
			errorTitle = "Connection Refused"
			errorReason = fmt.Sprintf("Cannot connect to %s", req.Host)
			errorDetails = "The server may be down, firewall blocking, or port closed."
		} else if strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "deadline exceeded") {
			errorTitle = "Connection Timeout"
			errorReason = fmt.Sprintf("Connection to %s timed out", req.Host)
			errorDetails = "Server taking too long to respond. Network issues or server overload."
		} else {
			errorTitle = "Connection Error"
			errorReason = fmt.Sprintf("Failed to connect to %s", req.Host)
			errorDetails = fmt.Sprintf("Network error: %s", err.Error())
		}

		// Use existing RenderErrorPage function with session info appended
		sessionInfo := fmt.Sprintf("Session ID: %s | Asset ID: %d | Host: %s", reqCtx.SessionID, reqCtx.AssetID, req.Host)
		finalDetails := fmt.Sprintf("%s\n\n%s", errorDetails, sessionInfo)

		errorHTML := RenderErrorPage("connection_error", errorTitle, errorReason, finalDetails)

		rw.Header().Set("Content-Type", "text/html; charset=utf-8")
		rw.WriteHeader(http.StatusBadGateway)
		rw.Write([]byte(errorHTML))
	}

	return proxy, nil
}

// processResponse processes the response and injects necessary modifications
func (c *WebProxyCore) processResponse(resp *http.Response, reqCtx *RequestContext) error {
	contentType := resp.Header.Get("Content-Type")

	// Add CORS headers to resolve cross-origin issues
	resp.Header.Set("Access-Control-Allow-Origin", "*")
	resp.Header.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	resp.Header.Set("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization, X-Requested-With")
	resp.Header.Set("Access-Control-Allow-Credentials", "true")

	if resp.StatusCode >= 200 && resp.StatusCode < 300 && strings.Contains(contentType, "text/html") {
		return processHTMLContent(resp, reqCtx)
	} else if resp.StatusCode >= 200 && resp.StatusCode < 300 && strings.Contains(contentType, "text/css") {
		return processCSSContent(resp, reqCtx)
	}

	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		location := resp.Header.Get("Location")
		if location != "" {

			redirectURL, err := url.Parse(location)
			if err == nil && redirectURL.IsAbs() {
				if reqCtx.Session != nil {
					reqCtx.Session.CurrentHost = redirectURL.Host
				}

				baseDomain := strings.Split(reqCtx.ProxyHost, ":")[0]
				if strings.HasPrefix(baseDomain, "webproxy.") {
					parts := strings.SplitN(baseDomain, ".", 2)
					if len(parts) > 1 {
						baseDomain = parts[1]
					}
				}

				protocol := "http"
				if strings.HasPrefix(reqCtx.OriginalURL, "https://") ||
					strings.Contains(reqCtx.ProxyHost, ":443") {
					protocol = "https"
				}

				newProxyURL := fmt.Sprintf("%s://webproxy.%s%s", protocol, baseDomain, redirectURL.Path)

				q := redirectURL.Query()
				q.Set("session_id", reqCtx.SessionID)
				q.Set("asset_id", strconv.Itoa(reqCtx.AssetID))
				q.Set("target_host", redirectURL.Host)
				newProxyURL += "?" + q.Encode()

				resp.Header.Set("Location", newProxyURL)

			} else {
				processRedirect(resp, reqCtx)
			}
		}
		return nil
	}

	return nil
}

// GetSessionStats returns statistics about active sessions
func (c *WebProxyCore) GetSessionStats() map[string]any {
	totalSessions := 0
	activeSessions := 0
	assetCount := make(map[int]int)

	c.sessions.Range(func(key, value any) bool {
		session := value.(*WebSession)
		totalSessions++
		if session.IsActive {
			activeSessions++
			assetCount[session.AssetID]++
		}
		return true
	})

	return map[string]any{
		"total_sessions":  totalSessions,
		"active_sessions": activeSessions,
		"assets_count":    len(assetCount),
		"asset_breakdown": assetCount,
	}
}
