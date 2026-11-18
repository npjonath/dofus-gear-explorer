package fight

import (
	"strconv"
	"strings"

	"dofusdb/api/internal/engine"
	"dofusdb/api/internal/game"
)

// Kinds of event a fight reports, in the order things happen.
const (
	EventTurn   = "turn"
	EventCast   = "cast"
	EventMove   = "move"
	EventDamage = "damage"
	EventHeal   = "heal"
	EventBuff   = "buff"
	EventAP     = "ap"
	EventDefeat = "defeat"
	// EventSkipped is an effect the spell lists that the simulator does not
	// apply: the fight goes on without it.
	EventSkipped = "skipped"
)

// Event is one thing that happened in a fight.
type Event struct {
	Type   string `json:"type"`
	Entity int    `json:"entity,omitempty"`
	// Spell is the spell cast, or the one a lasting effect comes from.
	Spell int `json:"spell,omitempty"`
	// Value is the turn started, the points lost or gained, or the amount
	// of the lasting effect.
	Value    int  `json:"value,omitempty"`
	Element  int  `json:"element,omitempty"`
	Critical bool `json:"critical,omitempty"`
	// Push says the damage comes from a pushback.
	Push bool `json:"push,omitempty"`
	// Stat is the characteristic a lasting effect changes and Turns how
	// long it lasts, -1 for ever.
	Stat  *int `json:"stat,omitempty"`
	Turns int  `json:"turns,omitempty"`
	// Cell is where a spell was aimed, and Path the cells an entity went
	// through, from where it stood.
	Cell *int  `json:"cell,omitempty"`
	Path []int `json:"path,omitempty"`
	// Action is the game's number of a skipped effect.
	Action int `json:"action,omitempty"`
}

// casting is one cast being resolved, the spells it casts in turn included.
type casting struct {
	// root is the spell the player cast: lasting effects are filed under it.
	root int
	crit bool
	// random says the effects drawn at random are drawn; a preview leaves
	// them out.
	random bool
	roll   func(lo, hi int) int
	events []Event
}

func (c *casting) skip(e game.SpellEffect) {
	if !e.Shown {
		return
	}
	for _, ev := range c.events {
		if ev.Type == EventSkipped && ev.Action == e.Action {
			return
		}
	}
	c.events = append(c.events, Event{Type: EventSkipped, Spell: c.root, Action: e.Action})
}

// Actions the simulator applies besides damage, steals and the effects that
// change a characteristic.
const (
	actionTeleport      = 4
	actionPush          = 5
	actionPull          = 6
	actionSwap          = 8
	actionRefundAP      = 120
	actionSpellRange    = 281
	actionSpellBase     = 293
	actionRemoveSpell   = 406
	actionState         = 950
	actionRemoveState   = 951
	actionRecoil        = 1041
	actionAdvance       = 1042
	actionPushHarmless  = 1103
	actionHealPercent   = 1109
	actionHealsReceived = 1159
	actionDamageTaken   = 1163
)

// healActions maps the heals that scale with a characteristic to their element.
var healActions = map[int]int{
	108: game.ElementFire, 2998: game.ElementWater, 2999: game.ElementAir, 3000: game.ElementEarth,
}

// maxDepth bounds the spells cast in turn by one cast.
const maxDepth = 8

// mask is who an effect applies to, read from the game's target mask.
type mask struct {
	anyone      bool
	ally        bool
	allyOther   bool
	enemy       bool
	caster      bool
	casterToo   bool
	needs       []int
	refuses     []int
	casterNeeds []int
	casterLacks []int
	// never says a condition can never hold in a fight against dummies, and
	// unknown that the mask carries a condition the simulator cannot read.
	never   bool
	unknown bool
}

func readMask(raw string) mask {
	var m mask
	named := false
	for _, token := range strings.Split(raw, ",") {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		switch token {
		case "a", "h", "l":
			m.ally, named = true, true
			continue
		case "g":
			m.allyOther, named = true, true
			continue
		case "A", "H", "L", "M":
			m.enemy, named = true, true
			continue
		case "C":
			m.caster, named = true, true
			continue
		case "c":
			m.casterToo, named = true, true
			continue
		case "m", "d", "D", "j", "J", "i", "I", "s", "S":
			// Summons, companions and monsters on the caster's side: a fight
			// against dummies has none.
			named = true
			continue
		}
		onCaster := strings.HasPrefix(token, "*")
		body := strings.TrimPrefix(token, "*")
		id, err := strconv.Atoi(body[1:])
		if err != nil {
			m.unknown = true
			continue
		}
		switch {
		case body[0] == 'E' && onCaster:
			m.casterNeeds = append(m.casterNeeds, id)
		case body[0] == 'e' && onCaster:
			m.casterLacks = append(m.casterLacks, id)
		case body[0] == 'E':
			m.needs = append(m.needs, id)
		case body[0] == 'e':
			m.refuses = append(m.refuses, id)
		case body[0] == 'F' && !onCaster:
			// Only a given monster: neither the player nor a dummy.
			m.never = true
		case body[0] == 'f' && !onCaster:
			// Anyone but a given monster.
		default:
			m.unknown = true
		}
	}
	m.anyone = !named
	return m
}

func (e *Entity) inState(id int) bool {
	for _, b := range e.Buffs {
		if b.Kind == BuffState && b.Ref == id {
			return true
		}
	}
	return false
}

func (m mask) accepts(caster, target *Entity) bool {
	if m.never {
		return false
	}
	for _, id := range m.casterNeeds {
		if !caster.inState(id) {
			return false
		}
	}
	for _, id := range m.casterLacks {
		if caster.inState(id) {
			return false
		}
	}
	for _, id := range m.needs {
		if !target.inState(id) {
			return false
		}
	}
	for _, id := range m.refuses {
		if target.inState(id) {
			return false
		}
	}
	switch {
	case m.anyone:
		return true
	case target.ID == caster.ID:
		return m.ally || m.caster || m.casterToo
	case target.Kind == caster.Kind:
		return m.ally || m.allyOther
	}
	return m.enemy
}

// targets lists who an effect applies to when the spell is aimed at a cell:
// those of the zone the mask accepts, nearest first, and the caster wherever
// it stands when the mask names it.
func (s *Sim) targets(st *State, caster *Entity, e game.SpellEffect, m mask, cell int) []int {
	var ids []int
	for _, c := range st.Board.zone(e.Zone, caster.Cell, cell) {
		if target := st.at(c); target != nil && m.accepts(caster, target) {
			ids = append(ids, target.ID)
		}
	}
	if m.caster && m.accepts(caster, caster) {
		for _, id := range ids {
			if id == caster.ID {
				return ids
			}
		}
		ids = append(ids, caster.ID)
	}
	return ids
}

func isHit(e game.SpellEffect) bool {
	_, _, ok := game.AttackAction(e.Action)
	return ok
}

// resolve applies the effects of an attack aimed at a cell, in order.
func (s *Sim) resolve(st *State, c *casting, casterID int, a *attack, cell, depth int) {
	effects := a.effects
	if c.crit && len(a.critEffects) > 0 {
		effects = a.critEffects
	}
	// A hit the spell only describes is dealt by something it sets up; when
	// every hit is described only, they are the spell's own.
	real := false
	for _, e := range effects {
		real = real || (isHit(e) && !e.Described && e.Delay == 0 && e.Trigger == "" && e.Duration == 0)
	}
	drawn := s.draw(c, effects)
	for i, e := range effects {
		if e.Random > 0 && !drawn[i] {
			continue
		}
		if e.Delay != 0 || e.Trigger != "" {
			c.skip(e)
			continue
		}
		if e.Described && (real || !isHit(e)) {
			continue
		}
		caster := st.entity(casterID)
		m := readMask(e.Targets)
		if m.unknown {
			c.skip(e)
			continue
		}
		for _, id := range s.targets(st, caster, e, m, cell) {
			s.apply(st, c, casterID, id, a, e, cell, depth)
		}
	}
}

// draw picks, among the effects that apply at random, the one of each group
// that does: its weight is its chance in percent.
func (s *Sim) draw(c *casting, effects []game.SpellEffect) map[int]bool {
	drawn := map[int]bool{}
	if !c.random {
		return drawn
	}
	groups := map[int][]int{}
	var order []int
	for i, e := range effects {
		if e.Random <= 0 {
			continue
		}
		if _, known := groups[e.Group]; !known {
			order = append(order, e.Group)
		}
		groups[e.Group] = append(groups[e.Group], i)
	}
	for _, group := range order {
		total := 0.0
		for _, i := range groups[group] {
			total += effects[i].Random
		}
		at := s.Rand.Float64() * max(total, 100)
		for _, i := range groups[group] {
			at -= effects[i].Random
			if at < 0 {
				drawn[i] = true
				break
			}
		}
	}
	return drawn
}

func span(e game.SpellEffect) (lo, hi int) {
	lo, hi = e.Min, e.Max
	if hi < lo {
		hi = lo
	}
	return lo, hi
}

// apply applies one effect to one target.
func (s *Sim) apply(st *State, c *casting, casterID, targetID int, a *attack, e game.SpellEffect, cell, depth int) {
	caster, target := st.entity(casterID), st.entity(targetID)
	if caster == nil || target == nil {
		return
	}
	lo, hi := span(e)
	if element, steal, ok := game.AttackAction(e.Action); ok {
		if e.Duration != 0 {
			// Damage over several turns is a poison.
			c.skip(e)
			return
		}
		s.hit(st, c, caster, target, a, e, element, steal, cell)
		return
	}
	if element, ok := healActions[e.Action]; ok {
		base := c.roll(lo, hi)
		spec := engine.AttackSpec{Level: s.Level}
		line := game.AttackLine{Kind: game.LineHeal, Element: element, Min: base, Max: base}
		amount, _ := spec.Line(line, s.StatOf(caster))
		s.heal(c, target, amount.Min)
		return
	}
	switch e.Action {
	case actionHealPercent:
		s.heal(c, target, target.MaxHP*c.roll(lo, hi)/100)
	case actionPush, actionPushHarmless:
		s.shove(st, c, caster, target, origin(caster, target, cell), e.Min, true, e.Action == actionPush)
	case actionPull:
		s.shove(st, c, caster, target, origin(caster, target, cell), e.Min, false, false)
	case actionRecoil:
		s.shove(st, c, caster, caster, target.Cell, e.Min, true, false)
	case actionAdvance:
		s.shove(st, c, caster, caster, target.Cell, e.Min, false, false)
	case actionTeleport:
		if st.free(cell) {
			c.events = append(c.events, Event{Type: EventMove, Entity: caster.ID, Path: []int{caster.Cell, cell}})
			caster.Cell = cell
		}
	case actionSwap:
		if target.ID != caster.ID {
			c.events = append(c.events,
				Event{Type: EventMove, Entity: caster.ID, Path: []int{caster.Cell, target.Cell}},
				Event{Type: EventMove, Entity: target.ID, Path: []int{target.Cell, caster.Cell}})
			caster.Cell, target.Cell = target.Cell, caster.Cell
		}
	case actionRefundAP:
		if target.Kind == KindPlayer {
			gain := c.roll(lo, hi)
			st.APUsed -= gain
			c.events = append(c.events, Event{Type: EventAP, Entity: target.ID, Value: gain})
		}
	case actionState:
		s.buff(c, target, a, Buff{Kind: BuffState, Ref: e.Value, Turns: e.Duration}, false)
	case actionRemoveState:
		target.drop(func(b Buff) bool { return b.Kind == BuffState && b.Ref == e.Value })
	case actionRemoveSpell:
		target.drop(func(b Buff) bool { return b.Spell == e.Value })
	case actionSpellBase:
		s.buff(c, target, a, Buff{Kind: BuffBase, Ref: e.Min, Value: e.Value, Turns: e.Duration}, false)
	case actionSpellRange:
		s.buff(c, target, a, Buff{Kind: BuffRange, Ref: e.Min, Value: e.Value, Turns: e.Duration}, false)
	case actionDamageTaken:
		s.buff(c, target, a, Buff{Kind: BuffSuffered, Value: e.Min, Turns: e.Duration}, false)
	case actionHealsReceived:
		s.buff(c, target, a, Buff{Kind: BuffHealed, Value: e.Min, Turns: e.Duration}, false)
	case game.ActionCastSpell, game.ActionTriggerSpell:
		sub := s.attack(e.Min, e.Max)
		if sub == nil || depth >= maxDepth {
			return
		}
		if e.Action == game.ActionTriggerSpell {
			s.resolve(st, c, target.ID, sub, target.Cell, depth+1)
		} else {
			s.resolve(st, c, caster.ID, sub, target.Cell, depth+1)
		}
	default:
		if e.Sign == 0 {
			c.skip(e)
			return
		}
		value := c.roll(lo, hi) * e.Sign
		turns := e.Duration
		if turns == 0 {
			// Points given or taken at once count for the turn in progress.
			if e.Stat != game.StatAP && e.Stat != game.StatMP {
				c.skip(e)
				return
			}
			turns = 1
		}
		s.buff(c, target, a, Buff{Kind: BuffStat, Ref: e.Stat, Value: value, Turns: turns}, true)
	}
}

// origin is the cell a push moves a target away from, and a pull toward: the
// caster for the target a spell is aimed at, the aimed cell for the others.
func origin(caster, target *Entity, cell int) int {
	if target.Cell == cell {
		return caster.Cell
	}
	return cell
}

func (e *Entity) drop(match func(Buff) bool) {
	kept := e.Buffs[:0]
	for _, b := range e.Buffs {
		if !match(b) {
			kept = append(kept, b)
		}
	}
	e.Buffs = kept
}

// buff puts a lasting effect on a target, unless the spell's effects have
// piled up on it as much as they can.
func (s *Sim) buff(c *casting, target *Entity, a *attack, b Buff, report bool) {
	b.Spell, b.From = a.id, c.root
	if b.Turns == 0 {
		b.Turns = 1
	}
	if a.rules.MaxStack > 0 {
		count := 0
		for _, other := range target.Buffs {
			if other.Kind == b.Kind && other.Spell == b.Spell && other.Ref == b.Ref {
				count++
			}
		}
		if count >= a.rules.MaxStack {
			return
		}
	}
	target.Buffs = append(target.Buffs, b)
	if report {
		stat := b.Ref
		c.events = append(c.events, Event{Type: EventBuff, Entity: target.ID, Spell: c.root, Stat: &stat, Value: b.Value, Turns: b.Turns})
	}
}

func (s *Sim) heal(c *casting, target *Entity, amount int) {
	for _, b := range target.Buffs {
		if b.Kind == BuffHealed {
			amount = amount * b.Value / 100
		}
	}
	amount = max(0, min(amount, target.MaxHP-target.HP))
	target.HP += amount
	c.events = append(c.events, Event{Type: EventHeal, Entity: target.ID, Value: amount})
}

// hit deals one damage line to a target.
func (s *Sim) hit(st *State, c *casting, caster, target *Entity, a *attack, e game.SpellEffect, element int, steal bool, cell int) {
	lo, hi := span(e)
	base := c.roll(lo, hi)
	for _, b := range caster.Buffs {
		if b.Kind == BuffBase && b.Ref == a.id && !a.weapon {
			base += b.Value
		}
	}
	stat := s.StatOf(caster)
	spec := engine.AttackSpec{Weapon: a.weapon, Melee: st.Board.Distance(caster.Cell, target.Cell) <= 1, Level: s.Level}
	line := game.AttackLine{Element: element, Min: base, Max: base, CritMin: base, CritMax: base}
	rolled, element := spec.Line(line, stat)
	damage := rolled.Min
	if c.crit {
		damage = rolled.CritMin
	}
	damage = damage * max(0, 100+stat(statFinalDamage)) / 100
	// Away from the centre of a zone, a hit loses a share of its damage per
	// cell.
	if e.Zone.Shape != "" && e.Zone.Shape != "P" {
		steps := min(st.Board.Distance(cell, target.Cell), e.Zone.Steps)
		damage = damage * max(0, 100-e.Zone.Decrease*steps) / 100
	}
	for _, b := range target.Buffs {
		if b.Kind == BuffSuffered {
			damage = damage * b.Value / 100
		}
	}
	// The target's fixed resistance comes off first, then its percentage.
	resist := s.StatOf(target)
	fixed := resist(fixedResist[element])
	if c.crit {
		fixed += resist(statCriticalResis)
	}
	damage = max(0, damage-fixed) * max(0, 100-resist(percentResist[element])) / 100
	target.HP -= damage
	c.events = append(c.events, Event{Type: EventDamage, Entity: target.ID, Value: damage, Element: element, Critical: c.crit})
	if steal && caster.ID != target.ID {
		s.heal(c, caster, damage/2)
	}
}

// shove moves an entity in a straight line, away from a cell or toward it,
// until something stands in the way. A push stopped short deals pushback
// damage for the cells left, and half of it to whoever blocked the way.
func (s *Sim) shove(st *State, c *casting, caster, moved *Entity, from, cells int, away, harmful bool) {
	if from == moved.Cell || cells <= 0 {
		return
	}
	dx, dy := st.Board.heading(from, moved.Cell)
	if !away {
		dx, dy = -dx, -dy
	}
	path := []int{moved.Cell}
	var blocker *Entity
	left := cells
	for ; left > 0; left-- {
		x, y := st.Board.xy(moved.Cell)
		if !st.Board.inside(x+dx, y+dy) {
			break
		}
		next := st.Board.cell(x+dx, y+dy)
		if !st.free(next) {
			blocker = st.at(next)
			break
		}
		moved.Cell = next
		path = append(path, next)
	}
	if len(path) > 1 {
		c.events = append(c.events, Event{Type: EventMove, Entity: moved.ID, Path: path})
	}
	if !harmful || left == 0 {
		return
	}
	damage := max(0, (s.Level/2+32+s.StatOf(caster)(game.StatPushDamage))*left/4)
	for i, hurt := range []*Entity{moved, blocker} {
		if hurt == nil {
			continue
		}
		taken := max(0, damage/(i+1)-s.StatOf(hurt)(statPushResist))
		hurt.HP -= taken
		c.events = append(c.events, Event{Type: EventDamage, Entity: hurt.ID, Value: taken, Push: true})
	}
}
