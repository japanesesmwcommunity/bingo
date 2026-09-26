package main

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestAPIOrganizerAndFourPlayers(t *testing.T) {
	s := testServer(t)
	w := request(t, s, "POST", "/api/rooms", map[string]string{"name": "tournament", "passphrase": "secret"}, nil, 201)
	var created responseData
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if len(created.Room.Players) != 0 || created.PlayerID != created.Room.OwnerID {
		t.Fatal(created)
	}
	ownerCookie := w.Result().Cookies()[0]
	path := "/api/rooms/" + created.Room.ID
	request(t, s, "GET", path, nil, ownerCookie, 200)
	request(t, s, "POST", path+"/card", map[string]string{"seed": "staff"}, ownerCookie, 200)
	connection := dialRoom(t, socketServer(t, s), created.Room.ID, ownerCookie)
	initial := readRoomEvent(t, connection)
	if initial.Type != "room" || len(initial.Room.Players) != 0 {
		t.Fatal("organizer cannot observe", initial)
	}
	for i := 0; i < 4; i++ {
		joinTestRoom(t, s, created.Room.ID, fmt.Sprint(i))
	}
	game, _ := s.rooms.GetRoom(created.Room.ID)
	event := awaitRoomVersion(t, connection, game.GetRoomStatus().Version)
	if len(event.Room.Players) != 4 {
		t.Fatal("organizer occupied a slot")
	}
	request(t, s, "POST", path+"/join", map[string]string{"playerName": "staff"}, ownerCookie, 409)
	request(t, s, "PUT", path+"/progress", map[string]any{"index": 0, "completed": true}, ownerCookie, 403)
	request(t, s, "POST", path+"/finish", nil, ownerCookie, 200)
	event = awaitRoomVersion(t, connection, game.GetRoomStatus().Version)
	if event.Room.FinishedAt == nil {
		t.Fatal("organizer missing finish notification")
	}
	request(t, s, "DELETE", path, nil, ownerCookie, 204)
	// Room snapshots sent before deletion can still be queued on the socket.
	for i := 0; i < 10; i++ {
		if readRoomEvent(t, connection).Type == "error" {
			if _, err := s.rooms.GetRoom(created.Room.ID); err == nil {
				t.Fatal("deleted room still accessible")
			}
			return
		}
	}
	t.Fatal("room deletion was not delivered")
}

func TestAPIOrganizerCanWithdrawWithoutLosingAccess(t *testing.T) {
	s := testServer(t)
	owner, cookie := createTestRoom(t, s, "line")
	path := "/api/rooms/" + owner.Room.ID
	connection := dialRoom(t, socketServer(t, s), owner.Room.ID, cookie)
	readRoomEvent(t, connection)
	request(t, s, "POST", path+"/leave", nil, cookie, 200)
	game, _ := s.rooms.GetRoom(owner.Room.ID)
	event := awaitRoomVersion(t, connection, game.GetRoomStatus().Version)
	if len(event.Room.Players) != 0 {
		t.Fatal("withdraw failed")
	}
	request(t, s, "GET", path, nil, cookie, 200)
	request(t, s, "POST", path+"/join", map[string]string{"playerName": "again"}, cookie, 200)
	request(t, s, "POST", path+"/join", map[string]string{"playerName": "again"}, cookie, 200)
	if len(game.GetRoomStatus().Players) != 1 {
		t.Fatal("duplicate participation")
	}
}
