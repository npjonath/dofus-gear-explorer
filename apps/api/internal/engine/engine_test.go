package engine

import (
	"reflect"
	"testing"

	"dofusdb/api/internal/game"
	"dofusdb/api/internal/gametest"
)

func newEngine() *Engine { return New(game.NewCatalog(gametest.Dataset())) }

func TestPointCost(t *testing.T) {
	cases := []struct {
		key   string
		value int
		want  int
	}{
		{"vitality", 250, 250},
		{"wisdom", 100, 300},
		{"strength", 100, 100},
		{"strength", 150, 200},
		{"strength", 300, 600},
		{"strength", 350, 800},
	}
	for _, c := range cases {
		if got := PointCost(c.key, c.value); got != c.want {
			t.Errorf("PointCost(%s, %d) = %d, want %d", c.key, c.value, got, c.want)
		}
	}
}

func TestEvaluateSetBonusAndDerivedStats(t *testing.T) {
	eng := newEngine()
	res := eng.Evaluate(Build{
		Character: Character{Level: 200, Base: map[string]int{"vitality": 100}},
		Items:     map[string]int{"ring1": 10, "ring2": 11, "amulet": 30},
	}, "fr")

	// 7 base AP at level 200, +1 from the ring, +1 from the three-piece bonus.
	if got := res.Stats[game.StatAP]; got != 9 {
		t.Errorf("AP = %d, want 9", got)
	}
	// 40 + 30 from items, +50 from the set.
	if got := res.Stats[game.StatStrength]; got != 120 {
		t.Errorf("Strength = %d, want 120", got)
	}
	// 50 + 5 per level, plus 100 base and 150 item vitality.
	if got := res.Stats[game.StatHP]; got != 1300 {
		t.Errorf("HP = %d, want 1300", got)
	}
	if got := res.Stats[game.StatInitiative]; got != 120 {
		t.Errorf("Initiative = %d, want 120", got)
	}
	if res.SetBonuses != 2 || len(res.Sets) != 1 || res.Sets[0].Count != 3 {
		t.Errorf("sets = %+v, bonuses = %d", res.Sets, res.SetBonuses)
	}
	if !res.Valid {
		t.Errorf("build should be valid, issues = %+v", res.Issues)
	}
}

func TestEvaluateFlagsProblems(t *testing.T) {
	eng := newEngine()

	res := eng.Evaluate(Build{
		Character: Character{Level: 200},
		Items:     map[string]int{"hat": 40, "ring1": 10},
	}, "fr")
	// 7 + 1 + 1 = 9 AP breaks the hat's "AP < 8".
	if res.Valid || len(res.Conditions) != 1 || res.Conditions[0].OK {
		t.Errorf("condition should fail: %+v", res.Conditions)
	}
	if got := res.Conditions[0].Text; got != "PA < 8" {
		t.Errorf("condition text = %q", got)
	}

	res = eng.Evaluate(Build{
		Character: Character{Level: 50},
		Items:     map[string]int{"ring1": 10, "ring2": 10, "dofus1": 50, "dofus2": 50},
	}, "fr")
	codes := map[string]int{}
	for _, issue := range res.Issues {
		codes[issue.Code]++
	}
	if codes[IssueLevel] != 4 || codes[IssueDuplicate] != 2 {
		t.Errorf("issues = %+v", res.Issues)
	}

	res = eng.Evaluate(Build{Character: Character{Level: 10, Base: map[string]int{"strength": 100}}}, "fr")
	if len(res.Issues) != 1 || res.Issues[0].Code != IssuePoints {
		t.Errorf("expected a points issue, got %+v", res.Issues)
	}
}

func TestCriteria(t *testing.T) {
	eng := newEngine()
	cat := eng.Catalog
	totals := make([]int32, eng.StatCount())
	totals[cat.Idx(game.StatAP)] = 12
	totals[cat.Idx(game.StatMP)] = 5
	totals[cat.Idx(game.StatStrength)] = 300
	ctx := &CriteriaContext{Catalog: cat, Totals: totals, Base: make([]int32, eng.StatCount()), SetBonuses: 2}

	cases := []struct {
		expr string
		ok   bool
		text string
	}{
		{"CP<12|CM<6", true, "PA < 12 ou PM < 6"},
		{"CP<12&CM<6", false, "PA < 12 et PM < 6"},
		{"CS>299", true, "Force > 299"},
		{"pk<2", false, "Bonus de panoplie < 2"},
		{"Cs>10", false, "Force de base > 10"},
		{"Qa=1865&Qo>12441", true, "Condition spéciale"},
		{"(CS>299&CP<12)|CM<6", true, "(Force > 299 et PA < 12) ou PM < 6"},
	}
	for _, c := range cases {
		crit := ParseCriteria(c.expr)
		if got := crit.Check(ctx); got != c.ok {
			t.Errorf("%s: Check = %v, want %v", c.expr, got, c.ok)
		}
		if got := crit.Text(cat, "fr"); got != c.text {
			t.Errorf("%s: Text = %q, want %q", c.expr, got, c.text)
		}
	}
	if ParseCriteria("") != nil {
		t.Error("empty criteria should parse to nil")
	}
}

func TestCriteriaWordsCoverEveryLanguage(t *testing.T) {
	for _, lang := range []string{"fr", "en", "es", "pt"} {
		words, ok := criteriaWords[lang]
		if !ok {
			t.Errorf("no criteria words for %s", lang)
			continue
		}
		for _, key := range []string{"and", "or", "special", "sets", "base"} {
			if words[key] == "" {
				t.Errorf("%s: no %q", lang, key)
			}
		}
	}
}

// withPrysmaradites adds two prysmaradites, each worth one AP.
func withPrysmaradites() *Engine {
	ds := gametest.Dataset()
	ds.Types = append(ds.Types, game.ItemType{
		ID: game.TypePrysmaradite, SuperTypeID: 13, Slot: game.SlotDofus, Name: game.Texts{"fr": "Prysmaradite"},
	})
	for _, id := range []int{70, 71} {
		ds.Items = append(ds.Items, game.Item{
			ID: id, TypeID: game.TypePrysmaradite, Slot: game.SlotDofus, Level: 200, IconID: id,
			Name: game.Texts{"fr": "Prysma d'Essai"}, Description: game.Texts{},
			Effects: []game.ItemEffect{{
				EffectID: gametest.EffectAP, Kind: game.EffectStat, StatID: game.StatAP, Min: 1, Max: 1,
			}},
		})
	}
	return New(game.NewCatalog(ds))
}

func TestPrysmaraditeOnlyInTheFirstDofusPosition(t *testing.T) {
	eng := withPrysmaradites()

	ok := eng.Evaluate(Build{
		Character: Character{Level: 200},
		Items:     map[string]int{"dofus1": 70, "dofus2": 50},
	}, "fr")
	if !ok.Valid {
		t.Errorf("a prysmaradite in the first position is fine: %+v", ok.Issues)
	}

	// Anywhere else it is refused, which also rules out wearing two.
	bad := eng.Evaluate(Build{
		Character: Character{Level: 200},
		Items:     map[string]int{"dofus1": 70, "dofus4": 71},
	}, "fr")
	if bad.Valid || len(bad.Issues) != 1 || bad.Issues[0].Code != IssuePrysmaradite || bad.Issues[0].Position != "dofus4" {
		t.Errorf("issues = %+v", bad.Issues)
	}
}

func TestStatsAreCappedLikeInGame(t *testing.T) {
	eng := withPrysmaradites()

	// Gear can add up past the cap; the game keeps the cap and the rest is lost.
	raw := make([]int32, eng.StatCount())
	raw[eng.Catalog.Idx(game.StatAP)] = 14
	raw[eng.Catalog.Idx(game.StatMP)] = 9
	out := make([]int32, eng.StatCount())
	eng.Derive(200, raw, out)
	if ap, mp := out[eng.Catalog.Idx(game.StatAP)], out[eng.Catalog.Idx(game.StatMP)]; ap != MaxAP || mp != MaxMP {
		t.Errorf("AP %d, MP %d; want %d and %d", ap, mp, MaxAP, MaxMP)
	}

	// 7 base AP, the ring, the three-piece bonus, one prysmaradite and an exo: 11.
	items := map[string]int{"ring1": 10, "ring2": 11, "amulet": 30, "dofus1": 70}
	mods := map[string]*ItemMods{"ring2": {Exos: []Exo{{StatID: game.StatAP, Value: 1}}}}
	res := eng.Evaluate(Build{Character: Character{Level: 200}, Items: items, Mods: mods}, "fr")
	if res.Stats[game.StatAP] != 11 || len(res.Capped) != 0 || !res.Valid {
		t.Fatalf("AP %d, capped %+v, valid %v", res.Stats[game.StatAP], res.Capped, res.Valid)
	}

	// The hat and a second prysmaradite bring the sum to 13: 12 are kept.
	items["hat"], items["dofus2"] = 40, 71
	res = eng.Evaluate(Build{Character: Character{Level: 200}, Items: items, Mods: mods}, "fr")
	if got := res.Stats[game.StatAP]; got != MaxAP {
		t.Errorf("AP = %d, want the cap of %d", got, MaxAP)
	}
	want := []CappedStat{{StatID: game.StatAP, Max: MaxAP, Raw: 13}}
	if !reflect.DeepEqual(res.Capped, want) {
		t.Errorf("capped = %+v, want %+v", res.Capped, want)
	}
}
