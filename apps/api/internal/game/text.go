package game

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

var foldTable = map[rune]string{
	'à': "a", 'á': "a", 'â': "a", 'ã': "a", 'ä': "a", 'å': "a",
	'ç': "c",
	'è': "e", 'é': "e", 'ê': "e", 'ë': "e",
	'ì': "i", 'í': "i", 'î': "i", 'ï': "i",
	'ñ': "n",
	'ò': "o", 'ó': "o", 'ô': "o", 'õ': "o", 'ö': "o",
	'ù': "u", 'ú': "u", 'û': "u", 'ü': "u",
	'ý': "y", 'ÿ': "y",
	'œ': "oe", 'æ': "ae", 'ß': "ss",
	'’': "'",
}

// Fold lowercases a string and strips diacritics, for accent-insensitive search.
func Fold(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range strings.ToLower(s) {
		if repl, ok := foldTable[r]; ok {
			b.WriteString(repl)
			continue
		}
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}

var (
	conditionalRe = regexp.MustCompile(`\{\{~((?:\d~?)+)([^}]*)\}\}`)
	inflectionRe  = regexp.MustCompile(`\{\{~([a-z])([^}]*)\}\}`)
	valuePrefixRe = regexp.MustCompile(`^-?#1\{\{~1~2[^}]*\}\}#2\s*(%?)\s*`)
	// The Spanish and Portuguese templates put a preposition between the
	// value and the name ("#2 de daño", "#2 al retiro de PA").
	leadingWordRe = regexp.MustCompile(`^(?:de|del|al|a la|a las|a los|a)\s+`)
	leftoverRe    = regexp.MustCompile(`\{\{[^}]*\}\}|#\d`)
	spacesRe      = regexp.MustCompile(`\s+`)
)

// RenderTemplate fills one of the game's effect description templates.
// params are the positional values (#1, #2, …); nil entries are absent.
func RenderTemplate(template string, params ...*int) string {
	has := func(n int) bool { return n >= 1 && n <= len(params) && params[n-1] != nil }

	out := conditionalRe.ReplaceAllStringFunc(template, func(m string) string {
		sub := conditionalRe.FindStringSubmatch(m)
		for _, d := range strings.Split(sub[1], "~") {
			if d == "" {
				continue
			}
			n, _ := strconv.Atoi(d)
			if !has(n) {
				return ""
			}
		}
		return sub[2]
	})

	// The quantity that drives plural forms is the largest value shown.
	// {{~p}} is shown for a plural, {{~s}} for a singular and {{~z}} for zero,
	// where a template has one (zero is otherwise singular).
	hasZero := strings.Contains(out, "{{~z")
	quantity := 0
	for _, p := range params {
		if p != nil && abs(*p) > quantity {
			quantity = abs(*p)
		}
	}
	out = inflectionRe.ReplaceAllStringFunc(out, func(m string) string {
		sub := inflectionRe.FindStringSubmatch(m)
		switch sub[1] {
		case "p":
			if quantity > 1 {
				return sub[2]
			}
		case "z":
			if quantity == 0 {
				return sub[2]
			}
		case "s":
			if quantity == 1 || (quantity == 0 && !hasZero) {
				return sub[2]
			}
		}
		return ""
	})

	for i, p := range params {
		if p != nil {
			out = strings.ReplaceAll(out, "#"+strconv.Itoa(i+1), strconv.Itoa(*p))
		}
	}
	out = leftoverRe.ReplaceAllString(out, "")
	return strings.TrimSpace(spacesRe.ReplaceAllString(out, " "))
}

// RenderEffect renders an item or set effect line from its signed range.
func RenderEffect(template string, min, max int) string {
	lo, hi := abs(min), abs(max)
	if lo > hi {
		lo, hi = hi, lo
	}
	if lo == hi {
		return RenderTemplate(template, &lo)
	}
	return RenderTemplate(template, &lo, &hi)
}

// StatLabel derives a stat's display label and unit from the template of the
// effect that grants it ("#1{{~1~2 à }}#2% Résistance Air" → "Résistance Air", "%").
func StatLabel(template string) (label, unit string) {
	m := valuePrefixRe.FindStringSubmatch(template)
	if m == nil {
		return "", ""
	}
	rest := template[len(m[0]):]
	plural := 2
	label = leadingWordRe.ReplaceAllString(RenderTemplate(rest, &plural), "")
	return label, m[1]
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
