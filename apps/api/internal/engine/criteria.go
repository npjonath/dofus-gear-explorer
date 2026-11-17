package engine

import (
	"strconv"
	"strings"

	"dofusdb/api/internal/game"
)

type nodeKind int

const (
	nodeAtom nodeKind = iota
	nodeAnd
	nodeOr
)

// Criteria is a parsed equip condition such as "CP<12|CM<6".
type Criteria struct {
	root *node
}

type node struct {
	kind     nodeKind
	children []*node

	key   string
	cmp   byte
	value int
	// subject says what the atom reads; subjectUnknown atoms cannot be checked.
	subject subject
	statID  int
}

type subject int

const (
	subjectUnknown subject = iota
	subjectTotal
	subjectBase
	subjectSetBonuses
)

var criterionStats = map[byte]int{
	's': game.StatStrength,
	'i': game.StatIntel,
	'c': game.StatChance,
	'a': game.StatAgility,
	'v': game.StatVitality,
	'w': game.StatWisdom,
	'p': game.StatAP,
	'm': game.StatMP,
}

// ParseCriteria parses the game's criterion syntax. It returns nil for an
// empty expression.
func ParseCriteria(expr string) *Criteria {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return nil
	}
	p := &criteriaParser{src: expr}
	root := p.parseOr()
	if root == nil {
		return nil
	}
	return &Criteria{root: root}
}

type criteriaParser struct {
	src string
	pos int
}

func (p *criteriaParser) peek() byte {
	if p.pos < len(p.src) {
		return p.src[p.pos]
	}
	return 0
}

func (p *criteriaParser) parseOr() *node {
	parts := []*node{p.parseAnd()}
	for p.peek() == '|' {
		p.pos++
		parts = append(parts, p.parseAnd())
	}
	return group(nodeOr, parts)
}

func (p *criteriaParser) parseAnd() *node {
	parts := []*node{p.parsePrimary()}
	for p.peek() == '&' {
		p.pos++
		parts = append(parts, p.parsePrimary())
	}
	return group(nodeAnd, parts)
}

func group(kind nodeKind, parts []*node) *node {
	var kept []*node
	for _, part := range parts {
		if part != nil {
			kept = append(kept, part)
		}
	}
	switch len(kept) {
	case 0:
		return nil
	case 1:
		return kept[0]
	}
	return &node{kind: kind, children: kept}
}

func (p *criteriaParser) parsePrimary() *node {
	if p.peek() == '(' {
		p.pos++
		inner := p.parseOr()
		if p.peek() == ')' {
			p.pos++
		}
		return inner
	}
	start := p.pos
	for p.pos < len(p.src) && !strings.ContainsRune("&|()", rune(p.src[p.pos])) {
		p.pos++
	}
	return parseAtom(p.src[start:p.pos])
}

func parseAtom(raw string) *node {
	if raw == "" {
		return nil
	}
	n := &node{kind: nodeAtom, key: raw}
	if len(raw) < 4 {
		return n
	}
	key, cmp, rest := raw[:2], raw[2], raw[3:]
	if !strings.ContainsRune("<>=!", rune(cmp)) {
		return n
	}
	value, err := strconv.Atoi(rest)
	if err != nil {
		return n
	}
	n.key, n.cmp, n.value = key, cmp, value

	switch {
	case key == "pk" || key == "Pk":
		n.subject = subjectSetBonuses
	case key[0] == 'C':
		lower := key[1] | 0x20
		statID, ok := criterionStats[lower]
		if !ok {
			return n
		}
		n.statID = statID
		n.subject = subjectTotal
		// A lowercase letter reads the character's own points, without gear.
		if key[1] == lower && lower != 'p' && lower != 'm' {
			n.subject = subjectBase
		}
	}
	return n
}

// CriteriaContext is what a condition is checked against.
type CriteriaContext struct {
	Catalog *game.Catalog
	// Totals are the final stats of the build; Base the character without gear.
	Totals     []int32
	Base       []int32
	SetBonuses int
}

// Check reports whether the condition holds. Conditions the engine cannot
// read (quests, alignment, …) are assumed to hold.
func (c *Criteria) Check(ctx *CriteriaContext) bool {
	if c == nil {
		return true
	}
	return c.root.check(ctx)
}

func (n *node) check(ctx *CriteriaContext) bool {
	switch n.kind {
	case nodeAnd:
		for _, child := range n.children {
			if !child.check(ctx) {
				return false
			}
		}
		return true
	case nodeOr:
		for _, child := range n.children {
			if child.check(ctx) {
				return true
			}
		}
		return false
	}

	var actual int
	switch n.subject {
	case subjectSetBonuses:
		actual = ctx.SetBonuses
	case subjectTotal, subjectBase:
		idx := ctx.Catalog.Idx(n.statID)
		if idx < 0 {
			return true
		}
		if n.subject == subjectTotal {
			actual = int(ctx.Totals[idx])
		} else {
			actual = int(ctx.Base[idx])
		}
	default:
		return true
	}
	switch n.cmp {
	case '<':
		return actual < n.value
	case '>':
		return actual > n.value
	case '=':
		return actual == n.value
	case '!':
		return actual != n.value
	}
	return true
}

// Checkable reports whether at least one part of the condition can be verified.
func (c *Criteria) Checkable() bool {
	if c == nil {
		return false
	}
	return c.root.checkable()
}

func (n *node) checkable() bool {
	if n.kind == nodeAtom {
		return n.subject != subjectUnknown
	}
	for _, child := range n.children {
		if child.checkable() {
			return true
		}
	}
	return false
}

var criteriaWords = map[string]map[string]string{
	"fr": {"and": " et ", "or": " ou ", "special": "Condition spéciale", "sets": "Bonus de panoplie", "base": " de base"},
	"en": {"and": " and ", "or": " or ", "special": "Special condition", "sets": "Set bonuses", "base": " (base)"},
	"es": {"and": " y ", "or": " o ", "special": "Condición especial", "sets": "Bonus de sets", "base": " base"},
	"pt": {"and": " e ", "or": " ou ", "special": "Condição especial", "sets": "Bônus de conjuntos", "base": " base"},
}

// Text renders the condition in a language, e.g. "PA < 12 ou PM < 6".
func (c *Criteria) Text(cat *game.Catalog, lang string) string {
	if c == nil {
		return ""
	}
	words, ok := criteriaWords[lang]
	if !ok {
		words = criteriaWords["en"]
	}
	return c.root.text(cat, lang, words, true)
}

func (n *node) text(cat *game.Catalog, lang string, words map[string]string, top bool) string {
	if n.kind == nodeAtom {
		var label string
		switch n.subject {
		case subjectSetBonuses:
			label = words["sets"]
		case subjectTotal, subjectBase:
			st := cat.StatByID[n.statID]
			if st == nil {
				return words["special"]
			}
			label = st.Label.Get(lang)
			if n.subject == subjectBase {
				label += words["base"]
			}
		default:
			return words["special"]
		}
		cmp := string(n.cmp)
		if n.cmp == '!' {
			cmp = "≠"
		}
		return label + " " + cmp + " " + strconv.Itoa(n.value)
	}
	joiner := words["and"]
	if n.kind == nodeOr {
		joiner = words["or"]
	}
	var parts []string
	seen := map[string]bool{}
	for _, child := range n.children {
		part := child.text(cat, lang, words, false)
		if !seen[part] {
			seen[part] = true
			parts = append(parts, part)
		}
	}
	out := strings.Join(parts, joiner)
	if !top && len(parts) > 1 {
		out = "(" + out + ")"
	}
	return out
}
