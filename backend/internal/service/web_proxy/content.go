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

	// Step 1: URL rewriting for external links
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

		// Add session management JavaScript
		sessionJS := fmt.Sprintf(`
		<script>
		(function() {
			var sessionId = '%s';
			var heartbeatInterval;
			
			// Send heartbeat every 30 seconds
			function sendHeartbeat() {
				fetch('/api/oneterm/v1/web_proxy/heartbeat', {
					method: 'POST',
					headers: {'Content-Type': 'application/json'},
					body: JSON.stringify({session_id: sessionId})
				}).catch(function() {});
			}
			
			// Start heartbeat
			heartbeatInterval = setInterval(sendHeartbeat, 30000);
			
			// Handle page unload (tab close, navigation away)
			window.addEventListener('beforeunload', function() {
				clearInterval(heartbeatInterval);
				// Use sendBeacon for reliable cleanup on page unload
				if (navigator.sendBeacon) {
					navigator.sendBeacon('/api/oneterm/v1/web_proxy/cleanup', 
						JSON.stringify({session_id: sessionId}));
				}
			});
			
			// Handle visibility change (tab switching)
			document.addEventListener('visibilitychange', function() {
				if (document.hidden) {
					clearInterval(heartbeatInterval);
				} else {
					heartbeatInterval = setInterval(sendHeartbeat, 30000);
				}
			});
		})();
		</script>`, session.SessionId)

		if strings.Contains(content, "</head>") {
			content = strings.Replace(content, "</head>", watermarkCSS+"</head>", 1)
		} else {
			content = watermarkCSS + content
		}

		if strings.Contains(content, "</body>") {
			content = strings.Replace(content, "</body>", watermarkHTML+sessionJS+"</body>", 1)
		} else {
			content = content + watermarkHTML + sessionJS
		}
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

// RenderSessionExpiredPage renders the page shown when session has expired
func RenderSessionExpiredPage(reason string) string {
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
