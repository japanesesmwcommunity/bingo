package main

import (
	"bingo/room"
	"encoding/json"
	"testing"
)

func TestAPIFinishAndBroadcast(t *testing.T) {
	s := testServer(t)
	host, hc := createTestRoom(t, s, "standard")
	_, gc := joinTestRoom(t, s, host.Room.ID, "guest")
	path := "/api/rooms/" + host.Room.ID
	request(t, s, "POST", path+"/finish", nil, nil, 401)

	request(t, s, "POST", path+"/finish", nil, gc, 403)
	connection := dialRoom(t, socketServer(t, s), host.Room.ID, gc)
	initial := readRoomEvent(t, connection)
	w := request(t, s, "POST", path+"/finish", nil, hc, 200)
	var status room.Status
	if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status.FinishedAt == nil || status.Version != initial.Room.Version+1 {
		t.Fatal(status)
	}
	event := awaitRoomVersion(t, connection, status.Version)
	if event.Room.FinishedAt == nil || !event.Room.FinishedAt.Equal(*status.FinishedAt) {
		t.Fatal("finish not broadcast", event)
	}

	request(t, s, "PUT", path+"/progress", map[string]any{"index": 0, "completed": true}, gc, 409)
	request(t, s, "POST", path+"/finish", nil, hc, 409)
	request(t, s, "DELETE", path, nil, hc, 204)
}
