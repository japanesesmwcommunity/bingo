package main

import (
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
)

func (s *server) roomSession(token, roomID string) (session, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	value, ok := s.sessions[token]
	if ok && !value.Expires.After(time.Now()) {
		delete(s.sessions, token)
		ok = false
	}
	return value, ok && value.RoomID == roomID
}

func (s *server) subscribeRoom(roomID string) (chan struct{}, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.subscribers[roomID]) >= maxRoomSubscribers {
		return nil, false
	}
	if s.subscribers[roomID] == nil {
		s.subscribers[roomID] = make(map[chan struct{}]struct{})
	}
	updates := make(chan struct{}, 1)
	s.subscribers[roomID][updates] = struct{}{}
	return updates, true
}

func (s *server) unsubscribeRoom(roomID string, updates chan struct{}) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.subscribers[roomID], updates)
	if len(s.subscribers[roomID]) == 0 {
		delete(s.subscribers, roomID)
	}
}

// Slow clients keep one pending notification. The writer always reads the
// latest room snapshot, so the request handler never waits for a client.
func (s *server) notifyRoom(roomID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for updates := range s.subscribers[roomID] {
		select {
		case updates <- struct{}{}:
		default:
		}
	}
}

func (s *server) websocketOrigin(r *http.Request) bool {
	origin, err := url.Parse(r.Header.Get("Origin"))
	if err != nil || origin.Host != r.Host || r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		return false
	}
	scheme := "http"
	if r.TLS != nil || s.secureCookie {
		scheme = "https"
	}
	return origin.Scheme == scheme
}

func (s *server) roomEvents(w http.ResponseWriter, r *http.Request) {
	// Public readers need no session. Only same-origin participant connections
	// receive personal session identity and session-expiry notifications.
	if _, err := r.Cookie("bingo_session"); err != nil || !s.websocketOrigin(r) {
		s.spectatorEvents(w, r)
		return
	}
	_, _, ok := s.authenticate(w, r)
	if !ok {
		return
	}
	cookie, _ := r.Cookie("bingo_session")
	s.streamRoom(w, r, s.websocketOrigin, func(connection *websocket.Conn) bool {
		return s.sendRoomSnapshot(connection, cookie.Value, r.PathValue("id"))
	})
}

func (s *server) streamRoom(w http.ResponseWriter, r *http.Request, checkOrigin func(*http.Request) bool, sendSnapshot func(*websocket.Conn) bool) {
	roomID := r.PathValue("id")
	updates, ok := s.subscribeRoom(roomID)
	if !ok {
		writeError(w, http.StatusTooManyRequests, "このルームへの接続数が多すぎます")
		return
	}
	defer s.unsubscribeRoom(roomID, updates)

	upgrader := websocket.Upgrader{
		HandshakeTimeout: socketWriteTimeout,
		CheckOrigin:      checkOrigin,
	}
	connection, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer connection.Close()
	connection.SetReadLimit(1024)
	_ = connection.SetReadDeadline(time.Now().Add(socketReadTimeout))
	connection.SetPongHandler(func(string) error {
		return connection.SetReadDeadline(time.Now().Add(socketReadTimeout))
	})

	disconnected := make(chan struct{})
	go readRoomSocket(connection, disconnected)

	ticker := time.NewTicker(roomSyncInterval)
	defer ticker.Stop()
	ping := time.NewTicker(socketPingInterval)
	defer ping.Stop()

	for {
		if !sendSnapshot(connection) {
			return
		}
		select {
		case <-disconnected:
			return
		case <-r.Context().Done():
			return
		case <-updates:
		case <-ticker.C:
		case <-ping.C:
			if err := connection.WriteControl(websocket.PingMessage, nil, time.Now().Add(socketWriteTimeout)); err != nil {
				return
			}
		}
	}
}

// Progress updates use the authenticated HTTP API. The socket is read-only;
// reading control frames is still required for disconnect and pong handling.
func readRoomSocket(connection *websocket.Conn, disconnected chan<- struct{}) {
	defer close(disconnected)
	for {
		if _, _, err := connection.ReadMessage(); err != nil {
			return
		}
	}
}

func (s *server) sendRoomSnapshot(connection *websocket.Conn, token, roomID string) bool {
	value, valid := s.roomSession(token, roomID)
	message := roomEvent{Type: "error", Error: "参加セッションが無効です。合言葉を入力して参加してください。"}

	if valid {
		game, err := s.rooms.GetRoom(roomID)
		if err == nil && game.CheckAccess(value.PlayerID) == nil {
			status := game.GetRoomStatus()
			message = roomEvent{Type: "room", Room: &status, PlayerID: value.PlayerID}
		}
	}

	_ = connection.SetWriteDeadline(time.Now().Add(socketWriteTimeout))
	if err := connection.WriteJSON(message); err != nil {
		return false
	}
	if message.Type == "error" {
		_ = connection.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "room access ended"), time.Now().Add(socketWriteTimeout))
		return false
	}
	return true
}
