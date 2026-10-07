package ebiten

import (
	"image/color"
	"strconv"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

var workColumns = []string{"FF", "Doc", "Grow", "Cook", "Bld", "Hunt", "Haul", "Cln", "Rsh", "Mine", "Crft"}

// workLive maps live work columns (indexes into workColumns, which follow
// work.Type order) to their command and pawn flag.
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
	8:  {command.WorkResearch, func(p game.PawnView) bool { return p.WorkResearch }},
	9:  {command.WorkMine, func(p game.PawnView) bool { return p.WorkMine }},
	10: {command.WorkCraft, func(p game.PawnView) bool { return p.WorkCraft }},
}

// workPriority is the colonist's priority for a work column (0 = off).
func workPriority(p game.PawnView, col int) int {
	if col < len(p.Priorities) {
		return p.Priorities[col]
	}
	if w, ok := workLive[col]; ok && w.on(p) {
		return 3
	}
	return 0
}

// nextPriority cycles a cell: off -> 1 -> 2 -> 3 -> 4 -> off.
func nextPriority(v int) int {
	if v >= 4 {
		return 0
	}
	return v + 1
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
	if _, live := workLive[col]; !live {
		return ""
	}
	if v := workPriority(p, col); v > 0 {
		return strconv.Itoa(v)
	}
	return "-"
}

func (a *App) workButtons() []button {
	if a.renderer == nil || !a.workOpen {
		return nil
	}
	if a.panelMode == "schedule" {
		return a.scheduleButtons()
	}
	if a.panelMode == "research" {
		return a.researchButtons()
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
	if a.panelMode == "schedule" {
		a.drawSchedule(screen, sw, sh)
		return
	}
	if a.panelMode == "research" {
		a.drawResearch(screen, sw, sh)
		return
	}
	r := workRect(sw, sh)
	drawPanel(screen, r.x, r.y, r.w, r.h)
	drawLabel(screen, int(r.x)+12, int(r.y)+10, "Work priorities  (1 = do first ... 4 = do last, - = never; click a cell to change)")
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
	}
	mx, my := a.currentCursor()
	a.drawButtons(screen, a.workButtons(), mx, my)
	for row, p := range pawns {
		y := int(r.y) + 48 + row*workRowH
		if y > int(r.y+r.h)-20 {
			break
		}
		for col := range workColumns {
			v := workCellValue(p, col)
			if v == "" {
				continue
			}
			x := hx + col*workCellW
			if pr := workPriority(p, col); pr > 0 {
				// brighter for more urgent work
				vector.FillRect(screen, float32(x), float32(y), float32(workCellW-4), float32(workRowH-2),
					color.RGBA{uint8(150 - pr*22), uint8(120 - pr*16), 50, 255}, false)
			}
			drawLabel(screen, x+12, y+3, v)
		}
	}
}
