package engine

import "dofusdb/api/internal/game"

// This file computes what a weapon or a spell hits for with a stuff's final
// characteristics: per line, floor(base x (100 + characteristic + Power) /
// 100) + fixed damage, then the final multipliers (% weapon or spell damage,
// % melee or ranged damage). The target has no resistance.

// How the melee or ranged bonus of an attack is chosen.
const (
	// RangeAuto takes melee for an attack that only reaches the next cell.
	RangeAuto   = ""
	RangeMelee  = "melee"
	RangeRanged = "ranged"
)

// AttackSpec is an attack reduced to what the formula needs.
type AttackSpec struct {
	Lines  []game.AttackLine
	Weapon bool
	Melee  bool
	// CritChance is the attack's own chance of a critical hit, in percent,
	// before the character's Critical.
	CritChance int
	// Power is added to the character's Power for this attack only: the
	// weapon mastery.
	Power int
}

// WeaponMastery is the Power a weapon mastery adds to a weapon's hits.
const WeaponMastery = 300

// weaponElements are the elements a weapon's Neutral damage can be forged to.
var weaponElements = map[string]int{
	"earth": game.ElementEarth, "fire": game.ElementFire, "water": game.ElementWater, "air": game.ElementAir,
}

// ElementID gives the game's number of an element named earth, fire, water
// or air.
func ElementID(name string) (int, bool) {
	id, ok := weaponElements[name]
	return id, ok
}

// CanTakeElement reports whether an item is a weapon with Neutral damage that
// can be forged to the element.
func CanTakeElement(it *game.Item, element string) bool {
	if _, ok := weaponElements[element]; !ok || it == nil || it.Weapon == nil {
		return false
	}
	for _, e := range it.Effects {
		if e.Kind != game.EffectWeapon {
			continue
		}
		if el, _, ok := game.AttackAction(e.EffectID); ok && el == game.ElementNeutral {
			return true
		}
	}
	return false
}

// Hit is a damage range, without and with a critical hit.
type Hit struct {
	Min     int `json:"min"`
	Max     int `json:"max"`
	CritMin int `json:"critMin"`
	CritMax int `json:"critMax"`
}

// BestElement is the element of the highest elemental characteristic, the
// first one in the game's order when several are equal.
func BestElement(stat func(statID int) int) int {
	best := game.ElementEarth
	for _, el := range []int{game.ElementFire, game.ElementWater, game.ElementAir} {
		if stat(game.ElementStats[el].Stat) > stat(game.ElementStats[best].Stat) {
			best = el
		}
	}
	return best
}

func percent(value int) int {
	if value < -100 {
		return 0
	}
	return 100 + value
}

// terms is what the formula reads of the characteristics for one element:
// read once, it prices every roll of every line in that element.
type terms struct {
	multiplier int // 100 + characteristic + Power, never below zero
	fixed      int // Damage + the element's damage
	critical   int // Critical damage, added on a critical hit
	kind       int // 100 + % weapon or spell damage
	reach      int // 100 + % melee or ranged damage
}

func (spec *AttackSpec) terms(element int, stat func(statID int) int) terms {
	el := game.ElementStats[element]
	kind, reach := game.StatSpellDamage, game.StatRangedDamage
	if spec.Weapon {
		kind = game.StatWeaponDamage
	}
	if spec.Melee {
		reach = game.StatMeleeDamage
	}
	return terms{
		multiplier: percent(stat(el.Stat) + stat(game.StatPower) + spec.Power),
		fixed:      stat(game.StatDamage) + stat(el.Damage),
		critical:   stat(game.StatCritDamage),
		kind:       percent(stat(kind)),
		reach:      percent(stat(reach)),
	}
}

// hit applies the formula to one base value.
func (t terms) hit(base int, crit bool) int {
	damage := base*t.multiplier/100 + t.fixed
	if crit {
		damage += t.critical
	}
	if damage < 0 {
		return 0
	}
	return damage * t.kind / 100 * t.reach / 100
}

func (t terms) line(line game.AttackLine) Hit {
	hit := Hit{Min: t.hit(line.Min, false), Max: t.hit(line.Max, false)}
	if line.CritMax > 0 {
		hit.CritMin = t.hit(line.CritMin, true)
		hit.CritMax = t.hit(line.CritMax, true)
	}
	return hit
}

// Line computes one line. It also returns the element the line ends up in.
func (spec *AttackSpec) Line(line game.AttackLine, stat func(statID int) int) (Hit, int) {
	element := line.Element
	if element == game.ElementBest {
		element = BestElement(stat)
	}
	return spec.terms(element, stat).line(line), element
}

// Total sums the lines that hit when the attack is made, leaving out the
// delayed ones.
func (spec *AttackSpec) Total(stat func(statID int) int) Hit {
	var total Hit
	// The characteristics are read once per element, not once per roll: the
	// search calls this millions of times.
	var cached [5]terms
	var known [5]bool
	best := -1
	for _, line := range spec.Lines {
		if line.Delay > 0 {
			continue
		}
		element := line.Element
		if element == game.ElementBest {
			if best < 0 {
				best = BestElement(stat)
			}
			element = best
		}
		if element < 0 || element >= len(cached) {
			continue
		}
		if !known[element] {
			cached[element], known[element] = spec.terms(element, stat), true
		}
		hit := cached[element].line(line)
		total.Min += hit.Min
		total.Max += hit.Max
		total.CritMin += hit.CritMin
		total.CritMax += hit.CritMax
	}
	return total
}

// CritRate is the chance of a critical hit with a stuff, from 0 to 100: the
// attack's own chance plus the character's Critical. An attack with no
// critical line never crits.
func (spec *AttackSpec) CritRate(stat func(statID int) int) int {
	if spec.CritChance <= 0 {
		return 0
	}
	critical := false
	for _, line := range spec.Lines {
		critical = critical || (line.Delay == 0 && line.CritMax > 0)
	}
	if !critical {
		return 0
	}
	return max(0, min(100, spec.CritChance+stat(game.StatCritical)))
}

// Mean is what the attack hits for on average over many casts: the average
// roll of the normal and of the critical hit, weighted by the critical rate.
func (spec *AttackSpec) Mean(stat func(statID int) int) int {
	total := spec.Total(stat)
	rate := spec.CritRate(stat)
	normal := total.Min + total.Max
	critical := total.CritMin + total.CritMax
	return (normal*(100-rate) + critical*rate + 100) / 200
}

// WeaponLines returns the damage lines of a weapon; a critical hit adds the
// weapon's bonus to each of them. An element turns the Neutral lines into
// that element.
func WeaponLines(it *game.Item, element string) []game.AttackLine {
	if it == nil || it.Weapon == nil {
		return nil
	}
	forged, hasForged := weaponElements[element]
	var lines []game.AttackLine
	for _, e := range it.Effects {
		if e.Kind != game.EffectWeapon {
			continue
		}
		lineElement, steal, ok := game.AttackAction(e.EffectID)
		if !ok {
			continue
		}
		if hasForged && lineElement == game.ElementNeutral {
			lineElement = forged
		}
		line := game.AttackLine{Element: lineElement, Steal: steal, Min: e.Min, Max: e.Max}
		if it.Weapon.CritChance > 0 {
			line.CritMin, line.CritMax = e.Min+it.Weapon.CritBonus, e.Max+it.Weapon.CritBonus
		}
		lines = append(lines, line)
	}
	return lines
}

func melee(mode string, rangeMax int) bool {
	switch mode {
	case RangeMelee:
		return true
	case RangeRanged:
		return false
	}
	return rangeMax <= 1
}

// WeaponOptions says how a weapon is wielded.
type WeaponOptions struct {
	// Range is RangeAuto, RangeMelee or RangeRanged.
	Range string
	// Element is the weapon's elemental forgemagic, if any.
	Element string
	// Mastery adds the weapon mastery's Power.
	Mastery bool
}

// WeaponSpec builds the attack of a weapon, or nil when the item has no
// damage line.
func WeaponSpec(it *game.Item, opts WeaponOptions) *AttackSpec {
	lines := WeaponLines(it, opts.Element)
	if len(lines) == 0 {
		return nil
	}
	spec := &AttackSpec{
		Lines: lines, Weapon: true, Melee: melee(opts.Range, it.Weapon.RangeMax), CritChance: it.Weapon.CritChance,
	}
	if opts.Mastery {
		spec.Power = WeaponMastery
	}
	return spec
}

// SpellSpec builds the attack of a spell at a character level, or nil when
// the spell is not available at that level or deals no direct damage.
func SpellSpec(sp *game.Spell, level int, mode string) (*AttackSpec, *game.SpellLevel) {
	if sp == nil {
		return nil, nil
	}
	grade := sp.LevelAt(level)
	if grade == nil || len(grade.Lines) == 0 {
		return nil, grade
	}
	return &AttackSpec{Lines: grade.Lines, Melee: melee(mode, grade.RangeMax), CritChance: grade.CritChance}, grade
}

// AttackLineResult is one line of an attack with a stuff's characteristics.
type AttackLineResult struct {
	// Element is the element the line hits in; Best says it follows the
	// highest characteristic.
	Element int  `json:"element"`
	Best    bool `json:"best"`
	Steal   bool `json:"steal"`
	Delay   int  `json:"delay"`
	Hit
}

// Attack is a weapon or a spell with what it hits for.
type Attack struct {
	// ID is the item id of a weapon or the spell id.
	ID         int                `json:"id"`
	Grade      int                `json:"grade"`
	APCost     int                `json:"apCost"`
	RangeMin   int                `json:"rangeMin"`
	RangeMax   int                `json:"rangeMax"`
	CritChance int                `json:"critChance"`
	Melee      bool               `json:"melee"`
	Lines      []AttackLineResult `json:"lines"`
	// Total sums the lines that hit at once.
	Total Hit `json:"total"`
	// CritRate is the chance of a critical hit with this stuff, and Mean the
	// average hit over many casts at that rate.
	CritRate int `json:"critRate"`
	Mean     int `json:"mean"`
}

func describe(spec *AttackSpec, stat func(statID int) int) ([]AttackLineResult, Hit) {
	lines := make([]AttackLineResult, 0, len(spec.Lines))
	for _, line := range spec.Lines {
		hit, element := spec.Line(line, stat)
		lines = append(lines, AttackLineResult{
			Element: element, Best: line.Element == game.ElementBest, Steal: line.Steal, Delay: line.Delay, Hit: hit,
		})
	}
	return lines, spec.Total(stat)
}

// WeaponAttack computes what a weapon hits for, or nil for an item that
// deals no damage.
func WeaponAttack(it *game.Item, opts WeaponOptions, stat func(statID int) int) *Attack {
	spec := WeaponSpec(it, opts)
	if spec == nil {
		return nil
	}
	a := &Attack{
		ID: it.ID, APCost: it.Weapon.APCost, RangeMin: it.Weapon.RangeMin, RangeMax: it.Weapon.RangeMax,
		CritChance: it.Weapon.CritChance, Melee: spec.Melee,
	}
	a.Lines, a.Total = describe(spec, stat)
	a.CritRate, a.Mean = spec.CritRate(stat), spec.Mean(stat)
	return a
}

// SpellAttack computes what a spell hits for at a character level, or nil
// when the spell is not available or deals no direct damage.
func SpellAttack(sp *game.Spell, level int, mode string, stat func(statID int) int) *Attack {
	spec, grade := SpellSpec(sp, level, mode)
	if spec == nil {
		return nil
	}
	a := &Attack{
		ID: sp.ID, Grade: grade.Grade, APCost: grade.APCost, RangeMin: grade.RangeMin, RangeMax: grade.RangeMax,
		CritChance: grade.CritChance, Melee: spec.Melee,
	}
	a.Lines, a.Total = describe(spec, stat)
	a.CritRate, a.Mean = spec.CritRate(stat), spec.Mean(stat)
	return a
}
