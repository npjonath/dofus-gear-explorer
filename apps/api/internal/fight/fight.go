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
	BuffSuffered = "suffered"
	BuffHealed   = "healed"
)

// Buff is a lasting effect on an entity.
type Buff struct {
	Kind string `json:"kind"`
	// Spell is the spell that applied it, which may be one another spell
	// cast in turn, and From the spell the player cast.
	Spell int `json:"spell"`
	From  int `json:"from"`
	Ref   int `json:"ref"`
	Value int `json:"value"`
	// Turns is how many turns it still lasts, the current one included; -1
	// never ends.
	Turns int `json:"turns"`
}

// Entity is a fighter on the board.
type Entity struct {
	ID    int    `json:"id"`
	Kind  string `json:"kind"`
	Cell  int    `json:"cell"`
	HP    int    `json:"hp"`
	MaxHP int    `json:"maxHp"`
	// Stats holds a dummy's own characteristics: its resistances. The
	// player's come from the stuff.
	Stats map[int]int `json:"stats,omitempty"`
	Buffs []Buff      `json:"buffs"`
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
	Board    Board    `json:"board"`
	Phase    string   `json:"phase"`
	Turn     int      `json:"turn"`
	Entities []Entity `json:"entities"`
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
	Count int `json:"count"`
	HP    int `json:"hp"`
	// Stats are their characteristics, by the game's ids: the resistances
	// in percent and fixed, per element and to critical hits.
	Stats map[int]int `json:"stats"`
}

// Sim plays a fight for one stuff.
type Sim struct {
	Cat *game.Catalog
	// Stat gives the stuff's final characteristics.
	Stat  func(statID int) int
	Level int
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
	out.Board.Walls = append([]int(nil), st.Board.Walls...)
	out.Entities = make([]Entity, len(st.Entities))
	for i, e := range st.Entities {
		e.Buffs = append([]Buff(nil), e.Buffs...)
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
	board := DefaultBoard()
	st := &State{Board: board, Phase: PhasePlacement, Uses: map[int]*Use{}}
	health := max(1, s.Stat(game.StatHP))
	mid := board.Height / 2
	st.Entities = append(st.Entities, Entity{ID: 1, Kind: KindPlayer, Cell: board.cell(3, mid), HP: health, MaxHP: health, Buffs: []Buff{}})
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
		st.Entities = append(st.Entities, Entity{
			ID: i + 2, Kind: KindDummy, Cell: board.cell(spots[i][0], spots[i][1]), HP: hp, MaxHP: hp, Stats: stats, Buffs: []Buff{},
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
	stat := s.StatOf(st.Player())
	return stat(game.StatAP) - st.APUsed, stat(game.StatMP) - st.MPUsed
}

// attack is a weapon or a spell grade, as the simulator casts it.
type attack struct {
	id          int
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
		id: id, cost: level.APCost, rangeMin: level.RangeMin, rangeMax: level.RangeMax, crit: level.CritChance,
		rules: level.Cast, effects: level.Effects, critEffects: level.CritEffects,
	}
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
	if use := st.Uses[a.id]; use != nil && use.Last > 0 && a.rules.Interval > 0 {
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
	if use := st.Uses[a.id]; use != nil && a.rules.PerTurn > 0 && use.Turn >= a.rules.PerTurn {
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
		if st.Board.wall(c) || st.at(c) != nil {
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
	if use := st.Uses[a.id]; use != nil && occupant != nil && a.rules.PerTarget > 0 && use.Targets[occupant.ID] >= a.rules.PerTarget {
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
	if !st.free(cell) {
		return BlockedCell
	}
	e.Cell = cell
	return ""
}

// Begin starts the first turn.
func (s *Sim) Begin(st *State) ([]Event, string) {
	if st.Phase != PhasePlacement {
		return nil, BlockedPhase
	}
	st.Phase, st.Turn = PhaseFight, 1
	return []Event{{Type: EventTurn, Value: 1}}, ""
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
	start := st.Player().Cell
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
	player := st.Player()
	path := []int{cell}
	for at := step; at.From != player.Cell; {
		at = byCell[at.From]
		path = append([]int{at.Cell}, path...)
	}
	path = append([]int{player.Cell}, path...)
	player.Cell = cell
	st.MPUsed += step.Cost
	return []Event{{Type: EventMove, Entity: player.ID, Path: path, Value: step.Cost}}, ""
}

// EndTurn ends the player's turn: the dummies do nothing, so the next one
// starts at once, points restored and lasting effects one turn shorter.
func (s *Sim) EndTurn(st *State) ([]Event, string) {
	if st.Phase != PhaseFight {
		return nil, BlockedPhase
	}
	st.Turn++
	st.APUsed, st.MPUsed, st.TurnDamage = 0, 0, 0
	for _, use := range st.Uses {
		use.Turn, use.Targets = 0, map[int]int{}
	}
	for i := range st.Entities {
		e := &st.Entities[i]
		kept := e.Buffs[:0]
		for _, b := range e.Buffs {
			if b.Turns > 0 {
				b.Turns--
				if b.Turns == 0 {
					continue
				}
			}
			kept = append(kept, b)
		}
		e.Buffs = kept
	}
	return []Event{{Type: EventTurn, Value: st.Turn}}, ""
}

// Cast casts the weapon or a spell at a cell.
func (s *Sim) Cast(st *State, spellID, cell int) ([]Event, string) {
	a := s.attack(spellID, 0)
	if a == nil {
		return nil, BlockedUnknown
	}
	player := st.Player()
	if reason := s.blocked(st, a); reason != "" {
		return nil, reason
	}
	if reason := s.aimable(st, a, player, cell); reason != "" {
		return nil, reason
	}
	crit := s.Rand.IntN(100) < s.critRate(a, s.StatOf(player))
	c := &casting{root: spellID, crit: crit, random: true, roll: func(lo, hi int) int { return lo + s.Rand.IntN(hi-lo+1) }}
	c.events = append(c.events, Event{Type: EventCast, Entity: player.ID, Spell: spellID, Critical: crit, Cell: &cell})
	s.spend(st, a, cell)
	s.resolve(st, c, player.ID, a, cell, 0)
	s.settle(st, c)
	return c.events, ""
}

// spend pays for a cast and counts it.
func (s *Sim) spend(st *State, a *attack, cell int) {
	st.APUsed += a.cost
	use := st.Uses[a.id]
	if use == nil {
		use = &Use{Targets: map[int]int{}}
		st.Uses[a.id] = use
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
		if ev.Type != EventDamage {
			continue
		}
		if e := st.entity(ev.Entity); e != nil && e.Kind == KindDummy {
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
			e.HP, e.Buffs = e.MaxHP, []Buff{}
		} else {
			e.HP = 0
		}
	}
}
