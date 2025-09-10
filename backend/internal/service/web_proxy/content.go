package web_proxy

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/veops/oneterm/internal/service"
	"github.com/veops/oneterm/pkg/logger"
	"go.uber.org/zap"
)

var (
	compiledRegex = sync.Once{}
	srcRegex      *regexp.Regexp
	actionRegex   *regexp.Regexp
	hrefRegex     *regexp.Regexp
)

func initRegexPatterns() {
	srcRegex = regexp.MustCompile(`(src\s*=\s*["'])([^"']*?)["']`)
	actionRegex = regexp.MustCompile(`(action\s*=\s*["'])([^"']*?)["']`)
	hrefRegex = regexp.MustCompile(`(href\s*=\s*["'])([^"']*?)["']`)
}

const (
	maxContentSize = 5 * 1024 * 1024
)

func processHTMLContent(resp *http.Response, reqCtx *RequestContext) error {
	if resp.Body == nil {
		return nil
	}

	contentEncoding := resp.Header.Get("Content-Encoding")

	resp.Header.Del("Content-Encoding")
	resp.Header.Del("Content-Length")

	var body []byte
	var err error
	var reader io.Reader

	if contentEncoding == "gzip" {
		gzipReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			resp.Body.Close()
			return err
		}
		defer gzipReader.Close()
		reader = io.LimitReader(gzipReader, maxContentSize)
	} else if contentEncoding == "br" {
		brotliReader := brotli.NewReader(resp.Body)
		reader = io.LimitReader(brotliReader, maxContentSize)
	} else if contentEncoding == "deflate" {
		resp.Header.Set("Content-Encoding", contentEncoding)
		reader = io.LimitReader(resp.Body, maxContentSize)
		body, err = io.ReadAll(reader)
		if err != nil {
			resp.Body.Close()
			return err
		}
		resp.Body.Close()
		newBody := bytes.NewReader(body)
		resp.Body = io.NopCloser(newBody)
		resp.ContentLength = int64(len(body))
		resp.Header.Set("Content-Length", strconv.Itoa(len(body)))
		return nil
	} else {
		reader = io.LimitReader(resp.Body, maxContentSize)
	}

	body, err = io.ReadAll(reader)

	if err != nil {
		resp.Body.Close()
		return err
	}
	resp.Body.Close()

	if len(body) >= maxContentSize {
		logger.L().Warn("HTML content too large, skipping processing",
			zap.String("sessionId", reqCtx.SessionID),
			zap.Int("contentSize", len(body)))
		newBody := bytes.NewReader(body)
		resp.Body = io.NopCloser(newBody)
		resp.ContentLength = int64(len(body))
		resp.Header.Set("Content-Length", strconv.Itoa(len(body)))
		return nil
	}

	content := string(body)

	content = rewriteHTMLLinks(content, reqCtx.AssetID, reqCtx.SessionID)

	sessionJS := generateSimpleJavaScript(reqCtx.AssetID, reqCtx.SessionID)

	watermarkHTML := ""
	if needsWatermark(reqCtx) {
		watermarkHTML = generateWatermarkHTML(reqCtx)
	}

	combinedContent := sessionJS + watermarkHTML
	if strings.Contains(content, "</body>") {
		content = strings.Replace(content, "</body>", combinedContent+"</body>", 1)
	} else if strings.Contains(content, "</html>") {
		content = strings.Replace(content, "</html>", combinedContent+"</html>", 1)
	} else {
		content = content + sessionJS
	}

	newBody := bytes.NewReader([]byte(content))
	resp.Body = io.NopCloser(newBody)
	resp.ContentLength = int64(len(content))
	resp.Header.Set("Content-Length", strconv.Itoa(len(content)))

	return nil
}

func readResponseBody(resp *http.Response) ([]byte, error) {
	if resp.Body == nil {
		return []byte{}, nil
	}
	defer resp.Body.Close()

	contentEncoding := resp.Header.Get("Content-Encoding")
	resp.Header.Del("Content-Encoding")
	resp.Header.Del("Content-Length")

	var reader io.Reader = resp.Body

	if contentEncoding == "gzip" {
		gzipReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, err
		}
		defer gzipReader.Close()
		reader = gzipReader
	}

	limitedReader := io.LimitReader(reader, maxContentSize)
	return io.ReadAll(limitedReader)
}

func processCSSContent(resp *http.Response, reqCtx *RequestContext) error {

	body, err := readResponseBody(resp)
	if err != nil {
		logger.L().Error("Failed to read CSS response body", zap.Error(err))
		return err
	}

	bodyStr := string(body)

	// Rewrite URL references in CSS and add session parameters
	rewrittenCSS := rewriteCSSUrls(bodyStr, reqCtx.AssetID, reqCtx.SessionID)

	// Update response
	newBodyBytes := []byte(rewrittenCSS)
	newBody := bytes.NewReader(newBodyBytes)
	resp.Body = io.NopCloser(newBody)
	resp.ContentLength = int64(len(newBodyBytes))
	resp.Header.Set("Content-Length", strconv.Itoa(len(newBodyBytes)))

	return nil
}

// rewriteCSSUrls rewrites URL references in CSS
func rewriteCSSUrls(cssContent string, assetID int, sessionID string) string {
	result := cssContent

	// Process url() references
	urlStart := 0
	for {
		urlIdx := strings.Index(result[urlStart:], "url(")
		if urlIdx == -1 {
			break
		}

		urlIdx += urlStart
		urlStart = urlIdx + 4

		// Find corresponding closing bracket
		parenIdx := strings.Index(result[urlStart:], ")")
		if parenIdx == -1 {
			continue
		}

		urlContent := strings.TrimSpace(result[urlStart : urlStart+parenIdx])
		// Remove quotes
		urlContent = strings.Trim(urlContent, "\"'")

		// Skip existing session parameters or external URLs
		if strings.Contains(urlContent, "session_id=") || strings.HasPrefix(urlContent, "http") || strings.HasPrefix(urlContent, "//") {
			continue
		}

		// Only process relative paths
		if strings.HasPrefix(urlContent, "/") || strings.HasPrefix(urlContent, ".") {
			// Add session parameters
			sessionParams := fmt.Sprintf("asset_id=%d&session_id=%s", assetID, sessionID)
			var newUrl string
			if strings.Contains(urlContent, "?") {
				newUrl = urlContent + "&" + sessionParams
			} else {
				newUrl = urlContent + "?" + sessionParams
			}

			// Replace original URL
			newUrlPart := `"` + newUrl + `"`
			result = result[:urlStart] + newUrlPart + result[urlStart+parenIdx:]

			// Adjust next search position
			urlStart += len(newUrlPart)
		}
	}

	return result
}

// generateSimpleJavaScript generates enhanced JavaScript proxy code
func generateSimpleJavaScript(assetID int, sessionID string) string {
	return fmt.Sprintf(`
<script type="text/javascript">
(function() {
	var ASSET_ID = %d;
	var SESSION_ID = '%s';
	
	// Add session parameters to URLs with smart domain rewriting - avoid breaking original encoding
	function addSessionParams(url) {
		if (!url || url.startsWith('javascript:') || url.startsWith('#') || url.includes('session_id=')) {
			return url;
		}
		
		// Use simple string operations, avoid URL object re-encoding
		try {
			// Handle domain replacement for absolute URLs - only rewrite URLs that need cross-domain proxying
			if (url.startsWith('http://') || url.startsWith('https://')) {
				// Extract domain from URL
				var protocolEnd = url.indexOf('://') + 3;
				var pathStart = url.indexOf('/', protocolEnd);
				if (pathStart === -1) pathStart = url.length;
				
				var urlHost = url.substring(protocolEnd, pathStart);
				var currentHost = window.location.host;
				
				// Only rewrite specific domains to avoid breaking domain-sensitive sites like Baidu
				var needsRewrite = false;
				
				// Check if it's an external API domain that needs proxying (like veops.cn)
				if (urlHost !== currentHost && 
				    (urlHost.includes('veops.cn') || 
				     urlHost.includes('api.') || 
				     url.includes('/api/') ||
				     url.includes('/console/'))) {
					needsRewrite = true;
				}
				
				if (needsRewrite) {
					// Extract path and query parameters part
					if (pathStart === url.length) url += '/';
					// Replace with proxy domain, keep path unchanged
					url = window.location.protocol + '//' + window.location.host + url.substring(pathStart);
				}
			}
			
			var separator = url.includes('?') ? '&' : '?';
			var sessionParams = 'asset_id=' + ASSET_ID + '&session_id=' + SESSION_ID;
			return url + separator + sessionParams;
		} catch (e) {
			console.log('[WebProxy] URL processing error:', e);
			return url;
		}
	}
	
	// Intercept HTMLFormElement.prototype.submit (programmatic submission)
	if (window.HTMLFormElement) {
		var originalSubmit = HTMLFormElement.prototype.submit;
		HTMLFormElement.prototype.submit = function() {
			try {
				// Add session parameters to action
				if (this.action && !this.action.includes('session_id=')) {
					this.action = addSessionParams(this.action);
				}
				
				// Add hidden fields as backup (especially for POST forms)
				if (!this.querySelector('input[name="asset_id"]')) {
					var assetInput = document.createElement('input');
					assetInput.type = 'hidden';
					assetInput.name = 'asset_id';
					assetInput.value = ASSET_ID;
					this.appendChild(assetInput);
				}
				if (!this.querySelector('input[name="session_id"]')) {
					var sessionInput = document.createElement('input');
					sessionInput.type = 'hidden';
					sessionInput.name = 'session_id';
					sessionInput.value = SESSION_ID;
					this.appendChild(sessionInput);
				}
			} catch (err) {
				console.log('[WebProxy] Form submit preparation error:', err);
			}
			return originalSubmit.call(this);
		};
	}
	
	// Intercept form event submission - simplified version, reduce interference
	document.addEventListener('submit', function(e) {
		try {
			var form = e.target;
			if (form.tagName === 'FORM' && form.action && !form.action.includes('session_id=')) {
				// Only process forms whose action doesn't already contain session parameters
				form.action = addSessionParams(form.action);
				
				// Add hidden fields as backup
				if (!form.querySelector('input[name="asset_id"]')) {
					var assetInput = document.createElement('input');
					assetInput.type = 'hidden';
					assetInput.name = 'asset_id';
					assetInput.value = ASSET_ID;
					form.appendChild(assetInput);
				}
				if (!form.querySelector('input[name="session_id"]')) {
					var sessionInput = document.createElement('input');
					sessionInput.type = 'hidden';
					sessionInput.name = 'session_id';
					sessionInput.value = SESSION_ID;
					form.appendChild(sessionInput);
				}
			}
		} catch (err) {
			console.log('[WebProxy] Form submit error:', err);
		}
	}, true);
	
	// Intercept fetch requests
	if (window.fetch) {
		var originalFetch = window.fetch;
		window.fetch = function(input, init) {
			if (typeof input === 'string') {
				input = addSessionParams(input);
			} else if (input && input.url) {
				input = new Request(addSessionParams(input.url), input);
			}
			return originalFetch.call(this, input, init);
		};
	}
	
	// Intercept XMLHttpRequest
	if (window.XMLHttpRequest) {
		var OriginalXHR = window.XMLHttpRequest;
		window.XMLHttpRequest = function() {
			var xhr = new OriginalXHR();
			var originalOpen = xhr.open;
			xhr.open = function(method, url, async, user, password) {
				url = addSessionParams(url);
				return originalOpen.call(this, method, url, async, user, password);
			};
			return xhr;
		};
		window.XMLHttpRequest.prototype = OriginalXHR.prototype;
	}
	
	// Intercept navigator.sendBeacon
	if (window.navigator && window.navigator.sendBeacon) {
		var originalSendBeacon = window.navigator.sendBeacon;
		window.navigator.sendBeacon = function(url, data) {
			url = addSessionParams(url);
			return originalSendBeacon.call(this, url, data);
		};
	}
	
	// Intercept Image object src setting
	if (window.Image) {
		var OriginalImage = window.Image;
		window.Image = function(width, height) {
			var img = new OriginalImage(width, height);
			var originalSrcDescriptor = Object.getOwnPropertyDescriptor(HTMLImageElement.prototype, 'src') || 
			                            Object.getOwnPropertyDescriptor(Image.prototype, 'src') ||
			                            { set: function(v) { this.setAttribute('src', v); }, get: function() { return this.getAttribute('src'); } };
			
			Object.defineProperty(img, 'src', {
				set: function(value) {
					value = addSessionParams(value);
					originalSrcDescriptor.set.call(this, value);
				},
				get: originalSrcDescriptor.get,
				enumerable: true,
				configurable: true
			});
			return img;
		};
		// Maintain prototype chain
		window.Image.prototype = OriginalImage.prototype;
	}
	
	// Intercept dynamically created img elements
	if (window.document && window.document.createElement) {
		var originalCreateElement = document.createElement;
		document.createElement = function(tagName) {
			var element = originalCreateElement.call(this, tagName);
			if (tagName.toLowerCase() === 'img') {
				var originalSrcDescriptor = Object.getOwnPropertyDescriptor(HTMLImageElement.prototype, 'src') ||
				                            { set: function(v) { this.setAttribute('src', v); }, get: function() { return this.getAttribute('src'); } };
				
				Object.defineProperty(element, 'src', {
					set: function(value) {
						if (value) {
							value = addSessionParams(value);
						}
						originalSrcDescriptor.set.call(this, value);
					},
					get: originalSrcDescriptor.get,
					enumerable: true,
					configurable: true
				});
			}
			return element;
		};
	}
	
	// Intercept window.location related methods
	if (window.location) {
		var originalAssign = window.location.assign;
		var originalReplace = window.location.replace;
		
		if (originalAssign) {
			window.location.assign = function(url) {
				url = addSessionParams(url);
				return originalAssign.call(this, url);
			};
		}
		
		if (originalReplace) {
			window.location.replace = function(url) {
				url = addSessionParams(url);
				return originalReplace.call(this, url);
			};
		}
	}
	
	// Rewrite links in the page and add session parameters
	function rewriteLinks() {
		try {
			// Rewrite href attributes of all anchor tags
			var links = document.querySelectorAll('a[href]');
			links.forEach(function(link) {
				var href = link.getAttribute('href');
				if (href && !href.includes('session_id=') && !href.startsWith('javascript:') && !href.startsWith('#')) {
					try {
						var url = new URL(href, window.location.href);
						// Relax conditions: not only check hostname, but also check relative paths
						if (url.hostname === window.location.hostname || href.startsWith('/') || !href.includes('://')) {
							var newHref = addSessionParams(href);
							if (newHref !== href) {
								link.setAttribute('href', newHref);
								console.log('[WebProxy] Updated link href:', href, ' -> ', newHref);
							}
						}
					} catch (e) {
						// Handle relative links and complex URLs
						if (!href.startsWith('mailto:') && !href.startsWith('tel:')) {
							var newHref = addSessionParams(href);
							if (newHref !== href) {
								link.setAttribute('href', newHref);
								console.log('[WebProxy] Updated relative link:', href, ' -> ', newHref);
							}
						}
					}
				}
			});
			
			// Rewrite action attributes of all forms (including forms without action)
			var forms = document.querySelectorAll('form');
			forms.forEach(function(form) {
				var action = form.getAttribute('action') || '';
				if (!action.includes('session_id=') && !action.startsWith('javascript:')) {
					var newAction = action || window.location.pathname;
					newAction = addSessionParams(newAction);
					form.setAttribute('action', newAction);
					console.log('[WebProxy] Preemptively set form action:', action, ' -> ', newAction);
				}
				
				// Ensure form has hidden session fields
				if (!form.querySelector('input[name="asset_id"]')) {
					var assetInput = document.createElement('input');
					assetInput.type = 'hidden';
					assetInput.name = 'asset_id';
					assetInput.value = ASSET_ID;
					form.appendChild(assetInput);
				}
				if (!form.querySelector('input[name="session_id"]')) {
					var sessionInput = document.createElement('input');
					sessionInput.type = 'hidden';
					sessionInput.name = 'session_id';
					sessionInput.value = SESSION_ID;
					form.appendChild(sessionInput);
				}
			});
		} catch (err) {
			console.log('[WebProxy] Link rewrite error:', err);
		}
	}
	
	// Intercept setAttribute method, prevent losing session parameters when dynamically setting href
	if (window.Element) {
		var originalSetAttribute = Element.prototype.setAttribute;
		Element.prototype.setAttribute = function(name, value) {
			if (name === 'href' && this.tagName === 'A' && value && !value.includes('session_id=')) {
				// If setting href attribute and no session parameters, add them
				try {
					var isRelativeOrSameDomain = !value.startsWith('http') || 
						(value.startsWith('http') && new URL(value, window.location.href).hostname === window.location.hostname);
					if (isRelativeOrSameDomain && !value.startsWith('javascript:') && !value.startsWith('#') && !value.startsWith('mailto:')) {
						value = addSessionParams(value);
						console.log('[WebProxy] setAttribute href intercepted and rewritten:', arguments[1], ' -> ', value);
					}
				} catch (e) {
				}
			} else if (name === 'action' && this.tagName === 'FORM' && value && !value.includes('session_id=')) {
				// If setting form action and no session parameters, add them
				try {
					var isRelativeOrSameDomain = !value.startsWith('http') || 
						(value.startsWith('http') && new URL(value, window.location.href).hostname === window.location.hostname);
					if (isRelativeOrSameDomain && !value.startsWith('javascript:')) {
						value = addSessionParams(value);
						console.log('[WebProxy] setAttribute action intercepted and rewritten:', arguments[1], ' -> ', value);
					}
				} catch (e) {
				}
			}
			return originalSetAttribute.call(this, name, value);
		};
	}
	
	// MutationObserver listens for DOM changes and handles dynamically loaded content
	if (window.MutationObserver) {
		var observer = new MutationObserver(function(mutations) {
			var shouldRewrite = false;
			mutations.forEach(function(mutation) {
				if (mutation.type === 'childList' && mutation.addedNodes.length > 0) {
					mutation.addedNodes.forEach(function(node) {
						if (node.nodeType === 1) { // Element node
							// Check if newly added nodes contain elements that need rewriting
							if (node.tagName === 'A' || node.tagName === 'FORM' || 
								node.querySelector && node.querySelector('a[href], form[action]')) {
								shouldRewrite = true;
							}
						}
					});
				}
				// Listen for attribute changes
				else if (mutation.type === 'attributes') {
					var target = mutation.target;
					if ((mutation.attributeName === 'href' && target.tagName === 'A') ||
						(mutation.attributeName === 'action' && target.tagName === 'FORM')) {
						shouldRewrite = true;
					}
				}
			});
			
			if (shouldRewrite) {
				// Delayed execution to avoid frequent rewrites
				setTimeout(rewriteLinks, 50);
			}
		});
		
		// Start listening for DOM changes, including attribute changes
		if (document.body) {
			observer.observe(document.body, { 
				childList: true, 
				subtree: true, 
				attributes: true,
				attributeFilter: ['href', 'action']
			});
		} else {
			// If body hasn't loaded yet, wait for it to complete
			document.addEventListener('DOMContentLoaded', function() {
				observer.observe(document.body, { 
					childList: true, 
					subtree: true, 
					attributes: true,
					attributeFilter: ['href', 'action']
				});
			});
		}
	}
	
	// Initialize link rewriting after page load completion
	function initializeLinkRewriting() {
		rewriteLinks();
		// Delay execution again to ensure all dynamic content is loaded
		setTimeout(rewriteLinks, 1000);
	}
	
	// Multiple timing points to ensure link rewriting execution
	if (document.readyState === 'loading') {
		document.addEventListener('DOMContentLoaded', initializeLinkRewriting);
	} else {
		// DOM has already finished loading
		setTimeout(initializeLinkRewriting, 100);
	}
	
	// Execute again after page fully loads
	window.addEventListener('load', function() {
		setTimeout(rewriteLinks, 500);
	});
	
	// Global click event interceptor - last line of defense
	document.addEventListener('click', function(event) {
		try {
			var target = event.target;
			// Find nearest link element
			while (target && target !== document) {
				if (target.tagName === 'A' && target.href) {
					var href = target.href;
					// Check if session parameters need to be added
					if (!href.includes('session_id=') && !href.includes('asset_id=') && 
						!href.startsWith('javascript:') && !href.startsWith('#') && 
						!href.startsWith('mailto:') && !href.startsWith('tel:')) {
						
						// Check if it's same domain or relative link
						try {
							var url = new URL(href);
							if (url.hostname === window.location.hostname) {
								var newHref = addSessionParams(href);
								target.href = newHref;
								console.log('[WebProxy] Click interceptor updated link:', href, ' -> ', newHref);
							}
						} catch (e) {
							// Case of relative links
							var newHref = addSessionParams(href);
							target.href = newHref;
							console.log('[WebProxy] Click interceptor updated relative link:', href, ' -> ', newHref);
						}
					}
					break;
				}
				target = target.parentNode;
			}
		} catch (err) {
			console.log('[WebProxy] Click interceptor error:', err);
		}
	}, true); // Use capture phase
	
	// Periodic rewrite mechanism - more aggressively capture dynamically generated links
	setInterval(function() {
		try {
			rewriteLinks();
		} catch (e) {
			console.log('[WebProxy] Periodic rewrite error:', e);
		}
	}, 2000);
	
	// Rewrite links on page scroll (may trigger lazy loading)
	var scrollTimeout;
	window.addEventListener('scroll', function() {
		clearTimeout(scrollTimeout);
		scrollTimeout = setTimeout(function() {
			try {
				rewriteLinks();
			} catch (e) {
				console.log('[WebProxy] Scroll rewrite error:', e);
			}
		}, 500);
	});
	
	// Intercept event listener addition, especially click events
	if (window.EventTarget && EventTarget.prototype.addEventListener) {
		var originalAddEventListener = EventTarget.prototype.addEventListener;
		EventTarget.prototype.addEventListener = function(type, listener, options) {
			if (type === 'click' && this.tagName === 'A' && this.href && !this.href.includes('session_id=')) {
				// Check and fix href when adding click listener
				try {
					var isRelativeOrSameDomain = !this.href.startsWith('http') || 
						(this.href.startsWith('http') && new URL(this.href, window.location.href).hostname === window.location.hostname);
					if (isRelativeOrSameDomain && !this.href.startsWith('javascript:') && !this.href.startsWith('#')) {
						this.href = addSessionParams(this.href);
						console.log('[WebProxy] Click listener href fixed:', this.href);
					}
				} catch (e) {
				}
			}
			return originalAddEventListener.call(this, type, listener, options);
		};
	}
	
	// Heartbeat mechanism
	setInterval(function() {
		fetch('/api/oneterm/v1/web_proxy/heartbeat', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ session_id: SESSION_ID })
		}).catch(function() {});
	}, 15000);
})();
</script>`, assetID, sessionID)
}

// rewriteHTMLLinks rewrites links in HTML and adds session parameters
func rewriteHTMLLinks(htmlContent string, assetID int, sessionID string) string {
	result := htmlContent

	compiledRegex.Do(initRegexPatterns)

	result = srcRegex.ReplaceAllStringFunc(result, func(match string) string {
		matches := srcRegex.FindStringSubmatch(match)
		if len(matches) < 3 || len(matches[1]) == 0 || len(matches[2]) == 0 {
			return match
		}
		urlStr := matches[2]
		if strings.Contains(urlStr, "session_id=") || strings.HasPrefix(urlStr, "javascript:") || urlStr == "" {
			return match
		}
		sessionParams := fmt.Sprintf("asset_id=%d&session_id=%s", assetID, sessionID)
		separator := "?"
		if strings.Contains(urlStr, "?") {
			separator = "&"
		}
		return matches[1] + urlStr + separator + sessionParams + `"`
	})

	result = actionRegex.ReplaceAllStringFunc(result, func(match string) string {
		matches := actionRegex.FindStringSubmatch(match)
		if len(matches) < 3 || len(matches[1]) == 0 || len(matches[2]) == 0 {
			return match
		}
		urlStr := matches[2]
		if strings.Contains(urlStr, "session_id=") || strings.HasPrefix(urlStr, "javascript:") || urlStr == "" {
			return match
		}
		sessionParams := fmt.Sprintf("asset_id=%d&session_id=%s", assetID, sessionID)
		separator := "?"
		if strings.Contains(urlStr, "?") {
			separator = "&"
		}
		return matches[1] + urlStr + separator + sessionParams + `"`
	})

	result = hrefRegex.ReplaceAllStringFunc(result, func(match string) string {
		matches := hrefRegex.FindStringSubmatch(match)
		if len(matches) < 3 || len(matches[1]) == 0 || len(matches[2]) == 0 {
			return match
		}
		urlStr := matches[2]
		if strings.Contains(urlStr, "session_id=") || strings.HasPrefix(urlStr, "javascript:") ||
			strings.HasPrefix(urlStr, "#") || strings.HasPrefix(urlStr, "mailto:") ||
			strings.HasPrefix(urlStr, "tel:") || urlStr == "" {
			return match
		}

		if strings.HasPrefix(urlStr, "http://") || strings.HasPrefix(urlStr, "https://") {
			return match
		}

		sessionParams := fmt.Sprintf("asset_id=%d&session_id=%s", assetID, sessionID)
		separator := "?"
		if strings.Contains(urlStr, "?") {
			separator = "&"
		}
		return matches[1] + urlStr + separator + sessionParams + `"`
	})

	return result
}

// processRedirect handles redirect responses
func processRedirect(resp *http.Response, reqCtx *RequestContext) error {
	location := resp.Header.Get("Location")
	if location == "" {
		return nil
	}

	if strings.HasPrefix(location, "http://") || strings.HasPrefix(location, "https://") {
		redirectURL, err := url.Parse(location)
		if err != nil {
			return nil
		}

		if reqCtx.Session != nil {
			reqCtx.Session.CurrentHost = redirectURL.Host
		}

		baseDomain := reqCtx.ProxyHost
		if baseDomain == "" {
			return nil
		}

		if colonIdx := strings.LastIndex(baseDomain, ":"); colonIdx != -1 {
			baseDomain = baseDomain[:colonIdx]
		}

		if baseDomainWithoutPrefix, found := strings.CutPrefix(baseDomain, "webproxy."); found {
			baseDomain = baseDomainWithoutPrefix
		}

		protocol := "http"
		if strings.HasPrefix(reqCtx.OriginalURL, "https://") ||
			strings.Contains(reqCtx.ProxyHost, ":443") {
			protocol = "https"
		}

		newProxyURL := fmt.Sprintf("%s://webproxy.%s%s", protocol, baseDomain, redirectURL.Path)
		if redirectURL.RawQuery != "" {
			newProxyURL += "?" + redirectURL.RawQuery + "&asset_id=" + fmt.Sprintf("%d", reqCtx.AssetID) + "&session_id=" + reqCtx.SessionID + "&target_host=" + redirectURL.Host
		} else {
			newProxyURL += "?asset_id=" + fmt.Sprintf("%d", reqCtx.AssetID) + "&session_id=" + reqCtx.SessionID + "&target_host=" + redirectURL.Host
		}
		resp.Header.Set("Location", newProxyURL)

	} else {
		if !strings.Contains(location, "session_id=") {
			separator := "?"
			if strings.Contains(location, "?") {
				separator = "&"
			}
			newLocation := location + separator + fmt.Sprintf("asset_id=%d&session_id=%s", reqCtx.AssetID, reqCtx.SessionID)
			resp.Header.Set("Location", newLocation)
		}
	}

	return nil
}

// needsWatermark checks if watermark should be added based on asset configuration
func needsWatermark(reqCtx *RequestContext) bool {
	if reqCtx.Session == nil {
		return false
	}

	assetService := service.NewAssetService()
	asset, err := assetService.GetById(context.TODO(), reqCtx.AssetID)
	if err != nil {
		return false
	}

	if asset.WebConfig != nil &&
		asset.WebConfig.ProxySettings != nil {
		watermarkEnabled := asset.WebConfig.ProxySettings.WatermarkEnabled
		return watermarkEnabled
	}

	return false
}

// generateWatermarkHTML generates HTML for watermark overlay
func generateWatermarkHTML(reqCtx *RequestContext) string {
	currentUser := ""
	_ = time.Now().Format("2006-01-02 15:04:05")
	assetName := ""

	if reqCtx.Session != nil {
		assetService := service.NewAssetService()
		asset, err := assetService.GetById(context.TODO(), reqCtx.AssetID)
		if err == nil {
			assetName = asset.Name
		}
	} else {
		assetName = fmt.Sprintf("Asset-%d", reqCtx.AssetID)
	}

	watermarkText := fmt.Sprintf("OneTerm %s", assetName)
	if currentUser != "" {
		watermarkText = fmt.Sprintf("OneTerm %s", assetName)
	}
	
	var watermarkLines []string
	for row := 0; row < 20; row++ {
		var line string
		for col := 0; col < 10; col++ {
			line += watermarkText + "        "
		}
		watermarkLines = append(watermarkLines, line)
	}
	fullWatermarkText := strings.Join(watermarkLines, "\n")

	return fmt.Sprintf(`
<style>
.oneterm-watermark {
    position: fixed;
    top: 0;
    left: 0;
    width: 200vw;
    height: 200vh;
    z-index: 999999;
    pointer-events: none;
    user-select: none;
    overflow: hidden;
    transform: rotate(-30deg) translate(-50vh, -50vw);
    font-family: Arial, sans-serif;
    font-size: 18px;
    font-weight: normal;
    color: rgba(200, 200, 200, 0.3);
    white-space: pre;
    line-height: 100px;
    letter-spacing: 2px;
}
@media print {
    .oneterm-watermark { display: none; }
}
</style>
<div class="oneterm-watermark">%s</div>
`, fullWatermarkText)
}
