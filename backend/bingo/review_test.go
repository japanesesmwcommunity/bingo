package bingo

import (
	"math"
	"reflect"
	"testing"
)

func TestPendingGoalsAndConflictGroups(t *testing.T) {
	goals := fixtureGoals(30)
	for i := 25; i < len(goals); i++ {
		goals[i].DisabledReason = "条件未確定"
		goals[i].TimeMin = 1440
	}
	goals[0].ConflictGroups = []string{"nested"}
	goals[1].ConflictGroups = []string{"nested"}
	card, err := SelectGoals(goals, "pending", DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	replay, err := SelectGoals(goals, "pending", DefaultOptions())
	if err != nil || !reflect.DeepEqual(card, replay) {
		t.Fatal("not reproducible", err)
	}
	for cell, goal := range card.Goals {
		if goal.DisabledReason != "" || !canPlaceGroups(card, cell, goal.ConflictGroups) {
			t.Fatal("pending/conflicting goal selected", goal)
		}
	}
	card.Goals[0].ConflictGroups = []string{"mutated"}
	if goals[0].ConflictGroups[0] != "nested" {
		t.Fatal("input alias")
	}
	goals[24].DisabledReason = "未確認"
	if _, err := SelectGoals(goals, "pending", DefaultOptions()); err == nil {
		t.Fatal("fewer than 25 active goals accepted")
	}
	if err := ValidateGoals(goals); err == nil {
		t.Fatal("unsuitable catalog saved")
	}
	for _, line := range lines {
		c := BingoCard{}
		c.Goals[line[0]].ConflictGroups = []string{"nested"}
		for _, cell := range line[1:] {
			if canPlaceGroups(c, cell, []string{"nested"}) || !canPlaceGroups(c, cell, []string{"other"}) || !canPlaceGroups(c, cell, nil) {
				t.Fatal("line exclusion")
			}
		}
		if !canPlaceGroups(c, line[0], []string{"nested"}) {
			t.Fatal("self conflict")
		}
	}
	c := BingoCard{}
	c.Goals[0].ConflictGroups = []string{"nested"}
	if !canPlaceGroups(c, 7, []string{"nested"}) {
		t.Fatal("unrelated line rejected")
	}
	for _, groups := range [][]string{{""}, {" x"}, {"x "}, {"a,b"}, {"x", "x"}} {
		g := fixtureGoals(25)
		g[0].ConflictGroups = groups
		if err := ValidateGoals(g); err == nil {
			t.Fatal("invalid group accepted", groups)
		}
	}
	g := fixtureGoals(26)
	g[0].DisabledReason = " "
	if err := ValidateGoals(g); err == nil {
		t.Fatal("blank reason accepted")
	}
}

func TestFinishRoutesChoosePerLineAndDeductUnlocksOnce(t *testing.T) {
	card := BingoCard{RouteAreaTimes: map[string]int{"star": 8, "back": 10}, BowserRoutes: []FinishRoute{
		{Name: "star", TimeMin: 12, RouteAreas: []string{"star"}},
		{Name: "back", TimeMin: 15, RouteAreas: []string{"back"}},
	}}
	for i := range card.Goals {
		card.Goals[i] = BingoGoal{Name: "goal", TimeMin: 12, Risk: 1}
	}
	for i := 0; i < 5; i++ {
		card.Goals[i].RouteAreas = []string{"back"}
	}
	card.Goals[5].RouteAreas = []string{"star"}
	costs := RaceLineCosts(card, Options{Rule: Standard, BaseRoute: 500})
	// First row: 60 - 4*10 + (15-10) = 25. Back beats standalone star.
	if costs[0] != 25 || costs[1] != 64 || costs[2] != 72 {
		t.Fatal(costs)
	}
	if EstimatedMinutes(card, Options{Rule: Standard}) != 25 {
		t.Fatal("wrong winning line")
	}
	if RaceLineCosts(card, Options{Rule: LineOnly}) != LineCosts(card) {
		t.Fatal("finish cost added to line-only")
	}
	if RaceLineCosts(card, Options{}) != costs {
		t.Fatal("default rule")
	}
	// Risk premiums remain attached to tasks, while shared unlock work is only once.
	card.Goals[0].Risk = 3
	if math.Abs(RaceLineCosts(card, Options{})[0]-32.2) > 1e-9 {
		t.Fatal("risk premium lost")
	}
	card.BowserRoutes = nil
	if RaceLineCosts(card, Options{BaseRoute: 15})[2] != 75 {
		t.Fatal("legacy fallback")
	}
}

func TestFinishRouteValidationAndBounds(t *testing.T) {
	catalog := BingoData{Goals: fixtureGoals(25), RouteAreaTimes: map[string]int{"shared": 4, "ドーナツへいや": 4}, BowserRoutes: []FinishRoute{{Name: "star", TimeMin: 6, RouteAreas: []string{"shared"}}}}
	for i := range catalog.Goals {
		catalog.Goals[i].RouteAreas = []string{"shared"}
	}
	options := Options{Rule: Standard, MinTarget: 11, MaxTime: 11, BaseRoute: 1440}
	card, err := SelectCatalog(catalog, "finish", options)
	if err != nil || EstimatedMinutes(card, options) != 11 {
		t.Fatal("shared finish rejected by bounds", err)
	}
	if _, err := SelectCatalog(catalog, "finish", Options{Rule: Standard, MaxTime: 10}); err == nil {
		t.Fatal("over-budget result")
	}
	if _, err := SelectCatalog(catalog, "finish", Options{Rule: Standard, MinTarget: 32, MaxTime: 100}); err == nil {
		t.Fatal("impossible lower target")
	}
	for _, route := range []FinishRoute{
		{Name: "", TimeMin: 6}, {Name: " spaced", TimeMin: 6}, {Name: "ok", TimeMin: 0}, {Name: "ok", TimeMin: 1441},
		{Name: "ok", TimeMin: 6, RouteAreas: []string{"missing"}},
		{Name: "ok", TimeMin: 10, RouteAreas: []string{"shared", "shared"}},
		{Name: "ok", TimeMin: 4, RouteAreas: []string{"shared"}},
		{Name: "ok", TimeMin: 6, RouteAreas: []string{"ドーナツへいや"}},
	} {
		bad := catalog
		bad.BowserRoutes = []FinishRoute{route}
		if err := ValidateData(bad); err == nil {
			t.Fatal("invalid finish accepted", route)
		}
	}
	catalog.BowserRoutes = append(catalog.BowserRoutes, catalog.BowserRoutes[0])
	if err := ValidateData(catalog); err == nil {
		t.Fatal("duplicate route name")
	}
}

func TestFinishAndRelationSnapshots(t *testing.T) {
	t.Cleanup(func() { _ = InitData("../bingo.json") })
	catalog := BingoData{Goals: fixtureGoals(25), RouteAreaTimes: map[string]int{"shared": 4}, BowserRoutes: []FinishRoute{{Name: "star", TimeMin: 6, RouteAreas: []string{"shared"}}}}
	catalog.Goals[0].ConflictGroups = []string{"nested"}
	if err := ReplaceData(catalog); err != nil {
		t.Fatal(err)
	}
	card, err := CreateCard("snapshot", DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	copy := GetBingoData()
	clone := card.Clone()
	catalog.BowserRoutes[0].RouteAreas[0] = "input mutation"
	copy.BowserRoutes[0].TimeMin = 999
	copy.BowserRoutes[0].RouteAreas[0] = "read mutation"
	copy.Goals[0].ConflictGroups[0] = "read mutation"
	clone.BowserRoutes[0].RouteAreas[0] = "card mutation"
	for i := range clone.Goals {
		if len(clone.Goals[i].ConflictGroups) > 0 {
			clone.Goals[i].ConflictGroups[0] = "card mutation"
		}
	}
	actual := GetBingoData()
	if actual.BowserRoutes[0].TimeMin != 6 || actual.BowserRoutes[0].RouteAreas[0] != "shared" || actual.Goals[0].ConflictGroups[0] != "nested" || card.BowserRoutes[0].RouteAreas[0] != "shared" {
		t.Fatal("snapshot alias")
	}
	for _, g := range card.Goals {
		if len(g.ConflictGroups) > 0 && g.ConflictGroups[0] != "nested" {
			t.Fatal("group alias")
		}
	}
	if cloneFinishRoutes(nil) != nil {
		t.Fatal("nil snapshot")
	}
}

func TestScoreBalancesCorrectedTotals(t *testing.T) {
	card := BingoCard{RouteAreaTimes: map[string]int{"shared": 8}}
	goal := BingoGoal{Name: "goal", TimeMin: 10, Risk: 1, RouteAreas: []string{"shared"}}
	for i := range card.Goals {
		card.Goals[i] = goal
	}
	if math.Abs(score(card, 12, goal, 10)) > 1e-9 {
		t.Fatal("uniform shared routes penalized")
	}
	longer := goal
	longer.TimeMin = 15
	if score(card, 12, longer, 10) <= score(card, 12, goal, 10) {
		t.Fatal("imbalance not penalized")
	}
}
