package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPublicRoomDirectory(t *testing.T) {
	s := testServer(t)
	empty := request(t, s, "GET", "/api/rooms", nil, nil, 200)
	if strings.TrimSpace(empty.Body.String()) != `{"rooms":[]}` {
		t.Fatal(empty.Body.String())
	}
	host, cookie := createTestRoom(t, s, "line")
	path := "/api/rooms/" + host.Room.ID
	response := request(t, s, "GET", "/api/rooms", nil, nil, 200)
	var data struct {
		Rooms []map[string]any `json:"rooms"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Rooms) != 1 {
		t.Fatal(data)
	}
	for _, key := range []string{"id", "name", "mode", "rule", "playerCount", "maxPlayers", "status"} {
		if _, ok := data.Rooms[0][key]; !ok {
			t.Fatal("missing summary field", key)
		}
	}
	if len(data.Rooms[0]) != 7 || strings.Contains(response.Body.String(), "secret") {
		t.Fatal("directory must only expose public summary fields", response.Body.String())
	}
	request(t, s, "GET", path, nil, nil, 401)
	request(t, s, "DELETE", path, nil, cookie, 204)
	after := request(t, s, "GET", "/api/rooms", nil, nil, 200)
	if strings.TrimSpace(after.Body.String()) != `{"rooms":[]}` {
		t.Fatal(after.Body.String())
	}
}
