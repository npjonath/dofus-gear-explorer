package httpapi

import (
	"cmp"
	"slices"
	"testing"

	"dofusdb/api/internal/fight"
)

// The two room configurations allow short-range, movement and conditional
// spells to act on a legal target. They exercise the ordinary HTTP dice.
func TestSpeedrushSpellCatalogWithInstalledGameData(t *testing.T) {
	const village, astrub = 99222029, 192419850
	seen := map[int]bool{}
	for _, breed := range []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 20} {
		r := newSpeedrushReplay(t, breed, village, 3, 10000)
		r.req.Level = 200
		setup := func(room int) fightDTO {
			request := r.req
			request.Action, request.State = nil, nil
			request.Dummies = &fight.Dummies{MapID: room, Count: 3, Allies: 1, HP: 10000}
			var v fightDTO
			r.post("/api/fight", request, &v)
			return v
		}
		begin := func(v fightDTO, cell int) fightDTO {
			r.view = v
			if cell != v.State.Player().Cell {
				r.act(fightAction{Type: "place", Entity: 1, Cell: cell})
			}
			r.act(fightAction{Type: "begin"})
			return r.view
		}
		first := begin(setup(village), 113)
		var alternates []fightDTO
		for _, slot := range first.Spells {
			if seen[slot.ID] {
				continue
			}
			seen[slot.ID] = true
			r.view = first
			// Pandawa's five carry-dependent spells need a passenger.
			if !tryCatalogSpell(t, r, slot.ID) {
				if alternates == nil {
					other := setup(astrub)
					for _, cell := range other.State.Board.Blue {
						occupied := slices.ContainsFunc(other.State.Entities, func(e fight.Entity) bool { return e.Cell == cell && e.Kind != fight.KindPlayer })
						if !occupied {
							alternates = append(alternates, begin(other, cell))
						}
					}
				}
				ok := false
				for _, ready := range alternates {
					r.view = ready
					if breed == 12 && slot.Blocked != "" {
						const pickup = 12787
						aim := r.aim(pickup)
						if len(aim.Targets) > 0 {
							r.act(fightAction{Type: "cast", Spell: pickup, Cell: aim.Targets[0].Cell})
						}
					}
					if tryCatalogSpell(t, r, slot.ID) {
						ok = true
						break
					}
				}
				if !ok {
					t.Errorf("class %d, %s (%d): no successful legal cast", breed, slot.Name, slot.ID)
				}
			}
		}
	}
	if len(seen) != 863 {
		t.Errorf("3.7.14.18 should expose 862 spells and one weapon, got %d entries", len(seen))
	}
}

func tryCatalogSpell(t *testing.T, r *speedrushReplay, id int) bool {
	t.Helper()
	for range 8 {
		idx := slices.IndexFunc(r.view.Spells, func(s fightSlotDTO) bool { return s.ID == id })
		if idx < 0 {
			return false
		}
		why := r.view.Spells[idx].Blocked
		if why == "" {
			break
		}
		if why != "cooldown" {
			return false
		}
		r.act(fightAction{Type: "end"})
	}
	targets := r.aim(id).Targets
	if len(targets) == 0 {
		return false
	}
	// Prioritize damage to a dummy, then movement and other affected fighters.
	score := func(v fight.Target) int {
		damage := 0
		for _, loss := range v.Losses {
			if loss.Entity != 1 {
				damage += max(0, loss.Max, loss.CritMax)
			}
		}
		return damage
	}
	best := slices.MaxFunc(targets, func(a, b fight.Target) int {
		return cmp.Or(cmp.Compare(score(a), score(b)), cmp.Compare(len(a.Shifts), len(b.Shifts)), cmp.Compare(len(a.Losses), len(b.Losses)))
	})
	r.act(fightAction{Type: "cast", Spell: id, Cell: best.Cell})
	check := func() {
		for _, e := range r.view.Events {
			if e.Type == fight.EventSkipped {
				t.Errorf("spell %d skipped action %d", id, e.Action)
			}
		}
	}
	check()
	r.act(fightAction{Type: "end"})
	check()
	return true
}
