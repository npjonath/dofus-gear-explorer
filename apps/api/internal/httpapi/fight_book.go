package httpapi

import (
	"net/http"
	"slices"
	"strconv"

	"dofusdb/api/internal/engine"
	"dofusdb/api/internal/game"
)

type bookSpell struct {
	ID          int      `json:"id"`
	BreedID     int      `json:"breedId"`
	VariantOf   int      `json:"variantOf"`
	IconID      int      `json:"iconId"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Levels      []int    `json:"levels"`
	Tags        []string `json:"tags"`
}

var elementTags = map[int]string{game.ElementNeutral: "neutral", game.ElementEarth: "earth", game.ElementFire: "fire", game.ElementWater: "water", game.ElementAir: "air", game.ElementBest: "variable", game.ElementWorst: "variable"}

func bookTags(cat *game.Catalog, root *game.Spell) []string {
	tags := map[string]bool{}
	seen := map[int]bool{}
	var visit func(*game.Spell)
	visit = func(sp *game.Spell) {
		if sp == nil || seen[sp.ID] || len(sp.Levels) == 0 {
			return
		}
		seen[sp.ID] = true
		level := sp.Levels[len(sp.Levels)-1]
		for _, line := range level.Lines {
			if line.Kind == game.LineHeal || line.Kind == game.LineHealPercent {
				tags["heal"] = true
			} else {
				tags["damage"] = true
			}
			if tag := elementTags[line.Element]; tag != "" {
				tags[tag] = true
			}
			if line.Steal {
				tags["heal"] = true
			}
		}
		for _, e := range level.Effects {
			if game.CastsSpell(e.Action) || game.PlacesMark(e.Action) {
				visit(cat.SpellByID[e.Min])
			}
			switch {
			case game.SummonsCreature(e.Action):
				tags["summon"] = true
			case e.Action == 1020 || e.Action == 1039 || e.Action == 1040 || e.Action == 265 || e.Action == 1163:
				tags["protection"] = true
			case e.Action == 4 || e.Action == 5 || e.Action == 6 || e.Action == 8 || e.Action == 50 || e.Action == 51 || e.Action == 1181 || (e.Action >= 1099 && e.Action <= 1106):
				tags["placement"] = true
			case e.Action == 90 || e.Action == 2935 || e.Action == 2973 || e.Action == 3002:
				tags["heal"] = true
			case e.Sign > 0:
				tags["bonus"] = true
			case e.Sign < 0 || e.Action == 1079 || e.Action == 1080:
				tags["penalty"] = true
			case e.Action == 150 || e.Action == 202 || game.PlacesMark(e.Action):
				tags["special"] = true
			}
		}
	}
	visit(root)
	if len(tags) == 0 {
		tags["special"] = true
	}
	list := make([]string, 0, len(tags))
	for tag := range tags {
		list = append(list, tag)
	}
	slices.Sort(list)
	return list
}

func (s *Server) handleFightBook(w http.ResponseWriter, r *http.Request, eng *engine.Engine, lang string) {
	breed, err := strconv.Atoi(r.URL.Query().Get("breed"))
	if err != nil || (breed != 0 && eng.Catalog.BreedByID[breed] == nil) {
		writeError(w, http.StatusBadRequest, "bad_request", "Unknown class.")
		return
	}
	breeds := []int{breed}
	if breed != 0 {
		breeds = append(breeds, 0)
	}
	out := []bookSpell{}
	for _, id := range breeds {
		for _, sp := range eng.Catalog.SpellsByBreed[id] {
			item := bookSpell{ID: sp.ID, BreedID: sp.BreedID, VariantOf: sp.VariantOf, IconID: sp.IconID, Name: sp.Name.Get(lang), Description: sp.Description.Get(lang), Tags: bookTags(eng.Catalog, sp)}
			for _, grade := range sp.Levels {
				item.Levels = append(item.Levels, grade.MinLevel)
			}
			out = append(out, item)
		}
	}
	writeJSON(w, http.StatusOK, out)
}
