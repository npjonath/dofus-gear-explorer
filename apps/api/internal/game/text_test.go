package game

import "testing"

func TestRenderEffect(t *testing.T) {
	cases := []struct {
		template string
		min, max int
		want     string
	}{
		{"#1{{~1~2 à }}#2 Vitalité", 251, 300, "251 à 300 Vitalité"},
		{"#1{{~1~2 à }}#2 PA", 1, 1, "1 PA"},
		{"-#1{{~1~2 à -}}#2 Force", -40, -11, "-11 à -40 Force"},
		{"#1{{~1~2 à }}#2 Dommage{{~ps}}{{~zs}} Feu", 1, 1, "1 Dommage Feu"},
		{"#1{{~1~2 à }}#2 Dommage{{~ps}}{{~zs}} Feu", 5, 7, "5 à 7 Dommages Feu"},
		{"#1{{~1~2 to }}#2% Air Resistance", 3, 6, "3 to 6% Air Resistance"},
	}
	for _, c := range cases {
		if got := RenderEffect(c.template, c.min, c.max); got != c.want {
			t.Errorf("RenderEffect(%q, %d, %d) = %q, want %q", c.template, c.min, c.max, got, c.want)
		}
	}
}

func TestStatLabel(t *testing.T) {
	cases := []struct{ template, label, unit string }{
		{"#1{{~1~2 à }}#2% Résistance Air", "Résistance Air", "%"},
		{"#1{{~1~2 à }}#2 Dommage{{~ps}}{{~zs}} Feu", "Dommages Feu", ""},
		{"#1{{~1~2 to }}#2 Fire Damage{{~p}}{{~z}}", "Fire Damage", ""},
		{"Change l'apparence", "", ""},
	}
	for _, c := range cases {
		label, unit := StatLabel(c.template)
		if label != c.label || unit != c.unit {
			t.Errorf("StatLabel(%q) = %q, %q; want %q, %q", c.template, label, unit, c.label, c.unit)
		}
	}
}

func TestFold(t *testing.T) {
	if got := Fold("  Épée d'Œuf Brûlâme "); got != "epee d'oeuf brulame" {
		t.Errorf("Fold = %q", got)
	}
}
