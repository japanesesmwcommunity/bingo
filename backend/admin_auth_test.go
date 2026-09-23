package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type adminTransport func(*http.Request) (*http.Response, error)

func (f adminTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func testAdminServer(t *testing.T) *server {
	t.Helper()
	s := testServer(t)
	contents, err := os.ReadFile("bingo.json")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "bingo.json")
	if err := os.WriteFile(path, contents, 0600); err != nil {
		t.Fatal(err)
	}
	s.admin = newAdminService(adminConfig{ClientID: "123456789012345678", ClientSecret: "server-secret", RedirectURI: "http://127.0.0.1:8080/api/admin/callback", AllowedIDs: map[string]bool{"234567890123456789": true}}, path)
	return s
}

func beginAdminLogin(t *testing.T, s *server) (string, *http.Cookie) {
	t.Helper()
	w := request(t, s, "GET", "/api/admin/login", nil, nil, 302)
	u, err := url.Parse(w.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	if u.Scheme+"://"+u.Host+u.Path != discordAuthorizeURL || u.Query().Get("scope") != "identify" || u.Query().Get("redirect_uri") != s.admin.config.RedirectURI {
		t.Fatal(u)
	}
	state := u.Query().Get("state")
	cookie := w.Result().Cookies()[0]
	if state == "" || cookie.Value != state || !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatal("unbound oauth state")
	}
	return state, cookie
}

func mockDiscord(t *testing.T, s *server, id string) {
	t.Helper()
	s.admin.client.Transport = adminTransport(func(r *http.Request) (*http.Response, error) {
		body := ""
		switch r.URL.String() {
		case discordTokenURL:
			client, secret, ok := r.BasicAuth()
			if !ok || client != s.admin.config.ClientID || secret != "server-secret" {
				t.Error("missing confidential client credentials")
			}
			if err := r.ParseForm(); err != nil {
				t.Fatal(err)
			}
			if r.PostForm.Get("grant_type") != "authorization_code" || r.PostForm.Get("code") != "valid-code" || r.PostForm.Get("redirect_uri") != s.admin.config.RedirectURI {
				t.Error("invalid code exchange")
			}
			body = `{"access_token":"server-token","token_type":"Bearer"}`
		case discordUserURL:
			if r.Header.Get("Authorization") != "Bearer server-token" {
				t.Error("missing identity token")
			}
			body = fmt.Sprintf(`{"id":%q,"username":"admin-user"}`, id)
		default:
			t.Fatalf("unexpected provider request %s", r.URL)
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})
}

func loginAdmin(t *testing.T, s *server) *http.Cookie {
	t.Helper()
	mockDiscord(t, s, "234567890123456789")
	state, cookie := beginAdminLogin(t, s)
	w := request(t, s, "GET", "/api/admin/callback?state="+state+"&code=valid-code", nil, cookie, 303)
	if w.Header().Get("Location") != "/admin" {
		t.Fatal(w.Header())
	}
	for _, result := range w.Result().Cookies() {
		if result.Name == adminCookieName {
			return result
		}
	}
	t.Fatal("missing admin session")
	return nil
}

func TestAdminAuthenticationAndSessionIsolation(t *testing.T) {
	s := testAdminServer(t)
	request(t, s, "GET", "/admin", nil, nil, 200)
	request(t, s, "GET", "/api/admin/goals", nil, nil, 401)
	host, roomCookie := createTestRoom(t, s, "line")
	request(t, s, "GET", "/api/admin/goals", nil, roomCookie, 401)
	cookie := loginAdmin(t, s)
	if cookie.Path != "/api/admin" || !cookie.HttpOnly || cookie.MaxAge != int(adminSessionLifetime.Seconds()) {
		t.Fatal(cookie)
	}
	request(t, s, "GET", "/api/rooms/"+host.Room.ID, nil, cookie, 401)
	w := request(t, s, "GET", "/api/admin/session", nil, cookie, 200)
	if strings.Contains(w.Body.String(), "secret") || strings.Contains(w.Body.String(), "token") || !strings.Contains(w.Body.String(), "admin-user") {
		t.Fatal(w.Body.String())
	}
	request(t, s, "GET", "/api/admin/goals", nil, cookie, 200)
	s.admin.config.AllowedIDs = map[string]bool{"345678901234567890": true}
	request(t, s, "GET", "/api/admin/goals", nil, cookie, 401)
	s.admin.config.AllowedIDs = map[string]bool{"234567890123456789": true}
	cookie = loginAdmin(t, s)
	s.admin.mu.Lock()
	value := s.admin.sessions[cookie.Value]
	value.Expires = time.Now().Add(-time.Second)
	s.admin.sessions[cookie.Value] = value
	s.admin.mu.Unlock()
	request(t, s, "GET", "/api/admin/goals", nil, cookie, 401)
	cookie = loginAdmin(t, s)
	request(t, s, "POST", "/api/admin/logout", nil, cookie, 204)
	request(t, s, "GET", "/api/admin/goals", nil, cookie, 401)
	request(t, s, "POST", "/api/admin/logout", nil, nil, 204)
}

func TestAdminOAuthRejectsUnboundExpiredAndDisallowedUsers(t *testing.T) {
	s := testAdminServer(t)
	mockDiscord(t, s, "345678901234567890")
	state, cookie := beginAdminLogin(t, s)
	w := request(t, s, "GET", "/api/admin/callback?state="+state+"&code=valid-code", nil, nil, 303)
	if w.Header().Get("Location") != "/admin?error=state" {
		t.Fatal(w.Header())
	}
	w = request(t, s, "GET", "/api/admin/callback?state=wrong&code=valid-code", nil, cookie, 303)
	if w.Header().Get("Location") != "/admin?error=state" {
		t.Fatal(w.Header())
	}
	w = request(t, s, "GET", "/api/admin/callback?state="+state+"&code=valid-code", nil, cookie, 303)
	if w.Header().Get("Location") != "/admin?error=denied" || len(s.admin.sessions) != 0 {
		t.Fatal("unlisted user accepted")
	}
	w = request(t, s, "GET", "/api/admin/callback?state="+state+"&code=valid-code", nil, cookie, 303)
	if w.Header().Get("Location") != "/admin?error=state" {
		t.Fatal("state replay accepted")
	}
	state, cookie = beginAdminLogin(t, s)
	s.admin.states[state] = time.Now().Add(-time.Second)
	w = request(t, s, "GET", "/api/admin/callback?state="+state+"&code=valid-code", nil, cookie, 303)
	if w.Header().Get("Location") != "/admin?error=state" {
		t.Fatal("expired state accepted")
	}
	state, cookie = beginAdminLogin(t, s)
	w = request(t, s, "GET", "/api/admin/callback?state="+state+"&error=access_denied", nil, cookie, 303)
	if w.Header().Get("Location") != "/admin?error=cancelled" {
		t.Fatal(w.Header())
	}
	s.admin.config = adminConfig{}
	request(t, s, "GET", "/api/admin/login", nil, nil, 503)
	request(t, s, "GET", "/api/admin/callback", nil, nil, 503)
	w = request(t, s, "GET", "/api/admin/session", nil, nil, 200)
	var status struct {
		Configured bool
		User       *discordUser
	}
	if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil || status.Configured || status.User != nil {
		t.Fatal(status, err)
	}
}

func TestAdminDiscordFailuresAndSecureCookies(t *testing.T) {
	s := testAdminServer(t)
	s.admin.config.RedirectURI = "https://example.com/api/admin/callback"
	if !s.adminCookie(adminCookieName, "x", time.Hour).Secure {
		t.Fatal("HTTPS cookie must be secure")
	}
	for _, body := range []string{"{", `{}`, `{"access_token":"x","token_type":"unknown"}`, `{"access_token":"x","token_type":"Bearer"}`} {
		s.admin.client.Transport = adminTransport(func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
		})
		state, cookie := beginAdminLogin(t, s)
		w := request(t, s, "GET", "/api/admin/callback?state="+state+"&code=valid-code", nil, cookie, 303)
		if w.Header().Get("Location") != "/admin?error=discord" {
			t.Fatal(w.Header())
		}
	}
	for _, code := range []int{429, 500} {
		s.admin.client.Transport = adminTransport(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader("private provider error"))}, nil
		})
		if _, err := s.admin.identify(httptest.NewRequest("GET", "/", nil), "valid-code"); err == nil {
			t.Fatal("provider error ignored")
		}
	}
	s.admin.client.Transport = adminTransport(func(*http.Request) (*http.Response, error) { return nil, fmt.Errorf("offline") })
	if _, err := s.admin.identify(httptest.NewRequest("GET", "/", nil), "valid-code"); err == nil {
		t.Fatal("offline provider accepted")
	}
}
