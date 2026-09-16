package ebiten

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/domain/sim/entity"
)

var workColumns = []string{"FF", "Doc", "Grow", "Cook", "Bld", "Hunt", "Haul", "Cln", "Rsh"}

const (
	workRowH  = 22
	workNameW = 110
	workCellW = 36
)

func workRect(sw, sh int) button {
	h := hudBarY(sh) - float32(hudTopH) - 2*hudPad
	if h < 80 {
		h = 80
	}
	return button{
		x: hudPad,
		y: hudTopH + hudPad,
		w: float32(sw) - 2*hudPad,
		h: h,
	}
}

func workStubCell(id entity.EntityID, col int) string {
	if (uint64(id)+uint64(col))%5 == 0 {
		return "-"
	}
	return "1"
}

func (a *App) drawWork(screen *ebiten.Image, sw, sh int) {
	if a.renderer == nil {
		return
	}
	r := workRect(sw, sh)
	drawPanel(screen, r.x, r.y, r.w, r.h)
	drawLabel(screen, int(r.x)+12, int(r.y)+10, "Work  (placeholders; does not change simulation)")
	hx := int(r.x) + 12 + workNameW
	for i, col := range workColumns {
		drawLabel(screen, hx+i*workCellW, int(r.y)+28, col)
	}
	pawns := a.renderer.snapshot()
	for row, p := range pawns {
		y := int(r.y) + 48 + row*workRowH
		if y > int(r.y+r.h)-20 {
			break
		}
		drawLabel(screen, int(r.x)+12, y, pawnLabel(p.ID))
		for col := range workColumns {
			drawLabel(screen, hx+col*workCellW, y, workStubCell(p.ID, col))
		}
	}
}
