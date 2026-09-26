package room

import (
	"bingo/bingo"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"
)

func TestKickPermissionsAndCapacity(t *testing.T) {
	_, r, owner := newManagedGame(t, Race, bingo.Standard)
	var players []string
	for i := 0; i < MaxPlayers; i++ {
		id, err := r.AddPlayer("secret", fmt.Sprint(i), "")
		if err != nil {
			t.Fatal(err)
		}
		players = append(players, id)
	}
	before := r.GetRoomStatus()
	for _, actor := range []string{players[0], "unknown"} {
		if err := r.KickPlayer(actor, players[1]); !errors.Is(err, ErrForbidden) {
			t.Fatal(err)
		}
	}
	if err := r.KickPlayer(owner, owner); !errors.Is(err, ErrForbidden) {
		t.Fatal(err)
	}
	if err := r.KickPlayer(owner, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, r.GetRoomStatus()) {
		t.Fatal("rejected kick changed state")
	}
	if err := r.KickPlayer(owner, players[0]); err != nil {
		t.Fatal(err)
	}
	after := r.GetRoomStatus()
	if len(after.Players) != 3 || after.Version != before.Version+1 {
		t.Fatal(after)
	}
	if err := r.CheckAccess(players[0]); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if err := r.KickPlayer(owner, players[0]); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := r.AddPlayer("secret", "replacement", ""); err != nil {
		t.Fatal(err)
	}
}

func TestKickReleasesLockoutAndPreservesFinishedResults(t *testing.T) {
	rm, r, owner := newGame(t, Lockout, bingo.LineOnly)
	guest, err := r.AddPlayer("secret", "guest", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.UpdatePlayerProgress(guest, 0, true); err != nil {
		t.Fatal(err)
	}
	if err := r.UpdatePlayerProgress(owner, 0, true); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	if err := r.KickPlayer(owner, guest); err != nil {
		t.Fatal(err)
	}
	if err := r.UpdatePlayerProgress(guest, 1, true); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if err := r.UpdateBowser(guest, true); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		if err := r.UpdatePlayerProgress(owner, i, true); err != nil {
			t.Fatal(err)
		}
	}
	before := r.GetRoomStatus()
	if err := r.KickPlayer(owner, guest); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	if before.WinnerID != owner || !reflect.DeepEqual(before, r.GetRoomStatus()) {
		t.Fatal("finished result changed")
	}
	if err := rm.DeleteRoom(before.ID, owner); err != nil {
		t.Fatal(err)
	}
	if err := r.KickPlayer(owner, guest); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
}

func TestKickAndConcurrentProgress(t *testing.T) {
	_, r, owner := newManagedGame(t, Race, bingo.Standard)
	guest, err := r.AddPlayer("secret", "guest", "")
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			if err := r.UpdatePlayerProgress(guest, 0, i%2 == 0); err != nil && !errors.Is(err, ErrNotFound) {
				t.Error(err)
			}
		}
	}()
	if err := r.KickPlayer(owner, guest); err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	if len(r.GetRoomStatus().Players) != 0 {
		t.Fatal("kicked player restored by progress update")
	}
	if err := r.Finish(owner); err != nil {
		t.Fatal(err)
	}
}
