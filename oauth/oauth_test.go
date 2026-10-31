package oauth

import (
	"net/http/httptest"
	"net/url"
	"testing"
)

// TestAuthorizeURL checks the address the browser opens: Yandex needs the
// client, the registered redirect, the state and the PKCE challenge.
func TestAuthorizeURL(t *testing.T) {
	verifier := "verifier123"
	address := authorizeURL("state123", verifier)

	parsed, err := url.Parse(address)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Scheme != "https" || parsed.Host != "oauth.yandex.ru" ||
		parsed.Path != "/authorize" {
		t.Fatalf("the address is %s", address)
	}

	query := parsed.Query()
	want := map[string]string{
		"response_type":         "code",
		"client_id":             clientID,
		"redirect_uri":          redirectURI,
		"state":                 "state123",
		"code_challenge_method": "S256",
		"code_challenge":        codeChallenge(verifier),
	}
	for key, value := range want {
		if got := query.Get(key); got != value {
			t.Errorf("%s is %q, want %q", key, got, value)
		}
	}
}

// TestCallbackChecksTheState checks that a foreign request cannot end the
// authorization of this program.
func TestCallbackChecksTheState(t *testing.T) {
	answers := make(chan oauthAnswer, 1)
	handler := callbackHandler("right", answers)

	foreign := httptest.NewRecorder()
	handler.ServeHTTP(foreign, httptest.NewRequest("GET", "/?code=abc&state=wrong", nil))
	if foreign.Code == 200 {
		t.Fatal("a request with a foreign state must be refused")
	}
	select {
	case a := <-answers:
		t.Fatalf("a foreign request answered with %+v", a)
	default:
	}

	right := httptest.NewRecorder()
	handler.ServeHTTP(right, httptest.NewRequest("GET", "/?code=abc&state=right", nil))
	if right.Code != 200 {
		t.Fatalf("the callback answered with %d", right.Code)
	}
	a := <-answers
	if a.err != nil || a.code != "abc" {
		t.Fatalf("the callback gave %+v", a)
	}
}

// TestCallbackReportsTheError checks the refusal of the user.
func TestCallbackReportsTheError(t *testing.T) {
	answers := make(chan oauthAnswer, 1)
	handler := callbackHandler("right", answers)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(
		"GET", "/?error=access_denied&error_description=denied&state=right", nil))

	a := <-answers
	if a.err == nil {
		t.Fatal("the refusal must be an error")
	}
	if oauthErr, ok := a.err.(*OAuthError); !ok || oauthErr.Code != "access_denied" {
		t.Fatalf("the error is %+v", a.err)
	}
}
