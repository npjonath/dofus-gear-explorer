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
	PortalPath []int  `json:"portalPath,omitempty"`
	Carrier    int    `json:"carrier,omitempty"`
	Type       string `json:"type"`
	Entity     int    `json:"entity,omitempty"`
	// Spell is the spell cast, or the one a lasting effect comes from.
	Spell        int   `json:"spell"`
	TriggerSpell int   `json:"triggerSpell,omitempty"`
	CasterStates []int `json:"casterStates,omitempty"`
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

// casting is one cast being resolved, the spells it casts in turn and the
// waiting effects it sets off included.
type casting struct {
	triggeringDamage int // Damage from the hit currently firing reactive effects.
	finishTurn       bool
	acting           int // Fighter taking the action; zero for delayed effects and turns.
	projection       portalProjection
	telefrag         map[int]bool
	// root is the spell the player cast: lasting effects are filed under it.
	root       int
	aura       int
	markAction int
	lastMarkID int
	crit       bool
	// pick chooses, among the outcomes of a spell that are drawn at random,
	// the one that happens: its rank, or a negative number for none. A
	// preview goes through them one by one.
	pick func(weights []float64) int
	roll func(lo, hi int) int
	// quiet keeps the effects that are not simulated out of the events: the
	// spells a fight starts with hold many the player never asked about.
	quiet  bool
	fires  int
	events []Event
	// work is what is left of the effects this cast, or the preview it is
	// part of, may still apply: a fight sent with spells waiting on spells
	// cannot make one request run without end.
	work *int
}

// spent counts one effect applied and reports whether the budget is gone.
func (c *casting) spent() bool {
	if c.work == nil {
		return false
	}
	*c.work--
	return *c.work < 0
}

func (c *casting) emit(ev Event) {
	c.events = append(c.events, ev)
}

func (c *casting) skip(e game.SpellEffect) {
	if !e.Shown || c.quiet {
		return
	}
	for _, ev := range c.events {
		if ev.Type == EventSkipped && ev.Action == e.Action {
			return
		}
	}
	c.events = append(c.events, Event{Type: EventSkipped, Spell: c.root, Action: e.Action})
}

// Actions the simulator applies besides damage, steals, the effects that
// change a characteristic and those that cast another spell.
const (
	actionTeleport      = 4
	actionPush          = 5
	actionPull          = 6
	actionSwap          = 8
	actionRefundAP      = 120
	actionSpellRange    = 281
	actionSpellBase     = 293
	actionRemoveSpell   = 406
	actionNothing       = 666
	actionState         = 950
	actionRemoveState   = 951
	actionRecoil        = 1041
	actionAdvance       = 1042
	actionPushHarmless  = 1103
	actionHealPercent   = 1109
	actionHealsReceived = 1159
	actionDamageTaken   = 1163
)

// Actions that change how one spell is cast: the spell is the effect's Min
// and the amount its Value.
const (
	modMinRange      = 280
	modRangeBoosted  = 282
	modDamage        = 283
	modCostLess      = 285
	modCooldownLess  = 286
	modCritical      = 287
	modNoLine        = 288
	modNoSight       = 289
	modPerTurn       = 290
	modPerTarget     = 291
	modCooldownSet   = 292
	modMaxRangeLess  = 294
	modMinRangeLess  = 295
	modCostMore      = 296
	modTakenCellOff  = 297
	modFreeCellOff   = 298
	modFreeCellOn    = 299
	modTakenCellOn   = 314
	modMaxRangeSet   = 2905
	modMinRangeSet   = 2906
	firstSpellMod    = 280
	lastSpellMod     = 299
	statSpellPower   = 98
	statWeaponPower  = 103
	statResistGlobal = 101
)

func isSpellMod(action int) bool {
	if action == actionSpellBase || action == actionSpellRange {
		return false
	}
	return (action >= firstSpellMod && action <= lastSpellMod) || (action >= 2908 && action <= 2911) || action == modTakenCellOn || action == modMaxRangeSet || action == modMinRangeSet
}

// healActions maps the heals that scale with a characteristic to their element.
var healActions = map[int]int{
	108: game.ElementFire, 2998: game.ElementWater, 2999: game.ElementAir, 3000: game.ElementEarth,
}

// Bounds on what one cast can set off: the spells cast in turn, the waiting
// effects fired, and the effects applied in all, for one action and for the
// preview of a spell over every cell.
const (
	maxDepth   = 8
	maxFires   = 400
	maxWork    = 20_000
	maxAimWork = 400_000
)

// mask is who an effect applies to, read from the game's target mask: who
// it names (allies, enemies, the caster), then conditions each of which
// must hold.
type mask struct {
	projected, direct         bool
	telefrag, withoutTelefrag bool
	allySummon, enemySummon   bool
	allyPlayer, ownerOnly     bool
	anyone                    bool
	ally                      bool
	allyOther                 bool
	enemy                     bool
	caster                    bool
	casterToo                 bool
	conds                     []cond
}

// cond is one condition of a mask, on the target or, starred, on the caster.
// Its letter says what is asked; lower case asks for the opposite.
type cond struct {
	letter   byte
	arg      int
	onCaster bool
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
		case "R":
			m.projected = true
			continue
		case "r":
			m.direct = true
			continue
		case "h", "l":
			m.allyPlayer, named = true, true
			continue
		case "T":
			m.telefrag = true
			continue
		case "t":
			m.withoutTelefrag = true
			continue
		case "P":
			m.ownerOnly = true
			continue
		case "a":
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
		case "j", "i", "s":
			m.allySummon, named = true, true
			continue
		case "J", "I", "S":
			m.enemySummon, named = true, true
			continue
		case "m", "d", "D":
			// Summons, companions and monsters on the caster's side: a fight
			// against dummies has none.
			named = true
			continue
		}
		onCaster := strings.HasPrefix(token, "*")
		body := strings.TrimPrefix(token, "*")
		if body == "" {
			continue
		}
		arg, _ := strconv.Atoi(body[1:])
		m.conds = append(m.conds, cond{letter: body[0], arg: arg, onCaster: onCaster})
	}
	m.anyone = !named
	return m
}

// holds reports whether a condition is met by an entity. Besides states,
// health and class, the conditions ask whether the entity is a given
// monster, a summon, carried, and the like: never so for the player or a
// dummy, so the upper-case letter fails and its lower-case opposite passes.
//
// States are read as they were when the spell was cast, when the caller has
// them: a spell that puts its caster in a state and has another effect for
// casters in that state applies one of the two, not both.
func (s *Sim) holds(c cond, e *Entity, had map[int]map[int]bool) bool {
	in := e.inState(c.arg)
	if had != nil {
		in = had[e.ID][c.arg]
	}
	switch c.letter {
	case 'E':
		return in
	case 'e':
		return !in
	case 'V':
		// At or under a share of its health.
		return e.HP*100 <= c.arg*e.MaxHP
	case 'v':
		return e.HP*100 > c.arg*e.MaxHP
	case 'F':
		return e.Kind == KindSummon && e.MonsterID == c.arg
	case 'f':
		return e.Kind != KindSummon || e.MonsterID != c.arg
	case 'B':
		return e.Kind == KindPlayer && s.Breed == c.arg
	case 'b':
		return e.Kind != KindPlayer || s.Breed != c.arg
	}
	return c.letter >= 'a' && c.letter <= 'z'
}

func (e *Entity) inState(id int) bool {
	for _, b := range e.Buffs {
		if b.Kind == buffStateDisabled && b.Ref == id {
			return false
		}
	}
	for _, b := range e.Buffs {
		if b.Kind == BuffState && b.Ref == id {
			return true
		}
	}
	return false
}

// names reports whether the mask is meant for the target, whatever state
// anyone is in.
func (m mask) names(caster, target *Entity) bool {
	switch {
	case m.ownerOnly && target.ID != caster.Owner:
		return false
	case m.allyPlayer && target.Kind == KindPlayer && caster.Kind != KindDummy:
		return true
	case m.anyone:
		return true
	case target.ID == caster.ID:
		return m.ally || m.caster || m.casterToo
	case target.enemyTeam() == caster.enemyTeam():
		return m.ally || m.allyOther || m.allySummon && target.Kind == KindSummon
	}
	return m.enemy || m.enemySummon && target.Kind == KindSummon
}

// met reports whether every condition of the mask holds for the caster and
// the target as they are now.
func (s *Sim) met(m mask, caster, target *Entity, had map[int]map[int]bool) bool {
	for _, c := range m.conds {
		who := target
		if c.onCaster {
			who = caster
		}
		if !s.holds(c, who, had) {
			return false
		}
	}
	return true
}

// targets lists who an effect is meant for when the spell is aimed at a
// cell: those of the zone the mask names, nearest first, and the caster
// wherever it stands when the mask names it. Who stands in the zone is read
// from where everyone stood as the spell was cast: an effect that pushes its
// target away does not make the next one miss.
func (s *Sim) targets(st *State, caster *Entity, e game.SpellEffect, m mask, cell, origin int, stood map[int]int, had map[int]map[int]bool) []int {
	var ids []int
	for _, c := range st.Board.zone(e.Zone, origin, cell) {
		if e.Zone.Sight && !st.sees(cell, c) {
			continue
		}
		for i := range st.Entities {
			target := &st.Entities[i]
			if e.Zone.ExcludeCarried && had[target.ID][carriedState] {
				continue
			}
			if stood[target.ID] == c && m.names(caster, target) {
				ids = append(ids, target.ID)
			}
		}
	}
	if m.caster {
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

// outcomes groups the effects of a spell that are drawn at random: one
// group happens, with the chance its weights add up to, in percent. Effects
// that share a group number happen together; without one, each stands
// alone.
func outcomes(effects []game.SpellEffect) (groups [][]int, weights []float64) {
	byGroup := map[int]int{}
	for i, e := range effects {
		if e.Random <= 0 {
			continue
		}
		at, known := byGroup[e.Group]
		if !known || e.Group == 0 {
			at = len(groups)
			byGroup[e.Group] = at
			groups, weights = append(groups, nil), append(weights, 0)
		}
		groups[at] = append(groups[at], i)
		weights[at] += e.Random
	}
	return groups, weights
}

// place is where an effect is applied from.
type place struct {
	dispellable bool
	// caster casts; source is who set the effect off: the caster, or for an
	// effect that waited, whoever triggered it.
	caster, source int
	cell, depth    int
	stood          map[int]int
	// fired says the effect waited and its trigger just happened.
	fired bool
}

// resolve applies the effects of an attack aimed at a cell, in order.
func (s *Sim) resolve(st *State, c *casting, casterID, sourceID int, a *attack, cell, depth int) {
	s.resolveOn(st, c, casterID, sourceID, a, cell, depth, 0)
}

// A start/end glyph or an aura resolves for each triggering fighter. Its
// centre remains fixed, so hits still lose damage by distance from the glyph.
func (s *Sim) resolveOn(st *State, c *casting, casterID, sourceID int, a *attack, cell, depth, onlyTarget int) {
	effects, crit := a.effects, false
	if c.crit && len(a.critEffects) > 0 {
		effects, crit = a.critEffects, true
	}
	// A hit the spell only describes is dealt by something it sets up; when
	// every hit is described only, they are the spell's own.
	real := false
	for _, e := range effects {
		real = real || (isHit(e) && !e.Described && e.Delay == 0 && e.Trigger == "" && e.Duration == 0)
		// Rune detonators describe one damage line per possible element,
		// while the runes actually present supply the real hits.
		real = real || (e.Action == 2023 && !e.Described && e.Delay == 0 && e.Trigger == "")
	}
	drawn := map[int]bool{}
	if groups, weights := outcomes(effects); len(groups) > 0 {
		if at := c.pick(weights); at >= 0 && at < len(groups) {
			for _, i := range groups[at] {
				drawn[i] = true
			}
		}
	}
	// Where everyone stands and the states everyone is in are those of the
	// moment the spell is cast, for all of its effects.
	stood := make(map[int]int, len(st.Entities))
	had := make(map[int]map[int]bool, len(st.Entities))
	for _, e := range st.Entities {
		stood[e.ID] = e.Cell
		states := map[int]bool{}
		for _, b := range e.Buffs {
			if b.Kind == BuffState && e.inState(b.Ref) {
				states[b.Ref] = true
			}
		}
		had[e.ID] = states
	}
	at := place{caster: casterID, source: sourceID, cell: cell, depth: depth, stood: stood}
	for i, e := range effects {
		at.dispellable = e.Dispellable
		if e.Random > 0 && !drawn[i] {
			continue
		}
		if e.Action == actionNothing {
			continue
		}
		if c.spent() {
			return
		}
		if e.Described && e.Trigger == "" && e.Delay == 0 && (real || !isHit(e)) {
			continue
		}
		caster := st.entity(casterID)
		if caster == nil {
			return
		}
		m := readMask(e.Targets)
		if m.projected && len(c.projection.route.Cells) == 0 || m.direct && len(c.projection.route.Cells) > 0 {
			continue
		}
		if portalAction(e.Action) && e.Trigger == "" && e.Delay == 0 {
			s.applyPortal(st, c, at, a, e, had)
			continue
		}
		if e.Action == actionTeleport && st.free(cell) && e.Trigger == "" && e.Delay == 0 {
			// Teleportation targets the tile. There is no occupant to test
			// against an unstarred condition on the recipient.
			allowed := true
			for _, cond := range m.conds {
				allowed = allowed && cond.onCaster && s.holds(cond, caster, had)
			}
			if allowed {
				s.apply(st, c, at, caster.ID, a, e)
			}
			continue
		}
		if e.Action == 2023 && e.Trigger == "" && e.Delay == 0 {
			s.actOnRunes(st, c, at, e, had)
			continue
		}
		if e.Action == throwAction && !at.fired && e.Trigger == "" && e.Delay == 0 {
			s.throw(st, c, at, caster, e, had)
			continue
		}
		if e.Action == 310 && e.Trigger == "" && e.Delay == 0 {
			for _, mark := range append([]Mark(nil), st.Marks...) {
				if mark.Action != runeMark && mark.Cell == cell {
					s.removeMark(st, c, mark, depth)
				}
			}
			continue
		}
		if game.PlacesMark(e.Action) && e.Trigger == "" && e.Delay == 0 {
			s.placeMark(st, c, at, a, i, crit, e)
			continue
		}
		if game.SummonsCreature(e.Action) && e.Delay == 0 && e.Trigger == "" {
			s.summon(st, c, at, a, e)
			continue
		}
		if e.Action == game.ActionCastsOnCell {
			// Cast on the aimed cell whoever stands there, if anyone: only
			// what the mask asks of the caster counts.
			ok := true
			for _, c := range m.conds {
				ok = ok && (!c.onCaster || s.holds(c, caster, had))
			}
			if sub := s.attack(e.Min, e.Max); ok && sub != nil && depth < maxDepth {
				s.resolve(st, c, casterID, sourceID, sub, cell, depth+1)
			}
			continue
		}
		origin := stood[caster.ID]
		if depth == 0 {
			origin = c.projection.origin(origin)
		}
		for _, id := range s.targets(st, caster, e, m, cell, origin, stood, had) {
			if onlyTarget != 0 && id != onlyTarget && !(m.caster && id == casterID) {
				continue
			}
			target := st.entity(id)
			if m.telefrag && !c.telefrag[id] || m.withoutTelefrag && c.telefrag[id] {
				continue
			}
			if target == nil {
				continue
			}
			if e.Action == 265 && e.Trigger == "D" && e.Delay == 0 && s.met(m, caster, target, had) {
				amount := e.Value * (100 + 5*s.Level) / 100
				s.buff(st, c, at, target, a, Buff{Kind: BuffArmor, Value: amount, Turns: e.TriggerTurns}, false)
				c.emit(Event{Type: "armor", Entity: target.ID, Spell: c.root, Value: amount, Turns: e.TriggerTurns})
				continue
			}
			if e.Trigger != "" || e.Delay > 0 {
				s.hook(st, target, a, e, Hook{Spell: a.id, Grade: a.grade, Critical: crit, Index: i, From: c.root, Caster: casterID, Mark: c.aura, Delay: max(0, e.Delay)})
				// Composite triggers such as I|TB run on application and again at turn start.
				if e.Delay == 0 && waitsFor(e.Trigger, []string{"I"}) && s.met(m, caster, target, had) {
					s.apply(st, c, at, id, a, e)
				}
				continue
			}
			if s.met(m, caster, target, had) {
				s.apply(st, c, at, id, a, e)
			}
		}
	}
}

// hook leaves an effect waiting on an entity until one of its triggers
// happens.
func (s *Sim) hook(st *State, target *Entity, a *attack, e game.SpellEffect, h Hook) {
	h.Dispellable = e.Dispellable
	h.Turns = e.Duration
	if h.Turns == 0 {
		h.Turns = e.TriggerTurns
	}
	switch {
	case h.Turns >= 63 || h.Turns < 0:
		h.Turns = -1
	case h.Turns == 0:
		h.Turns = 1
	}
	for i, other := range target.Hooks {
		if other.Spell == h.Spell && other.Grade == h.Grade && other.Critical == h.Critical && other.Index == h.Index && other.Caster == h.Caster && other.Delay == h.Delay && other.Mark == h.Mark {
			target.Hooks[i].Turns = h.Turns
			return
		}
	}
	if len(target.Hooks) < maxHooks {
		target.Hooks = append(target.Hooks, h)
	}
}

// fire applies the effects waiting on an entity for one of the triggers
// that just happened. source is who made it happen.
func (s *Sim) fire(st *State, c *casting, entityID, sourceID, depth int, triggers ...string) {
	entity := st.entity(entityID)
	if entity == nil || len(entity.Hooks) == 0 || depth >= maxDepth {
		return
	}
	hooks := append([]Hook(nil), entity.Hooks...)
	for _, h := range hooks {
		if c.fires >= maxFires || c.spent() {
			return
		}
		if h.Delay > 0 {
			continue
		}
		a := s.attack(h.Spell, h.Grade)
		if a == nil {
			continue
		}
		effects := a.effects
		if h.Critical {
			effects = a.critEffects
		}
		if h.Index < 0 || h.Index >= len(effects) {
			continue
		}
		e := effects[h.Index]
		if !waitsFor(e.Trigger, triggers) {
			continue
		}
		entity, caster := st.entity(entityID), st.entity(h.Caster)
		if entity == nil {
			return
		}
		if caster == nil {
			caster = entity
		}
		if !s.met(readMask(e.Targets), caster, entity, nil) {
			continue
		}
		c.fires++
		stood := make(map[int]int, len(st.Entities))
		for _, other := range st.Entities {
			stood[other.ID] = other.Cell
		}
		at := place{caster: caster.ID, source: sourceID, cell: entity.Cell, depth: depth + 1, stood: stood, fired: true}
		s.applyHook(st, c, at, entityID, a, e, h)
	}
}

// due counts a turn off the effects an entity holds back, and applies those
// whose time has come: once for an effect with no trigger, while one with a
// trigger starts waiting.
func (s *Sim) due(st *State, c *casting, entityID int) {
	entity := st.entity(entityID)
	if entity == nil {
		return
	}
	var ready []Hook
	kept := entity.Hooks[:0]
	for _, h := range entity.Hooks {
		if h.Delay > 0 {
			if h.Delay--; h.Delay == 0 {
				ready = append(ready, h)
			}
		}
		kept = append(kept, h)
	}
	entity.Hooks = kept
	for _, h := range ready {
		if c.spent() {
			return
		}
		a := s.attack(h.Spell, h.Grade)
		if a == nil {
			continue
		}
		effects := a.effects
		if h.Critical {
			effects = a.critEffects
		}
		if h.Index < 0 || h.Index >= len(effects) || effects[h.Index].Trigger != "" {
			continue
		}
		e := effects[h.Index]
		entity, caster := st.entity(entityID), st.entity(h.Caster)
		if entity == nil {
			return
		}
		if caster == nil {
			caster = entity
		}
		// Applied once: it does not stay waiting.
		for i, other := range entity.Hooks {
			if other == h {
				entity.Hooks = append(entity.Hooks[:i], entity.Hooks[i+1:]...)
				break
			}
		}
		if !s.met(readMask(e.Targets), caster, entity, nil) {
			continue
		}
		stood := make(map[int]int, len(st.Entities))
		for _, other := range st.Entities {
			stood[other.ID] = other.Cell
		}
		at := place{caster: caster.ID, source: caster.ID, cell: entity.Cell, depth: 1, stood: stood, fired: true}
		s.applyHook(st, c, at, entityID, a, e, h)
	}
}

func (s *Sim) applyHook(st *State, c *casting, at place, target int, a *attack, e game.SpellEffect, h Hook) {
	acting := c.acting
	c.acting = 0
	root, aura, action := c.root, c.aura, c.markAction
	projected := c.projection
	c.projection = portalProjection{}
	c.root, c.aura, c.markAction = h.From, h.Mark, 0
	if game.PlacesMark(e.Action) {
		s.placeMark(st, c, at, a, h.Index, h.Critical, e)
	} else {
		s.apply(st, c, at, target, a, e)
	}
	c.root, c.aura, c.markAction = root, aura, action
	c.acting = acting
	c.projection = projected
}

func waitsFor(trigger string, happened []string) bool {
	for _, want := range strings.Split(trigger, "|") {
		for _, got := range happened {
			if want == got {
				return true
			}
		}
	}
	return false
}

func span(e game.SpellEffect) (lo, hi int) {
	lo, hi = e.Min, e.Max
	if hi < lo {
		hi = lo
	}
	return lo, hi
}

// apply applies one effect to one target.
func (s *Sim) apply(st *State, c *casting, at place, targetID int, a *attack, e game.SpellEffect) {
	at.dispellable = e.Dispellable
	caster, target := st.entity(at.caster), st.entity(targetID)
	if caster == nil || target == nil {
		return
	}
	lo, hi := span(e)
	if s.extraEffect(st, c, at, a, e, caster, target, lo, hi) {
		return
	}
	if element, steal, ok := game.AttackAction(e.Action); ok {
		if e.Duration != 0 && !at.fired {
			// Damage over several turns with nothing to trigger it.
			c.skip(e)
			return
		}
		s.hit(st, c, at, caster, target, a, e, element, steal)
		return
	}
	if element, ok := healActions[e.Action]; ok {
		base := c.amount(caster, target, lo, hi) + spellHealBonus(caster, a.id)
		spec := engine.AttackSpec{Level: s.Level}
		line := game.AttackLine{Kind: game.LineHeal, Element: element, Min: base, Max: base}
		amount, _ := spec.Line(line, s.StatOf(caster))
		s.heal(st, c, at, target, s.projectedHeal(st, c, at, amount.Min))
		return
	}
	if game.CastsSpell(e.Action) {
		s.castOn(st, c, at, caster, target, e)
		return
	}
	if isSpellMod(e.Action) {
		s.buff(st, c, at, target, a, Buff{Kind: BuffSpell, Mod: e.Action, Ref: e.Min, Value: e.Value, Turns: e.Duration}, false)
		return
	}
	switch e.Action {
	case 1181, 1182, 1183, 2183, 4007:
		s.applyPortal(st, c, at, a, e, nil)
	case 150:
		was := target.invisible()
		s.buff(st, c, at, target, a, Buff{Kind: BuffInvisible, Turns: e.Duration}, false)
		if !was && target.invisible() {
			c.emit(Event{Type: "invisible", Entity: target.ID, Spell: c.root, Turns: e.Duration})
		}
	case 202:
		s.reveal(st, c, at, target)
	case pickupAction:
		s.pickup(st, c, at, caster, target, a)
	case throwAction:
		s.throw(st, c, at, caster, e, nil)
	case 1020, 1039, 1040:
		value := c.roll(lo, hi)
		if e.Action == 1020 {
			value = value * s.Level / 100
		}
		if e.Action == 1039 {
			value = value * target.MaxHP / 100
		}
		s.buff(st, c, at, target, a, Buff{Kind: BuffShield, Value: value, Turns: e.Duration}, false)
		c.emit(Event{Type: "shield", Entity: target.ID, Spell: c.root, Value: value, Turns: e.Duration})

	case 1079, 1080:
		s.drainGlyphPoints(st, c, at, a, e, caster, target)
	case 1025, 1026, 2018, 2019, 2024:
		for _, mark := range append([]Mark(nil), st.Marks...) {
			isTrap := mark.Action == trapMark
			switch e.Action {
			case 1025, 1026:
				if mark.Action != runeMark && isTrap == (e.Action == 1025) && (e.Value == 0 || mark.From == e.Value) {
					s.triggerMark(st, c, mark, targetID, at.depth+1)
				}
			case 2018, 2019, 2024:
				if isTrap == (e.Action == 2019) && (mark.Action == runeMark) == (e.Action == 2024) &&
					mark.Caster == targetID && (e.Min == 0 || mark.From == e.Min) {
					s.removeMark(st, c, mark, at.depth+1)
				}
			}
		}
	case actionHealPercent:
		s.heal(st, c, at, target, s.projectedHeal(st, c, at, target.MaxHP*c.roll(lo, hi)/100))
	case actionPush, actionPushHarmless:
		s.shove(st, c, caster, target, origin(caster, target, at.cell), e.Min, true, e.Action == actionPush, at.depth)
	case actionPull:
		s.shove(st, c, caster, target, origin(caster, target, at.cell), e.Min, false, false, at.depth)
	case actionRecoil:
		s.shove(st, c, caster, caster, target.Cell, e.Min, true, false, at.depth)
	case actionAdvance:
		s.shove(st, c, caster, caster, target.Cell, e.Min, false, false, at.depth)
	case actionTeleport:
		if st.free(at.cell) {
			from := caster.Cell
			c.emit(Event{Type: EventMove, Entity: caster.ID, Path: []int{caster.Cell, at.cell}})
			s.relocate(st, c, caster, at.cell, at.depth)
			s.crossMarks(st, c, caster.ID, from, at.depth)
		}
	case actionSwap:
		if target.Cell != caster.Cell && target.CarriedBy == 0 && caster.CarriedBy == 0 {
			c.emit(Event{Type: EventMove, Entity: caster.ID, Path: []int{caster.Cell, target.Cell}})
			c.emit(Event{Type: EventMove, Entity: target.ID, Path: []int{target.Cell, caster.Cell}})
			casterFrom, targetFrom := caster.Cell, target.Cell
			casterID, targetID := caster.ID, target.ID
			s.relocate(st, c, caster, targetFrom, at.depth)
			if target := st.entity(targetID); target != nil {
				s.relocate(st, c, target, casterFrom, at.depth)
			}
			s.crossMarks(st, c, casterID, casterFrom, at.depth)
			s.crossMarks(st, c, targetID, targetFrom, at.depth)
		}
	case 1104, 1105, 1106:
		s.reflect(st, c, at, a, e.Action, caster, target)
	case 1075:
		turns := max(0, c.roll(lo, hi))
		for i := range target.Buffs {
			b := &target.Buffs[i]
			if b.Dispellable && b.Turns > 0 {
				b.Turns = max(0, b.Turns-turns)
			}
		}
		s.drop(st, c, at, target, func(b Buff) bool { return b.Dispellable && b.Turns == 0 })
		kept := target.Hooks[:0]
		for _, h := range target.Hooks {
			if h.Dispellable && h.Turns > 0 {
				h.Turns -= turns
				if h.Turns <= 0 {
					continue
				}
			}
			kept = append(kept, h)
		}
		target.Hooks = kept
	case actionRefundAP:
		if target.ID == st.Active().ID {
			gain := c.roll(lo, hi)
			st.APUsed -= gain
			c.emit(Event{Type: EventAP, Entity: target.ID, Value: gain})
		}
	case actionState:
		s.buff(st, c, at, target, a, Buff{Kind: BuffState, Ref: e.Value, Turns: e.Duration}, false)
	case actionRemoveState:
		s.drop(st, c, at, target, func(b Buff) bool { return b.Kind == BuffState && b.Ref == e.Value })
	case actionRemoveSpell:
		s.drop(st, c, at, target, func(b Buff) bool { return b.Spell == e.Value })
		kept := target.Hooks[:0]
		for _, h := range target.Hooks {
			if h.Spell != e.Value {
				kept = append(kept, h)
			}
		}
		target.Hooks = kept
	case actionSpellBase:
		s.buff(st, c, at, target, a, Buff{Kind: BuffBase, Ref: e.Min, Value: e.Value, Turns: e.Duration}, false)
	case actionSpellRange:
		s.buff(st, c, at, target, a, Buff{Kind: BuffRange, Ref: e.Min, Value: e.Value, Turns: e.Duration}, false)
	case actionDamageTaken:
		s.buff(st, c, at, target, a, Buff{Kind: BuffSuffered, Value: e.Min, Turns: e.Duration}, false)
	case actionHealsReceived:
		s.buff(st, c, at, target, a, Buff{Kind: BuffHealed, Value: e.Min, Turns: e.Duration}, false)
	default:
		if e.Sign == 0 {
			c.skip(e)
			return
		}
		value := c.amount(caster, target, lo, hi) * e.Sign
		turns := e.Duration
		if turns == 0 {
			// Points given or taken at once count for the turn in progress.
			if e.Stat != game.StatAP && e.Stat != game.StatMP {
				c.skip(e)
				return
			}
			turns = 1
		}
		s.buff(st, c, at, target, a, Buff{Kind: BuffStat, Ref: e.Stat, Value: value, Turns: turns}, true)
	}
}

// castOn has an effect cast another spell: who casts it and where depends on
// the action.
func (s *Sim) castOn(st *State, c *casting, at place, caster, target *Entity, e game.SpellEffect) {
	sub := s.attack(e.Min, e.Max)
	if sub == nil || at.depth >= maxDepth {
		return
	}
	source := st.entity(at.source)
	if source == nil {
		source = caster
	}
	by, cell := caster, target.Cell
	switch e.Action {
	case game.ActionTargetCasts, game.ActionTargetExecutes, game.ActionTargetExecutesShown:
		by = target
	case game.ActionTargetCastsOnSource:
		by, cell = target, source.Cell
	case game.ActionSourceCasts:
		by = source
	case game.ActionTargetCastsOnCell:
		by, cell = target, at.cell
	}
	s.resolve(st, c, by.ID, at.source, sub, cell, at.depth+1)
}

// origin is the cell a push moves a target away from, and a pull toward: the
// caster for the target a spell is aimed at, the aimed cell for the others.
func origin(caster, target *Entity, cell int) int {
	if target.Cell == cell {
		return caster.Cell
	}
	return cell
}

// drop removes lasting effects from an entity, and tells the effects that
// wait for it to leave a state.
func (s *Sim) drop(st *State, c *casting, at place, target *Entity, match func(Buff) bool) {
	var left []int
	kept := target.Buffs[:0]
	for _, b := range target.Buffs {
		if !match(b) {
			kept = append(kept, b)
		} else if b.Kind == buffVitality {
			target.MaxHP = max(1, target.MaxHP-b.Value)
			target.HP = min(target.MaxHP, target.HP-b.Value)
		} else if b.Kind == BuffState {
			left = append(left, b.Ref)
		}
	}
	target.Buffs = kept
	id := target.ID
	for _, state := range left {
		if e := st.entity(id); e != nil && !e.inState(state) {
			s.fire(st, c, id, at.source, at.depth, "EOFF"+strconv.Itoa(state))
		}
	}
}

// buff puts a lasting effect on a target, unless the spell's effects have
// piled up on it as much as they can.
func (s *Sim) buff(st *State, c *casting, at place, target *Entity, a *attack, b Buff, report bool) bool {
	b.Dispellable = at.dispellable
	b.Spell, b.From, b.Mark = a.id, c.root, c.aura
	if b.Turns == 0 {
		b.Turns = 1
	}
	hadState := b.Kind == BuffState && target.inState(b.Ref)
	for i, other := range target.Buffs {
		if other.Kind != b.Kind || other.Ref != b.Ref || other.Mark != b.Mark {
			continue
		}
		refresh := b.Mark != 0 && other.Spell == b.Spell && other.Mod == b.Mod
		if refresh || hadState {
			if other.Turns >= 0 && (b.Turns < 0 || b.Turns > other.Turns) {
				target.Buffs[i].Turns = b.Turns
			}
			return false
		}
	}
	if a.rules.MaxStack > 0 {
		count := 0
		for _, other := range target.Buffs {
			if other.Kind == b.Kind && other.Spell == b.Spell && other.Ref == b.Ref && other.Mod == b.Mod {
				count++
			}
		}
		if count >= a.rules.MaxStack {
			return false
		}
	}
	if len(target.Buffs) >= maxBuffs {
		return false
	}
	target.Buffs = append(target.Buffs, b)
	if report {
		stat := b.Ref
		c.emit(Event{Type: EventBuff, Entity: target.ID, Spell: c.root, Stat: &stat, Value: b.Value, Turns: b.Turns})
	}
	if b.Kind == BuffState && !hadState {
		s.fire(st, c, target.ID, at.source, at.depth, "EON"+strconv.Itoa(b.Ref))
	}
	return true
}

func (s *Sim) heal(st *State, c *casting, at place, target *Entity, amount int) {
	for _, b := range target.Buffs {
		if b.Kind == BuffHealed {
			amount = amount * b.Value / 100
		}
	}
	amount = max(0, min(amount, target.MaxHP-target.HP))
	target.HP += amount
	c.emit(Event{Type: EventHeal, Entity: target.ID, Value: amount})
	if amount > 0 {
		s.fire(st, c, target.ID, at.source, at.depth, "H")
	}
}

// damageTriggers names what a hit sets off on its target: any damage, its
// element, and whether it came from the next cell or from afar.
var damageTriggers = map[int]string{
	game.ElementNeutral: "DN", game.ElementEarth: "DE", game.ElementFire: "DF", game.ElementWater: "DW", game.ElementAir: "DA",
}

// hit deals one damage line to a target.
func (s *Sim) hit(st *State, c *casting, at place, caster, target *Entity, a *attack, e game.SpellEffect, element int, steal bool) {
	s.hitWithRatio(st, c, at, caster, target, a, e, element, steal, 1, 1)
}
func (s *Sim) hitWithRatio(st *State, c *casting, at place, caster, target *Entity, a *attack, e game.SpellEffect, element int, steal bool, numerator, denominator int) {
	lo, hi := span(e)
	base := c.amount(caster, target, lo, hi)
	flat := 0
	for _, b := range caster.Buffs {
		if a.weapon || b.Ref != a.id {
			continue
		}
		if b.Kind == BuffBase {
			base += b.Value
		} else if b.Kind == BuffSpell && b.Mod == modDamage {
			flat += b.Value
		}
	}
	stat := s.StatOf(caster)
	// Melee or ranged, and how far from the centre of the zone, are those of
	// the moment the spell was cast.
	melee := st.Board.Distance(at.stood[caster.ID], at.stood[target.ID]) <= 1
	spec := engine.AttackSpec{Weapon: a.weapon, Melee: melee, Level: s.Level, Power: stat(statSpellPower)}
	if a.weapon {
		spec.Power = stat(statWeaponPower)
	} else if c.markAction == trapMark {
		spec.Power += stat(69) // trapDamageBonusPercent
		baseStat := stat
		stat = func(id int) int {
			if id == game.StatDamage {
				return baseStat(id) + baseStat(70) // trapDamageBonus
			}
			return baseStat(id)
		}
	} else if c.markAction == runeMark {
		spec.Power += stat(110) // runePower
	} else if c.markAction != 0 {
		spec.Power += stat(106) // glyphPower
	}
	damage, element := spec.CombatHit(base, element, c.crit, stat)
	s.dealHit(st, c, at, caster, target, e, element, (damage+flat)*numerator/denominator, melee, steal)
}

func (s *Sim) dealHit(st *State, c *casting, at place, caster, target *Entity, e game.SpellEffect, element, damage int, melee, steal bool) {
	stat := s.StatOf(caster)
	damage = damage * max(0, 100+stat(statFinalDamage)) / 100
	if c.projection.route.Bonus > 0 {
		damage = damage * (100 + c.projection.route.Bonus) / 100
	}
	// Away from the centre of a zone, a hit loses a share of its damage per
	// cell.
	if e.Zone.Shape != "" && e.Zone.Shape != "P" && !at.fired {
		steps := min(st.Board.Distance(at.cell, at.stood[target.ID]), e.Zone.Steps)
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
	for _, b := range target.Buffs {
		if b.Kind == BuffArmor {
			fixed += b.Value
		}
	}
	if c.crit {
		fixed += resist(statCriticalResis)
	}
	damage = max(0, damage-fixed) * max(0, 100-resist(percentResist[element])-resist(statResistGlobal)) / 100
	damage = s.absorb(c, target, damage)
	s.loseHealth(target, damage)
	c.emit(Event{Type: EventDamage, Entity: target.ID, Value: damage, Element: element, Critical: c.crit})
	if damage > 0 && c.acting == caster.ID && caster.ID != target.ID && !at.fired && c.markAction == 0 {
		s.reveal(st, c, at, caster)
	}
	casterID, targetID := caster.ID, target.ID
	if steal && casterID != targetID {
		s.heal(st, c, at, caster, damage/2)
	}
	if damage > 0 {
		reach, side := "DR", "DBE"
		if melee {
			reach = "DM"
		}
		if caster.enemyTeam() == target.enemyTeam() {
			side = "DBA"
		}
		previous := c.triggeringDamage
		c.triggeringDamage = damage
		s.fire(st, c, targetID, casterID, at.depth, "D", damageTriggers[element], reach, side)
		c.triggeringDamage = previous
	}
}

func (e *Entity) invisible() bool {
	for _, b := range e.Buffs {
		if b.Kind == BuffInvisible {
			return true
		}
	}
	return false
}

func (s *Sim) reveal(st *State, c *casting, at place, target *Entity) {
	if !target.invisible() {
		return
	}
	s.drop(st, c, at, target, func(b Buff) bool { return b.Kind == BuffInvisible })
	c.emit(Event{Type: "reveal", Entity: target.ID, Spell: c.root})
}

// shove moves an entity in a straight line, away from a cell or toward it,
// until something stands in the way. A push stopped short deals pushback
// damage for the cells left, and half of it to whoever blocked the way.
func (s *Sim) shove(st *State, c *casting, caster, moved *Entity, from, cells int, away, harmful bool, depth int) {
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
		x, y := st.Board.xy(path[len(path)-1])
		if !st.Board.inside(x+dx, y+dy) {
			break
		}
		next := st.Board.cell(x+dx, y+dy)
		if !st.free(next) {
			blocker = st.at(next)
			break
		}
		path = append(path, next)
	}
	if len(path) > 1 {
		s.relocate(st, c, moved, path[len(path)-1], depth)
		c.emit(Event{Type: EventMove, Entity: moved.ID, Path: path})
		defer s.crossMarks(st, c, moved.ID, path[0], depth)
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
		taken = s.absorb(c, hurt, taken)
		s.loseHealth(hurt, taken)
		c.emit(Event{Type: EventDamage, Entity: hurt.ID, Value: taken, Push: true})
	}
}

func (s *Sim) absorb(c *casting, target *Entity, damage int) int {
	remaining := damage
	for i := range target.Buffs {
		shield := &target.Buffs[i]
		if shield.Kind != BuffShield || remaining <= 0 {
			continue
		}
		absorbed := min(remaining, max(0, shield.Value))
		shield.Value -= absorbed
		remaining -= absorbed
	}
	if damage != remaining {
		c.emit(Event{Type: "shield", Entity: target.ID, Spell: c.root, Value: remaining - damage})
	}
	return remaining
}

// Erosion lowers the ceiling for heals. A training dummy regains its initial
// pool on revival, including after several hits in one cast.
func (s *Sim) loseHealth(target *Entity, damage int) {
	if damage <= 0 {
		return
	}
	if target.InitialHP == 0 {
		target.InitialHP = target.MaxHP
	}
	target.HP -= damage
	erosion := min(50, max(0, 10+s.StatOf(target)(75)))
	target.MaxHP = max(1, target.MaxHP-damage*erosion/100)
}

// Reflect a fighter through the indicated pivot. The spells that react to a
// telefrag identify its participants with the T mask in their following effects.
func (s *Sim) reflect(st *State, c *casting, at place, a *attack, action int, caster, target *Entity) {
	moved, pivot := target, at.cell
	switch action {
	case 1104:
		moved, pivot = caster, at.stood[target.ID]
	case 1105:
		pivot = at.stood[caster.ID]
	}
	x, y := st.Board.xy(at.stood[moved.ID])
	px, py := st.Board.xy(pivot)
	x, y = 2*px-x, 2*py-y
	if !st.Board.inside(x, y) {
		return
	}
	to := st.Board.cell(x, y)
	if to == moved.Cell || st.Board.wall(to) {
		return
	}
	s.teleportWithSwap(st, c, at, a, moved, to)
}
func (s *Sim) teleportWithSwap(st *State, c *casting, at place, a *attack, moved *Entity, to int) bool {
	if !st.Board.valid(to) || st.Board.wall(to) || to == moved.Cell {
		return false
	}
	from, id := moved.Cell, moved.ID
	blocker := st.at(to)
	if blocker != nil {
		swap := false
		for _, e := range a.effects {
			if readMask(e.Targets).telefrag {
				swap = true
				break
			}
		}
		if !swap {
			return false
		}
		if c.telefrag == nil {
			c.telefrag = map[int]bool{}
		}
		c.telefrag[id], c.telefrag[blocker.ID] = true, true
		s.relocate(st, c, blocker, from, at.depth)
		c.emit(Event{Type: EventMove, Entity: blocker.ID, Path: []int{to, from}})
	}
	s.relocate(st, c, moved, to, at.depth)
	c.emit(Event{Type: EventMove, Entity: id, Path: []int{from, to}})
	if blocker != nil {
		s.crossMarks(st, c, blocker.ID, to, at.depth)
	}
	s.crossMarks(st, c, id, from, at.depth)
	return true
}
