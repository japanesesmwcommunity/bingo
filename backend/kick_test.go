package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAPIKickRevokesOnlyTargetAndBroadcasts(t *testing.T) {
	s := testServer(t)
	host, hc := createTestRoom(t, s, "standard")
	guest, gc := joinTestRoom(t, s, host.Room.ID, "guest")
	other, oc := createTestRoom(t, s, "standard")
	_, otherGuestCookie := joinTestRoom(t, s, other.Room.ID, "guest")
	path := "/api/rooms/" + host.Room.ID
	kickPath := path + "/players/" + guest.PlayerID
	request(t, s, "DELETE", kickPath, nil, nil, 401)
	request(t, s, "DELETE", kickPath, nil, gc, 403)
	request(t, s, "DELETE", kickPath, nil, oc, 401)
	request(t, s, "DELETE", path+"/players/"+host.PlayerID, nil, hc, 403)
	request(t, s, "DELETE", path+"/players/"+other.PlayerID, nil, hc, 404)
	request(t, s, "PUT", path+"/progress", map[string]any{"index": 0, "completed": true}, gc, 200)
	base := socketServer(t, s)
	hostSocket := dialRoom(t, base, host.Room.ID, hc)
	guestSocket := dialRoom(t, base, host.Room.ID, gc)
	readRoomEvent(t, hostSocket)
	readRoomEvent(t, guestSocket)
	// Multiple browser sessions belonging to the target must all be revoked.
	secondSession := httptest.NewRecorder()
	s.setSession(secondSession, host.Room.ID, guest.PlayerID)
	gc2 := secondSession.Result().Cookies()[0]
	w := request(t, s, "DELETE", kickPath, nil, hc, 200)
	if len(w.Result().Cookies()) != 0 {
		t.Fatal("kick overwrote organizer cookie")
	}
	var status struct {
		Players []any  `json:"players"`
		Version uint64 `json:"version"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if len(status.Players) != 1 {
		t.Fatal("target not removed")
	}
	event := awaitRoomVersion(t, hostSocket, status.Version)
	if len(event.Room.Players) != 1 {
		t.Fatal("kick not broadcast")
	}
	for readRoomEvent(t, guestSocket).Type != "error" {
	}
	for _, cookie := range []*http.Cookie{gc, gc2} {
		request(t, s, "GET", path+"/session", nil, cookie, 401)
		request(t, s, "PUT", path+"/progress", map[string]any{"index": 1, "completed": true}, cookie, 401)
		if _, ok := s.sessions[cookie.Value]; ok {
			t.Fatal("target session retained")
		}
	}
	request(t, s, "GET", path, nil, hc, 200)
	request(t, s, "GET", "/api/rooms/"+other.Room.ID, nil, otherGuestCookie, 200)
	request(t, s, "POST", path+"/finish", nil, hc, 200)
	request(t, s, "DELETE", kickPath, nil, hc, 409)
}
