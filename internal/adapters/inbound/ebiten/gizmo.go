package ebiten

import (
	"sort"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/platform/input"
)

// Gizmo keys (UI-016): B H Y N J U M K I L O P press the selected thing's
// inspector command buttons in display order (top to bottom, left to
// right); each button shows its letter in a corner badge.

// gizmoSkip reports buttons that are not commands: closers, tabs, list
// rows and the bill row's small +/-/mode/pause/delete controls.
func gizmoSkip(b button) bool {
	switch {
	case strings.HasSuffix(b.id, ".close"), strings.HasPrefix(b.id, "inspector."),
		strings.HasPrefix(b.id, "group.pick."), strings.HasPrefix(b.id, "bill.") && !strings.HasPrefix(b.id, "bill.add."),
		b.w < 40:
		return true
	}
	return false
}

// gizmoButtons are the visible inspector's command buttons in key order.
func (a *App) gizmoButtons() []button {
	if a.renderer == nil || a.hideUI {
		return nil
	}
	var all []button
	for _, bs := range [][]button{a.groupButtons(), a.inspectorButtons(), a.buildingButtons(), a.animalButtons(),
		a.plantButtons(), a.itemButtons(), a.frameButtons()} {
		all = append(all, bs...)
	}
	var out []button
	for _, b := range all {
		if !gizmoSkip(b) {
			out = append(out, b)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].y != out[j].y {
			return out[i].y < out[j].y
		}
		return out[i].x < out[j].x
	})
	if len(out) > len(input.Gizmos) {
		out = out[:len(input.Gizmos)]
	}
	return out
}

// pressGizmo presses command button i (a disabled one shows its reason).
func (a *App) pressGizmo(i int) {
	bs := a.gizmoButtons()
	if i >= len(bs) {
		return
	}
	b := bs[i]
	if b.disabled {
		a.hint = b.reason
		return
	}
	a.hint = ""
	a.applyClick(b.id)
}

// drawGizmoKeys badges each command button with its key.
func (a *App) drawGizmoKeys(screen *ebiten.Image) {
	for i, b := range a.gizmoButtons() {
		l := a.bindings().Label(input.Gizmos[i])
		if l == "" {
			continue
		}
		w := textW(l, tsSmall) + 4
		x, y := b.x+b.w-w-1, b.y+1
		uiFillRect(screen, x, y, w, 13, uiSpace, false)
		drawText(screen, l, float64(x)+2, float64(y)-3, tsSmall)
	}
}
