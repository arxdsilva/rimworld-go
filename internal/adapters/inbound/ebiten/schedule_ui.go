package ebiten

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/pawn/schedule"
	"rimworld-go/internal/domain/sim/command"
)

// Schedule tab: one row per colonist (plus "Everyone") of 24 hour cells.
// Pick a brush (anything / work / recreation / sleep) and click or drag
// across cells to paint them.

const (
	schedCellW = 26
	schedRowH  = 24
	schedTop   = 78 // first row below the header and brushes
)

var schedBrushes = []schedule.Activity{schedule.Anything, schedule.Work, schedule.Recreation, schedule.Sleep}

func schedColor(a schedule.Activity) color.RGBA {
	switch a {
	case schedule.Work:
		return color.RGBA{196, 150, 70, 255}
	case schedule.Recreation:
		return color.RGBA{150, 100, 180, 255}
	case schedule.Sleep:
		return color.RGBA{70, 100, 170, 255}
	}
	return color.RGBA{110, 110, 104, 255}
}

func (a *App) scheduleButtons() []button {
	r := workRect(a.sw, a.sh)
	var out []button
	for i, b := range schedBrushes {
		out = append(out, button{id: "sched.brush." + strconv.Itoa(int(b)), label: b.String(),
			x: r.x + 12 + float32(i*118), y: r.y + 30, w: 112, h: 26, selected: a.schedBrush == b})
	}
	gx := r.x + 12 + workNameW
	for row, id := range a.scheduleRows() {
		y := r.y + schedTop + float32(row*schedRowH)
		if y > r.y+r.h-20 {
			break
		}
		for h := 0; h < 24; h++ {
			out = append(out, button{id: "sched.cell." + strconv.FormatUint(id, 10) + "." + strconv.Itoa(h),
				x: gx + float32(h*schedCellW), y: y, w: schedCellW - 2, h: schedRowH - 3})
		}
	}
	return out
}

// scheduleRows lists row owners: 0 (everyone) then each colonist.
func (a *App) scheduleRows() []uint64 {
	rows := []uint64{0}
	for _, p := range a.renderer.snapshot() {
		if p.Status != "dead" {
			rows = append(rows, uint64(p.ID))
		}
	}
	return rows
}

func (a *App) drawSchedule(screen *ebiten.Image, sw, sh int) {
	r := workRect(sw, sh)
	drawPanel(screen, r.x, r.y, r.w, r.h)
	drawLabel(screen, int(r.x)+12, int(r.y)+10, "Schedule  (pick an activity, then click or drag across the hours)")
	mx, my := a.currentCursor()
	for _, b := range a.scheduleButtons() {
		if !strings.HasPrefix(b.id, "sched.brush.") {
			continue
		}
		drawButton(screen, b, b.contains(mx, my))
		v, _ := strconv.Atoi(strings.TrimPrefix(b.id, "sched.brush."))
		vector.FillRect(screen, b.x+b.w-16, b.y+8, 10, 10, schedColor(schedule.Activity(v)), false)
	}
	gx := int(r.x) + 12 + workNameW
	for h := 0; h < 24; h += 3 {
		drawLabel(screen, gx+h*schedCellW+2, int(r.y)+schedTop-16, fmt.Sprintf("%02d", h))
	}
	byID := map[uint64]game.PawnView{}
	for _, p := range a.renderer.snapshot() {
		byID[uint64(p.ID)] = p
	}
	now := a.renderer.game.Date().Hour
	for row, id := range a.scheduleRows() {
		y := int(r.y) + schedTop + row*schedRowH
		if y > int(r.y+r.h)-20 {
			break
		}
		name := "Everyone"
		grid := a.renderer.world().Schedule
		if p, ok := byID[id]; ok {
			name, grid = pawnLabel(p), p.Schedule
		}
		drawLabel(screen, int(r.x)+12, y+4, name)
		for h := 0; h < 24; h++ {
			x := float32(gx + h*schedCellW)
			c := color.RGBA{60, 56, 50, 255}
			if grid != nil && h < len(grid) {
				c = schedColor(grid[h])
			}
			vector.FillRect(screen, x, float32(y), schedCellW-2, schedRowH-3, c, false)
			if h == now {
				vector.StrokeRect(screen, x, float32(y), schedCellW-2, schedRowH-3, 2, color.RGBA{250, 240, 200, 255}, false)
			}
		}
	}
}

// paintSchedule paints the clicked cell with the current brush.
func (a *App) paintSchedule(id string) {
	parts := strings.Split(strings.TrimPrefix(id, "sched.cell."), ".")
	if len(parts) != 2 || a.renderer == nil {
		return
	}
	pawn, err1 := strconv.ParseUint(parts[0], 10, 64)
	hour, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return
	}
	if pawn != 0 {
		for _, p := range a.renderer.snapshot() {
			if uint64(p.ID) == pawn && hour < len(p.Schedule) && p.Schedule[hour] == a.schedBrush {
				return // already painted: don't resend while dragging
			}
		}
	}
	a.renderer.enqueue(command.Command{Kind: command.SetSchedule, Pawn: pawn, Index: hour, Mode: uint8(a.schedBrush)})
}

// handleScheduleDrag keeps painting while the mouse is held over cells.
func (a *App) handleScheduleDrag() {
	if !a.workOpen || a.panelMode != "schedule" || a.renderer == nil || !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		return
	}
	mx, my := a.currentCursor()
	if b, ok := hitButton(a.scheduleButtons(), mx, my); ok && strings.HasPrefix(b.id, "sched.cell.") {
		if b.id != a.lastPainted {
			a.lastPainted = b.id
			a.paintSchedule(b.id)
		}
		return
	}
	a.lastPainted = ""
}
