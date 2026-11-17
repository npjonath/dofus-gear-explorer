// Package game holds the domain model shared by the importer, the store,
// the build engine and the solver.
package game

import "sort"

// Slot is the kind of equipment slot an item goes into.
type Slot string

const (
	SlotAmulet Slot = "amulet"
	SlotWeapon Slot = "weapon"
	SlotRing   Slot = "ring"
	SlotBelt   Slot = "belt"
	SlotBoots  Slot = "boots"
	SlotShield Slot = "shield"
	SlotHat    Slot = "hat"
	SlotCloak  Slot = "cloak"
	SlotPet    Slot = "pet"
	SlotDofus  Slot = "dofus"
)

// SlotBySuperType maps the game's item super types to equipment slots.
// Super types that are not listed are not equipment (cosmetics, buffs, …).
var SlotBySuperType = map[int]Slot{
	1:  SlotAmulet,
	2:  SlotWeapon,
	3:  SlotRing,
	4:  SlotBelt,
	5:  SlotBoots,
	7:  SlotShield,
	10: SlotHat,
	11: SlotCloak,
	12: SlotPet,
	13: SlotDofus,
}

// Slots lists every slot kind in display order.
var Slots = []Slot{
	SlotAmulet, SlotHat, SlotRing, SlotWeapon, SlotShield,
	SlotBelt, SlotCloak, SlotBoots, SlotPet, SlotDofus,
}

// Position is one concrete place on a character where an item is worn.
type Position struct {
	Key  string `json:"key"`
	Slot Slot   `json:"slot"`
}

// Positions is the full equipment layout of a character: 16 places.
var Positions = []Position{
	{"amulet", SlotAmulet},
	{"hat", SlotHat},
	{"ring1", SlotRing},
	{"ring2", SlotRing},
	{"weapon", SlotWeapon},
	{"shield", SlotShield},
	{"belt", SlotBelt},
	{"cloak", SlotCloak},
	{"boots", SlotBoots},
	{"pet", SlotPet},
	{"dofus1", SlotDofus},
	{"dofus2", SlotDofus},
	{"dofus3", SlotDofus},
	{"dofus4", SlotDofus},
	{"dofus5", SlotDofus},
	{"dofus6", SlotDofus},
}

// TypePrysmaradite is the item type of prysmaradites. They share the Dofus
// slot kind but only go in the first Dofus position, so a character wears
// at most one.
const (
	TypePrysmaradite     = 217
	PrysmaraditePosition = "dofus1"
)

// Fits reports whether an item may be worn at a position.
func Fits(it *Item, pos Position) bool {
	if it.Slot != pos.Slot {
		return false
	}
	return it.TypeID != TypePrysmaradite || pos.Key == PrysmaraditePosition
}

// PositionIndex returns the index of a position key in Positions, or -1.
func PositionIndex(key string) int {
	for i, p := range Positions {
		if p.Key == key {
			return i
		}
	}
	return -1
}

// Characteristic ids used by the engine. They are the game's own ids.
const (
	StatHP          = 0
	StatAP          = 1
	StatStrength    = 10
	StatVitality    = 11
	StatWisdom      = 12
	StatChance      = 13
	StatAgility     = 14
	StatIntel       = 15
	StatCritical    = 18
	StatRange       = 19
	StatMP          = 23
	StatPower       = 25
	StatSummons     = 26
	StatDodgeAP     = 27
	StatDodgeMP     = 28
	StatPods        = 40
	StatInitiative  = 44
	StatProspecting = 48
	StatHeal        = 49
	StatEscape      = 78
	StatTackle      = 79
	StatAPReduction = 82
	StatMPReduction = 83
)

// Texts holds one string per language code.
type Texts map[string]string

// Get returns the text for lang, falling back to any available language.
func (t Texts) Get(lang string) string {
	if s, ok := t[lang]; ok {
		return s
	}
	for _, fallback := range []string{"en", "fr"} {
		if s, ok := t[fallback]; ok {
			return s
		}
	}
	return ""
}

type StatCategory struct {
	ID    int
	Order int
	Name  Texts
}

// Stat is a character characteristic that equipment can modify.
type Stat struct {
	ID         int
	Key        string
	CategoryID int
	Order      int
	Unit       string
	// Weight is the forgemagic weight of one point of the stat, from the
	// game data. Zero means the stat cannot be forged.
	Weight float64
	Label  Texts
}

// ForgeLineWeight is the most weight one forged line may carry.
const ForgeLineWeight = 101

// ForgeMax is the highest value forgemagic can give a line of this stat:
// the "101 of weight" rule. Zero when the stat cannot be forged.
func (s *Stat) ForgeMax() int {
	if s.Weight <= 0 {
		return 0
	}
	return int(ForgeLineWeight/s.Weight + 1e-9)
}

// Passive is the in-fight effect of a legendary item or a Dofus.
type Passive struct {
	Name        Texts
	Description Texts
}

// Rune is a transcendence rune: one fixed bonus that seals the item.
type Rune struct {
	ID       int
	Level    int
	EffectID int
	StatID   int
	Value    int
	// Density is the forgemagic weight the rune counts for on its line, from
	// the game data: 40, 60 or 80.
	Density int
	Name    Texts
}

// LineWeight is the forgemagic weight of a line holding value points of the
// stat. Negative lines weigh nothing.
func (s *Stat) LineWeight(value int) float64 {
	if value <= 0 || s.Weight <= 0 {
		return 0
	}
	return float64(value) * s.Weight
}

type EffectKind string

const (
	// EffectStat modifies a characteristic while the item is worn.
	EffectStat EffectKind = "stat"
	// EffectWeapon is a line of a weapon's attack (damage, steal, heal…).
	EffectWeapon EffectKind = "weapon"
)

// EffectDef describes a game effect referenced by items and set bonuses.
type EffectDef struct {
	ID       int
	Kind     EffectKind
	StatID   int
	Sign     int
	Element  int
	Template Texts
}

type ItemType struct {
	ID          int
	SuperTypeID int
	Slot        Slot
	Name        Texts
}

// ItemEffect is one line of an item. Min and Max are signed and Min <= Max,
// so Max is always the best possible roll.
type ItemEffect struct {
	EffectID int
	Kind     EffectKind
	StatID   int
	Min      int
	Max      int
}

type WeaponInfo struct {
	APCost         int `json:"apCost"`
	RangeMin       int `json:"rangeMin"`
	RangeMax       int `json:"rangeMax"`
	CritChance     int `json:"critChance"`
	CritBonus      int `json:"critBonus"`
	MaxCastPerTurn int `json:"maxCastPerTurn"`
}

type Item struct {
	ID          int
	TypeID      int
	Slot        Slot
	Level       int
	IconID      int
	SetID       int
	Criteria    string
	Pods        int
	Weapon      *WeaponInfo
	Passive     *Passive
	Name        Texts
	Description Texts
	Effects     []ItemEffect
}

type SetBonus struct {
	EffectID int
	StatID   int
	Value    int
}

// Set is an item set. Bonuses[n] is the bonus granted with n items worn.
type Set struct {
	ID      int
	Level   int
	Name    Texts
	ItemIDs []int
	Bonuses [][]SetBonus
}

// Dataset is everything imported from one game data release.
type Dataset struct {
	Version    string
	Languages  []string
	Categories []StatCategory
	Stats      []Stat
	Effects    []EffectDef
	Types      []ItemType
	Items      []Item
	Sets       []Set
	Runes      []Rune
	Breeds     []Breed
	Spells     []Spell
}

// StatValue is a sparse entry of a stat vector.
type StatValue struct {
	Index int32
	Value int32
}

// Catalog is a Dataset indexed for fast lookups by the engine and the solver.
type Catalog struct {
	*Dataset

	ItemByID   map[int]*Item
	SetByID    map[int]*Set
	TypeByID   map[int]*ItemType
	StatByID   map[int]*Stat
	EffectByID map[int]*EffectDef
	RuneByID   map[int]*Rune
	BreedByID  map[int]*Breed
	SpellByID  map[int]*Spell
	// SpellsByBreed lists the spells of each class in game order; key zero
	// holds the common spells.
	SpellsByBreed map[int][]*Spell

	// StatIndex maps a stat id to its position in dense stat vectors.
	StatIndex map[int]int32
	// StatIDs is the inverse of StatIndex.
	StatIDs []int

	// ItemVector holds each item's best-roll stats as a sparse vector.
	ItemVector map[int][]StatValue
	// SetVectors holds, per set, the sparse bonus vector for each item count.
	SetVectors map[int][][]StatValue
	// SetOrdinal gives each set a dense index, for per-set counters.
	SetOrdinal map[int]int
	// ItemsBySlot lists items per slot kind, sorted by level then id.
	ItemsBySlot map[Slot][]*Item
}

// NewCatalog indexes a dataset.
func NewCatalog(ds *Dataset) *Catalog {
	c := &Catalog{
		Dataset:       ds,
		ItemByID:      make(map[int]*Item, len(ds.Items)),
		SetByID:       make(map[int]*Set, len(ds.Sets)),
		TypeByID:      make(map[int]*ItemType, len(ds.Types)),
		StatByID:      make(map[int]*Stat, len(ds.Stats)),
		EffectByID:    make(map[int]*EffectDef, len(ds.Effects)),
		RuneByID:      make(map[int]*Rune, len(ds.Runes)),
		BreedByID:     make(map[int]*Breed, len(ds.Breeds)),
		SpellByID:     make(map[int]*Spell, len(ds.Spells)),
		SpellsByBreed: map[int][]*Spell{},
		StatIndex:     make(map[int]int32, len(ds.Stats)),
		ItemVector:    make(map[int][]StatValue, len(ds.Items)),
		SetVectors:    make(map[int][][]StatValue, len(ds.Sets)),
		SetOrdinal:    make(map[int]int, len(ds.Sets)),
		ItemsBySlot:   make(map[Slot][]*Item),
	}
	for i := range ds.Stats {
		s := &ds.Stats[i]
		c.StatByID[s.ID] = s
		c.StatIndex[s.ID] = int32(i)
		c.StatIDs = append(c.StatIDs, s.ID)
	}
	for i := range ds.Effects {
		c.EffectByID[ds.Effects[i].ID] = &ds.Effects[i]
	}
	for i := range ds.Types {
		c.TypeByID[ds.Types[i].ID] = &ds.Types[i]
	}
	for i := range ds.Runes {
		c.RuneByID[ds.Runes[i].ID] = &ds.Runes[i]
	}
	for i := range ds.Breeds {
		c.BreedByID[ds.Breeds[i].ID] = &ds.Breeds[i]
	}
	for i := range ds.Spells {
		sp := &ds.Spells[i]
		c.SpellByID[sp.ID] = sp
		c.SpellsByBreed[sp.BreedID] = append(c.SpellsByBreed[sp.BreedID], sp)
	}
	for _, list := range c.SpellsByBreed {
		sort.SliceStable(list, func(a, b int) bool { return list[a].Order < list[b].Order })
	}
	for i := range ds.Items {
		it := &ds.Items[i]
		c.ItemByID[it.ID] = it
		c.ItemsBySlot[it.Slot] = append(c.ItemsBySlot[it.Slot], it)
		c.ItemVector[it.ID] = c.sparse(func(add func(statID, value int)) {
			for _, e := range it.Effects {
				if e.Kind == EffectStat {
					add(e.StatID, e.Max)
				}
			}
		})
	}
	for slot := range c.ItemsBySlot {
		items := c.ItemsBySlot[slot]
		sort.Slice(items, func(a, b int) bool {
			if items[a].Level != items[b].Level {
				return items[a].Level < items[b].Level
			}
			return items[a].ID < items[b].ID
		})
	}
	for i := range ds.Sets {
		s := &ds.Sets[i]
		c.SetByID[s.ID] = s
		c.SetOrdinal[s.ID] = i
		vectors := make([][]StatValue, len(s.Bonuses))
		for n, bonuses := range s.Bonuses {
			vectors[n] = c.sparse(func(add func(statID, value int)) {
				for _, b := range bonuses {
					if b.StatID >= 0 {
						add(b.StatID, b.Value)
					}
				}
			})
		}
		c.SetVectors[s.ID] = vectors
	}
	return c
}

func (c *Catalog) sparse(fill func(add func(statID, value int))) []StatValue {
	acc := map[int32]int32{}
	var order []int32
	fill(func(statID, value int) {
		idx, ok := c.StatIndex[statID]
		if !ok {
			return
		}
		if _, seen := acc[idx]; !seen {
			order = append(order, idx)
		}
		acc[idx] += int32(value)
	})
	out := make([]StatValue, 0, len(order))
	for _, idx := range order {
		out = append(out, StatValue{Index: idx, Value: acc[idx]})
	}
	return out
}

// Idx returns the dense index of a stat id, or -1 when the stat is unknown.
func (c *Catalog) Idx(statID int) int32 {
	if idx, ok := c.StatIndex[statID]; ok {
		return idx
	}
	return -1
}

// SetBonusVector returns the bonus vector of a set for a number of worn items.
func (c *Catalog) SetBonusVector(setID, count int) []StatValue {
	vectors := c.SetVectors[setID]
	if count < 0 || len(vectors) == 0 {
		return nil
	}
	if count >= len(vectors) {
		count = len(vectors) - 1
	}
	return vectors[count]
}
