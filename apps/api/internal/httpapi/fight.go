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
	State  *fight.State  `json:"state"`
	Events []fight.Event `json:"events"`
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
	if len(st.Board.Walls) > cells || len(st.Entities) > maxFightEntities || len(st.Uses) > maxFightUses || st.Player() == nil {
		return false
	}
	for _, e := range st.Entities {
		if e.Cell < 0 || e.Cell >= cells || len(e.Buffs) > maxFightBuffs || len(e.Hooks) > maxFightBuffs {
			return false
		}
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
	out := fightDTO{Events: []fight.Event{}, Spells: []fightSlotDTO{}, Names: map[int]string{}}
	st := req.State
	switch {
	case st == nil:
		dummies := fight.Dummies{Count: 1, HP: 50000}
		if req.Dummies != nil {
			dummies = *req.Dummies
		}
		st = sim.Start(dummies)
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
	stat := sim.StatOf(st.Player())
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
	for _, ev := range out.Events {
		name(ev.Spell)
	}
	for _, e := range st.Entities {
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
	for _, breedID := range breeds {
		for _, sp := range cat.SpellsByBreed[breedID] {
			slot := sim.Slot(st, sp.ID)
			if slot == nil {
				continue
			}
			out.Spells = append(out.Spells, fightSlotDTO{
				Slot: *slot, Name: sp.Name.Get(lang), IconID: sp.IconID, BreedID: sp.BreedID, VariantOf: sp.VariantOf,
				Attack: engine.SpellAttack(sp, sim.Level, engine.RangeAuto, stat),
			})
		}
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
	aim := fightSim(eng, &req, lang).Aim(req.State, req.Spell)
	if aim == nil {
		aim = &fight.Aim{Spell: req.Spell, Range: []int{}, Targets: []fight.Target{}}
	}
	writeJSON(w, http.StatusOK, aim)
}
