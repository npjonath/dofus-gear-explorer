// Package engine computes the characteristics of a character wearing a
// set of items: base values, item rolls, set bonuses and derived stats.
package engine

import (
	"math"
	"sort"
	"strconv"
	"strings"

	"dofusdb/api/internal/game"
)

const MaxLevel = 200

// The game caps these stats. Gear and exos can add up past the cap, and the
// stuff stays valid: the game simply keeps the cap and the rest is lost.
const (
	MaxAP    = 12
	MaxMP    = 6
	MaxRange = 6
)

// PrimaryStats are the characteristics a player spends points in.
var PrimaryStats = []string{"vitality", "wisdom", "strength", "intelligence", "chance", "agility"}

var primaryStatIDs = map[string]int{
	"vitality":     game.StatVitality,
	"wisdom":       game.StatWisdom,
	"strength":     game.StatStrength,
	"intelligence": game.StatIntel,
	"chance":       game.StatChance,
	"agility":      game.StatAgility,
}

// Character is the player's own contribution to a build.
type Character struct {
	Level int `json:"level"`
	// Base is the value bought with characteristic points, per primary stat.
	Base map[string]int `json:"base"`
	// Scrolls is the permanent bonus from scrolls (0–100), per primary stat.
	Scrolls map[string]int `json:"scrolls"`
}

// Normalize clamps a character to valid values.
func (c *Character) Normalize() {
	if c.Level < 1 || c.Level > MaxLevel {
		c.Level = MaxLevel
	}
	clamp := func(m map[string]int, max int) map[string]int {
		out := make(map[string]int, len(PrimaryStats))
		for _, key := range PrimaryStats {
			v := m[key]
			if v < 0 {
				v = 0
			}
			if v > max {
				v = max
			}
			out[key] = v
		}
		return out
	}
	c.Base = clamp(c.Base, 9999)
	c.Scrolls = clamp(c.Scrolls, 100)
}

// PointCost returns the characteristic points needed to raise a primary stat
// from zero to value.
func PointCost(key string, value int) int {
	switch key {
	case "vitality":
		return value
	case "wisdom":
		return value * 3
	}
	cost, tier := 0, 1
	for value > 0 {
		step := value
		if tier < 4 && step > 100 {
			step = 100
		}
		cost += step * tier
		value -= step
		if tier < 4 {
			tier++
		}
	}
	return cost
}

// PointsTotal is the number of characteristic points available at a level.
func PointsTotal(level int) int { return (level - 1) * 5 }

// Engine evaluates builds against one catalog.
type Engine struct {
	Catalog  *game.Catalog
	criteria map[int]*Criteria
	// gainEffect and lossEffect give, per stat, the effect whose template
	// words a bonus or a malus of that stat.
	gainEffect map[int]*game.EffectDef
	lossEffect map[int]*game.EffectDef

	n                              int
	hp, ap, mp, rng, summons       int32
	str, intel, chance, agi        int32
	vit, wis                       int32
	init, prospecting, pods        int32
	tackle, escape                 int32
	dodgeAP, dodgeMP, apRed, mpRed int32
}

func New(cat *game.Catalog) *Engine {
	e := &Engine{
		Catalog:  cat,
		criteria: map[int]*Criteria{},
		n:        len(cat.Stats),

		hp: cat.Idx(game.StatHP), ap: cat.Idx(game.StatAP), mp: cat.Idx(game.StatMP),
		summons: cat.Idx(game.StatSummons), rng: cat.Idx(game.StatRange),
		str: cat.Idx(game.StatStrength), intel: cat.Idx(game.StatIntel),
		chance: cat.Idx(game.StatChance), agi: cat.Idx(game.StatAgility),
		vit: cat.Idx(game.StatVitality), wis: cat.Idx(game.StatWisdom),
		init: cat.Idx(game.StatInitiative), prospecting: cat.Idx(game.StatProspecting),
		pods: cat.Idx(game.StatPods), tackle: cat.Idx(game.StatTackle), escape: cat.Idx(game.StatEscape),
		dodgeAP: cat.Idx(game.StatDodgeAP), dodgeMP: cat.Idx(game.StatDodgeMP),
		apRed: cat.Idx(game.StatAPReduction), mpRed: cat.Idx(game.StatMPReduction),
	}
	for i := range cat.Items {
		it := &cat.Items[i]
		if c := ParseCriteria(it.Criteria); c != nil {
			e.criteria[it.ID] = c
		}
	}
	e.gainEffect = map[int]*game.EffectDef{}
	e.lossEffect = map[int]*game.EffectDef{}
	for i := range cat.Effects {
		def := &cat.Effects[i]
		if def.Kind != game.EffectStat {
			continue
		}
		target := e.gainEffect
		if def.Sign < 0 {
			target = e.lossEffect
		}
		if _, taken := target[def.StatID]; !taken {
			target[def.StatID] = def
		}
	}
	return e
}

// StatText words a stat value the way the game does on an item line.
func (e *Engine) StatText(statID, value int, lang string) string {
	def := e.gainEffect[statID]
	if value < 0 && e.lossEffect[statID] != nil {
		def = e.lossEffect[statID]
	}
	if def != nil {
		return game.RenderEffect(def.Template.Get(lang), value, value)
	}
	label := ""
	if st := e.Catalog.StatByID[statID]; st != nil {
		label = st.Unit + " " + st.Label.Get(lang)
	}
	return strings.TrimSpace(strconv.Itoa(value) + label)
}

// StatCount is the length of dense stat vectors.
func (e *Engine) StatCount() int { return e.n }

// Criteria returns the parsed equip condition of an item, or nil.
func (e *Engine) Criteria(itemID int) *Criteria { return e.criteria[itemID] }

func add(v []int32, idx int32, delta int32) {
	if idx >= 0 {
		v[idx] += delta
	}
}

func get(v []int32, idx int32) int32 {
	if idx < 0 {
		return 0
	}
	return v[idx]
}

// BaseVector returns the stats of a naked character.
func (e *Engine) BaseVector(c Character) []int32 {
	v := make([]int32, e.n)
	ap := int32(6)
	if c.Level >= 100 {
		ap = 7
	}
	add(v, e.ap, ap)
	add(v, e.mp, 3)
	add(v, e.summons, 1)
	add(v, e.prospecting, 100)
	add(v, e.pods, 1000)
	for key, statID := range primaryStatIDs {
		add(v, e.Catalog.Idx(statID), int32(c.Base[key]+c.Scrolls[key]))
	}
	return v
}

// Derive turns summed raw stats into final stats, applying the bonuses that
// characteristics grant to each other. out must have the same length as raw.
func (e *Engine) Derive(level int, raw, out []int32) {
	copy(out, raw)
	positive := func(idx int32) int32 {
		if v := get(raw, idx); v > 0 {
			return v
		}
		return 0
	}
	add(out, e.hp, int32(50+5*level)+get(raw, e.vit))
	add(out, e.init, positive(e.str)+positive(e.intel)+positive(e.chance)+positive(e.agi))
	add(out, e.prospecting, positive(e.chance)/10)
	agility, wisdom := positive(e.agi)/10, positive(e.wis)/10
	add(out, e.tackle, agility)
	add(out, e.escape, agility)
	add(out, e.dodgeAP, wisdom)
	add(out, e.dodgeMP, wisdom)
	add(out, e.apRed, wisdom)
	add(out, e.mpRed, wisdom)
	add(out, e.pods, positive(e.str)*5)
	limit(out, e.ap, MaxAP)
	limit(out, e.mp, MaxMP)
	limit(out, e.rng, MaxRange)
}

func limit(v []int32, idx int32, ceiling int32) {
	if idx >= 0 && v[idx] > ceiling {
		v[idx] = ceiling
	}
}

// Build is a character and the item worn at each position (0 = empty).
type Build struct {
	Character
	Items map[string]int `json:"items"`
	// Mods is the forgemagic of the worn items, by position.
	Mods map[string]*ItemMods `json:"mods,omitempty"`
}

type EffectLine struct {
	StatID int    `json:"statId"`
	Value  int    `json:"value"`
	Text   string `json:"text"`
}

type SetStatus struct {
	ID      int          `json:"id"`
	Name    string       `json:"name"`
	Count   int          `json:"count"`
	Size    int          `json:"size"`
	Bonuses []EffectLine `json:"bonuses"`
}

type ConditionStatus struct {
	Position string `json:"position"`
	ItemID   int    `json:"itemId"`
	Text     string `json:"text"`
	OK       bool   `json:"ok"`
}

// Issue codes.
const (
	IssueUnknownItem = "unknown_item"
	IssueWrongSlot   = "wrong_slot"
	IssueLevel       = "level"
	IssueDuplicate   = "duplicate"
	// IssuePrysmaradite: a prysmaradite outside the first Dofus position.
	IssuePrysmaradite = "prysmaradite_slot"
	IssuePoints       = "points"
)

type Issue struct {
	Code     string `json:"code"`
	Position string `json:"position,omitempty"`
	ItemID   int    `json:"itemId,omitempty"`
}

// IgnoredExo is an AP, MP or Range exo that brings nothing: a character counts
// one exo of each, and another item of the stuff already carries it. Taking a
// malus off one of those lines counts as an exo too.
type IgnoredExo struct {
	Position string `json:"position"`
	ItemID   int    `json:"itemId"`
	StatID   int    `json:"statId"`
	// Value is what the exo would have added.
	Value int `json:"value"`
}

// CappedStat reports a stat whose sum went over the game's cap.
type CappedStat struct {
	StatID int `json:"statId"`
	Max    int `json:"max"`
	// Raw is the sum before the cap: Raw - Max points are lost.
	Raw int `json:"raw"`
}

type Points struct {
	Spent int `json:"spent"`
	Total int `json:"total"`
}

type Result struct {
	Level int         `json:"level"`
	Stats map[int]int `json:"stats"`
	// Forge holds, per position, the lines of each forgeable item after its
	// forgemagic.
	Forge map[string][]ForgedLine `json:"forge"`
	// Runes holds, per position, the transcendence runes the item can take
	// with its other forgemagic.
	Runes map[string][]int `json:"runes"`
	// ForgeWeight holds, per position, the weight the item's overs and exos
	// use out of the 101 they share.
	ForgeWeight map[string]float64 `json:"forgeWeight"`
	// IgnoredExos lists the AP, MP and Range exos that do not count; the stuff
	// stays valid, as the game lets both items be worn.
	IgnoredExos []IgnoredExo `json:"ignoredExos"`
	// Capped lists the stats that went over the game's cap.
	Capped     []CappedStat      `json:"capped"`
	Sets       []SetStatus       `json:"sets"`
	SetBonuses int               `json:"setBonuses"`
	Conditions []ConditionStatus `json:"conditions"`
	Issues     []Issue           `json:"issues"`
	Points     Points            `json:"points"`
	Valid      bool              `json:"valid"`
}

// Evaluate computes the final stats of a build and checks its validity.
func (e *Engine) Evaluate(b Build, lang string) Result {
	b.Normalize()
	cat := e.Catalog
	res := Result{
		Level:       b.Level,
		Stats:       make(map[int]int, e.n),
		Forge:       map[string][]ForgedLine{},
		Runes:       map[string][]int{},
		ForgeWeight: map[string]float64{},
		IgnoredExos: []IgnoredExo{},
		Capped:      []CappedStat{},
		Sets:        []SetStatus{},
		Conditions:  []ConditionStatus{},
		Issues:      []Issue{},
	}

	base := e.BaseVector(b.Character)
	raw := make([]int32, e.n)
	copy(raw, base)

	type worn struct {
		position string
		item     *game.Item
	}
	var items []worn
	setCounts := map[int]int{}
	seen := map[int]string{}
	limitedExos := map[int][]IgnoredExo{}

	for _, pos := range game.Positions {
		id := b.Items[pos.Key]
		if id == 0 {
			continue
		}
		it := cat.ItemByID[id]
		if it == nil {
			res.Issues = append(res.Issues, Issue{Code: IssueUnknownItem, Position: pos.Key, ItemID: id})
			continue
		}
		if it.Slot != pos.Slot {
			res.Issues = append(res.Issues, Issue{Code: IssueWrongSlot, Position: pos.Key, ItemID: id})
			continue
		}
		if !game.Fits(it, pos) {
			res.Issues = append(res.Issues, Issue{Code: IssuePrysmaradite, Position: pos.Key, ItemID: id})
		}
		if it.Level > b.Level {
			res.Issues = append(res.Issues, Issue{Code: IssueLevel, Position: pos.Key, ItemID: id})
		}
		// Dofus and trophies are unique; so are rings that belong to a set.
		if _, dup := seen[id]; dup && (it.Slot == game.SlotDofus || it.SetID > 0) {
			res.Issues = append(res.Issues, Issue{Code: IssueDuplicate, Position: pos.Key, ItemID: id})
		}
		seen[id] = pos.Key
		items = append(items, worn{pos.Key, it})
		vector := cat.ItemVector[id]
		mods := b.Mods[pos.Key]
		if ForgeableSlots[it.Slot] || !mods.Empty() {
			forged := e.Forge(it, mods)
			vector = forged.Vector
			for _, code := range forged.Issues {
				res.Issues = append(res.Issues, Issue{Code: code, Position: pos.Key, ItemID: id})
			}
			for i := range forged.Lines {
				forged.Lines[i].Text = e.StatText(forged.Lines[i].StatID, forged.Lines[i].Value, lang)
			}
			if ForgeableSlots[it.Slot] {
				res.Forge[pos.Key] = forged.Lines
				res.Runes[pos.Key] = append([]int{}, forged.Runes...)
				res.ForgeWeight[pos.Key] = math.Round(forged.Weight*10) / 10
			}
			for _, line := range forged.Lines {
				if !isLimitedExo(line.StatID) {
					continue
				}
				gain := 0
				switch {
				case line.Origin == OriginExo:
					gain = line.Value
				case line.Natural && line.Max < 0:
					gain = line.Value - line.Max
				}
				if gain > 0 {
					limitedExos[line.StatID] = append(limitedExos[line.StatID], IgnoredExo{Position: pos.Key, ItemID: id, StatID: line.StatID, Value: gain})
				}
			}
		}
		for _, sv := range vector {
			raw[sv.Index] += sv.Value
		}
		if it.SetID > 0 {
			setCounts[it.SetID]++
		}
	}

	// A character counts one AP exo, one MP exo and one Range exo: the largest
	// stays, and what the others add is taken back.
	for _, statID := range LimitedExos {
		exos := limitedExos[statID]
		kept := 0
		for i, exo := range exos {
			if exo.Value > exos[kept].Value {
				kept = i
			}
		}
		for i, exo := range exos {
			if i == kept {
				continue
			}
			if idx := cat.Idx(statID); idx >= 0 {
				raw[idx] -= int32(exo.Value)
			}
			res.IgnoredExos = append(res.IgnoredExos, exo)
		}
	}

	setIDs := make([]int, 0, len(setCounts))
	for id := range setCounts {
		setIDs = append(setIDs, id)
	}
	sort.Ints(setIDs)
	for _, setID := range setIDs {
		count := setCounts[setID]
		set := cat.SetByID[setID]
		if set == nil {
			continue
		}
		if count > 1 {
			res.SetBonuses += count - 1
		}
		for _, sv := range cat.SetBonusVector(setID, count) {
			raw[sv.Index] += sv.Value
		}
		status := SetStatus{
			ID: setID, Name: set.Name.Get(lang), Count: count, Size: len(set.ItemIDs),
			Bonuses: e.SetBonusLines(set, count, lang),
		}
		res.Sets = append(res.Sets, status)
	}

	totals := make([]int32, e.n)
	e.Derive(b.Level, raw, totals)
	baseTotals := make([]int32, e.n)
	e.Derive(b.Level, base, baseTotals)
	for i, statID := range cat.StatIDs {
		res.Stats[statID] = int(totals[i])
	}
	for _, c := range []struct {
		idx     int32
		statID  int
		ceiling int
	}{{e.ap, game.StatAP, MaxAP}, {e.mp, game.StatMP, MaxMP}, {e.rng, game.StatRange, MaxRange}} {
		if c.idx >= 0 && int(raw[c.idx]) > c.ceiling {
			res.Capped = append(res.Capped, CappedStat{StatID: c.statID, Max: c.ceiling, Raw: int(raw[c.idx])})
		}
	}

	ctx := &CriteriaContext{Catalog: cat, Totals: totals, Base: baseTotals, SetBonuses: res.SetBonuses}
	conditionsOK := true
	for _, w := range items {
		crit := e.criteria[w.item.ID]
		if crit == nil {
			continue
		}
		ok := crit.Check(ctx)
		if !ok {
			conditionsOK = false
		}
		res.Conditions = append(res.Conditions, ConditionStatus{
			Position: w.position, ItemID: w.item.ID, Text: crit.Text(cat, lang), OK: ok,
		})
	}

	for _, key := range PrimaryStats {
		res.Points.Spent += PointCost(key, b.Base[key])
	}
	res.Points.Total = PointsTotal(b.Level)
	if res.Points.Spent > res.Points.Total {
		res.Issues = append(res.Issues, Issue{Code: IssuePoints})
	}
	res.Valid = conditionsOK && len(res.Issues) == 0
	return res
}

// SetBonusLines renders the bonus a set grants for a number of worn items.
func (e *Engine) SetBonusLines(set *game.Set, count int, lang string) []EffectLine {
	lines := []EffectLine{}
	if count < 0 || len(set.Bonuses) == 0 {
		return lines
	}
	if count >= len(set.Bonuses) {
		count = len(set.Bonuses) - 1
	}
	for _, b := range set.Bonuses[count] {
		text := ""
		if def := e.Catalog.EffectByID[b.EffectID]; def != nil {
			text = game.RenderEffect(def.Template.Get(lang), b.Value, b.Value)
		}
		lines = append(lines, EffectLine{StatID: b.StatID, Value: b.Value, Text: text})
	}
	return lines
}
