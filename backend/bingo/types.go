package bingo

type BingoData struct {
	Goals          []BingoGoal    `json:"goals"`
	RouteAreaTimes map[string]int `json:"routeAreaTimes,omitempty"`
	BowserRoutes   []FinishRoute  `json:"bowserRoutes,omitempty"`
}

// FinishRoute describes one start-to-Bowser route. Only unlock work is shared;
// movement, re-entry and the fight remain in TimeMin.
type FinishRoute struct {
	Name       string   `json:"name"`
	TimeMin    int      `json:"timeMin"`
	RouteAreas []string `json:"routeAreas,omitempty"`
}

// TimeMin includes access and preparation from a fresh game, before risk weighting.
// Existing catalog values are provisional until measured.
type BingoGoal struct {
	DisabledReason string   `json:"disabledReason,omitempty"`
	ConflictGroups []string `json:"conflictGroups,omitempty"`
	Name           string   `json:"name"`
	World          string   `json:"world"`
	Level          string   `json:"level"`
	TimeMin        int      `json:"timeMin"`
	Exec           int      `json:"exec"`
	Risk           int      `json:"risk"`
	Tags           []string `json:"tags"`
	RouteAreas     []string `json:"routeAreas,omitempty"`
}

type BingoCard struct {
	Goals          [25]BingoGoal  `json:"goals"`
	Seed           string         `json:"seed"`
	RouteAreaTimes map[string]int `json:"routeAreaTimes,omitempty"`
	BowserRoutes   []FinishRoute  `json:"bowserRoutes,omitempty"`
}

type Rule string

const (
	Standard         Rule = "standard"
	LineOnly         Rule = "line"
	DefaultMaxTime        = 90
	DefaultMinTarget      = 0
	DefaultBaseRoute      = 15
)

type Options struct {
	MaxTime   int  `json:"maxTime"`
	MinTarget int  `json:"minTarget"`
	BaseRoute int  `json:"baseRoute"`
	Rule      Rule `json:"rule"`
}

// Lines contains the five rows, five columns and two diagonals.
var lines = [12][5]int{
	{0, 1, 2, 3, 4}, {5, 6, 7, 8, 9}, {10, 11, 12, 13, 14}, {15, 16, 17, 18, 19}, {20, 21, 22, 23, 24},
	{0, 5, 10, 15, 20}, {1, 6, 11, 16, 21}, {2, 7, 12, 17, 22}, {3, 8, 13, 18, 23}, {4, 9, 14, 19, 24},
	{0, 6, 12, 18, 24}, {4, 8, 12, 16, 20},
}

var placementOrder = [25]int{12, 0, 4, 6, 8, 16, 18, 20, 24, 1, 2, 3, 5, 7, 9, 10, 11, 13, 14, 15, 17, 19, 21, 22, 23}
