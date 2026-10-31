// Package oauth signs in to Yandex Music with the device flow: the user gets
// a short code, approves it on the Yandex page, and the token is polled in the
// background. The client is the one the official Music app uses, so the token
// can open the Music API, unlike the token of a self-registered app.
package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

const (
	clientID     = "23cabbbdc6cd418abb4b39c32c41195d"
	clientSecret = "53bc75238f0c4d08a118e51fe9203300"

	deviceName = "yatui"
)

// oauthServer is a variable, so the tests can answer instead of Yandex.
var oauthServer = "https://oauth.yandex.ru"

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

// DeviceCode is the answer of the device authorization request.
type DeviceCode struct {
	Code            string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURL string `json:"verification_url"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// TokenResponse is the answer of the token endpoint.
type TokenResponse struct {
	AccessToken string `json:"access_token"`
}

// RequestCode asks Yandex for a device code: the user is told to open the page
// and enter the short code, while the long one is kept for the polling.
func RequestCode(ctx context.Context) (*DeviceCode, error) {
	form := url.Values{
		"client_id":   {clientID},
		"device_id":   {deviceID()},
		"device_name": {deviceName},
	}

	body, err := postForm(ctx, oauthServer+"/device/code", form)
	if err != nil {
		return nil, err
	}

	var code DeviceCode
	if err := json.Unmarshal(body, &code); err != nil {
		return nil, err
	}
	if code.Code == "" || code.UserCode == "" {
		return nil, errors.New("empty device code in the response")
	}
	if code.VerificationURL == "" {
		code.VerificationURL = "https://ya.ru/device"
	}
	return &code, nil
}

// WaitToken polls the token endpoint until the user approves the code. Yandex
// answers authorization_pending while the page is still open, and slow_down
// when the polling is too fast.
func WaitToken(ctx context.Context, code *DeviceCode) (string, error) {
	interval := time.Duration(code.Interval) * time.Second
	if interval <= 0 {
		interval = 5 * time.Second
	}

	deadline := time.Now().Add(time.Duration(code.ExpiresIn) * time.Second)
	if code.ExpiresIn <= 0 {
		deadline = time.Now().Add(5 * time.Minute)
	}

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(interval):
		}

		if time.Now().After(deadline) {
			return "", &OAuthError{Code: "expired_token",
				Description: "the code has expired, start the sign-in again"}
		}

		token, err := pollToken(ctx, code.Code)
		if err == nil {
			return token, nil
		}

		var oauthErr *OAuthError
		if !errors.As(err, &oauthErr) {
			return "", err
		}

		switch oauthErr.Code {
		case "authorization_pending":
			continue
		case "slow_down":
			interval += 5 * time.Second
			continue
		default:
			return "", err
		}
	}
}

// pollToken makes one request to the token endpoint. The answer is 400 with
// authorization_pending until the user approves the code.
func pollToken(ctx context.Context, deviceCode string) (string, error) {
	form := url.Values{
		"grant_type":    {"device_code"},
		"code":          {deviceCode},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
	}

	body, err := postForm(ctx, oauthServer+"/token", form)
	if err != nil {
		return "", err
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

// postForm sends the form and returns the body: a refusal of Yandex is turned
// into OAuthError, so the caller can tell a pending code from a real problem.
func postForm(ctx context.Context, address string, form url.Values) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, address,
		strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		var oauthErr OAuthError
		if json.Unmarshal(body, &oauthErr) == nil && oauthErr.Code != "" {
			return nil, &oauthErr
		}
		return nil, fmt.Errorf("request failed (%d): %s",
			resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return body, nil
}

// deviceID is a stable name of this installation: Yandex shows the devices
// that have access to the account, and a new random id on every sign-in would
// fill that list.
func deviceID() string {
	if host, err := os.Hostname(); err == nil && host != "" {
		return host
	}
	return fmt.Sprintf("yatui-%d", time.Now().UnixNano())
}

// OpenBrowser opens the address in the default browser. On Windows the whole
// address is passed to rundll32: "cmd /c start" cuts it at the first &.
func OpenBrowser(target string) {
	switch runtime.GOOS {
	case "windows":
		exec.Command("rundll32", "url.dll,FileProtocolHandler", target).Start()
	case "darwin":
		exec.Command("open", target).Start()
	default:
		exec.Command("xdg-open", target).Start()
	}
}

// CopyToClipboard puts the short code into the clipboard, so the user only
// pastes it on the Yandex page. Other systems are left to the user: there is
// no standard clipboard tool there.
func CopyToClipboard(text string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "clip")
	case "darwin":
		cmd = exec.Command("pbcopy")
	default:
		cmd = exec.Command("xclip", "-selection", "clipboard")
	}
	cmd.Stdin = strings.NewReader(text)
	_ = cmd.Run()
}
