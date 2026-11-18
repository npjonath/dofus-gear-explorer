package solver

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"dofusdb/api/internal/engine"
	"dofusdb/api/internal/game"
	"dofusdb/api/internal/gametest"
)

func newEngine() *engine.Engine { return engine.New(game.NewCatalog(gametest.Dataset())) }

func TestSolveMeetsConstraints(t *testing.T) {
	eng := newEngine()
	req := Request{
		Character:   engine.Character{Level: 200},
		Constraints: []Constraint{{StatID: game.StatAP, Op: "gte", Value: 9}},
		Maximize:    []Weight{{StatID: game.StatStrength, Weight: 1}},
	}
	resp, err := Solve(context.Background(), eng, req)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Solutions) == 0 || !resp.Solutions[0].Feasible {
		t.Fatalf("expected a feasible solution, got %+v", resp.Solutions)
	}
	best := resp.Solutions[0]
	res := eng.Evaluate(engine.Build{Character: req.Character, Items: best.Items}, "fr")
	if !res.Valid {
		t.Errorf("best solution is not a valid build: %+v %+v", res.Issues, res.Conditions)
	}
	if res.Stats[game.StatAP] < 9 {
		t.Errorf("AP = %d, want at least 9", res.Stats[game.StatAP])
	}
	// The whole set (+50 Strength, +1 AP) beats the conditioned hat's AP.
	if res.Stats[game.StatStrength] != 180 {
		t.Errorf("Strength = %d, want 180 (items %v)", res.Stats[game.StatStrength], best.Items)
	}

	again, err := Solve(context.Background(), eng, req)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(resp.Solutions, again.Solutions) {
		t.Error("the same request should give the same solutions")
	}
}

func TestSolveRespectsLevelLocksAndExclusions(t *testing.T) {
	eng := newEngine()
	resp, err := Solve(context.Background(), eng, Request{
		Character: engine.Character{Level: 70},
		Maximize:  []Weight{{StatID: game.StatStrength, Weight: 1}},
		Locked:    map[string]int{"dofus1": 51},
		Excluded:  []int{30},
	})
	if err != nil {
		t.Fatal(err)
	}
	items := resp.Solutions[0].Items
	if items["dofus1"] != 51 {
		t.Errorf("locked trophy was replaced: %v", items)
	}
	for position, id := range items {
		if id == 30 {
			t.Errorf("excluded amulet used at %s", position)
		}
		// A locked item is the player's choice and stays, whatever its level.
		if it := eng.Catalog.ItemByID[id]; position != "dofus1" && it.Level > 70 {
			t.Errorf("%s is above the character level", it.Name.Get("fr"))
		}
	}
}

func TestSolveReportsUnmetConstraints(t *testing.T) {
	eng := newEngine()
	resp, err := Solve(context.Background(), eng, Request{
		Character:   engine.Character{Level: 200},
		Constraints: []Constraint{{StatID: game.StatAP, Op: "gte", Value: 20}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Solutions) == 0 || resp.Solutions[0].Feasible || !reflect.DeepEqual(resp.Solutions[0].Unmet, []int{0}) {
		t.Errorf("expected an infeasible best effort, got %+v", resp.Solutions)
	}
}

func TestSolveNeedsCriteria(t *testing.T) {
	_, err := Solve(context.Background(), newEngine(), Request{Character: engine.Character{Level: 200}})
	if !errors.Is(err, ErrNothingToSearch) {
		t.Errorf("err = %v, want ErrNothingToSearch", err)
	}
}

func TestSolveHandsOutAllowedExos(t *testing.T) {
	eng := newEngine()
	req := Request{
		Character:   engine.Character{Level: 200},
		Constraints: []Constraint{{StatID: game.StatAP, Op: "gte", Value: 10}},
		Maximize:    []Weight{{StatID: game.StatStrength, Weight: 1}},
	}
	without, err := Solve(context.Background(), eng, req)
	if err != nil {
		t.Fatal(err)
	}
	if without.Solutions[0].Feasible {
		t.Fatal("10 AP should be out of reach without an exo")
	}

	req.Forge = ForgeOptions{Exos: []int{game.StatAP}}
	with, err := Solve(context.Background(), eng, req)
	if err != nil {
		t.Fatal(err)
	}
	best := with.Solutions[0]
	if !best.Feasible {
		t.Fatalf("10 AP should be reachable with one AP exo: %+v", best)
	}
	exos := 0
	for position, mods := range best.Mods {
		for _, exo := range mods.Exos {
			exos++
			item := eng.Catalog.ItemByID[best.Items[position]]
			if exo.StatID != game.StatAP || exo.Value != 1 || !engine.ForgeableSlots[item.Slot] || hasStat(item, game.StatAP) {
				t.Errorf("bad exo %+v on %s", exo, item.Name.Get("fr"))
			}
		}
	}
	if exos != 1 {
		t.Errorf("exos handed out = %d, want 1", exos)
	}
	res := eng.Evaluate(engine.Build{Character: req.Character, Items: best.Items, Mods: best.Mods}, "fr")
	if !res.Valid || res.Stats[game.StatAP] < 10 {
		t.Errorf("valid %v, AP %d, issues %+v", res.Valid, res.Stats[game.StatAP], res.Issues)
	}
}

func TestSolveWearsAtMostOnePrysmaradite(t *testing.T) {
	ds := gametest.Dataset()
	ds.Types = append(ds.Types, game.ItemType{
		ID: game.TypePrysmaradite, SuperTypeID: 13, Slot: game.SlotDofus, Name: game.Texts{"fr": "Prysmaradite"},
	})
	// Three prysmaradites, each better than any trophy: the search would wear
	// all three if nothing stopped it.
	for _, id := range []int{70, 71, 72} {
		ds.Items = append(ds.Items, game.Item{
			ID: id, TypeID: game.TypePrysmaradite, Slot: game.SlotDofus, Level: 200, IconID: id,
			Name: game.Texts{"fr": "Prysma d'Essai"}, Description: game.Texts{},
			Effects: []game.ItemEffect{{
				EffectID: gametest.EffectStrength, Kind: game.EffectStat, StatID: game.StatStrength, Min: 500, Max: 500,
			}},
		})
	}
	eng := engine.New(game.NewCatalog(ds))
	req := Request{
		Character: engine.Character{Level: 200},
		Maximize:  []Weight{{StatID: game.StatStrength, Weight: 1}},
	}
	resp, err := Solve(context.Background(), eng, req)
	if err != nil {
		t.Fatal(err)
	}
	for _, sol := range resp.Solutions {
		for position, id := range sol.Items {
			if eng.Catalog.ItemByID[id].TypeID == game.TypePrysmaradite && position != game.PrysmaraditePosition {
				t.Errorf("prysmaradite %d at %s", id, position)
			}
		}
	}
	best := resp.Solutions[0]
	if eng.Catalog.ItemByID[best.Items["dofus1"]].TypeID != game.TypePrysmaradite {
		t.Errorf("the best stuff should wear one prysmaradite in the first position: %v", best.Items)
	}
	if res := eng.Evaluate(engine.Build{Character: req.Character, Items: best.Items}, "fr"); !res.Valid {
		t.Errorf("best stuff is not valid: %+v", res.Issues)
	}
}

func TestSolvePrefersWhatServesTheRequestOverItemLevel(t *testing.T) {
	// The level-80 ring brings 40 Strength, the level-190 one takes 10 away:
	// a level-200 character wears the weaker-levelled ring.
	eng := newEngine()
	resp, err := Solve(context.Background(), eng, Request{
		Character: engine.Character{Level: 200},
		Maximize:  []Weight{{StatID: game.StatStrength, Weight: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	items := resp.Solutions[0].Items
	if items["ring1"] != 11 && items["ring2"] != 11 {
		t.Errorf("the level-80 ring with the most Strength should be worn: %v", items)
	}
	for position, id := range items {
		if id == 12 {
			t.Errorf("the level-190 ring that lowers Strength is worn at %s", position)
		}
	}
}

func TestSolveFillsEverySlotItCan(t *testing.T) {
	eng := newEngine()
	// Only Wisdom is asked for. The trophies give none, yet the Dofus slots
	// must still be filled with the best of what is left.
	resp, err := Solve(context.Background(), eng, Request{
		Character: engine.Character{Level: 200},
		Maximize:  []Weight{{StatID: game.StatWisdom, Weight: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	items := resp.Solutions[0].Items
	worn := map[int]bool{}
	for _, id := range items {
		worn[id] = true
	}
	if !worn[50] || !worn[51] {
		t.Errorf("both trophies should be worn even though they give no Wisdom: %v", items)
	}
	if items["ring1"] == 0 || items["ring2"] == 0 || items["amulet"] == 0 {
		t.Errorf("rings and amulet should be filled: %v", items)
	}
	// The only hat breaks its own condition with this stuff's AP: the slot
	// stays empty rather than making the stuff invalid.
	res := eng.Evaluate(engine.Build{Character: engine.Character{Level: 200}, Items: items}, "fr")
	if !res.Valid {
		t.Errorf("filling slots must not cost validity: %+v %+v", res.Issues, res.Conditions)
	}
}

func TestSolvePicksRunesThatFit(t *testing.T) {
	const taFo, rataFo = 900, 901
	ds := gametest.Dataset()
	ds.Runes = append(ds.Runes,
		game.Rune{ID: taFo, EffectID: gametest.EffectStrength, StatID: game.StatStrength, Value: 10, Density: 40, Name: game.Texts{"fr": "Rune Ta Fo"}},
		game.Rune{ID: rataFo, EffectID: gametest.EffectStrength, StatID: game.StatStrength, Value: 20, Density: 80, Name: game.Texts{"fr": "Rune Rata Fo"}},
	)
	eng := engine.New(game.NewCatalog(ds))
	req := Request{
		Character: engine.Character{Level: 200},
		Maximize:  []Weight{{StatID: game.StatStrength, Weight: 1}},
		Forge:     ForgeOptions{Transcendence: true},
	}
	out, err := Solve(context.Background(), eng, req)
	if err != nil {
		t.Fatal(err)
	}
	best := out.Solutions[0]
	res := eng.Evaluate(engine.Build{Character: best.Character, Items: best.Items, Mods: best.Mods}, "fr")
	if !res.Valid {
		t.Fatalf("issues %+v", res.Issues)
	}
	// A line of more than 21 Strength only has room for the light rune; an
	// item without Strength takes the heavy one.
	light, heavy := 0, 0
	for position, mods := range best.Mods {
		item := eng.Catalog.ItemByID[best.Items[position]]
		strength := 0
		for _, e := range item.Effects {
			if e.StatID == game.StatStrength {
				strength += e.Max
			}
		}
		switch {
		case strength > 21 && mods.Transcendence == taFo:
			light++
		case strength <= 21 && mods.Transcendence == rataFo:
			heavy++
		default:
			t.Errorf("%s (%d Strength) holds rune %d", item.Name.Get("fr"), strength, mods.Transcendence)
		}
	}
	if light == 0 || heavy == 0 {
		t.Errorf("light runes %d, heavy runes %d, want both", light, heavy)
	}
}

func TestSolveWearsRequiredItems(t *testing.T) {
	eng := newEngine()
	// Three rings are required for two ring positions: the first two are
	// worn, the third is reported. The ring kept at ring1 gives way.
	req := Request{
		Character: engine.Character{Level: 200},
		Maximize:  []Weight{{StatID: game.StatStrength, Weight: 1}},
		Required:  []int{12, 10, 11, 12, 99999},
		Locked:    map[string]int{"ring1": 11, "amulet": 30},
	}
	out, err := Solve(context.Background(), eng, req)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(out.Unplaced, []int{11}) {
		t.Errorf("unplaced = %v, want [11]", out.Unplaced)
	}
	for _, sol := range out.Solutions {
		if sol.Items["ring1"] != 12 || sol.Items["ring2"] != 10 || sol.Items["amulet"] != 30 {
			t.Errorf("items = %v", sol.Items)
		}
	}

	out, err = Solve(context.Background(), eng, Request{
		Character: engine.Character{Level: 200},
		Maximize:  []Weight{{StatID: game.StatStrength, Weight: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Unplaced == nil || len(out.Unplaced) != 0 {
		t.Errorf("unplaced = %#v, want an empty list", out.Unplaced)
	}
}

func TestSolveRanksWhatItMaximizes(t *testing.T) {
	// 100 Strength weigh 100 and 60 Wisdom weigh 180: with equal shares the
	// Wisdom amulet wins, so only the order can put the Strength one first.
	const strong, wise = 70, 71
	ds := gametest.Dataset()
	amulet := func(id int, name string, effectID, statID, value int) game.Item {
		return game.Item{
			ID: id, TypeID: 1, Slot: game.SlotAmulet, Level: 200, IconID: id,
			Name: game.Texts{"fr": name}, Description: game.Texts{},
			Effects: []game.ItemEffect{{EffectID: effectID, Kind: game.EffectStat, StatID: statID, Min: value, Max: value}},
		}
	}
	ds.Items = append(ds.Items,
		amulet(strong, "Amulette Forte", gametest.EffectStrength, game.StatStrength, 100),
		amulet(wise, "Amulette Sage", gametest.EffectWisdom, game.StatWisdom, 60),
	)
	eng := engine.New(game.NewCatalog(ds))
	worn := func(first, second int) int {
		t.Helper()
		out, err := Solve(context.Background(), eng, Request{
			Character: engine.Character{Level: 200},
			Maximize:  []Weight{{StatID: first, Weight: 1}, {StatID: second, Weight: 1}},
		})
		if err != nil {
			t.Fatal(err)
		}
		return out.Solutions[0].Items["amulet"]
	}
	if got := worn(game.StatStrength, game.StatWisdom); got != strong {
		t.Errorf("Strength first: amulet %d, want %d", got, strong)
	}
	if got := worn(game.StatWisdom, game.StatStrength); got != wise {
		t.Errorf("Wisdom first: amulet %d, want %d", got, wise)
	}
}

func TestSolveReachesDamageGoals(t *testing.T) {
	const bow = 90
	ds := gametest.Dataset()
	ds.Types = append(ds.Types, game.ItemType{ID: 2, SuperTypeID: 2, Slot: game.SlotWeapon, Name: game.Texts{"fr": "Arc"}})
	ds.Items = append(ds.Items, game.Item{
		ID: bow, TypeID: 2, Slot: game.SlotWeapon, Level: 100, IconID: bow,
		Name: game.Texts{"fr": "Arc d'Essai"}, Description: game.Texts{},
		Weapon:  &game.WeaponInfo{APCost: 4, RangeMin: 2, RangeMax: 6, CritChance: 10, CritBonus: 5},
		Effects: []game.ItemEffect{{EffectID: 97, Kind: game.EffectWeapon, StatID: -1, Min: 10, Max: 20}},
	})
	eng := engine.New(game.NewCatalog(ds))
	solve := func(goal DamageGoal) *Response {
		t.Helper()
		out, err := Solve(context.Background(), eng, Request{
			Character: engine.Character{Level: 200}, AutoPoints: true, Damage: []DamageGoal{goal},
		})
		if err != nil {
			t.Fatal(err)
		}
		return out
	}

	// The bow averages 15: with the points in Strength and the Strength gear
	// it passes 60, and the stuff has to wear it.
	best := solve(DamageGoal{Kind: "weapon", ID: bow, Value: 60}).Solutions[0]
	if !best.Feasible || best.Items["weapon"] != bow || len(best.Damage) != 1 || best.Damage[0] < 60 {
		t.Fatalf("weapon goal: %+v", best)
	}
	if best.Character.Base["strength"] == 0 {
		t.Errorf("the points should go to Strength: %v", best.Character.Base)
	}
	// What the solver reports is what the engine computes on the stuff.
	res := eng.Evaluate(engine.Build{Character: best.Character, Items: best.Items, Mods: best.Mods}, "")
	hit := engine.WeaponSpec(eng.Catalog.ItemByID[bow], engine.WeaponOptions{Mastery: true}).Total(func(id int) int { return res.Stats[id] })
	if got := (hit.Min + hit.Max) / 2; got != best.Damage[0] {
		t.Errorf("reported %d, engine %d", best.Damage[0], got)
	}

	// A critical hit counts the bow's bonus; a value out of reach is reported.
	crit := solve(DamageGoal{Kind: "weapon", ID: bow, Measure: MeasureCrit, Value: 100000}).Solutions[0]
	if crit.Feasible || !reflect.DeepEqual(crit.UnmetDamage, []int{0}) || crit.Damage[0] <= best.Damage[0] {
		t.Errorf("unreachable critical goal: feasible %v, unmet %v, damage %v", crit.Feasible, crit.UnmetDamage, crit.Damage)
	}

	// A spell goal uses the grade of the character's level.
	spell := solve(DamageGoal{Kind: "spell", ID: gametest.SpellEarth, Value: 100}).Solutions[0]
	if !spell.Feasible || spell.Damage[0] < 100 {
		t.Errorf("spell goal: %+v", spell)
	}

	for _, bad := range []DamageGoal{
		{Kind: "weapon", ID: 40, Value: 10},                               // a hat
		{Kind: "spell", ID: 99999, Value: 10},                             // unknown
		{Kind: "spell", ID: gametest.SpellEarth, Value: 0},                // no value
		{Kind: "spell", ID: gametest.SpellEarth, Range: "far", Value: 10}, // bad range
		{Kind: "armes", ID: bow, Value: 10},                               // bad kind
	} {
		_, err := Solve(context.Background(), eng, Request{Character: engine.Character{Level: 200}, Damage: []DamageGoal{bad}})
		if !errors.Is(err, ErrBadDamageGoal) {
			t.Errorf("%+v: err = %v", bad, err)
		}
	}
}

// forgeDataset adds Critical, three forgeable items and runes to the shared
// dataset: a Critical rune and a Strength rune compete for the same items.
func forgeDataset() *game.Dataset {
	ds := gametest.Dataset()
	ds.Stats = append(ds.Stats, game.Stat{
		ID: game.StatCritical, Key: "criticalHit", CategoryID: 2, Order: 18, Weight: 10, Label: game.Texts{"fr": "Critique"},
	})
	ds.Types = append(ds.Types,
		game.ItemType{ID: 10, SuperTypeID: 4, Slot: game.SlotBelt, Name: game.Texts{"fr": "Ceinture"}},
		game.ItemType{ID: 11, SuperTypeID: 5, Slot: game.SlotBoots, Name: game.Texts{"fr": "Bottes"}},
		game.ItemType{ID: 17, SuperTypeID: 11, Slot: game.SlotCloak, Name: game.Texts{"fr": "Cape"}},
	)
	gear := func(id, typeID int, slot game.Slot, name string, effects ...game.ItemEffect) game.Item {
		return game.Item{
			ID: id, TypeID: typeID, Slot: slot, Level: 200, IconID: id,
			Name: game.Texts{"fr": name}, Description: game.Texts{}, Effects: effects,
		}
	}
	vitality := game.ItemEffect{EffectID: gametest.EffectVitality, Kind: game.EffectStat, StatID: game.StatVitality, Min: 100, Max: 100}
	ds.Items = append(ds.Items,
		gear(60, 10, game.SlotBelt, "Ceinture d'Essai", vitality),
		gear(61, 11, game.SlotBoots, "Bottes d'Essai", vitality,
			game.ItemEffect{EffectID: 171, Kind: game.EffectStat, StatID: game.StatCritical, Min: -7, Max: -5}),
		gear(62, 17, game.SlotCloak, "Cape d'Essai", vitality),
	)
	ds.Runes = append(ds.Runes,
		game.Rune{ID: 900, EffectID: 115, StatID: game.StatCritical, Value: 2, Density: 80, Name: game.Texts{"fr": "Rune Pata Cri"}},
		game.Rune{ID: 901, EffectID: gametest.EffectStrength, StatID: game.StatStrength, Value: 20, Density: 80, Name: game.Texts{"fr": "Rune Rata Fo"}},
	)
	return ds
}

func TestSolveChoosesForgemagicForTheStuff(t *testing.T) {
	eng := engine.New(game.NewCatalog(forgeDataset()))
	solve := func(critical int, forge ForgeOptions) Solution {
		t.Helper()
		out, err := Solve(context.Background(), eng, Request{
			Character:   engine.Character{Level: 200},
			Constraints: []Constraint{{StatID: game.StatCritical, Op: "gte", Value: critical}},
			Maximize:    []Weight{{StatID: game.StatStrength, Weight: 1}},
			Forge:       forge,
		})
		if err != nil {
			t.Fatal(err)
		}
		return out.Solutions[0]
	}
	count := func(sol Solution, runeID int) int {
		n := 0
		for _, mods := range sol.Mods {
			if mods.Transcendence == runeID {
				n++
			}
		}
		return n
	}

	// Six items can be forged; the amulet and one ring already carry too
	// much Strength for a Strength rune. The boots cost 5 Critical.
	free := solve(-100, ForgeOptions{Transcendence: true})
	if !free.Feasible || count(free, 900) != 0 || count(free, 901) != 4 {
		t.Errorf("no need of Critical: %+v", free.Mods)
	}
	// Critical >= 3 takes four Critical runes, two of them on items that
	// would otherwise hold a Strength rune.
	tight := solve(3, ForgeOptions{Transcendence: true})
	if !tight.Feasible || count(tight, 900) != 4 || count(tight, 901) != 2 {
		t.Errorf("Critical >= 3: feasible %v, %d Critical runes, %d Strength runes", tight.Feasible, count(tight, 900), count(tight, 901))
	}
	// Forging the boots' malus up to -1 saves two of those runes: the boots
	// give up their own Strength rune and two other items get theirs back.
	raised := solve(3, ForgeOptions{Transcendence: true, RaiseCritical: true})
	boots := raised.Mods["boots"]
	if !raised.Feasible || boots == nil || boots.Lines[game.StatCritical] != -1 || count(raised, 900) != 2 || count(raised, 901) != 3 {
		t.Errorf("raised malus: feasible %v, boots %+v, %d Critical runes, %d Strength runes",
			raised.Feasible, boots, count(raised, 900), count(raised, 901))
	}
	res := eng.Evaluate(engine.Build{Character: raised.Character, Items: raised.Items, Mods: raised.Mods}, "")
	if !res.Valid {
		t.Errorf("issues %+v", res.Issues)
	}
	// The raised malus weighs 20 of the 101 an item's overs and exos share,
	// so the MP exo (90) goes on another item.
	withExo := solve(3, ForgeOptions{Transcendence: true, RaiseCritical: true, Exos: []int{game.StatMP}})
	exos := 0
	for position, mods := range withExo.Mods {
		exos += len(mods.Exos)
		if len(mods.Exos) > 0 && len(mods.Lines) > 0 {
			t.Errorf("%s carries the exo and the raised malus: %+v", position, mods)
		}
	}
	res = eng.Evaluate(engine.Build{Character: withExo.Character, Items: withExo.Items, Mods: withExo.Mods}, "")
	if !withExo.Feasible || exos != 1 || !res.Valid {
		t.Errorf("raised malus and MP exo: feasible %v, %d exos, issues %+v", withExo.Feasible, exos, res.Issues)
	}
}

func TestSolveMaximizesTheAverageHit(t *testing.T) {
	eng := engine.New(game.NewCatalog(forgeDataset()))
	goal := DamageGoal{Kind: "spell", ID: gametest.SpellEarth, Measure: MeasureMean}
	out, err := Solve(context.Background(), eng, Request{
		Character: engine.Character{Level: 200}, AutoPoints: true,
		Maximize: []Weight{{Damage: &goal}, {StatID: game.StatVitality, Weight: 1}},
		Forge:    ForgeOptions{Transcendence: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	best := out.Solutions[0]
	if !best.Feasible || len(best.Damage) != 1 || best.Character.Base["strength"] == 0 {
		t.Fatalf("average hit: %+v", best)
	}
	// What is reported is the engine's average on the stuff.
	res := eng.Evaluate(engine.Build{Character: best.Character, Items: best.Items, Mods: best.Mods}, "")
	spec, _ := engine.SpellSpec(eng.Catalog.SpellByID[gametest.SpellEarth], 200, engine.RangeAuto)
	if got := spec.Mean(func(id int) int { return res.Stats[id] }); got != best.Damage[0] {
		t.Errorf("reported %d, engine %d", best.Damage[0], got)
	}
	// Ranked after Vitality, the spell gives way: the points go to Vitality.
	second, err := Solve(context.Background(), eng, Request{
		Character: engine.Character{Level: 200}, AutoPoints: true,
		Maximize: []Weight{{StatID: game.StatVitality, Weight: 1}, {Damage: &goal}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if after := second.Solutions[0]; after.Character.Base["vitality"] <= best.Character.Base["vitality"] || after.Damage[0] >= best.Damage[0] {
		t.Errorf("Vitality first: base %v, damage %v (spell first: base %v, damage %v)",
			after.Character.Base, after.Damage, best.Character.Base, best.Damage)
	}
	// A goal that must reach a value needs one, and a maximized attack must exist.
	if _, err := Solve(context.Background(), eng, Request{
		Character: engine.Character{Level: 200}, Damage: []DamageGoal{{Kind: "spell", ID: gametest.SpellEarth}},
	}); !errors.Is(err, ErrBadDamageGoal) {
		t.Errorf("goal without a value: err = %v", err)
	}
	if _, err := Solve(context.Background(), eng, Request{
		Character: engine.Character{Level: 200}, Maximize: []Weight{{Damage: &DamageGoal{Kind: "spell", ID: 99999}}},
	}); !errors.Is(err, ErrBadDamageGoal) {
		t.Errorf("unknown spell to maximize: err = %v", err)
	}
}

func TestSolveMaximizesDamageInAnElement(t *testing.T) {
	eng := engine.New(game.NewCatalog(forgeDataset()))
	solve := func(element string, breedID int) *Response {
		t.Helper()
		out, err := Solve(context.Background(), eng, Request{
			Character: engine.Character{Level: 200}, AutoPoints: true, BreedID: breedID,
			Maximize: []Weight{{Damage: &DamageGoal{Kind: "element", Element: element, Measure: MeasureMean}}},
		})
		if err != nil {
			t.Fatal(err)
		}
		return out
	}

	// With a class, the goal reads the class's spells of the element: here
	// the one Earth spell, so it reports that spell's average hit.
	earth := solve("earth", gametest.BreedID).Solutions[0]
	res := eng.Evaluate(engine.Build{Character: earth.Character, Items: earth.Items, Mods: earth.Mods}, "")
	spec, _ := engine.SpellSpec(eng.Catalog.SpellByID[gametest.SpellEarth], 200, engine.RangeAuto)
	if want := spec.Mean(func(id int) int { return res.Stats[id] }); earth.Damage[0] != want || earth.Character.Base["strength"] == 0 {
		t.Errorf("earth with a class: damage %v, want %d, base %v", earth.Damage, want, earth.Character.Base)
	}
	// Without a class, or in an element the class has no spell for, a
	// reference hit stands in.
	if noClass := solve("earth", 0).Solutions[0]; noClass.Damage[0] <= 0 || noClass.Damage[0] == earth.Damage[0] {
		t.Errorf("earth without a class: %v", noClass.Damage)
	}
	if fire := solve("fire", gametest.BreedID).Solutions[0]; fire.Damage[0] <= 0 {
		t.Errorf("fire: %v", fire.Damage)
	}
	// Left open, the element is the one that hits hardest: the gear of the
	// dataset only gives Strength.
	open := solve("", gametest.BreedID)
	if best := open.Solutions[0]; best.Element != "earth" || best.Damage[0] != earth.Damage[0] {
		t.Errorf("open element: %q, damage %v, want earth %v", best.Element, best.Damage, earth.Damage)
	}
	if _, err := Solve(context.Background(), eng, Request{
		Character: engine.Character{Level: 200},
		Maximize:  []Weight{{Damage: &DamageGoal{Kind: "element", Element: "neutral"}}},
	}); !errors.Is(err, ErrBadDamageGoal) {
		t.Errorf("unknown element: err = %v", err)
	}
}

func TestSolveLeavesForbiddenItemsOut(t *testing.T) {
	eng := newEngine()
	// The Strength ring is the best pick, kept from the current stuff, and
	// forbidden: no result wears it. A required item wins over the ban.
	out, err := Solve(context.Background(), eng, Request{
		Character: engine.Character{Level: 200},
		Maximize:  []Weight{{StatID: game.StatStrength, Weight: 1}},
		Locked:    map[string]int{"ring1": 11},
		Excluded:  []int{11, 30},
		Required:  []int{30},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, sol := range out.Solutions {
		for position, id := range sol.Items {
			if id == 11 {
				t.Errorf("forbidden ring worn at %s", position)
			}
		}
		if sol.Items["amulet"] != 30 {
			t.Errorf("required amulet missing: %v", sol.Items)
		}
	}
}
