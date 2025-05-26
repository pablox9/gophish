package o365auth

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	deviceCodeURL = "https://login.microsoftonline.com/common/oauth2/v2.0/devicecode"
	tokenURL      = "https://login.microsoftonline.com/common/oauth2/v2.0/token"
)

// DeviceAuthResponse represents the response from the device code endpoint.
type DeviceAuthResponse struct {
	UserCode        string `json:"user_code"`
	DeviceCode      string `json:"device_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
	Message         string `json:"message"`
}

// TokenResponse represents the response from the token endpoint.
type TokenResponse struct {
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
	ExpiresIn    int    `json:"expires_in"`
	ExtExpiresIn int    `json:"ext_expires_in"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token,omitempty"`
}

// ErrorResponse represents an error response from Microsoft's identity platform.
type ErrorResponse struct {
	Error            string   `json:"error"`
	ErrorDescription string   `json:"error_description"`
	ErrorCodes       []int    `json:"error_codes"`
	Timestamp        string   `json:"timestamp"`
	TraceID          string   `json:"trace_id"`
	CorrelationID    string   `json:"correlation_id"`
	ErrorURI         string   `json:"error_uri"`
}

// InitiateDeviceAuth initiates the device authorization flow.
func InitiateDeviceAuth(clientID string, scope string) (*DeviceAuthResponse, error) {
	data := url.Values{}
	data.Set("client_id", clientID)
	data.Set("scope", scope)

	req, err := http.NewRequest("POST", deviceCodeURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request to device code endpoint: %w", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if err := json.Unmarshal(body, &errResp); err == nil {
			return nil, fmt.Errorf("device auth error: %s - %s", errResp.Error, errResp.ErrorDescription)
		}
		return nil, fmt.Errorf("device auth request failed with status %s: %s", resp.Status, string(body))
	}

	var deviceAuthResp DeviceAuthResponse
	if err := json.Unmarshal(body, &deviceAuthResp); err != nil {
		return nil, fmt.Errorf("error unmarshalling device auth response: %w", err)
	}

	return &deviceAuthResp, nil
}

// PollTokenEndpoint polls the token endpoint until the user authenticates or the request expires.
func PollTokenEndpoint(clientID string, deviceCode string, interval int, expiresIn int) (*TokenResponse, error) {
	data := url.Values{}
	data.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
	data.Set("client_id", clientID)
	data.Set("device_code", deviceCode)

	startTime := time.Now()
	timeout := time.Duration(expiresIn) * time.Second

	client := &http.Client{}

	for {
		// Check for timeout before making the request
		if time.Since(startTime) > timeout {
			return nil, errors.New("polling timed out: user did not authenticate in time")
		}

		req, err := http.NewRequest("POST", tokenURL, strings.NewReader(data.Encode()))
		if err != nil {
			return nil, fmt.Errorf("error creating token request: %w", err)
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		resp, err := client.Do(req)
		if err != nil {
			// It's possible the request itself failed before we even get a status code
			// Wait for the interval before retrying, unless we've timed out.
			if time.Since(startTime) < timeout {
				time.Sleep(time.Duration(interval) * time.Second)
				continue
			}
			return nil, fmt.Errorf("error making request to token endpoint: %w", err)
		}
		
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		resp.Body.Close() // Close body immediately after reading
		if err != nil {
			return nil, fmt.Errorf("error reading token response body: %w", err)
		}

		if resp.StatusCode == http.StatusOK {
			var tokenResp TokenResponse
			if err := json.Unmarshal(bodyBytes, &tokenResp); err != nil {
				return nil, fmt.Errorf("error unmarshalling token response: %w", err)
			}
			if tokenResp.AccessToken != "" {
				return &tokenResp, nil
			}
			// It's unusual to get a 200 OK without an access token if no error is specified,
			// but we'll treat it like a pending situation just in case.
			log.Println("Received 200 OK but no access token, treating as pending.")
		} else {
			var errResp ErrorResponse
			if err := json.Unmarshal(bodyBytes, &errResp); err == nil {
				switch errResp.Error {
				case "authorization_pending":
					// Continue polling
				case "authorization_declined":
					return nil, fmt.Errorf("authorization declined: %s", errResp.ErrorDescription)
				case "bad_verification_code":
					return nil, fmt.Errorf("bad verification code: %s", errResp.ErrorDescription)
				case "expired_token":
					return nil, fmt.Errorf("expired token: %s (device code expired)", errResp.ErrorDescription)
				default:
					return nil, fmt.Errorf("token endpoint error: %s - %s", errResp.Error, errResp.ErrorDescription)
				}
			} else {
				// Could not parse the error response, return generic error
				return nil, fmt.Errorf("token endpoint request failed with status %s: %s", resp.Status, string(bodyBytes))
			}
		}

		// Wait for the interval before the next poll, but check timeout first
		if time.Since(startTime) < timeout {
			time.Sleep(time.Duration(interval) * time.Second)
		} else {
			// If we exit the loop due to timeout after the sleep check.
			return nil, errors.New("polling timed out after waiting for interval: user did not authenticate in time")
		}
	}
}

// RefreshToken refreshes an expired access token using a refresh token.
func RefreshToken(clientID string, refreshToken string, scope string) (*TokenResponse, error) {
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("client_id", clientID)
	data.Set("refresh_token", refreshToken)
	data.Set("scope", scope) // Scope is often optional for refresh, but can be included

	req, err := http.NewRequest("POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("error creating refresh token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request to refresh token: %w", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading refresh token response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if err := json.Unmarshal(body, &errResp); err == nil {
			return nil, fmt.Errorf("refresh token error: %s - %s", errResp.Error, errResp.ErrorDescription)
		}
		return nil, fmt.Errorf("refresh token request failed with status %s: %s", resp.Status, string(body))
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("error unmarshalling refresh token response: %w", err)
	}
	if tokenResp.AccessToken == "" {
		// This would be an unusual success case, but good to check
		return nil, errors.New("refresh token response successful but access_token is missing")
	}

	return &tokenResp, nil
}

// CheckTokenStatusOnce makes a single request to the token endpoint to check the status of user authorization.
// It returns a TokenResponse if successful, a status string ("success", "authorization_pending", or other error types),
// and an error if the request failed or a non-pending error occurred.
func CheckTokenStatusOnce(clientID string, deviceCode string) (*TokenResponse, string, error) {
	data := url.Values{}
	data.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
	data.Set("client_id", clientID)
	data.Set("device_code", deviceCode)

	req, err := http.NewRequest("POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, "communication_error", fmt.Errorf("error creating token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Use a client with a timeout for a single, non-blocking check.
	// The overall timeout for the device code flow is handled by DeviceCodeExpiry.
	httpClient := &http.Client{Timeout: 15 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, "communication_error", fmt.Errorf("error making request to token endpoint: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, "communication_error", fmt.Errorf("error reading token response body: %w", err)
	}

	if resp.StatusCode == http.StatusOK {
		var tokenResp TokenResponse
		if err := json.Unmarshal(bodyBytes, &tokenResp); err != nil {
			return nil, "communication_error", fmt.Errorf("error unmarshalling token response: %w", err)
		}
		if tokenResp.AccessToken != "" {
			return &tokenResp, "success", nil
		}
		// This case (200 OK but no access_token and no specific error in body) is unusual.
		// Microsoft's spec implies an error structure should be present if not success.
		// Treating as "authorization_pending" is a safe fallback but should be monitored if it occurs.
		log.Printf("Warning: Received 200 OK from token endpoint but no access_token for device_code %s. Body: %s", deviceCode, string(bodyBytes))
		return nil, "authorization_pending", nil
	}

	var errResp ErrorResponse
	if err := json.Unmarshal(bodyBytes, &errResp); err == nil {
		// Log the actual error description for better debugging
		log.Printf("Token endpoint returned error for device_code %s: %s - %s", deviceCode, errResp.Error, errResp.ErrorDescription)
		parsedError := fmt.Errorf("%s: %s", errResp.Error, errResp.ErrorDescription)
		switch errResp.Error {
		case "authorization_pending":
			return nil, "authorization_pending", nil // No error for pending, it's an expected state
		case "authorization_declined":
			return nil, "authorization_declined", parsedError
		case "bad_verification_code":
			return nil, "bad_verification_code", parsedError
		case "expired_token":
			return nil, "expired_token", parsedError
		default:
			// For any other error string from Microsoft.
			return nil, "unknown_error", parsedError
		}
	}
	// If the response body was not a standard error JSON.
	return nil, "communication_error", fmt.Errorf("token endpoint request failed with status %s: %s", resp.Status, string(bodyBytes))
}

// Helper function to pretty print structs for debugging
func prettyPrint(i interface{}) string {
	s, _ := json.MarshalIndent(i, "", "\t")
	return string(s)
}

// Example Usage (can be removed or kept for testing)
func main() {
	// This is a placeholder and will not work without a real Client ID.
	// You also need to ensure the application is configured for device code flow.
	clientID := "YOUR_CLIENT_ID_HERE" 
	scope := "User.Read Mail.Read offline_access" // offline_access is needed for refresh_token

	// Initiate Device Auth
	log.Println("Initiating Device Authentication...")
	deviceAuthResp, err := InitiateDeviceAuth(clientID, scope)
	if err != nil {
		log.Fatalf("Error initiating device auth: %v\n", err)
	}
	log.Printf("Device Auth Response:\n%s\n", prettyPrint(deviceAuthResp))
	log.Printf("Please go to %s and enter code: %s\n", deviceAuthResp.VerificationURI, deviceAuthResp.UserCode)

	// Poll for Token
	log.Println("Polling for token...")
	tokenResp, err := PollTokenEndpoint(clientID, deviceAuthResp.DeviceCode, deviceAuthResp.Interval, deviceAuthResp.ExpiresIn)
	if err != nil {
		log.Fatalf("Error polling for token: %v\n", err)
	}
	log.Printf("Token Response:\n%s\n", prettyPrint(tokenResp))
	log.Println("Authentication successful!")

	// Example: Refresh Token (if you got a refresh_token)
	if tokenResp.RefreshToken != "" {
		log.Println("Attempting to refresh token...")
		// Give it a moment, or simulate waiting for the token to be near expiry
		time.Sleep(5 * time.Second) 
		
		refreshedTokenResp, err := RefreshToken(clientID, tokenResp.RefreshToken, scope)
		if err != nil {
			log.Fatalf("Error refreshing token: %v\n", err)
		}
		log.Printf("Refreshed Token Response:\n%s\n", prettyPrint(refreshedTokenResp))
		log.Println("Token refresh successful!")
	} else {
		log.Println("No refresh token received, skipping refresh token example.")
	}
}
