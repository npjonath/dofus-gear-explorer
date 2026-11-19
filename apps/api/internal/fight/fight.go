package fight

import (
	"math/rand/v2"

	"dofusdb/api/internal/engine"
	"dofusdb/api/internal/game"
)

// Phases of a fight: the entities are placed, then the turns are played.
const (
	PhasePlacement = "placement"
	PhaseFight     = "fight"
)

// Kinds of entity.
const (
	KindPlayer = "player"
	KindDummy  = "dummy"
)

// WeaponID is the id the weapon is cast under, among the spells.
const WeaponID = 0

// Kinds of lasting effect on an entity.
const (
	// BuffStat adds Value to the characteristic Ref.
	BuffStat = "stat"
	// BuffState puts the entity in the state Ref.
	BuffState = "state"
	// BuffBase adds Value to the base damage of the spell Ref, and BuffRange
	// to its range.
	BuffBase  = "base"
	BuffRange = "range"
	// BuffSuffered multiplies the damage the entity takes by Value percent,
	// and BuffHealed the heals it receives.
	BuffShield   = "shield"
	BuffArmor    = "armor"
	BuffSuffered = "suffered"
	BuffHealed   = "healed"
	// BuffSpell changes how the spell Ref is cast: Mod is the game's action
	// (its cost, its range, its casts per turn...) and Value the amount.
	BuffSpell = "spell"
)

// Caps on what an entity carries, far above what a fight puts on it.
const (
	maxBuffs = 256
	maxHooks = 256
)

// Buff is a lasting effect on an entity.
type Buff struct {
	Mark int    `json:"mark,omitempty"`
	Kind string `json:"kind"`
	// Spell is the spell that applied it, which may be one another spell
	// cast in turn, and From the spell the player cast.
	Spell int `json:"spell"`
	From  int `json:"from"`
	Ref   int `json:"ref"`
	Value int `json:"value"`
	Mod   int `json:"mod,omitempty"`
	// Turns is how many turns it still lasts, the current one included; -1
	// never ends.
	Turns int `json:"turns"`
}

// Hook is an effect waiting on an entity for one of its triggers: the start
// or the end of a turn, a state gained or lost, a hit taken. It names the
// effect by where it is in the game data rather than carrying it.
type Hook struct {
	Mark     int  `json:"mark,omitempty"`
	Spell    int  `json:"spell"`
	Grade    int  `json:"grade"`
	Critical bool `json:"critical,omitempty"`
	Index    int  `json:"index"`
	// From is the spell the player cast and Caster who cast it.
	From   int `json:"from"`
	Caster int `json:"caster"`
	// Turns is how many turns it still waits; -1 for the whole fight.
	Turns int `json:"turns"`
	// Delay is the number of turns before the effect counts at all: an
	// effect with no trigger applies once when it reaches zero, one with a
	// trigger starts waiting then.
	Delay int `json:"delay,omitempty"`
}

// Entity is a fighter on the board.
type Entity struct {
	InitialHP int          `json:"initialHp,omitempty"`
	Ally      bool         `json:"ally,omitempty"`
	Name      string       `json:"name,omitempty"`
	MonsterID int          `json:"monsterId,omitempty"`
	Grade     int          `json:"grade,omitempty"`
	Owner     int          `json:"owner,omitempty"`
	Uses      map[int]*Use `json:"uses,omitempty"`
	ID        int          `json:"id"`
	Kind      string       `json:"kind"`
	Cell      int          `json:"cell"`
	HP        int          `json:"hp"`
	MaxHP     int          `json:"maxHp"`
	// Stats holds a dummy's own characteristics: its resistances. The
	// player's come from the stuff.
	Stats map[int]int `json:"stats,omitempty"`
	Buffs []Buff      `json:"buffs"`
	Hooks []Hook      `json:"hooks,omitempty"`
}

// Use counts the casts of a spell.
type Use struct {
	// Turn is the number of casts this turn and Last the turn of the latest.
	Turn int `json:"turn"`
	Last int `json:"last"`
	// Targets counts this turn's casts per entity aimed at.
	Targets map[int]int `json:"targets,omitempty"`
}

// State is a fight as it stands. The interface holds it and sends it back
// with each action.
type State struct {
	ActiveID     int      `json:"activeId,omitempty"`
	LastEntityID int      `json:"lastEntityId,omitempty"`
	Marks        []Mark   `json:"marks,omitempty"`
	Board        Board    `json:"board"`
	Phase        string   `json:"phase"`
	Turn         int      `json:"turn"`
	Entities     []Entity `json:"entities"`
	// APUsed and MPUsed are the points the player spent this turn.
	APUsed int          `json:"apUsed"`
	MPUsed int          `json:"mpUsed"`
	Uses   map[int]*Use `json:"uses"`
	// TurnDamage and TotalDamage sum what the dummies lost.
	TurnDamage  int `json:"turnDamage"`
	TotalDamage int `json:"totalDamage"`
}

// Dummies configures the training dummies of a fight.
type Dummies struct {
	Allies int `json:"allies,omitempty"`
	MapID  int `json:"mapId,omitempty"`
	Count  int `json:"count"`
	HP     int `json:"hp"`
	// Stats are their characteristics, by the game's ids: the resistances
	// in percent and fixed, per element and to critical hits.
	Stats map[int]int `json:"stats"`
}

// Sim plays a fight for one stuff.
type Sim struct {
	Creatures map[int]*Creature

	Cat *game.Catalog
	// Stat gives the stuff's final characteristics.
	Stat  func(statID int) int
	Level int
	// Breed is the character's class, whose starting spells open the fight;
	// zero for none. Passives are the spells of the passives of the items
	// worn, which open it too.
	Breed    int
	Passives []int
	// Weapon is the weapon worn, if any, and WeaponElement its elemental
	// forgemagic.
	Weapon        *game.Item
	WeaponElement string
	// Rand draws the rolls, the critical hits and the random effects.
	Rand *rand.Rand
}

// Characteristics the simulator reads besides those of the damage formula.
const (
	statFinalDamage   = 107
	statPushResist    = 85
	statCriticalResis = 87
)

var (
	percentResist = map[int]int{game.ElementEarth: 33, game.ElementFire: 34, game.ElementWater: 35, game.ElementAir: 36, game.ElementNeutral: 37}
	fixedResist   = map[int]int{game.ElementEarth: 54, game.ElementFire: 55, game.ElementWater: 56, game.ElementAir: 57, game.ElementNeutral: 58}
)

// ResistStats lists the characteristics a dummy can be given.
func ResistStats() []int {
	return []int{37, 33, 34, 35, 36, 58, 54, 55, 56, 57, statCriticalResis}
}

func (st *State) entity(id int) *Entity {
	for i := range st.Entities {
		if st.Entities[i].ID == id {
			return &st.Entities[i]
		}
	}
	return nil
}

func (st *State) at(cell int) *Entity {
	for i := range st.Entities {
		if st.Entities[i].Cell == cell {
			return &st.Entities[i]
		}
	}
	return nil
}

// Player returns the player's entity.
func (st *State) Player() *Entity {
	for i := range st.Entities {
		if st.Entities[i].Kind == KindPlayer {
			return &st.Entities[i]
		}
	}
	return nil
}

func (st *State) free(cell int) bool {
	return st.Board.valid(cell) && !st.Board.wall(cell) && st.at(cell) == nil
}

func (st *State) clone() *State {
	out := *st
	out.Marks = append([]Mark(nil), st.Marks...)
	out.Board.Walls = append([]int(nil), st.Board.Walls...)
	out.Entities = make([]Entity, len(st.Entities))
	for i, e := range st.Entities {
		e.Buffs = append([]Buff(nil), e.Buffs...)
		e.Hooks = append([]Hook(nil), e.Hooks...)
		out.Entities[i] = e
	}
	out.Uses = make(map[int]*Use, len(st.Uses))
	for id, use := range st.Uses {
		copied := *use
		copied.Targets = make(map[int]int, len(use.Targets))
		for k, v := range use.Targets {
			copied.Targets[k] = v
		}
		out.Uses[id] = &copied
	}
	return &out
}

// Start sets a fight up: the player on one side, the dummies on the other,
// ready to be placed.
func (s *Sim) Start(d Dummies) *State {
	return s.StartOn(d, DefaultBoard())
}

func (s *Sim) StartOn(d Dummies, board Board) *State {
	st := &State{Board: board, Phase: PhasePlacement, Uses: map[int]*Use{}}
	health := max(1, s.Stat(game.StatHP))
	mid := board.Height / 2
	playerCell := board.cell(3, mid)
	if len(board.Blue) > 0 {
		playerCell = board.Blue[0]
	}
	st.Entities = append(st.Entities, Entity{ID: 1, Kind: KindPlayer, Cell: playerCell, HP: health, MaxHP: health, Buffs: []Buff{}})
	count := min(4, max(1, d.Count))
	hp := min(1_000_000, max(1, d.HP))
	spots := [][2]int{{9, mid}, {9, mid - 1}, {9, mid + 1}, {10, mid}}
	for i := 0; i < count; i++ {
		stats := map[int]int{}
		for _, id := range ResistStats() {
			if v := d.Stats[id]; v != 0 {
				stats[id] = v
			}
		}
		dummyCell := board.cell(spots[i][0], spots[i][1])
		ally := i >= count-min(count-1, max(0, d.Allies))
		if len(board.Red) > i {
			dummyCell = board.Red[i]
		}
		if ally && len(board.Blue) > i+1 {
			dummyCell = board.Blue[i+1]
		}
		st.Entities = append(st.Entities, Entity{
			ID: i + 2, Kind: KindDummy, Ally: ally, Cell: dummyCell, HP: hp, MaxHP: hp, Stats: stats, Buffs: []Buff{},
		})
	}
	return st
}

// StatOf gives an entity's characteristics as they stand: the stuff's for
// the player, a dummy's own, and what the lasting effects add.
func (s *Sim) StatOf(e *Entity) func(statID int) int {
	return func(id int) int {
		v := e.Stats[id]
		if e.Kind == KindPlayer {
			v += s.Stat(id)
		}
		for _, b := range e.Buffs {
			if b.Kind == BuffStat && b.Ref == id {
				v += b.Value
			}
		}
		return v
	}
}

// Points returns the action and movement points the player has left.
func (s *Sim) Points(st *State) (ap, mp int) {
	stat := s.StatOf(st.Active())
	return stat(game.StatAP) - st.APUsed, stat(game.StatMP) - st.MPUsed
}

// attack is a weapon or a spell grade, as the simulator casts it.
type attack struct {
	id          int
	grade       int
	weapon      bool
	cost        int
	rangeMin    int
	rangeMax    int
	crit        int
	rules       game.CastRules
	effects     []game.SpellEffect
	critEffects []game.SpellEffect
}

// weaponActions gives the game's action of a weapon line, per element.
var weaponActions = map[bool]map[int]int{
	false: {game.ElementNeutral: 100, game.ElementEarth: 97, game.ElementFire: 99, game.ElementWater: 96, game.ElementAir: 98},
	true:  {game.ElementNeutral: 95, game.ElementEarth: 92, game.ElementFire: 94, game.ElementWater: 91, game.ElementAir: 93},
}

// attack returns the weapon, or a spell at a grade; grade zero is the one a
// character of the fight's level casts. It is nil when there is none.
func (s *Sim) attack(id, grade int) *attack {
	if id == WeaponID {
		lines := engine.WeaponLines(s.Weapon, s.WeaponElement)
		if len(lines) == 0 {
			return nil
		}
		w := s.Weapon.Weapon
		a := &attack{
			weapon: true, cost: w.APCost, rangeMin: w.RangeMin, rangeMax: w.RangeMax, crit: w.CritChance,
			rules: game.CastRules{Sight: true, PerTurn: w.MaxCastPerTurn},
		}
		for _, line := range lines {
			action := weaponActions[line.Steal][line.Element]
			a.effects = append(a.effects, game.SpellEffect{Action: action, Min: line.Min, Max: line.Max, Zone: game.Zone{Shape: "P"}})
			if line.CritMax > 0 {
				a.critEffects = append(a.critEffects, game.SpellEffect{Action: action, Min: line.CritMin, Max: line.CritMax, Zone: game.Zone{Shape: "P"}})
			}
		}
		return a
	}
	sp := s.Cat.SpellByID[id]
	if sp == nil {
		return nil
	}
	var level *game.SpellLevel
	for i := range sp.Levels {
		if sp.Levels[i].Grade == grade {
			level = &sp.Levels[i]
		}
	}
	if level == nil {
		level = sp.LevelAt(s.Level)
	}
	if level == nil {
		return nil
	}
	return &attack{
		id: id, grade: level.Grade, cost: level.APCost, rangeMin: level.RangeMin, rangeMax: level.RangeMax, crit: level.CritChance,
		rules: level.Cast, effects: level.Effects, critEffects: level.CritEffects,
	}
}

// tuned returns a spell as an entity casts it, with what its lasting
// effects change of the spell: its cost, its range, its limits.
func (s *Sim) tuned(caster *Entity, a *attack) *attack {
	if a == nil || a.weapon {
		return a
	}
	out := *a
	for _, b := range caster.Buffs {
		if b.Kind != BuffSpell || b.Ref != a.id {
			continue
		}
		switch b.Mod {
		case modCostLess:
			out.cost -= b.Value
		case modCostMore:
			out.cost += b.Value
		case modMinRange:
			out.rangeMin += b.Value
		case modMinRangeLess:
			out.rangeMin -= b.Value
		case modMinRangeSet:
			out.rangeMin = b.Value
		case modMaxRangeLess:
			out.rangeMax -= b.Value
		case modMaxRangeSet:
			out.rangeMax = b.Value
		case modCritical:
			out.crit += b.Value
		case modPerTurn:
			if out.rules.PerTurn > 0 {
				out.rules.PerTurn += b.Value
			}
		case modPerTarget:
			if out.rules.PerTarget > 0 {
				out.rules.PerTarget += b.Value
			}
		case modCooldownLess:
			out.rules.Interval -= b.Value
		case modCooldownSet:
			out.rules.Interval = b.Value
		case modNoSight:
			out.rules.Sight = false
		case modNoLine:
			out.rules.Line = false
		case modRangeBoosted:
			out.rules.BoostRange = true
		case modTakenCellOff:
			out.rules.TakenCell = false
		case modTakenCellOn:
			out.rules.TakenCell = true
		case modFreeCellOff:
			out.rules.FreeCell = false
		case modFreeCellOn:
			out.rules.FreeCell = true
		}
	}
	out.cost, out.rangeMin = max(0, out.cost), max(0, out.rangeMin)
	out.rangeMax = max(out.rangeMin, out.rangeMax)
	return &out
}

// spell is the weapon or a spell as the player casts it now.
func (s *Sim) spell(st *State, id int) *attack {
	active := st.Active()
	if active.Kind == KindSummon {
		_, g := s.grade(active.MonsterID, active.Grade)
		if g != nil {
			for _, sp := range g.Spells {
				if sp.ID == id {
					return s.tuned(active, s.attack(id, sp.Grade))
				}
			}
		}
		return nil
	}
	sp := s.Cat.SpellByID[id]
	if id != WeaponID && (sp == nil || sp.BreedID != 0 && sp.BreedID != s.Breed) {
		return nil
	}
	return s.tuned(active, s.attack(id, 0))
}

// critRate is the chance of a critical hit, from 0 to 100.
func (s *Sim) critRate(a *attack, stat func(int) int) int {
	if a.crit <= 0 || len(a.critEffects) == 0 {
		return 0
	}
	return max(0, min(100, a.crit+stat(game.StatCritical)))
}

// Why a spell cannot be cast, or cannot be aimed at a cell.
const (
	BlockedPhase     = "phase"
	BlockedAP        = "ap"
	BlockedCooldown  = "cooldown"
	BlockedPerTurn   = "perTurn"
	BlockedPerTarget = "perTarget"
	BlockedRange     = "range"
	BlockedLine      = "line"
	BlockedSight     = "sight"
	BlockedCell      = "cell"
	BlockedMP        = "mp"
	BlockedUnknown   = "unknown"
)

// cooldown is the number of turns before the spell can be cast again.
func (st *State) cooldown(a *attack) int {
	// Before the first turn, the wait is the one that turn will start with.
	left := a.rules.InitialCooldown - max(1, st.Turn) + 1
	if use := st.counters()[a.id]; use != nil && use.Last > 0 && a.rules.Interval > 0 {
		left = max(left, a.rules.Interval-(st.Turn-use.Last))
	}
	return max(0, left)
}

// blocked says why the spell cannot be cast right now, wherever it is aimed.
func (s *Sim) blocked(st *State, a *attack) string {
	if st.Phase != PhaseFight {
		return BlockedPhase
	}
	if st.cooldown(a) > 0 {
		return BlockedCooldown
	}
	if use := st.counters()[a.id]; use != nil && a.rules.PerTurn > 0 && use.Turn >= a.rules.PerTurn {
		return BlockedPerTurn
	}
	if ap, _ := s.Points(st); a.cost > ap {
		return BlockedAP
	}
	return ""
}

// reach is the spell's range for the caster as it stands.
func (s *Sim) reach(caster *Entity, a *attack) (lo, hi int) {
	lo, hi = a.rangeMin, a.rangeMax
	if a.rules.BoostRange {
		hi += s.StatOf(caster)(game.StatRange)
	}
	for _, b := range caster.Buffs {
		if b.Kind == BuffRange && b.Ref == a.id && !a.weapon {
			hi += b.Value
		}
	}
	return lo, max(lo, hi)
}

func (st *State) sees(from, to int) bool {
	for _, c := range st.Board.between(from, to) {
		if st.Board.blocksSight(c) || st.at(c) != nil {
			return false
		}
	}
	return true
}

// aimable says why the spell cannot be aimed at a cell; the range and the
// shape of the cast come first, then what stands in the way.
func (s *Sim) aimable(st *State, a *attack, caster *Entity, cell int) string {
	if !st.Board.valid(cell) || st.Board.wall(cell) {
		return BlockedCell
	}
	lo, hi := s.reach(caster, a)
	if d := st.Board.Distance(caster.Cell, cell); d < lo || d > hi {
		return BlockedRange
	}
	x0, y0 := st.Board.xy(caster.Cell)
	x1, y1 := st.Board.xy(cell)
	straight, diagonal := x0 == x1 || y0 == y1, abs(x1-x0) == abs(y1-y0)
	if (a.rules.Line || a.rules.Diagonal) && !(a.rules.Line && straight) && !(a.rules.Diagonal && diagonal) {
		return BlockedLine
	}
	occupant := st.at(cell)
	if (a.rules.FreeCell && occupant != nil) || (a.rules.TakenCell && occupant == nil) {
		return BlockedCell
	}
	if a.rules.Sight && !st.sees(caster.Cell, cell) {
		return BlockedSight
	}
	if use := st.counters()[a.id]; use != nil && occupant != nil && a.rules.PerTarget > 0 && use.Targets[occupant.ID] >= a.rules.PerTarget {
		return BlockedPerTarget
	}
	return ""
}

// Place moves an entity to a free cell before the fight starts.
func (s *Sim) Place(st *State, entityID, cell int) string {
	e := st.entity(entityID)
	if st.Phase != PhasePlacement || e == nil {
		return BlockedPhase
	}
	if !st.free(cell) || !st.Board.inCamp(e, cell) {
		return BlockedCell
	}
	e.Cell = cell
	return ""
}

// draw picks one of the outcomes a spell draws at random, by their chance
// in percent; none when the chances fall short of a hundred and luck says so.
func (s *Sim) draw(weights []float64) int {
	total := 0.0
	for _, w := range weights {
		total += w
	}
	at := s.Rand.Float64() * max(total, 100)
	for i, w := range weights {
		if at -= w; at < 0 {
			return i
		}
	}
	return -1
}

func (s *Sim) casting(root int, crit, quiet bool) *casting {
	work := maxWork
	return &casting{root: root, crit: crit, quiet: quiet, work: &work, pick: s.draw, roll: func(lo, hi int) int { return lo + s.Rand.IntN(hi-lo+1) }}
}

// Begin starts the first turn: the class's starting spells are cast on the
// character, which puts it in its states and sets its passive effects
// waiting.
func (s *Sim) Begin(st *State) ([]Event, string) {
	if st.Phase != PhasePlacement {
		return nil, BlockedPhase
	}
	st.Phase, st.Turn = PhaseFight, 1
	var events []Event
	player := st.Player()
	var opening []int
	for _, sp := range s.Cat.StartSpells[s.Breed] {
		opening = append(opening, sp.ID)
	}
	for i, id := range append(opening, s.Passives...) {
		a := s.attack(id, 0)
		if i >= len(opening) {
			// The grades of an item's passive are its phases, not levels:
			// the fight starts with the first.
			a = s.attack(id, 1)
		}
		if a == nil {
			continue
		}
		c := s.casting(id, false, true)
		if i >= len(opening) {
			c.events = append(c.events, Event{Type: EventCast, Entity: player.ID, Spell: id})
		}
		s.resolve(st, c, player.ID, player.ID, a, player.Cell, 0)
		s.settle(st, c)
		events = append(events, c.events...)
	}
	events = append(events, Event{Type: EventTurn, Value: 1})
	c := s.casting(0, false, false)
	s.fire(st, c, player.ID, player.ID, 0, "TB")
	s.settle(st, c)
	return append(events, c.events...), ""
}

// Step is a cell the player can walk to: what it costs, and the cell the
// shortest way there comes from.
type Step struct {
	Cell int `json:"cell"`
	Cost int `json:"cost"`
	From int `json:"from"`
}

// Reach lists the cells the player can walk to with the movement points
// left, nearest first.
func (s *Sim) Reach(st *State) []Step {
	steps := []Step{}
	if st.Phase != PhaseFight {
		return steps
	}
	_, mp := s.Points(st)
	start := st.Active().Cell
	seen := map[int]bool{start: true}
	frontier := []Step{{Cell: start}}
	for len(frontier) > 0 {
		cur := frontier[0]
		frontier = frontier[1:]
		if cur.Cost >= mp {
			continue
		}
		x, y := st.Board.xy(cur.Cell)
		for _, d := range [][2]int{{1, 0}, {0, 1}, {-1, 0}, {0, -1}} {
			if !st.Board.inside(x+d[0], y+d[1]) {
				continue
			}
			next := st.Board.cell(x+d[0], y+d[1])
			if seen[next] || !st.free(next) {
				continue
			}
			seen[next] = true
			step := Step{Cell: next, Cost: cur.Cost + 1, From: cur.Cell}
			steps = append(steps, step)
			frontier = append(frontier, step)
		}
	}
	return steps
}

// Move walks the player to a cell within reach.
func (s *Sim) Move(st *State, cell int) ([]Event, string) {
	if st.Phase != PhaseFight {
		return nil, BlockedPhase
	}
	steps := s.Reach(st)
	byCell := map[int]Step{}
	for _, step := range steps {
		byCell[step.Cell] = step
	}
	step, ok := byCell[cell]
	if !ok {
		return nil, BlockedMP
	}
	player := st.Active()
	path := []int{cell}
	for at := step; at.From != player.Cell; {
		at = byCell[at.From]
		path = append([]int{at.Cell}, path...)
	}
	path = append([]int{player.Cell}, path...)
	c := s.casting(0, false, false)
	id := player.ID
	for i, next := range path[1:] {
		entity := st.entity(id)
		_, remaining := s.Points(st)
		if entity == nil || entity.HP <= 0 || remaining <= 0 || entity.Cell != path[i] || !st.free(next) {
			break
		}
		entity.Cell = next
		st.MPUsed++
		last := len(c.events) - 1
		if last >= 0 && c.events[last].Type == EventMove && c.events[last].Entity == id && c.events[last].Value > 0 {
			c.events[last].Path = append(c.events[last].Path, next)
			c.events[last].Value++
		} else {
			c.emit(Event{Type: EventMove, Entity: id, Path: []int{path[i], next}, Value: 1})
		}
		if s.crossMarks(st, c, id, path[i], 0) {
			break
		}
	}
	s.settle(st, c)
	return c.events, ""
}

// EndTurn ends the player's turn. The dummies do nothing on theirs, so the
// next one starts at once: what waits for the end or the start of a turn
// applies, points are restored and lasting effects are one turn shorter.
func (s *Sim) EndTurn(st *State) ([]Event, string) {
	if st.Phase != PhaseFight {
		return nil, BlockedPhase
	}
	c := s.casting(0, false, false)
	current := st.Active()
	currentID := current.ID
	s.fire(st, c, currentID, currentID, 0, "TE")
	s.markTurn(st, c, currentID, false)
	for i := range st.Entities {
		next := &st.Entities[i]
		if !s.playable(st, next) || (current.Kind == KindSummon && next.ID <= currentID) {
			continue
		}
		st.ActiveID = next.ID
		st.APUsed, st.MPUsed = 0, 0
		s.ageMarks(st, c, next.ID)
		s.markTurn(st, c, next.ID, true)
		s.fire(st, c, next.ID, next.ID, 0, "TB")
		s.settle(st, c)
		return c.events, ""
	}
	player := st.Player().ID
	st.ActiveID = 0
	for i := range st.Entities {
		e := st.Entities[i]
		if e.ID != player && !s.playable(st, &e) {
			s.ageMarks(st, c, e.ID)
			s.markTurn(st, c, e.ID, true)
			s.fire(st, c, e.ID, e.ID, 0, "TB")
			s.fire(st, c, e.ID, e.ID, 0, "TE")
			s.markTurn(st, c, e.ID, false)
		}
		for _, use := range e.Uses {
			use.Turn = 0
			use.Targets = map[int]int{}
		}
	}
	st.Turn++
	st.APUsed, st.MPUsed, st.TurnDamage = 0, 0, 0
	for _, use := range st.Uses {
		use.Turn, use.Targets = 0, map[int]int{}
	}
	at := place{caster: player, source: player}
	for i := range st.Entities {
		e := &st.Entities[i]
		for j := range e.Buffs {
			if e.Buffs[j].Turns > 0 {
				e.Buffs[j].Turns--
			}
		}
		hooks := e.Hooks[:0]
		for _, h := range e.Hooks {
			if h.Delay == 0 && h.Turns > 0 {
				if h.Turns--; h.Turns == 0 {
					continue
				}
			}
			hooks = append(hooks, h)
		}
		e.Hooks = hooks
		// An effect that ran out is dropped the way a spell would remove it,
		// so that what waits for the end of a state hears of it.
		s.drop(st, c, at, &st.Entities[i], func(b Buff) bool { return b.Turns == 0 })
	}
	c.events = append(c.events, Event{Type: EventTurn, Value: st.Turn})
	s.ageMarks(st, c, player)
	for i := range st.Entities {
		s.due(st, c, st.Entities[i].ID)
	}
	s.fire(st, c, player, player, 0, "TB")
	s.markTurn(st, c, player, true)
	s.settle(st, c)
	return c.events, ""
}

// Cast casts the weapon or a spell at a cell.
func (s *Sim) Cast(st *State, spellID, cell int) ([]Event, string) {
	a := s.spell(st, spellID)
	if a == nil {
		return nil, BlockedUnknown
	}
	player := st.Active()
	if reason := s.blocked(st, a); reason != "" {
		return nil, reason
	}
	if reason := s.aimable(st, a, player, cell); reason != "" {
		return nil, reason
	}
	crit := s.Rand.IntN(100) < s.critRate(a, s.StatOf(player))
	c := s.casting(spellID, crit, false)
	c.events = append(c.events, Event{Type: EventCast, Entity: player.ID, Spell: spellID, Critical: crit, Cell: &cell})
	s.spend(st, a, cell)
	id := player.ID
	s.resolve(st, c, id, id, a, cell, 0)
	if crit {
		s.fire(st, c, id, id, 0, "CC")
	}
	s.settle(st, c)
	return c.events, ""
}

// spend pays for a cast and counts it.
func (s *Sim) spend(st *State, a *attack, cell int) {
	st.APUsed += a.cost
	use := st.counters()[a.id]
	if use == nil {
		use = &Use{Targets: map[int]int{}}
		st.counters()[a.id] = use
	}
	if use.Targets == nil {
		use.Targets = map[int]int{}
	}
	use.Turn++
	use.Last = st.Turn
	if occupant := st.at(cell); occupant != nil {
		use.Targets[occupant.ID]++
	}
}

// settle closes a cast: it counts what the dummies lost, and stands a
// defeated dummy back up, as a training dummy does.
func (s *Sim) settle(st *State, c *casting) {
	for _, ev := range c.events {
		if ev.Type == EventTurn {
			st.TurnDamage = 0
		}
		if ev.Type != EventDamage {
			continue
		}
		if e := st.entity(ev.Entity); e != nil && e.Kind == KindDummy && !e.Ally {
			st.TurnDamage += ev.Value
			st.TotalDamage += ev.Value
		}
	}
	for i := range st.Entities {
		e := &st.Entities[i]
		if e.HP > 0 {
			continue
		}
		if e.Kind == KindDummy {
			c.events = append(c.events, Event{Type: EventDefeat, Entity: e.ID})
			if e.InitialHP > 0 {
				e.MaxHP = e.InitialHP
			}
			e.HP, e.Buffs, e.Hooks = e.MaxHP, []Buff{}, nil
		} else {
			e.HP = 0
		}
	}
	s.removeDefeatedSummons(st, c)
	for _, mark := range append([]Mark(nil), st.Marks...) {
		if st.entity(mark.Caster) == nil {
			s.removeMark(st, c, mark, 0)
		}
	}
}
