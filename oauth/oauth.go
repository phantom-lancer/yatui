// Package oauth obtains a Yandex token through the OAuth authorization code
// flow with PKCE.
//
// It is not wired into the login page yet: Yandex requires the redirect URI to
// be registered for the client, and the client id of the original yamusic-tui
// project is not registered for a localhost redirect. The login page keeps the
// manual token input until a client of the user's own is configured here.
package oauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// TokenResponse represents the OAuth token response from Yandex.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// OAuthError represents an error returned by Yandex OAuth.
type OAuthError struct {
	OAuthErrorField string `json:"error"`
	ErrorDesc       string `json:"error_description"`
}

func (e *OAuthError) Error() string {
	if e.ErrorDesc != "" {
		return fmt.Sprintf("%s: %s", e.OAuthErrorField, e.ErrorDesc)
	}
	return e.OAuthErrorField
}

// GetToken performs OAuth2 Authorization Code Flow with PKCE.
// It opens a browser for authorization, then listens on a local port
// to receive the authorization code and exchange it for a token.
func GetToken(ctx context.Context) (string, error) {
	// Generate PKCE parameters
	codeVerifier := generateCodeVerifier()
	codeChallenge := generateCodeChallenge(codeVerifier)

	// Use a fixed redirect_uri that should be registered for this client_id
	// Try multiple common ports since we don't know which is registered
	ports := []int{8080, 8081, 8082, 4321, 9876, 3000, 5000, 5001, 1234, 6543}

	var lastErr error
	for _, port := range ports {
		redirectURI := fmt.Sprintf("http://127.0.0.1:%d", port)

		// Try to start a listener on this port
		listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err != nil {
			continue // Port busy, try next
		}

		// Successfully bound to port, try OAuth with this redirect_uri
		token, err := tryOAuth(ctx, redirectURI, codeChallenge, codeVerifier, listener)
		if err != nil {
			listener.Close()
			lastErr = err
			// Check if it's a Yandex registration error (redirect_uri not registered)
			if isRegistrationError(err) {
				// Try next port
				continue
			}
			return "", err
		}
		return token, nil
	}

	// All ports failed - provide helpful error message
	msg := "OAuth authorization failed. The Yandex OAuth client is not registered with any redirect_uri.\n\n"
	msg += "To fix this:\n"
	msg += "1. Go to https://oauth.yandex.ru/\n"
	msg += "2. Click 'New client'\n"
	msg += "3. Select 'Web service' and 'Yandex Account'\n"
	msg += "4. Set redirect URI: http://127.0.0.1:8080\n"
	msg += "5. Copy the client_id and replace it in oauth/oauth.go\n\n"
	if lastErr != nil {
		msg += "Last error: " + lastErr.Error()
	}
	return "", fmt.Errorf("%s", msg)
}

// isRegistrationError checks if the error indicates redirect_uri is not registered.
func isRegistrationError(err error) bool {
	if oauthErr, ok := err.(*OAuthError); ok {
		// Yandex returns "unauthorized_client" or "invalid_request" for unregistered redirect_uri
		return oauthErr.OAuthErrorField == "unauthorized_client" ||
			oauthErr.OAuthErrorField == "invalid_request" ||
			strings.Contains(oauthErr.ErrorDesc, "redirect_uri") ||
			strings.Contains(oauthErr.ErrorDesc, "не найден") ||
			strings.Contains(oauthErr.ErrorDesc, "устарело")
	}
	return false
}

func tryOAuth(ctx context.Context, redirectURI, codeChallenge, codeVerifier string, listener net.Listener) (string, error) {
	// Channel to receive the authorization code
	authCodeChan := make(chan string, 1)
	errChan := make(chan error, 1)

	// HTTP handler for the OAuth callback
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Check for authorization code in query params
		code := r.URL.Query().Get("code")
		_ = r.URL.Query().Get("state") // state is optional
		if code != "" {
			authCodeChan <- code
			// Show success page
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			io.WriteString(w, `<!DOCTYPE html>
<html><head><meta charset="utf-8"><title>Authorization Success</title></head>
<body style="font-family:system-ui;text-align:center;padding:50px;background:#1a1a2e;color:#e0e0e0;">
<h2>Authorization successful!</h2>
<p>You can close this window and return to the application.</p>
</body></html>`)
			go func() {
				time.Sleep(2 * time.Second)
				listener.Close()
			}()
			return
		}

		// Check for error from Yandex
		errMsg := r.URL.Query().Get("error")
		errDesc := r.URL.Query().Get("error_description")
		if errMsg != "" {
			errChan <- &OAuthError{OAuthErrorField: errMsg, ErrorDesc: errDesc}
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, fmt.Sprintf(`<!DOCTYPE html>
<html><head><meta charset="utf-8"><title>Authorization Error</title></head>
<body style="font-family:system-ui;text-align:center;padding:50px;background:#1a1a2e;color:#ff6b6b;">
<h2>Authorization failed</h2>
<p>%s</p>
</body></html>`, errDesc))
			go func() {
				time.Sleep(2 * time.Second)
				listener.Close()
			}()
			return
		}

		// Unknown response
		w.WriteHeader(http.StatusNotFound)
	})

	server := &http.Server{Handler: mux}
	go func() {
		server.Serve(listener)
	}()

	// Open browser with OAuth URL
	oauthURL := fmt.Sprintf(
		"https://oauth.yandex.ru/authorize?client_id=23bebbd942d5470da5c4c8357a3ac388"+
			"&redirect_uri=%s"+
			"&response_type=code"+
			"&code_challenge=%s"+
			"&code_challenge_method=S256",
		redirectURI,
		codeChallenge,
	)
	openBrowser(oauthURL)

	// Wait for auth code, error, or context cancellation
	select {
	case code := <-authCodeChan:
		// Exchange code for token
		token, err := exchangeCode(ctx, code, codeVerifier, redirectURI)
		if err != nil {
			return "", err
		}
		return token, nil

	case err := <-errChan:
		server.Close()
		return "", err

	case <-ctx.Done():
		server.Close()
		return "", ctx.Err()
	}
}

// generateCodeVerifier creates a random code verifier (43-128 characters).
func generateCodeVerifier() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return base64.RawURLEncoding.EncodeToString(bytes)
}

// generateCodeChallenge creates the code challenge from the verifier.
func generateCodeChallenge(verifier string) string {
	hash := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}

// exchangeCode exchanges the authorization code for an access token.
func exchangeCode(ctx context.Context, code, codeVerifier, redirectURI string) (string, error) {
	data := fmt.Sprintf(
		"grant_type=authorization_code&code=%s&code_verifier=%s&redirect_uri=%s",
		code, codeVerifier, redirectURI,
	)

	req, err := http.NewRequestWithContext(ctx, "POST", "https://oauth.yandex.ru/token", strings.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("failed to create token request: %w", err)
	}

	// Use Basic Auth with client_id (empty password as per OAuth2 spec for public clients)
	req.SetBasicAuth("23bebbd942d5470da5c4c8357a3ac388", "")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send token request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	// Check for error response
	if resp.StatusCode != http.StatusOK {
		var oauthErr OAuthError
		if json.Unmarshal(body, &oauthErr) == nil {
			return "", &oauthErr
		}
		return "", fmt.Errorf("token request failed (status %d): %s", resp.StatusCode, string(body))
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("failed to parse token response: %w", err)
	}

	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("empty access token in response")
	}

	return tokenResp.AccessToken, nil
}

func openBrowser(targetURL string) {
	switch runtime.GOOS {
	case "windows":
		exec.Command("cmd", "/c", "start", targetURL).Start()
	case "darwin":
		exec.Command("open", targetURL).Start()
	case "linux":
		exec.Command("xdg-open", targetURL).Start()
	default:
		exec.Command("xdg-open", targetURL).Start()
	}
}
