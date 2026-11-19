package fight

import (
	"slices"
	"testing"

	"dofusdb/api/internal/game"
)

func TestHammerZoneHitsNeighboursAndExcludesWielder(t *testing.T) {
	s := newSim(map[int]int{})
	s.Weapon = &game.Item{TypeID: 7, Weapon: &game.WeaponInfo{APCost: 2, RangeMin: 1, RangeMax: 1, CritChance: 10, CritBonus: 20}, Effects: []game.ItemEffect{{Kind: game.EffectWeapon, EffectID: 100, Min: 100, Max: 100}}}
	s.Cat.TypeByID[7] = &game.ItemType{RawZone: "X1,0,10,1"}
	st := fightAt(s, Dummies{}, [2]int{4, 6}, [2]int{4, 7})
	center := st.Entities[1].Cell
	for _, target := range s.Aim(st, WeaponID).Targets {
		if target.Cell != center {
			continue
		}
		want := []Loss{{Entity: 2, Min: 100, Max: 100, CritMin: 120, CritMax: 120}, {Entity: 3, Min: 90, Max: 90, CritMin: 108, CritMax: 108}}
		if !slices.Equal(target.Losses, want) {
			t.Fatalf("hammer preview: %+v", target.Losses)
		}
		events, reason := s.Cast(st, WeaponID, center)
		if reason != "" || lost(events, 1) != 0 || lost(events, 2) < 100 || lost(events, 3) < 90 {
			t.Fatalf("hammer hit: %v, %s", events, reason)
		}
		return
	}
	t.Fatal("missing hammer aim")
}

func TestReturnToPreviousCellReversesWholePush(t *testing.T) {
	s := newSim(map[int]int{})
	st := fightAt(s, Dummies{}, [2]int{4, 6})
	target := &st.Entities[1]
	origin := target.Cell
	if _, reason := s.Cast(st, spellShove, origin); reason != "" || target.Cell != origin+3 {
		t.Fatalf("push: %d (%s)", target.Cell, reason)
	}
	spell := s.Cat.SpellByID[spellBolt]
	spell.Levels[0].Effects = []game.SpellEffect{{Action: 1100, Targets: "A", Zone: point()}}
	spell.Levels[0].CritEffects = nil
	aim := s.Aim(st, spellBolt)
	if len(aim.Targets) == 0 || !slices.Equal(target.PreviousCells, []int{origin}) {
		t.Fatalf("preview mutated history: %+v", target.PreviousCells)
	}
	if _, reason := s.Cast(st, spellBolt, target.Cell); reason != "" || target.Cell != origin || len(target.PreviousCells) != 0 {
		t.Fatalf("rewind: %+v (%s)", target, reason)
	}
}

func TestTransferCostsCasterHealthAndVitalityExpires(t *testing.T) {
	s := newSim(map[int]int{})
	st := fightAt(s, Dummies{}, [2]int{4, 6})
	spell := s.Cat.SpellByID[spellBolt]
	spell.Levels[0].CritEffects = nil
	spell.Levels[0].Effects = []game.SpellEffect{{Action: 90, Min: 10, Targets: "A", Zone: point()}}
	target := &st.Entities[1]
	target.HP -= 500
	if _, refused := s.Cast(st, spellBolt, target.Cell); refused != "" {
		t.Fatal(refused)
	}
	if st.Player().HP != 2700 || st.Player().MaxHP != 3000 || target.HP != 9800 {
		t.Fatalf("transfer: %+v", st.Entities)
	}
	spell = s.Cat.SpellByID[spellHaste]
	spell.Levels[0].Effects = []game.SpellEffect{{Action: 1078, Min: 10, Targets: "C", Duration: 1, Zone: point()}}
	if _, refused := s.Cast(st, spellHaste, st.Player().Cell); refused != "" {
		t.Fatal(refused)
	}
	if st.Player().MaxHP != 3300 || st.Player().HP != 3000 || eroded(st.Player()) != 0 {
		t.Fatalf("temporary vitality: %+v", st.Player())
	}
	s.EndTurn(st)
	if st.Player().MaxHP != 3000 || st.Player().HP != 2700 {
		t.Fatalf("expiry: %+v", st.Player())
	}
}

func TestArenaPlacesAlliesAndEnemiesIndependently(t *testing.T) {
	sim := newSim(map[int]int{})
	b := Board{Width: 10, Height: 15, Blue: []int{116, 23, 24, 25, 26, 53, 63, 73, 83, 113, 114, 115}, Red: []int{52, 62, 72, 82}}
	for _, room := range []Board{b, DefaultBoard()} {
		st := sim.StartOn(Dummies{Count: 7, Allies: 3, HP: 50000}, room)
		if len(st.Entities) != 8 || !ValidPositions(st) {
			t.Fatalf("invalid full teams: %+v", st.Entities)
		}
		for i, e := range st.Entities[1:] {
			if e.Ally != (i >= 4) {
				t.Fatalf("wrong team at %d: %+v", i, e)
			}
		}
		if len(room.Blue) > 0 && st.Entities[5].Cell != 115 {
			t.Errorf("allied poutch starts at %d, expected 115", st.Entities[5].Cell)
		}
	}
}

func TestSetAndReduceCurrentSpellCooldown(t *testing.T) {
	s := newSim(map[int]int{})
	st := fightAt(s, Dummies{}, [2]int{4, 6})
	spell := s.Cat.SpellByID[spellHaste]
	spell.Levels[0].Effects = []game.SpellEffect{{Action: 1045, Min: spellBlast, Value: 3, Targets: "C", Zone: point()}}
	if _, why := s.Cast(st, spellHaste, st.Player().Cell); why != "" {
		t.Fatal(why)
	}
	if got := s.Slot(st, spellBlast).Cooldown; got != 3 {
		t.Fatalf("set to 3: %d", got)
	}
	spell.Levels[0].Effects[0].Action = 1036
	spell.Levels[0].Effects[0].Value = 2
	s.Aim(st, spellHaste)
	if got := s.Slot(st, spellBlast).Cooldown; got != 3 {
		t.Fatalf("preview changed cooldown: %d", got)
	}
	s.Cast(st, spellHaste, st.Player().Cell)
	if got := s.Slot(st, spellBlast).Cooldown; got != 1 {
		t.Fatalf("reduce by 2: %d", got)
	}
	s.EndTurn(st)
	if _, why := s.Cast(st, spellBlast, st.Entities[1].Cell); why != "" {
		t.Fatal(why)
	}
	if got := s.Slot(st, spellBlast).Cooldown; got != 2 {
		t.Fatalf("restore ordinary cooldown: %d", got)
	}
}

func TestFinalHealingBuffAppliesToOutgoingHeals(t *testing.T) {
	s := newSim(map[int]int{})
	st := fightAt(s, Dummies{}, [2]int{4, 6})
	spell := s.Cat.SpellByID[spellHaste]
	spell.Levels[0].Effects = []game.SpellEffect{{Action: 2971, Min: 50, Targets: "C", Duration: 1, Zone: point()}}
	s.Cast(st, spellHaste, st.Player().Cell)
	st.Entities[1].HP -= 500
	spell = s.Cat.SpellByID[spellBolt]
	spell.Levels[0].Effects = []game.SpellEffect{{Action: 108, Min: 100, Targets: "A", Zone: point()}}
	spell.Levels[0].CritEffects = nil
	s.Cast(st, spellBolt, st.Entities[1].Cell)
	if st.Entities[1].HP != 9650 {
		t.Fatalf("final heal %+v", st.Entities[1])
	}
}
