package fight

import (
	"testing"

	"dofusdb/api/internal/game"
)

func TestSummonTakesItsOwnTurnAndGrantsItsOpeningBuff(t *testing.T) {
	s := newSim(map[int]int{game.StatSummons: 1})
	s.Level = 150
	const summonID, monsterID, opening = 151, 301, 303
	summon := spell(summonID, game.SpellLevel{APCost: 3, RangeMax: 3, Effects: []game.SpellEffect{{Action: 181, Min: monsterID, Max: 1, Zone: point(), Targets: "a,A"}}})
	buff := spell(opening, game.SpellLevel{Effects: []game.SpellEffect{{Action: 138, Stat: game.StatPower, Sign: 1, Min: 60, Targets: "h,P", Duration: -1, Zone: game.Zone{Shape: "A"}}}})
	s.Cat.SpellByID[summon.ID] = &summon
	s.Cat.SpellByID[buff.ID] = &buff
	s.Creatures = map[int]*Creature{monsterID: {UsesSlot: true, Cost: 1, Grades: []CreatureGrade{{
		Grade: 1, AP: 6, MP: 4, BonusHP: 66, StartingSpell: opening, StartingSpellGrade: 1, Spells: []CreatureSpell{{ID: spellBolt, Grade: 1}},
	}}}}
	st := fightAt(s, Dummies{}, [2]int{5, 6})
	cell := st.Board.cell(4, 6)
	if _, why := s.Cast(st, summonID, cell); why != "" {
		t.Fatal(why)
	}
	ally := st.at(cell)
	if ally == nil || ally.Kind != KindSummon || ally.HP != 528 || s.StatOf(st.Player())(game.StatPower) != 60 {
		t.Fatalf("unexpected summon or owner strength: %+v", st)
	}
	s.EndTurn(st)
	if ap, mp := s.Points(st); st.Active().ID != ally.ID || st.Turn != 1 || ap != 6 || mp != 4 {
		t.Fatalf("summon turn: %+v", st)
	}
	if s.Slot(st, summonID) != nil || s.Slot(st, spellBolt) == nil {
		t.Fatal("wrong active spell bar")
	}
	s.EndTurn(st)
	if st.Active().Kind != KindPlayer || st.Turn != 2 {
		t.Fatalf("round did not advance: %+v", st)
	}
}

func TestShieldTakesDamageBeforeHealth(t *testing.T) {
	s := newSim(map[int]int{game.StatAP: 12})
	st := fightAt(s, Dummies{}, [2]int{5, 6})
	d := &st.Entities[1]
	d.Buffs = append(d.Buffs, Buff{Kind: BuffShield, Value: 80, Turns: 2})
	initial := d.HP
	ev, why := s.Cast(st, spellBolt, d.Cell)
	loss := lost(ev, d.ID)
	if why != "" || loss != 20 && loss != 70 || d.HP != initial-loss || d.Buffs[0].Value != 0 {
		t.Fatalf("shield, HP or event mismatch: %+v %+v", d, ev)
	}
}

func TestTrainingMapUsesSeparateSightAndPlacementMasks(t *testing.T) {
	s := newSim(map[int]int{})
	board := Board{MapID: 11, Width: 6, Height: 3, Walls: []int{8}, SightWalls: []int{}, Blue: []int{6, 12, 13, 14}, Red: []int{10, 11, 16, 17}}
	st := s.StartOn(Dummies{Count: 2, HP: 1200, Allies: 1}, board)
	if !st.sees(6, 10) {
		t.Fatal("water incorrectly blocks sight")
	}
	if why := s.Place(st, 1, 11); why != BlockedCell {
		t.Fatal("wrong camp accepted")
	}
	if !st.Entities[2].Ally || !board.inCamp(&st.Entities[2], st.Entities[2].Cell) {
		t.Fatal("allied dummy outside its camp")
	}
}

func TestTrainingDummyRecoversErodedHealthOnRevival(t *testing.T) {
	sim := newSim(map[int]int{})
	st := fightAt(sim, Dummies{HP: 1200}, [2]int{4, 6})
	dummy := &st.Entities[1]
	sim.loseHealth(dummy, 338)
	if dummy.HP != 862 || dummy.MaxHP != 1167 {
		t.Fatalf("after critical: %d / %d", dummy.HP, dummy.MaxHP)
	}
	sim.loseHealth(dummy, 900)
	sim.settle(st, sim.casting(0, false, false))
	if dummy.HP != 1200 || dummy.MaxHP != 1200 {
		t.Fatalf("after revival: %d / %d", dummy.HP, dummy.MaxHP)
	}
}

func TestSymmetricTeleportUsesThePivotAndTheRealTerrain(t *testing.T) {
	s := newSim(map[int]int{})
	st := fightAt(s, Dummies{}, [2]int{4, 6})
	player, enemy := st.Player(), &st.Entities[1]
	from := player.Cell
	at := place{stood: map[int]int{player.ID: from, enemy.ID: enemy.Cell}, cell: enemy.Cell}
	a := attack{}
	c := s.casting(1, false, false)
	stop := st.Board.cell(5, 6)
	st.Board.Walls = []int{stop}
	s.reflect(st, c, at, &a, 1104, player, enemy)
	if player.Cell != from {
		t.Fatal("crossed a wall")
	}
	st.Board.Walls = nil
	s.reflect(st, c, at, &a, 1104, player, enemy)
	if player.Cell != stop || len(c.events) != 1 {
		t.Fatalf("player=%d events=%v", player.Cell, c.events)
	}
}
