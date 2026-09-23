package bingo

import (
	"math"
	"reflect"
	"testing"
)

func TestRouteValidation(t *testing.T) {
	for _, tc := range []struct {
		name    string
		times   map[string]int
		routes  []string
		invalid bool
	}{
		{"legacy", nil, nil, false},
		{"multiple segments", map[string]int{"a": 1, "b": 1, "c": 1, "d": 2}, []string{"a", "b", "c", "d"}, false},
		{"unused segment", map[string]int{"a": 1440}, nil, false},
		{"empty name", map[string]int{"": 1}, nil, true},
		{"space", map[string]int{" a": 1}, nil, true},
		{"comma", map[string]int{"a,b": 1}, nil, true},
		{"zero", map[string]int{"a": 0}, nil, true},
		{"negative", map[string]int{"a": -1}, nil, true},
		{"too long", map[string]int{"a": 1441}, nil, true},
		{"missing reference", nil, []string{"a"}, true},
		{"duplicate", map[string]int{"a": 1}, []string{"a", "a"}, true},
		{"exceeds goal", map[string]int{"a": 3, "b": 3}, []string{"a", "b"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			catalog := BingoData{Goals: fixtureGoals(25), RouteAreaTimes: tc.times}
			catalog.Goals[0].RouteAreas = tc.routes
			if err := ValidateData(catalog); (err != nil) != tc.invalid {
				t.Fatal(err)
			}
		})
	}
}

func TestSharedRouteLineCosts(t *testing.T) {
	card := BingoCard{RouteAreaTimes: map[string]int{"common": 8}}
	for i := range card.Goals {
		card.Goals[i] = BingoGoal{TimeMin: 10, Risk: 1, RouteAreas: []string{"common"}}
	}
	for _, cost := range LineCosts(card) {
		if cost != 18 { // 5 * 10 - 4 * 8, across all twelve lines.
			t.Fatal(cost)
		}
	}
	if EstimatedMinutes(card, Options{Rule: Standard, BaseRoute: 15}) != 33 || EstimatedMinutes(card, Options{Rule: LineOnly}) != 18 {
		t.Fatal("route correction or rule adjustment lost")
	}
	goals := [5]BingoGoal{
		{TimeMin: 10, Risk: 1, RouteAreas: []string{"a", "b"}},
		{TimeMin: 12, Risk: 2, RouteAreas: []string{"a", "c"}},
		{TimeMin: 8, Risk: 3, RouteAreas: []string{"b"}},
		{TimeMin: 5, Risk: 1},
		{TimeMin: 5, Risk: 1},
	}
	times := map[string]int{"a": 4, "b": 2, "c": 3}
	if cost := lineCost(goals, times); math.Abs(cost-42.4) > 1e-9 {
		t.Fatal("only duplicate baselines should be subtracted; keep risk premiums", cost)
	}
	if cost := lineCost(goals, nil); math.Abs(cost-48.4) > 1e-9 {
		t.Fatal("unconfigured segments must not reduce the estimate", cost)
	}
	card.RouteAreaTimes["common"] = 10
	if LineCosts(card)[0] != 10 {
		t.Fatal("shared travel cannot shorten the longest individual goal")
	}
}

func TestRouteAwareSelectionAndScore(t *testing.T) {
	catalog := BingoData{Goals: fixtureGoals(25), RouteAreaTimes: map[string]int{"common": 8}}
	for i := range catalog.Goals {
		catalog.Goals[i].TimeMin = 10
		catalog.Goals[i].RouteAreas = []string{"common"}
	}
	o := Options{Rule: LineOnly, MaxTime: 18, MinTarget: 18}
	a, err := SelectCatalog(catalog, "overlap", o)
	if err != nil || EstimatedMinutes(a, o) != 18 {
		t.Fatal("the uncorrected sum must not reject a feasible card", err)
	}
	b, err := SelectCatalog(catalog, "overlap", o)
	if err != nil || !reflect.DeepEqual(a, b) {
		t.Fatal("route-aware generation must be deterministic", err)
	}
	for _, bounds := range []Options{
		{Rule: LineOnly, MaxTime: 17},
		{Rule: LineOnly, MaxTime: 9},
		{Rule: LineOnly, MaxTime: 60, MinTarget: 51},
		{Rule: "bad"},
	} {
		if _, err := SelectCatalog(catalog, "overlap", bounds); err == nil {
			t.Fatal("out-of-range card", bounds)
		}
	}
	card := BingoCard{RouteAreaTimes: map[string]int{"common": 8}}
	goal := BingoGoal{Name: "goal", TimeMin: 10, Risk: 1, RouteAreas: []string{"common"}}
	for i := 0; i < 5; i++ {
		card.Goals[i] = goal
	}
	if math.Abs(score(card, 2, goal, 10)-44.0/9) > 1e-9 {
		t.Fatal("placement score must use the corrected row cost")
	}
	// Neither returned cards nor callers can change each other's route snapshots.
	a.RouteAreaTimes["common"] = 1
	a.Goals[0].RouteAreas[0] = "changed"
	if catalog.RouteAreaTimes["common"] != 8 || catalog.Goals[0].RouteAreas[0] != "common" || b.RouteAreaTimes["common"] != 8 {
		t.Fatal("route snapshot alias")
	}
}

func TestRouteCatalogAndCardSnapshots(t *testing.T) {
	t.Cleanup(func() { _ = InitData("../bingo.json") })
	catalog := BingoData{Goals: fixtureGoals(25), RouteAreaTimes: map[string]int{"a": 4}}
	for i := range catalog.Goals {
		catalog.Goals[i].RouteAreas = []string{"a"}
	}
	if err := ReplaceData(catalog); err != nil {
		t.Fatal(err)
	}
	catalog.RouteAreaTimes["a"] = 500
	catalog.Goals[0].RouteAreas[0] = "changed"
	copy := GetBingoData()
	if copy.RouteAreaTimes["a"] != 4 || copy.Goals[0].RouteAreas[0] != "a" {
		t.Fatal("input aliases global data")
	}
	copy.RouteAreaTimes["a"] = 1
	copy.Goals[0].RouteAreas[0] = "changed"
	card, err := CreateCard("snapshot", Options{Rule: LineOnly, MaxTime: 9})
	if err != nil || LineCosts(card)[0] != 9 {
		t.Fatal("card must use the stored route snapshot", err)
	}
	cloned := card.Clone()
	cloned.RouteAreaTimes["a"] = 1
	cloned.Goals[0].RouteAreas[0] = "changed"
	if card.RouteAreaTimes["a"] != 4 || card.Goals[0].RouteAreas[0] != "a" {
		t.Fatal("card clone aliases original")
	}
	if err := ReplaceData(catalog); err == nil || GetBingoData().RouteAreaTimes["a"] != 4 {
		t.Fatal("invalid replacement changed live routes")
	}
}
