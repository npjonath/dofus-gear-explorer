package solver

import (
	"math"

	"dofusdb/api/internal/engine"
	"dofusdb/api/internal/game"
)

// This file spends a character's characteristic points for the solver.
// For each stuff it looks at, the solver first buys what the constraints
// still miss, then puts the rest where it serves the request best, and
// leaves anything unspent in Vitality. Costs follow the game: Vitality is
// 1 point for 1, Wisdom 3 for 1, and the four elements cost 1, 2, 3 then
// 4 points per point for each hundred already bought.

const primaries = 6

// primaryIDs lists the characteristics points are spent on, in the order of
// engine.PrimaryStats.
var primaryIDs = [primaries]int{
	game.StatVitality, game.StatWisdom, game.StatStrength, game.StatIntel, game.StatChance, game.StatAgility,
}

// lagStep is how many points an element held to a floor gets at a time, so
// that the elements stay level as the points are spent.
const lagStep = 10

// lag describes one element of a goal that holds its elements to a floor.
type lag struct {
	// k is the element's characteristic among the primaries.
	k int
	// level is what the element hits for before any point is spent, over
	// its share, and perPoint what one point adds to that.
	level, perPoint float64
}

// costKind says how a primary stat is priced.
type costKind int

const (
	costFlat1 costKind = iota
	costFlat3
	costTiered
)

// unitCost is the price of the next point once `owned` are bought.
func unitCost(kind costKind, owned int32) int32 {
	switch kind {
	case costFlat1:
		return 1
	case costFlat3:
		return 3
	}
	return min(owned/100, 3) + 1
}

// tierRoom is how many points keep the current price.
func tierRoom(kind costKind, owned int32) int32 {
	if kind != costTiered || owned >= 300 {
		return math.MaxInt32
	}
	return 100 - owned%100
}

// buy adds up to `want` points of one stat within the budget and returns
// what it got and what it paid.
func buy(kind costKind, owned, want, budget int32) (got, paid int32) {
	for want > 0 {
		price := unitCost(kind, owned)
		n := min(want, tierRoom(kind, owned), (budget-paid)/price)
		if n <= 0 {
			break
		}
		got += n
		paid += n * price
		owned += n
		want -= n
	}
	return got, paid
}

// need ties a constraint to the primary stat that can pay for it.
type need struct {
	con int
	k   int
	// perPoint is how much of the constrained stat one point of the primary gives.
	perPoint float64
}

type pointPlan struct {
	budget int32
	// idx is the dense index of each primary, in engine.PrimaryStats order.
	idx  [primaries]int32
	kind [primaries]costKind
	// value is what one point of each primary adds to the objective.
	value [primaries]float64
	needs []need
}

// newPointPlan prepares the spending rules for one search. userWeight gives
// the objective weight of each stat by dense index, before tie-breaks.
func newPointPlan(cat *game.Catalog, level int, userWeight []float64, cons []constraint) *pointPlan {
	plan := &pointPlan{budget: int32(engine.PointsTotal(level))}
	ids := primaryIDs
	const (
		vit = iota
		wis
		str
		intel
		cha
		agi
	)
	weight := func(statID int) float64 {
		if idx := cat.Idx(statID); idx >= 0 {
			return userWeight[idx]
		}
		return 0
	}
	for k, id := range ids {
		plan.idx[k] = cat.Idx(id)
		plan.kind[k] = costTiered
		plan.value[k] = weight(id)
	}
	plan.kind[vit], plan.kind[wis] = costFlat1, costFlat3

	// What each primary feeds, as the engine derives it.
	feeds := map[int][]struct {
		k        int
		perPoint float64
	}{
		game.StatHP:          {{vit, 1}},
		game.StatInitiative:  {{str, 1}, {intel, 1}, {cha, 1}, {agi, 1}},
		game.StatPods:        {{str, 5}},
		game.StatProspecting: {{cha, 0.1}},
		game.StatTackle:      {{agi, 0.1}},
		game.StatEscape:      {{agi, 0.1}},
		game.StatDodgeAP:     {{wis, 0.1}},
		game.StatDodgeMP:     {{wis, 0.1}},
		game.StatAPReduction: {{wis, 0.1}},
		game.StatMPReduction: {{wis, 0.1}},
	}
	for statID, sources := range feeds {
		for _, src := range sources {
			plan.value[src.k] += weight(statID) * src.perPoint
		}
	}

	for i, c := range cons {
		if !c.gte {
			continue
		}
		statID := cat.StatIDs[c.idx]
		matched := false
		for k, id := range ids {
			if id == statID {
				plan.needs = append(plan.needs, need{con: i, k: k, perPoint: 1})
				matched = true
			}
		}
		if matched {
			continue
		}
		// A derived stat is bought through the source that also serves the
		// request best.
		if sources := feeds[statID]; len(sources) > 0 {
			best := sources[0]
			for _, src := range sources[1:] {
				if plan.value[src.k] > plan.value[best.k] {
					best = src
				}
			}
			plan.needs = append(plan.needs, need{con: i, k: best.k, perPoint: best.perPoint})
		}
	}
	return plan
}

// spend returns the points bought in each primary for a stuff whose final
// stats, before any point is spent, are `totals`. Among the elements held to
// a floor, only the one furthest behind is worth its full value: the others
// count as mixSlope of theirs until it catches up.
func (plan *pointPlan) spend(cons []constraint, totals []int32, lagging []lag) [primaries]int32 {
	var bought [primaries]int32
	budget := plan.budget

	for _, nd := range plan.needs {
		c := cons[nd.con]
		have := float64(totals[c.idx]) + float64(bought[nd.k])*nd.perPoint
		missing := float64(c.value) - have
		if missing <= 0 {
			continue
		}
		want := int32(math.Ceil(missing / nd.perPoint))
		got, paid := buy(plan.kind[nd.k], bought[nd.k], want, budget)
		bought[nd.k] += got
		budget -= paid
	}

	for budget > 0 {
		behind := -1
		var held [primaries]bool
		for i, l := range lagging {
			held[l.k] = true
			level := func(l lag) float64 { return l.level + l.perPoint*float64(bought[l.k]) }
			if behind < 0 || level(l) < level(lagging[behind]) {
				behind = i
			}
		}
		best, bestRatio := -1, 0.0
		for k := range primaries {
			value := plan.value[k]
			if held[k] && k != lagging[behind].k {
				value *= mixSlope
			}
			if value <= 0 || plan.idx[k] < 0 {
				continue
			}
			price := unitCost(plan.kind[k], bought[k])
			if price > budget {
				continue
			}
			if ratio := value / float64(price); ratio > bestRatio {
				best, bestRatio = k, ratio
			}
		}
		if best < 0 {
			break
		}
		room := tierRoom(plan.kind[best], bought[best])
		if held[best] {
			room = min(room, lagStep)
		}
		got, paid := buy(plan.kind[best], bought[best], room, budget)
		if got == 0 {
			break
		}
		bought[best] += got
		budget -= paid
	}

	// Points nothing asked for go to Vitality, which always helps.
	bought[0] += budget
	return bought
}
