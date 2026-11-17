// Package gametest builds a small, hand-written dataset for tests.
package gametest

import "dofusdb/api/internal/game"

func texts(fr, en string) game.Texts { return game.Texts{"fr": fr, "en": en} }

const (
	EffectAP       = 111
	EffectStrength = 118
	EffectVitality = 125
	EffectWisdom   = 124
	EffectMinusStr = 157
)

// Dataset has two rings from one set, an amulet, a hat with a condition and
// two trophies.
// The class and spells of the dataset.
const (
	BreedID     = 9
	SpellEarth  = 500
	SpellCommon = 501
)

func Dataset() *game.Dataset {
	// Forgemagic weights as in the game: one AP weighs 100, one Vitality 0.2.
	weights := map[int]float64{
		game.StatAP: 100, game.StatMP: 90, game.StatStrength: 1, game.StatVitality: 0.2,
		game.StatWisdom: 3, game.StatInitiative: 0.1,
	}
	stat := func(id int, key, fr, en string) game.Stat {
		return game.Stat{ID: id, Key: key, CategoryID: 2, Order: id, Weight: weights[id], Label: texts(fr, en)}
	}
	effect := func(id, statID, sign int, fr string) game.EffectDef {
		return game.EffectDef{ID: id, Kind: game.EffectStat, StatID: statID, Sign: sign, Element: -1, Template: texts(fr, fr)}
	}
	line := func(effectID, statID, min, max int) game.ItemEffect {
		return game.ItemEffect{EffectID: effectID, Kind: game.EffectStat, StatID: statID, Min: min, Max: max}
	}
	item := func(id int, slot game.Slot, typeID, level, setID int, name string, effects ...game.ItemEffect) game.Item {
		return game.Item{
			ID: id, TypeID: typeID, Slot: slot, Level: level, IconID: id, SetID: setID,
			Name: texts(name, name), Description: game.Texts{}, Effects: effects,
		}
	}

	hat := item(40, game.SlotHat, 16, 100, 0, "Coiffe Limitée",
		line(EffectAP, game.StatAP, 1, 1), line(EffectStrength, game.StatStrength, 30, 50))
	hat.Criteria = "CP<8"

	return &game.Dataset{
		Version:    "test",
		Languages:  []string{"en", "fr"},
		Categories: []game.StatCategory{{ID: 2, Order: 1, Name: texts("Primaires", "Primary")}},
		Stats: []game.Stat{
			stat(game.StatHP, "hitPoints", "Points de vie", "Health points"),
			stat(game.StatAP, "actionPoints", "PA", "AP"),
			stat(game.StatStrength, "strength", "Force", "Strength"),
			stat(game.StatVitality, "vitality", "Vitalité", "Vitality"),
			stat(game.StatWisdom, "wisdom", "Sagesse", "Wisdom"),
			stat(game.StatMP, "movementPoints", "PM", "MP"),
			stat(game.StatInitiative, "initiative", "Initiative", "Initiative"),
		},
		Effects: []game.EffectDef{
			effect(EffectAP, game.StatAP, 1, "#1{{~1~2 à }}#2 PA"),
			effect(EffectStrength, game.StatStrength, 1, "#1{{~1~2 à }}#2 Force"),
			effect(EffectVitality, game.StatVitality, 1, "#1{{~1~2 à }}#2 Vitalité"),
			effect(EffectWisdom, game.StatWisdom, 1, "#1{{~1~2 à }}#2 Sagesse"),
			effect(EffectMinusStr, game.StatStrength, -1, "-#1{{~1~2 à -}}#2 Force"),
		},
		Types: []game.ItemType{
			{ID: 1, SuperTypeID: 1, Slot: game.SlotAmulet, Name: texts("Amulette", "Amulet")},
			{ID: 9, SuperTypeID: 3, Slot: game.SlotRing, Name: texts("Anneau", "Ring")},
			{ID: 16, SuperTypeID: 10, Slot: game.SlotHat, Name: texts("Chapeau", "Hat")},
			{ID: 151, SuperTypeID: 13, Slot: game.SlotDofus, Name: texts("Trophée", "Trophy")},
		},
		Items: []game.Item{
			item(10, game.SlotRing, 9, 60, 1, "Anneau du Set",
				line(EffectAP, game.StatAP, 1, 1)),
			item(11, game.SlotRing, 9, 80, 1, "Bague du Set",
				line(EffectStrength, game.StatStrength, 20, 40), line(EffectVitality, game.StatVitality, 100, 150)),
			item(12, game.SlotRing, 9, 190, 0, "Anneau Éloigné",
				line(EffectWisdom, game.StatWisdom, 30, 50), line(EffectMinusStr, game.StatStrength, -20, -10)),
			item(30, game.SlotAmulet, 1, 50, 1, "Amulette du Set",
				line(EffectStrength, game.StatStrength, 10, 30), line(EffectWisdom, game.StatWisdom, 10, 20)),
			hat,
			item(50, game.SlotDofus, 151, 100, 0, "Trophée Costaud",
				line(EffectStrength, game.StatStrength, 60, 60)),
			item(51, game.SlotDofus, 151, 100, 0, "Trophée Vital",
				line(EffectVitality, game.StatVitality, 200, 200)),
		},
		Sets: []game.Set{{
			ID: 1, Level: 80, Name: texts("Panoplie du Set", "The Set"), ItemIDs: []int{10, 11, 30},
			Bonuses: [][]game.SetBonus{
				nil, nil,
				{{EffectID: EffectStrength, StatID: game.StatStrength, Value: 25}},
				{{EffectID: EffectStrength, StatID: game.StatStrength, Value: 50}, {EffectID: EffectAP, StatID: game.StatAP, Value: 1}},
			},
		}},
		Breeds: []game.Breed{{ID: BreedID, Order: 2, Name: texts("Crâ", "Cra")}},
		Spells: []game.Spell{
			{
				ID: SpellEarth, IconID: 1, BreedID: BreedID, Order: 0, Name: texts("Flèche d'Essai", "Test Arrow"),
				Levels: []game.SpellLevel{
					{Grade: 1, MinLevel: 1, APCost: 3, RangeMin: 1, RangeMax: 6, CritChance: 10,
						Lines: []game.AttackLine{{Element: game.ElementEarth, Min: 10, Max: 12, CritMin: 14, CritMax: 16}}},
					{Grade: 2, MinLevel: 100, APCost: 3, RangeMin: 1, RangeMax: 8, CritChance: 10,
						Lines: []game.AttackLine{{Element: game.ElementEarth, Min: 20, Max: 30, CritMin: 30, CritMax: 40}}},
				},
			},
			{
				ID: SpellCommon, IconID: 2, Order: SpellCommon, Name: texts("Flamiche", "Flamiche"),
				Levels: []game.SpellLevel{
					{Grade: 1, MinLevel: 97, APCost: 2, RangeMin: 1, RangeMax: 6, CritChance: 5,
						Lines: []game.AttackLine{{Element: game.ElementBest, Min: 6, Max: 8, CritMin: 9, CritMax: 11}}},
				},
			},
		},
	}
}
