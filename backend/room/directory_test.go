package room

import (
	"bingo/bingo"
	"reflect"
	"sync"
	"testing"
)

func TestActiveRoomDirectory(t *testing.T) {
	rm, game, host := newGame(t, Race, bingo.LineOnly)
	roomID := game.GetRoomStatus().ID
	want := []Summary{{ID: roomID, Name: "test", Mode: Race, Rule: bingo.LineOnly, PlayerCount: 1, MaxPlayers: 4, Status: "playing"}}
	if got := rm.ListActiveRooms(); !reflect.DeepEqual(got, want) {
		t.Fatalf("initial directory: %+v", got)
	}
	for _, name := range []string{"second", "third", "fourth"} {
		if _, err := game.AddPlayer("secret", name, ""); err != nil {
			t.Fatal(err)
		}
	}
	if got := rm.ListActiveRooms(); len(got) != 1 || got[0].PlayerCount != 4 {
		t.Fatal("full rooms remain in the directory", got)
	}
	if got := rm.ListActiveRooms(); len(got) != 1 || got[0].Status != "playing" {
		t.Fatal(got)
	}
	for index := 0; index < 5; index++ {
		if err := game.UpdatePlayerProgress(host, index, true); err != nil {
			t.Fatal(err)
		}
	}
	if len(rm.ListActiveRooms()) != 1 {
		t.Fatal("completed line disappeared")
	}
	if err := game.Finish(host); err != nil {
		t.Fatal(err)
	}
	if got := rm.ListActiveRooms(); got == nil || len(got) != 0 {
		t.Fatal("finished rooms must be omitted with an empty slice", got)
	}
	if err := rm.DeleteRoom(roomID, host); err != nil {
		t.Fatal(err)
	}
	if got := rm.ListActiveRooms(); len(got) != 0 {
		t.Fatal("deleted rooms must be omitted", got)
	}
}

func TestDirectoryOrderAndConcurrentChanges(t *testing.T) {
	rm, playing, host := newGame(t, Race, bingo.Standard)
	for _, name := range []string{"B", "A", "A"} {
		if _, _, err := rm.CreateRoom(CreateOptions{Name: name, Passphrase: "secret"}); err != nil {
			t.Fatal(err)
		}
	}
	rooms := rm.ListActiveRooms()
	if len(rooms) != 4 || rooms[0].Name != "A" || rooms[1].Name != "A" || rooms[0].ID >= rooms[1].ID || rooms[2].Name != "B" || rooms[3].Status != "playing" {
		t.Fatal(rooms)
	}
	rooms[0].Name = "changed copy"
	if rm.ListActiveRooms()[0].Name != "A" {
		t.Fatal("directory must contain copies")
	}
	var workers sync.WaitGroup
	workers.Add(2)
	go func() {
		defer workers.Done()
		for i := 0; i < 100; i++ {
			_ = rm.ListActiveRooms()
		}
	}()
	go func() {
		defer workers.Done()
		for i := 0; i < 100; i++ {
			if err := playing.UpdatePlayerProgress(host, 0, i%2 == 0); err != nil {
				t.Error(err)
			}
		}
	}()
	workers.Wait()
}
