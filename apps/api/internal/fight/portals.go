package fight

import (
	"math"
	"slices"
	"sort"

	"dofusdb/api/internal/game"
)

func (st *State) cleanPortals() {
	st.Portals = slices.DeleteFunc(st.Portals, func(p Portal) bool { return st.entity(p.Caster) == nil })
	for i := range st.Portals {
		p := &st.Portals[i]
		if p.ClosedBy != 0 && st.entity(p.ClosedBy) == nil {
			p.ClosedBy, p.ClosedFor = 0, 0
		}
	}
}

func (s *Sim) resolveProjected(st *State, c *casting, id, entry int, a *attack, p portalProjection) {
	c.projection = p
	s.resolve(st, c, id, id, a, p.cell, 0)
	c.projection = portalProjection{}
	if len(p.route.Cells) > 1 {
		s.fire(st, c, id, id, 0, "CPT")
		s.rewardProjection(st, c, id, entry)
	}
}

func (s *Sim) rewardProjection(st *State, c *casting, id, entry int) {
	entity, portal := st.entity(id), st.portal(entry)
	if entity == nil || entity.Kind != KindPlayer || s.Breed != 16 || portal == nil {
		return
	}
	// The class's two Portal variants describe the post-projection gains
	// on their grade, including amount and duration. Other spells can lay
	// the same portal through a subsidiary spell without these rows.
	a := s.attack(portal.Spell, portal.Grade)
	if a == nil {
		return
	}
	if !slices.ContainsFunc(a.effects, func(e game.SpellEffect) bool { return e.Action == 2971 }) {
		for _, spell := range s.Cat.SpellsByBreed[s.Breed] {
			candidate := s.attack(spell.ID, 0)
			if candidate != nil && slices.ContainsFunc(candidate.effects, func(e game.SpellEffect) bool { return e.Action == 2971 && e.Described }) {
				a = candidate
				break
			}
		}
	}
	for _, e := range a.effects {
		if !e.Described || e.Duration <= 0 || (e.Action != 1171 && e.Action != 2971) {
			continue
		}
		kind, ref := BuffStat, statFinalDamage
		if e.Action == 2971 {
			kind, ref = buffFinalHeal, 0
		}
		count, oldest := 0, -1
		for i, b := range entity.Buffs {
			if b.Mod != 1181 || b.Kind != kind {
				continue
			}
			count++
			if oldest < 0 || b.Turns < entity.Buffs[oldest].Turns {
				oldest = i
			}
		}
		b := Buff{Kind: kind, Ref: ref, From: a.id, Spell: a.id, Mod: 1181, Turns: e.Duration, Value: e.Min}
		if count >= 10 {
			entity.Buffs[oldest] = b
		} else if len(entity.Buffs) < maxBuffs {
			entity.Buffs = append(entity.Buffs, b)
		} else {
			continue
		}
		event := Event{Type: EventBuff, Entity: id, Spell: a.id, Value: b.Value, Turns: b.Turns, Action: e.Action}
		if e.Action == 1171 {
			event.Stat = &ref
		}
		c.emit(event)
	}
}

func (s *Sim) projectedHeal(st *State, c *casting, at place, amount int) int {
	bonus := 0
	if caster := st.entity(at.caster); caster != nil {
		for _, b := range caster.Buffs {
			if b.Kind == buffFinalHeal {
				bonus += b.Value
			}
		}
	}
	return amount * max(0, 100+bonus) / 100 * (100 + c.projection.route.Bonus) / 100
}

const portalsPerSide = 4

// Portals are kept in placement order. Allies share their four slots.
type Portal struct {
	Cell      int  `json:"cell"`
	Caster    int  `json:"caster"`
	Spell     int  `json:"spell"`
	Grade     int  `json:"grade"`
	Flat      int  `json:"flat"`
	PerCell   int  `json:"perCell"`
	Used      bool `json:"used,omitempty"`
	ClosedBy  int  `json:"closedBy,omitempty"`
	ClosedFor int  `json:"closedFor,omitempty"`
}

type PortalRoute struct {
	Cells    []int `json:"cells"`
	Distance int   `json:"distance"`
	Bonus    int   `json:"bonus"`
}

type portalProjection struct {
	route PortalRoute
	cell  int
}

func (p portalProjection) origin(fallback int) int {
	if len(p.route.Cells) > 1 {
		return p.route.Cells[len(p.route.Cells)-1]
	}
	return fallback
}

func (st *State) portal(cell int) *Portal {
	for i := range st.Portals {
		if st.Portals[i].Cell == cell {
			return &st.Portals[i]
		}
	}
	return nil
}

// ValidPortals bounds client-held networks before resolving or previewing a turn.
func ValidPortals(st *State) bool {
	if st == nil || len(st.Portals) > 2*portalsPerSide {
		return false
	}
	seen := map[int]bool{}
	counts := map[bool]int{}
	for _, p := range st.Portals {
		owner := st.entity(p.Caster)
		if owner == nil || !st.Board.valid(p.Cell) || st.Board.wall(p.Cell) || seen[p.Cell] || p.Spell <= 0 || p.Grade < 1 ||
			p.Flat < 0 || p.Flat > 100 || p.PerCell < 0 || p.PerCell > 100 || p.ClosedFor < 0 || p.ClosedFor > 63 ||
			(p.ClosedBy != 0 && st.entity(p.ClosedBy) == nil) || (p.ClosedBy == 0) != (p.ClosedFor == 0) {
			return false
		}
		seen[p.Cell] = true
		counts[owner.enemyTeam()]++
		if counts[owner.enemyTeam()] > portalsPerSide {
			return false
		}
	}
	return true
}

// Equidistant gates use their circular order. When all of them lie on one
// side of the current portal, the edge before the empty half-circle wins.
// For opposite candidates the client's sweep starts along increasing map Y
// (the negative grid Y axis used by the combat board).
func (b Board) nextPortal(from int, candidates []int) int {
	near, best := []int{}, math.MaxInt
	for _, candidate := range candidates {
		d := b.Distance(from, candidate)
		if d < best {
			best, near = d, near[:0]
		}
		if d == best {
			near = append(near, candidate)
		}
	}
	if len(near) == 1 {
		return near[0]
	}
	x, y := b.xy(from)
	angle := func(cell int) float64 {
		xx, yy := b.xy(cell)
		return math.Mod(math.Atan2(float64(x-xx), float64(y-yy))+2*math.Pi, 2*math.Pi)
	}
	sort.Slice(near, func(i, j int) bool { return angle(near[i]) < angle(near[j]) })
	for i, cell := range near {
		next := near[(i+1)%len(near)]
		gap := math.Mod(angle(next)-angle(cell)+2*math.Pi, 2*math.Pi)
		if gap > math.Pi+1e-9 || len(near) > 2 && math.Abs(gap-math.Pi) < 1e-9 {
			return cell
		}
	}
	return near[0]
}

func (st *State) portalRoute(entry int, traveller int) PortalRoute {
	p := st.portal(entry)
	if p == nil || p.Used || p.ClosedFor > 0 {
		return PortalRoute{}
	}
	owner := st.entity(p.Caster)
	if owner == nil {
		return PortalRoute{}
	}
	if target := st.at(entry); target != nil && target.ID != traveller {
		return PortalRoute{}
	}
	var candidates []int
	for _, other := range st.Portals {
		by := st.entity(other.Caster)
		if other.Cell != entry && by != nil && by.enemyTeam() == owner.enemyTeam() && !other.Used && other.ClosedFor == 0 && st.at(other.Cell) == nil {
			candidates = append(candidates, other.Cell)
		}
	}
	if len(candidates) == 0 {
		return PortalRoute{}
	}
	route := PortalRoute{Cells: []int{entry}}
	for from := entry; len(candidates) > 0; {
		next := st.Board.nextPortal(from, candidates)
		route.Distance += st.Board.Distance(from, next)
		route.Cells = append(route.Cells, next)
		i := slices.Index(candidates, next)
		candidates = slices.Delete(candidates, i, i+1)
		from = next
	}
	route.Bonus = p.Flat + p.PerCell*route.Distance
	return route
}

// PortalRoutes reports the currently usable routes to the board. Hover and
// cast therefore use identical tie breaking without a second network solver.
func (st *State) PortalRoutes() map[int]PortalRoute {
	routes := map[int]PortalRoute{}
	for _, p := range st.Portals {
		if r := st.portalRoute(p.Cell, 0); len(r.Cells) > 1 {
			routes[p.Cell] = r
		}
	}
	return routes
}

func (st *State) project(a *attack, caster *Entity, entry int) portalProjection {
	result := portalProjection{cell: entry}
	if a.weapon || a.rules.NoPortal || a.rules.NoProjection || a.portalControl {
		return result
	}
	r := st.portalRoute(entry, 0)
	if len(r.Cells) < 2 {
		return result
	}
	x, y := st.Board.xy(caster.Cell)
	ix, iy := st.Board.xy(entry)
	ex, ey := st.Board.xy(r.Cells[len(r.Cells)-1])
	x, y = ex+ix-x, ey+iy-y
	result.route = r
	if !st.Board.inside(x, y) {
		result.cell = -1
	} else {
		result.cell = st.Board.cell(x, y)
	}
	return result
}

func (st *State) placePortal(at place, a *attack, e game.SpellEffect) {
	caster := st.entity(at.caster)
	if caster == nil || !st.Board.valid(at.cell) || st.Board.wall(at.cell) || st.portal(at.cell) != nil {
		return
	}
	count, first := 0, -1
	for i, p := range st.Portals {
		if owner := st.entity(p.Caster); owner != nil && owner.enemyTeam() == caster.enemyTeam() {
			if first < 0 {
				first = i
			}
			count++
		}
	}
	if count >= portalsPerSide {
		st.Portals = slices.Delete(st.Portals, first, first+1)
	}
	st.Portals = append(st.Portals, Portal{Cell: at.cell, Caster: caster.ID, Spell: a.id, Grade: a.grade, Flat: max(0, e.Value), PerCell: max(0, e.Min)})
}

func (st *State) startPortalTurn(id int) {
	for i := range st.Portals {
		p := &st.Portals[i]
		p.Used = false
		if p.ClosedBy == id {
			p.ClosedFor--
			if p.ClosedFor == 0 {
				p.ClosedBy = 0
			}
		}
	}
}

func (s *Sim) traversePortal(st *State, c *casting, id int, depth int) bool {
	traveller := st.entity(id)
	if traveller == nil || traveller.CarriedBy != 0 || traveller.HP <= 0 || depth >= maxDepth {
		return false
	}
	entry := st.portal(traveller.Cell)
	if entry == nil {
		return false
	}
	owner := st.entity(entry.Caster)
	if owner == nil || st.Turn == 1 && owner.enemyTeam() != traveller.enemyTeam() {
		return false
	}
	route := st.portalRoute(entry.Cell, id)
	if len(route.Cells) < 2 || c.spent() {
		return false
	}
	exit := st.portal(route.Cells[len(route.Cells)-1])
	origin := traveller.Cell
	entry.Used, exit.Used = true, true
	s.relocate(st, c, traveller, exit.Cell, depth)
	c.emit(Event{Type: EventMove, Entity: id, Action: 1182, Spell: entry.Spell, Path: []int{origin, exit.Cell}, PortalPath: route.Cells})
	s.fire(st, c, id, id, depth+1, "PT", "TP")
	s.crossMarks(st, c, id, origin, depth+1)
	return true
}

func portalAction(action int) bool {
	switch action {
	case 1181, 1182, 1183, 2183, 4007:
		return true
	}
	return false
}

func (s *Sim) applyPortal(st *State, c *casting, at place, a *attack, e game.SpellEffect, had map[int]map[int]bool) {
	caster := st.entity(at.caster)
	if caster == nil {
		return
	}
	occupant := st.at(at.cell)
	mask := readMask(e.Targets)
	for _, cond := range mask.conds {
		who := occupant
		if cond.onCaster {
			who = caster
		}
		if who == nil || !s.holds(cond, who, had) {
			return
		}
	}
	switch e.Action {
	case 1181:
		st.placePortal(at, a, e)
	case 1182:
		if occupant != nil {
			s.traversePortal(st, c, occupant.ID, at.depth)
		}
	case 1183, 2183, 4007:
		zone := st.Board.zone(e.Zone, caster.Cell, at.cell)
		if e.Action == 4007 {
			st.Portals = slices.DeleteFunc(st.Portals, func(p Portal) bool { return slices.Contains(zone, p.Cell) })
			return
		}
		for i := range st.Portals {
			p := &st.Portals[i]
			if !slices.Contains(zone, p.Cell) {
				continue
			}
			if e.Action == 1183 {
				p.ClosedBy, p.ClosedFor = caster.ID, max(1, min(63, e.Duration))
			} else {
				p.Used = false
				p.ClosedBy, p.ClosedFor = 0, 0
			}
		}
	}
}
