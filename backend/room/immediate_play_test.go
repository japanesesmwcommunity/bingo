package room

import (
	"bingo/bingo"
	"errors"
	"testing"
)

func TestImmediatePlayAndLateEnrollment(t *testing.T) {
	rm, r, owner := newManagedGame(t, Lockout, bingo.Standard)
	first, err := r.AddPlayer("secret", "first", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.UpdatePlayerProgress(first, 0, true); err != nil {
		t.Fatal(err)
	}
	second, err := r.AddPlayer("secret", "second", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.UpdatePlayerProgress(second, 0, true); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	if err := r.JoinOwner(owner, "owner", ""); err != nil {
		t.Fatal(err)
	}
	if err := r.DeletePlayer(first); err != nil {
		t.Fatal(err)
	}
	if err := r.UpdatePlayerProgress(second, 0, true); err != nil {
		t.Fatal("departed player's cell stayed occupied", err)
	}
	if err := r.DeletePlayer(owner); err != nil {
		t.Fatal(err)
	}
	if err := r.CheckAccess(owner); err != nil {
		t.Fatal(err)
	}
	if err := rm.DeleteRoom(r.GetRoomStatus().ID, owner); err != nil {
		t.Fatal(err)
	}
}

func TestRegenerationRequiresEmptyProgress(t *testing.T) {
	_, r, owner := newGame(t, Race, bingo.Standard)
	if err := r.UpdatePlayerProgress(owner, 0, true); err != nil {
		t.Fatal(err)
	}
	if err := r.GenerateCard("new", 90, 0, bingo.Standard); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	if err := r.UpdatePlayerProgress(owner, 0, false); err != nil {
		t.Fatal(err)
	}
	if err := r.GenerateCard("new", 90, 0, bingo.Standard); err != nil {
		t.Fatal(err)
	}
	if err := r.Finish(owner); err != nil {
		t.Fatal(err)
	}
	if err := r.GenerateCard("ended", 90, 0, bingo.Standard); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	if _, err := r.AddPlayer("secret", "late", ""); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	if err := r.DeletePlayer(owner); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
}

func TestEmptyRoomCanFinish(t *testing.T) {
	_, r, owner := newManagedGame(t, Race, bingo.Standard)
	if err := r.Finish(owner); err != nil {
		t.Fatal(err)
	}
	if r.GetRoomStatus().FinishedAt == nil {
		t.Fatal("missing finish time")
	}
}
