package solver

import (
	"context"
	"testing"

	"dofusdb/api/internal/engine"
	"dofusdb/api/internal/game"
)

func TestPointCostsFollowTheGameTiers(t *testing.T) {
	cases := []struct {
		kind              costKind
		owned, want, have int32
		got, paid         int32
	}{
		{costTiered, 0, 100, 995, 100, 100},
		{costTiered, 0, 200, 995, 200, 300},
		{costTiered, 0, 300, 995, 300, 600},
		// 600 points buy the first 300; the 395 left buy 98 more at 4 each.
		{costTiered, 0, 999, 995, 398, 992},
		{costTiered, 150, 100, 995, 100, 250},
		{costFlat1, 0, 500, 995, 500, 500},
		{costFlat3, 0, 500, 995, 331, 993},
		{costTiered, 300, 10, 3, 0, 0},
	}
	for _, c := range cases {
		got, paid := buy(c.kind, c.owned, c.want, c.have)
		if got != c.got || paid != c.paid {
			t.Errorf("buy(kind %d, owned %d, want %d, budget %d) = %d for %d; want %d for %d",
				c.kind, c.owned, c.want, c.have, got, paid, c.got, c.paid)
		}
		// The solver and the builder must price points the same way.
		if c.kind == costTiered && c.owned == 0 {
			if cost := engine.PointCost("strength", int(got)); cost != int(paid) {
				t.Errorf("engine prices %d Strength at %d, the solver at %d", got, cost, paid)
			}
		}
	}
}

func TestSolveSpendsCharacteristicPoints(t *testing.T) {
	eng := newEngine()
	req := Request{
		Character:  engine.Character{Level: 200, Base: map[string]int{"agility": 500}},
		Maximize:   []Weight{{StatID: game.StatStrength, Weight: 1}},
		AutoPoints: true,
	}
	resp, err := Solve(context.Background(), eng, req)
	if err != nil {
		t.Fatal(err)
	}
	best := resp.Solutions[0]
	// All 995 points go to Strength: 398 of it, and the 3 points that cannot
	// buy another one go to Vitality. The Agility asked for is ignored.
	if best.Character.Base["strength"] != 398 || best.Character.Base["vitality"] != 3 || best.Character.Base["agility"] != 0 {
		t.Errorf("points = %v", best.Character.Base)
	}
	res := eng.Evaluate(engine.Build{Character: best.Character, Items: best.Items}, "fr")
	if res.Points.Spent != 995 || !res.Valid {
		t.Errorf("spent %d of %d, valid %v, issues %+v", res.Points.Spent, res.Points.Total, res.Valid, res.Issues)
	}
	without, err := Solve(context.Background(), eng, Request{
		Character: engine.Character{Level: 200},
		Maximize:  []Weight{{StatID: game.StatStrength, Weight: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	plain := eng.Evaluate(engine.Build{Character: engine.Character{Level: 200}, Items: without.Solutions[0].Items}, "fr")
	if got, want := res.Stats[game.StatStrength], plain.Stats[game.StatStrength]+398; got != want {
		t.Errorf("Strength = %d, want %d (gear plus 398 from points)", got, want)
	}
}

func TestPointsPayForConstraintsFirst(t *testing.T) {
	eng := newEngine()
	// Gear alone cannot give 200 Wisdom: the points make up the difference,
	// at 3 for 1, and only what is left goes to Strength.
	resp, err := Solve(context.Background(), eng, Request{
		Character:   engine.Character{Level: 200, Scrolls: map[string]int{"wisdom": 100}},
		Constraints: []Constraint{{StatID: game.StatWisdom, Op: "gte", Value: 200}},
		Maximize:    []Weight{{StatID: game.StatStrength, Weight: 1}},
		AutoPoints:  true,
	})
	if err != nil {
		t.Fatal(err)
	}
	best := resp.Solutions[0]
	if !best.Feasible {
		t.Fatalf("200 Wisdom should be reachable with points: %+v", best)
	}
	res := eng.Evaluate(engine.Build{Character: best.Character, Items: best.Items}, "fr")
	if res.Stats[game.StatWisdom] < 200 {
		t.Errorf("Wisdom = %d", res.Stats[game.StatWisdom])
	}
	if best.Character.Base["wisdom"] == 0 || best.Character.Base["strength"] == 0 {
		t.Errorf("points should be split between Wisdom and Strength: %v", best.Character.Base)
	}
	if res.Points.Spent > res.Points.Total {
		t.Errorf("spent %d of %d", res.Points.Spent, res.Points.Total)
	}
	// Wisdom is not bought beyond what the constraint needs.
	if res.Stats[game.StatWisdom] > 202 {
		t.Errorf("Wisdom = %d: points were wasted past the constraint", res.Stats[game.StatWisdom])
	}
}
