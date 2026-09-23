package bingo

import (
	"fmt"
	"math"
	"strings"
)

// ValidateData checks route references against the catalog's shared segment times.
func ValidateData(catalog BingoData) error {
	if err := ValidateGoals(catalog.Goals); err != nil {
		return err
	}
	for area, minutes := range catalog.RouteAreaTimes {
		if area == "" || area != strings.TrimSpace(area) || strings.Contains(area, ",") || minutes < 1 || minutes > 1440 {
			return fmt.Errorf("経由エリアの名前と基準時間（1〜1440分の整数）を確認してください: %q", area)
		}
	}
	for _, goal := range catalog.Goals {
		seen := make(map[string]bool)
		for _, area := range goal.RouteAreas {
			if _, ok := catalog.RouteAreaTimes[area]; !ok {
				return fmt.Errorf("%s: 未登録の経由エリアです: %q", goal.Name, area)
			}
			if seen[area] {
				return fmt.Errorf("%s: 経由エリアが重複しています: %q", goal.Name, area)
			}
			seen[area] = true
		}
		if routeMinutes(goal, catalog.RouteAreaTimes) > goal.TimeMin {
			return fmt.Errorf("%s: 経由エリアの合計時間が最低所要時間を超えています", goal.Name)
		}
	}
	names := make(map[string]bool)
	for _, route := range catalog.BowserRoutes {
		if route.Name == "" || route.Name != strings.TrimSpace(route.Name) || names[route.Name] || route.TimeMin < 1 || route.TimeMin > 1440 {
			return fmt.Errorf("invalid Bowser route: %q", route.Name)
		}
		names[route.Name] = true
		seen := make(map[string]bool)
		total := 0
		for _, area := range route.RouteAreas {
			minutes, ok := catalog.RouteAreaTimes[area]
			if !ok || seen[area] || retiredRouteAreas[area] {
				return fmt.Errorf("invalid Bowser route segment: %q", area)
			}
			seen[area] = true
			total += minutes
		}
		if total >= route.TimeMin {
			return fmt.Errorf("クッパ経路には移動・再入場・戦闘の時間を残してください: %s", route.Name)
		}
	}
	return nil
}

func routeMinutes(goal BingoGoal, times map[string]int) int {
	minutes := 0
	for _, area := range goal.RouteAreas {
		minutes += times[area]
	}
	return minutes
}

// lineCost counts each shared segment's baseline time only once. Risk premiums
// stay attached to individual goals, as they did before route correction.
func lineCost(goals [5]BingoGoal, times map[string]int) float64 {
	total := 0.0
	seen := make(map[string]bool)
	for _, goal := range goals {
		total += Cost(goal)
		for _, area := range goal.RouteAreas {
			// Whole-area totals cannot establish which branch was shared.
			if retiredRouteAreas[area] {
				continue
			}
			if seen[area] {
				total -= float64(times[area])
			}
			seen[area] = true
		}
	}
	return total
}

func cloneFinishRoutes(routes []FinishRoute) []FinishRoute {
	if routes == nil {
		return nil
	}
	result := make([]FinishRoute, len(routes))
	for i, route := range routes {
		route.RouteAreas = append([]string(nil), route.RouteAreas...)
		result[i] = route
	}
	return result
}

// RaceLineCosts evaluates each line together with the cheapest compatible
// finish-route approximation. It does not solve equipment or movement order.
// Without measured finish routes, BaseRoute remains the legacy provisional cost.
func RaceLineCosts(card BingoCard, options Options) [12]float64 {
	costs := LineCosts(card)
	if options.Rule == LineOnly {
		return costs
	}
	for i, line := range lines {
		remaining := float64(options.BaseRoute)
		if len(card.BowserRoutes) > 0 {
			seen := make(map[string]bool)
			for _, cell := range line {
				for _, area := range card.Goals[cell].RouteAreas {
					seen[area] = true
				}
			}
			remaining = math.Inf(1)
			for _, route := range card.BowserRoutes {
				cost := float64(route.TimeMin)
				for _, area := range route.RouteAreas {
					if seen[area] && !retiredRouteAreas[area] {
						cost -= float64(card.RouteAreaTimes[area])
					}
				}
				remaining = math.Min(remaining, cost)
			}
		}
		costs[i] += remaining
	}
	return costs
}

// Explicit same-line exclusion is independent of location and route overlap.
// A group can conservatively keep nested or near-identical goals apart.
func canPlaceGroups(card BingoCard, cell int, groups []string) bool {
	for _, line := range lines {
		contains := false
		for _, index := range line {
			contains = contains || index == cell
		}
		if !contains {
			continue
		}
		for _, index := range line {
			if index == cell {
				continue
			}
			for _, a := range groups {
				for _, b := range card.Goals[index].ConflictGroups {
					if a == b {
						return false
					}
				}
			}
		}
	}
	return true
}
