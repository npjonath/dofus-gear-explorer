package fight

import (
	"dofusdb/api/internal/engine"
	"dofusdb/api/internal/game"
	"slices"
)

const (
	buffVitality      = "vitality"
	buffMinRoll       = "minimum"
	buffMaxRoll       = "maximum"
	buffHealBase      = "healBase"
	buffStateDisabled = "stateDisabled"
)

func (e *Entity) startPosition() { cell := e.Cell; e.TurnCell = &cell }
func (e *Entity) rememberPosition(next int) {
	if next == e.Cell {
		return
	}
	if len(e.PreviousCells) >= 32 {
		e.PreviousCells = slices.Delete(e.PreviousCells, 0, len(e.PreviousCells)-31)
	}
	e.PreviousCells = append(e.PreviousCells, e.Cell)
}

func (c *casting) amount(caster, target *Entity, lo, hi int) int {
	for _, b := range target.Buffs {
		if b.Kind == buffMaxRoll {
			return hi
		}
	}
	for _, b := range caster.Buffs {
		if b.Kind == buffMinRoll {
			return lo
		}
	}
	return c.roll(lo, hi)
}

func spellHealBonus(caster *Entity, id int) int {
	extra := 0
	for _, b := range caster.Buffs {
		if b.Kind == buffHealBase && b.Ref == id {
			extra += b.Value
		}
	}
	return extra
}

func (e *Entity) changeVitality(value int) {
	if e.InitialHP == 0 {
		e.InitialHP = e.MaxHP
	}
	e.HP += value
	e.MaxHP += value
}

func eroded(e *Entity) int {
	if e.InitialHP <= 0 {
		return 0
	}
	effective := e.MaxHP
	for _, b := range e.Buffs {
		if b.Kind == buffVitality {
			effective -= b.Value
		}
	}
	return max(0, e.InitialHP-effective)
}

func (s *Sim) extraEffect(st *State, c *casting, at place, a *attack, e game.SpellEffect, caster, target *Entity, lo, hi int) bool {
	switch e.Action {
	case 1099, 1100:
		to := -1
		var history []int
		if e.Action == 1100 {
			history = slices.Clone(target.PreviousCells)
		}
		if e.Action == 1099 && target.TurnCell != nil {
			to = *target.TurnCell
		}
		if e.Action == 1100 && len(history) > 0 {
			to = history[len(history)-1]
		}
		if to >= 0 && s.teleportWithSwap(st, c, at, a, target, to) && e.Action == 1100 {
			// A rewind consumes the old position; the rewind itself is not a new step.
			target.PreviousCells = append([]int(nil), history[:len(history)-1]...)
		}
	case 1048:
		amount := max(0, target.HP) * c.amount(caster, target, lo, hi) / 100
		target.HP -= amount
		c.emit(Event{Type: EventDamage, Entity: target.ID, Spell: c.root, Value: amount})
	case 1078, 1033:
		amount := target.MaxHP * c.amount(caster, target, lo, hi) / 100
		if e.Action == 1033 {
			amount = -min(amount, target.MaxHP-1)
		}
		if s.buff(st, c, at, target, a, Buff{Kind: buffVitality, Ref: game.StatVitality, Value: amount, Turns: e.Duration}, true) {
			target.changeVitality(amount)
		}
	case 89, 279, 1092, 1118:
		basis := max(0, caster.HP)
		switch e.Action {
		case 279:
			basis = max(0, caster.MaxHP-caster.HP)
		case 1092:
			basis = eroded(target)
		case 1118:
			basis = eroded(caster)
		}
		melee := st.Board.Distance(at.stood[caster.ID], at.stood[target.ID]) <= 1
		amount := basis * c.amount(caster, target, lo, hi) / 100
		s.dealHit(st, c, at, caster, target, e, game.ElementNeutral, amount, melee, false)
	case 90:
		amount := max(0, caster.HP) * c.amount(caster, target, lo, hi) / 100
		if caster.ID != target.ID && amount > 0 {
			caster.HP -= amount
			c.emit(Event{Type: EventDamage, Entity: caster.ID, Spell: c.root, Value: amount})
			s.heal(st, c, at, target, amount)
		}
	case 3002:
		stat := s.StatOf(caster)
		amount := c.amount(caster, target, lo, hi) + spellHealBonus(caster, a.id)
		spec := engine.AttackSpec{Level: s.Level}
		heal, _ := spec.Line(game.AttackLine{Kind: game.LineHeal, Element: game.ElementBest, Min: amount, Max: amount}, stat)
		s.heal(st, c, at, target, heal.Min)
	case 2935:
		s.buff(st, c, at, target, a, Buff{Kind: buffHealBase, Ref: e.Min, Value: e.Value, Turns: e.Duration}, false)
	case 2973:
		// Distribution is cast by a damage trigger. Each invocation uses the
		// individual hit absorbed by its bearer, including during multi-line attacks.
		s.heal(st, c, at, target, c.triggeringDamage*c.amount(caster, target, lo, hi)/100)
	case 266, 267, 268, 269, 270, 271, 320, 77:
		stats := map[int]int{266: game.StatChance, 267: game.StatVitality, 268: game.StatAgility, 269: game.StatIntel, 270: game.StatWisdom, 271: game.StatStrength, 320: game.StatRange, 77: game.StatMP}
		id := stats[e.Action]
		amount := c.amount(caster, target, lo, hi)
		if id == game.StatMP {
			amount = min(amount, max(0, s.StatOf(target)(id)))
			if target.ID == st.Active().ID {
				amount = min(amount, max(0, s.StatOf(target)(id)-st.MPUsed))
			}
		}
		if target.ID != caster.ID && amount > 0 {
			kind := BuffStat
			if id == game.StatVitality {
				kind = buffVitality
				amount = min(amount, max(0, target.MaxHP-1))
			}
			if s.buff(st, c, at, target, a, Buff{Kind: kind, Ref: id, Value: -amount, Turns: e.Duration}, true) {
				if kind == buffVitality {
					target.changeVitality(-amount)
				}
				if s.buff(st, c, at, caster, a, Buff{Kind: kind, Ref: id, Value: amount, Turns: e.Duration}, true) && kind == buffVitality {
					caster.changeVitality(amount)
				}
			}
		}
	case 141:
		target.HP = 0
		c.emit(Event{Type: "eliminated", Entity: target.ID, Spell: c.root})
	case 781, 782:
		kind := buffMinRoll
		if e.Action == 782 {
			kind = buffMaxRoll
		}
		s.buff(st, c, at, target, a, Buff{Kind: kind, Turns: e.Duration}, false)
	case 952:
		s.buff(st, c, at, target, a, Buff{Kind: buffStateDisabled, Ref: e.Value, Turns: e.Duration}, false)
	case 1031:
		if target.ID == st.Active().ID {
			c.finishTurn = true
		}
	case 1036:
		counters := st.Uses
		if target.Kind == KindSummon {
			counters = target.Uses
		}
		if use := counters[e.Min]; use != nil {
			use.Last = max(0, use.Last-e.Value)
		}
	case 1013, 1016, 1014, 1015:
		maxMP := s.StatOf(caster)(game.StatMP)
		if maxMP <= 0 {
			return true
		}
		remaining := maxMP
		if caster.ID == st.Active().ID {
			remaining = max(0, maxMP-st.MPUsed)
		}
		el := map[int]int{1013: game.ElementAir, 1014: game.ElementWater, 1015: game.ElementFire, 1016: game.ElementEarth}[e.Action]
		// A zero base must also suppress the flat bonuses. Apply the ratio to the full hit.
		s.hitWithRatio(st, c, at, caster, target, a, e, el, false, remaining, maxMP)
	default:
		return false
	}
	return true
}
