package rawdata

import (
	"reflect"
	"testing"

	"dofusdb/api/internal/game"
)

// The effects below are those of Enutrof spells in the game data.
func TestAttackLinesOfASpellGrade(t *testing.T) {
	const shown, described = 79, 95
	level := func(critChance int, effects, critical []rawSpellEffect) rawSpellLevel {
		return rawSpellLevel{SpellID: 13358, CritChance: critChance, Effects: array[rawSpellEffect]{Array: effects}, Critical: array[rawSpellEffect]{Array: critical}}
	}
	fx := func(action, lo, hi int, mask string, flags int) rawSpellEffect {
		return rawSpellEffect{ActionID: action, DiceNum: lo, DiceSide: hi, TargetMask: mask, Triggers: "I", Flags: flags}
	}

	// Éboulement: a hit, a state, then damage the state deals later.
	state := rawSpellEffect{ActionID: 950, Value: 7206, TargetMask: "A", Triggers: "I", Duration: 3, Flags: described}
	lines := attackLines(level(25,
		[]rawSpellEffect{fx(97, 31, 35, "a,A", shown), state, fx(97, 12, 14, "A", described)},
		[]rawSpellEffect{fx(97, 37, 42, "a,A", shown), state, fx(97, 15, 17, "A", described)},
	))
	want := []game.AttackLine{
		{Element: game.ElementEarth, Min: 31, Max: 35, CritMin: 37, CritMax: 42, Targets: "a,A"},
		{Element: game.ElementEarth, Min: 12, Max: 14, CritMin: 15, CritMax: 17, Targets: "A", Conditional: true},
	}
	if !reflect.DeepEqual(lines, want) {
		t.Errorf("Éboulement = %+v", lines)
	}

	// Harmattan: its only damage is described; it is the spell's hit.
	if lines := attackLines(level(0, []rawSpellEffect{fx(98, 31, 35, "A", described)}, nil)); len(lines) != 1 || lines[0].Conditional || !lines[0].Hits() {
		t.Errorf("Harmattan = %+v", lines)
	}

	// Tourbière: the damage grows with each MP used.
	boost := rawSpellEffect{ActionID: actionBoostBaseHits, DiceNum: 13358, Value: 5, TargetMask: "C", Triggers: "CCMPARR", Duration: 1, Flags: described}
	if lines := attackLines(level(20, []rawSpellEffect{fx(97, 31, 35, "a,A", shown), boost}, nil)); len(lines) != 1 || lines[0].PerMP != 5 {
		t.Errorf("Tourbière = %+v", lines)
	}

	// Gisement: damage to enemies, and a heal to allies as their turn starts.
	heal := fx(108, 29, 32, "a", shown)
	heal.Triggers = "TB"
	lines = attackLines(level(0, []rawSpellEffect{fx(99, 29, 32, "A", shown), heal}, nil))
	if len(lines) != 2 || lines[1].Kind != game.LineHeal || lines[1].Element != game.ElementFire || lines[1].Trigger != game.TriggerTurnStart || lines[1].Hits() {
		t.Errorf("Gisement = %+v", lines)
	}

	// Bêche des Anciens: damage, a loss of AP that is not kept, a pushback.
	lines = attackLines(level(0, []rawSpellEffect{fx(98, 32, 36, "A", shown), fx(1079, 2, 0, "A", shown), fx(actionPush, 2, 0, "g,A", shown)}, nil))
	if len(lines) != 2 || lines[1].Kind != game.LinePush || lines[1].Min != 2 || lines[1].Max != 2 {
		t.Errorf("Bêche des Anciens = %+v", lines)
	}

	// Corruption heals 40% of the health points; a poison and a hidden effect are left out.
	poison := fx(99, 10, 12, "A", shown)
	poison.Duration = 2
	lines = attackLines(level(0, []rawSpellEffect{fx(actionHealPercent, 40, 0, "a", shown), poison, fx(99, 10, 12, "A", 64)}, nil))
	if len(lines) != 1 || lines[0].Kind != game.LineHealPercent || lines[0].Min != 40 {
		t.Errorf("Corruption = %+v", lines)
	}
}
