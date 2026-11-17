package game

// Elements of an attack line, numbered as in the game data.
const (
	ElementNeutral = 0
	ElementEarth   = 1
	ElementFire    = 2
	ElementWater   = 3
	ElementAir     = 4
	// ElementBest is the element of the caster's highest characteristic.
	ElementBest = -1
)

// attackActions maps the game's damage and steal actions to their element.
var attackActions = map[int]struct {
	element int
	steal   bool
}{
	100: {ElementNeutral, false}, 97: {ElementEarth, false}, 99: {ElementFire, false},
	96: {ElementWater, false}, 98: {ElementAir, false}, 2822: {ElementBest, false},
	95: {ElementNeutral, true}, 92: {ElementEarth, true}, 94: {ElementFire, true},
	91: {ElementWater, true}, 93: {ElementAir, true}, 2828: {ElementBest, true},
}

// AttackAction reports whether an action deals damage, in which element, and
// whether it steals life.
func AttackAction(actionID int) (element int, steal, ok bool) {
	a, ok := attackActions[actionID]
	return a.element, a.steal, ok
}

// AttackLine is one damage line of a weapon or a spell, before the
// character's characteristics apply.
type AttackLine struct {
	Element int  `json:"element"`
	Steal   bool `json:"steal,omitempty"`
	Min     int  `json:"min"`
	Max     int  `json:"max"`
	// CritMin and CritMax are the line on a critical hit; both zero when the
	// attack cannot be critical.
	CritMin int `json:"critMin"`
	CritMax int `json:"critMax"`
	// Delay is the number of turns before the line applies.
	Delay int `json:"delay,omitempty"`
}

// SpellLevel is a spell at one of its grades.
type SpellLevel struct {
	Grade      int          `json:"grade"`
	MinLevel   int          `json:"minLevel"`
	APCost     int          `json:"apCost"`
	RangeMin   int          `json:"rangeMin"`
	RangeMax   int          `json:"rangeMax"`
	CritChance int          `json:"critChance"`
	Lines      []AttackLine `json:"lines"`
}

// Spell is a class spell or a common spell. Only its damage lines are kept.
type Spell struct {
	ID     int
	IconID int
	// BreedID is the class the spell belongs to; zero for a common spell.
	BreedID int
	// Order sorts the spells of a class as the game lists them, each variant
	// right after its spell.
	Order int
	// VariantOf is the spell this one replaces, or zero.
	VariantOf int
	Name      Texts
	Levels    []SpellLevel
}

// LevelAt returns the grade a character of that level casts, or nil when the
// spell is not available yet.
func (s *Spell) LevelAt(level int) *SpellLevel {
	var best *SpellLevel
	for i := range s.Levels {
		if l := &s.Levels[i]; l.MinLevel <= level && (best == nil || l.Grade > best.Grade) {
			best = l
		}
	}
	return best
}

// Breed is a character class.
type Breed struct {
	ID    int
	Order int
	Name  Texts
}

// Characteristics the damage formula reads, besides the four elemental ones
// and Power.
const (
	StatDamage        = 16
	StatDamageEarth   = 88
	StatDamageFire    = 89
	StatDamageWater   = 90
	StatDamageAir     = 91
	StatDamageNeutral = 92
	StatCritDamage    = 86
	// Final damage multipliers, in percent.
	StatRangedDamage = 120
	StatWeaponDamage = 122
	StatSpellDamage  = 123
	StatMeleeDamage  = 125
)

// ElementStats gives, per element, the characteristic that multiplies a line
// and the fixed damage added to it. Neutral damage scales with Strength.
var ElementStats = map[int]struct{ Stat, Damage int }{
	ElementNeutral: {StatStrength, StatDamageNeutral},
	ElementEarth:   {StatStrength, StatDamageEarth},
	ElementFire:    {StatIntel, StatDamageFire},
	ElementWater:   {StatChance, StatDamageWater},
	ElementAir:     {StatAgility, StatDamageAir},
}

// AttackStats lists every characteristic the damage formula reads.
var AttackStats = []int{
	StatStrength, StatIntel, StatChance, StatAgility, StatPower,
	StatDamage, StatDamageEarth, StatDamageFire, StatDamageWater, StatDamageAir, StatDamageNeutral,
	StatCritDamage, StatRangedDamage, StatWeaponDamage, StatSpellDamage, StatMeleeDamage,
}
