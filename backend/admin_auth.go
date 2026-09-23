package main

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

func (s *server) adminCookie(name, value string, lifetime time.Duration) *http.Cookie {
	return &http.Cookie{Name: name, Value: value, Path: "/api/admin", HttpOnly: true,
		Secure:   s.secureCookie || strings.HasPrefix(s.admin.config.RedirectURI, "https://"),
		SameSite: http.SameSiteLaxMode, MaxAge: int(lifetime.Seconds())}
}

func (s *server) adminLogin(w http.ResponseWriter, r *http.Request) {
	a := s.admin
	if !a.enabled() {
		writeError(w, 503, "管理者ログインは未設定です")
		return
	}
	if !s.allowAttempt(w, r) {
		return
	}
	state := uuid.NewString() + uuid.NewString()
	a.mu.Lock()
	now := time.Now()
	for key, expires := range a.states {
		if !expires.After(now) {
			delete(a.states, key)
		}
	}
	if len(a.states) >= 1000 {
		a.mu.Unlock()
		writeError(w, 429, "少し待ってから再試行してください")
		return
	}
	a.states[state] = now.Add(oauthStateLifetime)
	a.mu.Unlock()
	http.SetCookie(w, s.adminCookie(oauthCookieName, state, oauthStateLifetime))
	query := url.Values{"client_id": {a.config.ClientID}, "redirect_uri": {a.config.RedirectURI}, "response_type": {"code"}, "scope": {"identify"}, "state": {state}}
	http.Redirect(w, r, discordAuthorizeURL+"?"+query.Encode(), http.StatusFound)
}

func (s *server) adminCallback(w http.ResponseWriter, r *http.Request) {
	a := s.admin
	if !a.enabled() {
		writeError(w, 503, "管理者ログインは未設定です")
		return
	}
	state := r.URL.Query().Get("state")
	cookie, err := r.Cookie(oauthCookieName)
	if err != nil || state == "" || subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(state)) != 1 {
		http.Redirect(w, r, "/admin?error=state", http.StatusSeeOther)
		return
	}
	a.mu.Lock()
	expires, exists := a.states[state]
	delete(a.states, state)
	a.mu.Unlock()
	http.SetCookie(w, s.adminCookie(oauthCookieName, "", -time.Second))
	if !exists || !expires.After(time.Now()) {
		http.Redirect(w, r, "/admin?error=state", http.StatusSeeOther)
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" || len(code) > 4096 || r.URL.Query().Get("error") != "" {
		http.Redirect(w, r, "/admin?error=cancelled", http.StatusSeeOther)
		return
	}
	user, err := a.identify(r, code)
	if err != nil {
		http.Redirect(w, r, "/admin?error=discord", http.StatusSeeOther)
		return
	}
	if !a.config.AllowedIDs[user.ID] {
		http.Redirect(w, r, "/admin?error=denied", http.StatusSeeOther)
		return
	}
	token := uuid.NewString() + uuid.NewString()
	a.mu.Lock()
	now := time.Now()
	for key, value := range a.sessions {
		if !value.Expires.After(now) {
			delete(a.sessions, key)
		}
	}
	if old, err := r.Cookie(adminCookieName); err == nil {
		delete(a.sessions, old.Value)
	}
	if len(a.sessions) >= 1000 {
		a.mu.Unlock()
		http.Redirect(w, r, "/admin?error=busy", http.StatusSeeOther)
		return
	}
	a.sessions[token] = adminSession{User: user, Expires: now.Add(adminSessionLifetime)}
	a.mu.Unlock()
	http.SetCookie(w, s.adminCookie(adminCookieName, token, adminSessionLifetime))
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (a *adminService) identify(r *http.Request, code string) (discordUser, error) {
	form := url.Values{"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {a.config.RedirectURI}}
	request, err := http.NewRequestWithContext(r.Context(), "POST", discordTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return discordUser{}, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.SetBasicAuth(a.config.ClientID, a.config.ClientSecret)
	var token struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
	}
	if err := a.discordJSON(request, &token); err != nil {
		return discordUser{}, err
	}
	if token.AccessToken == "" || !strings.EqualFold(token.TokenType, "Bearer") {
		return discordUser{}, fmt.Errorf("invalid token response")
	}
	request, err = http.NewRequestWithContext(r.Context(), "GET", discordUserURL, nil)
	if err != nil {
		return discordUser{}, err
	}
	request.Header.Set("Authorization", "Bearer "+token.AccessToken)
	var user discordUser
	if err := a.discordJSON(request, &user); err != nil {
		return user, err
	}
	if !validDiscordID(user.ID) || user.Username == "" {
		return user, fmt.Errorf("invalid Discord user")
	}
	return user, nil
}

func (a *adminService) discordJSON(request *http.Request, target any) error {
	response, err := a.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("Discord request failed")
	}
	return json.NewDecoder(io.LimitReader(response.Body, 65536)).Decode(target)
}

func (s *server) currentAdmin(r *http.Request) (discordUser, bool) {
	a := s.admin
	if !a.enabled() {
		return discordUser{}, false
	}
	cookie, err := r.Cookie(adminCookieName)
	if err != nil {
		return discordUser{}, false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	value, ok := a.sessions[cookie.Value]
	if !ok || !value.Expires.After(time.Now()) || !a.config.AllowedIDs[value.User.ID] {
		delete(a.sessions, cookie.Value)
		return discordUser{}, false
	}
	return value.User, true
}

func (s *server) adminStatus(w http.ResponseWriter, r *http.Request) {
	user, ok := s.currentAdmin(r)
	var current *discordUser
	if ok {
		current = &user
	}
	writeJSON(w, 200, map[string]any{"configured": s.admin.enabled(), "user": current})
}

func (s *server) adminLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(adminCookieName); err == nil {
		s.admin.mu.Lock()
		delete(s.admin.sessions, cookie.Value)
		s.admin.mu.Unlock()
	}
	http.SetCookie(w, s.adminCookie(adminCookieName, "", -time.Second))
	w.WriteHeader(http.StatusNoContent)
}
