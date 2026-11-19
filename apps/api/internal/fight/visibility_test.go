package fight

import (
	"testing"

	"dofusdb/api/internal/game"
)

func TestInvisibilityAndRevelation(t *testing.T) {
	s := newSim(map[int]int{game.StatAP: 15, game.StatMP: 5})
	veil := spell(12913, game.SpellLevel{APCost: 2, RangeMax: 5, Effects: []game.SpellEffect{
		{Action: 150, Targets: "a,A", Duration: 1, Zone: point()},
		{Action: 128, Targets: "a", Stat: game.StatMP, Sign: 1, Min: 2, Max: 2, Duration: 2, Zone: point()},
	}})
	detect := spell(12852, game.SpellLevel{RangeMax: 5, Effects: []game.SpellEffect{{Action: 202, Targets: "A", Zone: point()}}})
	s.Cat.SpellByID[veil.ID] = &veil
	s.Cat.SpellByID[detect.ID] = &detect
	st := fightAt(s, Dummies{}, [2]int{4, 6})
	target := st.entity(2)
	self := st.Player()
	if _, reason := s.Cast(st, veil.ID, self.Cell); reason != "" || !self.invisible() {
		t.Fatalf("failed to turn invisible: %q", reason)
	}
	if ap, mp := s.Points(st); ap != 13 || mp != 7 {
		t.Fatalf("unexpected resources on self-cast: %d/%d", ap, mp)
	}
	if _, reason := s.Cast(st, spellBolt, st.Board.cell(3, 5)); reason != "" || !self.invisible() {
		t.Fatalf("an empty cast revealed the caster: %q", reason)
	}
	if _, reason := s.Cast(st, spellBolt, target.Cell); reason != "" || self.invisible() {
		t.Fatalf("direct damage did not reveal its caster: %q", reason)
	}
	if _, mp := s.Points(st); mp != 7 {
		t.Fatal("revelation removed the MP buff")
	}
	if _, reason := s.Cast(st, veil.ID, target.Cell); reason != "" || !target.invisible() {
		t.Fatalf("opposing target did not turn invisible: %q", reason)
	}
	if _, reason := s.Cast(st, detect.ID, target.Cell); reason != "" || target.invisible() {
		t.Fatal("action 202 did not reveal the target")
	}
	st = fightAt(s, Dummies{}, [2]int{4, 6})
	s.Cast(st, veil.ID, st.Player().Cell)
	s.EndTurn(st)
	if st.Player().invisible() || s.StatOf(st.Player())(game.StatMP) != 7 {
		t.Fatal("1-turn invisibility did not expire independently from 2-turn MP")
	}
}
