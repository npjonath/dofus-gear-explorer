package engine

import (
	"testing"

	"dofusdb/api/internal/game"
)

func statsOf(values map[int]int) func(int) int {
	return func(id int) int { return values[id] }
}

func TestLineFormula(t *testing.T) {
	// 30 base, 900 Strength and 50 Power: floor(30 x 10.5) = 315, plus 20
	// Damage and 15 Earth damage = 350; a critical adds 25.
	stat := statsOf(map[int]int{
		game.StatStrength: 900, game.StatPower: 50, game.StatDamage: 20, game.StatDamageEarth: 15,
		game.StatCritDamage: 25, game.StatSpellDamage: 10, game.StatRangedDamage: 20, game.StatMeleeDamage: 50,
		game.StatWeaponDamage: 100,
	})
	line := game.AttackLine{Element: game.ElementEarth, Min: 30, Max: 30, CritMin: 40, CritMax: 40}

	ranged := AttackSpec{Lines: []game.AttackLine{line}}
	// 350 x 1.10 = 385, x 1.20 = 462. Critical: 420 + 60 = 480 -> 528 -> 633.
	if hit, _ := ranged.Line(line, stat); hit.Min != 462 || hit.Max != 462 || hit.CritMin != 633 {
		t.Errorf("ranged spell = %+v", hit)
	}
	// A weapon in melee takes the weapon and melee bonuses instead: 350 x 2 x 1.5.
	weapon := AttackSpec{Lines: []game.AttackLine{line}, Weapon: true, Melee: true}
	if hit, _ := weapon.Line(line, stat); hit.Min != 1050 {
		t.Errorf("melee weapon = %+v", hit)
	}
	// Neutral damage scales with Strength and takes Neutral damage.
	neutral := game.AttackLine{Element: game.ElementNeutral, Min: 30, Max: 30}
	if hit, _ := ranged.Line(neutral, stat); hit.Min != (315+20)*110/100*120/100 || hit.CritMax != 0 {
		t.Errorf("neutral = %+v", hit)
	}
	// A characteristic below -100 cannot turn the base negative.
	if hit, _ := ranged.Line(line, statsOf(map[int]int{game.StatStrength: -500})); hit.Min != 0 {
		t.Errorf("negative multiplier = %+v", hit)
	}
}

func TestBestElementAndTotals(t *testing.T) {
	stat := statsOf(map[int]int{game.StatStrength: 100, game.StatChance: 400, game.StatAgility: 400})
	best := game.AttackLine{Element: game.ElementBest, Min: 10, Max: 20}
	spec := AttackSpec{Lines: []game.AttackLine{
		best,
		{Element: game.ElementEarth, Min: 10, Max: 10},
		{Element: game.ElementEarth, Min: 50, Max: 50, Delay: 1},
	}}
	// Chance and Agility tie: Water comes first in the game's order.
	hit, element := spec.Line(best, stat)
	if element != game.ElementWater || hit.Min != 50 || hit.Max != 100 {
		t.Errorf("best element = %d, hit %+v", element, hit)
	}
	// The delayed line is listed but not part of the total.
	if total := spec.Total(stat); total.Min != 50+20 || total.Max != 100+20 {
		t.Errorf("total = %+v", total)
	}
}

func TestWeaponAndSpellAttacks(t *testing.T) {
	bow := &game.Item{
		ID: 1, Slot: game.SlotWeapon,
		Weapon: &game.WeaponInfo{APCost: 5, RangeMin: 3, RangeMax: 7, CritChance: 20, CritBonus: 6},
		Effects: []game.ItemEffect{
			{EffectID: 100, Kind: game.EffectWeapon, StatID: -1, Min: 34, Max: 41},
			{EffectID: 91, Kind: game.EffectWeapon, StatID: -1, Min: 7, Max: 9},
			{EffectID: 118, Kind: game.EffectStat, StatID: game.StatStrength, Min: 50, Max: 50},
		},
	}
	stat := statsOf(map[int]int{})
	a := WeaponAttack(bow, WeaponOptions{}, stat)
	if a == nil || len(a.Lines) != 2 || a.Melee || !a.Lines[1].Steal || a.Lines[1].Element != game.ElementWater {
		t.Fatalf("bow = %+v", a)
	}
	// With no characteristic the lines are the weapon's own; a critical adds 6.
	if a.Total.Min != 41 || a.Total.Max != 50 || a.Total.CritMin != 53 || a.Total.CritMax != 62 {
		t.Errorf("bow total = %+v", a.Total)
	}
	if WeaponAttack(&game.Item{ID: 2, Slot: game.SlotHat}, WeaponOptions{}, stat) != nil {
		t.Errorf("a hat should have no attack")
	}
	// Forged to Water, the Neutral line follows Chance; the mastery adds 300
	// Power: 34 x (100 + 100 + 300) / 100 = 170.
	chance := statsOf(map[int]int{game.StatChance: 100})
	water := WeaponAttack(bow, WeaponOptions{Element: "water", Mastery: true}, chance)
	if water.Lines[0].Element != game.ElementWater || water.Lines[0].Min != 170 {
		t.Errorf("water bow = %+v", water.Lines[0])
	}
	if !CanTakeElement(bow, "water") || CanTakeElement(bow, "neutral") || CanTakeElement(&game.Item{}, "water") {
		t.Errorf("CanTakeElement is wrong")
	}

	spell := &game.Spell{ID: 3, Levels: []game.SpellLevel{
		{Grade: 1, MinLevel: 50, RangeMax: 1, Lines: []game.AttackLine{{Element: game.ElementFire, Min: 5, Max: 6}}},
		{Grade: 2, MinLevel: 120, RangeMax: 1, Lines: []game.AttackLine{{Element: game.ElementFire, Min: 9, Max: 10}}},
	}}
	if SpellAttack(spell, 49, RangeAuto, stat) != nil {
		t.Errorf("the spell should not be available at level 49")
	}
	if a := SpellAttack(spell, 119, RangeAuto, stat); a == nil || a.Grade != 1 || !a.Melee || a.Total.Max != 6 {
		t.Errorf("level 119 = %+v", a)
	}
	if a := SpellAttack(spell, 200, RangeRanged, stat); a == nil || a.Grade != 2 || a.Melee || a.Total.Max != 10 {
		t.Errorf("level 200 = %+v", a)
	}
}

func TestCritRateAndMean(t *testing.T) {
	line := game.AttackLine{Element: game.ElementWater, Min: 100, Max: 200, CritMin: 300, CritMax: 300}
	spec := AttackSpec{Lines: []game.AttackLine{line}, CritChance: 20}
	// 20% from the attack and 70 Critical: 9 casts in 10 are critical, so
	// the average is 0.1 x 150 + 0.9 x 300 = 285.
	stat := statsOf(map[int]int{game.StatCritical: 70})
	if rate := spec.CritRate(stat); rate != 90 {
		t.Errorf("rate = %d", rate)
	}
	if mean := spec.Mean(stat); mean != 285 {
		t.Errorf("mean = %d", mean)
	}
	// The rate stops at 100, and an attack with no critical line never crits.
	if rate := spec.CritRate(statsOf(map[int]int{game.StatCritical: 95})); rate != 100 {
		t.Errorf("capped rate = %d", rate)
	}
	plain := AttackSpec{Lines: []game.AttackLine{{Element: game.ElementWater, Min: 100, Max: 200}}, CritChance: 20}
	if plain.CritRate(stat) != 0 || plain.Mean(stat) != 150 {
		t.Errorf("no critical line: rate %d, mean %d", plain.CritRate(stat), plain.Mean(stat))
	}
}
