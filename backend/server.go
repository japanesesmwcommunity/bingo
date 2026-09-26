package main

import (
	"bingo/bingo"
	"bingo/room"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func newServer(secure bool) *server {
	s := &server{
		admin:        newAdminService(adminConfig{}, "bingo.json"),
		rooms:        room.NewRoomManager(),
		sessions:     make(map[string]session),
		attempts:     make(map[string]rateEntry),
		subscribers:  make(map[string]map[chan struct{}]struct{}),
		secureCookie: secure,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /admin", func(w http.ResponseWriter, r *http.Request) { http.ServeFileFS(w, r, webFiles, "static/index.html") })
	mux.HandleFunc("GET /api/admin/login", s.adminLogin)
	mux.HandleFunc("GET /api/admin/callback", s.adminCallback)
	mux.HandleFunc("GET /api/admin/session", s.adminStatus)
	mux.HandleFunc("POST /api/admin/logout", s.adminLogout)
	mux.HandleFunc("GET /api/admin/goals", s.adminGoals)
	mux.HandleFunc("PUT /api/admin/goals", s.saveAdminGoals)
	mux.HandleFunc("GET /api/config", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"defaults": bingo.DefaultOptions(), "maxPlayers": room.MaxPlayers})
	})
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, map[string]string{"status": "ok"}) })
	mux.HandleFunc("GET /create", s.createCard)
	mux.HandleFunc("POST /create", s.createRoom)
	mux.HandleFunc("POST /api/rooms", s.createRoom)
	mux.HandleFunc("GET /api/rooms", s.listRooms)
	mux.HandleFunc("POST /api/rooms/{id}/join", s.joinRoom)
	mux.HandleFunc("GET /api/rooms/{id}", s.spectatorStatus)
	mux.HandleFunc("GET /api/rooms/{id}/session", s.roomStatus)
	mux.HandleFunc("GET /api/rooms/{id}/events", s.roomEvents)
	mux.HandleFunc("GET /api/rooms/{id}/players/{player}", s.playerStatus)
	mux.HandleFunc("DELETE /api/rooms/{id}/players/{player}", s.kickPlayer)
	mux.HandleFunc("POST /api/rooms/{id}/finish", s.finishRoom)
	mux.HandleFunc("POST /api/rooms/{id}/card", s.regenerateCard)
	mux.HandleFunc("PUT /api/rooms/{id}/progress", s.progress)
	mux.HandleFunc("PUT /api/rooms/{id}/bowser", s.bowser)
	mux.HandleFunc("POST /api/rooms/{id}/leave", s.leaveRoom)
	mux.HandleFunc("DELETE /api/rooms/{id}", s.deleteRoom)
	root, _ := fs.Sub(webFiles, "static")
	mux.Handle("GET /", http.FileServer(http.FS(root)))
	s.handler = mux
	return s
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != "GET" && r.Method != "HEAD" {
		// Custom header prevents cross-origin forms; write APIs never grant CORS.
		if r.Header.Get("X-Requested-With") != "bingo" || r.Header.Get("Sec-Fetch-Site") == "cross-site" {
			writeError(w, 403, "同じサイトから操作してください")
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			u, err := url.Parse(origin)
			if err != nil || u.Host != r.Host || (u.Scheme != "http" && u.Scheme != "https") {
				writeError(w, 403, "送信元が一致しません")
				return
			}
		}
	}
	s.handler.ServeHTTP(w, r)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *server) listRooms(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	writeJSON(w, http.StatusOK, map[string]any{"rooms": s.rooms.ListActiveRooms()})
}
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
func domainError(w http.ResponseWriter, err error) {
	status := 400
	switch {
	case errors.Is(err, room.ErrNotFound):
		status = 404
	case errors.Is(err, room.ErrForbidden):
		status = 403
	case errors.Is(err, room.ErrConflict), errors.Is(err, room.ErrFull):
		status = 409
	}
	writeError(w, status, err.Error())
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	return decodeLimit(w, r, v, 4096)
}
func decodeLimit(w http.ResponseWriter, r *http.Request, v any, limit int64) bool {
	if strings.Split(r.Header.Get("Content-Type"), ";")[0] != "application/json" {
		writeError(w, 415, "application/json を指定してください")
		return false
	}
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, limit))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		writeError(w, 400, "JSONの形式が不正です")
		return false
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		writeError(w, 400, "JSONは1つだけ指定してください")
		return false
	}
	return true
}

func (s *server) allowAttempt(w http.ResponseWriter, r *http.Request) bool {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for key, entry := range s.attempts {
		if !entry.Expires.After(now) {
			delete(s.attempts, key)
		}
	}
	entry := s.attempts[ip]
	if entry.Count == 0 {
		entry.Expires = now.Add(time.Minute)
	}
	if entry.Count >= 20 || len(s.attempts) >= 10000 {
		w.Header().Set("Retry-After", "60")
		writeError(w, 429, "少し待ってから再試行してください")
		return false
	}
	entry.Count++
	s.attempts[ip] = entry
	return true
}

func (s *server) setSession(w http.ResponseWriter, roomID, playerID string) {
	token := uuid.NewString() + uuid.NewString()
	s.mu.Lock()
	now := time.Now()
	for key, value := range s.sessions {
		if !value.Expires.After(now) {
			delete(s.sessions, key)
		}
	}
	s.sessions[token] = session{RoomID: roomID, PlayerID: playerID, Expires: now.Add(24 * time.Hour)}
	s.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: "bingo_session", Value: token, Path: "/api/rooms/" + roomID, MaxAge: 86400, HttpOnly: true, Secure: s.secureCookie, SameSite: http.SameSiteStrictMode})
}

func (s *server) authenticate(w http.ResponseWriter, r *http.Request) (*room.Room, string, bool) {
	cookie, err := r.Cookie("bingo_session")
	if err != nil {
		writeError(w, 401, "合言葉を入力して参加してください")
		return nil, "", false
	}
	value, ok := s.roomSession(cookie.Value, r.PathValue("id"))
	if !ok {
		writeError(w, 401, "参加セッションが無効です")
		return nil, "", false
	}
	game, err := s.rooms.GetRoom(value.RoomID)
	if err != nil {
		domainError(w, err)
		return nil, "", false
	}
	if err := game.CheckAccess(value.PlayerID); err != nil {
		domainError(w, err)
		return nil, "", false
	}
	return game, value.PlayerID, true
}

func (s *server) createRoom(w http.ResponseWriter, r *http.Request) {
	if !s.allowAttempt(w, r) {
		return
	}
	request := room.CreateOptions{Options: bingo.DefaultOptions()}
	if !decode(w, r, &request) {
		return
	}
	game, id, err := s.rooms.CreateRoom(request)
	if err != nil {
		domainError(w, err)
		return
	}
	status := game.GetRoomStatus()
	s.setSession(w, status.ID, id)
	writeJSON(w, 201, map[string]any{"room": status, "playerId": id})
}
func (s *server) joinRoom(w http.ResponseWriter, r *http.Request) {
	var existing session
	hasSession := false
	if cookie, err := r.Cookie("bingo_session"); err == nil {
		existing, hasSession = s.roomSession(cookie.Value, r.PathValue("id"))
	}
	if !hasSession && !s.allowAttempt(w, r) {
		return
	}
	var request playerRequest
	if !decode(w, r, &request) {
		return
	}
	game, err := s.rooms.GetRoom(r.PathValue("id"))
	if err != nil {
		domainError(w, err)
		return
	}
	// Reusing an existing room session is idempotent and does not consume a seat.
	if hasSession {
		value := existing
		if value.PlayerID == game.GetRoomStatus().OwnerID {
			if err := game.JoinOwner(value.PlayerID, request.PlayerName, request.Color); err != nil {
				domainError(w, err)
				return
			}
			s.notifyRoom(r.PathValue("id"))
			writeJSON(w, 200, map[string]any{"room": game.GetRoomStatus(), "playerId": value.PlayerID})
			return
		}
		if _, err := game.GetPlayer(value.PlayerID); err == nil {
			writeJSON(w, 200, map[string]any{"room": game.GetRoomStatus(), "playerId": value.PlayerID})
			return
		}
	}
	if hasSession && !s.allowAttempt(w, r) {
		return
	}
	id, err := game.AddPlayer(request.Passphrase, request.PlayerName, request.Color)
	if err != nil {
		domainError(w, err)
		return
	}
	s.setSession(w, r.PathValue("id"), id)
	s.notifyRoom(r.PathValue("id"))
	writeJSON(w, 200, map[string]any{"room": game.GetRoomStatus(), "playerId": id})
}
func (s *server) roomStatus(w http.ResponseWriter, r *http.Request) {
	game, id, ok := s.authenticate(w, r)
	if !ok {
		return
	}
	writeJSON(w, 200, map[string]any{"room": game.GetRoomStatus(), "playerId": id})
}
func (s *server) playerStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	game, err := s.rooms.GetRoom(r.PathValue("id"))
	if err != nil {
		domainError(w, err)
		return
	}
	player, err := game.GetPlayer(r.PathValue("player"))
	if err != nil {
		domainError(w, err)
		return
	}
	writeJSON(w, 200, player)
}
func (s *server) kickPlayer(w http.ResponseWriter, r *http.Request) {
	game, ownerID, ok := s.authenticate(w, r)
	if !ok {
		return
	}
	playerID := r.PathValue("player")
	if err := game.KickPlayer(ownerID, playerID); err != nil {
		domainError(w, err)
		return
	}
	s.mu.Lock()
	for token, value := range s.sessions {
		if value.RoomID == r.PathValue("id") && value.PlayerID == playerID {
			delete(s.sessions, token)
		}
	}
	s.mu.Unlock()
	s.notifyRoom(r.PathValue("id"))
	writeJSON(w, 200, game.GetRoomStatus())
}

func (s *server) finishRoom(w http.ResponseWriter, r *http.Request) {
	game, id, ok := s.authenticate(w, r)
	if !ok {
		return
	}
	if err := game.Finish(id); err != nil {
		domainError(w, err)
		return
	}
	s.notifyRoom(r.PathValue("id"))
	writeJSON(w, 200, game.GetRoomStatus())
}
func (s *server) regenerateCard(w http.ResponseWriter, r *http.Request) {
	game, id, ok := s.authenticate(w, r)
	if !ok {
		return
	}
	status := game.GetRoomStatus()
	if id != status.OwnerID {
		domainError(w, room.ErrForbidden)
		return
	}
	request := cardRequest{MaxTime: status.Options.MaxTime, MinTarget: status.Options.MinTarget, Rule: status.Options.Rule}
	if !decode(w, r, &request) {
		return
	}
	if err := game.GenerateCard(request.Seed, request.MaxTime, request.MinTarget, request.Rule); err != nil {
		domainError(w, err)
		return
	}
	s.notifyRoom(r.PathValue("id"))
	writeJSON(w, 200, game.GetRoomStatus())
}
func (s *server) progress(w http.ResponseWriter, r *http.Request) {
	game, id, ok := s.authenticate(w, r)
	if !ok {
		return
	}
	var request progressRequest
	if !decode(w, r, &request) {
		return
	}
	if request.Index == nil || request.Completed == nil {
		writeError(w, 400, "index と completed が必要です")
		return
	}
	if err := game.UpdatePlayerProgress(id, *request.Index, *request.Completed); err != nil {
		domainError(w, err)
		return
	}
	s.notifyRoom(r.PathValue("id"))
	writeJSON(w, 200, game.GetRoomStatus())
}
func (s *server) bowser(w http.ResponseWriter, r *http.Request) {
	game, id, ok := s.authenticate(w, r)
	if !ok {
		return
	}
	var request struct {
		Completed *bool `json:"completed"`
	}
	if !decode(w, r, &request) {
		return
	}
	if request.Completed == nil {
		writeError(w, 400, "completed が必要です")
		return
	}
	if err := game.UpdateBowser(id, *request.Completed); err != nil {
		domainError(w, err)
		return
	}
	s.notifyRoom(r.PathValue("id"))
	writeJSON(w, 200, game.GetRoomStatus())
}
func (s *server) revoke(w http.ResponseWriter, r *http.Request, all bool) {
	cookie, _ := r.Cookie("bingo_session")
	s.mu.Lock()
	for token, value := range s.sessions {
		if value.RoomID == r.PathValue("id") && (all || cookie != nil && cookie.Value == token) {
			delete(s.sessions, token)
		}
	}
	s.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: "bingo_session", Path: "/api/rooms/" + r.PathValue("id"), MaxAge: -1, HttpOnly: true, Secure: s.secureCookie, SameSite: http.SameSiteStrictMode})
}
func (s *server) leaveRoom(w http.ResponseWriter, r *http.Request) {
	game, id, ok := s.authenticate(w, r)
	if !ok {
		return
	}
	if err := game.DeletePlayer(id); err != nil {
		domainError(w, err)
		return
	}
	if id == game.GetRoomStatus().OwnerID {
		s.notifyRoom(r.PathValue("id"))
		writeJSON(w, 200, game.GetRoomStatus())
		return
	}
	s.revoke(w, r, false)
	s.notifyRoom(r.PathValue("id"))
	w.WriteHeader(204)
}
func (s *server) deleteRoom(w http.ResponseWriter, r *http.Request) {
	_, id, ok := s.authenticate(w, r)
	if !ok {
		return
	}
	if err := s.rooms.DeleteRoom(r.PathValue("id"), id); err != nil {
		domainError(w, err)
		return
	}
	s.revoke(w, r, true)
	s.notifyRoom(r.PathValue("id"))
	w.WriteHeader(204)
}
func (s *server) createCard(w http.ResponseWriter, r *http.Request) {
	if !s.allowAttempt(w, r) {
		return
	}
	o := bingo.DefaultOptions()
	for name, pointer := range map[string]*int{"maxTime": &o.MaxTime, "minTarget": &o.MinTarget, "baseRoute": &o.BaseRoute} {
		if value := r.URL.Query().Get(name); value != "" {
			n, err := strconv.Atoi(value)
			if err != nil {
				writeError(w, 400, fmt.Sprintf("%s は整数で指定してください", name))
				return
			}
			*pointer = n
		}
	}
	if value := r.URL.Query().Get("rule"); value != "" {
		o.Rule = bingo.Rule(value)
	}
	card, err := bingo.CreateCard(r.URL.Query().Get("seed"), o)
	if err != nil {
		domainError(w, err)
		return
	}
	writeJSON(w, 200, card)
}
