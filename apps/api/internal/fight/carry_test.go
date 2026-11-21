package fight

import (
	"testing"

	"dofusdb/api/internal/game"
)

func TestPickupMoveAndThrow(t *testing.T) {
	s := newSim(map[int]int{game.StatAP: 12, game.StatMP: 5})
	const id = 12787
	sp := spell(id, game.SpellLevel{APCost: 1, RangeMin: 1, RangeMax: 1,
		Cast: game.CastRules{Line: true, TakenCell: true},
		Effects: []game.SpellEffect{
			{Action: pickupAction, Targets: "g,A,*e3", Zone: point()},
			{Action: actionSpellRange, Min: id, Value: 3, Described: true, Targets: "C", Zone: point()},
			{Action: throwAction, Targets: "g,A,*E3", Zone: point()},
		}})
	s.Cat.SpellByID[id] = &sp
	st := fightAt(s, Dummies{}, [2]int{4, 6}, [2]int{6, 6})
	player, rider := st.Player(), st.entity(2)
	if _, reason := s.Cast(st, id, st.Board.cell(3, 5)); reason != BlockedCell {
		t.Fatalf("picked up an empty square: %q", reason)
	}
	aim := s.Aim(st, id)
	if aim == nil || rider.CarriedBy != 0 {
		t.Fatal("preview changed the live stack")
	}
	if _, reason := s.Cast(st, id, rider.Cell); reason != "" || rider.CarriedBy != player.ID || rider.Cell != player.Cell ||
		!player.inState(carrierState) || !rider.inState(carriedState) || st.at(player.Cell) != rider || !ValidPositions(st) {
		t.Fatalf("could not lift dummy: %q, %+v", reason, st.Entities)
	}
	if slot := s.Slot(st, id); slot.RangeMax != 4 || !slot.Rules.FreeCell || slot.Rules.TakenCell {
		t.Fatalf("throw rules: %+v", slot)
	}
	if _, reason := s.Cast(st, id, st.entity(3).Cell); reason != BlockedCell {
		t.Fatalf("threw onto another fighter: %q", reason)
	}
	if _, reason := s.Move(st, st.Board.cell(3, 7)); reason != "" || player.Cell != rider.Cell || !ValidPositions(st) {
		t.Fatalf("passenger did not follow a step: %q", reason)
	}
	landing := st.Board.cell(6, 7)
	ev, reason := s.Cast(st, id, landing)
	if reason != "" || rider.Cell != landing || rider.CarriedBy != 0 || player.inState(carrierState) || rider.inState(carriedState) || !ValidPositions(st) {
		t.Fatalf("did not land on the aimed cell: %q, %+v", reason, st.Entities)
	}
	var threw bool
	for _, e := range ev {
		if e.Type == EventMove && e.Action == throwAction && e.Carrier == player.ID && e.Entity == rider.ID {
			threw = true
		}
	}
	if !threw || st.APUsed != 2 || st.MPUsed != 1 {
		t.Fatalf("events/resources: %+v %+v", ev, st)
	}
	if s.Slot(st, id).RangeMax != 1 {
		t.Fatalf("throw range stuck after landing")
	}
}

func TestCarryCellsAndTargetMasks(t *testing.T) {
	s := newSim(map[int]int{game.StatAP: 10})
	st := fightAt(s, Dummies{}, [2]int{4, 6}, [2]int{5, 6})
	hero, carried := st.Player(), st.entity(2)
	carried.Cell, carried.CarriedBy = hero.Cell, hero.ID
	carried.Buffs = []Buff{{Kind: BuffState, Ref: carriedState, Turns: -1}}
	hero.Buffs = []Buff{{Kind: BuffState, Ref: carrierState, Turns: -1}}
	row := game.SpellEffect{Action: 99, Min: 20, Max: 20, Targets: "A", Zone: game.Zone{Shape: "C", Size: 2, ExcludeCarried: true}}
	attack := &attack{id: 50000, effects: []game.SpellEffect{row}}
	apply := func() []Event {
		c := s.casting(attack.id, false, false)
		s.resolve(st, c, hero.ID, hero.ID, attack, hero.Cell, 0)
		return c.events
	}
	if got := lost(apply(), carried.ID); got != 0 {
		t.Fatalf("hit a carried target excluded from the area: %d", got)
	}
	attack.effects[0].Zone.ExcludeCarried = false
	if got := lost(apply(), carried.ID); got != 20 {
		t.Fatalf("area missing carried target: %d", got)
	}
	if !ValidPositions(st) {
		t.Fatal("valid stack refused")
	}
	st.entity(3).Cell = hero.Cell
	if ValidPositions(st) {
		t.Fatal("third occupant accepted")
	}
	st.entity(3).Cell = st.Board.cell(5, 6)
	hero.CarriedBy = carried.ID
	if ValidPositions(st) {
		t.Fatal("cyclic stack accepted")
	}
}
