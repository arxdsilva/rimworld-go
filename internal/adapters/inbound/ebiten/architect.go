package ebiten

import ebiten "github.com/hajimehoshi/ebiten/v2"

func architectRect() button {
	return button{x: hudPad, y: hudTopH + hudPad, w: 220, h: 170}
}

func (a *App) architectButtons() []button {
	if !a.archOpen {
		return nil
	}
	r := architectRect()
	tools := []struct{ id, label string }{
		{"wall", "Wall"},
		{"bed", "Bed"},
		{"stockpile", "Stockpile"},
		{"harvest", "Harvest"},
		{"cancel", "Cancel"},
	}
	out := make([]button, 0, len(tools))
	for i, t := range tools {
		out = append(out, button{
			id:       "arch.tool." + t.id,
			label:    t.label,
			x:        r.x + 12,
			y:        r.y + 36 + float32(i*26),
			w:        196,
			h:        22,
			selected: a.archTool == t.id,
		})
	}
	return out
}

func (a *App) drawArchitect(screen *ebiten.Image) {
	if !a.archOpen {
		return
	}
	r := architectRect()
	drawPanel(screen, r.x, r.y, r.w, r.h)
	drawLabel(screen, int(r.x)+12, int(r.y)+10, "Architect")
	mx, my := a.currentCursor()
	a.drawButtons(screen, a.architectButtons(), mx, my)
}

func (a *App) drawArchitectGhost(screen *ebiten.Image) {
	if a.renderer == nil || !a.archOpen || !a.archDrag {
		return
	}
	a.renderer.drawGhostRect(screen, a.archX0, a.archY0, a.archX1, a.archY1)
}
