// Package solver searches for gear sets that satisfy stat constraints.
//
// The search space (16 positions, hundreds of items each, set bonuses that
// couple positions together) is far too large to enumerate, so the solver
// prunes each slot to its most relevant items and runs parallel simulated
// annealing over the remaining combinations, then polishes the best results
// with a greedy pass.
package solver

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"runtime"
	"slices"
	"sort"
	"sync"
	"time"

	"dofusdb/api/internal/engine"
	"dofusdb/api/internal/game"
)

type Constraint struct {
	StatID int `json:"statId"`
	// Op is "gte" or "lte".
	Op    string `json:"op"`
	Value int    `json:"value"`
}

// Weight is one thing to maximize: a stat, or what an attack hits for.
type Weight struct {
	StatID int     `json:"statId"`
	Weight float64 `json:"weight"`
	// Damage, when set, makes the entry about an attack instead of a stat:
	// its measure is pushed up, at the entry's rank in the list.
	Damage *DamageGoal `json:"damage,omitempty"`
}

type Request struct {
	Character   engine.Character `json:"character"`
	Constraints []Constraint     `json:"constraints"`
	// Maximize lists what to push as high as possible once the constraints
	// hold, stats and attacks alike, by priority: each entry counts
	// priorityStep times less than the one before it. Empty means "the
	// constrained stats", which then all count the same.
	Maximize []Weight `json:"maximize"`
	// Damage lists attacks that must hit for at least a value.
	Damage []DamageGoal `json:"damage"`
	// BreedID is the character's class. A goal about an element reads it on
	// the class's spells; zero falls back on a reference hit.
	BreedID int `json:"breedId"`
	// Locked forces an item at a position.
	Locked map[string]int `json:"locked"`
	// Required lists items every stuff must wear, wherever they fit. They
	// are placed before the locked ones, which give way to them.
	Required []int `json:"required"`
	Excluded []int `json:"excluded"`
	// The character's level is the only level rule: items above it are left
	// out, and any item below it competes on what it brings.
	MaxResults int    `json:"maxResults"`
	Seed       uint64 `json:"seed"`
	// Forge says which forgemagic the solver may add to the items it picks.
	Forge ForgeOptions `json:"forge"`
	// AutoPoints lets the solver spend the character's characteristic points
	// itself, ignoring Character.Base, to serve the request.
	AutoPoints bool `json:"autoPoints"`
}

// What a damage goal measures.
const (
	// MeasureNormal is the average roll of a hit that is not critical.
	MeasureNormal = "normal"
	// MeasureCrit is the average roll of a critical hit.
	MeasureCrit = "crit"
	// MeasureMean is the average hit over many casts, at the stuff's
	// critical rate: the final damage.
	MeasureMean = "mean"
)

// DamageGoal is about what a weapon or a spell hits for, counting every line
// that hits at once. In Request.Damage it must reach Value; in an entry of
// Request.Maximize it is pushed as high as possible. Either way, a weapon
// goal makes the stuff wear that weapon.
type DamageGoal struct {
	// Kind is "weapon" or "spell", with ID the item or the spell, or
	// "element": the damage of the class's spells in one element, without
	// naming any.
	Kind string `json:"kind"`
	ID   int    `json:"id"`
	// Measure is one of the Measure constants; empty means MeasureNormal.
	Measure string `json:"measure"`
	// Range is "melee", "ranged" or empty to follow the attack's reach.
	Range string `json:"range"`
	// Element is, for a weapon goal, the elemental forgemagic the weapon
	// carries (its Neutral damage hits in that element; empty leaves it).
	// For an element goal it is the element measured; empty lets the search
	// try the four and keep the best.
	Element string `json:"element"`
	// Value is the least the measure must reach. It is required of a goal in
	// Request.Damage and ignored in Request.Maximize.
	Value int `json:"value"`
}

// Measured is what the goal reads on a stuff: the measure of its attack, or
// the average of the measure over its attacks when it covers several.
func (g DamageGoal) Measured(specs []*engine.AttackSpec, stat func(statID int) int) int {
	total := 0
	for _, spec := range specs {
		switch g.Measure {
		case MeasureMean:
			total += spec.Mean(stat)
		case MeasureCrit:
			hit := spec.Total(stat)
			total += (hit.CritMin + hit.CritMax) / 2
		default:
			hit := spec.Total(stat)
			total += (hit.Min + hit.Max) / 2
		}
	}
	if len(specs) == 0 {
		return 0
	}
	return total / len(specs)
}

// referenceHit stands for "a spell" when the character has no class, or a
// class with no spell in the element asked for: a ranged spell of ordinary
// base damage and critical chance.
var referenceHit = game.AttackLine{Min: 28, Max: 32, CritMin: 34, CritMax: 38}

const referenceCritChance = 10

// Specs resolves the attacks a goal reads, for a character level and class.
// It is empty when the item or spell deals no direct damage at that level.
func (g DamageGoal) Specs(cat *game.Catalog, level, breedID int) []*engine.AttackSpec {
	switch g.Kind {
	case "weapon":
		// A weapon is counted with its mastery, as players wield it.
		opts := engine.WeaponOptions{Range: g.Range, Element: g.Element, Mastery: true}
		if spec := engine.WeaponSpec(cat.ItemByID[g.ID], opts); spec != nil {
			return []*engine.AttackSpec{spec}
		}
	case "spell":
		if spec, _ := engine.SpellSpec(cat.SpellByID[g.ID], level, g.Range); spec != nil {
			return []*engine.AttackSpec{spec}
		}
	case "element":
		element, ok := engine.ElementID(g.Element)
		if !ok {
			return nil
		}
		// Every spell of the class with a line in the element, or one that
		// follows the best element; each counts with its own critical chance.
		var specs []*engine.AttackSpec
		for _, sp := range cat.SpellsByBreed[breedID] {
			spec, _ := engine.SpellSpec(sp, level, g.Range)
			if spec == nil || breedID == 0 {
				continue
			}
			var lines []game.AttackLine
			for _, line := range spec.Lines {
				if line.Delay == 0 && (line.Element == element || line.Element == game.ElementBest) {
					line.Element = element
					lines = append(lines, line)
				}
			}
			if len(lines) > 0 {
				copied := *spec
				copied.Lines = lines
				specs = append(specs, &copied)
			}
		}
		if len(specs) == 0 {
			line := referenceHit
			line.Element = element
			specs = append(specs, &engine.AttackSpec{
				Lines: []game.AttackLine{line}, CritChance: referenceCritChance, Melee: g.Range == engine.RangeMelee,
			})
		}
		return specs
	}
	return nil
}

// ForgeOptions opens forgemagic to the search. What each item takes is
// chosen with the rest of the stuff, on the stuff's score: a rune that fills
// a missed condition beats one that only adds to what is maximized.
type ForgeOptions struct {
	// Exos lists the limited exos allowed, by stat id: AP, MP, Range. Each
	// is worn at most once in a stuff.
	Exos []int `json:"exos"`
	// Transcendence lets every forgeable item take a rune.
	Transcendence bool `json:"transcendence"`
	// Draco lets every forgeable item take its best dracorune (update 3.8).
	Draco bool `json:"draco"`
	// RaiseCritical lets an item whose Critical line is negative have that
	// line forged up to what forgemagic allows, instead of a rune.
	RaiseCritical bool `json:"raiseCritical"`
}

type Solution struct {
	// Character is the character the stuff was computed for, with the
	// characteristic points the solver spent when it was asked to.
	Character engine.Character `json:"character"`
	Items     map[string]int   `json:"items"`
	// Mods is the forgemagic the solver put on the items, by position.
	Mods     map[string]*engine.ItemMods `json:"mods,omitempty"`
	Score    float64                     `json:"score"`
	Feasible bool                        `json:"feasible"`
	// Unmet lists the indexes of the constraints the solution misses, and
	// UnmetDamage those of the damage goals.
	Unmet       []int `json:"unmet"`
	UnmetDamage []int `json:"unmetDamage"`
	// Damage is what each attack measures on this stuff: the goals of
	// Request.Damage first, then the attacks of Request.Maximize in order.
	Damage []int `json:"damage"`
	// Element is the element this stuff was searched for, when a goal left
	// the element to the search.
	Element string `json:"element,omitempty"`
}

type Response struct {
	Solutions  []Solution `json:"solutions"`
	Explored   int64      `json:"explored"`
	Candidates int        `json:"candidates"`
	ElapsedMs  int64      `json:"elapsedMs"`
	// Unplaced lists the required items left out: every place they fit was
	// taken by another required item.
	Unplaced []int `json:"unplaced"`
}

var ErrNothingToSearch = errors.New("add at least one constraint or one stat to maximize")

// ErrBadDamageGoal is returned for a damage goal the character cannot make:
// an item that is not a weapon, a spell not learnt yet, a critical hit on an
// attack that has none.
var ErrBadDamageGoal = errors.New("a damage goal names an attack this character cannot make")

// runeWeights put stats on a common scale: the forgemagic weight of one
// point of each characteristic. One AP is worth about a hundred Strength.
var runeWeights = map[int]float64{
	game.StatHP: 0.2, game.StatVitality: 0.2,
	game.StatStrength: 1, game.StatIntel: 1, game.StatChance: 1, game.StatAgility: 1,
	game.StatWisdom: 3, game.StatAP: 100, game.StatMP: 90, game.StatRange: 51,
	game.StatSummons: 30, game.StatCritical: 10, game.StatPower: 2,
	game.StatHeal: 10, game.StatProspecting: 3, game.StatInitiative: 0.1, game.StatPods: 0.25,
	game.StatTackle: 4, game.StatEscape: 4,
	game.StatDodgeAP: 7, game.StatDodgeMP: 7, game.StatAPReduction: 7, game.StatMPReduction: 7,
	16: 20,                            // damage
	88: 5, 89: 5, 90: 5, 91: 5, 92: 5, // elemental damage
	86: 5, 84: 5, 70: 5, 69: 2, 50: 10, // critical, pushback, trap damage, trap power, reflect
	33: 6, 34: 6, 35: 6, 36: 6, 37: 6, // % resistances
	54: 2, 55: 2, 56: 2, 57: 2, 58: 2, // fixed resistances
	87: 2, 85: 2, // critical and pushback resistance
	120: 15, 122: 15, 123: 15, 125: 15, // % damage dealt
	121: 15, 124: 15, 141: 15, 142: 15, // % damage received
}

func runeWeight(statID int) float64 {
	if w, ok := runeWeights[statID]; ok {
		return w
	}
	return 1
}

const (
	// damageUnit is what one point of damage weighs on the scale of rune
	// weights: about one elemental damage rune.
	damageUnit = 4.0

	// exoHostPenalty is what a missing home for an allowed exo costs: an
	// exo needs a forgeable item with no transcendence rune.
	exoHostPenalty = 150.0
	// forgeCost is the token price of one piece of forgemagic: far below
	// any requested point, above the ties between stats nobody asked for.
	forgeCost = 0.05
	// variantMoveShare is the share of annealing moves that change the
	// forgemagic of a worn item instead of the item.
	variantMoveShare = 0.2

	constraintPenalty = 5.0
	constraintFlat    = 0.0
	criteriaPenalty   = 300.0

	perSlotCandidates = 40
	topSets           = 24
	annealRounds      = 8
	// quickRounds is the number of runs of a rough search.
	quickRounds      = 1
	annealIterations = 220000
	// The leaders are crossed with the pieces worn, at each position, by the
	// best eliteEntries stuffs: at most elitePerPosition pieces a position.
	eliteEntries     = 160
	elitePerPosition = 14
	eliteLeaders     = 10
	// The very first also get exchanges of four pieces, on fewer pieces.
	quadLeaders = 3
	quadWidth   = 6
	// deepVariantLeaders is how many of the best stuffs get their
	// forgemagic chosen again as a whole.
	deepVariantLeaders = 3
	// pairClimbLeaders is how many of the best stuffs get the pair climb.
	pairClimbLeaders = 6
	poolSize         = 160
	// tieBreakShare is the share of its weight a stat outside the request
	// counts for.
	tieBreakShare = 0.001
	// priorityStep is how much less a stat to maximize counts than the one
	// ranked just before it. A lower priority decides between stuffs that
	// are close on the higher ones, and a large gain can still outweigh a
	// small loss above.
	priorityStep = 10.0
	// emptyPenalty is what an empty slot costs. It is below the cost of a
	// missed constraint or a broken equip condition: filling a slot never
	// justifies an invalid stuff.
	emptyPenalty = 60.0
)

type cand struct {
	item *game.Item
	// mods is the forgemagic chosen for this item; vec already includes it.
	mods *engine.ItemMods
	// runeUtility is what the transcendence rune in mods brings, so the item
	// that loses it to an exo is the one that loses least.
	runeUtility float64
	// variants lists every forgemagic the search may give this item, this
	// candidate included; nil when there is only one.
	variants []*cand
	// exoHost says the item can take a limited exo as it is: forgeable and
	// without a transcendence rune.
	exoHost bool
	// cost is a token price per rune, dracorune or forged line, so that
	// forgemagic that changes nothing for the request is left out.
	cost    float64
	vec     []game.StatValue
	setOrd  int
	crit    *engine.Criteria
	utility float64
}

type constraint struct {
	idx    int32
	gte    bool
	value  int32
	weight float64
}

type damageGoal struct {
	specs []*engine.AttackSpec
	goal  DamageGoal
	value int
	// weight is what one point of the measure adds to the score when the
	// goal is maximized.
	weight float64
}

type problem struct {
	eng   *engine.Engine
	cat   *game.Catalog
	level int

	base       []int32
	baseTotals []int32

	cands  [][]*cand
	locked []bool
	free   []int

	objIdx []int32
	objW   []float64
	cons   []constraint
	damage []damageGoal
	// statIdx gives the index of a stat id in the stat vectors, or -1.
	statIdx [256]int32
	exos    []int
	// points is set when the solver spends the characteristic points itself.
	points *pointPlan

	setItems map[int][]*cand
	setList  []int

	unplaced []int
	// rounds is the number of annealing runs each worker does.
	rounds int
}

var npos = len(game.Positions)

// Solve runs the search. It is deterministic for a given request and catalog.
// autoElements are the elements tried when a goal leaves the choice open.
var autoElements = []string{"earth", "fire", "water", "air"}

// Solve searches for the best stuffs for a request. A goal about "damage in
// the best element" gets a rough search per element, then the full search in
// the element that came out on top; the stuffs returned are marked with it.
func Solve(ctx context.Context, eng *engine.Engine, req Request) (*Response, error) {
	open := func(g DamageGoal) bool { return g.Kind == "element" && g.Element == "" }
	auto := false
	for _, g := range req.Damage {
		auto = auto || open(g)
	}
	for _, m := range req.Maximize {
		auto = auto || (m.Damage != nil && open(*m.Damage))
	}
	if !auto {
		return solve(ctx, eng, req, false)
	}
	started := time.Now()
	withElement := func(element string) Request {
		one := req
		one.Damage = append([]DamageGoal(nil), req.Damage...)
		for i := range one.Damage {
			if open(one.Damage[i]) {
				one.Damage[i].Element = element
			}
		}
		one.Maximize = append([]Weight(nil), req.Maximize...)
		for i, m := range one.Maximize {
			if m.Damage != nil && open(*m.Damage) {
				goal := *m.Damage
				goal.Element = element
				one.Maximize[i].Damage = &goal
			}
		}
		return one
	}
	// A rough search per element says which one hits hardest; only that one
	// gets the full search.
	best, bestRank := "", math.Inf(-1)
	var explored int64
	for _, element := range autoElements {
		resp, err := solve(ctx, eng, withElement(element), true)
		if err != nil {
			return nil, err
		}
		explored += resp.Explored
		if len(resp.Solutions) == 0 {
			continue
		}
		if rank := rankScore(resp.Solutions[0].Score, resp.Solutions[0].Feasible); rank > bestRank {
			best, bestRank = element, rank
		}
	}
	if best == "" {
		best = autoElements[0]
	}
	resp, err := solve(ctx, eng, withElement(best), false)
	if err != nil {
		return nil, err
	}
	for i := range resp.Solutions {
		resp.Solutions[i].Element = best
	}
	resp.Explored += explored
	resp.ElapsedMs = time.Since(started).Milliseconds()
	return resp, nil
}

// solve runs one search. A quick search is a rough look: fewer annealing
// runs and no polishing of the leaders, enough to compare elements.
func solve(ctx context.Context, eng *engine.Engine, req Request, quick bool) (*Response, error) {
	started := time.Now()
	req.Character.Normalize()
	if req.AutoPoints {
		for key := range req.Character.Base {
			req.Character.Base[key] = 0
		}
	}
	p, err := newProblem(eng, req)
	if err != nil {
		return nil, err
	}
	p.rounds = annealRounds
	if quick {
		p.rounds = quickRounds
	}

	workers := min(runtime.NumCPU(), 8)
	pools := make([]*pool, workers)
	// A start per shortlisted set and per pair of them.
	var starts [][]int
	for i, a := range p.setList {
		starts = append(starts, []int{a})
		for _, b := range p.setList[i+1:] {
			starts = append(starts, []int{a, b})
		}
	}
	var explored int64
	var mu sync.Mutex
	var wg sync.WaitGroup
	for w := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rng := rand.New(rand.NewPCG(req.Seed+1, uint64(w)+1))
			pl := newPool()
			n := p.anneal(ctx, rng, pl)
			// Then this worker's share of the set starts: the same ones
			// whatever the seed, so the stuffs built around a set are found
			// every time.
			for i := w; i < len(starts); i += workers {
				n += p.seedSets(ctx, pl, starts[i])
			}
			mu.Lock()
			pools[w] = pl
			explored += n
			mu.Unlock()
		}()
	}
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	merged := newPool()
	for _, pl := range pools {
		for _, e := range pl.entries {
			merged.offer(e.assign, e.score, e.feasible)
		}
	}
	merged.sort()

	// Cross the leaders: the runs end in different stuffs that share good
	// pieces, and swapping three or four of those pieces at once reaches
	// stuffs no single run found.
	if !quick && len(merged.entries) > 0 && len(p.free) >= 3 {
		elite := make([][]*cand, npos)
		for i, e := range merged.entries {
			if i >= eliteEntries {
				break
			}
			for pos, c := range e.assign {
				if c == nil || p.locked[pos] || len(elite[pos]) >= elitePerPosition {
					continue
				}
				known := false
				for _, other := range elite[pos] {
					known = known || other.item.ID == c.item.ID
				}
				if !known {
					elite[pos] = append(elite[pos], c)
				}
			}
		}
		st := p.newState()
		for i := 0; i < min(eliteLeaders, len(merged.entries)); i++ {
			st.load(merged.entries[i].assign)
			explored += st.exchangeClimb(ctx, elite, 3, elitePerPosition)
			if i < quadLeaders {
				explored += st.exchangeClimb(ctx, elite, 4, quadWidth)
			}
			score, feasible := st.eval()
			merged.offer(st.cur, score, feasible)
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		merged.sort()
	}

	// Polish the leaders. Climbing pulls neighbours to the same optimum, so the
	// unpolished pool is kept alongside to offer real alternatives.
	polished := newPool()
	st := p.newState()
	for i, e := range merged.entries {
		polished.offer(e.assign, e.score, e.feasible)
		if i >= 24 || quick {
			continue
		}
		st.load(e.assign)
		explored += st.climb()
		// The very best also get every swap of two items tried at once, the
		// trades a one-item climb cannot see, and their forgemagic chosen
		// again as a whole.
		if i < pairClimbLeaders {
			explored += st.pairClimb(ctx)
		}
		if i < deepVariantLeaders {
			explored += st.deepVariants()
		}
		score, feasible := st.eval()
		polished.offer(st.cur, score, feasible)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	polished.sort()

	maxResults := req.MaxResults
	if maxResults <= 0 {
		maxResults = 8
	}
	maxResults = min(maxResults, 20)

	resp := &Response{Solutions: []Solution{}, Explored: explored, Unplaced: p.unplaced}
	for _, list := range p.cands {
		resp.Candidates += len(list)
	}
	var chosen [][]*cand
	for _, e := range polished.entries {
		if len(resp.Solutions) >= maxResults {
			break
		}
		// Skip near-duplicates: a result must differ by two items or more.
		distinct := true
		for _, other := range chosen {
			if distance(e.assign, other) < 2 {
				distinct = false
				break
			}
		}
		if !distinct {
			continue
		}
		chosen = append(chosen, e.assign)
		resp.Solutions = append(resp.Solutions, p.finish(e.assign, e.score, req))
	}
	// Giving the exos to items can cost a rune, so rank on the final stuffs.
	sort.SliceStable(resp.Solutions, func(a, b int) bool {
		sa, sb := resp.Solutions[a], resp.Solutions[b]
		if sa.Feasible != sb.Feasible {
			return sa.Feasible
		}
		return sa.Score > sb.Score
	})
	resp.ElapsedMs = time.Since(started).Milliseconds()
	return resp, nil
}

func newProblem(eng *engine.Engine, req Request) (*problem, error) {
	cat := eng.Catalog
	req.Character.Normalize()
	p := &problem{
		eng: eng, cat: cat, level: req.Character.Level,
		cands:    make([][]*cand, npos),
		locked:   make([]bool, npos),
		setItems: map[int][]*cand{},
	}
	p.base = eng.BaseVector(req.Character)
	p.baseTotals = make([]int32, eng.StatCount())
	eng.Derive(p.level, p.base, p.baseTotals)
	// An allowed exo is one more point whichever item ends up holding it, so
	// it is counted up front and given to an item once the stuff is chosen.
	for _, statID := range engine.LimitedExos {
		for _, allowed := range req.Forge.Exos {
			if allowed == statID && cat.Idx(statID) >= 0 {
				p.exos = append(p.exos, statID)
				p.base[cat.Idx(statID)]++
				break
			}
		}
	}

	relevance := make([]float64, eng.StatCount())
	for i, c := range req.Constraints {
		idx := cat.Idx(c.StatID)
		if idx < 0 || (c.Op != "gte" && c.Op != "lte") {
			return nil, fmt.Errorf("constraint %d is invalid", i+1)
		}
		w := runeWeight(c.StatID)
		p.cons = append(p.cons, constraint{idx: idx, gte: c.Op == "gte", value: int32(c.Value), weight: w})
		if c.Op == "gte" {
			relevance[idx] += w * 3
		}
	}

	for i := range p.statIdx {
		p.statIdx[i] = -1
	}
	for idx, statID := range cat.StatIDs {
		if statID >= 0 && statID < len(p.statIdx) {
			p.statIdx[statID] = int32(idx)
		}
	}
	// A "best element" line follows the first elemental characteristic the
	// request names, Strength otherwise: that is where the points will go.
	bestElement := game.ElementEarth
	named := []int{}
	for _, m := range req.Maximize {
		named = append(named, m.StatID)
	}
	for _, c := range req.Constraints {
		named = append(named, c.StatID)
	}
	for _, m := range req.Maximize {
		if m.Damage != nil && m.Damage.Kind == "element" {
			if el, ok := engine.ElementID(m.Damage.Element); ok {
				named = append([]int{game.ElementStats[el].Stat}, named...)
				break
			}
		}
	}
pick:
	for _, statID := range named {
		for _, el := range []int{game.ElementEarth, game.ElementFire, game.ElementWater, game.ElementAir} {
			if s := game.ElementStats[el]; statID == s.Stat || statID == s.Damage {
				bestElement = el
				break pick
			}
		}
	}
	// damageWeight is what a point of each characteristic adds to the hits
	// that must reach a value, roughly; damageObjective the same for the
	// maximized ones, at their share.
	damageWeight := make([]float64, eng.StatCount())
	damageObjective := make([]float64, eng.StatCount())
	// addGoal registers an attack: required to reach its value when share is
	// zero, maximized at that share otherwise.
	addGoal := func(g DamageGoal, share float64) error {
		specs := g.Specs(cat, p.level, req.BreedID)
		badMeasure := g.Measure != "" && g.Measure != MeasureNormal && g.Measure != MeasureCrit && g.Measure != MeasureMean
		badRange := g.Range != engine.RangeAuto && g.Range != engine.RangeMelee && g.Range != engine.RangeRanged
		badElement := g.Kind != "element" && g.Element != "" &&
			(g.Kind != "weapon" || !engine.CanTakeElement(cat.ItemByID[g.ID], g.Element))
		if len(specs) == 0 || badMeasure || badRange || badElement || (share == 0 && g.Value <= 0) {
			return ErrBadDamageGoal
		}
		goal := damageGoal{specs: specs, goal: g, weight: damageUnit * share}
		if share == 0 {
			goal.value = g.Value
		}

		// A point of the element's characteristic adds a hundredth of the
		// base, a point of damage adds one, a percent adds a hundredth of a
		// typical hit. A critical line counts for its share of the casts.
		normalShare, critShare := 1.0, 0.0
		switch g.Measure {
		case MeasureCrit:
			normalShare, critShare = 0, 1
		case MeasureMean:
			normalShare, critShare = 0.5, 0.5
		}
		weights := make([]float64, eng.StatCount())
		perSpec := 1 / float64(len(specs))
		add := func(statID int, perPoint float64) {
			if idx := cat.Idx(statID); idx >= 0 {
				weights[idx] += perPoint * damageUnit * perSpec
			}
		}
		hits := false
		for _, spec := range specs {
			for _, line := range spec.Lines {
				if line.Delay > 0 {
					continue
				}
				canCrit := line.CritMax > 0 && spec.CritChance > 0
				if g.Measure == MeasureCrit && !canCrit {
					continue
				}
				hits = true
				normal := float64(line.Min+line.Max) / 2
				critical := float64(line.CritMin+line.CritMax) / 2
				lineNormal, lineCrit := normalShare, critShare
				if !canCrit {
					lineNormal, lineCrit = normalShare+critShare, 0
				}
				base := normal*lineNormal + critical*lineCrit
				element := line.Element
				if element == game.ElementBest {
					element = bestElement
				}
				el := game.ElementStats[element]
				add(el.Stat, base/100)
				add(game.StatPower, base/100)
				add(game.StatDamage, 1)
				add(el.Damage, 1)
				add(game.StatCritDamage, lineCrit)
				typical := (base*11 + 100) / 100
				if spec.Weapon {
					add(game.StatWeaponDamage, typical)
				} else {
					add(game.StatSpellDamage, typical)
				}
				if spec.Melee {
					add(game.StatMeleeDamage, typical)
				} else {
					add(game.StatRangedDamage, typical)
				}
				// A percent of Critical turns a hundredth of the casts critical.
				if g.Measure == MeasureMean && canCrit {
					add(game.StatCritical, ((critical-normal)*11+100)/100)
				}
			}
		}
		if !hits {
			return ErrBadDamageGoal
		}
		p.damage = append(p.damage, goal)
		for idx, w := range weights {
			if share == 0 {
				damageWeight[idx] += w
			} else {
				damageObjective[idx] += w * share
			}
		}
		return nil
	}
	for _, g := range req.Damage {
		if err := addGoal(g, 0); err != nil {
			return nil, err
		}
	}

	maximize := req.Maximize
	ranked := len(maximize) > 0
	if !ranked {
		for _, c := range req.Constraints {
			if c.Op == "gte" {
				maximize = append(maximize, Weight{StatID: c.StatID, Weight: 1})
			}
		}
	}
	maxWeight := 0.0
	for _, m := range maximize {
		if m.Damage == nil {
			maxWeight = math.Max(maxWeight, math.Abs(m.Weight))
		}
	}
	// lowest is the share of the last priority; ties are broken below it.
	share, lowest := 1.0, 1.0
	for _, m := range maximize {
		if m.Damage != nil {
			if err := addGoal(*m.Damage, share); err != nil {
				return nil, err
			}
		} else {
			idx := cat.Idx(m.StatID)
			if idx < 0 || m.Weight == 0 || maxWeight == 0 {
				continue
			}
			w := m.Weight / maxWeight * runeWeight(m.StatID) * share
			p.objIdx = append(p.objIdx, idx)
			p.objW = append(p.objW, w)
			relevance[idx] += w
		}
		lowest = share
		if ranked {
			share /= priorityStep
		}
	}
	for idx := range damageWeight {
		relevance[idx] += damageWeight[idx]*3 + damageObjective[idx]
	}
	if len(p.cons) == 0 && len(p.objIdx) == 0 && len(p.damage) == 0 {
		return nil, ErrNothingToSearch
	}
	if req.AutoPoints {
		// The points follow what is maximized, and the characteristics the
		// damage goals scale with.
		userWeight := make([]float64, eng.StatCount())
		for idx := range userWeight {
			userWeight[idx] = damageWeight[idx] + damageObjective[idx]
		}
		for i, idx := range p.objIdx {
			userWeight[idx] += p.objW[i]
		}
		p.points = newPointPlan(cat, p.level, userWeight, p.cons)
	}

	// Derived stats make their source characteristics relevant too.
	spread := func(target int, sources []int, factor float64) {
		ti := cat.Idx(target)
		if ti < 0 || relevance[ti] == 0 {
			return
		}
		for _, src := range sources {
			if si := cat.Idx(src); si >= 0 {
				relevance[si] += relevance[ti] * factor
			}
		}
	}
	elements := []int{game.StatStrength, game.StatIntel, game.StatChance, game.StatAgility}
	spread(game.StatHP, []int{game.StatVitality}, 1)
	spread(game.StatInitiative, elements, 1)
	spread(game.StatProspecting, []int{game.StatChance}, 0.1)
	spread(game.StatTackle, []int{game.StatAgility}, 0.1)
	spread(game.StatEscape, []int{game.StatAgility}, 0.1)
	spread(game.StatPods, []int{game.StatStrength}, 5)
	for _, target := range []int{game.StatDodgeAP, game.StatDodgeMP, game.StatAPReduction, game.StatMPReduction} {
		spread(target, []int{game.StatWisdom}, 0.1)
	}

	utilityOf := func(vec []game.StatValue) float64 {
		u := 0.0
		for _, sv := range vec {
			u += relevance[sv.Index] * float64(sv.Value)
		}
		return u
	}

	excluded := map[int]bool{}
	for _, id := range req.Excluded {
		excluded[id] = true
	}
	lockedByPos := map[int]*game.Item{}
	required := map[int]bool{}
	unplaced := []int{}
	weapons := []int{}
	for _, g := range req.Damage {
		if g.Kind == "weapon" {
			weapons = append(weapons, g.ID)
		}
	}
	for _, m := range req.Maximize {
		if m.Damage != nil && m.Damage.Kind == "weapon" {
			weapons = append(weapons, m.Damage.ID)
		}
	}
	for _, id := range append(weapons, req.Required...) {
		it := cat.ItemByID[id]
		if it == nil || required[id] {
			continue
		}
		required[id] = true
		placed := false
		for i, pos := range game.Positions {
			if _, taken := lockedByPos[i]; !taken && game.Fits(it, pos) {
				lockedByPos[i] = it
				placed = true
				break
			}
		}
		if !placed {
			unplaced = append(unplaced, id)
		}
	}
	for key, id := range req.Locked {
		pos := game.PositionIndex(key)
		it := cat.ItemByID[id]
		// A forbidden item is not kept, even when the stuff wears it.
		if pos < 0 || it == nil || !game.Fits(it, game.Positions[pos]) || required[id] || excluded[id] {
			continue
		}
		if _, taken := lockedByPos[pos]; !taken {
			lockedByPos[pos] = it
		}
	}
	p.unplaced = unplaced

	// runes are the transcendence runes worth something for this search,
	// best first. Which of them an item can take depends on its lines.
	type runeChoice struct {
		id      int
		utility float64
	}
	var runes []runeChoice
	if req.Forge.Transcendence {
		for i := range cat.Runes {
			rn := &cat.Runes[i]
			idx := cat.Idx(rn.StatID)
			if idx < 0 {
				continue
			}
			if u := relevance[idx] * float64(rn.Value); u > 0 {
				runes = append(runes, runeChoice{rn.ID, u})
			}
		}
		sort.SliceStable(runes, func(a, b int) bool { return runes[a].utility > runes[b].utility })
	}

	// runesByStat lists, per stat worth something here, its runes from the
	// strongest down: an item takes the strongest one its line has room for.
	runesByStat := map[int][]runeChoice{}
	for _, choice := range runes {
		statID := cat.RuneByID[choice.id].StatID
		runesByStat[statID] = append(runesByStat[statID], choice)
	}
	runeStats := make([]int, 0, len(runesByStat))
	for statID := range runesByStat {
		runeStats = append(runeStats, statID)
	}
	sort.Ints(runeStats)

	// makeCand returns an item with the forgemagic worth most on the linear
	// scale used to shortlist items. Every other forgemagic the request
	// allows hangs off it as a variant: the search switches between them on
	// the stuff's real score.
	makeCand := func(it *game.Item) *cand {
		setOrd := -1
		if it.SetID > 0 {
			setOrd = cat.SetOrdinal[it.SetID]
		}
		forgeable := engine.ForgeableSlots[it.Slot]
		plain := &cand{
			item: it, vec: cat.ItemVector[it.ID], setOrd: setOrd, crit: eng.Criteria(it.ID), exoHost: forgeable,
		}
		plain.utility = utilityOf(plain.vec)
		if !forgeable {
			return plain
		}
		variants := []*cand{plain}
		// An item whose other forgemagic leaves no weight for an allowed exo
		// is no home for one: overs and exos share 101 of weight.
		hostsExos := func(mods engine.ItemMods) bool {
			if mods.Transcendence != 0 {
				return false
			}
			for _, statID := range req.Forge.Exos {
				if hasStat(it, statID) {
					continue
				}
				with := mods
				with.Exos = append(slices.Clone(mods.Exos), engine.Exo{StatID: statID, Value: 1})
				if len(eng.Forge(it, &with).Issues) != 0 {
					return false
				}
			}
			return true
		}
		add := func(mods engine.ItemMods, runeUtility float64) {
			forged := eng.Forge(it, &mods)
			if len(forged.Issues) != 0 {
				return
			}
			cost := 0.0
			if mods.Transcendence != 0 {
				cost += forgeCost
			}
			if mods.Draco != nil {
				cost += forgeCost
			}
			if len(mods.Lines) > 0 {
				cost += forgeCost
			}
			variants = append(variants, &cand{
				item: it, mods: &mods, vec: forged.Vector, setOrd: setOrd, crit: plain.crit,
				utility: utilityOf(forged.Vector) - cost, runeUtility: runeUtility,
				exoHost: hostsExos(mods), cost: cost,
			})
		}
		dracos := []*engine.Draco{nil}
		if req.Forge.Draco {
			for _, option := range eng.DracoOptions(it) {
				draco := option
				dracos = append(dracos, &draco)
			}
		}
		for _, draco := range dracos {
			forged := eng.Forge(it, &engine.ItemMods{Draco: draco})
			if len(forged.Issues) != 0 {
				continue
			}
			if draco != nil {
				add(engine.ItemMods{Draco: draco}, 0)
			}
			for _, statID := range runeStats {
				for _, choice := range runesByStat[statID] {
					if slices.Contains(forged.Runes, choice.id) {
						add(engine.ItemMods{Draco: draco, Transcendence: choice.id}, choice.utility)
						break
					}
				}
			}
			// A negative Critical line forged up to its cap: an over, so
			// the item takes no rune.
			if req.Forge.RaiseCritical {
				for _, line := range forged.Lines {
					if line.StatID == game.StatCritical && line.Natural && line.Max < 0 && line.Cap > line.Max {
						add(engine.ItemMods{Draco: draco, Lines: map[int]int{game.StatCritical: line.Cap}}, 0)
					}
				}
			}
		}
		if len(variants) == 1 {
			return plain
		}
		best := plain
		for _, v := range variants {
			v.variants = variants
			if v.utility > best.utility+1e-9 {
				best = v
			}
		}
		return best
	}

	// Rank sets by what wearing them whole brings, per piece.
	type setRank struct {
		id    int
		value float64
	}
	var ranks []setRank
	setShare := map[int]float64{}
	for i := range cat.Sets {
		set := &cat.Sets[i]
		total, pieces := 0.0, 0
		for _, id := range set.ItemIDs {
			it := cat.ItemByID[id]
			if it == nil || it.Level > p.level || excluded[id] {
				continue
			}
			total += utilityOf(cat.ItemVector[id])
			pieces++
		}
		if pieces < 2 {
			continue
		}
		bonus := utilityOf(cat.SetBonusVector(set.ID, pieces))
		setShare[set.ID] = bonus / float64(pieces)
		ranks = append(ranks, setRank{set.ID, (total + bonus) / float64(pieces)})
	}
	sort.Slice(ranks, func(a, b int) bool {
		if ranks[a].value != ranks[b].value {
			return ranks[a].value > ranks[b].value
		}
		return ranks[a].id < ranks[b].id
	})
	keepSet := map[int]bool{}
	for i, r := range ranks {
		if i >= topSets || r.value <= 0 {
			break
		}
		keepSet[r.id] = true
		p.setList = append(p.setList, r.id)
	}

	bySlot := map[game.Slot][]*cand{}
	for _, slot := range game.Slots {
		var all []*cand
		for _, it := range cat.ItemsBySlot[slot] {
			if it.Level > p.level || excluded[it.ID] {
				continue
			}
			all = append(all, makeCand(it))
		}
		rank := func(c *cand) float64 { return c.utility + setShare[c.item.SetID] }
		general := func(vec []game.StatValue) float64 {
			total := 0.0
			for _, sv := range vec {
				total += runeWeight(cat.StatIDs[sv.Index]) * float64(sv.Value)
			}
			return total
		}
		sort.SliceStable(all, func(a, b int) bool {
			ra, rb := rank(all[a]), rank(all[b])
			if ra != rb {
				return ra > rb
			}
			return all[a].item.ID < all[b].item.ID
		})
		limit := perSlotCandidates
		switch slot {
		case game.SlotRing:
			limit = perSlotCandidates * 3 / 2
		case game.SlotDofus:
			limit = perSlotCandidates * 2
		}
		var kept []*cand
		taken := map[*cand]bool{}
		for i, c := range all {
			if (i < limit && rank(c) > 0) || keepSet[c.item.SetID] {
				kept = append(kept, c)
				taken[c] = true
			}
		}
		// Every slot gets filled, so each needs enough choices even when few
		// of its items touch the search: the strongest items overall top up
		// the list.
		if need := limit/2 - len(kept); need > 0 {
			rest := make([]*cand, 0, len(all))
			for _, c := range all {
				if !taken[c] {
					rest = append(rest, c)
				}
			}
			sort.SliceStable(rest, func(a, b int) bool { return general(rest[a].vec) > general(rest[b].vec) })
			kept = append(kept, rest[:min(need, len(rest))]...)
		}
		bySlot[slot] = kept
		for _, c := range kept {
			if keepSet[c.item.SetID] {
				p.setItems[c.item.SetID] = append(p.setItems[c.item.SetID], c)
			}
		}
	}

	// Once the request is served, the rest of the stuff should still be the
	// strongest available: every other stat counts for a thousandth of its
	// weight (taken below the last priority), enough to break ties and never
	// enough to trade a requested point.
	inObjective := map[int32]bool{}
	for _, idx := range p.objIdx {
		inObjective[idx] = true
	}
	for idx, statID := range cat.StatIDs {
		if !inObjective[int32(idx)] {
			p.objIdx = append(p.objIdx, int32(idx))
			p.objW = append(p.objW, tieBreakShare*lowest*runeWeight(statID))
		}
	}

	for i, pos := range game.Positions {
		if it, ok := lockedByPos[i]; ok {
			p.cands[i] = []*cand{makeCand(it)}
			p.locked[i] = true
			continue
		}
		// nil is the empty slot.
		for _, c := range bySlot[pos.Slot] {
			if game.Fits(c.item, pos) {
				p.cands[i] = append(p.cands[i], c)
			}
		}
		// nil is the empty slot. It costs emptyPenalty, so a slot is left
		// empty only when every item for it would make the stuff invalid.
		p.cands[i] = append(p.cands[i], nil)
		if len(p.cands[i]) > 1 {
			p.free = append(p.free, i)
		}
	}
	return p, nil
}

// finish turns an assignment into a solution: it hands the allowed exos to
// items, then checks the constraints on the stuff as the engine evaluates it.
func (p *problem) finish(assign []*cand, score float64, req Request) Solution {
	sol := Solution{Items: map[string]int{}, Score: math.Round(score*10) / 10, Unmet: []int{}, UnmetDamage: []int{}, Damage: []int{}, Character: req.Character}
	if p.points != nil {
		st := p.newState()
		st.load(assign)
		st.derive()
		base := make(map[string]int, primaries)
		for k, key := range engine.PrimaryStats {
			base[key] = int(st.spent[k])
		}
		sol.Character.Base = base
	}
	mods := map[string]*engine.ItemMods{}
	for i, c := range assign {
		if c == nil {
			continue
		}
		key := game.Positions[i].Key
		sol.Items[key] = c.item.ID
		if c.mods != nil {
			copied := *c.mods
			mods[key] = &copied
		}
	}

	// Each exo goes to an item that lacks the stat and holds no other
	// limited exo, preferring the one whose rune matters least: a
	// transcendence rune and an exo cannot share an item.
	taken := map[int]bool{}
	for _, statID := range p.exos {
		best := -1
		for i, c := range assign {
			if c == nil || taken[i] || !engine.ForgeableSlots[c.item.Slot] || hasStat(c.item, statID) {
				continue
			}
			// The exo must fit with what the item already carries.
			with := engine.ItemMods{}
			if m := mods[game.Positions[i].Key]; m != nil {
				with = *m
			}
			with.Transcendence = 0
			with.Exos = append(slices.Clone(with.Exos), engine.Exo{StatID: statID, Value: 1})
			if len(p.eng.Forge(c.item, &with).Issues) != 0 {
				continue
			}
			// An item without a rune first, else the one whose rune matters least.
			if best < 0 || (c.exoHost && !assign[best].exoHost) ||
				(c.exoHost == assign[best].exoHost && c.runeUtility < assign[best].runeUtility) {
				best = i
			}
		}
		if best < 0 {
			continue
		}
		taken[best] = true
		key := game.Positions[best].Key
		if mods[key] == nil {
			mods[key] = &engine.ItemMods{}
		}
		mods[key].Transcendence = 0
		mods[key].Exos = append(mods[key].Exos, engine.Exo{StatID: statID, Value: 1})
	}
	// A weapon goal's elemental forgemagic goes on the weapon it names.
	for _, d := range p.damage {
		if d.goal.Element == "" || sol.Items["weapon"] != d.goal.ID {
			continue
		}
		if mods["weapon"] == nil {
			mods["weapon"] = &engine.ItemMods{}
		}
		mods["weapon"].Element = d.goal.Element
	}
	if len(mods) > 0 {
		sol.Mods = mods
	}

	res := p.eng.Evaluate(engine.Build{Character: sol.Character, Items: sol.Items, Mods: sol.Mods}, "")
	for i, c := range req.Constraints {
		value := res.Stats[c.StatID]
		if (c.Op == "gte" && value < c.Value) || (c.Op == "lte" && value > c.Value) {
			sol.Unmet = append(sol.Unmet, i)
		}
	}
	// The damage goals are judged on the evaluated stuff, like the rest.
	stat := func(id int) int { return res.Stats[id] }
	for i, d := range p.damage {
		got := d.goal.Measured(d.specs, stat)
		sol.Damage = append(sol.Damage, got)
		if got < d.value {
			sol.UnmetDamage = append(sol.UnmetDamage, i)
		}
	}
	sol.Feasible = len(sol.Unmet) == 0 && len(sol.UnmetDamage) == 0 && res.Valid
	return sol
}

func hasStat(it *game.Item, statID int) bool {
	for _, e := range it.Effects {
		if e.Kind == game.EffectStat && e.StatID == statID {
			return true
		}
	}
	return false
}

type state struct {
	p   *problem
	cur []*cand

	raw, totals []int32
	// spent holds the points bought in each primary stat; withPoints is raw
	// plus those points. Both are only used when the solver spends them.
	spent      [primaries]int32
	withPoints []int32
	setCount   []int16
	setBonuses int
	ctx        engine.CriteriaContext
	// stat reads a characteristic of totals by its id, for the damage goals.
	stat func(statID int) int
}

func (p *problem) newState() *state {
	n := p.eng.StatCount()
	s := &state{
		p:          p,
		cur:        make([]*cand, npos),
		raw:        make([]int32, n),
		totals:     make([]int32, n),
		withPoints: make([]int32, n),
		setCount:   make([]int16, len(p.cat.Sets)),
	}
	copy(s.raw, p.base)
	s.ctx = engine.CriteriaContext{Catalog: p.cat, Totals: s.totals, Base: p.baseTotals}
	s.stat = func(statID int) int {
		if statID < 0 || statID >= len(p.statIdx) || p.statIdx[statID] < 0 {
			return 0
		}
		return int(s.totals[p.statIdx[statID]])
	}
	return s
}

func (s *state) load(assign []*cand) {
	for i := range s.cur {
		s.set(i, nil)
	}
	for i, c := range assign {
		s.set(i, c)
	}
}

func (s *state) applyVec(vec []game.StatValue, sign int32) {
	for _, sv := range vec {
		s.raw[sv.Index] += sign * sv.Value
	}
}

func (s *state) bump(c *cand, delta int16) {
	if c.setOrd < 0 {
		return
	}
	setID := c.item.SetID
	count := s.setCount[c.setOrd]
	s.applyVec(s.p.cat.SetBonusVector(setID, int(count)), -1)
	if count > 1 {
		s.setBonuses -= int(count) - 1
	}
	count += delta
	s.setCount[c.setOrd] = count
	s.applyVec(s.p.cat.SetBonusVector(setID, int(count)), 1)
	if count > 1 {
		s.setBonuses += int(count) - 1
	}
}

// set puts a candidate at a position without scoring the result.
func (s *state) set(pos int, c *cand) {
	old := s.cur[pos]
	if old == c {
		return
	}
	if old != nil {
		s.applyVec(old.vec, -1)
		s.bump(old, -1)
	}
	if c != nil {
		s.applyVec(c.vec, 1)
		s.bump(c, 1)
	}
	s.cur[pos] = c
}

// allowed enforces uniqueness: Dofus and trophies, and rings from a set.
func (s *state) allowed(pos int, c *cand) bool {
	if c == nil {
		return true
	}
	slot := game.Positions[pos].Slot
	if slot != game.SlotDofus && !(slot == game.SlotRing && c.item.SetID > 0) {
		return true
	}
	for i, other := range s.cur {
		if i != pos && other != nil && other.item.ID == c.item.ID {
			return false
		}
	}
	return true
}

// derive computes the final stats of the current stuff into s.totals,
// spending the characteristic points first when the solver owns them.
func (s *state) derive() {
	p := s.p
	p.eng.Derive(p.level, s.raw, s.totals)
	if p.points == nil {
		return
	}
	s.spent = p.points.spend(p.cons, s.totals)
	copy(s.withPoints, s.raw)
	for k, idx := range p.points.idx {
		if idx >= 0 {
			s.withPoints[idx] += s.spent[k]
		}
	}
	p.eng.Derive(p.level, s.withPoints, s.totals)
}

func (s *state) eval() (float64, bool) {
	p := s.p
	s.derive()
	score := 0.0
	for i, idx := range p.objIdx {
		score += p.objW[i] * float64(s.totals[idx])
	}
	feasible := true
	for _, c := range p.cons {
		miss := c.value - s.totals[c.idx]
		if !c.gte {
			miss = -miss
		}
		if miss > 0 {
			feasible = false
			score -= constraintFlat + constraintPenalty*c.weight*float64(miss)
		}
	}
	for i := range p.damage {
		d := &p.damage[i]
		got := d.goal.Measured(d.specs, s.stat)
		score += d.weight * float64(got)
		if miss := d.value - got; miss > 0 {
			feasible = false
			score -= constraintFlat + constraintPenalty*damageUnit*float64(miss)
		}
	}
	s.ctx.SetBonuses = s.setBonuses
	hosts := 0
	for _, c := range s.cur {
		if c == nil {
			score -= emptyPenalty
			continue
		}
		if c.exoHost {
			hosts++
		}
		score -= c.cost
		if c.crit != nil && !c.crit.Check(&s.ctx) {
			feasible = false
			score -= criteriaPenalty
		}
	}
	// Each allowed exo was counted up front and needs an item to sit on.
	if missing := len(p.exos) - hosts; missing > 0 {
		feasible = false
		score -= exoHostPenalty * float64(missing)
	}
	return score, feasible
}

// climb greedily replaces one item at a time while the score improves.
func (s *state) climb() int64 {
	var evals int64
	best, _ := s.eval()
	for improved := true; improved; {
		improved = false
		for _, pos := range s.p.free {
			original := s.cur[pos]
			winner := original
			for _, c := range s.p.cands[pos] {
				if c == original || !s.allowed(pos, c) {
					continue
				}
				s.set(pos, c)
				evals++
				if score, _ := s.eval(); score > best+1e-9 {
					best, winner = score, c
				}
			}
			s.set(pos, winner)
			if winner != original {
				improved = true
			}
		}
		// Then the forgemagic of each worn item, locked ones included.
		for pos, original := range s.cur {
			if original == nil || len(original.variants) == 0 {
				continue
			}
			winner := original
			for _, v := range original.variants {
				if v == original {
					continue
				}
				s.set(pos, v)
				evals++
				if score, _ := s.eval(); score > best+1e-9 {
					best, winner = score, v
				}
			}
			s.set(pos, winner)
			if winner != original {
				improved = true
			}
		}
	}
	return evals
}

// climbVariants improves the forgemagic of the worn items one item at a
// time, leaving the items themselves alone, until no single change helps.
func (s *state) climbVariants(best float64) (int64, float64) {
	var evals int64
	for improved := true; improved; {
		improved = false
		for pos, original := range s.cur {
			if original == nil || len(original.variants) == 0 {
				continue
			}
			winner := original
			for _, v := range original.variants {
				if v == original {
					continue
				}
				s.set(pos, v)
				evals++
				if score, _ := s.eval(); score > best+1e-9 {
					best, winner = score, v
				}
			}
			s.set(pos, winner)
			if winner != original {
				improved = true
			}
		}
	}
	return evals, best
}

// deepVariants looks for forgemagic that only pays once other items follow:
// it forces each variant of each item in turn, lets the rest of the stuff
// re-choose its forgemagic around it, and keeps the result when it beats the
// current one. One rune taking over the role of two others is found this
// way, where changing one item at a time is not.
func (s *state) deepVariants() int64 {
	var evals int64
	best, _ := s.eval()
	saved := make([]*cand, npos)
	for improved := true; improved; {
		improved = false
		for pos := 0; pos < npos; pos++ {
			original := s.cur[pos]
			if original == nil || len(original.variants) == 0 {
				continue
			}
			for _, v := range original.variants {
				if v == s.cur[pos] {
					continue
				}
				copy(saved, s.cur)
				s.set(pos, v)
				forced, _ := s.eval()
				n, reached := s.climbVariants(forced)
				evals += n + 1
				if reached > best+1e-9 {
					best = reached
					improved = true
					continue
				}
				for i, c := range saved {
					s.set(i, c)
				}
			}
		}
	}
	return evals
}

// exchangeClimb tries, for every group of k free positions, every
// combination of the pieces the leading stuffs wear there (at most width a
// position), and keeps one that beats the current stuff. A climb follows
// each gain; it repeats until none is left.
func (s *state) exchangeClimb(ctx context.Context, elite [][]*cand, k, width int) int64 {
	var evals int64
	free := s.p.free
	if len(free) < k {
		return 0
	}
	evals += s.climb()
	best, _ := s.eval()
	group := make([]int, k)
	old := make([]*cand, k)
	win := make([]*cand, k)
	trial := make([]*cand, k)
	found := best

	// fill tries every piece at the positions of the group from depth on.
	var fill func(depth int)
	fill = func(depth int) {
		if depth == k {
			evals++
			if score, _ := s.eval(); score > found+1e-9 {
				found = score
				copy(win, trial)
			}
			return
		}
		pos := group[depth]
		for n, c := range elite[pos] {
			if n >= width {
				break
			}
			s.set(pos, c)
			if s.allowed(pos, c) {
				trial[depth] = c
				fill(depth + 1)
			}
		}
		s.set(pos, old[depth])
	}

	// choose walks the groups of k positions.
	var choose func(start, depth int) bool
	choose = func(start, depth int) bool {
		if depth == k {
			if ctx.Err() != nil {
				return false
			}
			for n, pos := range group {
				old[n] = s.cur[pos]
			}
			copy(win, old)
			found = best
			fill(0)
			changed := false
			for n, pos := range group {
				s.set(pos, win[n])
				changed = changed || win[n] != old[n]
			}
			if changed {
				evals += s.climb()
				best, _ = s.eval()
			}
			return changed
		}
		changed := false
		for x := start; x <= len(free)-(k-depth); x++ {
			group[depth] = free[x]
			if choose(x+1, depth+1) {
				changed = true
			}
		}
		return changed
	}
	for choose(0, 0) {
		if ctx.Err() != nil {
			break
		}
	}
	return evals
}

// seedSets builds a stuff around whole sets: every shortlisted piece of the
// sets given, the most relevant item elsewhere, then a climb. Sets are what
// ties positions together, so these starts reach stuffs that one-item moves
// from a random start may never find.
func (p *problem) seedSets(ctx context.Context, pl *pool, sets []int) int64 {
	if ctx.Err() != nil || len(p.free) == 0 {
		return 0
	}
	s := p.newState()
	for i, list := range p.cands {
		if p.locked[i] {
			s.set(i, list[0])
		}
	}
	for _, setID := range sets {
		for _, c := range p.setItems[setID] {
			for _, pos := range p.free {
				if s.cur[pos] == nil && game.Fits(c.item, game.Positions[pos]) && s.allowed(pos, c) {
					s.set(pos, c)
					break
				}
			}
		}
	}
	for _, pos := range p.free {
		if s.cur[pos] != nil {
			continue
		}
		for _, c := range p.cands[pos] {
			if c != nil && s.allowed(pos, c) {
				s.set(pos, c)
				break
			}
		}
	}
	evals := s.climb() + 1
	score, feasible := s.eval()
	pl.offer(s.cur, score, feasible)
	return evals
}

// pairClimb tries, for every two free positions, every pair of shortlisted
// items, and keeps a pair that beats the current stuff once its forgemagic
// is chosen again. It repeats until no pair helps.
func (s *state) pairClimb(ctx context.Context) int64 {
	var evals int64
	free := s.p.free
	best, _ := s.eval()
	for improved := true; improved; {
		improved = false
		for x := 0; x < len(free); x++ {
			for y := x + 1; y < len(free); y++ {
				if ctx.Err() != nil {
					return evals
				}
				i, j := free[x], free[y]
				oldI, oldJ := s.cur[i], s.cur[j]
				bestI, bestJ := oldI, oldJ
				found := best
				for _, a := range s.p.cands[i] {
					if a == oldI {
						continue
					}
					s.set(i, a)
					if !s.allowed(i, a) {
						continue
					}
					for _, b := range s.p.cands[j] {
						if b == oldJ || !s.allowed(j, b) {
							continue
						}
						s.set(j, b)
						evals++
						if score, _ := s.eval(); score > found+1e-9 {
							found, bestI, bestJ = score, a, b
						}
					}
					s.set(j, oldJ)
				}
				s.set(i, bestI)
				s.set(j, bestJ)
				if bestI != oldI || bestJ != oldJ {
					// The new pair may want other forgemagic around it.
					evals += s.climb()
					best, _ = s.eval()
					improved = true
				}
			}
		}
	}
	return evals
}

func (p *problem) anneal(ctx context.Context, rng *rand.Rand, pl *pool) int64 {
	if len(p.free) == 0 {
		s := p.newState()
		for i, list := range p.cands {
			s.set(i, list[0])
		}
		evals := s.climb() + 1
		score, feasible := s.eval()
		pl.offer(s.cur, score, feasible)
		return evals
	}
	var evals int64
	s := p.newState()
	type change struct {
		pos int
		old *cand
	}
	var undo []change

	// pick favours the head of the list, where the most relevant items are.
	pick := func(list []*cand) *cand {
		r := rng.Float64()
		return list[int(r*r*float64(len(list)))]
	}

	for round := 0; round < p.rounds; round++ {
		for i := range s.cur {
			s.set(i, nil)
		}
		for i, list := range p.cands {
			if p.locked[i] {
				s.set(i, list[0])
				continue
			}
			// Start from a full stuff: a random strong choice, else the
			// first one the uniqueness rules allow.
			c := list[rng.IntN(min(len(list), 6))]
			if !s.allowed(i, c) {
				for _, other := range list {
					if s.allowed(i, other) {
						c = other
						break
					}
				}
			}
			if s.allowed(i, c) {
				s.set(i, c)
			}
		}
		score, feasible := s.eval()
		pl.offer(s.cur, score, feasible)

		// Calibrate the temperature on the size of typical moves.
		sum, samples := 0.0, 0
		for range 120 {
			pos := p.free[rng.IntN(len(p.free))]
			c := pick(p.cands[pos])
			if !s.allowed(pos, c) {
				continue
			}
			old := s.cur[pos]
			s.set(pos, c)
			next, _ := s.eval()
			s.set(pos, old)
			sum += math.Abs(next - score)
			samples++
		}
		t0 := 1.0
		if samples > 0 && sum > 0 {
			t0 = sum / float64(samples) * 0.6
		}
		t1 := t0 * 0.004
		cooling := math.Pow(t1/t0, 1/float64(annealIterations))
		temp := t0

		for it := 0; it < annealIterations; it++ {
			if it&4095 == 0 && ctx.Err() != nil {
				return evals
			}
			temp *= cooling
			undo = undo[:0]

			if len(p.setList) > 0 && rng.Float64() < 0.04 {
				// Set move: put on several pieces of one set at once.
				pieces := p.setItems[p.setList[rng.IntN(len(p.setList))]]
				for _, c := range pieces {
					if rng.Float64() < 0.25 {
						continue
					}
					for _, pos := range p.free {
						if !game.Fits(c.item, game.Positions[pos]) || s.cur[pos] == c || !s.allowed(pos, c) {
							continue
						}
						undo = append(undo, change{pos, s.cur[pos]})
						s.set(pos, c)
						break
					}
				}
			} else if rng.Float64() < variantMoveShare {
				// Forgemagic move: another variant of a worn item.
				pos := rng.IntN(npos)
				cur := s.cur[pos]
				if cur == nil || len(cur.variants) == 0 {
					continue
				}
				v := cur.variants[rng.IntN(len(cur.variants))]
				if v == cur {
					continue
				}
				undo = append(undo, change{pos, cur})
				s.set(pos, v)
			} else {
				pos := p.free[rng.IntN(len(p.free))]
				c := pick(p.cands[pos])
				if c == s.cur[pos] || !s.allowed(pos, c) {
					continue
				}
				undo = append(undo, change{pos, s.cur[pos]})
				s.set(pos, c)
			}
			if len(undo) == 0 {
				continue
			}

			next, nextFeasible := s.eval()
			evals++
			if next >= score || rng.Float64() < math.Exp((next-score)/temp) {
				score, feasible = next, nextFeasible
				pl.offer(s.cur, score, feasible)
				continue
			}
			for i := len(undo) - 1; i >= 0; i-- {
				s.set(undo[i].pos, undo[i].old)
			}
		}
		evals += s.climb()
		score, feasible = s.eval()
		pl.offer(s.cur, score, feasible)
	}
	return evals
}

type entry struct {
	assign   []*cand
	score    float64
	feasible bool
}

// pool keeps the best distinct assignments seen so far.
// A set of items is kept once, with the best forgemagic seen for it.
type pool struct {
	entries []entry
	// seen gives, per set of items, the best rank offered so far.
	seen  map[[16]int32]float64
	floor float64
}

func newPool() *pool {
	return &pool{seen: map[[16]int32]float64{}, floor: math.Inf(-1)}
}

// rankScore orders feasible results above infeasible ones.
func rankScore(score float64, feasible bool) float64 {
	if feasible {
		return score + 1e9
	}
	return score
}

// signature identifies a build regardless of which ring or Dofus slot holds what.
func signature(assign []*cand) [16]int32 {
	var sig [16]int32
	for i, c := range assign {
		if c != nil {
			sig[i] = int32(c.item.ID)
		}
	}
	for i := 0; i < npos; {
		j := i + 1
		for j < npos && game.Positions[j].Slot == game.Positions[i].Slot {
			j++
		}
		group := sig[i:j]
		sort.Slice(group, func(a, b int) bool { return group[a] < group[b] })
		i = j
	}
	return sig
}

func (pl *pool) offer(assign []*cand, score float64, feasible bool) {
	rank := rankScore(score, feasible)
	if len(pl.entries) >= poolSize && rank <= pl.floor {
		return
	}
	sig := signature(assign)
	if prev, ok := pl.seen[sig]; ok && rank <= prev {
		return
	}
	pl.seen[sig] = rank
	pl.entries = append(pl.entries, entry{append([]*cand(nil), assign...), score, feasible})
	if len(pl.entries) >= poolSize*3 {
		pl.sort()
		if len(pl.entries) > poolSize {
			for _, e := range pl.entries[poolSize:] {
				delete(pl.seen, signature(e.assign))
			}
			pl.entries = pl.entries[:poolSize]
			last := pl.entries[len(pl.entries)-1]
			pl.floor = rankScore(last.score, last.feasible)
		}
	}
}

// sort ranks the entries, best first, and drops the weaker forgemagic of a
// set of items already listed.
func (pl *pool) sort() {
	sort.SliceStable(pl.entries, func(a, b int) bool {
		ra := rankScore(pl.entries[a].score, pl.entries[a].feasible)
		rb := rankScore(pl.entries[b].score, pl.entries[b].feasible)
		if ra != rb {
			return ra > rb
		}
		return lessSig(signature(pl.entries[a].assign), signature(pl.entries[b].assign))
	})
	listed := make(map[[16]int32]bool, len(pl.entries))
	kept := pl.entries[:0]
	for _, e := range pl.entries {
		sig := signature(e.assign)
		if listed[sig] {
			continue
		}
		listed[sig] = true
		kept = append(kept, e)
	}
	pl.entries = kept
}

func lessSig(a, b [16]int32) bool {
	for i := range a {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return false
}

// distance counts the items that two builds do not share.
func distance(a, b []*cand) int {
	count := map[int]int{}
	for _, c := range a {
		if c != nil {
			count[c.item.ID]++
		}
	}
	shared := 0
	for _, c := range b {
		if c != nil && count[c.item.ID] > 0 {
			count[c.item.ID]--
			shared++
		}
	}
	worn := 0
	for _, c := range a {
		if c != nil {
			worn++
		}
	}
	return worn - shared
}
