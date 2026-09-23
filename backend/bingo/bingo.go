package bingo

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"maps"
	"math"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/google/uuid"
)

var dataMu sync.RWMutex
var data *BingoData

// A failed load leaves the previous catalog intact and can be retried.
func InitData(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var next BingoData
	if err := json.Unmarshal(b, &next); err != nil {
		return err
	}
	return ReplaceData(next)
}

func ReplaceData(next BingoData) error {
	if err := ValidateData(next); err != nil {
		return err
	}
	dataMu.Lock()
	defer dataMu.Unlock()
	copy := &BingoData{Goals: make([]BingoGoal, len(next.Goals)), RouteAreaTimes: maps.Clone(next.RouteAreaTimes), BowserRoutes: cloneFinishRoutes(next.BowserRoutes)}
	for i, goal := range next.Goals {
		copy.Goals[i] = cloneGoal(goal)
	}
	data = copy
	return nil
}

func GetBingoData() *BingoData {
	dataMu.RLock()
	defer dataMu.RUnlock()
	if data == nil {
		return nil
	}
	copy := &BingoData{Goals: make([]BingoGoal, len(data.Goals)), RouteAreaTimes: maps.Clone(data.RouteAreaTimes), BowserRoutes: cloneFinishRoutes(data.BowserRoutes)}
	for i, g := range data.Goals {
		copy.Goals[i] = cloneGoal(g)
	}
	return copy
}

func cloneGoal(g BingoGoal) BingoGoal {
	g.Tags = append([]string(nil), g.Tags...)
	g.RouteAreas = append([]string(nil), g.RouteAreas...)
	g.ConflictGroups = append([]string(nil), g.ConflictGroups...)
	return g
}

func (bc BingoCard) Clone() BingoCard {
	bc.RouteAreaTimes = maps.Clone(bc.RouteAreaTimes)
	bc.BowserRoutes = cloneFinishRoutes(bc.BowserRoutes)
	for i, g := range bc.Goals {
		bc.Goals[i] = cloneGoal(g)
	}
	return bc
}

func (bc BingoCard) GetSeed() string { return bc.Seed }

func DefaultOptions() Options {
	return Options{MaxTime: DefaultMaxTime, MinTarget: DefaultMinTarget, BaseRoute: DefaultBaseRoute, Rule: Standard}
}

func (o Options) Normalize() (Options, error) {
	if o.Rule == "" {
		o.Rule = Standard
	}
	if o.MaxTime == 0 {
		o.MaxTime = DefaultMaxTime
	}
	if o.Rule != Standard && o.Rule != LineOnly {
		return o, fmt.Errorf("unknown rule")
	}
	if o.MaxTime < 1 || o.MaxTime > 1440 || o.MinTarget < 0 || o.MinTarget > o.MaxTime || o.BaseRoute < 0 || o.BaseRoute > 1440 {
		return o, fmt.Errorf("invalid time range (0 <= minTarget <= maxTime <= 1440)")
	}
	return o, nil
}

func ValidateGoals(goals []BingoGoal) error {
	if len(goals) < 25 {
		return fmt.Errorf("at least 25 distinct goals are required")
	}
	active := 0
	seen := make(map[[3]string]bool)
	for _, g := range goals {
		if g.DisabledReason == "" {
			active++
		}
		name := strings.TrimSpace(g.Name)
		identity := [3]string{g.World, g.Level, name}
		if name == "" || seen[identity] {
			return fmt.Errorf("empty or duplicate goal: %q", g.Name)
		}
		seen[identity] = true
		if g.DisabledReason != strings.TrimSpace(g.DisabledReason) {
			return fmt.Errorf("invalid disabled reason: %s", g.Name)
		}
		groups := make(map[string]bool)
		for _, group := range g.ConflictGroups {
			if group == "" || group != strings.TrimSpace(group) || strings.Contains(group, ",") || groups[group] {
				return fmt.Errorf("invalid conflict group: %s", g.Name)
			}
			groups[group] = true
		}
		if g.World != strings.TrimSpace(g.World) || g.Level != strings.TrimSpace(g.Level) || (g.World == "" && g.Level != "") {
			return fmt.Errorf("invalid world/level: %s", g.Name)
		}
		if g.TimeMin < 1 || g.TimeMin > 1440 || g.Exec < 1 || g.Exec > 3 || g.Risk < 1 || g.Risk > 3 {
			return fmt.Errorf("invalid metadata: %s", g.Name)
		}
		area, kind := 0, 0
		tags := make(map[string]bool)
		for _, tag := range g.Tags {
			if tags[tag] {
				return fmt.Errorf("duplicate tag: %s", tag)
			}
			tags[tag] = true
			if strings.HasPrefix(tag, "area:") && len(tag) > 5 {
				area++
			} else if strings.HasPrefix(tag, "type:") && len(tag) > 5 {
				kind++
			} else {
				return fmt.Errorf("invalid tag: %s", tag)
			}
		}
		if area < 1 || area > 2 || kind < 1 || kind > 2 {
			return fmt.Errorf("area/type tags required: %s", g.Name)
		}
	}
	if active < 25 {
		return fmt.Errorf("抽選対象のお題が25件以上必要です（保留中のお題は除外されます）")
	}
	return nil
}

func CreateBingoCard(seed string) (BingoCard, error) { return CreateCard(seed, DefaultOptions()) }

func CreateCard(seed string, options Options) (BingoCard, error) {
	catalog := GetBingoData()
	if catalog == nil {
		return BingoCard{}, fmt.Errorf("bingo data not initialized")
	}
	if seed == "" {
		seed = uuid.NewString()
	}
	return SelectCatalog(*catalog, seed, options)
}

// Cost weights the goal's full start-to-completion time. Stage access and
// preparation are already included in TimeMin; no world-based time is added here.
func Cost(g BingoGoal) float64 { return float64(g.TimeMin) * (1 + .3*float64(g.Risk-1)) }

func LineCosts(card BingoCard) [12]float64 {
	var costs [12]float64
	for i, line := range lines {
		var goals [5]BingoGoal
		for j, cell := range line {
			goals[j] = card.Goals[cell]
		}
		costs[i] = lineCost(goals, card.RouteAreaTimes)
	}
	return costs
}

func EstimatedMinutes(card BingoCard, options Options) float64 {
	costs := RaceLineCosts(card, options)
	minimum := costs[0]
	for _, cost := range costs[1:] {
		minimum = math.Min(minimum, cost)
	}
	return minimum
}

func HasLine(progress [25]bool) bool {
	for _, line := range lines {
		complete := true
		for _, cell := range line {
			complete = complete && progress[cell]
		}
		if complete {
			return true
		}
	}
	return false
}

// score balances all twelve corrected projected totals against their own mean.
// Unknown cells use the selected goals' mean cost.
func score(card BingoCard, cell int, candidate BingoGoal, mean float64) float64 {
	penalty, deviation, average := 0.0, 0.0, 0.0
	var totals [12]float64
	for lineIndex, line := range lines {
		contains := false
		for _, index := range line {
			contains = contains || index == cell
		}
		var goals [5]BingoGoal
		missing := 0
		for j, index := range line {
			if index == cell {
				goals[j] = candidate
				continue
			}
			g := card.Goals[index]
			if g.Name == "" {
				missing++
				continue
			}
			goals[j] = g
			if !contains {
				continue
			}
			for _, a := range g.Tags {
				for _, b := range candidate.Tags {
					if a == b {
						if strings.HasPrefix(a, "area:") {
							penalty += 2
						} else {
							penalty++
						}
					}
				}
			}
		}
		totals[lineIndex] = lineCost(goals, card.RouteAreaTimes) + float64(missing)*mean
		average += totals[lineIndex]
	}
	average /= 12
	for _, total := range totals {
		deviation += math.Abs(total-average) / 12
	}
	return penalty + deviation
}

// canPlaceWorld checks only the lines containing cell. An empty world denotes
// an unrestricted goal and does not conflict with other unrestricted goals.
func canPlaceWorld(card BingoCard, cell int, world string) bool {
	if world == "" {
		return true
	}
	for _, line := range lines {
		contains := false
		for _, index := range line {
			if index == cell {
				contains = true
			}
		}
		if !contains {
			continue
		}
		for _, index := range line {
			if index != cell && card.Goals[index].World == world {
				return false
			}
		}
	}
	return true
}

// SelectGoals is deterministic and has no global state. Bounded randomized
// greedy search may reject a narrow feasible range; it never returns an
// out-of-range card. The caller can try another seed or widen the range.
func SelectGoals(goals []BingoGoal, seed string, options Options) (BingoCard, error) {
	return SelectCatalog(BingoData{Goals: goals}, seed, options)
}

// SelectCatalog uses a single catalog snapshot for route costs and goal selection.
func SelectCatalog(catalog BingoData, seed string, options Options) (BingoCard, error) {
	if err := ValidateData(catalog); err != nil {
		return BingoCard{}, err
	}
	goals := make([]BingoGoal, 0, len(catalog.Goals))
	for _, goal := range catalog.Goals {
		if goal.DisabledReason == "" {
			goals = append(goals, goal)
		}
	}
	if len(goals) < 25 {
		return BingoCard{}, fmt.Errorf("抽選対象のお題が25件以上必要です（保留中のお題は除外されます）")
	}
	o, err := options.Normalize()
	if err != nil {
		return BingoCard{}, err
	}
	ordered := append([]BingoGoal(nil), goals...)
	sort.SliceStable(ordered, func(i, j int) bool { return Cost(ordered[i]) < Cost(ordered[j]) })
	base := 0.0
	if o.Rule == Standard {
		base = float64(o.BaseRoute)
	}
	// Finish routes may share all unlock work; use conservative bounds.
	finishLower := base
	if o.Rule == Standard && len(catalog.BowserRoutes) > 0 {
		base = math.Inf(1)
		finishLower = 0
		for _, route := range catalog.BowserRoutes {
			base = math.Min(base, float64(route.TimeMin))
		}
	}
	lower, upper := finishLower, base
	residuals := make([]float64, len(goals))
	for i, goal := range goals {
		residuals[i] = Cost(goal) - float64(routeMinutes(goal, catalog.RouteAreaTimes))
	}
	sort.Float64s(residuals)
	for i := 0; i < 5; i++ {
		lower += residuals[i]
		upper += Cost(ordered[len(ordered)-1-i])
	}
	// Shared travel may reduce a sum, but cannot make a line cheaper than any
	// of its individual goals. Both bounds remain safe when routes overlap.
	lower = math.Max(lower, finishLower+Cost(ordered[4]))
	if lower > float64(o.MaxTime) || upper < float64(o.MinTarget) {
		return BingoCard{}, fmt.Errorf("requested time range is impossible for this catalog")
	}
	rng := rand.New(rand.NewSource(strToInt64(seed)))
	for attempt := 0; attempt < 256; attempt++ {
		indices := generateIndex(rng, len(goals))
		pool := make([]BingoGoal, 25)
		mean := 0.0
		for i, index := range indices {
			pool[i] = goals[index]
			mean += Cost(pool[i]) / 25
		}
		card := BingoCard{Seed: seed, BowserRoutes: cloneFinishRoutes(catalog.BowserRoutes)}
		if len(catalog.RouteAreaTimes) > 0 {
			card.RouteAreaTimes = maps.Clone(catalog.RouteAreaTimes)
		}
		complete := true
		for _, cell := range placementOrder {
			best, bestScore, ties := -1, math.Inf(1), 0
			for i, g := range pool {
				if !canPlaceWorld(card, cell, g.World) || !canPlaceGroups(card, cell, g.ConflictGroups) {
					continue
				}
				s := score(card, cell, g, mean)
				if s < bestScore-1e-9 {
					best, bestScore, ties = i, s, 1
				} else if math.Abs(s-bestScore) < 1e-9 {
					ties++
					if rng.Intn(ties) == 0 {
						best = i
					}
				}
			}
			if best == -1 {
				complete = false
				break
			}
			card.Goals[cell] = cloneGoal(pool[best])
			pool = append(pool[:best], pool[best+1:]...)
		}
		if !complete {
			continue // Draw a new set of 25 goals when no valid placement remains.
		}
		estimate := EstimatedMinutes(card, o)
		if estimate >= float64(o.MinTarget)-1e-9 && estimate <= float64(o.MaxTime)+1e-9 {
			return card, nil
		}
	}
	return BingoCard{}, fmt.Errorf("ワールド重複のないカードを指定時間内で生成できませんでした。シードやお題・時間設定を見直してください")
}

func generateIndex(r *rand.Rand, max int) [25]int {
	if max < 25 {
		return [25]int{}
	}
	return [25]int(r.Perm(max)[:25])
}

func strToInt64(s string) int64 {
	if val, err := strconv.ParseInt(s, 10, 64); err == nil {
		return val
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return int64(h.Sum64())
}
