package engine

import (
	"sort"

	"dofusdb/api/internal/game"
)

// This file models what forgemagic leaves on an item, not the act of
// forging: no success odds, no leftover weight. A player states the result
// they want (an over, an exo, a transcendence rune, a dracorune) and the
// engine applies it and checks that the game allows it.

// Exo is a line added to an item that does not naturally have that stat.
type Exo struct {
	StatID int `json:"statId"`
	Value  int `json:"value"`
}

// Draco is a dracorune (update 3.8): it converts lines of an item.
type Draco struct {
	// Rune is one of the Draco* keys.
	Rune string `json:"rune"`
	// From is the element converted; unused by the ebony rune.
	From string `json:"from,omitempty"`
	// To is the element obtained; only the ivory rune lets the player choose.
	To string `json:"to,omitempty"`
}

// ItemMods is the forgemagic applied to one worn item.
type ItemMods struct {
	// Lines sets the value of natural lines, by stat id. A value above the
	// item's best roll is an over.
	Lines         map[int]int `json:"lines,omitempty"`
	Exos          []Exo       `json:"exos,omitempty"`
	Transcendence int         `json:"transcendence,omitempty"`
	Draco         *Draco      `json:"draco,omitempty"`
	// Element is the elemental forgemagic of a weapon: its Neutral damage
	// lines hit in that element instead. One of earth, fire, water, air.
	Element string `json:"element,omitempty"`
}

// Empty reports whether the mods change nothing.
func (m *ItemMods) Empty() bool {
	return m == nil || (len(m.Lines) == 0 && len(m.Exos) == 0 && m.Transcendence == 0 && m.Draco == nil && m.Element == "")
}

const (
	DracoTurquoise = "turquoise"
	DracoEmerald   = "emeraude"
	DracoCrimson   = "pourpre"
	DracoOchre     = "ocre"
	DracoIvory     = "ivoire"
	DracoEbony     = "ebene"
)

// Elements, in the game's order.
var Elements = []string{"neutral", "earth", "fire", "water", "air"}

// ElementStats lists, per element, the stats a dracorune can move.
type ElementStats struct {
	Stat       int `json:"statId"`
	Damage     int `json:"damageId"`
	ResFixed   int `json:"resFixedId"`
	ResPercent int `json:"resPercentId"`
}

// ElementTable maps each element to its characteristic ids. Neutral has no
// primary characteristic.
var ElementTable = map[string]ElementStats{
	"neutral": {Stat: -1, Damage: 92, ResFixed: 58, ResPercent: 37},
	"earth":   {Stat: game.StatStrength, Damage: 88, ResFixed: 54, ResPercent: 33},
	"fire":    {Stat: game.StatIntel, Damage: 89, ResFixed: 55, ResPercent: 34},
	"water":   {Stat: game.StatChance, Damage: 90, ResFixed: 56, ResPercent: 35},
	"air":     {Stat: game.StatAgility, Damage: 91, ResFixed: 57, ResPercent: 36},
}

// DracoTargets gives the element each offensive dracorune converts to. The
// devblog names Emerald as Agility; the other three follow the element of
// the primordial Dofus they are named after.
var DracoTargets = map[string]string{
	DracoOchre:     "earth",
	DracoCrimson:   "fire",
	DracoTurquoise: "water",
	DracoEmerald:   "air",
}

// DracoRunes lists every dracorune key, offensive ones first.
var DracoRunes = []string{DracoTurquoise, DracoEmerald, DracoCrimson, DracoOchre, DracoIvory, DracoEbony}

const (
	statAllDamage = 16
	// ebonyKeep is what the ebony rune keeps of the converted value.
	ebonyKeep = 0.7
)

// ForgeableSlots are the slots whose items can be forged. Dofus, trophies,
// pets and mounts cannot.
var ForgeableSlots = map[game.Slot]bool{
	game.SlotAmulet: true, game.SlotRing: true, game.SlotBelt: true, game.SlotBoots: true,
	game.SlotHat: true, game.SlotCloak: true, game.SlotWeapon: true, game.SlotShield: true,
}

// LimitedExos are the exos a character may carry only once across the whole
// stuff, and only one of per item: AP, MP and Range.
var LimitedExos = []int{game.StatAP, game.StatMP, game.StatRange}

func isLimitedExo(statID int) bool {
	for _, id := range LimitedExos {
		if id == statID {
			return true
		}
	}
	return false
}

// Line origins.
const (
	OriginBase          = "base"
	OriginOver          = "over"
	OriginExo           = "exo"
	OriginTranscendence = "transcendence"
	OriginDraco         = "draco"
)

// Forge issue codes, reported per item.
const (
	ForgeIssueSlot          = "forge_slot"
	ForgeIssueLine          = "forge_line"
	ForgeIssueExo           = "forge_exo"
	ForgeIssueExoLimit      = "forge_exo_limit"
	ForgeIssueWeight        = "forge_weight"
	ForgeIssueTranscendence = "forge_transcendence"
	ForgeIssueDraco         = "forge_draco"
	ForgeIssueElement       = "forge_element"
)

// ForgedLine is one stat line of an item after forgemagic.
type ForgedLine struct {
	StatID int `json:"statId"`
	// Min and Max are the item's natural range; both zero for added lines.
	Min int `json:"min"`
	Max int `json:"max"`
	// Cap is the highest value forgemagic allows on this line.
	Cap    int    `json:"cap"`
	Value  int    `json:"value"`
	Origin string `json:"origin"`
	// Rune is the part of Value that comes from the transcendence rune.
	Rune int `json:"rune"`
	// Natural is true for lines the item has before any forgemagic.
	Natural bool   `json:"natural"`
	Text    string `json:"text"`
}

// Forged is an item with its forgemagic applied.
type Forged struct {
	Lines  []ForgedLine
	Issues []string
	Vector []game.StatValue
	// Runes lists the transcendence runes the item can take as it stands.
	Runes []int
	// Weight is the forgemagic weight the overs and exos of the item use, out
	// of game.ForgeLineWeight.
	Weight float64
}

// overLine is a natural line forged above its best roll.
type overLine struct {
	// natural is the weight of the line at its best roll, over the weight
	// added above it.
	natural, over float64
}

// forgeWeight checks the weight rule of the game on the overs and exos of an
// item and returns the weight they use.
//
// When a rune would leave a line in over or exo, the game adds the whole
// weight of that line to the weight of the overs (the part above the best
// roll) and exos already on the other lines, and refuses the rune above 101.
// The result does not say in which order the runes went, so it is allowed
// when some order passes: exos last, which asks for a total within 101, and
// the overs from the heaviest natural line to the lightest.
func forgeWeight(overs []overLine, exos float64) (weight float64, ok bool) {
	const limit = game.ForgeLineWeight + 1e-9
	sort.Slice(overs, func(i, j int) bool { return overs[i].natural > overs[j].natural })
	ok = true
	for _, line := range overs {
		weight += line.over
		if line.natural+weight > limit {
			ok = false
		}
	}
	weight += exos
	return weight, ok && weight <= limit
}

type naturalRange struct{ min, max int }

// naturalLines returns the item's stat lines summed per stat, in item order.
func naturalLines(it *game.Item) (order []int, ranges map[int]naturalRange) {
	ranges = map[int]naturalRange{}
	for _, e := range it.Effects {
		if e.Kind != game.EffectStat {
			continue
		}
		r, seen := ranges[e.StatID]
		if !seen {
			order = append(order, e.StatID)
		}
		ranges[e.StatID] = naturalRange{r.min + e.Min, r.max + e.Max}
	}
	return order, ranges
}

// lineCap is the highest value a natural line can be forged to: the weight
// rule, unless the item's own roll already goes higher. A malus can be
// forged down to -1 and no further: at zero the line would be gone.
func (e *Engine) lineCap(statID, naturalMax int) int {
	st := e.Catalog.StatByID[statID]
	if st == nil {
		return naturalMax
	}
	if naturalMax < 0 {
		if st.Weight <= 0 {
			return naturalMax
		}
		return -1
	}
	if forged := st.ForgeMax(); forged > naturalMax {
		return forged
	}
	return naturalMax
}

// runeFits reports whether a transcendence rune can land on a line holding
// value: the weight of the line plus the density of the rune must stay
// within the weight one line may carry.
func (e *Engine) runeFits(rn *game.Rune, value int) bool {
	st := e.Catalog.StatByID[rn.StatID]
	if st == nil {
		return false
	}
	return st.LineWeight(value)+float64(rn.Density) <= game.ForgeLineWeight+1e-9
}

// Forge applies mods to an item. Invalid requests are reported in Issues and
// applied as far as they make sense, so the player still sees numbers.
func (e *Engine) Forge(it *game.Item, mods *ItemMods) Forged {
	cat := e.Catalog
	order, natural := naturalLines(it)
	values := make(map[int]int, len(order)+4)
	origin := make(map[int]string, len(order)+4)
	for _, id := range order {
		values[id] = natural[id].max
		origin[id] = OriginBase
	}
	var out Forged
	issue := func(code string) {
		for _, existing := range out.Issues {
			if existing == code {
				return
			}
		}
		out.Issues = append(out.Issues, code)
	}
	addLine := func(statID, delta int, from string) {
		if _, ok := values[statID]; !ok {
			listed := false
			for _, id := range order {
				listed = listed || id == statID
			}
			if !listed {
				order = append(order, statID)
			}
			origin[statID] = from
		} else if from != OriginTranscendence {
			origin[statID] = from
		}
		values[statID] += delta
	}

	if !mods.Empty() && !ForgeableSlots[it.Slot] {
		issue(ForgeIssueSlot)
		mods = nil
	}

	// forged is true once the item holds an over or an exo.
	forged := false
	runeBonus := map[int]int{}
	if !mods.Empty() && mods.Element != "" && !CanTakeElement(it, mods.Element) {
		issue(ForgeIssueElement)
	}

	if !mods.Empty() {
		lineIDs := make([]int, 0, len(mods.Lines))
		for id := range mods.Lines {
			lineIDs = append(lineIDs, id)
		}
		sort.Ints(lineIDs)
		for _, id := range lineIDs {
			value := mods.Lines[id]
			r, ok := natural[id]
			if !ok {
				issue(ForgeIssueLine)
				continue
			}
			if limit := e.lineCap(id, r.max); value > limit {
				issue(ForgeIssueLine)
				value = limit
			}
			if value < r.min {
				issue(ForgeIssueLine)
				value = r.min
			}
			values[id] = value
			if value > r.max {
				origin[id] = OriginOver
				forged = true
			}
		}

		// What the overs weigh. Raising a malus towards -1 is an over too,
		// and a rune weighs half on a negative line.
		var overs []overLine
		for _, id := range order {
			r := natural[id]
			st := cat.StatByID[id]
			if st == nil || st.Weight <= 0 || values[id] <= r.max {
				continue
			}
			raised := float64(values[id]-r.max) * st.Weight
			if r.max < 0 {
				overs = append(overs, overLine{over: raised / 2})
			} else {
				overs = append(overs, overLine{natural: st.LineWeight(r.max), over: raised})
			}
		}

		limited := 0
		exoWeight := 0.0
		seen := map[int]bool{}
		for _, exo := range mods.Exos {
			st := cat.StatByID[exo.StatID]
			_, isNatural := natural[exo.StatID]
			if st == nil || isNatural || seen[exo.StatID] || exo.Value < 1 || exo.Value > st.ForgeMax() {
				issue(ForgeIssueExo)
				continue
			}
			if isLimitedExo(exo.StatID) {
				limited++
				if limited > 1 {
					issue(ForgeIssueExo)
					continue
				}
			}
			seen[exo.StatID] = true
			addLine(exo.StatID, exo.Value, OriginExo)
			exoWeight += st.LineWeight(exo.Value)
			forged = true
		}

		weight, fits := forgeWeight(overs, exoWeight)
		out.Weight = weight
		if !fits {
			issue(ForgeIssueWeight)
		}

		if mods.Draco != nil {
			if !e.applyDraco(mods.Draco, natural, values, origin, &order) {
				issue(ForgeIssueDraco)
			}
		}
	}

	// The transcendence rune comes last: it seals the item, so its weight
	// rule is judged on the lines as they stand, dracorune included. It
	// needs an item with neither over nor exo.
	if ForgeableSlots[it.Slot] && !forged {
		for i := range cat.Runes {
			if rn := &cat.Runes[i]; e.runeFits(rn, values[rn.StatID]) {
				out.Runes = append(out.Runes, rn.ID)
			}
		}
	}
	if !mods.Empty() && mods.Transcendence != 0 {
		rn := cat.RuneByID[mods.Transcendence]
		if rn == nil {
			issue(ForgeIssueTranscendence)
		} else {
			if forged || !e.runeFits(rn, values[rn.StatID]) {
				issue(ForgeIssueTranscendence)
			}
			addLine(rn.StatID, rn.Value, OriginTranscendence)
			runeBonus[rn.StatID] = rn.Value
		}
	}

	for _, id := range order {
		value, ok := values[id]
		if !ok {
			continue
		}
		r, isNatural := natural[id]
		line := ForgedLine{StatID: id, Value: value, Origin: origin[id], Rune: runeBonus[id], Natural: isNatural}
		if isNatural {
			line.Min, line.Max, line.Cap = r.min, r.max, e.lineCap(id, r.max)
		} else if st := cat.StatByID[id]; st != nil {
			line.Cap = st.ForgeMax()
		}
		out.Lines = append(out.Lines, line)
		if idx := cat.Idx(id); idx >= 0 && value != 0 {
			out.Vector = append(out.Vector, game.StatValue{Index: idx, Value: int32(value)})
		}
	}
	return out
}

// applyDraco converts lines in place. It reports false when the rune cannot
// be used that way on this item.
func (e *Engine) applyDraco(d *Draco, natural map[int]naturalRange, values map[int]int, origin map[int]string, order *[]int) bool {
	has := func(id int) bool { _, ok := natural[id]; return ok && id >= 0 }

	// move turns the line of one stat into another, keeping its place.
	move := func(from, to int) bool {
		value, ok := values[from]
		if !ok || from < 0 || to < 0 || e.Catalog.StatByID[to] == nil {
			return false
		}
		delete(values, from)
		delete(origin, from)
		if _, exists := values[to]; exists {
			values[to] += value
			origin[to] = OriginDraco
			return true
		}
		for i, id := range *order {
			if id == from {
				(*order)[i] = to
			}
		}
		values[to] = value
		origin[to] = OriginDraco
		return true
	}

	switch d.Rune {
	case DracoTurquoise, DracoEmerald, DracoCrimson, DracoOchre:
		target := DracoTargets[d.Rune]
		from, ok := ElementTable[d.From]
		to := ElementTable[target]
		// The element obtained must not already be on the item.
		if !ok || d.From == target || d.From == "neutral" || has(to.Stat) || has(to.Damage) {
			return false
		}
		movedStat := move(from.Stat, to.Stat)
		movedDamage := move(from.Damage, to.Damage)
		return movedStat || movedDamage

	case DracoIvory:
		from, okFrom := ElementTable[d.From]
		to, okTo := ElementTable[d.To]
		if !okFrom || !okTo || d.From == d.To || has(to.ResFixed) || has(to.ResPercent) {
			return false
		}
		movedFixed := move(from.ResFixed, to.ResFixed)
		movedPercent := move(from.ResPercent, to.ResPercent)
		return movedFixed || movedPercent

	case DracoEbony:
		// Positive elemental lines merge into one Power line and one Damage
		// line worth 70% of the highest; negative lines stay as they are.
		merge := func(ids []int, into int) bool {
			best := 0
			for _, id := range ids {
				if value := values[id]; value > 0 {
					if value > best {
						best = value
					}
					delete(values, id)
					delete(origin, id)
				}
			}
			if best == 0 || e.Catalog.StatByID[into] == nil {
				return false
			}
			if _, exists := values[into]; !exists {
				*order = append(*order, into)
			}
			values[into] += int(float64(best)*ebonyKeep + 1e-9)
			origin[into] = OriginDraco
			return true
		}
		var stats, damages []int
		for _, name := range Elements {
			el := ElementTable[name]
			if el.Stat >= 0 {
				stats = append(stats, el.Stat)
			}
			damages = append(damages, el.Damage)
		}
		mergedStats := merge(stats, game.StatPower)
		mergedDamages := merge(damages, statAllDamage)
		return mergedStats || mergedDamages
	}
	return false
}

// DracoOptions lists the conversions a dracorune can do on an item, judged
// on its natural lines.
func (e *Engine) DracoOptions(it *game.Item) []Draco {
	if !ForgeableSlots[it.Slot] {
		return nil
	}
	_, natural := naturalLines(it)
	has := func(id int) bool { _, ok := natural[id]; return ok && id >= 0 }
	positive := func(id int) bool { r, ok := natural[id]; return ok && id >= 0 && r.max > 0 }

	var out []Draco
	for _, key := range []string{DracoTurquoise, DracoEmerald, DracoCrimson, DracoOchre} {
		target := DracoTargets[key]
		to := ElementTable[target]
		if has(to.Stat) || has(to.Damage) {
			continue
		}
		for _, name := range Elements {
			from := ElementTable[name]
			if name == target || name == "neutral" || !(has(from.Stat) || has(from.Damage)) {
				continue
			}
			out = append(out, Draco{Rune: key, From: name, To: target})
		}
	}
	for _, source := range Elements {
		from := ElementTable[source]
		if !(has(from.ResFixed) || has(from.ResPercent)) {
			continue
		}
		for _, target := range Elements {
			to := ElementTable[target]
			if target == source || has(to.ResFixed) || has(to.ResPercent) {
				continue
			}
			out = append(out, Draco{Rune: DracoIvory, From: source, To: target})
		}
	}
	for _, name := range Elements {
		el := ElementTable[name]
		if positive(el.Stat) || positive(el.Damage) {
			out = append(out, Draco{Rune: DracoEbony})
			break
		}
	}
	return out
}
