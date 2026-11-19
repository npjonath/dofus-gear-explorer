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

// BreedTriggered files the spells no character casts directly: those another
// spell casts in turn. They belong to no class and are never listed.
const BreedTriggered = -1

// Actions through which a spell casts another one: the spell cast is the
// effect's Min and its grade the effect's Max. They differ in who casts and
// where: the target is who the effect applies to, the source who set the
// effect off (the caster, or for an effect that waits, whoever triggered it).
const (
	// ActionTargetCasts has the target cast the spell on its own cell.
	ActionTargetCasts = 792
	// ActionTargetExecutes and ActionTargetExecutesShown do the same.
	ActionTargetExecutes      = 1017
	ActionTargetExecutesShown = 1018
	// ActionTargetCastsOnSource has the target cast it on the source.
	ActionTargetCastsOnSource = 1019
	// ActionCastSpell has the caster cast the spell on the target's cell.
	ActionCastSpell = 1160
	// ActionSourceCasts has the source cast it on the target's cell, and
	// ActionSourceCastsOnSelf on its own cell.
	ActionSourceCasts       = 2160
	ActionSourceCastsOnSelf = 2794
	// ActionCastsOnCell has the caster cast it on the aimed cell, whether
	// anyone stands there or not.
	ActionCastsOnCell = 2960
)

// CastsSpell reports whether an action casts another spell.
func CastsSpell(action int) bool {
	switch action {
	case ActionTargetCasts, ActionTargetExecutes, ActionTargetExecutesShown, ActionTargetCastsOnSource,
		ActionCastSpell, ActionSourceCasts, ActionSourceCastsOnSelf, ActionCastsOnCell:
		return true
	}
	return false
}

// PlacesMark identifies effects whose first two arguments specify the payload
// spell and its grade. The importer retains it even when no breed casts it.
func PlacesMark(action int) bool {
	switch action {
	case 400, 401, 402, 1091, 1165:
		return true
	}
	return false
}

// Zone is the cells an effect covers around the cell a spell is aimed at.
type Zone struct {
	// Shape is the game's letter for the shape: "P" a single cell, "C" a
	// circle, "X" a cross, "L" a line, and so on.
	Shape string `json:"shape,omitempty"`
	// Size is how far the shape reaches and Min the distance it starts at.
	Size int `json:"size,omitempty"`
	Min  int `json:"min,omitempty"`
	// Decrease is the percentage of damage lost per cell away from the
	// centre of the zone, at most Steps times.
	Decrease int `json:"decrease,omitempty"`
	Steps    int `json:"steps,omitempty"`
}

// SpellEffect is one effect of a spell grade as the game data gives it,
// which the fight simulator applies.
type SpellEffect struct {
	// Action is the game's effect number; Min, Max and Value are its
	// parameters: a roll, a number of cells, a state, another spell.
	Action int `json:"action"`
	// Stat and Sign are set on an effect that raises (1) or lowers (-1) a
	// characteristic.
	Stat  int `json:"stat,omitempty"`
	Sign  int `json:"sign,omitempty"`
	Min   int `json:"min,omitempty"`
	Max   int `json:"max,omitempty"`
	Value int `json:"value,omitempty"`
	// Targets is the game's mask of who the effect applies to.
	Targets string `json:"targets,omitempty"`
	// Duration is the number of turns the effect lasts, zero for an effect
	// applied once, -1 for one that never ends.
	Duration int `json:"duration,omitempty"`
	Delay    int `json:"delay,omitempty"`
	// Trigger is empty for an effect applied as the spell is cast, or the
	// game's triggers otherwise: the effect waits on its target and applies
	// each time one of them happens, for TriggerTurns turns (63 and more is
	// for the whole fight).
	Trigger      string `json:"trigger,omitempty"`
	TriggerTurns int    `json:"triggerTurns,omitempty"`
	Zone         Zone   `json:"zone"`
	// Random is the weight of an effect drawn among those of its Group.
	Random float64 `json:"random,omitempty"`
	Group  int     `json:"group,omitempty"`
	// Shown says the game lists the effect in the spell's description, and
	// Described that the spell only describes it: something it sets up
	// applies it later.
	Shown     bool `json:"shown,omitempty"`
	Described bool `json:"described,omitempty"`
}

// CastRules is how a spell grade is aimed and how often it can be cast.
type CastRules struct {
	// Line and Diagonal restrict the cells it can be aimed at; with both,
	// either is allowed.
	Line     bool `json:"line,omitempty"`
	Diagonal bool `json:"diagonal,omitempty"`
	// Sight says the caster must see the cell.
	Sight     bool `json:"sight,omitempty"`
	FreeCell  bool `json:"freeCell,omitempty"`
	TakenCell bool `json:"takenCell,omitempty"`
	// BoostRange says the caster's Range adds to the spell's.
	BoostRange bool `json:"boostRange,omitempty"`
	// PerTurn and PerTarget cap the casts in a turn; zero is no cap.
	PerTurn   int `json:"perTurn,omitempty"`
	PerTarget int `json:"perTarget,omitempty"`
	// Interval is the number of turns between two casts.
	Interval        int `json:"interval,omitempty"`
	InitialCooldown int `json:"initialCooldown,omitempty"`
	// MaxStack caps how many times the spell's effects pile up on a target.
	MaxStack int `json:"maxStack,omitempty"`
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
	// Cast, Effects and CritEffects are what the fight simulator reads.
	Cast        CastRules     `json:"cast"`
	Effects     []SpellEffect `json:"effects,omitempty"`
	CritEffects []SpellEffect `json:"critEffects,omitempty"`
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
	// StartsFor is the class whose fights start with this spell cast on the
	// character: what sets its states and passive effects. Zero for others.
	StartsFor int
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

// SummonsCreature identifies direct creation of a creature on the aimed cell.
func SummonsCreature(action int) bool {
	switch action {
	case 181, 405, 1008, 1011, 2796:
		return true
	default:
		return false
	}
}
