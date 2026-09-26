package room

import (
	"bingo/bingo"
	"errors"
	"fmt"
	"testing"
)

func TestFixedPlayerColors(t *testing.T) {
	_, r, owner := newManagedGame(t, Race, bingo.LineOnly)
	want := []string{"#e71e07", "#019ad7", "#fcd000", "#42b132"}
	ids := make([]string, 4)
	for i := range ids {
		if i == 1 {
			if err := r.JoinOwner(owner, "host", "#ffffff"); err != nil {
				t.Fatal(err)
			}
			ids[i] = owner
		} else {
			id, err := r.AddPlayer("secret", fmt.Sprint(i), "red")
			if err != nil {
				t.Fatal(err)
			}
			ids[i] = id
		}
		p, err := r.GetPlayer(ids[i])
		if err != nil || p.Color != want[i] {
			t.Fatalf("player %d: %+v, %v", i, p, err)
		}
	}
	if _, err := r.AddPlayer("secret", "fifth", ""); !errors.Is(err, ErrFull) {
		t.Fatalf("fifth player: %v", err)
	}
	if err := r.JoinOwner(owner, "host", "#000000"); err != nil {
		t.Fatal(err)
	}
	if err := r.DeletePlayer(owner); err != nil {
		t.Fatal(err)
	}
	if err := r.JoinOwner(owner, "host", ""); err != nil {
		t.Fatal(err)
	}
	if err := r.KickPlayer(owner, ids[0]); err != nil {
		t.Fatal(err)
	}
	id, err := r.AddPlayer("secret", "replacement", "#ffffff")
	if err != nil {
		t.Fatal(err)
	}
	ids[0] = id
	for i, id := range ids {
		p, err := r.GetPlayer(id)
		if err != nil || p.Color != want[i] {
			t.Fatalf("color changed or was not reused: %+v, %v", p, err)
		}
	}
}
