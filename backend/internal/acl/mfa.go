package acl

import (
	"strings"
	"time"

	"github.com/go-resty/resty/v2"

	"github.com/veops/oneterm/pkg/config"
)

// MFAIntrospectRequest represents the request to MFA introspect endpoint
type MFAIntrospectRequest struct {
	MfaToken string `json:"mfa_token"`
}

// MFAIntrospectResponse represents the response from MFA introspect endpoint
type MFAIntrospectResponse struct {
	Active bool   `json:"active"`
	UID    int64  `json:"uid"`
	Scope  string `json:"scope"`
	Exp    int64  `json:"exp"`
}

// VerifyMFAToken verifies the MFA token by calling the introspect endpoint
func VerifyMFAToken(mfaToken string) bool {
	// Build URL from config, replacing v1 with common-setting/v1
	baseURL := config.Cfg.Auth.Acl.Url
	url := strings.Replace(baseURL, "/v1", "/common-setting/v1", 1) + "/mfa/introspect"

	// Prepare request data
	reqData := MFAIntrospectRequest{
		MfaToken: mfaToken,
	}

	// Create resty client with timeout
	client := resty.New().SetTimeout(5 * time.Second)

	var mfaResp MFAIntrospectResponse
	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetBody(reqData).
		SetResult(&mfaResp).
		Post(url)

	if err != nil {
		return false
	}

	if resp.StatusCode() != 200 {
		return false
	}

	// Check if token is active
	if !mfaResp.Active {
		return false
	}

	// Check if token is not expired
	now := time.Now().Unix()
	if mfaResp.Exp > 0 && now > mfaResp.Exp {
		return false
	}

	return true
}
