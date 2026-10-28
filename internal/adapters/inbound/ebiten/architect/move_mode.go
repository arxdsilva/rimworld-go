package architect

import (
	"fmt"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/platform/input"
)

// Move mode (UI-011): the building panel's Move button lets the player
// pick a new spot for a built object; a left click enqueues
// command.MoveBuilding, which plans a reinstall frame. Validity comes
// from the sim's read-only CanMoveTo (BuildingMover).

// Mover is the building Move mode.
type Mover struct {
	*Tool
	st MoveState
}

// MoveState is Move mode's state.
type MoveState struct {
	On     bool
	Bid    entity.EntityID
	Facing uint8
	Fresh  bool          // entered this frame: ignore the click that opened it
	Check  BuildingMover // test override of the sim's CanMoveTo
}

// State is Move mode's state.
func (m *Mover) State() *MoveState { return &m.st }

// MovableKind mirrors construction.Movable by name: everything except
// walls, doors, fences, gates, floors and graves.
func MovableKind(kind string, floor bool) bool {
	if floor {
		return false
	}
	switch kind {
	case "wall", "door", "grave", "fence", "fence gate":
		return false
	}
	return true
}

// FacingIndex is the construction facing named name (south when unknown).
func FacingIndex(name string) uint8 {
	for i, n := range FacingNames {
		if n == name {
			return uint8(i)
		}
	}
	return 0
}

// MoveCancelRect is the rectangle that cancels a pending move: exactly
// the reinstall frame's own cell (cancelRect matches frames by origin).
func MoveCancelRect(f game.FrameView) command.Command {
	return command.Command{Kind: command.DesignateCancel, X: f.X, Y: f.Y, X2: f.X, Y2: f.Y}
}

// PendingMoveFrame is the reinstall frame heading for building bid.
func PendingMoveFrame(w game.WorldView, bid entity.EntityID) (game.FrameView, bool) {
	for _, f := range w.Frames {
		if f.Moving && f.MovingFrom == bid {
			return f, true
		}
	}
	return game.FrameView{}, false
}

func (m *Mover) building() (game.BuildingView, bool) {
	s := &m.st
	if !s.On || m.sim() == nil {
		return game.BuildingView{}, false
	}
	for _, b := range m.sh.World().Buildings {
		if b.ID == s.Bid {
			return b, true
		}
	}
	return game.BuildingView{}, false
}

// Start enters Move mode for building b (an immovable kind is ignored).
func (m *Mover) Start(b game.BuildingView) {
	if !MovableKind(b.Kind, b.Floor) {
		return
	}
	s := &m.st
	s.On, s.Bid, s.Facing, s.Fresh = true, b.ID, FacingIndex(b.Facing), true
	m.ToolID = ""
	m.Drag = false
}

// valid asks the sim whether the building may be reinstalled at (x, y)
// with the current facing.
func (m *Mover) valid(b game.BuildingView, x, y int) (bool, string) {
	s := &m.st
	mv := s.Check
	if mv == nil {
		mv, _ = m.sim().(BuildingMover)
	}
	if mv == nil {
		return false, "can't move this"
	}
	return mv.CanMoveTo(b.ID, FacingNames[s.Facing], x, y)
}

// Step is one frame of move mode on the hovered tile; the input wrapper
// feeds it. It reports whether the input was consumed.
func (m *Mover) Step(tx, ty int, onMap bool, rotate int, left, cancel bool) bool {
	s := &m.st
	if !s.On || m.sim() == nil {
		return false
	}
	b, ok := m.building()
	if !ok {
		s.On = false
		return false
	}
	if s.Fresh {
		s.Fresh = false
		return true
	}
	if cancel {
		s.On = false
		return true
	}
	if bv, ok := Buildable(b.Kind); ok && bv.Rotatable && rotate != 0 {
		s.Facing = RotateFacing(s.Facing, rotate)
	}
	if left && onMap {
		okMove, why := m.valid(b, tx, ty)
		if !okMove {
			m.sh.Hint(why)
			return true
		}
		m.sh.Enqueue(command.Command{Kind: command.MoveBuilding, Target: uint64(b.ID), X: tx, Y: ty, Facing: s.Facing})
		s.On = false
	}
	return true
}

// Handle feeds move mode this frame's input; true when move mode took it
// (so selection must not also act on the click). Escape is the Esc
// chain's, so it is handled once.
func (m *Mover) Handle() (consumed bool) {
	if !m.st.On || m.sim() == nil {
		return false
	}
	f := m.frame()
	rot := 0
	if f.Fired(input.RotateLeft) {
		rot--
	}
	if f.Fired(input.RotateRight) {
		rot++
	}
	mx, my := m.sh.Cursor()
	sw, sh := m.sh.Size()
	pmx, pmy := m.sh.MapCursor()
	tx, ty := m.sh.TileAt(pmx, pmy)
	onMap := !m.sh.ChromeHit(mx, my, sw, sh)
	cancel := f.Clicked(inputframe.Right)
	left := f.Clicked(inputframe.Left)
	f.Take(inputframe.Left)
	f.Take(inputframe.Right)
	return m.Step(tx, ty, onMap, rot, left, cancel)
}

// Draw draws the ghost at the cursor tile and the faint marker on the
// original footprint.
func (m *Mover) Draw(screen *ebiten.Image) {
	b, ok := m.building()
	if !ok {
		return
	}
	mx, my := m.sh.Cursor()
	tile := float32(m.sh.TilePx())
	ox, oy := m.sh.WorldToScreen(b.X, b.Y)
	vector.StrokeRect(screen, ox+1, oy+1, tile*float32(max(b.W, 1))-2, tile*float32(max(b.H, 1))-2, 1, uikit.Accent, false)
	sw, sh := m.sh.Size()
	if m.sh.ChromeHit(mx, my, sw, sh) {
		return
	}
	pmx, pmy := m.sh.MapCursor()
	tx, ty := m.sh.TileAt(pmx, pmy)
	okMove, why := m.valid(b, tx, ty)
	m.sh.DrawPlacementGhost(screen, b.Kind, b.Stuff, FacingNames[m.st.Facing],
		[]GhostCell{{X: tx, Y: ty, OK: okMove, Reason: why}})
	msg := "Move " + m.sh.BuildingTitle(b)
	if !okMove {
		msg += "   can't move: " + why
	} else if bv, ok := Buildable(b.Kind); ok && bv.Rotatable {
		msg += "   " + m.rotateKeys() + " rotate"
	}
	gfx.FillRect(screen, float32(mx+16), float32(my+18), gfx.TextW(msg, gfx.Body)+16, gfx.LineStep(gfx.Body)+4, uikit.Panel, false)
	gfx.Text(screen, msg, float64(mx+24), float64(my+20), gfx.Body)
}

// DrawMarkers labels each reinstall frame with a small arrow.
func (m *Mover) DrawMarkers(screen *ebiten.Image) {
	if m.sim() == nil {
		return
	}
	for _, f := range m.sh.World().Frames {
		if !f.Moving {
			continue
		}
		sx, sy := m.sh.WorldToScreen(f.X, f.Y)
		gfx.Text(screen, "->", float64(sx)/gfx.UIScale()+2, float64(sy)/gfx.UIScale()+2, gfx.WithColor(gfx.Small, uikit.Accent))
	}
}

// MovingLine is the panel's pending-move text.
func MovingLine(b game.BuildingView) string {
	if b.MovingTo == nil {
		return ""
	}
	return fmt.Sprintf("Moving to (%d, %d)", b.MovingTo[0], b.MovingTo[1])
}
