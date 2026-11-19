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
	a := s.spell(st, id)
	if a == nil {
		return nil
	}
	player := st.Active()
	lo, hi := s.reach(player, a)
	slot := &Slot{
		ID: id, APCost: a.cost, RangeMin: lo, RangeMax: hi, CritRate: s.critRate(a, s.StatOf(player)),
		Blocked: s.blocked(st, a), Cooldown: st.cooldown(a), Left: -1, Rules: a.rules,
	}
	if a.rules.PerTurn > 0 {
		slot.Left = a.rules.PerTurn
		if use := st.counters()[id]; use != nil {
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
	a := s.spell(st, spellID)
	if a == nil || st.Phase != PhaseFight {
		return nil
	}
	player := st.Active()
	aim := &Aim{Spell: spellID, Range: []int{}, Targets: []Target{}, CanCrit: s.critRate(a, s.StatOf(player)) > 0}
	if s.blocked(st, a) != "" {
		return aim
	}
	// One budget for the whole preview, every cell and every roll.
	work := maxAimWork
	for cell := 0; cell < st.Board.Width*st.Board.Height && work > 0; cell++ {
		reason := s.aimable(st, a, player, cell)
		if reason == BlockedRange || reason == BlockedLine || st.Board.wall(cell) {
			continue
		}
		aim.Range = append(aim.Range, cell)
		if reason != "" {
			continue
		}
		aim.Targets = append(aim.Targets, s.preview(st, a, spellID, cell, aim.CanCrit, &work))
	}
	return aim
}

func (s *Sim) preview(st *State, a *attack, spellID, cell int, canCrit bool, work *int) Target {
	target := Target{Cell: cell, Zone: s.shown(st, a, cell), Losses: []Loss{}, Shifts: []Shift{}}
	// run plays the cast on a copy of the fight, with the lowest or the
	// highest rolls and one outcome of what the spell draws at random, and
	// returns what each entity lost and where the cast left everyone.
	run := func(crit, high bool, outcome int) (map[int]int, *State) {
		trial := st.clone()
		top := true
		c := &casting{root: spellID, crit: crit, work: work,
			roll: func(lo, hi int) int {
				if high {
					return hi
				}
				return lo
			},
			// The spell's own draw is the outcome asked for; those of the
			// spells it casts in turn are left out.
			pick: func([]float64) int {
				if top {
					top = false
					return outcome
				}
				return -1
			},
		}
		id := trial.Active().ID
		s.resolve(trial, c, id, id, a, cell, 0)
		if crit {
			s.fire(trial, c, id, id, 0, "CC")
		}
		sums := map[int]int{}
		for _, ev := range c.events {
			switch ev.Type {
			case EventDamage:
				sums[ev.Entity] += ev.Value
			case EventHeal:
				sums[ev.Entity] -= ev.Value
			}
		}
		return sums, trial
	}
	// span gives, per entity, the least and the most it loses over every
	// outcome the spell can draw.
	span := func(crit bool) (lows, highs map[int]int, after *State) {
		effects := a.effects
		if crit && len(a.critEffects) > 0 {
			effects = a.critEffects
		}
		groups, _ := outcomes(effects)
		lows, highs = map[int]int{}, map[int]int{}
		for outcome := 0; outcome < max(1, len(groups)); outcome++ {
			low, trial := run(crit, false, outcome)
			high, _ := run(crit, true, outcome)
			if after == nil {
				after = trial
			}
			for _, e := range st.Entities {
				if outcome == 0 || low[e.ID] < lows[e.ID] {
					lows[e.ID] = low[e.ID]
				}
				if outcome == 0 || high[e.ID] > highs[e.ID] {
					highs[e.ID] = high[e.ID]
				}
			}
		}
		return lows, highs, after
	}
	lows, highs, after := span(false)
	critLows, critHighs := map[int]int{}, map[int]int{}
	if canCrit {
		critLows, critHighs, _ = span(true)
	}
	for _, e := range st.Entities {
		l := Loss{Entity: e.ID, Min: lows[e.ID], Max: highs[e.ID], CritMin: critLows[e.ID], CritMax: critHighs[e.ID]}
		if l.Min != 0 || l.Max != 0 || l.CritMin != 0 || l.CritMax != 0 {
			target.Losses = append(target.Losses, l)
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
	player := st.Active()
	seen := map[int]bool{cell: true}
	zone := []int{cell}
	for _, e := range a.effects {
		if !e.Shown && !a.weapon {
			continue
		}
		if m := readMask(e.Targets); m.caster && !m.ally && !m.enemy && !m.allyOther && !m.anyone {
			continue
		}
		// An effect kept for a monster no fight has is there for the
		// spell's description: the one that applies comes from a spell
		// cast in turn, which may cover other cells.
		if game.CastsSpell(e.Action) {
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
