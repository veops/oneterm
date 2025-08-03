package web_proxy

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/samber/lo"
)

// RewriteHTMLContent rewrites HTML content to redirect external links through proxy
func RewriteHTMLContent(resp *http.Response, assetID int, scheme, proxyHost string) {
	if resp.Body == nil {
		return
	}

	// Remove Content-Encoding to avoid decoding issues
	resp.Header.Del("Content-Encoding")
	resp.Header.Del("Content-Length")

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
		// Static resources: <img src=""> <script src=""> <link href="">
		{
			`(src\s*=\s*["'])https?://([^/'"]+)(/[^"']*)?["']`,
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

// ProcessHTMLResponse processes HTML response for content rewriting and injection
func ProcessHTMLResponse(resp *http.Response, assetID int, scheme, proxyHost string, session *WebProxySession) {
	if resp.Body == nil {
		return
	}

	// Check if content is compressed
	contentEncoding := resp.Header.Get("Content-Encoding")

	// Remove Content-Encoding to avoid decoding issues
	resp.Header.Del("Content-Encoding")
	resp.Header.Del("Content-Length")

	var body []byte
	var err error

	// Handle compressed content
	if contentEncoding == "gzip" {
		gzipReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			resp.Body.Close()
			return
		}
		defer gzipReader.Close()
		body, err = io.ReadAll(gzipReader)
	} else {
		body, err = io.ReadAll(resp.Body)
	}

	if err != nil {
		resp.Body.Close()
		return
	}
	resp.Body.Close()

	content := string(body)

	// URL rewriting for external links
	baseDomain := lo.Ternary(strings.HasPrefix(proxyHost, "asset-"),
		func() string {
			parts := strings.SplitN(proxyHost, ".", 2)
			return lo.Ternary(len(parts) > 1, parts[1], proxyHost)
		}(),
		proxyHost)

	patterns := []struct {
		pattern string
		rewrite func(matches []string) string
	}{
		{
			`(window\.location(?:\.href)?\s*=\s*["'])https?://([^/'"]+)(/[^"']*)?["']`,
			func(matches []string) string {
				path := lo.Ternary(len(matches) > 3 && matches[3] != "", matches[3], "")
				return fmt.Sprintf(`%s%s://asset-%d.%s%s"`, matches[1], scheme, assetID, baseDomain, path)
			},
		},
		{
			`(action\s*=\s*["'])https?://([^/'"]+)(/[^"']*)?["']`,
			func(matches []string) string {
				path := lo.Ternary(len(matches) > 3 && matches[3] != "", matches[3], "")
				return fmt.Sprintf(`%s%s://asset-%d.%s%s"`, matches[1], scheme, assetID, baseDomain, path)
			},
		},
		{
			`(href\s*=\s*["'])https?://([^/'"]+)(/[^"']*)?["']`,
			func(matches []string) string {
				path := lo.Ternary(len(matches) > 3 && matches[3] != "", matches[3], "")
				return fmt.Sprintf(`%s%s://asset-%d.%s%s"`, matches[1], scheme, assetID, baseDomain, path)
			},
		},
		{
			`(src\s*=\s*["'])https?://([^/'"]+)(/[^"']*)?["']`,
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

	// Step 2: Add watermark if enabled
	if session.WebConfig != nil && session.WebConfig.ProxySettings != nil && session.WebConfig.ProxySettings.WatermarkEnabled {
		watermarkCSS := `
		<style>
		.oneterm-watermark-container {
			position: fixed;
			top: -50%;
			left: -50%;
			width: 200%;
			height: 200%;
			z-index: 1;
			pointer-events: none;
			user-select: none;
			transform: rotate(-45deg);
			overflow: hidden;
		}
		.oneterm-watermark-text {
			position: absolute;
			color: rgba(128,128,128,0.08);
			font-size: 32px;
			font-family: Arial, sans-serif;
			font-weight: bold;
			white-space: nowrap;
		}
		</style>`

		// Generate watermark HTML with multiple OneTerm texts
		var watermarkTexts []string
		for row := 0; row < 30; row++ {
			for col := 0; col < 15; col++ {
				top := row * 100
				left := col * 300
				watermarkTexts = append(watermarkTexts,
					fmt.Sprintf(`<div class="oneterm-watermark-text" style="top: %dpx; left: %dpx;">OneTerm</div>`, top, left))
			}
		}

		watermarkHTML := fmt.Sprintf(`
		<div class="oneterm-watermark-container">
			%s
		</div>`, strings.Join(watermarkTexts, "\n"))

		if strings.Contains(content, "</head>") {
			content = strings.Replace(content, "</head>", watermarkCSS+"</head>", 1)
		} else {
			content = watermarkCSS + content
		}

		if strings.Contains(content, "</body>") {
			content = strings.Replace(content, "</body>", watermarkHTML+"</body>", 1)
		} else {
			content = content + watermarkHTML
		}
	}

	// Add session management JavaScript (always inject)
	sessionJS := fmt.Sprintf(`
		<script>
		(function() {tbeat', {
					method: 'POST',
					headers: {'Content-Type': 'application/json'},
					body: JSON.stringify({session_id: sessionId})
				}).catch(function() {});
			}
			var sessionId = '%s';
			var heartbeatInterval;
			
			// Send heartbeat every 15 seconds
			function sendHeartbeat() {
				fetch('/api/oneterm/v1/web_proxy/hear
			
			// Universal heartbeat mechanism - no complex event handling
			// The server will handle session cleanup based on heartbeat timeout
			heartbeatInterval = setInterval(sendHeartbeat, 15000);
			
			// Send initial heartbeat immediately
			sendHeartbeat();
		})();
		</script>`, session.SessionId)

	// Add JavaScript URL interceptor for dynamic requests (always inject - moved outside watermark condition)

	urlInterceptorJS := fmt.Sprintf(`
	<script>
	(function() {
		var originalHost = '%s';
		var proxyHost = 'asset-%d.%s';
		var proxyScheme = '%s';
		
		function rewriteUrl(url) {
			try {
				// Handle absolute URLs
				if (url.startsWith('http://') || url.startsWith('https://')) {
					var urlObj = new URL(url);
					// Only rewrite external domains, not our proxy domain
					if (urlObj.hostname !== window.location.hostname && 
						urlObj.hostname !== 'localhost' && 
						urlObj.hostname !== '127.0.0.1') {
						// Preserve the original path and query, but use proxy hostname
						var newUrl = proxyScheme + '://' + proxyHost + urlObj.pathname + urlObj.search + urlObj.hash;
						console.log('Rewriting URL:', url, '->', newUrl);
						return newUrl;
					}
				}
				// Handle relative URLs starting with /
				else if (url.startsWith('/')) {
					// Keep relative URLs as-is, they will be relative to current proxy domain
					return url;
				}
				return url;
			} catch (e) {
				console.warn('URL rewrite error:', e, 'for URL:', url);
				return url;
			}
		}
		
		// Download control enforcement
		var hasDownloadPermission = %t;
		
		// Override fetch API with download control
		if (window.fetch) {
			var originalFetch = window.fetch;
			window.fetch = function(input, init) {
				// Handle both string URLs and Request objects
				if (typeof input === 'string') {
					input = rewriteUrl(input);
				} else if (input && typeof input === 'object' && input.url) {
					// Handle Request object
					var rewrittenUrl = rewriteUrl(input.url);
					if (rewrittenUrl !== input.url) {
						input = new Request(rewrittenUrl, input);
					}
				}
				
				// Monitor for potential data export APIs
				if (!hasDownloadPermission) {
					var url = typeof input === 'string' ? input : (input.url || '');
					if (url.includes('/export') || url.includes('/download') || 
						url.includes('/report') || url.includes('/data')) {
						console.warn('Data export API access detected, but download permission denied');
						alert('File download denied: You do not have download permission to access data export APIs');
						return Promise.reject(new Error('Download permission required'));
					}
				}
				
				return originalFetch.call(this, input, init);
			};
		}
		
		// Override XMLHttpRequest with download control
		if (window.XMLHttpRequest) {
			var OriginalXHR = window.XMLHttpRequest;
			window.XMLHttpRequest = function() {
				var xhr = new OriginalXHR();
				var originalOpen = xhr.open;
				xhr.open = function(method, url, async, user, password) {
					if (typeof url === 'string') {
						url = rewriteUrl(url);
						
						// Monitor for potential data export APIs in XHR
						if (!hasDownloadPermission && (
							url.includes('/export') || url.includes('/download') || 
							url.includes('/report') || url.includes('/data'))) {
							console.warn('XHR data export API access detected, download permission denied');
							alert('File download denied: You do not have download permission to access data export APIs');
							throw new Error('Download permission required');
						}
					}
					return originalOpen.call(this, method, url, async, user, password);
				};
				return xhr;
			};
			// Copy static properties
			for (var prop in OriginalXHR) {
				if (OriginalXHR.hasOwnProperty(prop)) {
					window.XMLHttpRequest[prop] = OriginalXHR[prop];
				}
			}
		}
		
		// Override window.open for popup windows
		if (window.open) {
			var originalOpen = window.open;
			window.open = function(url, name, specs) {
				if (typeof url === 'string') {
					url = rewriteUrl(url);
				}
				return originalOpen.call(this, url, name, specs);
			};
		}
		
		// Client-side download monitoring
		if (!hasDownloadPermission) {
			// Monitor blob URL creation (used by XLSX.js and similar libraries)
			if (window.URL && window.URL.createObjectURL) {
				var originalCreateObjectURL = window.URL.createObjectURL;
				window.URL.createObjectURL = function(blob) {
					console.warn('Blob URL creation detected, download permission denied');
					// Block blob URL creation for file downloads
					if (blob && blob.type && (
						blob.type.includes('sheet') || 
						blob.type.includes('excel') ||
						blob.type.includes('pdf') ||
						blob.type.includes('zip') ||
						blob.type.includes('octet-stream')
					)) {
						alert('File download denied: You do not have download permission to create file download links');
						throw new Error('File download not permitted');
					}
					return originalCreateObjectURL.call(this, blob);
				};
			}
			
			// Monitor file download through anchor elements with download attribute
			document.addEventListener('click', function(e) {
				if (e.target && e.target.tagName === 'A' && e.target.hasAttribute('download')) {
					console.warn('Direct file download attempt detected, download permission denied');
					e.preventDefault();
					e.stopPropagation();
					alert('File download denied: You do not have download permission');
					return false;
				}
			}, true);
			
			// Monitor common file export libraries
			setTimeout(function() {
				// Block XLSX library
				if (window.XLSX && window.XLSX.writeFile) {
					window.XLSX.writeFile = function() {
						alert('File export denied: You do not have download permission to export Excel files');
						throw new Error('Excel export not permitted');
					};
				}
				// Block FileSaver.js
				if (window.saveAs) {
					window.saveAs = function() {
						alert('File save denied: You do not have download permission to save files');
						throw new Error('File save not permitted');
					};
				}
			}, 1000);
		}
	})();
	</script>`, session.CurrentHost, assetID, baseDomain, scheme, session.Permissions.FileDownload)

	// Always inject session management and URL interceptor

	if strings.Contains(content, "</body>") {
		content = strings.Replace(content, "</body>", sessionJS+urlInterceptorJS+"</body>", 1)
	} else {
		content = content + sessionJS + urlInterceptorJS
	}

	// Step 3: Record activity if enabled
	if session.WebConfig != nil && session.WebConfig.ProxySettings != nil && session.WebConfig.ProxySettings.RecordingEnabled {
		// Activity recording is handled elsewhere to avoid accessing ctx.Request here
	}

	// Update response
	newBody := bytes.NewReader([]byte(content))
	resp.Body = io.NopCloser(newBody)
	resp.ContentLength = int64(len(content))
	resp.Header.Set("Content-Length", fmt.Sprintf("%d", len(content)))
}

// RenderExternalRedirectPage renders the page shown when external redirect is blocked
func RenderExternalRedirectPage(targetURL string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>External Redirect Blocked - OneTerm</title>
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
}

// RenderErrorPage renders a general error page for web proxy errors
func RenderErrorPage(errorType, title, reason, details string) string {
	var bgColor, iconEmoji string

	switch errorType {
	case "access_denied":
		bgColor = "#ff6b6b 0%, #ee5a52 100%"
		iconEmoji = "🚫"
	case "session_expired":
		bgColor = "#f39c12 0%, #e67e22 100%"
		iconEmoji = "⏰"
	case "connection_error":
		bgColor = "#95a5a6 0%, #7f8c8d 100%"
		iconEmoji = "🔌"
	case "server_error":
		bgColor = "#8e44ad 0%, #9b59b6 100%"
		iconEmoji = "⚠️"
	case "concurrent_limit":
		bgColor = "#e74c3c 0%, #c0392b 100%"
		iconEmoji = "🚦"
	default:
		bgColor = "#34495e 0%, #2c3e50 100%"
		iconEmoji = "❌"
	}

	detailsHtml := ""
	if details != "" {
		detailsHtml = fmt.Sprintf(`
		<div class="info"><strong>Details:</strong></div>
		<div class="details">%s</div>`, details)
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>%s - OneTerm</title>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
        body { 
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: linear-gradient(135deg, %s);
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
            box-shadow: 0 2px 10px rgba(0,0,0,0.1);
            max-width: 600px;
            text-align: center;
        }
        .error-title { color: #e74c3c; font-size: 2em; margin-bottom: 20px; }
        .info { color: #666; margin: 20px 0; text-align: left; }
        .details { 
            background: #f8f9fa; 
            padding: 15px; 
            border-radius: 4px; 
            border-left: 4px solid #e74c3c;
            font-family: monospace;
            font-size: 14px;
            text-align: left;
            white-space: pre-wrap;
            word-break: break-word;
        }
        .action { 
            background: #e8f5e8; 
            padding: 15px; 
            border-radius: 4px; 
            border-left: 4px solid #27ae60;
            margin-top: 20px;
            text-align: center;
        }
        .back-link {
            color: #3498db;
            text-decoration: none;
            font-weight: 500;
            margin: 0 10px;
        }
        .back-link:hover {
            text-decoration: underline;
        }
        .reason {
            background: #fff3cd;
            color: #856404;
            padding: 15px;
            border-radius: 4px;
            border-left: 4px solid #ffc107;
            margin: 20px 0;
            text-align: left;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1 class="error-title">%s %s</h1>
        <div class="reason">%s</div>
        %s
        <div class="action">
            <a href="javascript:history.back()" class="back-link">← Go Back</a>
            <a href="javascript:location.reload()" class="back-link">🔄 Refresh</a>
            <a href="/" class="back-link">🏠 Home</a>
        </div>
    </div>
</body>
</html>`, title, bgColor, iconEmoji, title, reason, detailsHtml)
}

// RenderAccessDeniedPage renders the page shown when access is denied (download, read-only, etc.)
func RenderAccessDeniedPage(reason, details string) string {
	return RenderErrorPage("access_denied", "Access Denied", reason, details)
}

// RenderSessionExpiredPage renders the page shown when session has expired
func RenderSessionExpiredPage(reason string) string {
	return RenderErrorPage("session_expired", "Session Expired", reason, "")
}

// RenderConcurrentLimitPage renders the page when concurrent limit is exceeded
func RenderConcurrentLimitPage(maxConcurrent int) string {
	reason := fmt.Sprintf("Maximum concurrent connections (%d) exceeded", maxConcurrent)
	details := "Please wait for an existing session to end, or contact your administrator to increase the limit."
	return RenderErrorPage("concurrent_limit", "Connection Limit Exceeded", reason, details)
}

// RenderServerErrorPage renders the page for server errors
func RenderServerErrorPage(reason, details string) string {
	return RenderErrorPage("server_error", "Server Error", reason, details)
}

// RenderConnectionErrorPage renders the page for connection errors
func RenderConnectionErrorPage(reason, details string) string {
	return RenderErrorPage("connection_error", "Connection Error", reason, details)
}

// Legacy function - keeping the original style for compatibility
func RenderSessionExpiredPageOld(reason string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
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
}
