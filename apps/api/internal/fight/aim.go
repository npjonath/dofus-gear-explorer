package fight

import "dofusdb/api/internal/game"

// Slot is a spell of the player's bar as the fight stands.
type Slot struct {
	// ID is the spell, or WeaponID for the weapon.
	ID       int `json:"id"`
	APCost   int `json:"apCost"`
	RangeMin int `json:"rangeMin"`
	RangeMax int `json:"rangeMax"`
	// CritRate is the chance of a critical hit with this stuff, in percent.
	CritRate int `json:"critRate"`
	// Blocked is empty when the spell can be cast, or why it cannot.
	Blocked string `json:"blocked,omitempty"`
	// Cooldown is the number of turns before it can be cast again, and Left
	// the casts left this turn; -1 when the spell has no cap.
	Cooldown int `json:"cooldown"`
	Left     int `json:"left"`
	// What the game's rules of the spell are.
	Rules game.CastRules `json:"rules"`
}

// Slot describes a spell of the bar, or nil when the character cannot cast
// it.
func (s *Sim) Slot(st *State, id int) *Slot {
	a := s.attack(id, 0)
	if a == nil {
		return nil
	}
	player := st.Player()
	lo, hi := s.reach(player, a)
	slot := &Slot{
		ID: id, APCost: a.cost, RangeMin: lo, RangeMax: hi, CritRate: s.critRate(a, s.StatOf(player)),
		Blocked: s.blocked(st, a), Cooldown: st.cooldown(a), Left: -1, Rules: a.rules,
	}
	if a.rules.PerTurn > 0 {
		slot.Left = a.rules.PerTurn
		if use := st.Uses[id]; use != nil {
			slot.Left = max(0, a.rules.PerTurn-use.Turn)
		}
	}
	return slot
}

// Loss is what a cast would cost an entity: the health points it would lose
// on the lowest and the highest roll, without and with a critical hit. A
// negative loss is a heal.
type Loss struct {
	Entity  int `json:"entity"`
	Min     int `json:"min"`
	Max     int `json:"max"`
	CritMin int `json:"critMin"`
	CritMax int `json:"critMax"`
}

// Shift is where a cast would leave an entity it moves.
type Shift struct {
	Entity int `json:"entity"`
	Cell   int `json:"cell"`
}

// Target is a cell a spell can be aimed at, with what casting there does.
type Target struct {
	Cell int `json:"cell"`
	// Zone is the cells the spell covers from there.
	Zone   []int   `json:"zone"`
	Losses []Loss  `json:"losses"`
	Shifts []Shift `json:"shifts"`
}

// Aim is what the player sees with a spell in hand.
type Aim struct {
	Spell int `json:"spell"`
	// Range lists the cells within the spell's range and shape, and Targets
	// those among them it can really be aimed at.
	Range   []int    `json:"range"`
	Targets []Target `json:"targets"`
	// CanCrit says the spell has a critical hit.
	CanCrit bool `json:"canCrit"`
}

// Aim lists where a spell can be aimed and previews each cast. The effects
// drawn at random are left out of the preview.
func (s *Sim) Aim(st *State, spellID int) *Aim {
	a := s.attack(spellID, 0)
	if a == nil || st.Phase != PhaseFight {
		return nil
	}
	player := st.Player()
	aim := &Aim{Spell: spellID, Range: []int{}, Targets: []Target{}, CanCrit: s.critRate(a, s.StatOf(player)) > 0}
	if s.blocked(st, a) != "" {
		return aim
	}
	for cell := 0; cell < st.Board.Width*st.Board.Height; cell++ {
		reason := s.aimable(st, a, player, cell)
		if reason == BlockedRange || reason == BlockedLine || st.Board.wall(cell) {
			continue
		}
		aim.Range = append(aim.Range, cell)
		if reason != "" {
			continue
		}
		aim.Targets = append(aim.Targets, s.preview(st, a, spellID, cell, aim.CanCrit))
	}
	return aim
}

func (s *Sim) preview(st *State, a *attack, spellID, cell int, canCrit bool) Target {
	target := Target{Cell: cell, Zone: s.shown(st, a, cell), Losses: []Loss{}, Shifts: []Shift{}}
	losses := map[int]*Loss{}
	var order []int
	run := func(crit, high bool, set func(*Loss, int)) *State {
		trial := st.clone()
		c := &casting{root: spellID, crit: crit, roll: func(lo, hi int) int {
			if high {
				return hi
			}
			return lo
		}}
		s.resolve(trial, c, trial.Player().ID, a, cell, 0)
		sums := map[int]int{}
		for _, ev := range c.events {
			switch ev.Type {
			case EventDamage:
				sums[ev.Entity] += ev.Value
			case EventHeal:
				sums[ev.Entity] -= ev.Value
			}
		}
		for id, sum := range sums {
			if losses[id] == nil {
				losses[id] = &Loss{Entity: id}
				order = append(order, id)
			}
			set(losses[id], sum)
		}
		return trial
	}
	after := run(false, false, func(l *Loss, v int) { l.Min = v })
	run(false, true, func(l *Loss, v int) { l.Max = v })
	if canCrit {
		run(true, false, func(l *Loss, v int) { l.CritMin = v })
		run(true, true, func(l *Loss, v int) { l.CritMax = v })
	}
	for _, e := range st.Entities {
		if l := losses[e.ID]; l != nil && (l.Min != 0 || l.Max != 0 || l.CritMin != 0 || l.CritMax != 0) {
			target.Losses = append(target.Losses, *l)
		}
		if moved := after.entity(e.ID); moved != nil && moved.Cell != e.Cell {
			target.Shifts = append(target.Shifts, Shift{Entity: e.ID, Cell: moved.Cell})
		}
	}
	return target
}

// shown is the zone a spell covers when aimed at a cell: the cells of the
// effects the game lists that do not apply to the caster alone.
func (s *Sim) shown(st *State, a *attack, cell int) []int {
	player := st.Player()
	seen := map[int]bool{cell: true}
	zone := []int{cell}
	for _, e := range a.effects {
		if !e.Shown && !a.weapon {
			continue
		}
		if m := readMask(e.Targets); m.caster && !m.ally && !m.enemy && !m.allyOther && !m.anyone {
			continue
		}
		for _, c := range st.Board.zone(e.Zone, player.Cell, cell) {
			if !seen[c] && !st.Board.wall(c) {
				seen[c] = true
				zone = append(zone, c)
			}
		}
	}
	return zone
}
