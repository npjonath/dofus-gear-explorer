package fight

import (
	"slices"

	"dofusdb/api/internal/game"
)

const (
	trapMark  = 400
	startMark = 401
	endMark   = 402
	auraMark  = 1091
	entryMark = 1165
	runeMark  = 2022
	maxMarks  = 64
)

// Mark points to the source effect in the catalog. Cells are fixed at creation,
// even when the caster subsequently moves. The payload always comes from the
// referenced effect rather than a spell supplied by the client.
type Mark struct {
	ID       int   `json:"id"`
	Action   int   `json:"action"`
	Caster   int   `json:"caster"`
	From     int   `json:"from"`
	Spell    int   `json:"spell"`
	Grade    int   `json:"grade"`
	Index    int   `json:"index"`
	Critical bool  `json:"critical,omitempty"`
	Cell     int   `json:"cell"`
	Cells    []int `json:"cells"`
	Color    int   `json:"color"`
	Turns    int   `json:"turns"`
}

// ValidMarks bounds the persisted mark count, cells and catalog references
// before the same state can be used by a turn, cast or preview.
func ValidMarks(st *State) bool {
	if st == nil || len(st.Marks) > maxMarks {
		return false
	}
	ids := make(map[int]bool, len(st.Marks))
	size := st.Board.Width * st.Board.Height
	for _, mark := range st.Marks {
		if mark.ID < 1 || mark.ID > 1<<30 || ids[mark.ID] || !game.PlacesMark(mark.Action) ||
			mark.Spell <= 0 || mark.Grade < 1 || mark.Index < 0 || st.entity(mark.Caster) == nil ||
			!st.Board.valid(mark.Cell) || mark.Color < 0 || mark.Color > 0xffffff ||
			mark.Turns == 0 || mark.Turns < -1 || mark.Turns > 64 || len(mark.Cells) < 1 || len(mark.Cells) > size {
			return false
		}
		ids[mark.ID] = true
		last := -1
		for _, cell := range mark.Cells {
			if !st.Board.valid(cell) || cell <= last {
				return false
			}
			last = cell
		}
	}
	return true
}

func (s *Sim) placeMark(st *State, c *casting, at place, a *attack, index int, critical bool, e game.SpellEffect) {
	if at.depth >= maxDepth || !st.Board.valid(at.cell) {
		return
	}
	caster := st.entity(at.caster)
	if caster == nil || s.attack(e.Min, e.Max) == nil {
		return
	}
	for _, cond := range readMask(e.Targets).conds {
		if cond.onCaster && !s.holds(cond, caster, nil) {
			return
		}
	}
	if e.Action == runeMark {
		// A new elemental rune replaces this caster's rune on the same
		// cell. Other fighters retain their own independent marks.
		for _, m := range slices.Clone(st.Marks) {
			if m.Action == runeMark && m.Caster == at.caster && m.Cell == at.cell {
				s.removeMark(st, c, m, at.depth)
			}
		}
	}
	if len(st.Marks) >= maxMarks {
		return
	}
	for _, m := range st.Marks {
		c.lastMarkID = max(c.lastMarkID, m.ID)
		// A recursive payload may try to recreate its own aura on the tile.
		if m.Action == e.Action && m.Cell == at.cell && m.Caster == at.caster && m.Spell == a.id && m.Index == index {
			return
		}
	}
	if c.lastMarkID >= 1<<30 {
		return
	}
	c.lastMarkID++
	turns := e.Duration
	if turns <= 0 || turns >= 63 {
		turns = -1
	}
	cells := st.Board.zone(e.Zone, caster.Cell, at.cell)
	cells = slices.DeleteFunc(cells, st.Board.wall)
	if len(cells) == 0 {
		return
	}
	slices.Sort(cells)
	mark := Mark{ID: c.lastMarkID, Action: e.Action, Caster: at.caster, From: c.root,
		Spell: a.id, Index: index, Grade: a.grade, Critical: critical,
		Cell: at.cell, Cells: cells, Color: e.Value & 0xffffff, Turns: turns}
	st.Marks = append(st.Marks, mark)
	if mark.Action == runeMark {
		c.emit(Event{Type: "rune", Entity: caster.ID, Spell: a.id, Cell: &mark.Cell, Action: runeMark})
	}
	if mark.Action == auraMark {
		for _, target := range st.Entities {
			s.triggerMark(st, c, mark, target.ID, at.depth+1)
		}
	}
}

func (s *Sim) markEffect(m Mark) *game.SpellEffect {
	a := s.attack(m.Spell, m.Grade)
	if a == nil || a.grade != m.Grade {
		return nil
	}
	effects := a.effects
	if m.Critical {
		effects = a.critEffects
	}
	if m.Index < 0 || m.Index >= len(effects) || effects[m.Index].Action != m.Action {
		return nil
	}
	return &effects[m.Index]
}

// Runes are activated by another spell, even when the rune's tile is empty.
// The payload may target the occupant, the owner, or a designated fighter
// elsewhere on the board, as in Runic Overcharge.
func (s *Sim) actOnRunes(st *State, c *casting, at place, e game.SpellEffect, had map[int]map[int]bool) {
	caster := st.entity(at.caster)
	if caster == nil || at.depth >= maxDepth || len(st.Marks) == 0 {
		return
	}
	mask := readMask(e.Targets)
	for _, condition := range mask.conds {
		if condition.onCaster && !s.holds(condition, caster, had) {
			return
		}
	}
	zone := st.Board.zone(e.Zone, at.stood[caster.ID], at.cell)
	for _, m := range slices.Clone(st.Marks) {
		if m.Action != runeMark || m.Caster != at.caster || !slices.Contains(zone, m.Cell) || !st.hasMark(m) {
			continue
		}
		if c.fires >= maxFires || c.spent() {
			break
		}
		occupant := st.at(m.Cell)
		if occupant != nil && (!mask.names(caster, occupant) || !s.met(mask, caster, occupant, had)) {
			continue
		}
		record := s.markEffect(m)
		if record == nil || (e.Value > 0 && record.Min != e.Value && m.From != e.Value) {
			continue
		}
		payload := s.attack(record.Min, record.Max)
		if payload == nil {
			continue
		}
		s.removeMark(st, c, m, at.depth)
		entityID := 0
		if occupant != nil {
			entityID = occupant.ID
		}
		c.fires++
		states := []int{}
		for _, b := range caster.Buffs {
			if b.Kind == BuffState {
				states = append(states, b.Ref)
			}
		}
		c.emit(Event{Type: "mark", Entity: entityID, Spell: m.Spell, TriggerSpell: payload.id, Cell: &m.Cell, Action: runeMark, CasterStates: states})
		root, action := c.root, c.markAction
		c.root, c.markAction = m.Spell, runeMark
		s.resolve(st, c, m.Caster, m.Caster, payload, m.Cell, at.depth+1)
		c.root, c.markAction = root, action
	}
}

func (st *State) hasMark(m Mark) bool {
	for _, cur := range st.Marks {
		if cur.ID == m.ID && cur.Spell == m.Spell && cur.Index == m.Index && cur.Cell == m.Cell && cur.Caster == m.Caster {
			return true
		}
	}
	return false
}

func (s *Sim) triggerMark(st *State, c *casting, m Mark, targetID, depth int) bool {
	if depth >= maxDepth || c.fires >= maxFires || c.spent() {
		return false
	}
	entity, caster := st.entity(targetID), st.entity(m.Caster)
	if entity == nil || caster == nil || !st.hasMark(m) {
		return false
	}
	if _, inside := slices.BinarySearch(m.Cells, entity.Cell); !inside {
		return false
	}
	e := s.markEffect(m)
	if e == nil {
		return false
	}
	mask := readMask(e.Targets)
	if !mask.names(caster, entity) || !s.met(mask, caster, entity, nil) {
		return false
	}
	payload := s.attack(e.Min, e.Max)
	if payload == nil {
		return false
	}
	only := targetID
	cell := m.Cell
	if m.Action == entryMark {
		cell = entity.Cell
	}
	if m.Action == trapMark {
		// A trap springs once, then resolves over its entire area. Remove
		// it first: its own push can lead into the next trap in a chain.
		c.lastMarkID = max(c.lastMarkID, m.ID)
		st.Marks = slices.DeleteFunc(st.Marks, func(cur Mark) bool { return cur.ID == m.ID })
		only = 0
	}
	c.fires++
	c.emit(Event{Type: "mark", Entity: targetID, Spell: m.From, TriggerSpell: payload.id, Cell: &m.Cell, Action: m.Action})
	root, crit, aura, action := c.root, c.crit, c.aura, c.markAction
	c.root, c.crit, c.aura, c.markAction = m.From, m.Critical, 0, m.Action
	if m.Action == auraMark {
		c.aura = m.ID
	}
	s.resolveOn(st, c, m.Caster, targetID, payload, cell, depth+1, only)
	c.root, c.crit, c.aura, c.markAction = root, crit, aura, action
	return true
}

func (s *Sim) removeAura(st *State, c *casting, mark Mark, targetID, depth int) {
	target := st.entity(targetID)
	if target == nil {
		return
	}
	s.drop(st, c, place{caster: mark.Caster, source: mark.Caster, cell: mark.Cell, depth: depth},
		target, func(b Buff) bool { return b.Mark == mark.ID })
	if target = st.entity(targetID); target != nil {
		target.Hooks = slices.DeleteFunc(target.Hooks, func(h Hook) bool { return h.Mark == mark.ID })
	}
}

func (s *Sim) removeMark(st *State, c *casting, m Mark, depth int) {
	if !st.hasMark(m) {
		return
	}
	c.lastMarkID = max(c.lastMarkID, m.ID)
	st.Marks = slices.DeleteFunc(st.Marks, func(cur Mark) bool { return cur.ID == m.ID })
	if m.Action == auraMark {
		for _, e := range st.Entities {
			s.removeAura(st, c, m, e.ID, depth)
		}
	}
}

func (s *Sim) ageMarks(st *State, c *casting, caster int) {
	for _, old := range slices.Clone(st.Marks) {
		if old.Caster != caster || old.Turns < 0 {
			continue
		}
		for i, m := range st.Marks {
			if m.ID != old.ID || m.Spell != old.Spell {
				continue
			}
			if m.Turns > 1 {
				st.Marks[i].Turns--
			} else {
				s.removeMark(st, c, m, 0)
			}
			break
		}
	}
}

func (s *Sim) markTurn(st *State, c *casting, id int, start bool) {
	for _, m := range slices.Clone(st.Marks) {
		if start && (m.Action == startMark || m.Action == auraMark) || !start && m.Action == endMark {
			s.triggerMark(st, c, m, id, 0)
		}
	}
}

// crossMarks sees each step of a walk and the landing cell of an imposed
// displacement. An aura stays effective until the fighter leaves its zone.
func (s *Sim) crossMarks(st *State, c *casting, id, from, depth int) bool {
	if depth >= maxDepth {
		return false
	}
	sprung := false
	for _, m := range slices.Clone(st.Marks) {
		target := st.entity(id)
		if target == nil || target.HP <= 0 {
			break
		}
		_, before := slices.BinarySearch(m.Cells, from)
		_, after := slices.BinarySearch(m.Cells, target.Cell)
		if before && !after && m.Action == auraMark {
			s.removeAura(st, c, m, id, depth)
		} else if !before && after && (m.Action == trapMark || m.Action == entryMark || m.Action == auraMark) {
			if s.triggerMark(st, c, m, id, depth+1) && m.Action == trapMark {
				sprung = true
			}
		}
	}
	return sprung
}

// Each point has a separate loss check. A dummy with no points left dodges
// the entire loss, as it does in the Scorched Earth glyph.
func (s *Sim) drainGlyphPoints(st *State, c *casting, at place, a *attack, e game.SpellEffect, caster, target *Entity) {
	lo, hi := span(e)
	wanted := c.roll(lo, hi)
	stats := s.StatOf(target)
	stat, reductionStat, dodgeStat, used := game.StatMP, game.StatMPReduction, game.StatDodgeMP, st.MPUsed
	if e.Action == 1079 {
		stat, reductionStat, dodgeStat, used = game.StatAP, game.StatAPReduction, game.StatDodgeAP, st.APUsed
	}
	points := max(0, stats(stat))
	remaining := points
	if st.Active().ID == target.ID {
		remaining = max(0, remaining-used)
	}
	removed := 0
	reduction := max(1, s.StatOf(caster)(reductionStat))
	resistance := max(1, stats(dodgeStat))
	for range wanted {
		if remaining == 0 || points == 0 || c.spent() {
			break
		}
		chance := 50 * reduction * remaining / resistance / points
		if c.roll(1, 100) <= min(90, max(10, chance)) {
			removed++
			remaining--
		}
	}
	if removed > 0 {
		s.buff(st, c, at, target, a, Buff{Kind: BuffStat, Ref: stat, Value: -removed, Turns: e.Duration}, true)
	}
	if removed < wanted {
		c.emit(Event{Type: "dodge", Spell: c.root, Entity: target.ID, Stat: &stat, Value: wanted - removed})
	}
}
