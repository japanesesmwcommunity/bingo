package room

import (
	"bingo/bingo"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestFinish(t *testing.T) {
	for _, mode := range []Mode{Race, Lockout} {
		t.Run(string(mode), func(t *testing.T) {
			rm, r, host := newGame(t, mode, bingo.Standard)
			guest, err := r.AddPlayer("secret", "guest", "")
			if err != nil {
				t.Fatal(err)
			}
			if err := r.Finish(host); !errors.Is(err, ErrConflict) {
				t.Fatal(err)
			}
			if err := r.Start(host); err != nil {
				t.Fatal(err)
			}
			if err := r.UpdatePlayerProgress(guest, 0, true); err != nil {
				t.Fatal(err)
			}
			r.mu.Lock()
			r.startedAt = time.Now().Add(-10 * time.Second)
			r.mu.Unlock()
			before := r.GetRoomStatus()
			for _, id := range []string{guest, "unknown"} {
				if err := r.Finish(id); !errors.Is(err, ErrForbidden) {
					t.Fatal(err)
				}
			}
			if err := r.Finish(host); err != nil {
				t.Fatal(err)
			}
			after := r.GetRoomStatus()
			if after.FinishedAt == nil || after.WinnerID != "" || after.Version != before.Version+1 || !reflect.DeepEqual(after.Players, before.Players) {
				t.Fatal("finish must preserve progress and end without a winner", after)
			}
			if after.ElapsedSeconds != int(after.FinishedAt.Sub(*after.StartedAt).Seconds()) || after.ElapsedSeconds < 10 {
				t.Fatal("timer must stop at finish time", after)
			}
			if len(rm.ListActiveRooms()) != 0 {
				t.Fatal("finished room remains active")
			}
			if err := r.Finish(host); !errors.Is(err, ErrConflict) {
				t.Fatal(err)
			}
			if err := r.Start(host); !errors.Is(err, ErrConflict) {
				t.Fatal(err)
			}
			if err := r.UpdatePlayerProgress(guest, 1, true); !errors.Is(err, ErrConflict) {
				t.Fatal(err)
			}
			if err := r.UpdateBowser(host, true); !errors.Is(err, ErrConflict) {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(after, r.GetRoomStatus()) {
				t.Fatal("finished state changed")
			}
			if err := rm.DeleteRoom(after.ID, host); err != nil {
				t.Fatal(err)
			}
			if err := r.Finish(host); !errors.Is(err, ErrNotFound) {
				t.Fatal(err)
			}
		})
	}
}

func TestFinishPreservesWinner(t *testing.T) {
	_, r, host := newGame(t, Race, bingo.LineOnly)
	if err := r.Start(host); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		if err := r.UpdatePlayerProgress(host, i, true); err != nil {
			t.Fatal(err)
		}
	}
	before := r.GetRoomStatus()
	if err := r.Finish(host); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	if before.WinnerID != host || !reflect.DeepEqual(before, r.GetRoomStatus()) {
		t.Fatal("winner changed")
	}
}
