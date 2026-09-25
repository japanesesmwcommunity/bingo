package bingo

import (
	"encoding/json"
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
	for _, id := range []string{"unlock:vanilla-lower", "unlock:vanilla-upper", "unlock:star-front", "unlock:valley-back"} {
		if !seen[id] {
			t.Fatal("missing branch segment", id)
		}
	}
	if len(config.Segments) != 8 {
		t.Fatal("expected eight whole-course groups")
	}
	for _, id := range []string{"バニラドーム", "vanilla:common", "vanilla:plateau", "forest:castle", "star:1-secret"} {
		if !loadRetiredRouteAreas(routeSegmentConfig)[id] {
			t.Fatal("ambiguous old definition remains active", id)
		}
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
	times := map[string]int{"unlock:yoster": 4, "unlock:vanilla-lower": 6, "unlock:vanilla-upper": 7, "unlock:butter": 3}
	goals := [5]BingoGoal{
		{Name: "cheese route", TimeMin: 20, Risk: 1, RouteAreas: []string{"unlock:yoster", "unlock:vanilla-lower"}},
		{Name: "butter route", TimeMin: 25, Risk: 1, RouteAreas: []string{"unlock:yoster", "unlock:vanilla-upper", "unlock:butter"}},
		{Name: "another cheese goal", TimeMin: 12, Risk: 1, RouteAreas: []string{"unlock:yoster", "unlock:vanilla-lower"}},
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
