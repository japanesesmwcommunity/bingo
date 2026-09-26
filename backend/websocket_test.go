package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func socketServer(t *testing.T, s *server) string {
	t.Helper()
	httpServer := httptest.NewServer(s)
	t.Cleanup(httpServer.Close)
	return httpServer.URL
}

func dialRoom(t *testing.T, base, id string, cookie *http.Cookie) *websocket.Conn {
	t.Helper()
	headers := http.Header{"Origin": []string{base}}
	if cookie != nil {
		headers.Set("Cookie", cookie.Name+"="+cookie.Value)
	}
	connection, response, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(base, "http")+"/api/rooms/"+id+"/events", headers)
	if err != nil {
		t.Fatalf("websocket handshake: %v (%v)", err, response)
	}
	t.Cleanup(func() { _ = connection.Close() })
	return connection
}

func readRoomEvent(t *testing.T, connection *websocket.Conn) roomEvent {
	t.Helper()
	_ = connection.SetReadDeadline(time.Now().Add(3 * time.Second))
	var event roomEvent
	if err := connection.ReadJSON(&event); err != nil {
		t.Fatal(err)
	}
	return event
}

func awaitRoomVersion(t *testing.T, connection *websocket.Conn, version uint64) roomEvent {
	t.Helper()
	for i := 0; i < 10; i++ {
		event := readRoomEvent(t, connection)
		if event.Type != "room" {
			t.Fatalf("expected room, got %+v", event)
		}
		if event.Room.Version >= version {
			return event
		}
	}
	t.Fatal("room update was not received")
	return roomEvent{}
}

func TestWebSocketSnapshotsAndUpdates(t *testing.T) {
	s := testServer(t)
	host, hostCookie := createTestRoom(t, s, "standard")
	guest, guestCookie := joinTestRoom(t, s, host.Room.ID, "guest")
	base := socketServer(t, s)
	hostSocket := dialRoom(t, base, host.Room.ID, hostCookie)
	guestSocket := dialRoom(t, base, host.Room.ID, guestCookie)
	first := readRoomEvent(t, hostSocket)
	guestFirst := readRoomEvent(t, guestSocket)
	if first.PlayerID != host.PlayerID || guestFirst.PlayerID != guest.PlayerID || len(first.Room.Players) != 2 {
		t.Fatal("wrong initial snapshot")
	}
	if first.Room.ID != host.Room.ID || strings.Contains(fmt.Sprint(first), "secret") {
		t.Fatal("wrong room or leaked secret")
	}

	started := time.Now()
	second := readRoomEvent(t, hostSocket)
	interval := time.Since(started)
	if interval < 500*time.Millisecond || interval > 1800*time.Millisecond || second.Room.Version != first.Room.Version {
		t.Fatalf("expected one-second snapshot: %s", interval)
	}

	path := "/api/rooms/" + host.Room.ID
	request(t, s, "POST", path+"/start", nil, hostCookie, 200)
	game, _ := s.rooms.GetRoom(host.Room.ID)
	update := awaitRoomVersion(t, guestSocket, game.GetRoomStatus().Version)
	if update.Room.StartedAt == nil {
		t.Fatal("start not broadcast")
	}

	request(t, s, "PUT", path+"/progress", map[string]any{"index": 0, "completed": true}, hostCookie, 200)
	update = awaitRoomVersion(t, guestSocket, game.GetRoomStatus().Version)
	found := false
	for _, player := range update.Room.Players {
		if player.ID == host.PlayerID {
			found = player.Progress[0]
		}
	}
	if !found {
		t.Fatal("progress not broadcast")
	}
	request(t, s, "PUT", path+"/bowser", map[string]bool{"completed": true}, hostCookie, 200)
	update = awaitRoomVersion(t, guestSocket, game.GetRoomStatus().Version)
	for _, player := range update.Room.Players {
		if player.ID == host.PlayerID && !player.BowserDefeated {
			t.Fatal("bowser not broadcast")
		}
	}

	// Socket input never bypasses the normal authenticated update API.
	if err := guestSocket.WriteJSON(map[string]any{"index": 4, "completed": true, "playerId": host.PlayerID}); err != nil {
		t.Fatal(err)
	}
	_ = readRoomEvent(t, guestSocket)
	player, _ := game.GetPlayer(guest.PlayerID)
	if player.Progress[4] {
		t.Fatal("read-only stream mutated progress")
	}
}

func TestWebSocketMembershipAndRegeneration(t *testing.T) {
	s := testServer(t)
	host, cookie := createTestRoom(t, s, "line")
	base := socketServer(t, s)
	hostSocket := dialRoom(t, base, host.Room.ID, cookie)
	_ = readRoomEvent(t, hostSocket)
	guest, gc := joinTestRoom(t, s, host.Room.ID, "guest")
	game, _ := s.rooms.GetRoom(host.Room.ID)
	event := awaitRoomVersion(t, hostSocket, game.GetRoomStatus().Version)
	if len(event.Room.Players) != 2 {
		t.Fatal("join not broadcast")
	}
	guestSocket := dialRoom(t, base, host.Room.ID, gc)
	_ = readRoomEvent(t, guestSocket)
	path := "/api/rooms/" + host.Room.ID
	request(t, s, "POST", path+"/card", map[string]string{"seed": "socket-card"}, cookie, 200)
	event = awaitRoomVersion(t, guestSocket, game.GetRoomStatus().Version)
	if event.Room.Card.Seed != "socket-card" || event.PlayerID != guest.PlayerID {
		t.Fatal("regeneration not broadcast")
	}
	request(t, s, "POST", path+"/leave", nil, gc, 204)
	for {
		event = readRoomEvent(t, guestSocket)
		if event.Type == "error" {
			break
		}
	}
	if event.Room != nil {
		t.Fatal("revoked player received room data")
	}
	event = awaitRoomVersion(t, hostSocket, game.GetRoomStatus().Version)
	if len(event.Room.Players) != 1 {
		t.Fatal("leave not broadcast")
	}
	request(t, s, "DELETE", path, nil, cookie, 204)
	for readRoomEvent(t, hostSocket).Type != "error" {
	}
}

func TestWebSocketSessionExpiry(t *testing.T) {
	s := testServer(t)
	host, cookie := createTestRoom(t, s, "standard")
	connection := dialRoom(t, socketServer(t, s), host.Room.ID, cookie)
	_ = readRoomEvent(t, connection)
	s.mu.Lock()
	value := s.sessions[cookie.Value]
	value.Expires = time.Now().Add(-time.Second)
	s.sessions[cookie.Value] = value
	s.mu.Unlock()
	event := readRoomEvent(t, connection)
	if event.Type != "error" || event.Room != nil {
		t.Fatal("expired session received data")
	}
}

func TestWebSocketAuthorizationAndOrigins(t *testing.T) {
	s := testServer(t)
	host, cookie := createTestRoom(t, s, "standard")
	other, otherCookie := createTestRoom(t, s, "standard")
	base := socketServer(t, s)
	for _, headers := range []http.Header{
		{"Origin": {base}},
		{"Origin": {"https://external.example"}, "Cookie": {cookie.String()}},
		{"Cookie": {cookie.String()}},
	} {
		connection := dialSpectator(t, base, host.Room.ID, headers)
		event := readRoomEvent(t, connection)
		if event.Room == nil || event.Room.ID != host.Room.ID || event.PlayerID != "" {
			t.Fatal("public connection must return room data without session identity")
		}
	}
	for _, test := range []struct {
		cookie *http.Cookie
		origin string
		status int
	}{
		{otherCookie, base, 401},
	} {
		headers := http.Header{"Origin": []string{test.origin}}
		if test.cookie != nil {
			headers.Set("Cookie", test.cookie.Name+"="+test.cookie.Value)
		}
		conn, response, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(base, "http")+"/api/rooms/"+host.Room.ID+"/events", headers)
		if conn != nil {
			_ = conn.Close()
		}
		if err == nil || response == nil || response.StatusCode != test.status {
			t.Fatalf("expected %d: %v %v", test.status, response, err)
		}
		_ = response.Body.Close()
	}
	if other.Room.ID == host.Room.ID {
		t.Fatal("room IDs")
	}
	for _, secure := range []bool{false, true} {
		s.secureCookie = secure
		origin := "http://example.com"
		if secure {
			origin = "https://example.com"
		}
		r := httptest.NewRequest("GET", "http://example.com/api/rooms/x/events", nil)
		r.Header.Set("Origin", origin)
		if !s.websocketOrigin(r) {
			t.Fatal("valid origin rejected")
		}
		r.Header.Set("Sec-Fetch-Site", "cross-site")
		if s.websocketOrigin(r) {
			t.Fatal("cross-site origin accepted")
		}
	}
}

func TestRoomSubscriptions(t *testing.T) {
	s := testServer(t)
	updates, ok := s.subscribeRoom("room-a")
	if !ok {
		t.Fatal("subscribe")
	}
	other, _ := s.subscribeRoom("room-b")
	for i := 0; i < 100; i++ {
		s.notifyRoom("room-a")
	}
	if len(updates) != 1 || len(other) != 0 {
		t.Fatal("notifications must coalesce and stay within room")
	}
	<-updates
	s.notifyRoom("room-a")
	if len(updates) != 1 {
		t.Fatal("next notification lost")
	}
	channels := []chan struct{}{updates}
	for i := 1; i < maxRoomSubscribers; i++ {
		ch, ok := s.subscribeRoom("room-a")
		if !ok {
			t.Fatal(i)
		}
		channels = append(channels, ch)
	}
	if _, ok := s.subscribeRoom("room-a"); ok {
		t.Fatal("subscriber cap")
	}
	for _, ch := range channels {
		s.unsubscribeRoom("room-a", ch)
	}
	if _, exists := s.subscribers["room-a"]; exists {
		t.Fatal("room subscriptions leaked")
	}
	s.unsubscribeRoom("room-b", other)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ch, ok := s.subscribeRoom("concurrent")
			if ok {
				s.notifyRoom("concurrent")
				s.unsubscribeRoom("concurrent", ch)
			}
		}()
	}
	wg.Wait()
}

func TestWebSocketUpgradeFailureAndCapacity(t *testing.T) {
	s := testServer(t)
	host, cookie := createTestRoom(t, s, "line")
	path := "/api/rooms/" + host.Room.ID + "/events"
	r := httptest.NewRequest("GET", path, nil)
	r.Header.Set("Origin", "http://example.com")
	r.AddCookie(cookie)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != 400 {
		t.Fatal("non-websocket request", w.Code)
	}
	if len(s.subscribers) != 0 {
		t.Fatal("failed upgrade leaked subscription")
	}
	channels := make([]chan struct{}, 0, maxRoomSubscribers)
	for i := 0; i < maxRoomSubscribers; i++ {
		ch, _ := s.subscribeRoom(host.Room.ID)
		channels = append(channels, ch)
	}
	w = httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != 429 {
		t.Fatal("connection limit", w.Code)
	}
	for _, ch := range channels {
		s.unsubscribeRoom(host.Room.ID, ch)
	}
}
