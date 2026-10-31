package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// fakeYandex answers the device code and the token requests of the tests.
func fakeYandex(t *testing.T, tokenHandler func(w http.ResponseWriter, r *http.Request)) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("bad form: %s", err)
		}

		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/device/code":
			if got := r.Form.Get("client_id"); got != clientID {
				t.Errorf("client_id is %q", got)
			}
			if r.Form.Get("device_id") == "" || r.Form.Get("device_name") == "" {
				t.Error("the device is not named")
			}
			json.NewEncoder(w).Encode(map[string]any{
				"device_code":      "long-code",
				"user_code":        "abcd-efgh",
				"verification_url": "https://ya.ru/device",
				"expires_in":       300,
				"interval":         1,
			})
		case "/token":
			tokenHandler(w, r)
		default:
			http.NotFound(w, r)
		}
	}))

	saved := oauthServer
	oauthServer = server.URL
	t.Cleanup(func() {
		oauthServer = saved
		server.Close()
	})
	return server
}

// TestRequestCode checks the device code request and its answer.
func TestRequestCode(t *testing.T) {
	fakeYandex(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("the token must not be requested")
	})

	code, err := RequestCode(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if code.Code != "long-code" || code.UserCode != "abcd-efgh" {
		t.Fatalf("the code is %+v", code)
	}
	if code.VerificationURL != "https://ya.ru/device" {
		t.Fatalf("the page is %q", code.VerificationURL)
	}
}

// TestWaitTokenPollsUntilApproved checks that a pending code is polled again,
// and that the approved one becomes the token.
func TestWaitTokenPollsUntilApproved(t *testing.T) {
	polls := 0
	fakeYandex(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Form.Get("grant_type") != "device_code" {
			t.Errorf("grant_type is %q", r.Form.Get("grant_type"))
		}
		if r.Form.Get("code") != "long-code" {
			t.Errorf("code is %q", r.Form.Get("code"))
		}
		polls++
		if polls == 1 {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error":             "authorization_pending",
				"error_description": "User has not yet authorized your application",
			})
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"access_token": "good-token"})
	})

	code := &DeviceCode{Code: "long-code", Interval: 1, ExpiresIn: 30}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	token, err := WaitToken(ctx, code)
	if err != nil {
		t.Fatal(err)
	}
	if token != "good-token" {
		t.Fatalf("the token is %q", token)
	}
	if polls != 2 {
		t.Fatalf("the endpoint was polled %d times", polls)
	}
}

// TestWaitTokenReportsRefusal checks that a refused code ends the waiting with
// the error of Yandex.
func TestWaitTokenReportsRefusal(t *testing.T) {
	fakeYandex(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":             "access_denied",
			"error_description": "The user denied the request",
		})
	})

	code := &DeviceCode{Code: "long-code", Interval: 1, ExpiresIn: 30}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := WaitToken(ctx, code)
	var oauthErr *OAuthError
	if !errors.As(err, &oauthErr) || oauthErr.Code != "access_denied" {
		t.Fatalf("the error is %+v", err)
	}
}
