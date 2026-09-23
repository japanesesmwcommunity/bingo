package bingo

import (
	"fmt"
	"math/rand"
	"reflect"
	"testing"
)

func assertUniqueWorlds(t *testing.T, card BingoCard) {
	t.Helper()
	for lineIndex, line := range lines {
		seen := make(map[string]bool)
		for _, cell := range line {
			world := card.Goals[cell].World
			if world == "" {
				continue
			}
			if seen[world] {
				t.Fatalf("line %d repeats world %q (seed %q)", lineIndex, world, card.Seed)
			}
			seen[world] = true
		}
	}
}

func TestWorldPlacement(t *testing.T) {
	for lineIndex, line := range lines {
		card := BingoCard{}
		card.Goals[line[0]] = BingoGoal{Name: "existing", World: "ヨースターとう", Level: "コース1"}
		for _, cell := range line[1:] {
			if canPlaceWorld(card, cell, "ヨースターとう") {
				t.Fatalf("line %d must reject another course in the same world", lineIndex)
			}
			if !canPlaceWorld(card, cell, "ドーナツへいや") || !canPlaceWorld(card, cell, "") {
				t.Fatal("different and unrestricted worlds must be allowed")
			}
		}
	}
	card := BingoCard{}
	card.Goals[0] = BingoGoal{Name: "existing", World: "ヨースターとう"}
	if !canPlaceWorld(card, 0, "ヨースターとう") || !canPlaceWorld(card, 7, "ヨースターとう") {
		t.Fatal("the same world is allowed at the same cell or on unrelated lines")
	}
}

func TestWorldIdentityAndValidation(t *testing.T) {
	goals := fixtureGoals(25)
	goals[0].Name, goals[1].Name = "通常ゴール", "通常ゴール"
	goals[0].World, goals[1].World = "チョコレーとう", "チョコレーとう"
	goals[0].Level, goals[1].Level = "2", "3"
	if err := ValidateGoals(goals); err != nil {
		t.Fatal("the same task on different courses is distinct", err)
	}
	goals[1].Level = "2"
	goals[1].World = "ドーナツへいや"
	if err := ValidateGoals(goals); err != nil {
		t.Fatal("the same course number in different worlds is distinct", err)
	}
	goals[1].World = goals[0].World
	if err := ValidateGoals(goals); err == nil {
		t.Fatal("identical world, level and task must be rejected")
	}
	for _, stage := range [][2]string{{"", "1"}, {" ヨースターとう", "1"}, {"ヨースターとう ", "1"}, {"ヨースターとう", " 1"}, {"ヨースターとう", "1 "}} {
		invalid := fixtureGoals(25)
		invalid[0].World, invalid[0].Level = stage[0], stage[1]
		if err := ValidateGoals(invalid); err == nil {
			t.Fatalf("invalid stage accepted: %v", stage)
		}
	}
}

func TestRedrawWhenWorldsCannotBePlaced(t *testing.T) {
	goals := fixtureGoals(50)
	for index := 0; index < 10; index++ {
		goals[index].World = "ヨースターとう"
		goals[index].Level = fmt.Sprint(index + 1)
	}
	// The first selection contains more than five goals from the same world,
	// so even the row constraints alone make that selection impossible.
	seed := ""
	for n := 1; n <= 100; n++ {
		candidate := fmt.Sprint(n)
		count := 0
		for _, index := range generateIndex(rand.New(rand.NewSource(strToInt64(candidate))), len(goals)) {
			if goals[index].World != "" {
				count++
			}
		}
		if count > 5 {
			seed = candidate
			break
		}
	}
	if seed == "" {
		t.Fatal("fixture must require a redraw")
	}
	card, err := SelectGoals(goals, seed, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	assertUniqueWorlds(t, card)
	replay, err := SelectGoals(goals, seed, DefaultOptions())
	if err != nil || !reflect.DeepEqual(card, replay) {
		t.Fatal("redraws must remain deterministic", err)
	}
	for index := range goals {
		goals[index].World = "ヨースターとう"
	}
	impossible, err := SelectGoals(goals, seed, DefaultOptions())
	if err == nil || !reflect.DeepEqual(impossible, BingoCard{}) {
		t.Fatal("impossible catalogs must fail without returning a conflicting card")
	}
}

func TestFiveWorldsWithoutUnrestrictedGoals(t *testing.T) {
	goals := fixtureGoals(25)
	for index := range goals {
		goals[index].World = fmt.Sprintf("world-%d", index%5)
	}
	for n := 0; n < 10; n++ {
		card, err := SelectGoals(goals, fmt.Sprint(n), DefaultOptions())
		if err != nil {
			t.Fatal(n, err)
		}
		assertUniqueWorlds(t, card)
	}
}
