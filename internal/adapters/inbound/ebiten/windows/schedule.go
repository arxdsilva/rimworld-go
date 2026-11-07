package windows

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
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
	// selector declares right of the hours (UIS-062: a segmented strip 270
	// wide and a gap of 7; architect/area_select.go mirrors both).
	SchedAreaW   = hudlayout.SchedAreaW
	SchedAreaGap = hudlayout.SchedAreaGap
	// SchedCellW is the hour cell's pitch (a 19 box and 2 gap), SchedRowH the
	// row pitch (a 28 box and 2 gap), SchedTop the first row below the top
	// (UIS-062). The grid starts at schedGridX, which the Architect's area
	// selector (architect/area_select.go) mirrors as 12 + NameW.
	SchedCellW = 21
	SchedRowH  = 30
	SchedTop   = 70

	// SchedNameW is the name column left of the hours (UIS-062: the window is
	// 1011 wide at the reference's 1010); architect/area_select.go mirrors it.
	SchedNameW   = 200
	schedGridX   = 12 + SchedNameW
	schedBrushY  = hudlayout.WindowPad
	schedBottom  = 60
	schedCellBox = SchedCellW - 2
	schedRowBox  = SchedRowH - 2
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
			X: r.X + hudlayout.WindowPad + float32(i*118), Y: r.Y + schedBrushY, W: 112, H: 26, Selected: w.Brush == b})
	}
	gx := r.X + schedGridX
	for row, id := range w.Rows() {
		y := r.Y + SchedTop + float32(row*SchedRowH)
		if y+schedRowBox > r.Y+r.H-hudlayout.WindowPad {
			break
		}
		for h := 0; h < 24; h++ {
			out = append(out, uikit.Button{ID: ids.PrefixSchedCell + strconv.FormatUint(id, 10) + "." + strconv.Itoa(h),
				X: gx + float32(h*SchedCellW), Y: y, W: schedCellBox, H: schedRowBox})
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
	return float32(schedGridX + 24*SchedCellW + SchedAreaGap + SchedAreaW + hudlayout.WindowPad), float32(max(160, SchedTop+rows*SchedRowH+schedBottom))
}

func (w Schedule) draw(screen *ebiten.Image, sw, sh int) {
	r := w.Rect()
	pad := float64(hudlayout.WindowPad)
	uikit.DrawPanel(screen, r.X, r.Y, r.W, r.H)
	hintX := float64(r.X) + pad + 4*118 + 8
	gfx.Text(screen, gfx.Ellipsize(w.sh.T("pick an activity, then click or drag across the hours"), gfx.Small, float64(r.X+r.W)-pad-hintX),
		hintX, gfx.CenterTop(float64(r.Y)+pad, 26, gfx.Small), gfx.WithColor(gfx.Small, uikit.TextDim))
	mx, my := w.sh.Cursor()
	for _, b := range w.buttons() {
		if !strings.HasPrefix(b.ID, ids.PrefixSchedBrush) {
			continue
		}
		uikit.DrawButton(screen, b, b.Contains(mx, my))
		v, _ := strconv.Atoi(strings.TrimPrefix(b.ID, ids.PrefixSchedBrush))
		gfx.FillRect(screen, b.X+b.W-16, b.Y+8, 10, 10, schedColor(schedule.Activity(v)), false)
	}
	gx := int(r.X) + schedGridX
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
		if y+schedRowBox > int(r.Y+r.H)-hudlayout.WindowPad {
			break
		}
		name := w.sh.T("Everyone")
		grid := w.sh.World().Schedule
		if p, ok := byID[id]; ok {
			name, grid = w.sh.PawnLabel(p), p.Schedule
		}
		gfx.Text(screen, gfx.Ellipsize(name, gfx.Body, schedGridX-hudlayout.WindowPad-8), float64(r.X)+pad, gfx.CenterTop(float64(y), schedRowBox, gfx.Body), gfx.Body)
		for h := 0; h < 24; h++ {
			x := float32(gx + h*SchedCellW)
			c := uikit.BtnDisabled
			if grid != nil && h < len(grid) {
				c = schedColor(grid[h])
			}
			gfx.FillRect(screen, x, float32(y), schedCellBox, schedRowBox, c, false)
			if h == now {
				gfx.StrokeRect(screen, x, float32(y), schedCellBox, schedRowBox, 2, uikit.Text, false)
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
	if !w.Open || w.Mode != "schedule" || !w.live() {
		return
	}
	down := w.kit().Host().Frame().Down(inputframe.Left)
	cell := ""
	mx, my := w.sh.Cursor()
	if b, ok := uikit.HitButton(w.buttons(), mx, my); ok && strings.HasPrefix(b.ID, ids.PrefixSchedCell) {
		cell = b.ID
	}
	if id, ok := w.paint.Entered(down, cell); ok {
		w.Paint(id)
	}
}
