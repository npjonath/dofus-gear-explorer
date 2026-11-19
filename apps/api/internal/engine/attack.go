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
	// Level is the caster's level, which pushback damage scales with.
	Level int
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

// WorstElement is the element of the lowest elemental characteristic, the
// first one in the game's order when several are equal.
func WorstElement(stat func(statID int) int) int {
	worst := game.ElementEarth
	for _, el := range []int{game.ElementFire, game.ElementWater, game.ElementAir} {
		if stat(game.ElementStats[el].Stat) < stat(game.ElementStats[worst].Stat) {
			worst = el
		}
	}
	return worst
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
func (t terms) scaled(base int, crit bool) int {
	damage := base*t.multiplier/100 + t.fixed
	if crit {
		damage += t.critical
	}
	if damage < 0 {
		return 0
	}
	return damage
}

func (t terms) hit(base int, crit bool) int {
	return t.scaled(base, crit) * t.kind / 100 * t.reach / 100
}

func (t terms) line(line game.AttackLine) Hit {
	hit := Hit{Min: t.hit(line.Min, false), Max: t.hit(line.Max, false)}
	if line.CritMax > 0 {
		hit.CritMin = t.hit(line.CritMin, true)
		hit.CritMax = t.hit(line.CritMax, true)
	}
	return hit
}

// CombatHit applies the weapon/spell and distance multipliers together, as the
// fight preview does. The static damage sheet rounds at each step.
func (spec *AttackSpec) CombatHit(base, element int, critical bool, stat func(int) int) (int, int) {
	element = resolvedElement(element, stat)
	t := spec.terms(element, stat)
	return t.scaled(base, critical) * t.kind * t.reach / 10000, element
}

func resolvedElement(element int, stat func(int) int) int {
	switch element {
	case game.ElementBest:
		return BestElement(stat)
	case game.ElementWorst:
		return WorstElement(stat)
	default:
		return element
	}
}

// Line computes one line. It also returns the element the line ends up in.
func (spec *AttackSpec) Line(line game.AttackLine, stat func(statID int) int) (Hit, int) {
	element := resolvedElement(line.Element, stat)
	switch line.Kind {
	case game.LineHeal:
		// A heal scales with the element's characteristic alone, then adds
		// the caster's Heals.
		multiplier, heals := percent(stat(game.ElementStats[element].Stat)), stat(game.StatHeal)
		heal := func(base int) int { return max(0, base*multiplier/100+heals) }
		hit := Hit{Min: heal(line.Min), Max: heal(line.Max)}
		if line.CritMax > 0 {
			hit.CritMin, hit.CritMax = heal(line.CritMin), heal(line.CritMax)
		}
		return hit, element
	case game.LineHealPercent:
		health := stat(game.StatHP)
		hit := Hit{Min: health * line.Min / 100, Max: health * line.Max / 100}
		if line.CritMax > 0 {
			hit.CritMin, hit.CritMax = health*line.CritMin/100, health*line.CritMax/100
		}
		return hit, element
	case game.LinePush:
		// Pushback damage grows with the caster's level and the cells pushed.
		push := func(cells int) int { return max(0, (spec.Level/2+32+stat(game.StatPushDamage))*cells/4) }
		hit := Hit{Min: push(line.Min), Max: push(line.Max)}
		if line.CritMax > 0 {
			hit.CritMin, hit.CritMax = push(line.CritMin), push(line.CritMax)
		}
		return hit, element
	}
	return spec.terms(element, stat).line(line), element
}

// Steps computes a line whose rolls grow with the MP the caster has used:
// one hit per MP, up to the character's own. It is empty for other lines.
func (spec *AttackSpec) Steps(line game.AttackLine, stat func(statID int) int) []Hit {
	if line.PerMP == 0 || line.Kind != "" {
		return nil
	}
	steps := make([]Hit, 0, max(0, stat(game.StatMP)))
	for used := 1; used <= stat(game.StatMP); used++ {
		grown := line
		grown.Min, grown.Max = line.Min+used*line.PerMP, line.Max+used*line.PerMP
		if line.CritMax > 0 {
			grown.CritMin, grown.CritMax = line.CritMin+used*line.PerMP, line.CritMax+used*line.PerMP
		}
		hit, _ := spec.Line(grown, stat)
		steps = append(steps, hit)
	}
	return steps
}

// Total sums the lines that hit an enemy when the attack is made. It leaves
// out the delayed and conditional ones, those kept for summons or allies,
// and what is not damage.
func (spec *AttackSpec) Total(stat func(statID int) int) Hit {
	var total Hit
	// The characteristics are read once per element, not once per roll: the
	// search calls this millions of times.
	var cached [5]terms
	var known [5]bool
	best := -1
	for _, line := range spec.Lines {
		if !line.Hits() {
			continue
		}
		element := line.Element
		if element == game.ElementBest {
			if best < 0 {
				best = BestElement(stat)
			}
			element = best
		}
		if element == game.ElementWorst {
			element = WorstElement(stat)
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
	return &AttackSpec{Lines: grade.Lines, Melee: melee(mode, grade.RangeMax), CritChance: grade.CritChance, Level: level}, grade
}

// AttackLineResult is one line of an attack with a stuff's characteristics.
type AttackLineResult struct {
	// Element is the element the line hits in; Best says it follows the
	// highest characteristic, Worst the lowest.
	Element int  `json:"element"`
	Best    bool `json:"best"`
	Worst   bool `json:"worst,omitempty"`
	Steal   bool `json:"steal"`
	Delay   int  `json:"delay"`
	// Kind is empty for damage, or "heal", "healPercent" or "push".
	Kind string `json:"kind,omitempty"`
	// Targets names who the line applies to; empty for a weapon.
	Targets []string `json:"targets"`
	// Conditional says the line applies later, when a condition is met, and
	// Trigger when it applies if not as the spell is cast.
	Conditional bool   `json:"conditional,omitempty"`
	Trigger     string `json:"trigger,omitempty"`
	// Counted says the line is part of the attack's total.
	Counted bool `json:"counted"`
	// Cells is how far a pushback pushes, on a normal and on a critical hit.
	Cells     int `json:"cells,omitempty"`
	CritCells int `json:"critCells,omitempty"`
	// Regen is what a steal gives back to the caster: half the damage.
	Regen *Hit `json:"regen,omitempty"`
	// Steps holds the line for each MP the caster has used, from one MP up
	// to the character's own, when its rolls grow with them.
	Steps []Hit `json:"steps,omitempty"`
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
		result := AttackLineResult{
			Element: element, Best: line.Element == game.ElementBest, Steal: line.Steal, Delay: line.Delay, Hit: hit,
			Worst: line.Element == game.ElementWorst,
			Kind:  line.Kind, Targets: game.Targets(line.Targets), Conditional: line.Conditional, Trigger: line.Trigger,
			Counted: line.Hits(), Steps: spec.Steps(line, stat),
		}
		if result.Targets == nil {
			result.Targets = []string{}
		}
		if line.Kind == game.LinePush {
			result.Cells, result.CritCells = line.Min, line.CritMin
		}
		if line.Steal && line.Kind == "" {
			result.Regen = &Hit{Min: hit.Min / 2, Max: hit.Max / 2, CritMin: hit.CritMin / 2, CritMax: hit.CritMax / 2}
		}
		lines = append(lines, result)
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
