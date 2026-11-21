package fight

import "dofusdb/api/internal/game"

// The client attaches a carried fighter to its carrier's cell. Only one
// fighter can be carried at a time, and carriers cannot carry each other.
const (
	pickupAction = 50
	throwAction  = 51
	carrierState = 3
	carriedState = 8
)

func (st *State) passenger(id int) *Entity {
	for i := range st.Entities {
		if st.Entities[i].CarriedBy == id {
			return &st.Entities[i]
		}
	}
	return nil
}

func (st *State) canCarry(carrier, rider *Entity) bool {
	return carrier.ID != rider.ID && carrier.CarriedBy == 0 && rider.CarriedBy == 0 &&
		st.passenger(carrier.ID) == nil && st.passenger(rider.ID) == nil &&
		!rider.inState(63) && !rider.inState(97) && rider.HP > 0
}

// ValidPositions permits one shared cell only for a real carrier/passenger pair.
func ValidPositions(st *State) bool {
	floor := make(map[int]int)
	carried := make(map[int]int)
	for _, e := range st.Entities {
		if e.CarriedBy == 0 {
			if floor[e.Cell] != 0 {
				return false
			}
			floor[e.Cell] = e.ID
		} else {
			if carried[e.Cell] != 0 || e.CarriedBy == e.ID || st.Phase != PhaseFight {
				return false
			}
			carried[e.Cell] = e.CarriedBy
		}
	}
	for cell, carrier := range carried {
		if floor[cell] != carrier {
			return false
		}
	}
	return true
}

func (st *State) carryRules(caster *Entity, a *attack) *attack {
	if a == nil {
		return nil
	}
	canPick, canThrow, rangeBonus := false, false, 0
	for _, e := range a.effects {
		if e.Action == actionSpellRange && e.Described && e.Min == a.id {
			rangeBonus = e.Value
		}
		if e.Described || e.Trigger != "" || e.Delay != 0 {
			continue
		}
		canPick = canPick || e.Action == pickupAction
		canThrow = canThrow || e.Action == throwAction
	}
	if !canPick && !canThrow {
		return a
	}
	out := *a
	carrying := st.passenger(caster.ID) != nil
	out.pickup, out.throw = canPick && !carrying, canThrow && (carrying || !canPick)
	if out.throw {
		out.rules.FreeCell, out.rules.TakenCell = true, false
		for _, buff := range caster.Buffs {
			if buff.Kind == BuffRange && buff.Ref == a.id {
				rangeBonus = 0 // The class's opening passives already granted it.
			}
		}
		out.rangeMax += rangeBonus
	} else if out.pickup {
		out.rules.FreeCell, out.rules.TakenCell = false, true
	}
	return &out
}

func (s *Sim) pickup(st *State, c *casting, at place, caster, rider *Entity, a *attack) {
	if !st.canCarry(caster, rider) {
		return
	}
	from, carrierID, riderID := rider.Cell, caster.ID, rider.ID
	rider.CarriedBy, rider.Cell = carrierID, caster.Cell
	c.emit(Event{Type: EventMove, Entity: riderID, Carrier: carrierID, Action: pickupAction, Path: []int{from, caster.Cell}})
	s.buff(st, c, at, caster, a, Buff{Kind: BuffState, Ref: carrierState, Turns: -1}, false)
	if rider = st.entity(riderID); rider != nil && rider.CarriedBy == carrierID {
		s.buff(st, c, at, rider, a, Buff{Kind: BuffState, Ref: carriedState, Turns: -1}, false)
	}
	s.crossMarks(st, c, riderID, from, at.depth)
}

func (s *Sim) uncarry(st *State, c *casting, rider *Entity, cell, depth int, showThrow bool) {
	from, carrierID, riderID := rider.Cell, rider.CarriedBy, rider.ID
	rider.CarriedBy, rider.Cell = 0, cell
	at := place{caster: carrierID, source: carrierID, cell: cell, depth: depth}
	if showThrow {
		c.emit(Event{Type: EventMove, Entity: riderID, Carrier: carrierID, Action: throwAction, Path: []int{from, cell}})
	} else {
		c.emit(Event{Type: "release", Entity: riderID, Carrier: carrierID})
	}
	if carrier := st.entity(carrierID); carrier != nil {
		s.drop(st, c, at, carrier, func(b Buff) bool { return b.Kind == BuffState && b.Ref == carrierState })
	}
	if rider = st.entity(riderID); rider != nil {
		s.drop(st, c, at, rider, func(b Buff) bool { return b.Kind == BuffState && b.Ref == carriedState })
	}
}

func (s *Sim) relocate(st *State, c *casting, e *Entity, cell, depth int) {
	if e.CarriedBy != 0 {
		s.uncarry(st, c, e, cell, depth, false)
		return
	}
	e.rememberPosition(cell)
	e.Cell = cell
	if rider := st.passenger(e.ID); rider != nil {
		rider.rememberPosition(cell)
		rider.Cell = cell
	}
}

func (s *Sim) throw(st *State, c *casting, at place, caster *Entity, e game.SpellEffect, had map[int]map[int]bool) {
	rider := st.passenger(caster.ID)
	if rider == nil || !st.free(at.cell) {
		return
	}
	m := readMask(e.Targets)
	if !m.names(caster, rider) || !s.met(m, caster, rider, had) {
		return
	}
	id, from := rider.ID, rider.Cell
	s.uncarry(st, c, rider, at.cell, at.depth, true)
	s.crossMarks(st, c, id, from, at.depth)
}

func (s *Sim) releaseDefeated(st *State, c *casting, e *Entity) {
	rider := e
	if e.CarriedBy == 0 {
		rider = st.passenger(e.ID)
	}
	if rider == nil {
		return
	}
	// A dead summon leaves the board. A training dummy stands back up and
	// needs a free cell when its carrier is still present.
	cell := e.Cell
	if e.Kind != KindSummon {
		best := st.Board.Width + st.Board.Height
		for next := 0; next < st.Board.Width*st.Board.Height; next++ {
			if distance := st.Board.Distance(e.Cell, next); distance < best && st.free(next) {
				cell, best = next, distance
			}
		}
		if cell == e.Cell {
			return
		}
	}
	s.uncarry(st, c, rider, cell, 0, cell != rider.Cell)
}
