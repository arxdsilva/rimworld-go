package ebiten

import (
	"fmt"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

func inspectorRect() button {
	return button{x: hudPad, y: hudTopH + hudPad, w: 340, h: 250}
}

func (a *App) inspectorButtons() []button {
	if a.renderer == nil || !a.renderer.HasSelect() || a.workOpen {
		return nil
	}
	r := inspectorRect()
	tabs := []buttonSpec{
		{id: "inspector.character", label: "Character", selected: true, w: 72, h: 22},
		{id: "inspector.needs", label: "Needs", disabled: true, reason: "not implemented", w: 56, h: 22},
		{id: "inspector.health", label: "Health", disabled: true, reason: "not implemented", w: 56, h: 22},
		{id: "inspector.social", label: "Social", disabled: true, reason: "not implemented", w: 56, h: 22},
		{id: "inspector.gear", label: "Gear", disabled: true, reason: "not implemented", w: 48, h: 22},
	}
	out := inspectorTabRow(r, tabs)
	out = append(out, button{
		id:    "inspector.close",
		label: "X",
		x:     r.x + r.w - 28,
		y:     r.y + 6,
		w:     22,
		h:     22,
	})
	return out
}

func inspectorTabRow(r button, tabs []buttonSpec) []button {
	x := r.x + 8
	y := r.y + r.h - 28
	out := make([]button, 0, len(tabs))
	for _, t := range tabs {
		w := t.w
		if w == 0 {
			w = 56
		}
		out = append(out, button{
			id:       t.id,
			label:    t.label,
			x:        x,
			y:        y,
			w:        w,
			h:        22,
			disabled: t.disabled,
			reason:   t.reason,
			selected: t.selected,
		})
		x += w + 4
	}
	return out
}

func (a *App) drawInspector(screen *ebiten.Image) {
	if a.renderer == nil || !a.renderer.HasSelect() || a.workOpen {
		return
	}
	p, ok := a.selectedPawn()
	if !ok {
		return
	}
	r := inspectorRect()
	drawPanel(screen, r.x, r.y, r.w, r.h)
	drawLabel(screen, int(r.x)+12, int(r.y)+12, pawnLabel(p.ID))
	drawLabel(screen, int(r.x)+12, int(r.y)+32, "colonist")
	a.drawPawnPortrait(screen, p, float64(r.x)+12, float64(r.y)+52, 48, 64)
	drawLabel(screen, int(r.x)+72, int(r.y)+56, fmt.Sprintf("activity  %s", p.Activity))
	drawLabel(screen, int(r.x)+72, int(r.y)+72, fmt.Sprintf("tile      (%d, %d)", p.X, p.Y))
	drawLabel(screen, int(r.x)+12, int(r.y)+128, "Character  (skills/traits are placeholders)")
	drawLabel(screen, int(r.x)+12, int(r.y)+148, "Growing 2   Construction 1   Intellectual -")
	mx, my := a.currentCursor()
	a.drawButtons(screen, a.inspectorButtons(), mx, my)
}

func (a *App) selectedPawn() (game.PawnView, bool) {
	if a.renderer == nil || !a.renderer.HasSelect() {
		return game.PawnView{}, false
	}
	id := a.renderer.Selected()
	for _, p := range a.renderer.snapshot() {
		if p.ID == id {
			return p, true
		}
	}
	return game.PawnView{}, false
}
