package httpapi

import (
	"slices"
	"testing"

	"dofusdb/api/internal/fight"
	"dofusdb/api/internal/game"
)

// Astrub, level 150 SPEEDRUSH, compared with DoPoutch. Keep normal and
// critical outcomes separate without changing the HTTP server's dice.
func TestSpeedrushEffectsWithInstalledGameData(t *testing.T) {
	const astrub = 192419850
	t.Run("Martelo", func(t *testing.T) {
		const martelo = 13389
		r := newSpeedrushReplay(t, 14, astrub, 2, 1200)
		r.act(fightAction{Type: "place", Entity: 1, Cell: 63})
		r.act(fightAction{Type: "begin"})
		aim := r.aim(martelo)
		for _, tc := range []struct {
			cell int
			zone []int
			loss fight.Loss
		}{
			{73, []int{62, 64, 73}, fight.Loss{Entity: 3, Min: 216, Max: 232, CritMin: 286, CritMax: 309}},
			{62, []int{53, 62, 73}, fight.Loss{Entity: 3, Min: 271, Max: 290, CritMin: 358, CritMax: 387}},
		} {
			i := slices.IndexFunc(aim.Targets, func(target fight.Target) bool { return target.Cell == tc.cell })
			if i < 0 {
				t.Fatalf("Martelo cannot aim at cell %d", tc.cell)
			}
			target := aim.Targets[i]
			slices.Sort(target.Zone)
			if !slices.Equal(target.Zone, tc.zone) || !slices.Equal(target.Losses, []fight.Loss{tc.loss}) {
				t.Fatalf("cell %d: zone %v, losses %+v", tc.cell, target.Zone, target.Losses)
			}
		}
		r.act(fightAction{Type: "cast", Spell: martelo, Cell: 73})
		if r.view.State.Entities[1].HP != 1200 {
			t.Fatal("Martelo hit the dummy outside the area on cell 52")
		}
		hit := 1200 - r.view.State.Entities[2].HP
		critical := actualCastWasCritical(t, r.view.Events, martelo)
		if critical && (hit < 286 || hit > 309) || !critical && (hit < 216 || hit > 232) {
			t.Fatalf("off-centre Martelo hit for %d, critical=%v", hit, critical)
		}
	})
	t.Run("Invisibilité", func(t *testing.T) {
		const invisibility, flame, trap = 12913, 24006, 12906
		r := newSpeedrushReplay(t, 4, astrub, 2, 1200)
		r.act(fightAction{Type: "place", Entity: 1, Cell: 63})
		r.act(fightAction{Type: "begin"})
		r.act(fightAction{Type: "cast", Spell: invisibility, Cell: 63})
		invisibleTurns := func() int {
			for _, b := range r.view.State.Player().Buffs {
				if b.From == invisibility && b.Kind == fight.BuffInvisible {
					return b.Turns
				}
			}
			return 0
		}
		hasMP := slices.ContainsFunc(r.view.State.Player().Buffs, func(b fight.Buff) bool {
			return b.From == invisibility && b.Kind == fight.BuffStat && b.Ref == game.StatMP && b.Value == 2 && b.Turns == 2
		})
		if invisibleTurns() != 1 || !hasMP || r.view.AP != 10 || r.view.MP != 7 {
			t.Fatalf("invisibility at cast: %d AP, %d MP, %+v", r.view.AP, r.view.MP, r.view.State.Player().Buffs)
		}
		r.act(fightAction{Type: "cast", Spell: flame, Cell: 73})
		if invisibleTurns() != 1 {
			t.Fatal("casting on an empty cell revealed the Sram")
		}
		r.act(fightAction{Type: "cast", Spell: trap, Cell: 64})
		if invisibleTurns() != 1 || !slices.ContainsFunc(r.view.State.Marks, func(mark fight.Mark) bool { return mark.From == trap && mark.Cell == 64 }) {
			t.Fatal("the Sram should remain invisible after placing Piège Sournois")
		}
		r.act(fightAction{Type: "cast", Spell: flame, Cell: 62})
		if invisibleTurns() != 0 || r.view.MP != 7 || r.view.State.Entities[2].HP >= 1200 {
			t.Fatal("dealing direct damage should reveal the Sram and preserve the MP bonus")
		}
	})
	t.Run("Mutilation", func(t *testing.T) {
		const mutilation = 12737
		r := newSpeedrushReplay(t, 11, astrub, 2, 1200)
		r.act(fightAction{Type: "begin"})
		initialPower := r.view.CurrentStats[game.StatPower]
		r.act(fightAction{Type: "cast", Spell: mutilation, Cell: r.view.State.Player().Cell})
		if hp, power := r.view.State.Player().HP, r.view.CurrentStats[game.StatPower]-initialPower; hp != 1989 || power != 150 {
			t.Fatalf("on cast: HP=%d, power bonus=%d", hp, power)
		}
		r.act(fightAction{Type: "end"})
		if hp := r.view.State.Player().HP; hp != 1791 {
			t.Fatalf("next turn: HP=%d", hp)
		}
	})
	t.Run("Vitalité", func(t *testing.T) {
		const vitality = 13120
		r := newSpeedrushReplay(t, 8, astrub, 2, 1200)
		r.act(fightAction{Type: "begin"})
		r.act(fightAction{Type: "cast", Spell: vitality, Cell: r.view.State.Player().Cell})
		want := 2563
		if actualCastWasCritical(t, r.view.Events, vitality) {
			want = 2607
		}
		if p := r.view.State.Player(); p.HP != want || p.MaxHP != want {
			t.Fatalf("Vitalité: %d/%d HP, want %d/%d", p.HP, p.MaxHP, want, want)
		}
	})
}

func actualCastWasCritical(t *testing.T, events []fight.Event, spell int) bool {
	t.Helper()
	for _, event := range events {
		if event.Type == fight.EventCast && event.Spell == spell {
			return event.Critical
		}
	}
	t.Fatalf("missing cast event for spell %d", spell)
	return false
}
