package ebiten

import (
	"sort"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// Formation moves (UI-016): drafted colonists right-clicking a tile spread
// over the nearest free walkable cells around it; a right drag lays them
// along the line, evenly spaced. One MovePawn per colonist (Shift
// queues). Cells come from the read model and are deterministic.

// formationDrag is a right press with drafted colonists selected.
type formationDrag struct {
	on             bool
	x0, y0, x1, y1 int
}

// formationRadius bounds the search for free cells around a target.
const formationRadius = 8

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

// formationCells is n free cells around (x, y), nearest first.
func formationCells(x, y, n int, free func(x, y int) bool) [][2]int {
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

// lineCells is n cells evenly spaced along the line from a to b (both
// ends included when n > 1); a blocked or doubled cell moves to the
// nearest free one.
func lineCells(x0, y0, x1, y1, n int, free func(x, y int) bool) [][2]int {
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

// formationMove is one colonist's slot.
type formationMove struct {
	pawn entity.EntityID
	x, y int
}

// assignCells gives each cell (in order) the nearest colonist not yet
// placed, ties by id.
func assignCells(pawns []game.PawnView, cells [][2]int) []formationMove {
	left := append([]game.PawnView(nil), pawns...)
	var out []formationMove
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
		out = append(out, formationMove{pawn: left[best].ID, x: c[0], y: c[1]})
		left = append(left[:best], left[best+1:]...)
	}
	return out
}

// formationMoves is where the drafted colonists go for a click at (x0, y0)
// or a drag to (x1, y1).
func formationMoves(pawns []game.PawnView, x0, y0, x1, y1 int, free func(x, y int) bool) []formationMove {
	if len(pawns) == 1 && x0 == x1 && y0 == y1 {
		return []formationMove{{pawn: pawns[0].ID, x: x0, y: y0}} // one colonist goes exactly there
	}
	var cells [][2]int
	if x0 == x1 && y0 == y1 {
		cells = formationCells(x0, y0, len(pawns), free)
	} else {
		cells = lineCells(x0, y0, x1, y1, len(pawns), free)
	}
	return assignCells(pawns, cells)
}

// handleOrders is the right button with colonists selected: drafted ones
// start a formation (move on release), an undrafted single colonist gets
// the order menu (rightClickTile).
func (a *App) handleOrders() {
	if a.renderer == nil {
		return
	}
	f := &a.in
	mx, my := a.currentCursor()
	tx, ty := a.renderer.TileAt(mx, my)
	if a.form.on {
		if f.down(mbRight) {
			a.form.x1, a.form.y1 = tx, ty
			return
		}
		a.finishFormation()
		return
	}
	if !f.clicked(mbRight) {
		return
	}
	pawns := a.selectedPawns()
	if len(pawns) == 0 {
		return
	}
	sw, sh := a.size()
	if a.colonyChromeHit(mx, my, sw, sh) && !a.floatRect().contains(mx, my) {
		return
	}
	f.take(mbRight)
	if len(draftedOf(pawns)) > 0 && !a.creatureAt(tx, ty, 0) {
		a.closeFloat()
		a.form = formationDrag{on: true, x0: tx, y0: ty, x1: tx, y1: ty}
		return
	}
	if a.grouped() {
		a.hint = "Draft the colonists to move them together"
		return
	}
	a.rightClickTile(tx, ty, float32(mx), float32(my))
}

// finishFormation sends the formation's moves.
func (a *App) finishFormation() {
	d := a.form
	a.form = formationDrag{}
	pawns := draftedOf(a.selectedPawns())
	if len(pawns) == 0 {
		return
	}
	free := walkableFn(a.renderer.world(), a.renderer.game.Width(), a.renderer.game.Height())
	for _, m := range formationMoves(pawns, d.x0, d.y0, d.x1, d.y1, free) {
		a.renderer.enqueue(command.Command{Kind: command.MovePawn, Pawn: uint64(m.pawn), X: m.x, Y: m.y, Queue: a.queueing()})
	}
}

// drawFormation previews the cells while the right button is held.
func (a *App) drawFormation(screen *ebiten.Image) {
	if !a.form.on || a.renderer == nil {
		return
	}
	pawns := draftedOf(a.selectedPawns())
	r := a.renderer
	free := walkableFn(r.world(), r.game.Width(), r.game.Height())
	tile := float32(r.tilePx())
	for _, m := range formationMoves(pawns, a.form.x0, a.form.y0, a.form.x1, a.form.y1, free) {
		sx, sy := r.worldToScreen(m.x, m.y)
		vector.StrokeRect(screen, sx+tile*0.2, sy+tile*0.2, tile*0.6, tile*0.6, 2, uiAccent, false)
	}
}
