package game

import "strings"

// Elements of an attack line, numbered as in the game data.
const (
	ElementNeutral = 0
	ElementEarth   = 1
	ElementFire    = 2
	ElementWater   = 3
	ElementAir     = 4
	// ElementBest is the element of the caster's highest characteristic, and
	// ElementWorst that of the lowest.
	ElementBest  = -1
	ElementWorst = -2
)

// attackActions maps the game's damage and steal actions to their element.
var attackActions = map[int]struct {
	element int
	steal   bool
}{
	100: {ElementNeutral, false}, 97: {ElementEarth, false}, 99: {ElementFire, false},
	96: {ElementWater, false}, 98: {ElementAir, false}, 2822: {ElementBest, false}, 2832: {ElementWorst, false},
	95: {ElementNeutral, true}, 92: {ElementEarth, true}, 94: {ElementFire, true},
	91: {ElementWater, true}, 93: {ElementAir, true}, 2828: {ElementBest, true},
}

// AttackAction reports whether an action deals damage, in which element, and
// whether it steals life.
func AttackAction(actionID int) (element int, steal, ok bool) {
	a, ok := attackActions[actionID]
	return a.element, a.steal, ok
}

// What a line of a spell does besides damage.
const (
	// LineHeal heals for a roll that scales with the element's
	// characteristic and the caster's Heals.
	LineHeal = "heal"
	// LineHealPercent heals for a percentage of the caster's health points:
	// Min and Max hold the percentage.
	LineHealPercent = "healPercent"
	// LinePush pushes back: Min and Max hold the number of cells, and the
	// line deals pushback damage.
	LinePush = "push"
)

// When a line applies, when it is not as the spell is cast.
const (
	TriggerTurnStart = "turnStart"
	TriggerTurnEnd   = "turnEnd"
)

// AttackLine is one line of a weapon or a spell, before the character's
// characteristics apply: damage or a steal, unless Kind says otherwise.
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
	// Kind is empty for damage, or one of the Line constants.
	Kind string `json:"kind,omitempty"`
	// Targets is the game's mask of who the line applies to, such as "a,A"
	// for allies and enemies; empty for a weapon, which hits whoever it is
	// aimed at.
	Targets string `json:"targets,omitempty"`
	// Conditional says the spell only describes the line: it applies later,
	// when a condition is met, such as a state the spell leaves on its
	// target being triggered.
	Conditional bool `json:"conditional,omitempty"`
	// Trigger is empty for a line applied as the spell is cast, or one of
	// the Trigger constants.
	Trigger string `json:"trigger,omitempty"`
	// PerMP is what the line's rolls gain for each MP the caster has used
	// this turn.
	PerMP int `json:"perMp,omitempty"`
}

// Hits reports whether the line is damage dealt to an enemy when the attack
// is made: what the total of an attack counts. A line kept for summons, or
// for allies, or dealt later, is shown but not added.
func (l AttackLine) Hits() bool {
	return l.Kind == "" && l.Delay == 0 && !l.Conditional && l.Trigger == "" && TargetsEnemy(l.Targets)
}

// targetNames names the plain letters of a target mask. Lower case is the
// caster's side, upper case the other one.
var targetNames = map[string]string{
	"a": "ally", "A": "enemy", "g": "allyOther", "C": "caster",
	"h": "allyPlayer", "H": "enemyPlayer", "m": "allyMonster", "M": "enemyMonster",
	"d": "allyCompanion", "D": "enemyCompanion", "l": "allyPlayerCompanion", "L": "enemyPlayerCompanion",
	"j": "allySummon", "J": "enemySummon", "i": "allySummon", "I": "enemySummon", "s": "allySummon", "S": "enemySummon",
}

// Targets names who a mask applies to, in the mask's order. The conditions a
// mask can also carry, such as a state the target must be in, are left out.
func Targets(mask string) []string {
	var out []string
	for _, token := range strings.Split(mask, ",") {
		name, ok := targetNames[token]
		if !ok {
			continue
		}
		known := false
		for _, other := range out {
			known = known || other == name
		}
		if !known {
			out = append(out, name)
		}
	}
	return out
}

// TargetsEnemy reports whether a mask covers an enemy that is not a summon:
// the opponent an attack is measured on. A mask that names nobody does.
func TargetsEnemy(mask string) bool {
	names := Targets(mask)
	for _, name := range names {
		switch name {
		case "enemy", "enemyPlayer", "enemyMonster", "enemyPlayerCompanion":
			return true
		}
	}
	return len(names) == 0
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

// Spell is a class spell or a common spell. Only the lines the damage panel
// computes are kept: damage, steals, heals and pushbacks.
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
