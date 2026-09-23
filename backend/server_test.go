package main

import (
	"bingo/bingo"
	"bingo/room"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

type responseData struct {
	Room     room.Status `json:"room"`
	PlayerID string      `json:"playerId"`
}

func testServer(t *testing.T) *server {
	t.Helper()
	if err := bingo.InitData("bingo.json"); err != nil {
		t.Fatal(err)
	}
	return newServer(false)
}
func request(t *testing.T, s *server, method, path string, body any, cookie *http.Cookie, want int) *httptest.ResponseRecorder {
	t.Helper()
	var data []byte
	if body != nil {
		var err error
		data, err = json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
	}
	r := httptest.NewRequest(method, path, bytes.NewReader(data))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-Requested-With", "bingo")
	if cookie != nil {
		r.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != want {
		t.Fatalf("%s %s: got %d want %d: %s", method, path, w.Code, want, w.Body.String())
	}
	return w
}
func createTestRoom(t *testing.T, s *server, rule string) (responseData, *http.Cookie) {
	t.Helper()
	w := request(t, s, "POST", "/api/rooms", map[string]any{"name": "room", "passphrase": "secret", "rule": rule}, nil, 201)
	var data responseData
	if err := json.Unmarshal(w.Body.Bytes(), &data); err != nil {
		t.Fatal(err)
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatal("session cookie")
	}
	if strings.Contains(w.Body.String(), "secret") || strings.Contains(w.Body.String(), "passhash") {
		t.Fatal("credential leak")
	}
	if len(data.Room.Players) != 0 {
		t.Fatal("creation must not enroll a player")
	}
	w = request(t, s, "POST", "/api/rooms/"+data.Room.ID+"/join", map[string]string{"playerName": "host"}, cookies[0], 200)
	if err := json.Unmarshal(w.Body.Bytes(), &data); err != nil {
		t.Fatal(err)
	}
	return data, cookies[0]
}
func joinTestRoom(t *testing.T, s *server, id, name string) (responseData, *http.Cookie) {
	t.Helper()
	w := request(t, s, "POST", "/api/rooms/"+id+"/join", map[string]string{"passphrase": "secret", "playerName": name}, nil, 200)
	var data responseData
	if err := json.Unmarshal(w.Body.Bytes(), &data); err != nil {
		t.Fatal(err)
	}
	return data, w.Result().Cookies()[0]
}
func TestAPIEntireGame(t *testing.T) {
	s := testServer(t)
	host, hc := createTestRoom(t, s, "standard")
	id := host.Room.ID
	path := "/api/rooms/" + id
	guest, gc := joinTestRoom(t, s, id, "guest")
	request(t, s, "GET", path, nil, nil, 401)
	request(t, s, "GET", path, nil, hc, 200)
	request(t, s, "GET", path+"/players/"+guest.PlayerID, nil, hc, 200)
	request(t, s, "GET", path+"/players/missing", nil, hc, 404)
	request(t, s, "POST", path+"/start", nil, gc, 403)
	request(t, s, "POST", path+"/card", map[string]string{"seed": "preview"}, gc, 403)
	request(t, s, "POST", path+"/card", map[string]string{"seed": "preview"}, hc, 200)
	request(t, s, "PUT", path+"/progress", map[string]any{"index": 0, "completed": true}, hc, 409)
	request(t, s, "POST", path+"/start", nil, hc, 200)
	request(t, s, "POST", path+"/start", nil, hc, 409)
	request(t, s, "POST", path+"/card", map[string]string{"seed": "late"}, hc, 409)
	request(t, s, "POST", path+"/join", map[string]string{"passphrase": "secret", "playerName": "late"}, nil, 409)
	request(t, s, "POST", path+"/leave", nil, gc, 409)
	request(t, s, "DELETE", path, nil, hc, 409)
	for i := 0; i < 5; i++ {
		request(t, s, "PUT", path+"/progress", map[string]any{"index": i, "completed": true}, hc, 200)
	}
	w := request(t, s, "GET", path, nil, hc, 200)
	var before responseData
	_ = json.Unmarshal(w.Body.Bytes(), &before)
	if before.Room.WinnerID != "" {
		t.Fatal("early win")
	}
	w = request(t, s, "PUT", path+"/bowser", map[string]bool{"completed": true}, hc, 200)
	var status room.Status
	_ = json.Unmarshal(w.Body.Bytes(), &status)
	if status.WinnerID != host.PlayerID || status.FinishedAt == nil {
		t.Fatal("missing winner")
	}
	request(t, s, "PUT", path+"/progress", map[string]any{"index": 0, "completed": true}, gc, 409)
	request(t, s, "DELETE", path, nil, gc, 403)
	request(t, s, "DELETE", path, nil, hc, 204)
	request(t, s, "GET", path, nil, gc, 401)
}
func TestAPIJoinCapacityAndLeave(t *testing.T) {
	s := testServer(t)
	host, hc := createTestRoom(t, s, "line")
	path := "/api/rooms/" + host.Room.ID
	request(t, s, "POST", path+"/join", map[string]string{"passphrase": "wrong", "playerName": "guest"}, nil, 403)
	request(t, s, "POST", "/api/rooms/missing/join", map[string]string{"passphrase": "secret", "playerName": "guest"}, nil, 404)
	_, gc := joinTestRoom(t, s, host.Room.ID, "guest")
	request(t, s, "POST", path+"/join", map[string]string{"passphrase": "secret", "playerName": "guest"}, gc, 200)
	joinTestRoom(t, s, host.Room.ID, "third")
	joinTestRoom(t, s, host.Room.ID, "fourth")
	request(t, s, "POST", path+"/join", map[string]string{"passphrase": "secret", "playerName": "fifth"}, nil, 409)
	request(t, s, "POST", path+"/leave", nil, gc, 204)
	request(t, s, "GET", path, nil, gc, 401)
	joinTestRoom(t, s, host.Room.ID, "replacement")
	request(t, s, "DELETE", path, nil, hc, 204)
}
func TestAPIValidationAndIsolation(t *testing.T) {
	s := testServer(t)
	host, hc := createTestRoom(t, s, "standard")
	path := "/api/rooms/" + host.Room.ID
	other, oc := createTestRoom(t, s, "standard")
	request(t, s, "GET", path, nil, oc, 401)
	request(t, s, "GET", "/api/rooms/"+other.Room.ID, nil, hc, 401)
	request(t, s, "POST", "/api/rooms", map[string]string{"name": "bad"}, nil, 400)
	request(t, s, "POST", path+"/card", map[string]int{"maxTime": 1}, hc, 400)
	request(t, s, "POST", path+"/start", nil, hc, 200)
	for _, payload := range []any{map[string]bool{"completed": true}, map[string]int{"index": 0}, map[string]any{"index": 25, "completed": true}, map[string]any{"index": -1, "completed": true}, map[string]any{"index": 0, "completed": true, "playerId": "someone"}} {
		request(t, s, "PUT", path+"/progress", payload, hc, 400)
	}
	request(t, s, "PUT", path+"/bowser", map[string]bool{}, hc, 400)
	request(t, s, "PUT", path+"/bowser", map[string]bool{"completed": true}, hc, 200)
	request(t, s, "PUT", path+"/bowser", map[string]bool{"completed": false}, hc, 200)
	request(t, s, "GET", path, nil, &http.Cookie{Name: "bingo_session", Value: "forged"}, 401)
	s.mu.Lock()
	expired := s.sessions[hc.Value]
	expired.Expires = time.Now().Add(-time.Second)
	s.sessions[hc.Value] = expired
	s.mu.Unlock()
	request(t, s, "GET", path, nil, hc, 401)
	s.mu.Lock()
	if _, ok := s.sessions[hc.Value]; ok {
		t.Fatal("expired session retained")
	}
	s.mu.Unlock()
}
func TestAPIRequestGuards(t *testing.T) {
	s := testServer(t)
	for _, test := range []struct {
		body, content, origin, requested, site string
		status                                 int
	}{
		{`{}`, "application/json", "", "", "", 403},
		{`{}`, "application/json", "https://evil.example", "bingo", "", 403},
		{`{}`, "application/json", "null", "bingo", "", 403},
		{`{}`, "application/json", "", "bingo", "cross-site", 403},
		{`{}`, "text/plain", "", "bingo", "", 415},
		{`{`, "application/json", "", "bingo", "", 400},
		{`{} {}`, "application/json", "", "bingo", "", 400},
		{`{"unknown":true}`, "application/json", "", "bingo", "", 400},
		{`{"name":"` + strings.Repeat("x", 5000) + `"}`, "application/json", "", "bingo", "", 400},
	} {
		r := httptest.NewRequest("POST", "/api/rooms", strings.NewReader(test.body))
		r.Header.Set("Content-Type", test.content)
		r.Header.Set("Origin", test.origin)
		r.Header.Set("X-Requested-With", test.requested)
		r.Header.Set("Sec-Fetch-Site", test.site)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		if w.Code != test.status {
			t.Fatal(test, w.Code)
		}
	}
	r := httptest.NewRequest("POST", "http://example.com/api/rooms", strings.NewReader(`{"name":"room","passphrase":"secret"}`))
	r.Header.Set("Content-Type", "application/json; charset=utf-8")
	r.Header.Set("X-Requested-With", "bingo")
	r.Header.Set("Origin", "http://example.com")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
}
func TestPublicEndpointsAndRateLimit(t *testing.T) {
	s := testServer(t)
	for _, path := range []string{"/", "/api/config", "/health"} {
		w := request(t, s, "GET", path, nil, nil, 200)
		if w.Header().Get("Content-Security-Policy") == "" {
			t.Fatal("missing security headers")
		}
	}
	page := request(t, s, "GET", "/", nil, nil, 200)
	assets := regexp.MustCompile(`(?:src|href)="(/assets/[^"]+)"`).FindAllStringSubmatch(page.Body.String(), -1)
	if len(assets) < 2 || !strings.Contains(page.Body.String(), `id="root"`) {
		t.Fatal("missing React entry point or built assets")
	}
	for _, asset := range assets {
		w := request(t, s, "GET", asset[1], nil, nil, 200)
		if w.Body.Len() == 0 {
			t.Fatal("empty frontend asset", asset[1])
		}
	}
	request(t, s, "GET", "/src/App.tsx", nil, nil, 404)
	request(t, s, "GET", "/missing", nil, nil, 404)
	request(t, s, "GET", "/create?seed=repeat", nil, nil, 200)
	request(t, s, "GET", "/create?seed=repeat&maxTime=90&minTarget=0&baseRoute=0&rule=line", nil, nil, 200)
	request(t, s, "GET", "/create?maxTime=abc", nil, nil, 400)
	request(t, s, "GET", "/create?rule=invalid", nil, nil, 400)
	request(t, s, "GET", "/create?maxTime=1", nil, nil, 400)
	s = testServer(t)
	for i := 0; i < 20; i++ {
		request(t, s, "POST", "/api/rooms/missing/join", map[string]string{}, nil, 404)
	}
	w := request(t, s, "POST", "/api/rooms/missing/join", map[string]string{}, nil, 429)
	if w.Header().Get("Retry-After") != "60" {
		t.Fatal("retry header")
	}
	s.mu.Lock()
	for ip, entry := range s.attempts {
		entry.Expires = time.Now().Add(-time.Second)
		s.attempts[ip] = entry
	}
	s.mu.Unlock()
	request(t, s, "POST", "/api/rooms/missing/join", map[string]string{}, nil, 404)
}
func TestSessionPruningAndMissingMember(t *testing.T) {
	s := testServer(t)
	s.secureCookie = true
	s.sessions["expired"] = session{Expires: time.Now().Add(-time.Second)}
	host, cookie := createTestRoom(t, s, "line")
	if !cookie.Secure {
		t.Fatal("secure cookie")
	}
	if _, ok := s.sessions["expired"]; ok {
		t.Fatal("old session")
	}
	_, gc := joinTestRoom(t, s, host.Room.ID, "guest")
	game, _ := s.rooms.GetRoom(host.Room.ID)
	value := s.sessions[gc.Value]
	if err := game.DeletePlayer(value.PlayerID); err != nil {
		t.Fatal(err)
	}
	request(t, s, "GET", "/api/rooms/"+host.Room.ID, nil, gc, 404)
	if err := s.rooms.DeleteRoom(host.Room.ID, host.PlayerID); err != nil {
		t.Fatal(err)
	}
	request(t, s, "GET", "/api/rooms/"+host.Room.ID, nil, cookie, 404)
}
func TestConcurrentAPI(t *testing.T) {
	s := testServer(t)
	host, cookie := createTestRoom(t, s, "standard")
	path := "/api/rooms/" + host.Room.ID
	request(t, s, "POST", path+"/start", nil, cookie, 200)
	var wg sync.WaitGroup
	for i := 0; i < 25; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			request(t, s, "PUT", path+"/progress", map[string]any{"index": i, "completed": true}, cookie, 200)
			request(t, s, "GET", path, nil, cookie, 200)
		}(i)
	}
	wg.Wait()
	request(t, s, "PUT", path+"/bowser", map[string]bool{"completed": true}, cookie, 200)
}
func TestDecodeErrorsOnAllMutations(t *testing.T) {
	s := testServer(t)
	host, cookie := createTestRoom(t, s, "standard")
	path := "/api/rooms/" + host.Room.ID
	for _, endpoint := range []struct{ method, suffix string }{{"POST", "/join"}, {"POST", "/card"}, {"PUT", "/progress"}, {"PUT", "/bowser"}} {
		request(t, s, endpoint.method, path+endpoint.suffix, map[string]string{"unexpected": "field"}, cookie, 400)
		request(t, s, endpoint.method, path+endpoint.suffix, nil, nil, func() int {
			if endpoint.suffix == "/join" {
				return 400
			}
			return 401
		}())
	}
	for _, endpoint := range []struct{ method, suffix string }{{"POST", "/start"}, {"POST", "/leave"}, {"DELETE", ""}, {"GET", "/players/unknown"}} {
		request(t, s, endpoint.method, path+endpoint.suffix, nil, nil, 401)
	}
}
func TestRateLimitWithoutPort(t *testing.T) {
	s := testServer(t)
	r := httptest.NewRequest("GET", "/create", nil)
	r.RemoteAddr = "local"
	for i := 0; i < 21; i++ {
		w := httptest.NewRecorder()
		allowed := s.allowAttempt(w, r)
		if allowed != (i < 20) {
			t.Fatal(fmt.Sprint(i))
		}
	}
}
