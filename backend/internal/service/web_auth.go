package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"go.uber.org/zap"

	"github.com/samber/lo"
	"github.com/veops/oneterm/pkg/logger"
)

// WebAuthService handles Web authentication
type WebAuthService struct {
	strategies []WebAuthStrategy
}

// WebAuthStrategy defines the interface for Web authentication strategies
type WebAuthStrategy interface {
	Name() string
	Priority() int
	CanHandle(ctx context.Context, siteInfo *WebSiteInfo) bool
	Authenticate(ctx context.Context, credentials *WebCredentials, siteInfo *WebSiteInfo) (*WebAuthResult, error)
}

// WebSiteInfo contains information about the target Web site
type WebSiteInfo struct {
	URL         string
	HTMLContent string
	Headers     http.Header
	StatusCode  int
	LoginForms  []WebLoginForm
}

// WebLoginForm represents a login form found on the page
type WebLoginForm struct {
	Action           string         `json:"action"`
	Method           string         `json:"method"`
	UsernameField    WebFormField   `json:"username_field"`
	PasswordField    WebFormField   `json:"password_field"`
	SubmitButton     WebFormField   `json:"submit_button"`
	AdditionalFields []WebFormField `json:"additional_fields"`
	CSRFToken        string         `json:"csrf_token"`
}

// WebFormField represents a form field
type WebFormField struct {
	Name        string `json:"name"`
	ID          string `json:"id"`
	Type        string `json:"type"`
	Value       string `json:"value"`
	Selector    string `json:"selector"`
	Placeholder string `json:"placeholder"`
}

// WebCredentials contains authentication credentials
type WebCredentials struct {
	Username string
	Password string
}

// WebAuthResult contains authentication result
type WebAuthResult struct {
	Success     bool
	Message     string
	Cookies     []*http.Cookie
	RedirectURL string
	SessionData map[string]interface{}
}

// NewWebAuthService creates a new Web authentication service
func NewWebAuthService() *WebAuthService {
	service := &WebAuthService{
		strategies: []WebAuthStrategy{
			&HTTPBasicAuthStrategy{},
			&SmartFormAuthStrategy{},
			&APILoginAuthStrategy{},
		},
	}
	return service
}

// AnalyzeSite analyzes a Web site for authentication methods
func (s *WebAuthService) AnalyzeSite(ctx context.Context, targetURL string) (*WebSiteInfo, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Don't follow redirects during analysis
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Get(targetURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch site: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	siteInfo := &WebSiteInfo{
		URL:         targetURL,
		HTMLContent: string(body),
		Headers:     resp.Header,
		StatusCode:  resp.StatusCode,
	}

	// Analyze HTML for login forms
	if strings.Contains(resp.Header.Get("Content-Type"), "text/html") {
		forms, err := s.analyzeLoginForms(string(body))
		if err != nil {
			logger.L().Warn("Failed to analyze login forms", zap.Error(err))
		} else {
			siteInfo.LoginForms = forms
		}
	}

	return siteInfo, nil
}

// SelectBestStrategy selects the best authentication strategy for a site
func (s *WebAuthService) SelectBestStrategy(ctx context.Context, siteInfo *WebSiteInfo) WebAuthStrategy {
	var bestStrategy WebAuthStrategy
	highestPriority := -1

	for _, strategy := range s.strategies {
		if strategy.CanHandle(ctx, siteInfo) && strategy.Priority() > highestPriority {
			bestStrategy = strategy
			highestPriority = strategy.Priority()
		}
	}

	return bestStrategy
}

// Authenticate performs authentication using the best available strategy
func (s *WebAuthService) Authenticate(ctx context.Context, credentials *WebCredentials, siteInfo *WebSiteInfo) (*WebAuthResult, error) {
	strategy := s.SelectBestStrategy(ctx, siteInfo)
	if strategy == nil {
		return &WebAuthResult{
			Success: false,
			Message: "No suitable authentication strategy found",
		}, nil
	}

	return strategy.Authenticate(ctx, credentials, siteInfo)
}

// AuthenticateWithRetry performs authentication with automatic account retry
func (s *WebAuthService) AuthenticateWithRetry(ctx context.Context, accounts []WebCredentials, siteInfo *WebSiteInfo) (*WebAuthResult, error) {
	if len(accounts) == 0 {
		return &WebAuthResult{
			Success: false,
			Message: "No accounts available for authentication",
		}, nil
	}

	strategy := s.SelectBestStrategy(ctx, siteInfo)
	if strategy == nil {
		return &WebAuthResult{
			Success: false,
			Message: "No suitable authentication strategy found",
		}, nil
	}

	var lastError error
	var lastResult *WebAuthResult

	// 尝试每个账号，直到成功
	for i, credentials := range accounts {
		logger.L().Info("Attempting authentication",
			zap.String("strategy", strategy.Name()),
			zap.String("username", credentials.Username),
			zap.Int("attempt", i+1),
			zap.Int("total_accounts", len(accounts)))

		result, err := strategy.Authenticate(ctx, &credentials, siteInfo)
		if err != nil {
			lastError = err
			logger.L().Warn("Authentication error",
				zap.String("username", credentials.Username),
				zap.Error(err))
			continue
		}

		lastResult = result
		if result.Success {
			logger.L().Info("Authentication successful",
				zap.String("username", credentials.Username),
				zap.Int("attempt", i+1))
			return result, nil
		}

		logger.L().Warn("Authentication failed",
			zap.String("username", credentials.Username),
			zap.String("reason", result.Message))
	}

	// 所有账号都失败了
	if lastError != nil {
		return nil, fmt.Errorf("all authentication attempts failed, last error: %w", lastError)
	}

	if lastResult != nil {
		return lastResult, nil
	}

	return &WebAuthResult{
		Success: false,
		Message: "All configured accounts failed to authenticate",
	}, nil
}

// analyzeLoginForms analyzes HTML content for login forms
func (s *WebAuthService) analyzeLoginForms(htmlContent string) ([]WebLoginForm, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return nil, err
	}

	var forms []WebLoginForm

	doc.Find("form").Each(func(i int, formSel *goquery.Selection) {
		form := WebLoginForm{
			Method: strings.ToUpper(formSel.AttrOr("method", "GET")),
			Action: formSel.AttrOr("action", ""),
		}

		// Find username field
		formSel.Find("input").Each(func(j int, inputSel *goquery.Selection) {
			inputType := strings.ToLower(inputSel.AttrOr("type", "text"))
			inputName := inputSel.AttrOr("name", "")
			inputID := inputSel.AttrOr("id", "")
			placeholder := inputSel.AttrOr("placeholder", "")

			field := WebFormField{
				Name:        inputName,
				ID:          inputID,
				Type:        inputType,
				Selector:    s.generateSelector(inputSel),
				Placeholder: placeholder,
			}

			// Identify field type based on various indicators
			if s.isUsernameField(inputType, inputName, inputID, placeholder) && form.UsernameField.Name == "" {
				form.UsernameField = field
			} else if inputType == "password" && form.PasswordField.Name == "" {
				form.PasswordField = field
			}
		})

		// Find submit button
		formSel.Find("button, input[type=submit]").Each(func(j int, btnSel *goquery.Selection) {
			if form.SubmitButton.Name == "" {
				form.SubmitButton = WebFormField{
					Name:     btnSel.AttrOr("name", ""),
					ID:       btnSel.AttrOr("id", ""),
					Type:     btnSel.AttrOr("type", "submit"),
					Selector: s.generateSelector(btnSel),
				}
			}
		})

		// Only include forms that have both username and password fields
		if form.UsernameField.Name != "" && form.PasswordField.Name != "" {
			forms = append(forms, form)
		}
	})

	return forms, nil
}

// isUsernameField determines if a field is likely a username field
func (s *WebAuthService) isUsernameField(inputType, name, id, placeholder string) bool {
	if inputType == "password" {
		return false
	}

	keywords := []string{"user", "login", "email", "account", "name"}
	text := strings.ToLower(name + id + placeholder)

	return lo.SomeBy(keywords, func(keyword string) bool {
		return strings.Contains(text, keyword)
	})
}

// generateSelector generates a CSS selector for an element
func (s *WebAuthService) generateSelector(sel *goquery.Selection) string {
	if id := sel.AttrOr("id", ""); id != "" {
		return "#" + id
	}
	if name := sel.AttrOr("name", ""); name != "" {
		return fmt.Sprintf(`[name="%s"]`, name)
	}
	if class := sel.AttrOr("class", ""); class != "" {
		classes := strings.Split(class, " ")
		if len(classes) > 0 {
			return "." + strings.Join(classes, ".")
		}
	}
	return sel.Get(0).Data // fallback to tag name
}

// HTTPBasicAuthStrategy implements HTTP Basic Authentication
type HTTPBasicAuthStrategy struct{}

func (s *HTTPBasicAuthStrategy) Name() string  { return "http_basic" }
func (s *HTTPBasicAuthStrategy) Priority() int { return 10 }

func (s *HTTPBasicAuthStrategy) CanHandle(ctx context.Context, siteInfo *WebSiteInfo) bool {
	return siteInfo.StatusCode == 401 &&
		strings.Contains(siteInfo.Headers.Get("WWW-Authenticate"), "Basic")
}

func (s *HTTPBasicAuthStrategy) Authenticate(ctx context.Context, credentials *WebCredentials, siteInfo *WebSiteInfo) (*WebAuthResult, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	req, err := http.NewRequestWithContext(ctx, "GET", siteInfo.URL, nil)
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(credentials.Username, credentials.Password)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	success := resp.StatusCode != 401
	return &WebAuthResult{
		Success: success,
		Message: fmt.Sprintf("HTTP Basic auth %s", map[bool]string{true: "succeeded", false: "failed"}[success]),
		Cookies: resp.Cookies(),
	}, nil
}

// SmartFormAuthStrategy implements intelligent form-based authentication
type SmartFormAuthStrategy struct{}

func (s *SmartFormAuthStrategy) Name() string  { return "smart_form" }
func (s *SmartFormAuthStrategy) Priority() int { return 5 }

func (s *SmartFormAuthStrategy) CanHandle(ctx context.Context, siteInfo *WebSiteInfo) bool {
	return len(siteInfo.LoginForms) > 0
}

func (s *SmartFormAuthStrategy) Authenticate(ctx context.Context, credentials *WebCredentials, siteInfo *WebSiteInfo) (*WebAuthResult, error) {
	if len(siteInfo.LoginForms) == 0 {
		return nil, fmt.Errorf("no login forms found")
	}

	form := siteInfo.LoginForms[0] // Use the first form found

	// Prepare form data
	formData := url.Values{}
	formData.Set(form.UsernameField.Name, credentials.Username)
	formData.Set(form.PasswordField.Name, credentials.Password)

	// Add submit button if it has a name
	if form.SubmitButton.Name != "" {
		formData.Set(form.SubmitButton.Name, "")
	}

	// Determine the target URL
	actionURL := form.Action
	if actionURL == "" || strings.HasPrefix(actionURL, "/") {
		baseURL, _ := url.Parse(siteInfo.URL)
		if actionURL == "" {
			actionURL = siteInfo.URL
		} else {
			actionURL = baseURL.Scheme + "://" + baseURL.Host + actionURL
		}
	}

	// Create HTTP client
	client := &http.Client{Timeout: 30 * time.Second}

	// Submit the form
	req, err := http.NewRequestWithContext(ctx, form.Method, actionURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "OneTerm-WebProxy/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Check if authentication was successful
	// Usually a successful login redirects or returns 200 with cookies
	success := resp.StatusCode >= 200 && resp.StatusCode < 400 && len(resp.Cookies()) > 0

	return &WebAuthResult{
		Success:     success,
		Message:     fmt.Sprintf("Form auth %s", map[bool]string{true: "succeeded", false: "failed"}[success]),
		Cookies:     resp.Cookies(),
		RedirectURL: resp.Header.Get("Location"),
	}, nil
}

// APILoginAuthStrategy implements API-based authentication
type APILoginAuthStrategy struct{}

func (s *APILoginAuthStrategy) Name() string  { return "api_login" }
func (s *APILoginAuthStrategy) Priority() int { return 8 }

func (s *APILoginAuthStrategy) CanHandle(ctx context.Context, siteInfo *WebSiteInfo) bool {
	// Check for common API login endpoints
	commonEndpoints := []string{"/api/login", "/auth/login", "/login", "/signin"}
	baseURL, err := url.Parse(siteInfo.URL)
	if err != nil {
		return false
	}

	client := &http.Client{Timeout: 5 * time.Second}
	for _, endpoint := range commonEndpoints {
		testURL := baseURL.Scheme + "://" + baseURL.Host + endpoint
		resp, err := client.Head(testURL)
		if err == nil && resp.StatusCode != 404 {
			resp.Body.Close()
			return true
		}
	}
	return false
}

func (s *APILoginAuthStrategy) Authenticate(ctx context.Context, credentials *WebCredentials, siteInfo *WebSiteInfo) (*WebAuthResult, error) {
	baseURL, err := url.Parse(siteInfo.URL)
	if err != nil {
		return nil, err
	}

	// Try common API login endpoints
	commonEndpoints := []string{"/api/login", "/auth/login", "/login", "/signin"}

	client := &http.Client{Timeout: 30 * time.Second}

	for _, endpoint := range commonEndpoints {
		loginURL := baseURL.Scheme + "://" + baseURL.Host + endpoint

		// Prepare JSON payload
		payload := map[string]string{
			"username": credentials.Username,
			"password": credentials.Password,
		}

		jsonData, err := json.Marshal(payload)
		if err != nil {
			continue
		}

		req, err := http.NewRequestWithContext(ctx, "POST", loginURL, bytes.NewReader(jsonData))
		if err != nil {
			continue
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "OneTerm-WebProxy/1.0")

		resp, err := client.Do(req)
		if err != nil {
			continue
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			defer resp.Body.Close()
			return &WebAuthResult{
				Success: true,
				Message: "API login succeeded",
				Cookies: resp.Cookies(),
			}, nil
		}
		resp.Body.Close()
	}

	return &WebAuthResult{
		Success: false,
		Message: "API login failed - no valid endpoint found",
	}, nil
}
