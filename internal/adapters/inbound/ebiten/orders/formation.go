package orders

import (
	"sort"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// Formation moves (UI-016): drafted colonists right-clicking a tile spread
// over the nearest free walkable cells around it; a right drag lays them
// along the line, evenly spaced. One MovePawn per colonist (Shift
// queues). Cells come from the read model and are deterministic.

// Formation is a right press with drafted colonists selected.
type Formation struct {
	On             bool
	X0, Y0, X1, Y1 int
}

// formationRadius bounds the search for free cells around a target.
const formationRadius = 8

// pawnOrderAt reports the tile holds a pawn p can act on (MED-03, SIM-062): that
// right-click opens the order menu (float.go RightClickTile) instead of
// starting a formation drag, which used to claim every drafted
// right-click on a tile without a raider or animal (QA-001 finding).
func (c *Controller) pawnOrderAt(p game.PawnView, tx, ty int) bool {
	return hasPawnOrder(c.sh.OrdersAt(p.ID, tx, ty))
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// draftedOf keeps the drafted colonists.
func draftedOf(ps []game.PawnView) []game.PawnView {
	var out []game.PawnView
	for _, p := range ps {
		if p.Drafted {
			out = append(out, p)
		}
	}
	return out
}

// walkableFn is the read model's passability: walkable ground with no
// wall on it, inside the map.
func walkableFn(w game.WorldView, mapW, mapH int) func(x, y int) bool {
	soils := game.Soils()
	walls := map[[2]int]bool{}
	for _, b := range w.Buildings {
		if b.Kind == "wall" {
			walls[[2]int{b.X, b.Y}] = true
		}
	}
	return func(x, y int) bool {
		if x < 0 || y < 0 || x >= mapW || y >= mapH || walls[[2]int{x, y}] {
			return false
		}
		if w.TerrainW <= 0 {
			return true
		}
		i := y*w.TerrainW + x
		if i >= len(w.Terrain) || int(w.Terrain[i]) >= len(soils) {
			return true
		}
		return soils[w.Terrain[i]].Walkable
	}
}

// nearestFree is the free cell closest to (x, y): ring by ring (Chebyshev),
// inside a ring by distance then row then column. False when none is
// within formationRadius.
func nearestFree(x, y int, free func(x, y int) bool, taken map[[2]int]bool) ([2]int, bool) {
	for r := 0; r <= formationRadius; r++ {
		ring := ringCells(x, y, r)
		for _, c := range ring {
			if !taken[c] && free(c[0], c[1]) {
				return c, true
			}
		}
	}
	return [2]int{}, false
}

// ringCells is the cells at Chebyshev distance r from (x, y), nearest
// first, ties by row then column.
func ringCells(x, y, r int) [][2]int {
	if r == 0 {
		return [][2]int{{x, y}}
	}
	var out [][2]int
	for dy := -r; dy <= r; dy++ {
		for dx := -r; dx <= r; dx++ {
			if max(abs(dx), abs(dy)) == r {
				out = append(out, [2]int{x + dx, y + dy})
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		di := sq(out[i][0]-x) + sq(out[i][1]-y)
		dj := sq(out[j][0]-x) + sq(out[j][1]-y)
		if di != dj {
			return di < dj
		}
		if out[i][1] != out[j][1] {
			return out[i][1] < out[j][1]
		}
		return out[i][0] < out[j][0]
	})
	return out
}

func sq(v int) int { return v * v }

// FormationCells is n free cells around (x, y), nearest first.
func FormationCells(x, y, n int, free func(x, y int) bool) [][2]int {
	taken := map[[2]int]bool{}
	var out [][2]int
	for r := 0; r <= formationRadius && len(out) < n; r++ {
		for _, c := range ringCells(x, y, r) {
			if len(out) == n {
				break
			}
			if !taken[c] && free(c[0], c[1]) {
				taken[c] = true
				out = append(out, c)
			}
		}
	}
	return out
}

// bresenham is the cells of the straight line from a to b.
func bresenham(x0, y0, x1, y1 int) [][2]int {
	dx, dy := abs(x1-x0), -abs(y1-y0)
	sx, sy := 1, 1
	if x0 > x1 {
		sx = -1
	}
	if y0 > y1 {
		sy = -1
	}
	e := dx + dy
	var out [][2]int
	for {
		out = append(out, [2]int{x0, y0})
		if x0 == x1 && y0 == y1 {
			return out
		}
		if e2 := 2 * e; e2 >= dy {
			e += dy
			x0 += sx
		}
		if e2 := 2 * e; e2 <= dx {
			e += dx
			y0 += sy
		}
	}
}

// LineCells is n cells evenly spaced along the line from a to b (both
// ends included when n > 1); a blocked or doubled cell moves to the
// nearest free one.
func LineCells(x0, y0, x1, y1, n int, free func(x, y int) bool) [][2]int {
	line := bresenham(x0, y0, x1, y1)
	taken := map[[2]int]bool{}
	var out [][2]int
	for i := 0; i < n; i++ {
		j := len(line) - 1
		if n > 1 {
			j = (i*(len(line)-1)*2 + (n - 1)) / (2 * (n - 1)) // round(i*(L-1)/(n-1))
		}
		c := line[j]
		if taken[c] || !free(c[0], c[1]) {
			var ok bool
			if c, ok = nearestFree(c[0], c[1], free, taken); !ok {
				continue
			}
		}
		taken[c] = true
		out = append(out, c)
	}
	return out
}

// Move is one colonist's slot.
type Move struct {
	Pawn entity.EntityID
	X, Y int
}

// AssignCells gives each cell (in order) the nearest colonist not yet
// placed, ties by id.
func AssignCells(pawns []game.PawnView, cells [][2]int) []Move {
	left := append([]game.PawnView(nil), pawns...)
	var out []Move
	for _, c := range cells {
		if len(left) == 0 {
			break
		}
		best := 0
		for i, p := range left[1:] {
			d := sq(p.X-c[0]) + sq(p.Y-c[1])
			b := left[best]
			db := sq(b.X-c[0]) + sq(b.Y-c[1])
			if d < db || (d == db && p.ID < b.ID) {
				best = i + 1
			}
		}
		out = append(out, Move{Pawn: left[best].ID, X: c[0], Y: c[1]})
		left = append(left[:best], left[best+1:]...)
	}
	return out
}

// FormationMoves is where the drafted colonists go for a click at (x0, y0)
// or a drag to (x1, y1).
func FormationMoves(pawns []game.PawnView, x0, y0, x1, y1 int, free func(x, y int) bool) []Move {
	if len(pawns) == 1 && x0 == x1 && y0 == y1 {
		return []Move{{Pawn: pawns[0].ID, X: x0, Y: y0}} // one colonist goes exactly there
	}
	var cells [][2]int
	if x0 == x1 && y0 == y1 {
		cells = FormationCells(x0, y0, len(pawns), free)
	} else {
		cells = LineCells(x0, y0, x1, y1, len(pawns), free)
	}
	return AssignCells(pawns, cells)
}

// HandleOrders is the right button with colonists selected: drafted ones
// start a formation (move on release), an undrafted single colonist gets
// the order menu (RightClickTile).
func (c *Controller) HandleOrders() {
	f, _ := c.sh.Frame()
	mx, my := c.sh.Cursor()
	tx, ty := c.sh.TileAt(c.sh.MapCursor()) // the map is physical
	if c.Form.On {
		if f.Down(inputframe.Right) {
			c.Form.X1, c.Form.Y1 = tx, ty
			return
		}
		c.finishFormation()
		return
	}
	if !f.Clicked(inputframe.Right) {
		return
	}
	sel := c.sh.Selection()
	pawns := sel.SelectedPawns()
	if len(pawns) == 0 {
		return
	}
	if c.sh.ChromeHit(mx, my) && !c.FloatRect().Contains(mx, my) {
		return
	}
	f.Take(inputframe.Right)
	if drafted := draftedOf(pawns); len(drafted) > 0 && !c.creatureAt(tx, ty, 0) && !c.pawnOrderAt(drafted[0], tx, ty) {
		c.CloseFloat()
		c.Form = Formation{On: true, X0: tx, Y0: ty, X1: tx, Y1: ty}
		return
	}
	if sel.Grouped() {
		c.sh.Hint("Draft the colonists to move them together")
		return
	}
	c.RightClickTile(tx, ty, float32(mx), float32(my))
}

// finishFormation sends the formation's moves.
func (c *Controller) finishFormation() {
	d := c.Form
	c.Form = Formation{}
	pawns := draftedOf(c.sh.Selection().SelectedPawns())
	if len(pawns) == 0 {
		return
	}
	mapW, mapH := c.sh.MapSize()
	free := walkableFn(c.sh.World(), mapW, mapH)
	for _, m := range FormationMoves(pawns, d.X0, d.Y0, d.X1, d.Y1, free) {
		c.sh.Enqueue(command.Command{Kind: command.MovePawn, Pawn: uint64(m.Pawn), X: m.X, Y: m.Y, Queue: c.queueing()})
	}
}

// DrawFormation previews the cells while the right button is held.
func (c *Controller) DrawFormation(screen *ebiten.Image) {
	c.drawAim(screen) // mortar_aim.go
	if !c.Form.On {
		return
	}
	pawns := draftedOf(c.sh.Selection().SelectedPawns())
	mapW, mapH := c.sh.MapSize()
	free := walkableFn(c.sh.World(), mapW, mapH)
	tile := float32(c.sh.TilePx())
	for _, m := range FormationMoves(pawns, c.Form.X0, c.Form.Y0, c.Form.X1, c.Form.Y1, free) {
		sx, sy := c.sh.WorldToScreen(m.X, m.Y)
		vector.StrokeRect(screen, sx+tile*0.2, sy+tile*0.2, tile*0.6, tile*0.6, 2, uikit.Accent, false)
	}
}
