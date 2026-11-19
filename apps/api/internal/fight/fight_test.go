package fight

import (
	"math/rand/v2"
	"testing"

	"dofusdb/api/internal/game"
)

// Spells of the tests. Each has one grade, cast by a character of any level.
const (
	spellBolt = iota + 1
	spellBlast
	spellShove
	spellHaste
	spellMark
	spellFinisher
	spellChain
	spellSpark
	spellSharpen
	spellVenom
	spellDice
	spellOpening
	spellFocus
	spellRelic
	spellRelicEcho
)

const (
	stateFocused = 78
	stateReady   = 79
)

const stateMarked = 77

func point() game.Zone { return game.Zone{Shape: "P"} }

func spell(id int, level game.SpellLevel) game.Spell {
	level.Grade = 1
	return game.Spell{ID: id, Name: game.Texts{"en": "spell"}, Levels: []game.SpellLevel{level}}
}

// newSim builds a simulator over the test spells, for a level 200 character
// with the given characteristics.
func newSim(stats map[int]int) *Sim {
	fire := func(lo, hi int, zone game.Zone) game.SpellEffect {
		return game.SpellEffect{Action: 99, Min: lo, Max: hi, Targets: "A", Zone: zone, Shown: true}
	}
	spells := []game.Spell{
		// 100 Fire damage at range 1 to 6, in sight, twice a turn and once per target.
		spell(spellBolt, game.SpellLevel{APCost: 3, RangeMin: 1, RangeMax: 6, CritChance: 10,
			Cast:        game.CastRules{Sight: true, PerTurn: 2, PerTarget: 1},
			Effects:     []game.SpellEffect{fire(100, 100, point())},
			CritEffects: []game.SpellEffect{fire(150, 150, point())}}),
		// 100 to 120 Fire damage in a circle of two cells, every other turn.
		spell(spellBlast, game.SpellLevel{APCost: 4, RangeMax: 6, Cast: game.CastRules{Interval: 2},
			Effects: []game.SpellEffect{fire(100, 120, game.Zone{Shape: "C", Size: 2, Decrease: 10, Steps: 4})}}),
		// Pushes three cells.
		spell(spellShove, game.SpellLevel{APCost: 2, RangeMin: 1, RangeMax: 1,
			Effects: []game.SpellEffect{{Action: actionPush, Min: 3, Targets: "A", Zone: point(), Shown: true}}}),
		// Two AP for two turns, to the caster wherever the spell is aimed.
		spell(spellHaste, game.SpellLevel{APCost: 1, RangeMax: 3, Cast: game.CastRules{MaxStack: 1},
			Effects: []game.SpellEffect{{Action: 111, Min: 2, Stat: game.StatAP, Sign: 1, Duration: 2, Targets: "C", Zone: point(), Shown: true}}}),
		// Marks an enemy for two turns.
		spell(spellMark, game.SpellLevel{APCost: 1, RangeMax: 6,
			Effects: []game.SpellEffect{{Action: actionState, Value: stateMarked, Duration: 2, Targets: "A", Zone: point()}}}),
		// 50 Fire damage, or 200 on a marked enemy, whose mark it removes.
		// The mark is read as the spell is cast, for every effect.
		spell(spellFinisher, game.SpellLevel{APCost: 2, RangeMax: 6, Effects: []game.SpellEffect{
			{Action: 99, Min: 50, Max: 50, Targets: "A,e77", Zone: point(), Shown: true},
			{Action: 99, Min: 200, Max: 200, Targets: "A,E77", Zone: point(), Shown: true},
			{Action: actionRemoveState, Value: stateMarked, Targets: "A", Zone: point()},
		}}),
		// Deals its damage through another spell, cast on the target.
		spell(spellChain, game.SpellLevel{APCost: 2, RangeMax: 6, Effects: []game.SpellEffect{
			{Action: game.ActionCastSpell, Min: spellSpark, Max: 1, Targets: "A", Zone: point()},
			// Summons are not simulated: the game lists this effect, so the fight says so.
			{Action: 181, Min: 1, Targets: "C", Zone: point(), Shown: true},
		}}),
		spell(spellSpark, game.SpellLevel{Effects: []game.SpellEffect{fire(30, 30, point())}}),
		// The bolt's base damage rises by 20 for the turn.
		spell(spellSharpen, game.SpellLevel{APCost: 1, Effects: []game.SpellEffect{
			{Action: actionSpellBase, Min: spellBolt, Value: 20, Duration: 1, Targets: "C", Zone: point()},
		}}),
	}
	spells = append(spells,
		// 40 Fire damage at the start of each of the target's two next turns.
		spell(spellVenom, game.SpellLevel{APCost: 2, RangeMax: 6, Effects: []game.SpellEffect{
			{Action: 99, Min: 40, Max: 40, Targets: "A", Duration: 2, Trigger: "TB", Zone: point(), Shown: true},
		}}),
		// One of three hits, drawn at random: 10, 20 or 30.
		spell(spellDice, game.SpellLevel{APCost: 2, RangeMax: 6, Effects: []game.SpellEffect{
			{Action: 99, Min: 10, Max: 10, Targets: "A", Zone: point(), Random: 33.3, Group: 1, Shown: true},
			{Action: 99, Min: 20, Max: 20, Targets: "A", Zone: point(), Random: 33.3, Group: 2, Shown: true},
			{Action: 99, Min: 30, Max: 30, Targets: "A", Zone: point(), Random: 33.4, Group: 3, Shown: true},
		}}),
		// A class's opening: the caster is ready for the whole fight, each
		// time it focuses it gains 2 AP for the turn, and the bolt costs it
		// 1 AP less.
		spell(spellOpening, game.SpellLevel{Effects: []game.SpellEffect{
			{Action: actionState, Value: stateReady, Duration: -1, Targets: "C", Zone: point()},
			{Action: 111, Min: 2, Stat: game.StatAP, Sign: 1, Duration: 1, Targets: "C", Trigger: "EON78", TriggerTurns: 63, Zone: point()},
			{Action: modCostLess, Min: spellBolt, Value: 1, Duration: -1, Targets: "C", Zone: point()},
		}}),
		// Focuses the caster for the turn; only a ready caster can.
		spell(spellFocus, game.SpellLevel{APCost: 1, Effects: []game.SpellEffect{
			{Action: actionState, Value: stateFocused, Duration: 1, Targets: "C,*E79", Zone: point()},
		}}),
		// An item's passive: 10 % final damage on the first turn, 3 % on the
		// second, and from the second turn on its echo at each turn's start.
		spell(spellRelic, game.SpellLevel{Effects: []game.SpellEffect{
			{Action: 1171, Min: 10, Stat: statFinalDamage, Sign: 1, Duration: 1, Targets: "C", Zone: point()},
			{Action: 1171, Min: 3, Stat: statFinalDamage, Sign: 1, Duration: 1, Delay: 1, Targets: "C", Zone: point()},
			{Action: game.ActionTargetCasts, Min: spellRelicEcho, Max: 1, Targets: "C", Trigger: "TB", TriggerTurns: 63, Delay: 1, Zone: point()},
		}}),
		spell(spellRelicEcho, game.SpellLevel{Effects: []game.SpellEffect{
			{Action: 111, Min: 1, Stat: game.StatAP, Sign: 1, Duration: 1, Targets: "C", Zone: point()},
		}}),
	)
	spells[spellOpening-1].StartsFor = 1
	for _, id := range []int{spellSpark, spellOpening, spellRelic, spellRelicEcho} {
		spells[id-1].BreedID = game.BreedTriggered
	}
	if stats[game.StatAP] == 0 {
		stats[game.StatAP] = 12
	}
	if stats[game.StatMP] == 0 {
		stats[game.StatMP] = 6
	}
	stats[game.StatHP] = 3000
	return &Sim{
		Cat:   game.NewCatalog(&game.Dataset{Spells: spells}),
		Stat:  func(id int) int { return stats[id] },
		Level: 200,
		Rand:  rand.New(rand.NewPCG(1, 2)),
	}
}

// fightAt starts a fight on an empty 13 x 13 board, the player at (3, 6) and
// each dummy at the given cell.
func fightAt(s *Sim, d Dummies, cells ...[2]int) *State {
	d.Count = len(cells)
	if d.HP == 0 {
		d.HP = 10000
	}
	st := s.Start(d)
	st.Board.Walls = nil
	for i, at := range cells {
		st.Entities[i+1].Cell = st.Board.cell(at[0], at[1])
	}
	s.Begin(st)
	return st
}

func lost(events []Event, entity int) (total int) {
	for _, ev := range events {
		if ev.Type == EventDamage && ev.Entity == entity {
			total += ev.Value
		}
	}
	return total
}

func TestSightStopsAtWallsAndEntities(t *testing.T) {
	b := Board{Width: 13, Height: 13}
	if got := b.between(b.cell(0, 0), b.cell(3, 0)); len(got) != 2 || got[0] != b.cell(1, 0) || got[1] != b.cell(2, 0) {
		t.Errorf("straight line crosses %v", got)
	}
	// A diagonal passes through the corners: it only crosses the cells on it.
	if got := b.between(b.cell(0, 0), b.cell(3, 3)); len(got) != 2 || got[0] != b.cell(1, 1) {
		t.Errorf("diagonal crosses %v", got)
	}
	s := newSim(map[int]int{})
	st := fightAt(s, Dummies{}, [2]int{6, 6}, [2]int{8, 6})
	if !st.sees(st.Player().Cell, st.Board.cell(6, 6)) {
		t.Error("the first dummy is in sight")
	}
	if st.sees(st.Player().Cell, st.Board.cell(8, 6)) {
		t.Error("the second dummy stands behind the first")
	}
	st.Board.Walls = []int{st.Board.cell(4, 6)}
	if _, reason := s.Cast(st, spellBolt, st.Board.cell(6, 6)); reason != BlockedSight {
		t.Errorf("cast through a wall: %q", reason)
	}
}

func TestZones(t *testing.T) {
	b := Board{Width: 13, Height: 13}
	centre, caster := b.cell(6, 6), b.cell(3, 6)
	for _, c := range []struct {
		zone game.Zone
		want int
	}{
		{game.Zone{Shape: "P"}, 1},
		{game.Zone{Shape: "C", Size: 2}, 13},
		{game.Zone{Shape: "C", Size: 2, Min: 1}, 12},
		{game.Zone{Shape: "X", Size: 1}, 5},
		{game.Zone{Shape: "Q", Size: 2}, 8},
		{game.Zone{Shape: "L", Size: 2}, 3},
		{game.Zone{Shape: "T", Size: 1}, 3},
		{game.Zone{Shape: "G", Size: 1}, 9},
	} {
		if got := b.zone(c.zone, caster, centre); len(got) != c.want {
			t.Errorf("zone %+v covers %d cells, want %d", c.zone, len(got), c.want)
		}
	}
	// A line runs away from the caster, and a zone is cut at the board's edge.
	if line := b.zone(game.Zone{Shape: "L", Size: 2}, caster, centre); line[2] != b.cell(8, 6) {
		t.Errorf("line = %v", line)
	}
	// Another runs from the caster to the aimed cell.
	if reach := b.zone(game.Zone{Shape: "l", Size: 1, Min: 63}, caster, centre); len(reach) != 3 || reach[0] != b.cell(4, 6) || reach[2] != centre {
		t.Errorf("line from the caster = %v", reach)
	}
	if corner := b.zone(game.Zone{Shape: "C", Size: 1}, caster, b.cell(0, 0)); len(corner) != 3 {
		t.Errorf("circle in a corner covers %d cells", len(corner))
	}
	// A zone as wide as the map covers the board and no more, nearest first,
	// however long and thin the board is.
	thin := Board{Width: MaxSide, Height: 1}
	all := thin.zone(game.Zone{Shape: "a", Size: 1 << 30}, 0, thin.cell(5, 0))
	if len(all) != MaxSide || all[0] != thin.cell(5, 0) {
		t.Errorf("whole-map zone on a thin board covers %d cells from %d", len(all), all[0])
	}
	if line := thin.zone(game.Zone{Shape: "L", Size: 1 << 30}, 0, thin.cell(5, 0)); len(line) != MaxSide-5 {
		t.Errorf("endless line covers %d cells", len(line))
	}
}

func TestCastCostsLimitsAndCooldowns(t *testing.T) {
	s := newSim(map[int]int{game.StatAP: 9})
	st := fightAt(s, Dummies{}, [2]int{5, 6}, [2]int{3, 8})
	near, side := st.Board.cell(5, 6), st.Board.cell(3, 8)
	if _, reason := s.Cast(st, spellBolt, st.Board.cell(11, 6)); reason != BlockedRange {
		t.Errorf("out of range: %q", reason)
	}
	if _, reason := s.Cast(st, spellBolt, near); reason != "" {
		t.Fatalf("first cast refused: %q", reason)
	}
	if ap, _ := s.Points(st); ap != 6 {
		t.Errorf("AP left = %d, want 6", ap)
	}
	if _, reason := s.Cast(st, spellBolt, near); reason != BlockedPerTarget {
		t.Errorf("second cast on the same target: %q", reason)
	}
	if _, reason := s.Cast(st, spellBolt, side); reason != "" {
		t.Errorf("cast on another target refused: %q", reason)
	}
	if slot := s.Slot(st, spellBolt); slot.Blocked != BlockedPerTurn || slot.Left != 0 {
		t.Errorf("after two casts: %+v", slot)
	}
	// 3 AP left: the blast costs 4.
	if slot := s.Slot(st, spellBlast); slot.Blocked != BlockedAP {
		t.Errorf("blast with 3 AP: %+v", slot)
	}
	s.EndTurn(st)
	if _, reason := s.Cast(st, spellBlast, near); reason != "" {
		t.Fatalf("blast refused: %q", reason)
	}
	s.EndTurn(st)
	if slot := s.Slot(st, spellBlast); slot.Blocked != BlockedCooldown || slot.Cooldown != 1 {
		t.Errorf("blast the turn after: %+v", slot)
	}
	s.EndTurn(st)
	if slot := s.Slot(st, spellBlast); slot.Blocked != "" {
		t.Errorf("blast two turns after: %+v", slot)
	}
}

func TestDamageMeetsResistancesAndFallsOffInAZone(t *testing.T) {
	// 100 base with 100 Intelligence: 200. Never critical.
	s := newSim(map[int]int{game.StatIntel: 100, game.StatCritical: -100})
	resist := Dummies{Stats: map[int]int{34: 20, 55: 10}}
	st := fightAt(s, resist, [2]int{5, 6})
	events, _ := s.Cast(st, spellBolt, st.Board.cell(5, 6))
	// The fixed resistance comes off first: (200 - 10) x 0.8.
	if got := lost(events, 2); got != 152 {
		t.Errorf("damage against resistances = %d, want 152", got)
	}
	if st.TotalDamage != 152 || st.Entities[1].HP != 10000-152 {
		t.Errorf("fight after the hit: total %d, HP %d", st.TotalDamage, st.Entities[1].HP)
	}

	// A critical hit takes the critical line, the bonus and the resistance.
	s = newSim(map[int]int{game.StatIntel: 100, game.StatCritical: 100, game.StatCritDamage: 40})
	st = fightAt(s, Dummies{Stats: map[int]int{statCriticalResis: 15}}, [2]int{5, 6})
	events, _ = s.Cast(st, spellBolt, st.Board.cell(5, 6))
	if got := lost(events, 2); got != 150*2+40-15 {
		t.Errorf("critical hit = %d, want %d", got, 150*2+40-15)
	}

	// In a zone, each cell away from the centre costs 10 % of the hit.
	s = newSim(map[int]int{game.StatIntel: 100})
	st = fightAt(s, Dummies{}, [2]int{6, 6}, [2]int{7, 6}, [2]int{6, 8}, [2]int{9, 6})
	aim := s.Aim(st, spellBlast)
	var centre *Target
	for i := range aim.Targets {
		if aim.Targets[i].Cell == st.Board.cell(6, 6) {
			centre = &aim.Targets[i]
		}
	}
	if centre == nil || len(centre.Zone) != 13 || len(centre.Losses) != 3 {
		t.Fatalf("preview at the centre: %+v", centre)
	}
	want := []Loss{{Entity: 2, Min: 200, Max: 240}, {Entity: 3, Min: 180, Max: 216}, {Entity: 4, Min: 160, Max: 192}}
	for i, loss := range centre.Losses {
		if loss != want[i] {
			t.Errorf("loss %d = %+v, want %+v", i, loss, want[i])
		}
	}
}

func TestPushbackDamage(t *testing.T) {
	// At level 200 with 90 Pushback damage, a cell left unpushed is worth
	// (100 + 32 + 90) / 4 = 55.
	s := newSim(map[int]int{game.StatPushDamage: 90})
	st := fightAt(s, Dummies{}, [2]int{4, 6}, [2]int{6, 6})
	events, reason := s.Cast(st, spellShove, st.Board.cell(4, 6))
	if reason != "" {
		t.Fatalf("shove refused: %q", reason)
	}
	// Pushed one cell out of three, then stopped by the other dummy, which
	// takes half.
	if cell := st.Entities[1].Cell; cell != st.Board.cell(5, 6) {
		t.Errorf("pushed to %d, want %d", cell, st.Board.cell(5, 6))
	}
	if got := lost(events, 2); got != 111 {
		t.Errorf("pushback damage = %d, want 111", got)
	}
	if got := lost(events, 3); got != 55 {
		t.Errorf("damage to the one behind = %d, want 55", got)
	}

	// With room, the push deals nothing, and the preview shows where it ends.
	st = fightAt(s, Dummies{}, [2]int{4, 6})
	var onDummy *Target
	aim := s.Aim(st, spellShove)
	for i := range aim.Targets {
		if aim.Targets[i].Cell == st.Board.cell(4, 6) {
			onDummy = &aim.Targets[i]
		}
	}
	if onDummy == nil || len(onDummy.Losses) != 0 {
		t.Fatalf("preview of a free push: %+v", aim.Targets)
	}
	if shifts := onDummy.Shifts; len(shifts) != 1 || shifts[0].Cell != st.Board.cell(7, 6) {
		t.Errorf("preview shifts = %+v", shifts)
	}
}

func TestLastingEffectsEndWithTheirTurns(t *testing.T) {
	s := newSim(map[int]int{game.StatAP: 6})
	st := fightAt(s, Dummies{}, [2]int{9, 6})
	// Aimed at an empty cell, the spell still reaches its caster.
	if _, reason := s.Cast(st, spellHaste, st.Board.cell(4, 6)); reason != "" {
		t.Fatalf("haste refused: %q", reason)
	}
	if ap, _ := s.Points(st); ap != 6-1+2 {
		t.Errorf("AP after haste = %d, want 7", ap)
	}
	// It does not pile up on itself.
	s.Cast(st, spellHaste, st.Board.cell(4, 6))
	if ap, _ := s.Points(st); ap != 6-2+2 {
		t.Errorf("AP after a second haste = %d, want 6", ap)
	}
	s.EndTurn(st)
	if ap, _ := s.Points(st); ap != 8 {
		t.Errorf("AP the turn after = %d, want 8", ap)
	}
	s.EndTurn(st)
	if ap, _ := s.Points(st); ap != 6 {
		t.Errorf("AP once it has ended = %d, want 6", ap)
	}
}

func TestStatesSubSpellsAndBoosts(t *testing.T) {
	s := newSim(map[int]int{game.StatCritical: -100, game.StatAP: 30})
	st := fightAt(s, Dummies{}, [2]int{5, 6})
	target := st.Board.cell(5, 6)
	if events, _ := s.Cast(st, spellFinisher, target); lost(events, 2) != 50 {
		t.Errorf("finisher on an unmarked enemy = %d, want 50", lost(events, 2))
	}
	s.Cast(st, spellMark, target)
	if events, _ := s.Cast(st, spellFinisher, target); lost(events, 2) != 200 {
		t.Errorf("finisher on a marked enemy = %d, want 200", lost(events, 2))
	}
	// The mark is gone.
	if events, _ := s.Cast(st, spellFinisher, target); lost(events, 2) != 50 {
		t.Errorf("finisher once the mark is used = %d, want 50", lost(events, 2))
	}

	events, _ := s.Cast(st, spellChain, target)
	if lost(events, 2) != 30 {
		t.Errorf("damage through another spell = %d, want 30", lost(events, 2))
	}
	skipped := 0
	for _, ev := range events {
		if ev.Type == EventSkipped && ev.Action == 181 {
			skipped++
		}
	}
	if skipped != 1 {
		t.Errorf("the summon was reported skipped %d times, want 1", skipped)
	}

	s.Cast(st, spellSharpen, st.Player().Cell)
	if events, _ := s.Cast(st, spellBolt, target); lost(events, 2) != 120 {
		t.Errorf("bolt with 20 more base damage = %d, want 120", lost(events, 2))
	}
}

func TestMovingSpendsMovementPoints(t *testing.T) {
	s := newSim(map[int]int{game.StatMP: 3})
	st := fightAt(s, Dummies{}, [2]int{4, 6})
	// Three MP reach every free cell within three steps: the dummy stands in
	// the way, so the cells behind it cost a detour.
	steps := s.Reach(st)
	byCell := map[int]Step{}
	for _, step := range steps {
		byCell[step.Cell] = step
	}
	if _, ok := byCell[st.Board.cell(4, 6)]; ok {
		t.Error("the dummy's cell is not a step")
	}
	if _, ok := byCell[st.Board.cell(5, 6)]; ok {
		t.Error("the cell behind the dummy takes four MP")
	}
	events, reason := s.Move(st, st.Board.cell(4, 5))
	if reason != "" || len(events) != 1 || len(events[0].Path) != 3 {
		t.Fatalf("move: %q %+v", reason, events)
	}
	if _, mp := s.Points(st); mp != 1 {
		t.Errorf("MP left = %d, want 1", mp)
	}
	if _, reason := s.Move(st, st.Board.cell(4, 3)); reason != BlockedMP {
		t.Errorf("a move too far: %q", reason)
	}
}

func TestADefeatedDummyStandsBackUp(t *testing.T) {
	s := newSim(map[int]int{game.StatIntel: 100, game.StatCritical: -100})
	st := fightAt(s, Dummies{HP: 150}, [2]int{5, 6})
	events, _ := s.Cast(st, spellBolt, st.Board.cell(5, 6))
	if last := events[len(events)-1]; last.Type != EventDefeat || st.Entities[1].HP != 150 {
		t.Errorf("after a lethal hit: %+v, HP %d", last, st.Entities[1].HP)
	}
}

func TestDamageOverTurnsWaitsForItsTurn(t *testing.T) {
	s := newSim(map[int]int{game.StatCritical: -100})
	st := fightAt(s, Dummies{}, [2]int{5, 6})
	events, _ := s.Cast(st, spellVenom, st.Board.cell(5, 6))
	if lost(events, 2) != 0 {
		t.Errorf("the venom hit at once for %d", lost(events, 2))
	}
	for turn, want := range []int{40, 40, 0} {
		events, _ = s.EndTurn(st)
		if got := lost(events, 2); got != want {
			t.Errorf("venom after %d turn(s) = %d, want %d", turn+1, got, want)
		}
	}
	if st.TotalDamage != 80 {
		t.Errorf("total = %d, want 80", st.TotalDamage)
	}
}

func TestOneOutcomeIsDrawnAndThePreviewSpansThemAll(t *testing.T) {
	s := newSim(map[int]int{game.StatCritical: -100, game.StatAP: 60})
	st := fightAt(s, Dummies{}, [2]int{5, 6})
	target := st.Board.cell(5, 6)
	seen := map[int]bool{}
	for i := 0; i < 30; i++ {
		events, _ := s.Cast(st, spellDice, target)
		seen[lost(events, 2)] = true
	}
	if !seen[10] || !seen[20] || !seen[30] || len(seen) != 3 {
		t.Errorf("draws over 30 casts = %v, want 10, 20 and 30 only", seen)
	}
	for _, tg := range s.Aim(st, spellDice).Targets {
		if tg.Cell == target && (len(tg.Losses) != 1 || tg.Losses[0].Min != 10 || tg.Losses[0].Max != 30) {
			t.Errorf("preview = %+v, want 10 to 30", tg.Losses)
		}
	}
}

func TestAClassOpensItsFights(t *testing.T) {
	s := newSim(map[int]int{game.StatCritical: -100, game.StatAP: 6})
	s.Breed = 1
	st := fightAt(s, Dummies{}, [2]int{5, 6})
	if !st.Player().inState(stateReady) {
		t.Fatal("the opening spell did not put the character in its state")
	}
	// The opening makes the bolt cheaper.
	if slot := s.Slot(st, spellBolt); slot.APCost != 2 {
		t.Errorf("bolt costs %d AP, want 2", slot.APCost)
	}
	// Focusing costs 1 AP and sets off the 2 AP that waited for it.
	s.Cast(st, spellFocus, st.Player().Cell)
	if ap, _ := s.Points(st); ap != 6-1+2 {
		t.Errorf("AP after focusing = %d, want 7", ap)
	}
	// Both end with the turn.
	s.EndTurn(st)
	if ap, _ := s.Points(st); ap != 6 || st.Player().inState(stateFocused) {
		t.Errorf("next turn: %d AP, focused %v", ap, st.Player().inState(stateFocused))
	}

	// Without the class, nobody is ready: focusing does nothing.
	s.Breed = 0
	st = fightAt(s, Dummies{}, [2]int{5, 6})
	s.Cast(st, spellFocus, st.Player().Cell)
	if st.Player().inState(stateFocused) {
		t.Error("focused without being ready")
	}
}

func TestAnItemPassiveFollowsItsTurns(t *testing.T) {
	s := newSim(map[int]int{game.StatAP: 6})
	s.Passives = []int{spellRelic}
	st := fightAt(s, Dummies{}, [2]int{5, 6})
	want := []struct{ final, ap int }{{10, 6}, {3, 7}, {0, 7}}
	for turn, w := range want {
		stat := s.StatOf(st.Player())
		if ap, _ := s.Points(st); stat(statFinalDamage) != w.final || ap != w.ap {
			t.Errorf("turn %d: %d %% final damage and %d AP, want %d %% and %d", turn+1, stat(statFinalDamage), ap, w.final, w.ap)
		}
		s.EndTurn(st)
	}
}

func TestConditionsOnHealth(t *testing.T) {
	s := newSim(map[int]int{})
	dummy := &Entity{ID: 2, Kind: KindDummy, HP: 400, MaxHP: 1000}
	under, over := cond{letter: 'V', arg: 50}, cond{letter: 'v', arg: 50}
	if !s.holds(under, dummy, nil) || s.holds(over, dummy, nil) {
		t.Error("at 40 % of its health, the entity is under half")
	}
	// What a plain fighter can never be fails; its opposite passes.
	if s.holds(cond{letter: 'F', arg: 12}, dummy, nil) || !s.holds(cond{letter: 'f', arg: 12}, dummy, nil) {
		t.Error("a dummy is no given monster")
	}
}

func TestWorkIsBounded(t *testing.T) {
	// A spell that casts itself on its target would never end: the cast
	// stops once the spells cast in turn are too deep, and the budget of
	// effects stops what many of them waiting at once would cost.
	loop := spell(99, game.SpellLevel{APCost: 1, RangeMax: 6, Effects: []game.SpellEffect{
		{Action: game.ActionCastSpell, Min: 99, Max: 1, Targets: "A", Zone: point()},
		{Action: game.ActionCastSpell, Min: 99, Max: 1, Targets: "A", Zone: point()},
		{Action: 99, Min: 1, Max: 1, Targets: "A", Zone: point(), Shown: true},
	}})
	stats := map[int]int{game.StatAP: 12, game.StatMP: 6, game.StatHP: 3000, game.StatCritical: -100}
	s := &Sim{Cat: game.NewCatalog(&game.Dataset{Spells: []game.Spell{loop}}), Stat: func(id int) int { return stats[id] }, Level: 200, Rand: rand.New(rand.NewPCG(1, 2))}
	st := fightAt(s, Dummies{}, [2]int{5, 6})
	events, reason := s.Cast(st, 99, st.Board.cell(5, 6))
	if reason != "" {
		t.Fatalf("cast refused: %q", reason)
	}
	// Two casts per level over eight levels is 511 spells, each one hit.
	if got := lost(events, 2); got == 0 || got > 511 {
		t.Errorf("a spell casting itself dealt %d", got)
	}
	work := 3
	c := &casting{work: &work}
	for applied := 1; applied <= 4; applied++ {
		if gone := c.spent(); gone != (applied == 4) {
			t.Errorf("after %d effects of a budget of 3, spent = %v", applied, gone)
		}
	}
}
