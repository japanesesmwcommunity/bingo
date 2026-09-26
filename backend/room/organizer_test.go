package room

import (
	"bingo/bingo"
	"errors"
	"fmt"
	"sync"
	"testing"
)

func TestOrganizerDoesNotOccupyPlayerSlot(t *testing.T) {
	rm, r, owner := newManagedGame(t, Race, bingo.Standard)
	if len(r.GetRoomStatus().Players) != 0 || rm.ListActiveRooms()[0].PlayerCount != 0 {
		t.Fatal("organizer enrolled automatically")
	}
	if err := r.CheckAccess(owner); err != nil {
		t.Fatal(err)
	}
	if err := r.CheckAccess("unknown"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := r.GetPlayer(owner); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	var player string
	for i := 0; i < MaxPlayers; i++ {
		var err error
		player, err = r.AddPlayer("secret", fmt.Sprint(i), "")
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := r.CheckAccess(player); err != nil {
		t.Fatal(err)
	}
	if err := r.JoinOwner(owner, "organizer", ""); !errors.Is(err, ErrFull) {
		t.Fatal(err)
	}
	if len(r.GetRoomStatus().Players) != 4 {
		t.Fatal("capacity")
	}
	if err := r.JoinOwner(owner, "organizer", ""); !errors.Is(err, ErrFull) {
		t.Fatal(err)
	}
	if err := r.UpdatePlayerProgress(owner, 0, true); !errors.Is(err, ErrForbidden) {
		t.Fatal(err)
	}
	if err := r.UpdatePlayerProgress(player, 0, true); err != nil {
		t.Fatal(err)
	}
	if err := r.Finish(owner); err != nil {
		t.Fatal(err)
	}
	if r.GetRoomStatus().FinishedAt == nil {
		t.Fatal("organizer cannot finish")
	}
	if err := rm.DeleteRoom(r.GetRoomStatus().ID, owner); err != nil {
		t.Fatal(err)
	}
	if err := r.CheckAccess(owner); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if err := r.JoinOwner(owner, "organizer", ""); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
}

func TestOrganizerEnrollmentIsExplicitAndReversible(t *testing.T) {
	_, r, owner := newManagedGame(t, Race, bingo.LineOnly)
	if err := r.JoinOwner("unknown", "organizer", ""); !errors.Is(err, ErrForbidden) {
		t.Fatal(err)
	}
	for _, args := range [][2]string{{"", ""}, {"   ", "red"}} {
		if err := r.JoinOwner(owner, args[0], args[1]); err == nil {
			t.Fatal("invalid enrollment")
		}
	}
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := r.JoinOwner(owner, "organizer", ""); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	status := r.GetRoomStatus()
	if len(status.Players) != 1 || status.Players[0].ID != owner || status.Version != 2 {
		t.Fatal("duplicate owner enrollment", status)
	}
	if err := r.DeletePlayer(owner); err != nil {
		t.Fatal(err)
	}
	if err := r.CheckAccess(owner); err != nil {
		t.Fatal("withdraw revoked management", err)
	}
	if len(r.GetRoomStatus().Players) != 0 {
		t.Fatal("withdraw left player behind")
	}
	if err := r.JoinOwner(owner, "organizer", ""); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		if err := r.UpdatePlayerProgress(owner, i, true); err != nil {
			t.Fatal(err)
		}
	}
	if r.GetRoomStatus().FinishedAt != nil {
		t.Fatal("line must not auto-finish")
	}
}
