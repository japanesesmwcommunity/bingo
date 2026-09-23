package bingo

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
)

func fixtureGoals(count int) []BingoGoal {
	goals := make([]BingoGoal, count)
	for i := range goals {
		goals[i] = BingoGoal{Name: fmt.Sprintf("goal-%d", i), TimeMin: 5, Exec: 1, Risk: 1, Tags: []string{fmt.Sprintf("area:a%d", i%5), fmt.Sprintf("type:t%d", i%3)}}
	}
	return goals
}
func TestGenerateIndex(t *testing.T) {
	for _, max := range []int{25, 51, 150} {
		seen := map[int]bool{}
		for _, index := range generateIndex(rand.New(rand.NewSource(1)), max) {
			if index < 0 || index >= max || seen[index] {
				t.Fatalf("invalid index %d", index)
			}
			seen[index] = true
		}
		if len(seen) != 25 {
			t.Fatal(seen)
		}
	}
	if generateIndex(rand.New(rand.NewSource(1)), 24) != [25]int{} {
		t.Fatal("short catalog")
	}
}
func TestSeedAndSelection(t *testing.T) {
	goals := fixtureGoals(50)
	for _, seed := range []string{"0", "42", "-1", "マリオ"} {
		a, err := SelectGoals(goals, seed, DefaultOptions())
		if err != nil {
			t.Fatal(err)
		}
		b, err := SelectGoals(goals, seed, DefaultOptions())
		if err != nil || !reflect.DeepEqual(a, b) || a.GetSeed() != seed {
			t.Fatal("seed must be repeatable")
		}
		seen := map[string]bool{}
		for _, g := range a.Goals {
			if seen[g.Name] {
				t.Fatal("duplicate")
			}
			seen[g.Name] = true
		}
		a.Goals[0].Tags[0] = "changed"
		if goals[0].Tags[0] == "changed" {
			t.Fatal("input alias")
		}
	}
	a, _ := SelectGoals(goals, "first", DefaultOptions())
	b, _ := SelectGoals(goals, "second", DefaultOptions())
	if reflect.DeepEqual(a.Goals, b.Goals) {
		t.Fatal("different seeds")
	}
	if strToInt64("42") != 42 || strToInt64("smw") != strToInt64("smw") {
		t.Fatal("seed hash")
	}
}
func TestLinesAndVictory(t *testing.T) {
	card := BingoCard{}
	for i := range card.Goals {
		card.Goals[i] = BingoGoal{TimeMin: 2, Risk: 2}
	}
	if math.Abs(Cost(card.Goals[0])-2.6) > 1e-9 {
		t.Fatal("risk weighting")
	}
	for _, cost := range LineCosts(card) {
		if math.Abs(cost-13) > 1e-9 {
			t.Fatal(cost)
		}
	}
	if math.Abs(EstimatedMinutes(card, Options{Rule: Standard, BaseRoute: 15})-28) > 1e-9 || math.Abs(EstimatedMinutes(card, Options{Rule: LineOnly, BaseRoute: 15})-13) > 1e-9 {
		t.Fatal("base route")
	}
	if HasLine([25]bool{}) {
		t.Fatal("empty line")
	}
	for _, line := range lines {
		var progress [25]bool
		for _, index := range line {
			progress[index] = true
		}
		if !HasLine(progress) {
			t.Fatal(line)
		}
		progress[line[0]] = false
		if HasLine(progress) {
			t.Fatal("partial line")
		}
	}
}
func TestScore(t *testing.T) {
	g := BingoGoal{Name: "goal", TimeMin: 5, Risk: 1, Tags: []string{"area:test", "type:clear"}}
	card := BingoCard{}
	if score(card, 1, g, 5) != 0 {
		t.Fatal("balanced empty board")
	}
	card.Goals[0] = g
	if score(card, 1, g, 5) != 3 {
		t.Fatal("area=2, type=1")
	}
	heavier := g
	heavier.TimeMin = 6
	if score(card, 1, heavier, 5) <= score(card, 1, g, 5) {
		t.Fatal("time deviation not penalized")
	}
}
func TestOptions(t *testing.T) {
	defaults := DefaultOptions()
	if defaults.MinTarget != 0 {
		t.Fatal("an upper limit must not introduce a hidden lower limit")
	}
	shortGoals := fixtureGoals(25)
	for i := range shortGoals {
		shortGoals[i].TimeMin = 1
	}
	defaults.Rule = LineOnly
	defaults.MaxTime = 10
	if _, err := SelectGoals(shortGoals, "short-race", defaults); err != nil {
		t.Fatal(err)
	}
	o, err := (Options{}).Normalize()
	if err != nil || o.Rule != Standard || o.MaxTime != DefaultMaxTime {
		t.Fatal(o, err)
	}
	for _, o := range []Options{{Rule: "bad"}, {MaxTime: -1}, {MaxTime: 1441}, {MinTarget: -1}, {MaxTime: 30, MinTarget: 31}, {BaseRoute: -1}, {BaseRoute: 1441}} {
		if _, err := o.Normalize(); err == nil {
			t.Fatal(o)
		}
	}
}
func TestValidation(t *testing.T) {
	if err := ValidateGoals(fixtureGoals(24)); err == nil {
		t.Fatal("short catalog")
	}
	changes := []func(*BingoGoal){func(g *BingoGoal) { g.Name = "" }, func(g *BingoGoal) { g.Name = "goal-1" }, func(g *BingoGoal) { g.TimeMin = 0 }, func(g *BingoGoal) { g.Exec = 4 }, func(g *BingoGoal) { g.Risk = 0 }, func(g *BingoGoal) { g.Tags = nil }, func(g *BingoGoal) { g.Tags = []string{"invalid"} }, func(g *BingoGoal) { g.Tags = []string{"area:a", "type:t", "type:t"} }, func(g *BingoGoal) { g.Tags = []string{"area:a", "area:b", "area:c", "type:t"} }}
	for i, change := range changes {
		goals := fixtureGoals(25)
		change(&goals[0])
		if err := ValidateGoals(goals); err == nil {
			t.Fatalf("case %d", i)
		}
	}
}
func TestTimeConstraints(t *testing.T) {
	goals := fixtureGoals(25)
	for _, o := range []Options{{MaxTime: 40, MinTarget: 40, BaseRoute: 15, Rule: Standard}, {MaxTime: 25, MinTarget: 25, BaseRoute: 500, Rule: LineOnly}} {
		card, err := SelectGoals(goals, "seed", o)
		if err != nil {
			t.Fatal(err)
		}
		if EstimatedMinutes(card, o) != float64(o.MaxTime) {
			t.Fatal("range")
		}
	}
	for _, o := range []Options{{MaxTime: 39, BaseRoute: 15, Rule: Standard}, {MaxTime: 24, Rule: LineOnly}, {MaxTime: 50, MinTarget: 26, Rule: LineOnly}, {Rule: "bad"}} {
		if _, err := SelectGoals(goals, "seed", o); err == nil {
			t.Fatal(o)
		}
	}
	if _, err := SelectGoals(goals[:24], "seed", DefaultOptions()); err == nil {
		t.Fatal("short")
	}
	// Lower bound is feasible, but every card contains one slow goal and thus
	// still has cheap lines. A narrow requested lower bound exhausts the search.
	goals[0].TimeMin = 100
	if _, err := SelectGoals(goals, "seed", Options{MaxTime: 110, MinTarget: 100, Rule: LineOnly}); err == nil {
		t.Fatal("must not return out-of-range card")
	}
}
func TestDataLifecycle(t *testing.T) {
	dataMu.Lock()
	data = nil
	dataMu.Unlock()
	if GetBingoData() != nil {
		t.Fatal("expected nil")
	}
	if _, err := CreateBingoCard(""); err == nil {
		t.Fatal("uninitialized")
	}
	path := filepath.Join(t.TempDir(), "goals.json")
	if err := InitData(path); err == nil {
		t.Fatal("missing file")
	}
	for _, contents := range []string{"{", `{"goals":[]}`} {
		if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
		if err := InitData(path); err == nil {
			t.Fatal("invalid data")
		}
	}
	bytes, err := json.Marshal(BingoData{Goals: fixtureGoals(30)})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, bytes, 0600); err != nil {
		t.Fatal(err)
	}
	if err := InitData(path); err != nil {
		t.Fatal(err)
	}
	card, err := CreateBingoCard("")
	if err != nil || card.Seed == "" {
		t.Fatal(err)
	}
	replay, err := CreateBingoCard(card.Seed)
	if err != nil || !reflect.DeepEqual(card, replay) {
		t.Fatal("generated seed replay", err)
	}
	snapshot := GetBingoData()
	snapshot.Goals[0].Tags[0] = "changed"
	if GetBingoData().Goals[0].Tags[0] == "changed" {
		t.Fatal("mutable global")
	}
	copy := card.Clone()
	copy.Goals[0].Tags[0] = "changed"
	if card.Goals[0].Tags[0] == "changed" {
		t.Fatal("mutable card")
	}
	if err := InitData("missing-file"); err == nil {
		t.Fatal("missing")
	}
	if len(GetBingoData().Goals) != 30 {
		t.Fatal("failed load destroyed catalog")
	}
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := InitData(path); err != nil {
				t.Error(err)
			}
			_, err := CreateBingoCard("race")
			if err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
}
func TestProjectCatalog(t *testing.T) {
	if err := InitData("../bingo.json"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 200; i++ {
		card, err := CreateBingoCard(fmt.Sprint(i))
		if err != nil {
			t.Fatal(err)
		}
		assertUniqueWorlds(t, card)
		for cell, goal := range card.Goals {
			if goal.DisabledReason != "" || !canPlaceGroups(card, cell, goal.ConflictGroups) {
				t.Fatal("pending or conflicting goal", goal)
			}
		}
		estimate := EstimatedMinutes(card, DefaultOptions())
		if estimate < float64(DefaultMinTarget) || estimate > float64(DefaultMaxTime) {
			t.Fatal(estimate)
		}
	}
}
