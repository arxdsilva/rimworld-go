package ebiten

import (
	"strconv"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

var workColumns = []string{"FF", "Doc", "Grow", "Cook", "Bld", "Hunt", "Haul", "Cln", "Rsh", "Mine", "Crft"}

// workLive maps live work columns to their command and pawn flag.
var workLive = map[int]struct {
	cmd command.WorkType
	on  func(game.PawnView) bool
}{
	1:  {command.WorkDoctor, func(p game.PawnView) bool { return p.WorkDoctor }},
	2:  {command.WorkGrow, func(p game.PawnView) bool { return p.WorkGrow }},
	3:  {command.WorkCook, func(p game.PawnView) bool { return p.WorkCook }},
	4:  {command.WorkConstruct, func(p game.PawnView) bool { return p.WorkConstruct }},
	5:  {command.WorkHunt, func(p game.PawnView) bool { return p.WorkHunt }},
	6:  {command.WorkHaul, func(p game.PawnView) bool { return p.WorkHaul }},
	9:  {command.WorkMine, func(p game.PawnView) bool { return p.WorkMine }},
	10: {command.WorkCraft, func(p game.PawnView) bool { return p.WorkCraft }},
}

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

func workCellValue(p game.PawnView, col int) string {
	w, live := workLive[col]
	if !live {
		return "-"
	}
	if w.on(p) {
		return "Y"
	}
	return "-"
}

func (a *App) workButtons() []button {
	if a.renderer == nil || !a.workOpen {
		return nil
	}
	r := workRect(a.sw, a.sh)
	hx := int(r.x) + 12 + workNameW
	pawns := a.renderer.snapshot()
	out := make([]button, 0)
	for row, p := range pawns {
		y := int(r.y) + 48 + row*workRowH
		if y > int(r.y+r.h)-20 {
			break
		}
		for col := range workColumns {
			if _, ok := workTypeForCol(col); !ok {
				continue
			}
			out = append(out, button{
				id:    "work.cell." + strconv.FormatUint(uint64(p.ID), 10) + "." + strconv.Itoa(col),
				label: "",
				x:     float32(hx + col*workCellW),
				y:     float32(y),
				w:     float32(workCellW - 4),
				h:     float32(workRowH - 2),
			})
		}
	}
	return out
}

func workTypeForCol(col int) (command.WorkType, bool) {
	w, ok := workLive[col]
	return w.cmd, ok
}

func (a *App) drawWork(screen *ebiten.Image, sw, sh int) {
	if a.renderer == nil {
		return
	}
	r := workRect(sw, sh)
	drawPanel(screen, r.x, r.y, r.w, r.h)
	drawLabel(screen, int(r.x)+12, int(r.y)+10, "Work  (Y = enabled; click a cell to toggle)")
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
		drawLabel(screen, int(r.x)+12, y, pawnLabel(p))
		for col := range workColumns {
			drawLabel(screen, hx+col*workCellW, y, workCellValue(p, col))
		}
	}
	mx, my := a.currentCursor()
	a.drawButtons(screen, a.workButtons(), mx, my)
}
