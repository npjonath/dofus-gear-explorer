package fight

import (
	"slices"

	"dofusdb/api/internal/game"
)

const KindSummon = "summon"

func (e *Entity) enemyTeam() bool { return e.Kind == KindDummy && !e.Ally }

type CreatureSpell struct {
	ID    int `json:"id"`
	Grade int `json:"grade"`
}

type CreatureGrade struct {
	Grade              int             `json:"grade"`
	HP                 int             `json:"hp"`
	AP                 int             `json:"ap"`
	MP                 int             `json:"mp"`
	BonusHP            int             `json:"bonusHp"`
	Stats              map[int]int     `json:"stats"`
	Bonuses            map[int]int     `json:"bonuses"`
	Spells             []CreatureSpell `json:"spells"`
	StartingSpellGrade int             `json:"startingSpellGrade"`
	StartingSpell      int             `json:"startingSpellId"`
}

type Creature struct {
	Names    game.Texts      `json:"names"`
	Static   bool            `json:"static"`
	UsesSlot bool            `json:"usesSummonSlot"`
	Cost     int             `json:"summonCost"`
	Grades   []CreatureGrade `json:"grades"`
}

func (s *Sim) grade(id, rank int) (*Creature, *CreatureGrade) {
	m := s.Creatures[id]
	if m == nil || len(m.Grades) == 0 {
		return nil, nil
	}
	for i := range m.Grades {
		if m.Grades[i].Grade == rank {
			return m, &m.Grades[i]
		}
	}
	return m, &m.Grades[0]
}

func (st *State) Active() *Entity {
	if st.ActiveID != 0 {
		if e := st.entity(st.ActiveID); e != nil {
			return e
		}
	}
	return st.Player()
}

func (st *State) counters() map[int]*Use {
	if e := st.Active(); e != nil && e.Kind == KindSummon {
		if e.Uses == nil {
			e.Uses = map[int]*Use{}
		}
		return e.Uses
	}
	return st.Uses
}

func (s *Sim) summon(st *State, c *casting, at place, a *attack, e game.SpellEffect) {
	m, g := s.grade(e.Min, e.Max)
	if m == nil {
		c.skip(e)
		return
	}
	caster := st.entity(at.caster)
	if caster == nil || len(st.Entities) >= 16 {
		return
	}
	if e.Action == 405 || e.Action == 2796 {
		victim := st.at(at.cell)
		m := readMask(e.Targets)
		if victim == nil || victim.Kind != KindSummon || !m.names(caster, victim) || !s.met(m, caster, victim, nil) {
			return
		}
		victim.HP = 0
		s.removeDefeatedSummons(st, c)
		caster = st.entity(at.caster)
	}
	if caster == nil || !st.free(at.cell) {
		return
	}
	slots, maxID := 0, 0
	for _, fighter := range st.Entities {
		maxID = max(maxID, fighter.ID)
		if fighter.Owner == caster.ID {
			if other := s.Creatures[fighter.MonsterID]; other != nil && other.UsesSlot {
				slots += max(1, other.Cost)
			}
		}
	}
	if m.UsesSlot && slots+max(1, m.Cost) > max(1, s.StatOf(caster)(game.StatSummons)) {
		return
	}
	stats := map[int]int{}
	for id, value := range g.Stats {
		stats[id] = value
	}
	for id, bonus := range g.Bonuses {
		if id != game.StatHP {
			stats[id] += bonus
		}
	}
	stats[game.StatAP] = g.AP
	stats[game.StatMP] = g.MP
	hp := max(1, g.HP+g.BonusHP*(50+5*s.Level)/100)
	id := max(maxID, st.LastEntityID) + 1
	st.LastEntityID = id
	st.Entities = append(st.Entities, Entity{ID: id, Kind: KindSummon, MonsterID: e.Min, Grade: g.Grade, Owner: caster.ID, Cell: at.cell, HP: hp, MaxHP: hp, Stats: stats, Buffs: []Buff{}})
	c.emit(Event{Type: "summon", Entity: id, Spell: a.id, Cell: &at.cell})
	if g.StartingSpell > 0 && at.depth < maxDepth {
		if start := s.attack(g.StartingSpell, max(1, g.StartingSpellGrade)); start != nil {
			s.resolve(st, c, id, caster.ID, start, at.cell, at.depth+1)
		}
	}
	s.crossMarks(st, c, id, -1, at.depth)
}

func (s *Sim) playable(st *State, e *Entity) bool {
	if e.Kind != KindSummon || e.HP <= 0 {
		return false
	}
	m, g := s.grade(e.MonsterID, e.Grade)
	return m != nil && !m.Static && (g.AP > 0 || g.MP > 0 || len(g.Spells) > 0)
}

func (s *Sim) removeDefeatedSummons(st *State, c *casting) {
	st.Entities = slices.DeleteFunc(st.Entities, func(e Entity) bool {
		if e.Kind != KindSummon || e.HP > 0 {
			return false
		}
		c.emit(Event{Type: EventDefeat, Entity: e.ID})
		if st.ActiveID == e.ID {
			st.ActiveID = 0
		}
		return true
	})
}

func (s *Sim) SummonSpells(entity *Entity) []CreatureSpell {
	_, grade := s.grade(entity.MonsterID, entity.Grade)
	if grade != nil {
		return grade.Spells
	}
	return nil
}
