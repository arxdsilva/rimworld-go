package ebiten

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/ids"
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

// The sleep hour's blue: no chrome token is this deep, and it has to stay
// apart from the cool panels around the grid and from the other activities.
var schedSleepBlue = color.RGBA{R: 0x3F, G: 0x5F, B: 0xA8, A: 0xFF}

// schedColor is an activity's cell colour: amber work, green recreation,
// blue sleep, slate for anything.
func schedColor(a schedule.Activity) color.RGBA {
	switch a {
	case schedule.Work:
		return uiWarning
	case schedule.Recreation:
		return uiGood
	case schedule.Sleep:
		return schedSleepBlue
	}
	return uiBtnHover
}

func (a *App) scheduleButtons() []button {
	r := a.windowRect()
	var out []button
	for i, b := range schedBrushes {
		out = append(out, button{id: ids.PrefixSchedBrush + strconv.Itoa(int(b)), label: b.String(),
			x: r.x + 12 + float32(i*118), y: r.y + 30, w: 112, h: 26, selected: a.schedBrush == b})
	}
	gx := r.x + 12 + workNameW
	for row, id := range a.scheduleRows() {
		y := r.y + schedTop + float32(row*schedRowH)
		if y > r.y+r.h-20 {
			break
		}
		for h := 0; h < 24; h++ {
			out = append(out, button{id: ids.PrefixSchedCell + strconv.FormatUint(id, 10) + "." + strconv.Itoa(h),
				x: gx + float32(h*schedCellW), y: y, w: schedCellW - 2, h: schedRowH - 3})
		}
	}
	return out
}

// scheduleRows lists row owners: 0 (everyone) then each colonist.
func (a *App) scheduleRows() []uint64 {
	rows := []uint64{0}
	for _, p := range a.shell.Pawns() {
		if p.Status != "dead" {
			rows = append(rows, uint64(p.ID))
		}
	}
	return rows
}

// scheduleWant is the Schedule window's wanted size: the name column, the
// 24 hour cells, the allowed-area column, and a row for everyone and each
// colonist.
func (a *App) scheduleWant() (float32, float32) {
	rows := 1
	if a.renderer != nil {
		rows = len(a.scheduleRows())
	}
	return float32(12 + workNameW + 24*schedCellW + schedAreaGap + schedAreaW + 12), float32(max(160, schedTop+rows*schedRowH+20))
}

func (a *App) drawSchedule(screen *ebiten.Image, sw, sh int) {
	r := a.windowRect()
	drawPanel(screen, r.x, r.y, r.w, r.h)
	drawText(screen, "Schedule", float64(r.x)+12, float64(r.y)+6, withColor(tsLabel, uiHeading))
	hintX := float64(r.x) + 12 + float64(textW("Schedule", tsLabel)) + 16
	drawText(screen, ellipsize("pick an activity, then click or drag across the hours", tsSmall, float64(r.x+r.w)-12-hintX),
		hintX, float64(r.y)+6+(lineH(tsLabel)-lineH(tsSmall))/2, withColor(tsSmall, uiTextDim))
	mx, my := a.currentCursor()
	for _, b := range a.scheduleButtons() {
		if !strings.HasPrefix(b.id, ids.PrefixSchedBrush) {
			continue
		}
		drawButton(screen, b, b.contains(mx, my))
		v, _ := strconv.Atoi(strings.TrimPrefix(b.id, ids.PrefixSchedBrush))
		uiFillRect(screen, b.x+b.w-16, b.y+8, 10, 10, schedColor(schedule.Activity(v)), false)
	}
	gx := int(r.x) + 12 + workNameW
	for h := 0; h < 24; h += 3 {
		drawText(screen, fmt.Sprintf("%02d", h), float64(gx+h*schedCellW+2), float64(int(r.y)+schedTop)-lineH(tsSmall)-1, withColor(tsSmall, uiTextDim))
	}
	byID := map[uint64]game.PawnView{}
	for _, p := range a.shell.Pawns() {
		byID[uint64(p.ID)] = p
	}
	now := a.shell.Date().Hour
	for row, id := range a.scheduleRows() {
		y := int(r.y) + schedTop + row*schedRowH
		if y > int(r.y+r.h)-20 {
			break
		}
		name := "Everyone"
		grid := a.shell.World().Schedule
		if p, ok := byID[id]; ok {
			name, grid = pawnLabel(p), p.Schedule
		}
		drawText(screen, ellipsize(name, tsBody, workNameW-8), float64(r.x)+12, float64(y)+(schedRowH-3-lineH(tsBody))/2, tsBody)
		for h := 0; h < 24; h++ {
			x := float32(gx + h*schedCellW)
			c := uiBtnDisabled
			if grid != nil && h < len(grid) {
				c = schedColor(grid[h])
			}
			uiFillRect(screen, x, float32(y), schedCellW-2, schedRowH-3, c, false)
			if h == now {
				uiStrokeRect(screen, x, float32(y), schedCellW-2, schedRowH-3, 2, uiText, false)
			}
		}
	}
}

// paintSchedule paints the clicked cell with the current brush.
func (a *App) paintSchedule(id string) {
	parts := strings.Split(strings.TrimPrefix(id, ids.PrefixSchedCell), ".")
	if len(parts) != 2 || a.renderer == nil {
		return
	}
	pawn, err1 := strconv.ParseUint(parts[0], 10, 64)
	hour, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return
	}
	if pawn != 0 {
		for _, p := range a.shell.Pawns() {
			if uint64(p.ID) == pawn && hour < len(p.Schedule) && p.Schedule[hour] == a.schedBrush {
				return // already painted: don't resend while dragging
			}
		}
	}
	a.shell.Enqueue(command.Command{Kind: command.SetSchedule, Pawn: pawn, Index: hour, Mode: uint8(a.schedBrush)})
}

// handleScheduleDrag keeps painting while the mouse is held over cells.
func (a *App) handleScheduleDrag() {
	if !a.workOpen || a.panelMode != "schedule" || a.renderer == nil || !a.in.Down(mbLeft) {
		return
	}
	mx, my := a.currentCursor()
	if b, ok := hitButton(a.scheduleButtons(), mx, my); ok && strings.HasPrefix(b.id, ids.PrefixSchedCell) {
		if b.id != a.lastPainted {
			a.lastPainted = b.id
			a.paintSchedule(b.id)
		}
		return
	}
	a.lastPainted = ""
}
