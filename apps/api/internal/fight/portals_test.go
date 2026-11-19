package fight

import (
	"slices"
	"testing"

	"dofusdb/api/internal/game"
)

func portalFixture() (*Sim, *State) {
	s := newSim(map[int]int{game.StatHP: 1000, game.StatMP: 5, game.StatAP: 12})
	st := s.StartOn(Dummies{Count: 2, HP: 1000}, Board{Width: 10, Height: 15})
	st.Entities[0].Cell, st.Entities[1].Cell, st.Entities[2].Cell = 116, 124, 52
	s.Begin(st)
	return s, st
}

func placePortalFixture(st *State, cell, owner int) {
	st.placePortal(place{cell: cell, caster: owner}, &attack{id: 30, grade: 1}, game.SpellEffect{Min: 2})
}

func TestPortalPathProjectionAndReplacement(t *testing.T) {
	_, st := portalFixture()
	for _, cell := range []int{106, 134} {
		placePortalFixture(st, cell, 1)
	}
	p := st.project(&attack{}, st.Player(), 106)
	if p.cell != 124 || p.route.Bonus != 10 || p.route.Distance != 5 {
		t.Fatalf("projected spell=%+v", p)
	}
	for _, cell := range []int{117, 107} {
		placePortalFixture(st, cell, 1)
	}
	if r := st.portalRoute(107, 0); r.Distance != 8 || r.Bonus != 16 {
		t.Fatalf("four portals=%+v", r)
	}
	placePortalFixture(st, 127, 1)
	if len(st.Portals) != 4 || st.Portals[0].Cell != 134 || st.Portals[3].Cell != 127 || st.portalRoute(127, 0).Distance != 8 {
		t.Fatal(st.Portals)
	}
	if !ValidPortals(st) {
		t.Fatal("normal game portals rejected")
	}
}

func TestPortalWalkingUsesOneMPAndCannotBeChainedAgainThatTurn(t *testing.T) {
	s, st := portalFixture()
	for _, cell := range []int{106, 86} {
		placePortalFixture(st, cell, 1)
	}
	original := st.clone()
	events, refused := s.Move(st, 106)
	_, mp := s.Points(st)
	if refused != "" || st.Player().Cell != 86 || mp != 4 || len(events) != 2 || !slices.Equal(events[1].PortalPath, []int{106, 86}) {
		t.Fatalf("crossing: %q, %d MP, %+v", refused, mp, events)
	}
	if len(st.PortalRoutes()) != 0 || len(original.PortalRoutes()) != 2 {
		t.Fatal("used portals or preview clone changed")
	}
	s.EndTurn(st)
	if st.portal(106).Used || len(st.PortalRoutes()) != 0 {
		t.Fatal("occupied exit should still be inactive")
	}
	s.Move(st, 96)
	if len(st.PortalRoutes()) != 2 {
		t.Fatal("portal failed to reopen after the fighter left")
	}
}

func TestPortalControlsAndTargetRestrictions(t *testing.T) {
	s, st := portalFixture()
	for _, cell := range []int{106, 134} {
		placePortalFixture(st, cell, 1)
	}
	for _, rules := range []game.CastRules{{NoProjection: true}, {NoPortal: true}} {
		if p := st.project(&attack{rules: rules}, st.Player(), 106); p.cell != 106 || len(p.route.Cells) != 0 {
			t.Fatalf("forbidden projection=%+v", p)
		}
	}
	c := s.casting(9, false, false)
	action := func(code int, zone string) {
		s.applyPortal(st, c, place{caster: 1, cell: 106}, &attack{id: 9}, game.SpellEffect{Action: code, Targets: "a,A", Duration: 1, Zone: game.Zone{Shape: zone}}, nil)
	}
	action(1183, "P")
	if len(st.PortalRoutes()) != 0 || st.portal(134).ClosedFor != 0 {
		t.Fatal("Neutral disabled wrong portals")
	}
	s.EndTurn(st)
	if len(st.PortalRoutes()) != 2 {
		t.Fatal("disabled portal did not return on caster's next turn")
	}
	action(1183, "A")
	if st.portal(134).ClosedFor != 1 {
		t.Fatal("Interruption did not disable the whole board")
	}
	action(2183, "A")
	if len(st.PortalRoutes()) != 2 {
		t.Fatal("action 2183 did not enable the network")
	}
	action(4007, "P")
	if st.portal(106) != nil || st.portal(134) == nil || len(st.PortalRoutes()) != 0 {
		t.Fatal("removing the entrance left a stale usable chain")
	}
}

func TestPortalsUseSeparateTeamsAndRespectOccupiedExits(t *testing.T) {
	_, st := portalFixture()
	for _, cell := range []int{106, 134} {
		placePortalFixture(st, cell, 1)
	}
	for _, cell := range []int{107, 117} {
		placePortalFixture(st, cell, 2)
	}
	if r := st.portalRoute(106, 0); !slices.Equal(r.Cells, []int{106, 134}) {
		t.Fatal("crossed enemy network", r)
	}
	st.Entities[2].Cell = 134
	if len(st.portalRoute(106, 0).Cells) > 0 {
		t.Fatal("used occupied exit")
	}
	st.Portals = append(st.Portals, st.Portals[0])
	if ValidPortals(st) {
		t.Fatal("duplicate portal cell accepted")
	}
}

func TestPortalChecksTheLandingCellAndTheOriginalSightLine(t *testing.T) {
	s, st := portalFixture()
	placePortalFixture(st, 106, 1)
	placePortalFixture(st, 134, 1)
	a := &attack{rangeMin: 1, rangeMax: 4, rules: game.CastRules{Sight: true, TakenCell: true}}
	if reason := s.aimable(st, a, st.Player(), 106); reason != "" {
		t.Fatalf("empty entrance leading to a real target: %s", reason)
	}
	a.rules.NoProjection = true
	if reason := s.aimable(st, a, st.Player(), 106); reason != BlockedCell {
		t.Fatalf("projection forbidden should inspect the actual aim cell: %q", reason)
	}
	a.rules.NoProjection = false
	st.Portals[1].Cell = 4
	if reason := s.aimable(st, a, st.Player(), 106); reason != BlockedCell {
		t.Fatalf("permitted out-of-board projection: %q", reason)
	}
}

func TestPortalMasksChooseEffectsAndDoNotApplyToOrdinaryCasts(t *testing.T) {
	s, st := portalFixture()
	effects := []game.SpellEffect{
		{Action: 97, Min: 50, Targets: "A,R", Zone: point()},
		{Action: 97, Min: 10, Targets: "A,r", Zone: point()},
	}
	sp := spell(9001, game.SpellLevel{RangeMax: 20, Effects: effects})
	s.Cat.SpellByID[sp.ID] = &sp
	placePortalFixture(st, 106, 1)
	placePortalFixture(st, 134, 1)
	projected, refused := s.Cast(st, sp.ID, 106)
	if refused != "" || lost(projected, 2) != 55 {
		t.Fatalf("projected conditional effects: %q %+v", refused, projected)
	}
	direct, refused := s.Cast(st, sp.ID, 124)
	if refused != "" || lost(direct, 2) != 10 {
		t.Fatalf("direct conditional effects: %q %+v", refused, direct)
	}
}
