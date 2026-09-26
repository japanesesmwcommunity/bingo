package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestSpectatorPublicReadOnly(t *testing.T) {
	s := testServer(t)
	host, cookie := createTestRoom(t, s, "standard")
	id := host.Room.ID
	path := "/api/rooms/" + id
	view := "/api/rooms/" + id
	joinTestRoom(t, s, id, "guest")
	r := httptest.NewRequest("GET", view, nil)
	r.Header.Set("Origin", "https://nodecg.example")
	r.Header.Set("Sec-Fetch-Site", "cross-site")
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != 200 || w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("public cross-origin read failed: %d", w.Code)
	}
	var event spectatorEvent
	if err := json.Unmarshal(w.Body.Bytes(), &event); err != nil {
		t.Fatal(err)
	}
	if event.Type != "room" || event.Room.ID != id || len(event.Room.Card.Goals) != 25 || len(event.Room.Players) != 2 || event.Room.Options.Rule != "standard" {
		t.Fatalf("wrong snapshot: %+v", event)
	}
	for i, goal := range event.Room.Card.Goals {
		original := host.Room.Card.Goals[i]
		if goal.Name != original.Name || goal.World != original.World || goal.Level != original.Level {
			t.Fatal("goal order or text changed")
		}
	}
	for _, field := range []string{"playerId", "passphrase", "passhash"} {
		if strings.Contains(w.Body.String(), field) {
			t.Fatalf("unexpected field: %s", field)
		}
	}
	if w.Header().Get("Cache-Control") != "no-store" || len(w.Result().Cookies()) != 0 || w.Header().Get("Access-Control-Allow-Credentials") != "" {
		t.Fatal("unexpected cache or credentials")
	}
	for _, action := range []struct{ method, suffix string }{
		{"GET", "/session"}, {"POST", "/finish"}, {"POST", "/card"},
		{"PUT", "/progress"}, {"PUT", "/bowser"}, {"DELETE", ""},
	} {
		response := request(t, s, action.method, path+action.suffix, nil, nil, 401)
		if response.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Fatal("participant API granted CORS")
		}
	}
	// Public data and participant data differ only in personal session identity.
	private := request(t, s, "GET", path+"/session", nil, cookie, 200)
	var participant responseData
	if err := json.Unmarshal(private.Body.Bytes(), &participant); err != nil {
		t.Fatal(err)
	}
	if participant.PlayerID != host.PlayerID || !reflect.DeepEqual(*event.Room, participant.Room) {
		t.Fatal("public and participant room data differ")
	}
	for _, endpoint := range []string{"/api/rooms", path + "/players/" + host.PlayerID} {
		r := httptest.NewRequest("GET", endpoint, nil)
		r.Header.Set("Origin", "https://nodecg.example")
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		if w.Code != 200 || w.Header().Get("Access-Control-Allow-Origin") != "*" {
			t.Fatalf("public read failed: %s", endpoint)
		}
	}
	request(t, s, "GET", path+"/players/missing", nil, nil, 404)
	request(t, s, "GET", "/api/rooms/missing/players/missing", nil, nil, 404)
	request(t, s, "GET", "/api/rooms/missing", nil, nil, 404)
	request(t, s, "DELETE", path, nil, cookie, 204)
	request(t, s, "GET", view, nil, nil, 404)
}

func dialSpectator(t *testing.T, base, id string, headers http.Header) *websocket.Conn {
	t.Helper()
	connection, response, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(base, "http")+"/api/rooms/"+id+"/events", headers)
	if err != nil {
		t.Fatalf("handshake: %v (%v)", err, response)
	}
	t.Cleanup(func() { _ = connection.Close() })
	return connection
}

func readSpectatorEvent(t *testing.T, connection *websocket.Conn) spectatorEvent {
	t.Helper()
	_ = connection.SetReadDeadline(time.Now().Add(3 * time.Second))
	var event spectatorEvent
	if err := connection.ReadJSON(&event); err != nil {
		t.Fatal(err)
	}
	return event
}

func awaitSpectatorEvent(t *testing.T, connection *websocket.Conn, predicate func(spectatorEvent) bool) spectatorEvent {
	t.Helper()
	for i := 0; i < 10; i++ {
		event := readSpectatorEvent(t, connection)
		if predicate(event) {
			return event
		}
	}
	t.Fatal("expected spectator event was not received")
	return spectatorEvent{}
}

func TestSpectatorStreamUpdatesAndDeletion(t *testing.T) {
	s := testServer(t)
	host, cookie := createTestRoom(t, s, "line")
	id, path := host.Room.ID, "/api/rooms/"+host.Room.ID
	base := socketServer(t, s)
	connection := dialSpectator(t, base, id, nil)
	first := readSpectatorEvent(t, connection)
	if first.Type != "room" || first.Room.ID != id {
		t.Fatal("missing initial snapshot")
	}
	if event := readSpectatorEvent(t, connection); event.Room.Version != first.Room.Version {
		t.Fatal("missing periodic snapshot")
	}
	for i := 0; i < 5; i++ {
		request(t, s, "PUT", path+"/progress", map[string]any{"index": i, "completed": true}, cookie, 200)
	}
	winning := awaitSpectatorEvent(t, connection, func(event spectatorEvent) bool { return event.Room != nil && event.Room.WinnerID == host.PlayerID })
	if winning.Room.FinishedAt == nil || !winning.Room.Players[0].HasLine || !winning.Room.Players[0].Progress[4] {
		t.Fatal("incomplete winning state")
	}
	reconnected := dialSpectator(t, base, id, http.Header{"Origin": {"https://nodecg.example"}, "Sec-Fetch-Site": {"cross-site"}})
	if readSpectatorEvent(t, reconnected).Room.WinnerID != host.PlayerID {
		t.Fatal("stale reconnect")
	}
	request(t, s, "GET", "/api/rooms/"+id, nil, nil, 200)
	request(t, s, "DELETE", path, nil, cookie, 204)
	event := awaitSpectatorEvent(t, connection, func(event spectatorEvent) bool { return event.Type == "error" })
	if event.Room != nil {
		t.Fatal("deleted room data in error")
	}
	_, _, err := connection.ReadMessage()
	if !websocket.IsCloseError(err, websocket.ClosePolicyViolation) {
		t.Fatalf("expected policy close: %v", err)
	}
}

func TestSpectatorHandshakeAndReadOnly(t *testing.T) {
	s := testServer(t)
	host, _ := createTestRoom(t, s, "line")
	id := host.Room.ID
	base := socketServer(t, s)
	prefix := "ws" + strings.TrimPrefix(base, "http") + "/api/rooms/"
	_, response, err := websocket.DefaultDialer.Dial(prefix+"missing/events", nil)
	if response != nil {
		_ = response.Body.Close()
	}
	if err == nil || response == nil || response.StatusCode != 404 {
		t.Fatal("missing room accepted")
	}
	connection := dialSpectator(t, base, id, nil)
	_ = readSpectatorEvent(t, connection)
	if err := connection.WriteJSON(map[string]any{"index": 0, "completed": true, "playerId": host.PlayerID}); err != nil {
		t.Fatal(err)
	}
	if event := readSpectatorEvent(t, connection); event.Room.Players[0].Progress[0] {
		t.Fatal("spectator changed progress")
	}
	for i := 1; i < maxRoomSubscribers; i++ {
		updates, ok := s.subscribeRoom(id)
		if !ok {
			t.Fatal("unexpected limit")
		}
		t.Cleanup(func() { s.unsubscribeRoom(id, updates) })
	}
	_, response, err = websocket.DefaultDialer.Dial(prefix+id+"/events", nil)
	if response != nil {
		_ = response.Body.Close()
	}
	if err == nil || response == nil || response.StatusCode != 429 {
		t.Fatal("missing shared connection limit")
	}
}
