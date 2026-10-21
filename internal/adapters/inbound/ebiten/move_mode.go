package ebiten

import (
	"fmt"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// Move mode (UI-011): the building panel's Move button lets the player
// pick a new spot for a built object; a left click enqueues
// command.MoveBuilding, which plans a reinstall frame. Validity comes
// from the sim's read-only CanMoveTo through a consumer-owned interface.

// buildingMover is the optional sim capability move mode needs.
type buildingMover interface {
	CanMoveTo(bid entity.EntityID, facing string, x, y int) (bool, string)
}

// bldgMoveState is the move-mode state of one App (App.bldgMove).
type bldgMoveState struct {
	on     bool
	bid    entity.EntityID
	facing uint8
	fresh  bool          // entered this frame: ignore the click that opened it
	check  buildingMover // test override of the sim's CanMoveTo
}

func (a *App) moveSt() *bldgMoveState { return &a.bldgMove }

// movableKind mirrors construction.Movable by name: everything except
// walls, doors, floors and graves.
func movableKind(kind string, floor bool) bool {
	if floor {
		return false
	}
	switch kind {
	case "wall", "door", "grave":
		return false
	}
	return true
}

func facingIndex(name string) uint8 {
	for i, n := range facingNames {
		if n == name {
			return uint8(i)
		}
	}
	return 0
}

// moveCancelRect is the rectangle that cancels a pending move: exactly
// the reinstall frame's own cell (cancelRect matches frames by origin).
func moveCancelRect(f game.FrameView) command.Command {
	return command.Command{Kind: command.DesignateCancel, X: f.X, Y: f.Y, X2: f.X, Y2: f.Y}
}

// pendingMoveFrame is the reinstall frame heading for building bid.
func pendingMoveFrame(w game.WorldView, bid entity.EntityID) (game.FrameView, bool) {
	for _, f := range w.Frames {
		if f.Moving && f.MovingFrom == bid {
			return f, true
		}
	}
	return game.FrameView{}, false
}

func (a *App) moveBuilding() (game.BuildingView, bool) {
	s := a.moveSt()
	if !s.on || a.renderer == nil {
		return game.BuildingView{}, false
	}
	for _, b := range a.renderer.world().Buildings {
		if b.ID == s.bid {
			return b, true
		}
	}
	return game.BuildingView{}, false
}

func (a *App) startMove(b game.BuildingView) {
	if !movableKind(b.Kind, b.Floor) {
		return
	}
	s := a.moveSt()
	s.on, s.bid, s.facing, s.fresh = true, b.ID, facingIndex(b.Facing), true
	a.archTool = ""
	a.archDrag = false
}

func (a *App) endMove() { a.moveSt().on = false }

// moveValid asks the sim whether the building may be reinstalled at
// (x, y) with the current facing.
func (a *App) moveValid(b game.BuildingView, x, y int) (bool, string) {
	s := a.moveSt()
	var m buildingMover = s.check
	if m == nil {
		m, _ = a.renderer.game.(buildingMover)
	}
	if m == nil {
		return false, "can't move this"
	}
	return m.CanMoveTo(b.ID, facingNames[s.facing], x, y)
}

// moveStep is one frame of move mode on the hovered tile; the input
// wrapper feeds it. It reports whether the input was consumed.
func (a *App) moveStep(tx, ty int, onMap bool, rotate int, left, cancel bool) bool {
	s := a.moveSt()
	if !s.on || a.renderer == nil {
		return false
	}
	b, ok := a.moveBuilding()
	if !ok {
		s.on = false
		return false
	}
	if s.fresh {
		s.fresh = false
		return true
	}
	if cancel {
		s.on = false
		return true
	}
	if bv, ok := buildable(b.Kind); ok && bv.Rotatable && rotate != 0 {
		s.facing = rotateFacing(s.facing, rotate)
	}
	if left && onMap {
		okMove, why := a.moveValid(b, tx, ty)
		if !okMove {
			a.hint = why
			return true
		}
		a.renderer.enqueue(command.Command{Kind: command.MoveBuilding, Target: uint64(b.ID), X: tx, Y: ty, Facing: s.facing})
		s.on = false
	}
	return true
}

// handleMoveMode reads the mouse and keys; true when move mode took the
// input (so selection must not also act on the click).
func (a *App) handleMoveMode() (consumed bool) {
	if !a.moveSt().on || a.renderer == nil {
		return false
	}
	rot := 0
	if inpututil.IsKeyJustPressed(ebiten.KeyQ) {
		rot--
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyE) {
		rot++
	}
	mx, my := a.currentCursor()
	sw, sh := a.size()
	tx, ty := a.renderer.TileAt(mx, my)
	onMap := !a.colonyChromeHit(mx, my, sw, sh)
	cancel := inpututil.IsKeyJustPressed(ebiten.KeyEscape) || inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight)
	left := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
	return a.moveStep(tx, ty, onMap, rot, left, cancel)
}

// drawMoveMode draws the ghost at the cursor tile and the faint marker
// on the original footprint.
func (a *App) drawMoveMode(screen *ebiten.Image) {
	b, ok := a.moveBuilding()
	if !ok {
		return
	}
	mx, my := a.currentCursor()
	tile := float32(a.renderer.tilePx())
	ox, oy := a.renderer.worldToScreen(b.X, b.Y)
	vector.StrokeRect(screen, ox+1, oy+1, tile*float32(max(b.W, 1))-2, tile*float32(max(b.H, 1))-2, 1, uiAccent, false)
	sw, sh := a.size()
	if a.colonyChromeHit(mx, my, sw, sh) {
		return
	}
	tx, ty := a.renderer.TileAt(mx, my)
	okMove, why := a.moveValid(b, tx, ty)
	a.renderer.drawPlacementGhost(screen, b.Kind, b.Stuff, facingNames[a.moveSt().facing],
		[]ghostCell{{x: tx, y: ty, ok: okMove, reason: why}})
	msg := "Move " + buildingTitle(b)
	if !okMove {
		msg += "   can't move: " + why
	} else if bv, ok := buildable(b.Kind); ok && bv.Rotatable {
		msg += "   Q/E rotate"
	}
	vector.FillRect(screen, float32(mx+16), float32(my+18), float32(len(msg)*6+12), 18, uiPanel, false)
	drawLabel(screen, mx+22, my+19, msg)
}

// drawMoveMarkers labels each reinstall frame with a small arrow.
func (a *App) drawMoveMarkers(screen *ebiten.Image) {
	if a.renderer == nil {
		return
	}
	for _, f := range a.renderer.world().Frames {
		if !f.Moving {
			continue
		}
		sx, sy := a.renderer.worldToScreen(f.X, f.Y)
		drawLabel(screen, int(sx)+2, int(sy)+2, "->")
	}
}

// movingLine is the panel's pending-move text.
func movingLine(b game.BuildingView) string {
	if b.MovingTo == nil {
		return ""
	}
	return fmt.Sprintf("Moving to (%d, %d)", b.MovingTo[0], b.MovingTo[1])
}
