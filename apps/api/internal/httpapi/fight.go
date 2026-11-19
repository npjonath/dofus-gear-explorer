package httpapi

import (
	"math/rand/v2"
	"net/http"
	"sort"

	"dofusdb/api/internal/engine"
	"dofusdb/api/internal/fight"
	"dofusdb/api/internal/game"
)

// fightAction is one thing the player does in a fight.
type fightAction struct {
	// Type is "place", "begin", "move", "cast" or "end".
	Type   string `json:"type"`
	Entity int    `json:"entity"`
	Cell   int    `json:"cell"`
	Spell  int    `json:"spell"`
}

type fightRequest struct {
	engine.Build
	BreedID int `json:"breedId"`
	// Dummies sets a new fight up; State is the fight to go on with.
	Dummies *fight.Dummies `json:"dummies"`
	State   *fight.State   `json:"state"`
	Action  *fightAction   `json:"action"`
	// Spell is the spell to aim, for the aim endpoint.
	Spell int `json:"spell"`
}

type fightSlotDTO struct {
	fight.Slot
	Name      string `json:"name"`
	IconID    int    `json:"iconId"`
	Weapon    bool   `json:"weapon"`
	BreedID   int    `json:"breedId"`
	VariantOf int    `json:"variantOf"`
	// Attack is what the spell hits for on a target with no resistance.
	Attack *engine.Attack `json:"attack"`
}

type fightDTO struct {
	CurrentStats map[int]int   `json:"currentStats"`
	State        *fight.State  `json:"state"`
	Events       []fight.Event `json:"events"`
	// Refused says why the action was not done; empty when it was.
	Refused string `json:"refused,omitempty"`
	AP      int    `json:"ap"`
	MP      int    `json:"mp"`
	MaxAP   int    `json:"maxAp"`
	MaxMP   int    `json:"maxMp"`
	// Steps lists the cells the player can walk to.
	Steps  []fight.Step   `json:"steps"`
	Spells []fightSlotDTO `json:"spells"`
	// Names gives the name of each spell the events and the lasting effects
	// refer to.
	Names map[int]string `json:"names"`
}

// fightSim builds the simulator of a stuff.
func fightSim(eng *engine.Engine, req *fightRequest, lang string) *fight.Sim {
	res := eng.Evaluate(req.Build, lang)
	sim := &fight.Sim{
		Cat: eng.Catalog, Stat: func(id int) int { return res.Stats[id] }, Level: res.Level, Breed: req.BreedID,
		Rand: rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64())),
	}
	for _, id := range req.Items {
		if it := eng.Catalog.ItemByID[id]; it != nil && it.Passive != nil && it.Passive.SpellID != 0 {
			sim.Passives = append(sim.Passives, it.Passive.SpellID)
		}
	}
	sort.Ints(sim.Passives)
	if it := eng.Catalog.ItemByID[req.Items["weapon"]]; it != nil && it.Weapon != nil {
		sim.Weapon = it
		if mods := req.Mods["weapon"]; mods != nil {
			sim.WeaponElement = mods.Element
		}
	}
	return sim
}

// Bounds on a fight sent by the interface: far above what a fight holds, low
// enough that no request costs more than a few boards' worth of work.
const (
	maxFightEntities = 16
	maxFightBuffs    = 256
	maxFightUses     = 4096
	// Every entity's waiting effects together, and how long anything lasts
	// or waits: the game's "whole fight" is 63 turns.
	maxFightHooks = 512
	maxFightTurns = 64
)

// validState reports whether a fight sent by the interface can be played on:
// a board of a sane size, each side bounded on its own, with every entity
// on it and nothing piled up beyond what a fight can hold.
func validState(st *fight.State) bool {
	if st == nil {
		return false
	}
	w, h := st.Board.Width, st.Board.Height
	if w < 1 || h < 1 || w > fight.MaxSide || h > fight.MaxSide {
		return false
	}
	cells := w * h
	if len(st.Board.SightWalls) > cells || len(st.Board.Blue) > cells || len(st.Board.Red) > cells || len(st.Board.Walls) > cells || len(st.Entities) > maxFightEntities || len(st.Uses) > maxFightUses || st.Player() == nil {
		return false
	}
	if !fight.ValidMarks(st) || !fight.ValidPositions(st) || st.ActiveID != 0 && st.Active().ID != st.ActiveID || st.LastEntityID < 0 || st.LastEntityID > 1_000_000 {
		return false
	}
	seen := map[int]bool{}
	players := 0
	hooks := 0
	for _, e := range st.Entities {
		if seen[e.ID] || e.ID < 1 || e.ID > 1_000_000 || len(e.Uses) > maxFightUses || len(e.Stats) > 256 {
			return false
		}
		seen[e.ID] = true
		if e.Kind != fight.KindPlayer && e.Kind != fight.KindDummy && e.Kind != fight.KindSummon {
			return false
		}
		if e.Kind == fight.KindPlayer {
			players++
		}
		if e.Kind == fight.KindSummon && (e.MonsterID < 1 || e.Grade < 1) {
			return false
		}
		for _, use := range e.Uses {
			if use == nil || len(use.Targets) > maxFightEntities {
				return false
			}
		}
		if e.Cell < 0 || e.Cell >= cells || len(e.Buffs) > maxFightBuffs || len(e.Hooks) > maxFightBuffs {
			return false
		}
		hooks += len(e.Hooks)
		for _, h := range e.Hooks {
			if h.Index < 0 || h.Delay < 0 || h.Delay > maxFightTurns || h.Turns < -1 || h.Turns > maxFightTurns {
				return false
			}
		}
		for _, b := range e.Buffs {
			if b.Turns < -1 || b.Turns > maxFightTurns {
				return false
			}
		}
	}
	if players != 1 || hooks > maxFightHooks {
		return false
	}
	for id, use := range st.Uses {
		if use == nil || len(use.Targets) > maxFightEntities {
			delete(st.Uses, id)
		}
	}
	if st.Uses == nil {
		st.Uses = map[int]*fight.Use{}
	}
	return true
}

func (s *Server) handleFight(w http.ResponseWriter, r *http.Request, eng *engine.Engine, lang string) {
	var req fightRequest
	if !decodeBody(w, r, &req) {
		return
	}
	sim := fightSim(eng, &req, lang)
	sim.Creatures = s.fightCreatures()
	out := fightDTO{Events: []fight.Event{}, Spells: []fightSlotDTO{}, Names: map[int]string{}}
	st := req.State
	switch {
	case st == nil:
		dummies := fight.Dummies{Count: 1, HP: 50000}
		if req.Dummies != nil {
			dummies = *req.Dummies
		}
		if dummies.MapID != 0 {
			board, err := s.fightBoard(dummies.MapID)
			if err != nil {
				writeError(w, http.StatusBadRequest, "map_unavailable", "The selected combat map is unavailable.")
				return
			}
			st = sim.StartOn(dummies, board)
		} else {
			st = sim.Start(dummies)
		}
	case !validState(st):
		writeError(w, http.StatusBadRequest, "bad_request", "The fight sent cannot be played.")
		return
	}
	if a := req.Action; a != nil && req.State != nil {
		var events []fight.Event
		switch a.Type {
		case "place":
			out.Refused = sim.Place(st, a.Entity, a.Cell)
		case "begin":
			events, out.Refused = sim.Begin(st)
		case "move":
			events, out.Refused = sim.Move(st, a.Cell)
		case "cast":
			events, out.Refused = sim.Cast(st, a.Spell, a.Cell)
		case "end":
			events, out.Refused = sim.EndTurn(st)
		default:
			writeError(w, http.StatusBadRequest, "bad_request", "Unknown action.")
			return
		}
		out.Events = append(out.Events, events...)
	}
	out.State = st
	out.AP, out.MP = sim.Points(st)
	stat := sim.StatOf(st.Active())
	out.CurrentStats = map[int]int{}
	for _, id := range eng.Catalog.StatIDs {
		if value := stat(id); value != 0 {
			out.CurrentStats[id] = value
		}
	}
	out.MaxAP, out.MaxMP = stat(game.StatAP), stat(game.StatMP)
	out.Steps = sim.Reach(st)

	cat := eng.Catalog
	name := func(id int) {
		if sp := cat.SpellByID[id]; sp != nil {
			out.Names[id] = sp.Name.Get(lang)
		} else if id == fight.WeaponID && sim.Weapon != nil {
			out.Names[id] = sim.Weapon.Name.Get(lang)
		}
	}
	for _, mark := range st.Marks {
		name(mark.From)
		name(mark.Spell)
	}
	for _, ev := range out.Events {
		name(ev.Spell)
	}
	for i := range st.Entities {
		e := &st.Entities[i]
		if m := sim.Creatures[e.MonsterID]; m != nil {
			e.Name = m.Names.Get(lang)
		}
		for _, b := range e.Buffs {
			name(b.From)
		}
	}
	if slot := sim.Slot(st, fight.WeaponID); slot != nil {
		opts := engine.WeaponOptions{Element: sim.WeaponElement, Mastery: false}
		out.Spells = append(out.Spells, fightSlotDTO{
			Slot: *slot, Name: sim.Weapon.Name.Get(lang), IconID: sim.Weapon.IconID, Weapon: true,
			Attack: engine.WeaponAttack(sim.Weapon, opts, stat),
		})
	}
	breeds := []int{0}
	if cat.BreedByID[req.BreedID] != nil {
		breeds = []int{req.BreedID, 0}
	}
	var spells []*game.Spell
	if active := st.Active(); active.Kind == fight.KindSummon {
		for _, spell := range sim.SummonSpells(active) {
			if sp := cat.SpellByID[spell.ID]; sp != nil {
				spells = append(spells, sp)
			}
		}
	} else {
		for _, id := range breeds {
			spells = append(spells, cat.SpellsByBreed[id]...)
		}
	}
	for _, sp := range spells {
		slot := sim.Slot(st, sp.ID)
		if slot == nil {
			continue
		}
		out.Spells = append(out.Spells, fightSlotDTO{Slot: *slot, Name: sp.Name.Get(lang), IconID: sp.IconID, BreedID: sp.BreedID, VariantOf: sp.VariantOf, Attack: engine.SpellAttack(sp, sim.Level, engine.RangeAuto, stat)})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleFightAim(w http.ResponseWriter, r *http.Request, eng *engine.Engine, lang string) {
	var req fightRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if !validState(req.State) {
		writeError(w, http.StatusBadRequest, "bad_request", "The fight sent cannot be played.")
		return
	}
	sim := fightSim(eng, &req, lang)
	sim.Creatures = s.fightCreatures()
	aim := sim.Aim(req.State, req.Spell)
	if aim == nil {
		aim = &fight.Aim{Spell: req.Spell, Range: []int{}, Targets: []fight.Target{}}
	}
	writeJSON(w, http.StatusOK, aim)
}
