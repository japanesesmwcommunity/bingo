package bingo

import "testing"

func TestForestGoalsShareOnlyTheSameClear(t *testing.T) {
	times := map[string]int{
		"unlock:forest:1:normal": 2, "unlock:forest:1:secret": 3,
		"unlock:forest:2:normal": 2, "unlock:forest:3:secret": 2,
		"unlock:forest:ghost:normal": 2, "unlock:forest:4:secret": 3,
		"unlock:forest:secret:normal": 2,
	}
	castle := BingoGoal{TimeMin: 20, Risk: 1, RouteAreas: []string{"unlock:forest:1:normal", "unlock:forest:2:normal", "unlock:forest:3:secret"}}
	fortress := BingoGoal{TimeMin: 25, Risk: 1, RouteAreas: []string{"unlock:forest:1:secret", "unlock:forest:ghost:normal", "unlock:forest:4:secret", "unlock:forest:secret:normal"}}
	house := BingoGoal{TimeMin: 10, Risk: 1, RouteAreas: []string{"unlock:forest:1:secret"}}
	if got := lineCost([5]BingoGoal{castle, fortress}, times); got != 45 {
		t.Fatal("different goals in course 1 must not overlap", got)
	}
	if got := lineCost([5]BingoGoal{castle, fortress, house}, times); got != 52 {
		t.Fatal("only course 1 secret is shared", got)
	}
	var card BingoCard
	card.RouteAreaTimes = times
	card.Goals[0], card.Goals[1], card.Goals[2] = castle, fortress, house
	card.BowserRoutes = []FinishRoute{{Name: "forest star", TimeMin: 30, RouteAreas: fortress.RouteAreas}}
	if got := RaceLineCosts(card, Options{Rule: Standard})[0]; got != 72 {
		t.Fatal("finish must deduct each of the four shared clears once", got)
	}
	if got := RaceLineCosts(card, Options{Rule: LineOnly})[0]; got != 52 {
		t.Fatal("line-only must omit Bowser", got)
	}
}
