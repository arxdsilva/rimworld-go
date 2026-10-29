package orders

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// Mortar targeting (DEF-3): the mortar pane's "Set target" button enters
// this mode; the next map click sends SetMortarTarget with the clicked
// tile as X2, Y2. Esc (the root's chain) or a right click leaves it.

// MortarAim is the targeting mode's state.
type MortarAim struct {
	On    bool
	Bid   entity.EntityID
	X, Y  int  // the mortar's tile
	Fresh bool // entered this frame: ignore the click that opened it
}

// Start enters the mode for the mortar at (x, y).
func (m *MortarAim) Start(id entity.EntityID, x, y int) {
	*m = MortarAim{On: true, Bid: id, X: x, Y: y, Fresh: true}
}

// Step is one frame: left on a map tile sets the target, cancel leaves.
// It reports whether the input was consumed.
func (m *MortarAim) Step(sh ports.Commands, crew entity.EntityID, tx, ty int, onMap, left, cancel bool) bool {
	if !m.On {
		return false
	}
	if m.Fresh {
		m.Fresh = false
		return true
	}
	if cancel {
		m.On = false
		return true
	}
	if left && onMap {
		sh.Enqueue(command.Command{Kind: command.SetMortarTarget, X: m.X, Y: m.Y, X2: tx, Y2: ty, Pawn: uint64(crew)})
		m.On = false
	}
	return true
}

// HandleAim feeds the targeting mode this frame's input; true when it took
// the input (so selection must not also act on the click).
func (c *Controller) HandleAim() bool {
	if !c.Aim.On {
		return false
	}
	f, _ := c.sh.Frame()
	mx, my := c.sh.Cursor()
	tx, ty := c.sh.TileAt(c.sh.MapCursor())
	onMap := !c.sh.ChromeHit(mx, my)
	left, cancel := f.Clicked(inputframe.Left), f.Clicked(inputframe.Right)
	f.Take(inputframe.Left)
	f.Take(inputframe.Right)
	if c.Aim.Fresh { // once, as the mode opens (the hint draws through the canvas)
		c.sh.Hint("Click a tile to target the mortar; Esc or right-click cancels")
	}
	var crew entity.EntityID // the selected colonist mans it
	if ps := c.sh.Selection().SelectedPawns(); len(ps) == 1 {
		crew = ps[0].ID
	}
	return c.Aim.Step(c.sh, crew, tx, ty, onMap, left, cancel)
}

// drawAim outlines the mortar and the tile under the cursor (world
// drawing; the hint is shown by HandleAim through the canvas).
func (c *Controller) drawAim(screen *ebiten.Image) {
	if !c.Aim.On {
		return
	}
	tile := float32(c.sh.TilePx())
	ox, oy := c.sh.WorldToScreen(c.Aim.X, c.Aim.Y)
	vector.StrokeRect(screen, ox+1, oy+1, tile-2, tile-2, 1, uikit.Accent, false)
	mx, my := c.sh.Cursor()
	if c.sh.ChromeHit(mx, my) {
		return
	}
	tx, ty := c.sh.TileAt(c.sh.MapCursor())
	sx, sy := c.sh.WorldToScreen(tx, ty)
	vector.StrokeRect(screen, sx+1, sy+1, tile-2, tile-2, 2, uikit.Warning, false)
}
