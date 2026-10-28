package windows

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/pawn/schedule"
	"rimworld-go/internal/domain/sim/command"
)

// Schedule tab: one row per colonist (plus "Everyone") of 24 hour cells.
// Pick a brush (anything / work / recreation / sleep) and click or drag
// across cells to paint them.

const (
	// SchedAreaW and SchedAreaGap are the Allowed area column the area
	// selector declares right of the hours (its width, and its gap).
	SchedAreaW   = 170
	SchedAreaGap = 8
	SchedCellW   = 26
	SchedRowH    = 24
	SchedTop     = 78 // first row below the header and brushes
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
		return uikit.Warning
	case schedule.Recreation:
		return uikit.Good
	case schedule.Sleep:
		return schedSleepBlue
	}
	return uikit.BtnHover
}

func (w Schedule) buttons() []uikit.Button {
	r := w.Rect()
	var out []uikit.Button
	for i, b := range schedBrushes {
		out = append(out, uikit.Button{ID: ids.PrefixSchedBrush + strconv.Itoa(int(b)), Label: b.String(),
			X: r.X + 12 + float32(i*118), Y: r.Y + 30, W: 112, H: 26, Selected: w.Brush == b})
	}
	gx := r.X + 12 + WorkNameW
	for row, id := range w.Rows() {
		y := r.Y + SchedTop + float32(row*SchedRowH)
		if y > r.Y+r.H-20 {
			break
		}
		for h := 0; h < 24; h++ {
			out = append(out, uikit.Button{ID: ids.PrefixSchedCell + strconv.FormatUint(id, 10) + "." + strconv.Itoa(h),
				X: gx + float32(h*SchedCellW), Y: y, W: SchedCellW - 2, H: SchedRowH - 3})
		}
	}
	return out
}

// Rows lists row owners: 0 (everyone) then each colonist.
func (w Schedule) Rows() []uint64 {
	rows := []uint64{0}
	for _, p := range w.sh.Pawns() {
		if p.Status != "dead" {
			rows = append(rows, uint64(p.ID))
		}
	}
	return rows
}

// want is the Schedule window's wanted size: the name column, the
// 24 hour cells, the allowed-area column, and a row for everyone and each
// colonist.
func (w Schedule) want() (float32, float32) {
	rows := 1
	if w.live() {
		rows = len(w.Rows())
	}
	return float32(12 + WorkNameW + 24*SchedCellW + SchedAreaGap + SchedAreaW + 12), float32(max(160, SchedTop+rows*SchedRowH+20))
}

func (w Schedule) draw(screen *ebiten.Image, sw, sh int) {
	r := w.Rect()
	uikit.DrawPanel(screen, r.X, r.Y, r.W, r.H)
	gfx.Text(screen, "Schedule", float64(r.X)+12, float64(r.Y)+6, gfx.WithColor(gfx.Label, uikit.Heading))
	hintX := float64(r.X) + 12 + float64(gfx.TextW("Schedule", gfx.Label)) + 16
	gfx.Text(screen, gfx.Ellipsize("pick an activity, then click or drag across the hours", gfx.Small, float64(r.X+r.W)-12-hintX),
		hintX, float64(r.Y)+6+(gfx.LineH(gfx.Label)-gfx.LineH(gfx.Small))/2, gfx.WithColor(gfx.Small, uikit.TextDim))
	mx, my := w.sh.Cursor()
	for _, b := range w.buttons() {
		if !strings.HasPrefix(b.ID, ids.PrefixSchedBrush) {
			continue
		}
		uikit.DrawButton(screen, b, b.Contains(mx, my))
		v, _ := strconv.Atoi(strings.TrimPrefix(b.ID, ids.PrefixSchedBrush))
		gfx.FillRect(screen, b.X+b.W-16, b.Y+8, 10, 10, schedColor(schedule.Activity(v)), false)
	}
	gx := int(r.X) + 12 + WorkNameW
	for h := 0; h < 24; h += 3 {
		gfx.Text(screen, fmt.Sprintf("%02d", h), float64(gx+h*SchedCellW+2), float64(int(r.Y)+SchedTop)-gfx.LineH(gfx.Small)-1, gfx.WithColor(gfx.Small, uikit.TextDim))
	}
	byID := map[uint64]game.PawnView{}
	for _, p := range w.sh.Pawns() {
		byID[uint64(p.ID)] = p
	}
	now := w.sh.Date().Hour
	for row, id := range w.Rows() {
		y := int(r.Y) + SchedTop + row*SchedRowH
		if y > int(r.Y+r.H)-20 {
			break
		}
		name := "Everyone"
		grid := w.sh.World().Schedule
		if p, ok := byID[id]; ok {
			name, grid = w.sh.PawnLabel(p), p.Schedule
		}
		gfx.Text(screen, gfx.Ellipsize(name, gfx.Body, WorkNameW-8), float64(r.X)+12, float64(y)+(SchedRowH-3-gfx.LineH(gfx.Body))/2, gfx.Body)
		for h := 0; h < 24; h++ {
			x := float32(gx + h*SchedCellW)
			c := uikit.BtnDisabled
			if grid != nil && h < len(grid) {
				c = schedColor(grid[h])
			}
			gfx.FillRect(screen, x, float32(y), SchedCellW-2, SchedRowH-3, c, false)
			if h == now {
				gfx.StrokeRect(screen, x, float32(y), SchedCellW-2, SchedRowH-3, 2, uikit.Text, false)
			}
		}
	}
}

// Paint paints the clicked cell with the current brush.
func (w Schedule) Paint(id string) {
	parts := strings.Split(strings.TrimPrefix(id, ids.PrefixSchedCell), ".")
	if len(parts) != 2 || !w.live() {
		return
	}
	pawn, err1 := strconv.ParseUint(parts[0], 10, 64)
	hour, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return
	}
	if pawn != 0 {
		for _, p := range w.sh.Pawns() {
			if uint64(p.ID) == pawn && hour < len(p.Schedule) && p.Schedule[hour] == w.Brush {
				return // already painted: don't resend while dragging
			}
		}
	}
	w.sh.Enqueue(command.Command{Kind: command.SetSchedule, Pawn: pawn, Index: hour, Mode: uint8(w.Brush)})
}

// Drag keeps painting while the mouse is held over cells.
func (w Schedule) Drag() {
	if !w.Open || w.Mode != "schedule" || !w.live() || !w.kit().Host().Frame().Down(inputframe.Left) {
		return
	}
	mx, my := w.sh.Cursor()
	if b, ok := uikit.HitButton(w.buttons(), mx, my); ok && strings.HasPrefix(b.ID, ids.PrefixSchedCell) {
		if b.ID != w.lastPainted {
			w.lastPainted = b.ID
			w.Paint(b.ID)
		}
		return
	}
	w.lastPainted = ""
}
