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

func TestRunesActivateOnCommandAndKeepTheirOwner(t *testing.T) {
	s := newSim(map[int]int{game.StatAP: 12, game.StatMP: 6, 110: 100})
	withMark(s, 201, runeMark, 2, game.SpellEffect{Action: 99, Min: 20, Targets: "A", Zone: point()})
	// An actual Huppermage helper uses a point mark; the walk and the turn
	// change must leave it intact until an activation spell reaches it.
	s.Cat.SpellByID[201].Levels[0].Effects[0].Zone = point()
	detonate := spell(203, game.SpellLevel{RangeMax: 10, Effects: []game.SpellEffect{
		{Action: 2023, Targets: "a,A", Zone: point()},
		{Action: 97, Min: 200, Described: true, Targets: "A", Zone: point()},
	}})
	s.Cat.SpellByID[203] = &detonate
	st := fightAt(s, Dummies{}, [2]int{5, 6}, [2]int{8, 6})
	center := st.Entities[1].Cell
	s.Cast(st, 201, center)
	s.Cast(st, 201, center)
	if len(st.Marks) != 1 {
		t.Fatalf("overwriting one rune left %d marks", len(st.Marks))
	}
	other := st.Marks[0]
	other.ID++
	other.Caster = 3
	st.Marks = append(st.Marks, other)
	s.EndTurn(st)
	aim := s.Aim(st, 203)
	if !slices.ContainsFunc(aim.Targets, func(t Target) bool {
		return t.Cell == center && len(t.Losses) == 1 && t.Losses[0].Min == 40
	}) || len(st.Marks) != 2 {
		t.Fatalf("rune preview mutated or included descriptive-only hits: %+v", st.Marks)
	}
	ev, _ := s.Cast(st, 203, center)
	if lost(ev, 2) != 40 || len(st.Marks) != 1 || st.Marks[0].Caster != 3 {
		t.Fatalf("rune activated incorrectly: %+v %+v", ev, st.Marks)
	}
	s.EndTurn(st)
	if len(st.Marks) != 0 {
		t.Fatal("the second caster's unused rune did not expire")
	}
}

func TestActivatingAnEmptyRuneThroughADelegatedSpell(t *testing.T) {
	s := newSim(map[int]int{game.StatAP: 12, game.StatMP: 6})
	withMark(s, 205, runeMark, 2, game.SpellEffect{Action: actionTeleport, Targets: "a,A,*e7", Zone: point()})
	s.Cat.SpellByID[205].Levels[0].Effects[0].Zone = point()
	delegate := spell(207, game.SpellLevel{RangeMax: 10, Effects: []game.SpellEffect{
		{Action: game.ActionTargetCastsOnCell, Min: 208, Max: 1, Targets: "C", Zone: point()},
	}})
	detonate := spell(208, game.SpellLevel{Effects: []game.SpellEffect{{Action: 2023, Targets: "a,A", Zone: point()}}})
	s.Cat.SpellByID[207], s.Cat.SpellByID[208] = &delegate, &detonate
	st := fightAt(s, Dummies{}, [2]int{10, 6})
	center := st.Board.cell(5, 6)
	s.Cast(st, 205, center)
	s.Move(st, st.Board.cell(6, 6))
	if len(st.Marks) != 1 {
		t.Fatal("walking through a rune consumed it")
	}
	aim := s.Aim(st, 207)
	if !slices.ContainsFunc(aim.Targets, func(t Target) bool {
		return t.Cell == center && len(t.Shifts) == 1 && t.Shifts[0].Entity == 1 && t.Shifts[0].Cell == center
	}) || len(st.Marks) != 1 {
		t.Fatal("preview of the empty rune did not report the teleport")
	}
	events, why := s.Cast(st, 207, center)
	if why != "" || len(st.Marks) != 0 || st.Player().Cell != center || !slices.ContainsFunc(events, func(e Event) bool {
		return e.Type == "mark" && e.Entity == 0 && e.Cell != nil && *e.Cell == center
	}) {
		t.Fatalf("empty aimed tile not preserved through the delegated spell: %q %+v", why, events)
	}
}
