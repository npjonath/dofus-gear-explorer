package rawdata

import (
	"sort"

	"dofusdb/api/internal/game"
)

// commonSpellType is the spell type of the spells every class can learn.
const commonSpellType = 463

// effectVisibleInTooltip is the flag of the effects the game lists in a
// spell's description; helper effects that only trigger others lack it.
const effectVisibleInTooltip = 1

type rawBreed struct {
	ID          int        `json:"id"`
	ShortNameID flexInt    `json:"shortNameId"`
	SortIndex   int        `json:"sortIndex"`
	SpellIDs    array[int] `json:"breedSpellsId"`
}

type rawVariant struct {
	BreedID  int        `json:"breedId"`
	SpellIDs array[int] `json:"spellIds"`
}

type rawSpellEffect struct {
	ActionID int    `json:"actionId"`
	DiceNum  int    `json:"diceNum"`
	DiceSide int    `json:"diceSide"`
	Triggers string `json:"triggers"`
	Duration int    `json:"duration"`
	Delay    int    `json:"delay"`
	Flags    int    `json:"m_flags"`
}

type rawSpellLevel struct {
	ID         int                   `json:"id"`
	SpellID    int                   `json:"spellId"`
	Grade      int                   `json:"grade"`
	MinLevel   int                   `json:"minPlayerLevel"`
	APCost     int                   `json:"apCost"`
	MinRange   int                   `json:"minRange"`
	Range      int                   `json:"range"`
	CritChance int                   `json:"criticalHitProbability"`
	Effects    array[rawSpellEffect] `json:"effects"`
	Critical   array[rawSpellEffect] `json:"criticalEffect"`
}

// attackLines keeps the direct damage of a spell grade: instant damage and
// steal lines the game shows in the description. Poisons, triggered damage
// and heals are left out. A line's critical values are those of the line at
// the same rank in the critical list.
func attackLines(level rawSpellLevel) []game.AttackLine {
	type hit struct {
		element int
		steal   bool
		lo, hi  int
		delay   int
	}
	direct := func(effects []rawSpellEffect) []hit {
		var out []hit
		for _, e := range effects {
			element, steal, ok := game.AttackAction(e.ActionID)
			if !ok || e.Triggers != "I" || e.Duration != 0 || e.Flags&effectVisibleInTooltip == 0 {
				continue
			}
			lo, hi := e.DiceNum, e.DiceSide
			if hi == 0 {
				hi = lo
			}
			if lo > hi {
				lo, hi = hi, lo
			}
			out = append(out, hit{element, steal, lo, hi, e.Delay})
		}
		return out
	}
	normal, critical := direct(level.Effects.Array), direct(level.Critical.Array)
	lines := make([]game.AttackLine, 0, len(normal))
	for i, h := range normal {
		line := game.AttackLine{Element: h.element, Steal: h.steal, Min: h.lo, Max: h.hi, Delay: h.delay}
		if level.CritChance > 0 && i < len(critical) && critical[i].element == h.element && critical[i].steal == h.steal {
			line.CritMin, line.CritMax = critical[i].lo, critical[i].hi
		}
		lines = append(lines, line)
	}
	return lines
}

// parseSpells reads the classes and the spells a character can cast: each
// class's spells and their variants, and the common spells. Spell names are
// requested through want.
func parseSpells(dir string, spells []rawSpellHead, want func(game.Texts, flexInt)) ([]game.Breed, []game.Spell, error) {
	breedRefs, err := readRefs(dir, "breeds.json")
	if err != nil {
		return nil, nil, err
	}
	rawBreeds, err := decodeClass[rawBreed](breedRefs, "BreedData")
	if err != nil {
		return nil, nil, err
	}
	variantRefs, err := readRefs(dir, "spell_variants.json")
	if err != nil {
		return nil, nil, err
	}
	rawVariants, err := decodeClass[rawVariant](variantRefs, "SpellVariantData")
	if err != nil {
		return nil, nil, err
	}

	headByID := make(map[int]rawSpellHead, len(spells))
	for _, sp := range spells {
		headByID[sp.ID] = sp
	}

	// owner gives, for every castable spell, its class, rank and base spell.
	type place struct{ breed, order, variantOf int }
	owner := map[int]place{}
	var breeds []game.Breed
	for _, b := range rawBreeds {
		breed := game.Breed{ID: b.ID, Order: b.SortIndex, Name: game.Texts{}}
		want(breed.Name, b.ShortNameID)
		breeds = append(breeds, breed)
		for rank, id := range b.SpellIDs.Array {
			owner[id] = place{breed: b.ID, order: rank * 2}
		}
	}
	for _, v := range rawVariants {
		ids := v.SpellIDs.Array
		if len(ids) != 2 {
			continue
		}
		base, ok := owner[ids[0]]
		if !ok || base.breed != v.BreedID {
			continue
		}
		if _, known := owner[ids[1]]; !known {
			owner[ids[1]] = place{breed: v.BreedID, order: base.order + 1, variantOf: ids[0]}
		}
	}
	for _, sp := range spells {
		if _, known := owner[sp.ID]; !known && sp.TypeID == commonSpellType {
			owner[sp.ID] = place{order: sp.ID}
		}
	}

	levelRefs, err := readRefs(dir, "spell_levels.json")
	if err != nil {
		return nil, nil, err
	}
	levelsBySpell := map[int][]game.SpellLevel{}
	for _, ref := range levelRefs {
		level, err := decodeOne[rawSpellLevel](ref)
		if err != nil {
			return nil, nil, err
		}
		if _, castable := owner[level.SpellID]; !castable {
			continue
		}
		levelsBySpell[level.SpellID] = append(levelsBySpell[level.SpellID], game.SpellLevel{
			Grade: level.Grade, MinLevel: level.MinLevel, APCost: level.APCost,
			RangeMin: level.MinRange, RangeMax: level.Range, CritChance: level.CritChance,
			Lines: attackLines(level),
		})
	}

	var out []game.Spell
	for id, at := range owner {
		head, ok := headByID[id]
		levels := levelsBySpell[id]
		if !ok || len(levels) == 0 {
			continue
		}
		sort.Slice(levels, func(a, b int) bool { return levels[a].Grade < levels[b].Grade })
		spell := game.Spell{
			ID: id, IconID: head.IconID, BreedID: at.breed, Order: at.order, VariantOf: at.variantOf,
			Name: game.Texts{}, Levels: levels,
		}
		want(spell.Name, head.NameID)
		out = append(out, spell)
	}
	sort.Slice(out, func(a, b int) bool { return out[a].ID < out[b].ID })
	sort.Slice(breeds, func(a, b int) bool { return breeds[a].Order < breeds[b].Order })
	return breeds, out, nil
}
