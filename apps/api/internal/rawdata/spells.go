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

type rawZone struct {
	Shape    int `json:"shape"`
	Param1   int `json:"param1"`
	Param2   int `json:"param2"`
	Decrease int `json:"damageDecreaseStepPercent"`
	Steps    int `json:"maxDamageDecreaseApplyCount"`
}

type rawSpellEffect struct {
	ActionID   int     `json:"actionId"`
	DiceNum    int     `json:"diceNum"`
	DiceSide   int     `json:"diceSide"`
	Value      int     `json:"value"`
	Triggers   string  `json:"triggers"`
	Duration   int     `json:"duration"`
	Delay      int     `json:"delay"`
	Flags      int     `json:"m_flags"`
	TargetMask string  `json:"targetMask"`
	Lasts      int     `json:"effectTriggerDuration"`
	Random     float64 `json:"random"`
	Group      int     `json:"group"`
	Zone       rawZone `json:"zoneDescr"`
}

// Flags of a spell grade: how it is aimed.
const (
	castInLine       = 1
	castInDiagonal   = 2
	castNeedsSight   = 4
	castNeedsFree    = 8
	castNeedsTaken   = 16
	castRangeBoosted = 64
)

type rawSpellLevel struct {
	ID         int                   `json:"id"`
	SpellID    int                   `json:"spellId"`
	Grade      int                   `json:"grade"`
	MinLevel   int                   `json:"minPlayerLevel"`
	APCost     int                   `json:"apCost"`
	MinRange   int                   `json:"minRange"`
	Range      int                   `json:"range"`
	CritChance int                   `json:"criticalHitProbability"`
	Flags      int                   `json:"m_flags"`
	PerTurn    int                   `json:"maxCastPerTurn"`
	PerTarget  int                   `json:"maxCastPerTarget"`
	Interval   int                   `json:"minCastInterval"`
	Initial    int                   `json:"initialCooldown"`
	MaxStack   int                   `json:"maxStack"`
	Effects    array[rawSpellEffect] `json:"effects"`
	Critical   array[rawSpellEffect] `json:"criticalEffect"`
}

// castRules reads how a spell grade is aimed and how often it can be cast.
func castRules(level rawSpellLevel) game.CastRules {
	return game.CastRules{
		Line: level.Flags&castInLine != 0, Diagonal: level.Flags&castInDiagonal != 0,
		Sight: level.Flags&castNeedsSight != 0, FreeCell: level.Flags&castNeedsFree != 0,
		TakenCell: level.Flags&castNeedsTaken != 0, BoostRange: level.Flags&castRangeBoosted != 0,
		PerTurn: level.PerTurn, PerTarget: level.PerTarget, Interval: level.Interval,
		InitialCooldown: level.Initial, MaxStack: level.MaxStack,
	}
}

// unsignedBoosts gives the sign of the boosts the game data does not sign:
// final damage and resistance in percent, and the Power kept for spells or
// for weapons.
var unsignedBoosts = map[int]int{1171: 1, 1172: -1, 1076: 1, 1077: -1, 1054: 1, 1144: 1, 776: 1, 3804: -1}

// startSpellAdmin is how the game data labels the spells a fight starts
// with: those that set a class's states and passive effects.
const startSpellAdmin = "Sort initial"

type rawSpellType struct {
	ID         int     `json:"id"`
	LongNameID flexInt `json:"longNameId"`
}

// fightEffects keeps every effect of a grade as the game data gives it, for
// the fight simulator, which applies them one by one.
func fightEffects(effects []rawSpellEffect, defs map[int]rawEffect) []game.SpellEffect {
	out := make([]game.SpellEffect, 0, len(effects))
	for _, e := range effects {
		trigger := e.Triggers
		if trigger == "I" {
			trigger = ""
		}
		shape := ""
		if e.Zone.Shape > 32 && e.Zone.Shape < 127 {
			shape = string(rune(e.Zone.Shape))
		}
		// An effect that raises or lowers a characteristic says which one.
		stat, sign := 0, 0
		if def, ok := defs[e.ActionID]; ok && def.Characteristic != 0 {
			if def.BonusType != 0 {
				stat, sign = def.Characteristic, def.BonusType
			} else if known := unsignedBoosts[e.ActionID]; known != 0 {
				stat, sign = def.Characteristic, known
			}
		}
		out = append(out, game.SpellEffect{
			Stat: stat, Sign: sign,
			Action: e.ActionID, Min: e.DiceNum, Max: e.DiceSide, Value: e.Value, Targets: e.TargetMask,
			Duration: e.Duration, Delay: e.Delay, Trigger: trigger, Random: e.Random, Group: e.Group,
			TriggerTurns: e.Lasts,
			Zone:         game.Zone{Shape: shape, Size: e.Zone.Param1, Min: e.Zone.Param2, Decrease: e.Zone.Decrease, Steps: e.Zone.Steps},
			Shown:        e.Flags&effectVisibleInTooltip != 0,
			Described:    e.Flags&effectDescribedOnly != 0,
		})
	}
	return out
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
func parseSpells(dir string, spells []rawSpellHead, defs map[int]rawEffect, passives []int, want func(game.Texts, flexInt)) ([]game.Breed, []game.Spell, map[int]game.Texts, error) {
	breedRefs, err := readRefs(dir, "breeds.json")
	if err != nil {
		return nil, nil, nil, err
	}
	rawBreeds, err := decodeClass[rawBreed](breedRefs, "BreedData")
	if err != nil {
		return nil, nil, nil, err
	}
	variantRefs, err := readRefs(dir, "spell_variants.json")
	if err != nil {
		return nil, nil, nil, err
	}
	rawVariants, err := decodeClass[rawVariant](variantRefs, "SpellVariantData")
	if err != nil {
		return nil, nil, nil, err
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
		return nil, nil, nil, err
	}
	rawLevels := map[int][]rawSpellLevel{}
	for _, ref := range levelRefs {
		level, err := decodeOne[rawSpellLevel](ref)
		if err != nil {
			return nil, nil, nil, err
		}
		rawLevels[level.SpellID] = append(rawLevels[level.SpellID], level)
	}
	// The spells a fight starts with are filed by type, one type per class:
	// they are kept with their type's name, which says the class once the
	// texts are read.
	typeRefs, err := readRefs(dir, "spell_types.json")
	if err != nil {
		return nil, nil, nil, err
	}
	rawTypes, err := decodeClass[rawSpellType](typeRefs, "SpellTypeData")
	if err != nil {
		return nil, nil, nil, err
	}
	typeNameID := map[int]flexInt{}
	for _, t := range rawTypes {
		typeNameID[t.ID] = t.LongNameID
	}
	startTypes := map[int]game.Texts{}
	typeNames := map[int]game.Texts{}
	for _, sp := range spells {
		if _, known := owner[sp.ID]; known || !strings.HasPrefix(sp.AdminName, startSpellAdmin) || len(rawLevels[sp.ID]) == 0 {
			continue
		}
		if typeNames[sp.TypeID] == nil {
			typeNames[sp.TypeID] = game.Texts{}
			want(typeNames[sp.TypeID], typeNameID[sp.TypeID])
		}
		owner[sp.ID] = place{breed: game.BreedTriggered}
		startTypes[sp.ID] = typeNames[sp.TypeID]
	}

	for _, id := range passives {
		if _, known := owner[id]; !known && len(rawLevels[id]) > 0 {
			owner[id] = place{breed: game.BreedTriggered}
		}
	}

	// A spell often works through others it casts in turn, which no
	// character can cast directly: the simulator needs those too.
	monsterRefs, err := readRefs(dir, "monsters.json")
	if err != nil {
		return nil, nil, nil, err
	}
	monsters, err := decodeClass[struct {
		ID     int        `json:"id"`
		Spells array[int] `json:"spells"`
		Grades array[struct {
			Start int `json:"startingSpellId"`
		}] `json:"grades"`
	}](monsterRefs, "MonsterData")
	if err != nil {
		return nil, nil, nil, err
	}
	levelSpell := map[int]int{}
	for spell, levels := range rawLevels {
		for _, l := range levels {
			levelSpell[l.ID] = spell
		}
	}
	summonSpells := map[int][]int{}
	for _, m := range monsters {
		summonSpells[m.ID] = m.Spells.Array
		for _, grade := range m.Grades.Array {
			if grade.Start > 0 {
				summonSpells[m.ID] = append(summonSpells[m.ID], levelSpell[grade.Start])
			}
		}
	}
	queue := make([]int, 0, len(owner))
	for id := range owner {
		queue = append(queue, id)
	}
	for len(queue) > 0 {
		id := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		for _, level := range rawLevels[id] {
			for _, list := range [][]rawSpellEffect{level.Effects.Array, level.Critical.Array} {
				for _, e := range list {
					var linked []int
					switch {
					case game.CastsSpell(e.ActionID) || game.PlacesMark(e.ActionID):
						linked = []int{e.DiceNum}
					case game.SummonsCreature(e.ActionID):
						linked = summonSpells[e.DiceNum]
					}
					for _, child := range linked {
						if _, known := owner[child]; !known && len(rawLevels[child]) > 0 {
							owner[child] = place{breed: game.BreedTriggered}
							queue = append(queue, child)
						}
					}
				}
			}
		}
	}
	levelsBySpell := map[int][]game.SpellLevel{}
	for id := range owner {
		for _, level := range rawLevels[id] {
			levelsBySpell[id] = append(levelsBySpell[id], game.SpellLevel{
				Grade: level.Grade, MinLevel: level.MinLevel, APCost: level.APCost,
				RangeMin: level.MinRange, RangeMax: level.Range, CritChance: level.CritChance,
				Lines: attackLines(level),
				Cast:  castRules(level), Effects: fightEffects(level.Effects.Array, defs), CritEffects: fightEffects(level.Critical.Array, defs),
			})
		}
	}

	var out []game.Spell
	for id, at := range owner {
		head, ok := headByID[id]
		levels := levelsBySpell[id]
		if len(levels) == 0 || (!ok && at.breed != game.BreedTriggered) {
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
	return breeds, out, startTypes, nil
}

// startKeywords is what is left of a starting type's name once the class is
// taken out of it, in the languages the game names its types in.
var startKeywords = []string{"sorts initiaux", "sort initial", "starting spells", "starting spell"}

// assignStartSpells says, once the texts are read, which class each starting
// spell belongs to: the one its type is named after.
func assignStartSpells(ds *game.Dataset, startTypes map[int]game.Texts) {
	for i := range ds.Spells {
		sp := &ds.Spells[i]
		names, ok := startTypes[sp.ID]
		if !ok {
			continue
		}
		for _, breed := range ds.Breeds {
			for lang, typeName := range names {
				class := strings.ToLower(breed.Name[lang])
				rest, found := strings.CutSuffix(strings.ToLower(typeName), class)
				if !found {
					rest, found = strings.CutPrefix(strings.ToLower(typeName), class)
				}
				if class == "" || !found {
					continue
				}
				for _, keyword := range startKeywords {
					if strings.TrimSpace(rest) == keyword {
						sp.StartsFor = breed.ID
					}
				}
			}
		}
	}
}
