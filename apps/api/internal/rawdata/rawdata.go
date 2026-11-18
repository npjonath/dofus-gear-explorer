// Package rawdata reads the JSON files published in dofusdude/dofus3-main
// releases (the output of doduda) and turns them into a game.Dataset.
package rawdata

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"dofusdb/api/internal/game"
)

// DataFiles are the release assets the importer needs, besides one
// translation file per language (see LangFile).
var DataFiles = []string{
	"items.json",
	"item_sets.json",
	"item_types.json",
	"item_super_types.json",
	"effects.json",
	"characteristics.json",
	"chars_categories.json",
	"spells.json",
	"spell_levels.json",
	"spell_variants.json",
	"breeds.json",
}

// LangFile returns the translation asset name for a language code.
func LangFile(lang string) string { return lang + ".json" }

// flexInt decodes ids that the game data writes as either a number or a string.
type flexInt int

func (f *flexInt) UnmarshalJSON(b []byte) error {
	s := string(b)
	if len(s) >= 2 && s[0] == '"' {
		s = s[1 : len(s)-1]
	}
	if s == "" || s == "null" {
		*f = 0
		return nil
	}
	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		*f = 0
		return nil
	}
	*f = flexInt(n)
	return nil
}

type array[T any] struct {
	Array []T `json:"Array"`
}

type ridRef struct {
	Rid int64 `json:"rid"`
}

type refEntry struct {
	Rid  int64 `json:"rid"`
	Type struct {
		Class string `json:"class"`
	} `json:"type"`
	Data json.RawMessage `json:"data"`
}

type rootFile struct {
	References struct {
		RefIds []refEntry `json:"RefIds"`
	} `json:"references"`
}

type rawItem struct {
	ID              int           `json:"id"`
	TypeID          int           `json:"typeId"`
	Level           int           `json:"level"`
	IconID          int           `json:"iconId"`
	ItemSetID       int           `json:"itemSetId"`
	NameID          flexInt       `json:"nameId"`
	DescriptionID   flexInt       `json:"descriptionId"`
	Criterions      string        `json:"criterions"`
	RealWeight      float64       `json:"realWeight"`
	PossibleEffects array[ridRef] `json:"possibleEffects"`
	APCost          int           `json:"apCost"`
	Range           int           `json:"range"`
	MinRange        int           `json:"minRange"`
	CritProbability int           `json:"criticalHitProbability"`
	CritBonus       int           `json:"criticalHitBonus"`
	MaxCastPerTurn  int           `json:"maxCastPerTurn"`
}

type rawEffectInstance struct {
	ActionID int `json:"actionId"`
	DiceNum  int `json:"diceNum"`
	DiceSide int `json:"diceSide"`
	Value    int `json:"value"`
}

type rawEffect struct {
	ID             int     `json:"id"`
	Characteristic int     `json:"characteristic"`
	Operator       string  `json:"characteristicOperator"`
	BonusType      int     `json:"bonusType"`
	Category       int     `json:"category"`
	ElementID      int     `json:"elementId"`
	DescriptionID  flexInt `json:"descriptionId"`
	UseDice        int     `json:"useDice"`
	PowerRate      float64 `json:"effectPowerRate"`
}

type rawSpell struct {
	ID            int     `json:"id"`
	NameID        flexInt `json:"nameId"`
	DescriptionID flexInt `json:"descriptionId"`
	IconID        int     `json:"iconId"`
	TypeID        int     `json:"typeId"`
}

// rawSpellHead is what the spell importer needs of a spell.
type rawSpellHead = rawSpell

// Game ids the importer relies on.
const (
	// effectPassive attaches a spell to an item: a Dofus or legendary passive.
	effectPassive = 1175
	// typeTranscendenceRune is the item type of transcendence runes.
	typeTranscendenceRune = 211
	// effectRuneDensity carries, on a transcendence rune, the weight the rune
	// counts for on the line it lands on.
	effectRuneDensity = 2826
)

type rawCharacteristic struct {
	ID         int     `json:"id"`
	Keyword    string  `json:"keyword"`
	NameID     flexInt `json:"nameId"`
	CategoryID int     `json:"categoryId"`
	Order      int     `json:"order"`
}

type rawCategory struct {
	ID                int        `json:"id"`
	NameID            flexInt    `json:"nameId"`
	Order             int        `json:"order"`
	CharacteristicIDs array[int] `json:"characteristicIds"`
}

type rawType struct {
	ID          int     `json:"id"`
	NameID      flexInt `json:"nameId"`
	SuperTypeID int     `json:"superTypeId"`
}

type rawSet struct {
	ID         int        `json:"id"`
	NameID     flexInt    `json:"nameId"`
	IsCosmetic int        `json:"isCosmetic"`
	Items      array[int] `json:"items"`
	Effects    array[struct {
		Values array[ridRef] `json:"values"`
	}] `json:"effects"`
}

func readRefs(dir, name string) ([]refEntry, error) {
	f, err := os.Open(filepath.Join(dir, name))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var root rootFile
	if err := json.NewDecoder(f).Decode(&root); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	if len(root.References.RefIds) == 0 {
		return nil, fmt.Errorf("%s: no entries found, the release format may have changed", name)
	}
	return root.References.RefIds, nil
}

func decodeClass[T any](refs []refEntry, classes ...string) ([]T, error) {
	want := map[string]bool{}
	for _, c := range classes {
		want[c] = true
	}
	var out []T
	for _, r := range refs {
		if !want[r.Type.Class] {
			continue
		}
		var v T
		if err := json.Unmarshal(r.Data, &v); err != nil {
			return nil, fmt.Errorf("decode %s: %w", r.Type.Class, err)
		}
		out = append(out, v)
	}
	return out, nil
}

func decodeOne[T any](ref refEntry) (T, error) {
	var v T
	if err := json.Unmarshal(ref.Data, &v); err != nil {
		return v, fmt.Errorf("decode %s: %w", ref.Type.Class, err)
	}
	return v, nil
}

func instancesByRid(refs []refEntry) (map[int64]rawEffectInstance, error) {
	out := map[int64]rawEffectInstance{}
	for _, r := range refs {
		if r.Type.Class != "EffectInstanceDice" {
			continue
		}
		var v rawEffectInstance
		if err := json.Unmarshal(r.Data, &v); err != nil {
			return nil, fmt.Errorf("decode effect instance: %w", err)
		}
		out[r.Rid] = v
	}
	return out, nil
}

// readTexts streams a translation file and keeps only the wanted ids.
func readTexts(dir, lang string, wanted map[int]bool) (map[int]string, error) {
	f, err := os.Open(filepath.Join(dir, LangFile(lang)))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	out := make(map[int]string, len(wanted))

	expectDelim := func(d json.Delim) error {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		if got, ok := tok.(json.Delim); !ok || got != d {
			return fmt.Errorf("%s: unexpected token %v", LangFile(lang), tok)
		}
		return nil
	}
	if err := expectDelim('{'); err != nil {
		return nil, err
	}
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		if keyTok != "entries" {
			var skip json.RawMessage
			if err := dec.Decode(&skip); err != nil {
				return nil, err
			}
			continue
		}
		if err := expectDelim('{'); err != nil {
			return nil, err
		}
		for dec.More() {
			idTok, err := dec.Token()
			if err != nil {
				return nil, err
			}
			var text string
			if err := dec.Decode(&text); err != nil {
				return nil, err
			}
			id, err := strconv.Atoi(idTok.(string))
			if err == nil && wanted[id] {
				out[id] = text
			}
		}
		if err := expectDelim('}'); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// unobtainable reports criteria that no player can meet: game master
// rights (PX), items bound to one character name (PN) and disabled items (BI=1).
func unobtainable(criteria string) bool {
	return strings.Contains(criteria, "PX=") || strings.Contains(criteria, "PN~") || strings.Contains(criteria, "BI=1")
}

var (
	linkMarkupRe = regexp.MustCompile(`\{\{[^{}]*?::([^{}]*)\}\}`)
	tagMarkupRe  = regexp.MustCompile(`<[^<>]*>`)
	blanksRe     = regexp.MustCompile(`[ \t\r\n]+`)
)

// cleanPassive turns a spell description into plain text: links such as
// {{spell,8395,1::Pourpre Profond}} keep their label, sprite tags go, and
// the leading repeat of the passive's own name is dropped.
func cleanPassive(text, name string) string {
	text = linkMarkupRe.ReplaceAllString(text, "$1")
	text = tagMarkupRe.ReplaceAllString(text, "")
	text = strings.TrimSpace(blanksRe.ReplaceAllString(text, " "))
	if name != "" {
		text = strings.TrimSpace(strings.TrimPrefix(text, name))
		text = strings.TrimSpace(strings.TrimPrefix(text, ":"))
	}
	return strings.TrimSpace(strings.TrimPrefix(text, "•"))
}

func capitalize(s string) string {
	r, size := utf8.DecodeRuneInString(s)
	if size == 0 {
		return s
	}
	return string(unicode.ToUpper(r)) + s[size:]
}

// Parse reads a directory of release files into a Dataset.
func Parse(dir, version string, langs []string) (*game.Dataset, error) {
	itemRefs, err := readRefs(dir, "items.json")
	if err != nil {
		return nil, err
	}
	setRefs, err := readRefs(dir, "item_sets.json")
	if err != nil {
		return nil, err
	}
	typeRefs, err := readRefs(dir, "item_types.json")
	if err != nil {
		return nil, err
	}
	effectRefs, err := readRefs(dir, "effects.json")
	if err != nil {
		return nil, err
	}
	charRefs, err := readRefs(dir, "characteristics.json")
	if err != nil {
		return nil, err
	}
	catRefs, err := readRefs(dir, "chars_categories.json")
	if err != nil {
		return nil, err
	}

	spellRefs, err := readRefs(dir, "spells.json")
	if err != nil {
		return nil, err
	}
	rawSpells, err := decodeClass[rawSpell](spellRefs, "SpellData")
	if err != nil {
		return nil, err
	}
	spellByID := make(map[int]rawSpell, len(rawSpells))
	for _, sp := range rawSpells {
		spellByID[sp.ID] = sp
	}

	rawItems, err := decodeClass[rawItem](itemRefs, "ItemData", "WeaponData")
	if err != nil {
		return nil, err
	}
	weaponIDs := map[int]bool{}
	for _, r := range itemRefs {
		if r.Type.Class == "WeaponData" {
			var head struct {
				ID int `json:"id"`
			}
			_ = json.Unmarshal(r.Data, &head)
			weaponIDs[head.ID] = true
		}
	}
	itemInstances, err := instancesByRid(itemRefs)
	if err != nil {
		return nil, err
	}
	rawSets, err := decodeClass[rawSet](setRefs, "ItemSetData")
	if err != nil {
		return nil, err
	}
	setInstances, err := instancesByRid(setRefs)
	if err != nil {
		return nil, err
	}
	rawTypes, err := decodeClass[rawType](typeRefs, "ItemTypeData")
	if err != nil {
		return nil, err
	}
	rawEffects, err := decodeClass[rawEffect](effectRefs, "EffectData")
	if err != nil {
		return nil, err
	}
	rawChars, err := decodeClass[rawCharacteristic](charRefs, "CharacteristicData")
	if err != nil {
		return nil, err
	}
	rawCats, err := decodeClass[rawCategory](catRefs, "CharacteristicCategoryData")
	if err != nil {
		return nil, err
	}

	effectByID := map[int]rawEffect{}
	for _, e := range rawEffects {
		effectByID[e.ID] = e
	}
	charByID := map[int]rawCharacteristic{}
	for _, c := range rawChars {
		charByID[c.ID] = c
	}

	classify := func(e rawEffect) (game.EffectKind, bool) {
		_, knownChar := charByID[e.Characteristic]
		isBonus := e.BonusType == 1 || e.BonusType == -1
		if e.Characteristic > 0 && knownChar && isBonus && e.UseDice == 1 && (e.Category == 0 || e.Category == 1) {
			return game.EffectStat, true
		}
		if e.Category == 2 && e.UseDice == 1 {
			return game.EffectWeapon, true
		}
		return "", false
	}

	wantedText := map[int]bool{}
	usedEffects := map[int]int{}
	usedStats := map[int]bool{game.StatHP: true}

	ds := &game.Dataset{Version: version, Languages: langs}

	typeByID := map[int]rawType{}
	for _, t := range rawTypes {
		typeByID[t.ID] = t
	}
	usedTypes := map[int]bool{}

	type pendingText struct {
		target game.Texts
		id     int
	}
	var pending []pendingText
	want := func(target game.Texts, id flexInt) {
		if id > 0 {
			wantedText[int(id)] = true
			pending = append(pending, pendingText{target, int(id)})
		}
	}

	passiveSpells := map[*game.Passive]rawSpell{}
	equipmentIDs := map[int]bool{}
	for _, ri := range rawItems {
		if ri.TypeID == typeTranscendenceRune {
			var transcendence *game.Rune
			density := 0
			for _, ref := range ri.PossibleEffects.Array {
				inst, ok := itemInstances[ref.Rid]
				if !ok {
					continue
				}
				if inst.ActionID == effectRuneDensity {
					density = inst.Value
					continue
				}
				def, ok := effectByID[inst.ActionID]
				if !ok || transcendence != nil {
					continue
				}
				if kind, ok := classify(def); !ok || kind != game.EffectStat || def.BonusType != 1 {
					continue
				}
				transcendence = &game.Rune{
					ID: ri.ID, Level: ri.Level, EffectID: def.ID,
					StatID: def.Characteristic, Value: inst.DiceNum, Name: game.Texts{},
				}
			}
			if transcendence != nil {
				transcendence.Density = density
				want(transcendence.Name, ri.NameID)
				usedEffects[transcendence.EffectID]++
				usedStats[transcendence.StatID] = true
				ds.Runes = append(ds.Runes, *transcendence)
			}
			continue
		}
		t, ok := typeByID[ri.TypeID]
		if !ok {
			continue
		}
		slot, ok := game.SlotBySuperType[t.SuperTypeID]
		if !ok {
			continue
		}
		item := game.Item{
			ID:          ri.ID,
			TypeID:      ri.TypeID,
			Slot:        slot,
			Level:       ri.Level,
			IconID:      ri.IconID,
			SetID:       ri.ItemSetID,
			Criteria:    ri.Criterions,
			Pods:        int(ri.RealWeight),
			Name:        game.Texts{},
			Description: game.Texts{},
		}
		if item.SetID < 0 {
			item.SetID = 0
		}
		if weaponIDs[ri.ID] {
			item.Weapon = &game.WeaponInfo{
				APCost:         ri.APCost,
				RangeMin:       ri.MinRange,
				RangeMax:       ri.Range,
				CritChance:     ri.CritProbability,
				CritBonus:      ri.CritBonus,
				MaxCastPerTurn: ri.MaxCastPerTurn,
			}
		}
		for _, ref := range ri.PossibleEffects.Array {
			inst, ok := itemInstances[ref.Rid]
			if !ok {
				continue
			}
			if inst.ActionID == effectPassive {
				if sp, ok := spellByID[inst.DiceNum]; ok && item.Passive == nil {
					item.Passive = &game.Passive{Name: game.Texts{}, Description: game.Texts{}}
					passiveSpells[item.Passive] = sp
				}
				continue
			}
			def, ok := effectByID[inst.ActionID]
			if !ok {
				continue
			}
			kind, ok := classify(def)
			if !ok {
				continue
			}
			lo, hi := inst.DiceNum, inst.DiceSide
			if hi == 0 {
				hi = lo
			}
			if lo > hi {
				lo, hi = hi, lo
			}
			effect := game.ItemEffect{EffectID: def.ID, Kind: kind, StatID: -1, Min: lo, Max: hi}
			if kind == game.EffectStat {
				effect.StatID = def.Characteristic
				if def.BonusType < 0 {
					effect.Min, effect.Max = -hi, -lo
				}
			}
			item.Effects = append(item.Effects, effect)
		}
		if (len(item.Effects) == 0 && item.Passive == nil) || unobtainable(ri.Criterions) {
			continue
		}
		if item.Passive != nil {
			sp := passiveSpells[item.Passive]
			want(item.Passive.Name, sp.NameID)
			want(item.Passive.Description, sp.DescriptionID)
		}
		for _, e := range item.Effects {
			usedEffects[e.EffectID]++
			if e.Kind == game.EffectStat {
				usedStats[e.StatID] = true
			}
		}
		want(item.Name, ri.NameID)
		want(item.Description, ri.DescriptionID)
		usedTypes[ri.TypeID] = true
		equipmentIDs[ri.ID] = true
		ds.Items = append(ds.Items, item)
	}
	if len(ds.Items) == 0 {
		return nil, fmt.Errorf("no equipment found in items.json")
	}
	sort.Slice(ds.Items, func(a, b int) bool { return ds.Items[a].ID < ds.Items[b].ID })
	sort.Slice(ds.Runes, func(a, b int) bool { return ds.Runes[a].ID < ds.Runes[b].ID })

	ds.Breeds, ds.Spells, err = parseSpells(dir, rawSpells, effectByID, want)
	if err != nil {
		return nil, err
	}
	for _, id := range game.AttackStats {
		if _, known := charByID[id]; known {
			usedStats[id] = true
		}
	}

	levelByItem := map[int]int{}
	for _, it := range ds.Items {
		levelByItem[it.ID] = it.Level
	}

	for _, rs := range rawSets {
		if rs.IsCosmetic != 0 {
			continue
		}
		set := game.Set{ID: rs.ID, Name: game.Texts{}}
		for _, id := range rs.Items.Array {
			if equipmentIDs[id] {
				set.ItemIDs = append(set.ItemIDs, id)
				if levelByItem[id] > set.Level {
					set.Level = levelByItem[id]
				}
			}
		}
		if len(set.ItemIDs) < 2 {
			continue
		}
		// The game lists bonuses by number of items worn, starting at one item.
		set.Bonuses = make([][]game.SetBonus, len(rs.Effects.Array)+1)
		for i, level := range rs.Effects.Array {
			for _, ref := range level.Values.Array {
				inst, ok := setInstances[ref.Rid]
				if !ok {
					continue
				}
				def, ok := effectByID[inst.ActionID]
				if !ok {
					continue
				}
				kind, ok := classify(def)
				if !ok || kind != game.EffectStat {
					continue
				}
				value := inst.DiceNum
				if inst.DiceSide > value {
					value = inst.DiceSide
				}
				if def.BonusType < 0 {
					value = -value
				}
				usedEffects[def.ID]++
				usedStats[def.Characteristic] = true
				set.Bonuses[i+1] = append(set.Bonuses[i+1], game.SetBonus{
					EffectID: def.ID, StatID: def.Characteristic, Value: value,
				})
			}
		}
		want(set.Name, rs.NameID)
		ds.Sets = append(ds.Sets, set)
	}
	sort.Slice(ds.Sets, func(a, b int) bool { return ds.Sets[a].ID < ds.Sets[b].ID })

	// Items may point at sets that were dropped (cosmetic or single-item).
	keptSets := map[int]bool{}
	for _, s := range ds.Sets {
		keptSets[s.ID] = true
	}
	for i := range ds.Items {
		if !keptSets[ds.Items[i].SetID] {
			ds.Items[i].SetID = 0
		}
	}

	for _, t := range rawTypes {
		if !usedTypes[t.ID] {
			continue
		}
		it := game.ItemType{
			ID: t.ID, SuperTypeID: t.SuperTypeID,
			Slot: game.SlotBySuperType[t.SuperTypeID], Name: game.Texts{},
		}
		want(it.Name, t.NameID)
		ds.Types = append(ds.Types, it)
	}
	sort.Slice(ds.Types, func(a, b int) bool { return ds.Types[a].ID < ds.Types[b].ID })

	effectTemplateIDs := map[int]int{}
	for id := range usedEffects {
		def := effectByID[id]
		kind, _ := classify(def)
		e := game.EffectDef{
			ID: id, Kind: kind, StatID: -1, Sign: def.BonusType,
			Element: def.ElementID, Template: game.Texts{},
		}
		if kind == game.EffectStat {
			e.StatID = def.Characteristic
		}
		want(e.Template, def.DescriptionID)
		effectTemplateIDs[id] = int(def.DescriptionID)
		ds.Effects = append(ds.Effects, e)
	}
	sort.Slice(ds.Effects, func(a, b int) bool { return ds.Effects[a].ID < ds.Effects[b].ID })

	categoryOf := map[int]int{}
	for _, c := range rawCats {
		cat := game.StatCategory{ID: c.ID, Order: c.Order, Name: game.Texts{}}
		want(cat.Name, c.NameID)
		ds.Categories = append(ds.Categories, cat)
		for _, id := range c.CharacteristicIDs.Array {
			categoryOf[id] = c.ID
		}
	}
	sort.Slice(ds.Categories, func(a, b int) bool { return ds.Categories[a].Order < ds.Categories[b].Order })

	// The label of a stat comes from the most used effect that grants it.
	bestEffect := map[int]int{}
	for id, count := range usedEffects {
		def := effectByID[id]
		if kind, _ := classify(def); kind != game.EffectStat || def.BonusType != 1 {
			continue
		}
		if cur, ok := bestEffect[def.Characteristic]; !ok || count > usedEffects[cur] {
			bestEffect[def.Characteristic] = id
		}
	}
	statNameIDs := map[int]int{}
	for id := range usedStats {
		c, ok := charByID[id]
		if !ok {
			continue
		}
		category, ok := categoryOf[id]
		if !ok {
			category = c.CategoryID
		}
		weight := 0.0
		if effectID, ok := bestEffect[id]; ok {
			weight = effectByID[effectID].PowerRate
		}
		ds.Stats = append(ds.Stats, game.Stat{
			ID: id, Key: c.Keyword, CategoryID: category, Order: c.Order, Weight: weight, Label: game.Texts{},
		})
		statNameIDs[id] = int(c.NameID)
		wantedText[int(c.NameID)] = true
	}
	sort.Slice(ds.Stats, func(a, b int) bool { return ds.Stats[a].ID < ds.Stats[b].ID })

	for _, lang := range langs {
		texts, err := readTexts(dir, lang, wantedText)
		if err != nil {
			return nil, fmt.Errorf("read %s translations: %w", lang, err)
		}
		for _, p := range pending {
			if s, ok := texts[p.id]; ok {
				p.target[lang] = s
			}
		}
		for i := range ds.Items {
			if p := ds.Items[i].Passive; p != nil {
				p.Description[lang] = cleanPassive(p.Description[lang], p.Name[lang])
			}
		}
		for i := range ds.Stats {
			st := &ds.Stats[i]
			label, unit := "", ""
			if effectID, ok := bestEffect[st.ID]; ok {
				label, unit = game.StatLabel(texts[effectTemplateIDs[effectID]])
			}
			if label == "" {
				label = texts[statNameIDs[st.ID]]
			}
			st.Label[lang] = capitalize(label)
			if unit != "" {
				st.Unit = unit
			}
		}
	}
	return ds, nil
}
