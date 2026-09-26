package main

import (
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

func (s *server) spectatorStatus(w http.ResponseWriter, r *http.Request) {
	// Only this public, read-only response is available to arbitrary origins.
	w.Header().Set("Access-Control-Allow-Origin", "*")
	game, err := s.rooms.GetRoom(r.PathValue("id"))
	if err != nil {
		domainError(w, err)
		return
	}
	snapshot := game.GetRoomStatus()
	writeJSON(w, http.StatusOK, spectatorEvent{Type: "room", Room: &snapshot})
}

func (s *server) spectatorEvents(w http.ResponseWriter, r *http.Request) {
	roomID := r.PathValue("id")
	if _, err := s.rooms.GetRoom(roomID); err != nil {
		domainError(w, err)
		return
	}
	// Spectator streams are public and never authorize writes, regardless of origin.
	s.streamRoom(w, r, func(*http.Request) bool { return true }, func(connection *websocket.Conn) bool {
		return s.sendSpectatorSnapshot(connection, roomID)
	})
}

func (s *server) sendSpectatorSnapshot(connection *websocket.Conn, roomID string) bool {
	message := spectatorEvent{Type: "error", Error: "ルームが削除されました"}
	if game, err := s.rooms.GetRoom(roomID); err == nil {
		snapshot := game.GetRoomStatus()
		message = spectatorEvent{Type: "room", Room: &snapshot}
	}
	_ = connection.SetWriteDeadline(time.Now().Add(socketWriteTimeout))
	if err := connection.WriteJSON(message); err != nil {
		return false
	}
	if message.Type == "error" {
		_ = connection.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "spectator access ended"), time.Now().Add(socketWriteTimeout))
		return false
	}
	return true
}
