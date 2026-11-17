package engine

import (
	"reflect"
	"testing"

	"dofusdb/api/internal/game"
	"dofusdb/api/internal/gametest"
)

const (
	statAgility     = game.StatAgility
	statEarthDamage = 88
	statAirDamage   = 91
	statEarthRes    = 33
	statFireRes     = 34
	statWaterRes    = 35

	cloakID     = 60
	runeVitID   = 900
	runeTaVitID = 901
	runeAirID   = 902
	runeTaAirID = 903
)

// forgeEngine extends the shared dataset with what forgemagic needs: a cloak
// with elemental lines, the stats dracorunes move, and one transcendence rune.
func forgeEngine() *Engine {
	ds := gametest.Dataset()
	stat := func(id int, key, label string, weight float64) game.Stat {
		return game.Stat{ID: id, Key: key, CategoryID: 2, Order: id, Weight: weight, Label: game.Texts{"fr": label}}
	}
	effect := func(id, statID int, template string) game.EffectDef {
		return game.EffectDef{ID: id, Kind: game.EffectStat, StatID: statID, Sign: 1, Element: -1, Template: game.Texts{"fr": template}}
	}
	line := func(effectID, statID, min, max int) game.ItemEffect {
		return game.ItemEffect{EffectID: effectID, Kind: game.EffectStat, StatID: statID, Min: min, Max: max}
	}
	ds.Stats = append(ds.Stats,
		stat(statAgility, "agility", "Agilité", 1),
		stat(game.StatIntel, "intelligence", "Intelligence", 1),
		stat(game.StatPower, "damagePercent", "Puissance", 2),
		stat(statAllDamage, "allDamageBonus", "Dommages", 20),
		stat(statEarthDamage, "earthDamageBonus", "Dommages Terre", 5),
		stat(statAirDamage, "airDamageBonus", "Dommages Air", 5),
		stat(statEarthRes, "earthElementResistPercent", "Résistance Terre", 6),
		stat(statFireRes, "fireElementResistPercent", "Résistance Feu", 6),
		stat(statWaterRes, "waterElementResistPercent", "Résistance Eau", 6),
		stat(game.StatChance, "chance", "Chance", 1),
		stat(90, "waterDamageBonus", "Dommages Eau", 5),
		stat(36, "airElementResistPercent", "Résistance Air", 6),
		stat(37, "neutralElementResistPercent", "Résistance Neutre", 6),
	)
	ds.Effects = append(ds.Effects,
		effect(119, statAgility, "#1{{~1~2 à }}#2 Agilité"),
		effect(126, game.StatIntel, "#1{{~1~2 à }}#2 Intelligence"),
		effect(138, game.StatPower, "#1{{~1~2 à }}#2 Puissance"),
		effect(112, statAllDamage, "#1{{~1~2 à }}#2 Dommages"),
		effect(422, statEarthDamage, "#1{{~1~2 à }}#2 Dommages Terre"),
		effect(428, statAirDamage, "#1{{~1~2 à }}#2 Dommages Air"),
		effect(210, statEarthRes, "#1{{~1~2 à }}#2% Résistance Terre"),
		effect(213, statFireRes, "#1{{~1~2 à }}#2% Résistance Feu"),
		effect(211, statWaterRes, "#1{{~1~2 à }}#2% Résistance Eau"),
	)
	ds.Types = append(ds.Types, game.ItemType{ID: 17, SuperTypeID: 11, Slot: game.SlotCloak, Name: game.Texts{"fr": "Cape"}})
	ds.Items = append(ds.Items, game.Item{
		ID: cloakID, TypeID: 17, Slot: game.SlotCloak, Level: 200, IconID: cloakID,
		Name: game.Texts{"fr": "Cape d'Essai"}, Description: game.Texts{},
		Effects: []game.ItemEffect{
			line(gametest.EffectVitality, game.StatVitality, 251, 300),
			line(gametest.EffectStrength, game.StatStrength, 51, 70),
			line(126, game.StatIntel, 31, 50),
			line(422, statEarthDamage, 6, 10),
			line(210, statEarthRes, 6, 10),
			line(213, statFireRes, 6, 10),
		},
	})
	rune := func(id, effectID, statID, value, density int, name string) game.Rune {
		return game.Rune{ID: id, Level: 168, EffectID: effectID, StatID: statID, Value: value, Density: density, Name: game.Texts{"fr": name}}
	}
	ds.Runes = append(ds.Runes,
		rune(runeVitID, gametest.EffectVitality, game.StatVitality, 100, 80, "Rune Rata Vi"),
		rune(runeTaVitID, gametest.EffectVitality, game.StatVitality, 50, 40, "Rune Ta Vi"),
		rune(runeAirID, 428, statAirDamage, 6, 80, "Rune Rata Do Air"),
		rune(runeTaAirID, 428, statAirDamage, 2, 40, "Rune Ta Do Air"),
	)
	return New(game.NewCatalog(ds))
}

func forgedValues(f Forged) map[int]int {
	out := map[int]int{}
	for _, line := range f.Lines {
		out[line.StatID] = line.Value
	}
	return out
}

func TestForgeMaxFollowsTheWeightRule(t *testing.T) {
	cat := forgeEngine().Catalog
	cases := map[int]int{
		game.StatAP: 1, game.StatMP: 1, game.StatStrength: 101, game.StatVitality: 505,
		game.StatWisdom: 33, game.StatPower: 50, statAllDamage: 5, statEarthRes: 16,
		game.StatHP: 0,
	}
	for statID, want := range cases {
		if got := cat.StatByID[statID].ForgeMax(); got != want {
			t.Errorf("ForgeMax(%s) = %d, want %d", cat.StatByID[statID].Key, got, want)
		}
	}
}

func TestForgeOverAndExo(t *testing.T) {
	eng := forgeEngine()
	cloak := eng.Catalog.ItemByID[cloakID]

	base := eng.Forge(cloak, nil)
	if len(base.Issues) != 0 || forgedValues(base)[game.StatVitality] != 300 {
		t.Fatalf("an unforged item keeps its best rolls: %+v", base)
	}

	forged := eng.Forge(cloak, &ItemMods{
		Lines: map[int]int{game.StatVitality: 400, game.StatStrength: 60},
		Exos:  []Exo{{StatID: game.StatAP, Value: 1}},
	})
	if len(forged.Issues) != 0 {
		t.Fatalf("issues = %v", forged.Issues)
	}
	values := forgedValues(forged)
	if values[game.StatVitality] != 400 || values[game.StatStrength] != 60 || values[game.StatAP] != 1 {
		t.Errorf("values = %v", values)
	}
	origins := map[int]string{}
	for _, line := range forged.Lines {
		origins[line.StatID] = line.Origin
	}
	if origins[game.StatVitality] != OriginOver || origins[game.StatStrength] != OriginBase || origins[game.StatAP] != OriginExo {
		t.Errorf("origins = %v", origins)
	}
}

func TestForgeRejectsWhatTheGameForbids(t *testing.T) {
	eng := forgeEngine()
	cloak := eng.Catalog.ItemByID[cloakID]
	cases := []struct {
		name string
		mods ItemMods
		want string
	}{
		{"over above 101 of weight", ItemMods{Lines: map[int]int{game.StatVitality: 506}}, ForgeIssueLine},
		{"below the worst roll", ItemMods{Lines: map[int]int{game.StatStrength: 10}}, ForgeIssueLine},
		{"line the item does not have", ItemMods{Lines: map[int]int{game.StatWisdom: 20}}, ForgeIssueLine},
		{"exo of a natural stat", ItemMods{Exos: []Exo{{StatID: game.StatStrength, Value: 5}}}, ForgeIssueExo},
		{"exo above its cap", ItemMods{Exos: []Exo{{StatID: game.StatAP, Value: 2}}}, ForgeIssueExo},
		{"two limited exos", ItemMods{Exos: []Exo{{StatID: game.StatAP, Value: 1}, {StatID: game.StatMP, Value: 1}}}, ForgeIssueExo},
		{"exo of an unforgeable stat", ItemMods{Exos: []Exo{{StatID: game.StatHP, Value: 1}}}, ForgeIssueExo},
		{"transcendence with an exo", ItemMods{Exos: []Exo{{StatID: game.StatAP, Value: 1}}, Transcendence: runeTaAirID}, ForgeIssueTranscendence},
		{"transcendence with an over", ItemMods{Lines: map[int]int{game.StatStrength: 71}, Transcendence: runeTaAirID}, ForgeIssueTranscendence},
		{"transcendence too heavy for its line", ItemMods{Transcendence: runeVitID}, ForgeIssueTranscendence},
		{"transcendence too heavy for a converted line", ItemMods{Draco: &Draco{Rune: DracoEmerald, From: "earth"}, Transcendence: runeAirID}, ForgeIssueTranscendence},
		{"unknown rune", ItemMods{Transcendence: 1}, ForgeIssueTranscendence},
		{"dracorune to an element already there", ItemMods{Draco: &Draco{Rune: DracoCrimson, From: "earth"}}, ForgeIssueDraco},
		{"dracorune from an absent element", ItemMods{Draco: &Draco{Rune: DracoEmerald, From: "water"}}, ForgeIssueDraco},
		{"ivory to an element already there", ItemMods{Draco: &Draco{Rune: DracoIvory, From: "earth", To: "fire"}}, ForgeIssueDraco},
	}
	for _, c := range cases {
		mods := c.mods
		if got := eng.Forge(cloak, &mods).Issues; !reflect.DeepEqual(got, []string{c.want}) {
			t.Errorf("%s: issues = %v, want [%s]", c.name, got, c.want)
		}
	}

	// Dofus and trophies cannot be forged at all.
	trophy := eng.Catalog.ItemByID[50]
	forged := eng.Forge(trophy, &ItemMods{Exos: []Exo{{StatID: game.StatAP, Value: 1}}})
	if !reflect.DeepEqual(forged.Issues, []string{ForgeIssueSlot}) || forgedValues(forged)[game.StatAP] != 0 {
		t.Errorf("trophy: %+v", forged)
	}
}

func TestForgeTranscendence(t *testing.T) {
	eng := forgeEngine()
	cloak := eng.Catalog.ItemByID[cloakID]
	lineOf := func(f Forged, statID int) ForgedLine {
		for _, l := range f.Lines {
			if l.StatID == statID {
				return l
			}
		}
		return ForgedLine{}
	}

	// 300 Vitality weigh 60: the rune of density 40 fits, the one of 80 does not.
	forged := eng.Forge(cloak, &ItemMods{Transcendence: runeTaVitID})
	if l := lineOf(forged, game.StatVitality); len(forged.Issues) != 0 || l.Value != 350 || l.Rune != 50 || l.Origin != OriginBase {
		t.Errorf("Ta Vi: %+v, issues %v", l, forged.Issues)
	}
	if !reflect.DeepEqual(eng.Forge(cloak, nil).Runes, []int{runeTaVitID, runeAirID, runeTaAirID}) {
		t.Errorf("runes that fit = %v", eng.Forge(cloak, nil).Runes)
	}

	// On a stat the item lacks, any density fits and the line is the rune's.
	forged = eng.Forge(cloak, &ItemMods{Transcendence: runeAirID})
	if l := lineOf(forged, statAirDamage); len(forged.Issues) != 0 || l.Value != 6 || l.Rune != 6 || l.Origin != OriginTranscendence {
		t.Errorf("Rata Do Air: %+v, issues %v", l, forged.Issues)
	}

	// A dracorune turns 10 Earth damage into 10 Air damage, weight 50: only
	// the light rune still fits, and the line shows both.
	emerald := &Draco{Rune: DracoEmerald, From: "earth"}
	forged = eng.Forge(cloak, &ItemMods{Draco: emerald})
	if !reflect.DeepEqual(forged.Runes, []int{runeTaVitID, runeTaAirID}) {
		t.Errorf("runes after the dracorune = %v", forged.Runes)
	}
	forged = eng.Forge(cloak, &ItemMods{Draco: emerald, Transcendence: runeTaAirID})
	if l := lineOf(forged, statAirDamage); len(forged.Issues) != 0 || l.Value != 12 || l.Rune != 2 || l.Origin != OriginDraco {
		t.Errorf("dracorune and Ta Do Air: %+v, issues %v", l, forged.Issues)
	}

	// An item with an exo takes no rune at all.
	if got := eng.Forge(cloak, &ItemMods{Exos: []Exo{{StatID: game.StatAP, Value: 1}}}).Runes; len(got) != 0 {
		t.Errorf("runes with an exo = %v", got)
	}
}

func TestDracorunes(t *testing.T) {
	eng := forgeEngine()
	cloak := eng.Catalog.ItemByID[cloakID]

	// Emerald: Strength and Earth damage both become Agility and Air damage.
	emerald := eng.Forge(cloak, &ItemMods{Draco: &Draco{Rune: DracoEmerald, From: "earth"}})
	values := forgedValues(emerald)
	if len(emerald.Issues) != 0 || values[statAgility] != 70 || values[statAirDamage] != 10 {
		t.Errorf("emerald: issues %v, values %v", emerald.Issues, values)
	}
	if _, kept := values[game.StatStrength]; kept {
		t.Error("emerald: Strength should be gone")
	}
	if values[game.StatIntel] != 50 {
		t.Error("emerald: the other element stays")
	}
	// The converted line keeps the place of the one it replaces.
	if emerald.Lines[1].StatID != statAgility || emerald.Lines[1].Origin != OriginDraco {
		t.Errorf("emerald: second line = %+v", emerald.Lines[1])
	}

	// A dracorune works on an item that is already over and exo.
	both := eng.Forge(cloak, &ItemMods{
		Lines: map[int]int{game.StatStrength: 90},
		Exos:  []Exo{{StatID: game.StatMP, Value: 1}},
		Draco: &Draco{Rune: DracoEmerald, From: "earth"},
	})
	if len(both.Issues) != 0 || forgedValues(both)[statAgility] != 90 || forgedValues(both)[game.StatMP] != 1 {
		t.Errorf("over + exo + dracorune: %+v", both)
	}

	// Ivory: one resistance element moves to another the item lacks.
	ivory := eng.Forge(cloak, &ItemMods{Draco: &Draco{Rune: DracoIvory, From: "earth", To: "water"}})
	values = forgedValues(ivory)
	if len(ivory.Issues) != 0 || values[statWaterRes] != 10 || values[statFireRes] != 10 {
		t.Errorf("ivory: issues %v, values %v", ivory.Issues, values)
	}
	if _, kept := values[statEarthRes]; kept {
		t.Error("ivory: Earth resistance should be gone")
	}

	// Ebony: 70% of the highest stat becomes Power, 70% of the highest
	// elemental damage becomes Damage; the lines are not added together.
	ebony := eng.Forge(cloak, &ItemMods{Draco: &Draco{Rune: DracoEbony}})
	values = forgedValues(ebony)
	if len(ebony.Issues) != 0 || values[game.StatPower] != 49 || values[statAllDamage] != 7 {
		t.Errorf("ebony: issues %v, values %v", ebony.Issues, values)
	}
	for _, gone := range []int{game.StatStrength, game.StatIntel, statEarthDamage} {
		if _, kept := values[gone]; kept {
			t.Errorf("ebony: stat %d should be gone", gone)
		}
	}
}

func TestDracoOptions(t *testing.T) {
	eng := forgeEngine()
	options := eng.DracoOptions(eng.Catalog.ItemByID[cloakID])
	count := map[string]int{}
	for _, option := range options {
		count[option.Rune]++
		if forged := eng.Forge(eng.Catalog.ItemByID[cloakID], &ItemMods{Draco: &option}); len(forged.Issues) != 0 {
			t.Errorf("option %+v is listed but rejected: %v", option, forged.Issues)
		}
	}
	// Earth and Fire are on the cloak, so only Water and Air can be obtained,
	// each from Earth or Fire. Resistances can go to Neutral, Water or Air.
	want := map[string]int{DracoTurquoise: 2, DracoEmerald: 2, DracoIvory: 6, DracoEbony: 1}
	if !reflect.DeepEqual(count, want) {
		t.Errorf("options per rune = %v, want %v", count, want)
	}
	if got := eng.DracoOptions(eng.Catalog.ItemByID[50]); got != nil {
		t.Errorf("a trophy has no dracorune option, got %v", got)
	}
}

func TestEvaluateAppliesForgemagic(t *testing.T) {
	eng := forgeEngine()
	res := eng.Evaluate(Build{
		Character: Character{Level: 200},
		Items:     map[string]int{"cloak": cloakID, "ring1": 12},
		Mods: map[string]*ItemMods{
			"cloak": {Lines: map[int]int{game.StatVitality: 350}, Exos: []Exo{{StatID: game.StatAP, Value: 1}}},
		},
	}, "fr")
	if !res.Valid || res.Stats[game.StatAP] != 8 || res.Stats[game.StatVitality] != 350 {
		t.Errorf("valid %v, AP %d, Vitality %d, issues %+v", res.Valid, res.Stats[game.StatAP], res.Stats[game.StatVitality], res.Issues)
	}
	lines := res.Forge["cloak"]
	if len(lines) != 7 || lines[0].Text != "350 Vitalité" || lines[6].Text != "1 PA" || lines[6].Origin != OriginExo {
		t.Errorf("forged lines = %+v", lines)
	}
	if _, ok := res.Forge["ring1"]; !ok {
		t.Error("every forgeable item reports its lines, forged or not")
	}

	// One AP exo per character: the second one is flagged.
	res = eng.Evaluate(Build{
		Character: Character{Level: 200},
		Items:     map[string]int{"cloak": cloakID, "ring1": 12},
		Mods: map[string]*ItemMods{
			"cloak": {Exos: []Exo{{StatID: game.StatAP, Value: 1}}},
			"ring1": {Exos: []Exo{{StatID: game.StatAP, Value: 1}}},
		},
	}, "fr")
	if res.Valid || len(res.Issues) != 1 || res.Issues[0].Code != ForgeIssueExoLimit {
		t.Errorf("issues = %+v", res.Issues)
	}
}

func TestForgeNegativeLines(t *testing.T) {
	const bootsID, runeDoCri = 61, 910
	const statCritDamage = 86
	ds := gametest.Dataset()
	ds.Stats = append(ds.Stats,
		game.Stat{ID: game.StatCritical, Key: "criticalHit", CategoryID: 2, Order: 18, Weight: 10, Label: game.Texts{"fr": "Critique"}},
		game.Stat{ID: statCritDamage, Key: "criticalDamageBonus", CategoryID: 2, Order: 86, Weight: 5, Label: game.Texts{"fr": "Dommages Critiques"}},
	)
	ds.Types = append(ds.Types, game.ItemType{ID: 11, SuperTypeID: 5, Slot: game.SlotBoots, Name: game.Texts{"fr": "Bottes"}})
	ds.Items = append(ds.Items, game.Item{
		ID: bootsID, TypeID: 11, Slot: game.SlotBoots, Level: 200, IconID: bootsID,
		Name: game.Texts{"fr": "Bottes d'Essai"}, Description: game.Texts{},
		Effects: []game.ItemEffect{
			{EffectID: 171, Kind: game.EffectStat, StatID: game.StatCritical, Min: -7, Max: -5},
			{EffectID: 419, Kind: game.EffectStat, StatID: statCritDamage, Min: -15, Max: -11},
		},
	})
	ds.Runes = append(ds.Runes, game.Rune{
		ID: runeDoCri, EffectID: 418, StatID: statCritDamage, Value: 8, Density: 80, Name: game.Texts{"fr": "Rune Pata Do Cri"},
	})
	eng := New(game.NewCatalog(ds))
	boots := eng.Catalog.ItemByID[bootsID]
	value := func(f Forged, statID int) int { return forgedValues(f)[statID] }

	// A malus is forged up to -1, never to zero.
	raised := eng.Forge(boots, &ItemMods{Lines: map[int]int{game.StatCritical: -1}})
	if len(raised.Issues) != 0 || value(raised, game.StatCritical) != -1 {
		t.Errorf("-5 raised to -1: %+v", raised)
	}
	gone := eng.Forge(boots, &ItemMods{Lines: map[int]int{game.StatCritical: 0}})
	if !reflect.DeepEqual(gone.Issues, []string{ForgeIssueLine}) || value(gone, game.StatCritical) != -1 {
		t.Errorf("-5 raised to 0: %+v", gone)
	}
	// A negative line weighs nothing, so the heaviest rune lands on it.
	runed := eng.Forge(boots, &ItemMods{Transcendence: runeDoCri})
	if len(runed.Issues) != 0 || value(runed, statCritDamage) != -3 {
		t.Errorf("rune on a malus: %+v", runed)
	}
	// But not on an item whose malus was forged up: that is an over.
	both := eng.Forge(boots, &ItemMods{Lines: map[int]int{game.StatCritical: -1}, Transcendence: runeDoCri})
	if !reflect.DeepEqual(both.Issues, []string{ForgeIssueTranscendence}) {
		t.Errorf("over and rune: %v", both.Issues)
	}
}
