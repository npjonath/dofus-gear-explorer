package game

import (
	"reflect"
	"testing"
)

func TestTargetsOfAMask(t *testing.T) {
	for mask, want := range map[string][]string{
		"a,A":         {"ally", "enemy"},
		"g,A":         {"allyOther", "enemy"},
		"h,m,d,H,M,D": {"allyPlayer", "allyMonster", "allyCompanion", "enemyPlayer", "enemyMonster", "enemyCompanion"},
		"J,j":         {"enemySummon", "allySummon"},
		"C":           {"caster"},
		// Conditions on the target, such as a state it must be in, are not targets.
		"a,A,*E244": {"ally", "enemy"},
		"A,e251":    {"enemy"},
		"":          nil,
	} {
		if got := Targets(mask); !reflect.DeepEqual(got, want) {
			t.Errorf("Targets(%q) = %v, want %v", mask, got, want)
		}
	}
	for mask, want := range map[string]bool{
		"a,A": true, "A": true, "l,m,L,M": true, "H": true, "": true, "e251": true,
		"a": false, "J,j": false, "C": false, "g": false,
	} {
		if got := TargetsEnemy(mask); got != want {
			t.Errorf("TargetsEnemy(%q) = %v, want %v", mask, got, want)
		}
	}
}

func TestWhatALineCountsFor(t *testing.T) {
	hit := AttackLine{Element: ElementFire, Min: 10, Max: 12, Targets: "a,A"}
	if !hit.Hits() {
		t.Error("damage on allies and enemies hits")
	}
	for name, change := range map[string]func(*AttackLine){
		"a heal":            func(l *AttackLine) { l.Kind = LineHeal },
		"a pushback":        func(l *AttackLine) { l.Kind = LinePush },
		"a delayed line":    func(l *AttackLine) { l.Delay = 2 },
		"a conditional one": func(l *AttackLine) { l.Conditional = true },
		"a line each turn":  func(l *AttackLine) { l.Trigger = TriggerTurnStart },
		"a line on summons": func(l *AttackLine) { l.Targets = "j,J" },
		"a line on allies":  func(l *AttackLine) { l.Targets = "a" },
	} {
		line := hit
		change(&line)
		if line.Hits() {
			t.Errorf("%s should not count in the total", name)
		}
	}
}
