package ebiten

import (
	"fmt"
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/application/game"
)

func inspectorRect() button {
	return button{x: hudPad, y: hudTopH + hudPad, w: 340, h: 280}
}

func (a *App) inspectorButtons() []button {
	if a.renderer == nil || !a.renderer.HasSelect() || a.workOpen || a.archOpen {
		return nil
	}
	r := inspectorRect()
	tabs := []buttonSpec{
		{id: "inspector.character", label: "Character", selected: a.inspectTab != "needs", w: 72, h: 22},
		{id: "inspector.needs", label: "Needs", selected: a.inspectTab == "needs", w: 56, h: 22},
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
	if a.renderer == nil || !a.renderer.HasSelect() || a.workOpen || a.archOpen {
		return
	}
	p, ok := a.selectedPawn()
	if !ok {
		return
	}
	r := inspectorRect()
	drawPanel(screen, r.x, r.y, r.w, r.h)
	drawLabel(screen, int(r.x)+12, int(r.y)+12, pawnLabel(p))
	drawLabel(screen, int(r.x)+12, int(r.y)+32, "colonist  "+p.Status)
	a.drawPawnPortrait(screen, p, float64(r.x)+12, float64(r.y)+52, 48, 64)
	drawLabel(screen, int(r.x)+72, int(r.y)+56, fmt.Sprintf("job       %s", p.Job))
	drawLabel(screen, int(r.x)+72, int(r.y)+72, fmt.Sprintf("activity  %s", p.Activity))
	drawLabel(screen, int(r.x)+72, int(r.y)+88, fmt.Sprintf("tile      (%d, %d)", p.X, p.Y))
	if p.Reason != "" {
		drawLabel(screen, int(r.x)+72, int(r.y)+104, "reason    "+p.Reason)
	}
	drawNeedBar(screen, int(r.x)+12, int(r.y)+140, 300, 14, p.Hunger, "food")
	drawNeedBar(screen, int(r.x)+12, int(r.y)+164, 300, 14, p.Rest, "rest")
	mx, my := a.currentCursor()
	a.drawButtons(screen, a.inspectorButtons(), mx, my)
}

func drawNeedBar(screen *ebiten.Image, x, y, w, h, pct int, label string) {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	drawLabel(screen, x, y-2, fmt.Sprintf("%s  %d", label, pct))
	bg := color.RGBA{R: 40, G: 30, B: 24, A: 255}
	fill := color.RGBA{R: 180, G: 90, B: 50, A: 255}
	if label == "rest" {
		fill = color.RGBA{R: 70, G: 110, B: 180, A: 255}
	}
	vector.FillRect(screen, float32(x+50), float32(y), float32(w-50), float32(h), bg, false)
	fw := float32(w-50) * float32(pct) / 100
	vector.FillRect(screen, float32(x+50), float32(y), fw, float32(h), fill, false)
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
