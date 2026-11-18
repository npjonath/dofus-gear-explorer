// Package fight simulates a fight against training dummies: turns, action
// and movement points, range, line of sight, zones, and the effects of the
// spells as the game data lists them. It owns no state: every call takes the
// fight as it stands and returns what it became.
package fight

import "dofusdb/api/internal/game"

// Board is the arena: a grid of cells numbered row by row, some of them
// walls that block movement and sight.
type Board struct {
	Width  int   `json:"width"`
	Height int   `json:"height"`
	Walls  []int `json:"walls"`
}

// DefaultBoard is the arena a fight starts on: a square with four pillars
// to aim around.
func DefaultBoard() Board {
	b := Board{Width: 13, Height: 13}
	for _, at := range [][2]int{{4, 4}, {8, 4}, {4, 8}, {8, 8}} {
		b.Walls = append(b.Walls, b.cell(at[0], at[1]))
	}
	return b
}

func (b Board) cell(x, y int) int { return y*b.Width + x }

func (b Board) xy(cell int) (int, int) { return cell % b.Width, cell / b.Width }

func (b Board) inside(x, y int) bool { return x >= 0 && y >= 0 && x < b.Width && y < b.Height }

func (b Board) valid(cell int) bool { return cell >= 0 && cell < b.Width*b.Height }

func (b Board) wall(cell int) bool {
	for _, w := range b.Walls {
		if w == cell {
			return true
		}
	}
	return false
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func sign(v int) int {
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	}
	return 0
}

// Distance is the number of cells between two cells, as range counts them.
func (b Board) Distance(from, to int) int {
	x0, y0 := b.xy(from)
	x1, y1 := b.xy(to)
	return abs(x1-x0) + abs(y1-y0)
}

// between lists the cells a straight line crosses from one cell to another,
// both left out. A line through the exact corner of four cells passes
// between them without crossing any.
func (b Board) between(from, to int) []int {
	x0, y0 := b.xy(from)
	x1, y1 := b.xy(to)
	dx, dy := abs(x1-x0), abs(y1-y0)
	sx, sy := sign(x1-x0), sign(y1-y0)
	var cells []int
	x, y := x0, y0
	err := dx - dy
	dx, dy = dx*2, dy*2
	for x != x1 || y != y1 {
		switch {
		case err > 0:
			x += sx
			err -= dy
		case err < 0:
			y += sy
			err += dx
		default:
			x += sx
			y += sy
			err += dx - dy
		}
		if x != x1 || y != y1 {
			cells = append(cells, b.cell(x, y))
		}
	}
	return cells
}

// heading is the direction from one cell to another, as one step along each
// axis: straight when the cells are aligned or one axis clearly leads,
// diagonal when both differ by the same amount. Two equal cells head east.
func (b Board) heading(from, to int) (int, int) {
	x0, y0 := b.xy(from)
	x1, y1 := b.xy(to)
	dx, dy := x1-x0, y1-y0
	switch {
	case dx == 0 && dy == 0:
		return 1, 0
	case abs(dx) == abs(dy):
		return sign(dx), sign(dy)
	case abs(dx) > abs(dy):
		return sign(dx), 0
	}
	return 0, sign(dy)
}

// zone lists the cells a zone covers when a spell is aimed at target by a
// caster standing on caster, nearest to the centre first. Shapes the
// simulator does not know cover the aimed cell alone.
func (b Board) zone(z game.Zone, caster, target int) []int {
	tx, ty := b.xy(target)
	var cells []int
	add := func(x, y int) {
		if !b.inside(x, y) {
			return
		}
		c := b.cell(x, y)
		for _, known := range cells {
			if known == c {
				return
			}
		}
		cells = append(cells, c)
	}
	// around adds every cell at a distance from the centre the test accepts.
	around := func(reach int, keep func(dx, dy int) bool) {
		for d := 0; d <= reach*2; d++ {
			for dy := -reach; dy <= reach; dy++ {
				for dx := -reach; dx <= reach; dx++ {
					if abs(dx)+abs(dy) == d && keep(dx, dy) {
						add(tx+dx, ty+dy)
					}
				}
			}
		}
	}
	hx, hy := b.heading(caster, target)
	size, least := z.Size, z.Min
	switch z.Shape {
	case "C":
		around(size, func(dx, dy int) bool { d := abs(dx) + abs(dy); return d <= size && d >= least })
	case "O":
		around(size, func(dx, dy int) bool { return abs(dx)+abs(dy) == size })
	case "X", "Q":
		if z.Shape == "Q" && least < 1 {
			least = 1
		}
		around(size, func(dx, dy int) bool { d := abs(dx) + abs(dy); return (dx == 0 || dy == 0) && d <= size && d >= least })
	case "+", "#":
		if z.Shape == "#" && least < 1 {
			least = 1
		}
		around(size, func(dx, dy int) bool { return abs(dx) == abs(dy) && abs(dx) <= size && abs(dx) >= least })
	case "*":
		around(size, func(dx, dy int) bool {
			return (dx == 0 || dy == 0 || abs(dx) == abs(dy)) && max(abs(dx), abs(dy)) <= size
		})
	case "G":
		around(size, func(dx, dy int) bool { return max(abs(dx), abs(dy)) <= size })
	case "L", "l":
		for i := least; i <= size; i++ {
			add(tx+hx*i, ty+hy*i)
		}
	case "T":
		// A line across the direction of the cast.
		add(tx, ty)
		for i := 1; i <= size; i++ {
			add(tx-hy*i, ty+hx*i)
			add(tx+hy*i, ty-hx*i)
		}
	case "V":
		// A cone opening away from the caster.
		for i := 0; i <= size; i++ {
			for j := -i; j <= i; j++ {
				add(tx+hx*i-hy*j, ty+hy*i+hx*j)
			}
		}
	case "a", "A":
		around(b.Width+b.Height, func(dx, dy int) bool { return true })
	case "I":
		around(b.Width+b.Height, func(dx, dy int) bool { return abs(dx)+abs(dy) > size })
	default:
		add(tx, ty)
	}
	return cells
}
