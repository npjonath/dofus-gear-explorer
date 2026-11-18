package rawdata

import (
	"sort"
	"strings"

	"dofusdb/api/internal/game"
)

// commonSpellType is the spell type of the spells every class can learn.
const commonSpellType = 463

// Flags of a spell effect. The game lists in a spell's description the
// effects visible in its tooltip; helper effects that only trigger others
// lack the flag. An effect described only is not applied by the spell itself
// but by something the spell sets up, such as a state on its target.
const (
	effectVisibleInTooltip = 1
	effectDescribedOnly    = 16
)

// Actions of the spell effects the damage panel computes besides damage.
const (
	actionPush          = 5
	actionHealPercent   = 1109
	actionBoostBaseHits = 293
)

// healActions maps the heals that scale with a characteristic to their element.
var healActions = map[int]int{
	108: game.ElementFire, 2998: game.ElementWater, 2999: game.ElementAir, 3000: game.ElementEarth,
}

// triggers maps the game's trigger of an effect to when it applies; "I" is
// as the spell is cast.
var triggers = map[string]string{"I": "", "TB": game.TriggerTurnStart, "TE": game.TriggerTurnEnd}

// triggerOnMPUse marks an effect that applies each time the caster uses MP.
const triggerOnMPUse = "MPARR"

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
	ActionID   int    `json:"actionId"`
	DiceNum    int    `json:"diceNum"`
	DiceSide   int    `json:"diceSide"`
	Value      int    `json:"value"`
	Triggers   string `json:"triggers"`
	Duration   int    `json:"duration"`
	Delay      int    `json:"delay"`
	Flags      int    `json:"m_flags"`
	TargetMask string `json:"targetMask"`
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

// attackLines keeps what the damage panel computes of a spell grade, among
// the effects the game shows in the description: damage and steals dealt as
// the spell is cast, heals, and pushbacks. Poisons and other damage dealt
// over time are left out. A line's critical values are those of the line at
// the same rank in the critical list.
func attackLines(level rawSpellLevel) []game.AttackLine {
	read := func(effects []rawSpellEffect) []game.AttackLine {
		var out []game.AttackLine
		real := false
		perMP := 0
		for _, e := range effects {
			if e.ActionID == actionBoostBaseHits && e.DiceNum == level.SpellID && strings.Contains(e.Triggers, triggerOnMPUse) {
				perMP = e.Value
			}
			trigger, timed := triggers[e.Triggers]
			if !timed || e.Flags&effectVisibleInTooltip == 0 {
				continue
			}
			lo, hi := e.DiceNum, e.DiceSide
			if hi == 0 {
				hi = lo
			}
			if lo > hi {
				lo, hi = hi, lo
			}
			line := game.AttackLine{
				Min: lo, Max: hi, Delay: e.Delay, Targets: e.TargetMask, Trigger: trigger,
				Conditional: e.Flags&effectDescribedOnly != 0,
			}
			if element, steal, ok := game.AttackAction(e.ActionID); ok {
				// Damage over several turns is a poison: not computed.
				if trigger != "" || e.Duration != 0 {
					continue
				}
				line.Element, line.Steal = element, steal
				real = real || !line.Conditional
			} else if element, ok := healActions[e.ActionID]; ok {
				line.Kind, line.Element = game.LineHeal, element
			} else if e.ActionID == actionHealPercent {
				line.Kind = game.LineHealPercent
			} else if e.ActionID == actionPush && trigger == "" {
				line.Kind = game.LinePush
			} else {
				continue
			}
			out = append(out, line)
		}
		for i := range out {
			if out[i].Kind != "" {
				continue
			}
			// When every damage line is described only, the spell deals its
			// damage through another one: those lines are its hit.
			out[i].Conditional = out[i].Conditional && real
			out[i].PerMP = perMP
		}
		return out
	}
	lines, critical := read(level.Effects.Array), read(level.Critical.Array)
	for i := range lines {
		line := &lines[i]
		if level.CritChance <= 0 || i >= len(critical) {
			continue
		}
		if c := critical[i]; c.Kind == line.Kind && c.Element == line.Element && c.Steal == line.Steal && c.Targets == line.Targets {
			line.CritMin, line.CritMax = c.Min, c.Max
		}
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
	// The common spells come in pairs too, filed under a class id no class
	// has: the spell every character knows, then the one that replaces it.
	isBreed := map[int]bool{}
	for _, b := range rawBreeds {
		isBreed[b.ID] = true
	}
	for rank, v := range rawVariants {
		ids := v.SpellIDs.Array
		if len(ids) != 2 || isBreed[v.BreedID] {
			continue
		}
		if _, known := owner[ids[0]]; known {
			continue
		}
		owner[ids[0]] = place{order: rank * 2}
		if _, known := owner[ids[1]]; !known {
			owner[ids[1]] = place{order: rank*2 + 1, variantOf: ids[0]}
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
