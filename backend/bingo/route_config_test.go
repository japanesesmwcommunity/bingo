package bingo

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestRouteSegmentConfiguration(t *testing.T) {
	var config struct {
		Segments []struct {
			ID    string
			Label string
		}
		RetiredAreas []string
	}
	if err := json.Unmarshal(routeSegmentConfig, &config); err != nil {
		t.Fatal(err)
	}
	seen := make(map[string]bool)
	for _, segment := range config.Segments {
		if segment.ID == "" || segment.Label == "" || seen[segment.ID] || retiredRouteAreas[segment.ID] {
			t.Fatal(segment)
		}
		seen[segment.ID] = true
	}
	for _, id := range []string{"vanilla:common", "vanilla:cheese", "vanilla:plateau", "plateau:butter"} {
		if !seen[id] {
			t.Fatal("missing branch segment", id)
		}
	}
	if !reflect.DeepEqual(loadRetiredRouteAreas(routeSegmentConfig), map[string]bool{"バニラドーム": true, "バニラだいち": true, "ドーナツへいや": true, "まよいのもり": true, "チョコレーとう": true, "まおうクッパのたに": true, "スターロード": true}) {
		t.Fatal("retired areas")
	}
	t.Run("invalid bundled configuration", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Fatal("invalid configuration accepted")
			}
		}()
		loadRetiredRouteAreas([]byte("{"))
	})
}

func TestBranchSegmentsOnlyDeductTheActualSharedPath(t *testing.T) {
	times := map[string]int{"vanilla:common": 4, "vanilla:cheese": 6, "vanilla:plateau": 7, "plateau:butter": 3}
	goals := [5]BingoGoal{
		{Name: "cheese route", TimeMin: 20, Risk: 1, RouteAreas: []string{"vanilla:common", "vanilla:cheese"}},
		{Name: "butter route", TimeMin: 25, Risk: 1, RouteAreas: []string{"vanilla:common", "vanilla:plateau", "plateau:butter"}},
		{Name: "another cheese goal", TimeMin: 12, Risk: 1, RouteAreas: []string{"vanilla:common", "vanilla:cheese"}},
		{TimeMin: 8, Risk: 1},
		{TimeMin: 5, Risk: 1},
	}
	// 70 total: common appears three times (subtract 8); cheese twice (subtract 6).
	// Neither plateau nor butter is shared with the cheese branch.
	if got := lineCost(goals, times); got != 56 {
		t.Fatal(got)
	}
	goals[2].RouteAreas = nil
	if got := lineCost(goals, times); got != 66 {
		t.Fatal("only the two common segments should overlap", got)
	}
	for oldArea := range retiredRouteAreas {
		legacy := [5]BingoGoal{
			{TimeMin: 20, Risk: 1, RouteAreas: []string{oldArea}},
			{TimeMin: 25, Risk: 1, RouteAreas: []string{oldArea}},
		}
		if got := lineCost(legacy, map[string]int{oldArea: 10}); got != 45 {
			t.Fatal("ambiguous legacy area was deducted", got)
		}
	}
}
