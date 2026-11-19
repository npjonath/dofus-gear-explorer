package fight

import (
	"slices"
	"testing"

	"dofusdb/api/internal/game"
)

func withMark(s *Sim, id, action, duration int, childEffects ...game.SpellEffect) {
	payload := spell(id+1, game.SpellLevel{Effects: childEffects})
	source := spell(id, game.SpellLevel{
		APCost: 1, RangeMax: 12, Effects: []game.SpellEffect{{
			Action: action, Min: payload.ID, Max: 1, Value: 0xca1330,
			Targets: "a,A", Duration: duration, Zone: game.Zone{Shape: "C", Size: 1},
		}},
	})
	s.Cat.SpellByID[id], s.Cat.SpellByID[payload.ID] = &source, &payload
}

func TestGlyphTriggersOncePerFighterAndExpires(t *testing.T) {
	s := newSim(map[int]int{game.StatAP: 6})
	withMark(s, 101, startMark, 2, game.SpellEffect{
		Action: 99, Min: 100, Max: 100, Targets: "A",
		Zone: game.Zone{Shape: "C", Size: 1, Decrease: 10, Steps: 1},
	})
	st := fightAt(s, Dummies{}, [2]int{6, 6}, [2]int{7, 6})
	center := st.Entities[1].Cell
	aim := s.Aim(st, 101)
	for _, at := range aim.Targets {
		if at.Cell == center && len(at.Losses) > 0 {
			t.Fatalf("future glyph damage previewed as an immediate loss: %+v", at)
		}
	}
	if len(st.Marks) != 0 {
		t.Fatal("aim mutated the fight")
	}
	events, reason := s.Cast(st, 101, center)
	if reason != "" || len(st.Marks) != 1 || !ValidMarks(st) || lost(events, 2) != 0 {
		t.Fatalf("cast result: %q %+v %+v", reason, events, st.Marks)
	}
	for turn := 1; turn <= 2; turn++ {
		events, _ = s.EndTurn(st)
		if lost(events, 2) != 100 || lost(events, 3) != 90 {
			t.Fatalf("turn %d glyph hits: %+v", turn, events)
		}
		triggers := slices.DeleteFunc(slices.Clone(events), func(e Event) bool { return e.Type != "mark" })
		if len(triggers) != 2 || triggers[0].Spell != 101 || triggers[0].TriggerSpell != 102 || *triggers[0].Cell != center {
			t.Fatalf("glyph activation events: %+v", triggers)
		}
		if st.TurnDamage != 0 || st.TotalDamage != turn*190 {
			t.Fatalf("damage crossed turn boundary: %d, %d", st.TurnDamage, st.TotalDamage)
		}
	}
	if len(st.Marks) > 0 {
		t.Fatal("expired glyph persisted")
	}
}

func TestTrapSpringsOnEntryInterruptsWalkAndHitsItsEntireZone(t *testing.T) {
	s := newSim(map[int]int{game.StatAP: 6, game.StatMP: 6})
	withMark(s, 103, trapMark, 0, game.SpellEffect{
		Action: 99, Min: 50, Max: 50, Targets: "a,A", Zone: game.Zone{Shape: "C", Size: 1},
	})
	st := fightAt(s, Dummies{}, [2]int{6, 7})
	center := st.Board.cell(6, 6)
	if ev, why := s.Cast(st, 103, center); why != "" || len(st.Marks) != 1 || lost(ev, 1)+lost(ev, 2) != 0 {
		t.Fatalf("trap triggered on placement: %s %+v", why, ev)
	}
	events, why := s.Move(st, st.Board.cell(7, 6))
	if why != "" || len(st.Marks) != 0 || st.Player().Cell != st.Board.cell(5, 6) || st.MPUsed != 2 {
		t.Fatalf("trap did not interrupt the walk: %s %+v", why, st)
	}
	if lost(events, 1) != 50 || lost(events, 2) != 50 || events[0].Type != EventMove || events[1].Type != "mark" {
		t.Fatalf("trap hit wrong targets or in wrong order: %+v", events)
	}
}

func TestAuraBuffEndsOnExitAndAppliesAgainOnReentry(t *testing.T) {
	s := newSim(map[int]int{game.StatAP: 6, game.StatMP: 6})
	withMark(s, 105, auraMark, 3, game.SpellEffect{
		Action: 213, Stat: 34, Sign: 1, Min: 15, Targets: "a", Duration: 3, Zone: game.Zone{Shape: "C", Size: 1},
	})
	st := fightAt(s, Dummies{}, [2]int{10, 6})
	_, why := s.Cast(st, 105, st.Board.cell(4, 6))
	check := func(expected int) {
		t.Helper()
		if got := s.StatOf(st.Player())(34); got != expected {
			t.Fatalf("aura resistance=%d, want %d, buffs=%+v", got, expected, st.Player().Buffs)
		}
	}
	if why != "" {
		t.Fatal(why)
	}
	check(15)
	s.Move(st, st.Board.cell(2, 6))
	check(0)
	s.Move(st, st.Board.cell(3, 6))
	check(15)
	s.EndTurn(st)
	check(15)
}

func TestMarkStateValidationAndSharedWorkBudget(t *testing.T) {
	s := newSim(map[int]int{game.StatAP: 6})
	withMark(s, 107, startMark, 2, game.SpellEffect{Action: 99, Min: 50, Targets: "A", Zone: point()})
	st := fightAt(s, Dummies{}, [2]int{7, 6})
	s.Cast(st, 107, st.Entities[1].Cell)
	if !ValidMarks(st) {
		t.Fatal("valid mark rejected")
	}
	c := s.casting(0, false, false)
	*c.work = 0
	s.markTurn(st, c, 2, true)
	if len(c.events) > 0 {
		t.Fatalf("activation exceeded budget: %+v", c.events)
	}
	st.Marks[0].Cells = []int{st.Board.Width * st.Board.Height}
	if ValidMarks(st) {
		t.Fatal("out-of-range glyph accepted")
	}
}

func TestGlyphLogsTheThreeDodgedPointsAsAPOrMP(t *testing.T) {
	for _, tc := range []struct{ action, stat int }{{1079, game.StatAP}, {1080, game.StatMP}} {
		s := newSim(map[int]int{game.StatAP: 6})
		withMark(s, 109, auraMark, 2, game.SpellEffect{Action: tc.action, Min: 3, Targets: "A", Duration: 2, Zone: point()})
		st := fightAt(s, Dummies{}, [2]int{5, 6})
		events, reason := s.Cast(st, 109, st.Entities[1].Cell)
		found := slices.ContainsFunc(events, func(event Event) bool {
			return event.Type == "dodge" && event.Entity == st.Entities[1].ID && event.Value == 3 && event.Stat != nil && *event.Stat == tc.stat
		})
		if !found {
			t.Fatalf("expected the dummy to dodge all 3 points of stat %d: %q, %+v", tc.stat, reason, events)
		}
	}
}

func TestEquipmentBonusesScopedToTrapAndGlyphDamage(t *testing.T) {
	s := newSim(map[int]int{game.StatAP: 6, game.StatMP: 6, 69: 100, 70: 15, 106: 50})
	hit := game.SpellEffect{Action: 99, Min: 100, Targets: "A", Zone: game.Zone{Shape: "C", Size: 1}}
	withMark(s, 111, startMark, 1, hit)
	withMark(s, 113, trapMark, 0, hit)
	st := fightAt(s, Dummies{}, [2]int{6, 7})
	center := st.Board.cell(6, 6)
	s.Cast(st, 111, center)
	turn, _ := s.EndTurn(st)
	if damage := lost(turn, 2); damage != 150 {
		t.Fatalf("glyph got %d damage, want 150", damage)
	}
	s.Cast(st, 113, center)
	move, _ := s.Move(st, st.Board.cell(7, 6))
	if damage := lost(move, 2); damage != 215 {
		t.Fatalf("trap got %d damage, want 215", damage)
	}
}
