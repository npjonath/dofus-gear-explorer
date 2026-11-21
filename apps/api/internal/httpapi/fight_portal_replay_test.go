package httpapi

import (
	"slices"
	"testing"

	"dofusdb/api/internal/fight"
)

// Golden ranges and placements recorded while playing the same level-150
// SPEEDRUSH build in Astrub. Casts still roll their normal combat dice.
func TestSpeedrushEliotropePortalsWithInstalledGameData(t *testing.T) {
	const room, portal, flame = 192419850, 14574, 24006
	r := newSpeedrushReplay(t, 16, room, 2, 1200)
	r.act(fightAction{Type: "place", Entity: 2, Cell: 124})
	r.act(fightAction{Type: "begin"})
	lay := func(cell int) { t.Helper(); r.act(fightAction{Type: "cast", Spell: portal, Cell: cell}) }
	lay(106)
	lay(134)
	check := func(minimum, maximum, critMin, critMax int) {
		t.Helper()
		for _, target := range r.aim(flame).Targets {
			if target.Cell != 106 {
				continue
			}
			want := fight.Loss{Entity: 2, Min: minimum, Max: maximum, CritMin: critMin, CritMax: critMax}
			if target.Projected == nil || *target.Projected != 124 || !slices.Equal(target.Losses, []fight.Loss{want}) {
				t.Fatalf("projected preview: %+v, want %+v on 124", target, want)
			}
			return
		}
		t.Fatal("no projection through cell 106")
	}
	if route := r.view.PortalRoutes[106]; route.Distance != 5 || route.Bonus != 10 {
		t.Fatalf("first network: %+v", route)
	}
	check(42, 57, 121, 134)
	r.act(fightAction{Type: "cast", Spell: flame, Cell: 106})
	check(42, 58, 123, 136)
	r.act(fightAction{Type: "end"})
	lay(117)
	lay(107)
	if route := r.view.PortalRoutes[107]; route.Distance != 8 || route.Bonus != 16 {
		t.Fatalf("four portals: %+v", route)
	}
	r.act(fightAction{Type: "end"})
	lay(127)
	var cells []int
	for _, p := range r.view.State.Portals {
		cells = append(cells, p.Cell)
	}
	if !slices.Equal(cells, []int{134, 117, 107, 127}) || r.view.PortalRoutes[127].Distance != 8 {
		t.Fatalf("fifth portal: %+v", r.view.State.Portals)
	}

	r = newSpeedrushReplay(t, 16, room, 2, 1200)
	r.act(fightAction{Type: "begin"})
	lay(106)
	lay(86)
	if r.view.PortalRoutes[106].Bonus != 4 {
		t.Fatalf("two-cell portal route=%+v", r.view.PortalRoutes[106])
	}
	r.act(fightAction{Type: "move", Cell: 106})
	if r.view.State.Player().Cell != 86 || r.view.MP != 4 {
		t.Fatalf("walk through portal: cell %d, MP %d", r.view.State.Player().Cell, r.view.MP)
	}
}
