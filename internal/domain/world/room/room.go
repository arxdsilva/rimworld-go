package room

// Outdoors is the region id of tiles that reach the map edge.
const Outdoors = -1

// Wall is the region id of boundary tiles (walls, doors, rock).
const Wall = -2

// Detect flood-fills open tiles into regions. boundary reports tiles that
// separate rooms (walls, doors, natural rock). Regions that touch the map
// edge are outdoors. Returns a per-tile region grid (row-major) and the
// cell count of each room id 0..n-1. Deterministic: scan order is fixed.
func Detect(w, h int, boundary func(x, y int) bool) (grid []int, sizes []int) {
	grid = make([]int, w*h)
	for i := range grid {
		grid[i] = Outdoors - 10 // unvisited
	}
	stack := make([]int, 0, 64)
	for start := 0; start < w*h; start++ {
		if grid[start] != Outdoors-10 {
			continue
		}
		sx, sy := start%w, start/w
		if boundary(sx, sy) {
			grid[start] = Wall
			continue
		}
		region := make([]int, 0, 32)
		edge := false
		stack = append(stack[:0], start)
		grid[start] = Outdoors - 11 // queued
		for len(stack) > 0 {
			i := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			region = append(region, i)
			x, y := i%w, i/w
			if x == 0 || y == 0 || x == w-1 || y == h-1 {
				edge = true
			}
			for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
				nx, ny := x+d[0], y+d[1]
				if nx < 0 || ny < 0 || nx >= w || ny >= h {
					continue
				}
				j := ny*w + nx
				if grid[j] != Outdoors-10 {
					continue
				}
				if boundary(nx, ny) {
					grid[j] = Wall
					continue
				}
				grid[j] = Outdoors - 11
				stack = append(stack, j)
			}
		}
		id := Outdoors
		if !edge {
			id = len(sizes)
			sizes = append(sizes, len(region))
		}
		for _, i := range region {
			grid[i] = id
		}
	}
	return grid, sizes
}

// Contents is what a room contains, for scoring.
type Contents struct {
	Cells, Floored         int
	Beds, Tables, Chairs   int
	Lamps, Benches         int
	StoneWalls, TotalWalls int
}

// Role is what a room is used for.
type Role uint8

const (
	General Role = iota
	Bedroom
	Dining
	Workshop
	Barracks
	// Roles recognised by RoleOfFull only (RoleOf never returns them).
	Hospital
	PrisonCell
	PrisonBarracks
	RecRoom
	Kitchen
)

func (r Role) String() string {
	return [...]string{"room", "bedroom", "dining room", "workshop", "barracks", "hospital", "prison cell", "prison barracks", "recreation room", "kitchen"}[r]
}

// RoleOf picks the dominant use.
func RoleOf(c Contents) Role {
	switch {
	case c.Beds >= 2 && c.Benches == 0:
		return Barracks
	case c.Beds == 1 && c.Benches == 0:
		return Bedroom
	case c.Benches > 0:
		return Workshop
	case c.Tables > 0:
		return Dining
	}
	return General
}

// Impressiveness is an integer room score; higher is better.
func Impressiveness(c Contents) int {
	if c.Cells <= 0 {
		return 0
	}
	s := 12 * c.Floored / c.Cells
	s += 3 * min(c.Lamps, 2)
	s += 2 * min(c.Tables, 2)
	s += min(c.Chairs, 4)
	switch {
	case c.Cells < 6:
		s -= 6 // cramped
	case c.Cells < 12:
		s -= 4 // small
	case c.Cells >= 16:
		s += 3 // spacious
	}
	if c.TotalWalls > 0 && c.StoneWalls*2 >= c.TotalWalls {
		s += 3
	}
	return s
}

// Label is a player-facing word for an impressiveness score.
func Label(score int) string {
	switch {
	case score < 0:
		return "awful"
	case score < 6:
		return "dull"
	case score < 12:
		return "decent"
	case score < 18:
		return "nice"
	}
	return "impressive"
}
