// Package oauth obtains a Yandex token through the OAuth authorization code
// flow with PKCE.
//
// The client is registered on oauth.yandex.ru as a web service with the
// redirect URI http://127.0.0.1:8080: the browser is sent there after the
// user allows the access, and the local server hands the code over to the
// token endpoint.
package oauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

const (
	// clientID and clientSecret belong to the "yatui" web service of the
	// user. The redirect URI of that client must match redirectURI exactly.
	clientID     = "31c5aa11dbdd4fdd8752aa462ce0db47"
	clientSecret = "8c6c3f12a9d5446b8fcf5e47e157c1c3"
	redirectURI  = "http://127.0.0.1:8080"
)

// OAuthError is an error returned by Yandex OAuth.
type OAuthError struct {
	Code        string `json:"error"`
	Description string `json:"error_description"`
}

func (e *OAuthError) Error() string {
	if e.Description != "" {
		return e.Code + ": " + e.Description
	}
	return e.Code
}

// TokenResponse is the answer of the token endpoint.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// GetToken performs the authorization code flow: it opens the browser, waits
// for the callback and exchanges the code for a token. The caller decides how
// long the user may take through ctx.
func GetToken(ctx context.Context) (string, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:8080")
	if err != nil {
		return "", fmt.Errorf("port 8080 is busy: %w", err)
	}
	defer listener.Close()

	verifier := randomString()
	state := randomString()

	answers := make(chan oauthAnswer, 1)
	server := &http.Server{Handler: callbackHandler(state, answers)}
	go server.Serve(listener)
	defer shutdown(server)

	openBrowser(authorizeURL(state, verifier))

	code, err := awaitCode(ctx, answers)
	if err != nil {
		return "", err
	}
	return exchangeCode(ctx, code, verifier)
}

// callbackHandler answers the redirect of the browser. The state proves that
// the request belongs to the authorization the program has started.
func callbackHandler(state string, answers chan<- oauthAnswer) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		if query.Get("state") != state {
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}

		if code := query.Get("error"); code != "" {
			page(w, http.StatusBadRequest, "Authorization failed",
				query.Get("error_description"))
			answers <- oauthAnswer{err: &OAuthError{
				Code:        code,
				Description: query.Get("error_description"),
			}}
			return
		}

		if code := query.Get("code"); code != "" {
			page(w, http.StatusOK, "Authorization successful!",
				"You can close this page and return to yatui.")
			answers <- oauthAnswer{code: code}
			return
		}

		http.NotFound(w, r)
	})
	return mux
}

// oauthAnswer carries the result of the callback; the buffer keeps the
// handler from blocking on a late second request.
type oauthAnswer struct {
	code string
	err  error
}

func awaitCode(ctx context.Context, answers <-chan oauthAnswer) (string, error) {
	select {
	case a := <-answers:
		return a.code, a.err
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// authorizeURL builds the address the browser opens. The permissions are the
// ones chosen for the client at registration, so no scope is requested here.
func authorizeURL(state, verifier string) string {
	query := url.Values{
		"response_type":         {"code"},
		"client_id":             {clientID},
		"redirect_uri":          {redirectURI},
		"state":                 {state},
		"code_challenge":        {codeChallenge(verifier)},
		"code_challenge_method": {"S256"},
	}
	return "https://oauth.yandex.ru/authorize?" + query.Encode()
}

// exchangeCode trades the code for a token. The client secret is passed with
// Basic auth, the PKCE verifier proves that the code belongs to this attempt.
func exchangeCode(ctx context.Context, code, verifier string) (string, error) {
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"code_verifier": {verifier},
		"redirect_uri":  {redirectURI},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://oauth.yandex.ru/token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(clientID, clientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		var oauthErr OAuthError
		if json.Unmarshal(body, &oauthErr) == nil && oauthErr.Code != "" {
			return "", &oauthErr
		}
		return "", fmt.Errorf("token request failed (%d): %s",
			resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var token TokenResponse
	if err := json.Unmarshal(body, &token); err != nil {
		return "", err
	}
	if token.AccessToken == "" {
		return "", errors.New("empty token in the response")
	}
	return token.AccessToken, nil
}

// randomString is a code verifier and a state: 32 random bytes in base64url.
func randomString() string {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		// crypto/rand never fails on the supported systems; a weak value is
		// still better than no authorization at all
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return base64.RawURLEncoding.EncodeToString(bytes)
}

func codeChallenge(verifier string) string {
	hash := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}

// page shows a short message in the browser after the redirect.
func page(w http.ResponseWriter, status int, title, text string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	io.WriteString(w, `<!DOCTYPE html>
<html><head><meta charset="utf-8"><title>`+title+`</title></head>
<body style="font-family:system-ui;text-align:center;padding:50px;background:#1a1a2e;color:#e0e0e0;">
<h2>`+title+`</h2><p>`+text+`</p>
</body></html>`)
}

// shutdown closes the local server after the page has been sent.
func shutdown(server *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
}

// openBrowser opens the address in the default browser. On Windows the whole
// address is passed to rundll32: "cmd /c start" would cut it at the first &
// and lose the query parameters.
func openBrowser(target string) {
	switch runtime.GOOS {
	case "windows":
		exec.Command("rundll32", "url.dll,FileProtocolHandler", target).Start()
	case "darwin":
		exec.Command("open", target).Start()
	default:
		exec.Command("xdg-open", target).Start()
	}
}
