package room

import (
	"bingo/bingo"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"reflect"
	"sync"
	"testing"
)

func newManagedGame(t *testing.T, mode Mode, rule bingo.Rule) (*RoomManager, *Room, string) {
	t.Helper()
	if err := bingo.InitData("../bingo.json"); err != nil {
		t.Fatal(err)
	}
	rm := NewRoomManager()
	o := bingo.DefaultOptions()
	o.Rule = rule
	r, id, err := rm.CreateRoom(CreateOptions{Name: "test", Passphrase: "secret", Mode: mode, Options: o})
	if err != nil {
		t.Fatal(err)
	}
	return rm, r, id
}
func newGame(t *testing.T, mode Mode, rule bingo.Rule) (*RoomManager, *Room, string) {
	t.Helper()
	rm, r, id := newManagedGame(t, mode, rule)
	if err := r.JoinOwner(id, "host", ""); err != nil {
		t.Fatal(err)
	}
	return rm, r, id
}
func TestRoomLifecycle(t *testing.T) {
	rm, r, host := newGame(t, Race, bingo.Standard)
	status := r.GetRoomStatus()
	if _, err := uuid.Parse(status.ID); err != nil {
		t.Fatal(err)
	}
	if status.OwnerID != host {
		t.Fatal(status)
	}
	if got, err := rm.GetRoom(status.ID); err != nil || got != r {
		t.Fatal(err)
	}
	if _, err := rm.GetRoom("missing"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if err := rm.DeleteRoom("missing", host); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if err := rm.DeleteRoom(status.ID, "other"); !errors.Is(err, ErrForbidden) {
		t.Fatal(err)
	}
	guest, err := r.AddPlayer("secret", "guest", "#aBc123")
	if err != nil {
		t.Fatal(err)
	}
	p, err := r.GetPlayer(guest)
	if err != nil || p.Name != "guest" {
		t.Fatal(p, err)
	}
	p.Progress[0] = true
	again, _ := r.GetPlayer(guest)
	if again.Progress[0] {
		t.Fatal("player must be snapshot")
	}
	status.Card.Goals[0].Tags[0] = "mutated"
	if r.GetRoomStatus().Card.Goals[0].Tags[0] == "mutated" {
		t.Fatal("card snapshot alias")
	}
	if _, err := r.GetPlayer("missing"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if err := r.DeletePlayer("missing"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if err := r.DeletePlayer(guest); err != nil {
		t.Fatal(err)
	}
	if err := r.GenerateCard("replay", 90, 20, bingo.Standard); err != nil {
		t.Fatal(err)
	}
	card := r.GetRoomStatus().Card
	if card.Seed != "replay" {
		t.Fatal(card.Seed)
	}
	if err := r.GenerateCard("bad", 1, 0, bingo.Standard); err == nil {
		t.Fatal("impossible")
	}
	if err := r.GenerateCard("bad", 90, 0, "bad"); err == nil {
		t.Fatal("invalid rule")
	}
	if !reflect.DeepEqual(card, r.GetRoomStatus().Card) {
		t.Fatal("failed generation changed card")
	}
	if err := rm.DeleteRoom(status.ID, host); err != nil {
		t.Fatal(err)
	}
	if _, err := r.AddPlayer("secret", "late", ""); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if err := r.GenerateCard("", 90, 20, bingo.Standard); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
}
func TestJoinValidation(t *testing.T) {
	_, r, _ := newGame(t, Race, bingo.Standard)
	for _, c := range []struct{ pass, name, color string }{{"wrong", "guest", ""}, {"secret", "", ""}, {"secret", "host", ""}} {
		if _, err := r.AddPlayer(c.pass, c.name, c.color); err == nil {
			t.Fatal(c)
		}
	}
	for i := 0; i < 3; i++ {
		if _, err := r.AddPlayer("secret", fmt.Sprint(i), ""); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := r.AddPlayer("secret", "fifth", ""); !errors.Is(err, ErrFull) {
		t.Fatal(err)
	}
	if len(r.GetRoomStatus().Players) != 4 {
		t.Fatal("capacity")
	}
}
func TestCreateValidation(t *testing.T) {
	rm, _, _ := newGame(t, Race, bingo.Standard)
	valid := CreateOptions{Name: "test", Passphrase: "secret", Options: bingo.DefaultOptions()}
	changes := []func(*CreateOptions){func(o *CreateOptions) { o.Name = " " }, func(o *CreateOptions) { o.Passphrase = " " }, func(o *CreateOptions) { o.Mode = "bad" }, func(o *CreateOptions) { o.Rule = "bad" }, func(o *CreateOptions) { o.MaxTime = 1 }}
	for i, change := range changes {
		o := valid
		change(&o)
		if _, _, err := rm.CreateRoom(o); err == nil {
			t.Fatal(i)
		}
	}
	var zero RoomManager
	if _, _, err := zero.CreateRoom(valid); err != nil {
		t.Fatal(err)
	}
}
func TestCompletedCardsRemainEditable(t *testing.T) {
	for _, rule := range []bingo.Rule{bingo.Standard, bingo.LineOnly} {
		_, r, host := newGame(t, Race, rule)
		for i := 0; i < 25; i++ {
			if err := r.UpdatePlayerProgress(host, i, true); err != nil {
				t.Fatal(err)
			}
		}
		if r.GetRoomStatus().FinishedAt != nil {
			t.Fatal("completed card ended automatically")
		}
		if err := r.UpdatePlayerProgress(host, 0, false); err != nil {
			t.Fatal(err)
		}
		if _, err := r.AddPlayer("secret", "late", ""); err != nil {
			t.Fatal(err)
		}
		if err := r.Finish(host); err != nil {
			t.Fatal(err)
		}
		if err := r.UpdatePlayerProgress(host, 0, true); !errors.Is(err, ErrConflict) {
			t.Fatal(err)
		}
	}
}
func TestRaceAndLockout(t *testing.T) {
	for _, mode := range []Mode{Race, Lockout} {
		_, r, host := newGame(t, mode, bingo.LineOnly)
		guest, _ := r.AddPlayer("secret", "guest", "")
		if err := r.UpdatePlayerProgress(host, 0, true); err != nil {
			t.Fatal(err)
		}
		err := r.UpdatePlayerProgress(guest, 0, true)
		if mode == Race && err != nil {
			t.Fatal(err)
		}
		if mode == Lockout && !errors.Is(err, ErrConflict) {
			t.Fatal(err)
		}
		if err := r.UpdatePlayerProgress(host, 0, false); err != nil {
			t.Fatal(err)
		}
		if err := r.UpdatePlayerProgress(guest, 0, true); err != nil {
			t.Fatal(err)
		}
		for i := 1; i < 5; i++ {
			if err := r.UpdatePlayerProgress(guest, i, true); err != nil {
				t.Fatal(err)
			}
		}
		if r.GetRoomStatus().FinishedAt != nil {
			t.Fatal("line must not end the room")
		}
	}
}
func TestConcurrentJoins(t *testing.T) {
	_, r, _ := newGame(t, Race, bingo.Standard)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := r.AddPlayer("secret", fmt.Sprint(i), "")
			if err != nil && !errors.Is(err, ErrFull) {
				t.Error(err)
			}
			_ = r.GetRoomStatus()
		}(i)
	}
	wg.Wait()
	if len(r.GetRoomStatus().Players) != 4 {
		t.Fatal("overbooked")
	}
}
func TestConcurrentClaimsAndCompletedLines(t *testing.T) {
	_, r, host := newGame(t, Lockout, bingo.LineOnly)
	guest, _ := r.AddPlayer("secret", "guest", "")
	var wg sync.WaitGroup
	for _, id := range []string{host, guest} {
		wg.Add(1)
		go func(id string) { defer wg.Done(); _ = r.UpdatePlayerProgress(id, 12, true) }(id)
	}
	wg.Wait()
	count := 0
	for _, p := range r.GetRoomStatus().Players {
		if p.Progress[12] {
			count++
		}
	}
	if count != 1 {
		t.Fatal("double claim")
	}
	_, r, host = newGame(t, Race, bingo.Standard)
	guest, _ = r.AddPlayer("secret", "guest", "")
	for _, id := range []string{host, guest} {
		for i := 0; i < 5; i++ {
			_ = r.UpdatePlayerProgress(id, i, true)
		}
	}
	for _, id := range []string{host, guest} {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			_ = r.UpdatePlayerProgress(id, 5, true)
			_, _ = r.GetPlayer(id)
			_ = r.GetRoomStatus()
		}(id)
	}
	wg.Wait()
	if r.GetRoomStatus().FinishedAt != nil {
		t.Fatal("completed lines ended the room")
	}
	for _, p := range r.GetRoomStatus().Players {
		if !p.Progress[5] {
			t.Fatal("concurrent progress lost")
		}
	}
}

func TestRoomVersions(t *testing.T) {
	_, r, host := newGame(t, Race, bingo.Standard)
	version := r.GetRoomStatus().Version
	if version == 0 {
		t.Fatal("initial version")
	}
	guest, err := r.AddPlayer("secret", "guest", "")
	if err != nil {
		t.Fatal(err)
	}
	if r.GetRoomStatus().Version != version+1 {
		t.Fatal("join version")
	}
	version++
	if err := r.DeletePlayer(guest); err != nil {
		t.Fatal(err)
	}
	if r.GetRoomStatus().Version != version+1 {
		t.Fatal("leave version")
	}
	version++
	if err := r.GenerateCard("version", 90, 0, bingo.Standard); err != nil {
		t.Fatal(err)
	}
	if r.GetRoomStatus().Version != version+1 {
		t.Fatal("card version")
	}
	version++
	if err := r.UpdatePlayerProgress(host, 0, true); err != nil {
		t.Fatal(err)
	}
	if r.GetRoomStatus().Version != version+1 {
		t.Fatal("progress version")
	}
	version++
	if err := r.UpdatePlayerProgress(host, -1, true); err == nil {
		t.Fatal("invalid index")
	}
	if r.GetRoomStatus().Version != version {
		t.Fatal("failed mutation changed version")
	}
}
