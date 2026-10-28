package ebiten

import (
	"fmt"
	"image/color"
	"math"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// Buildings' and blueprints' commands and panes (PLAY-018: BUILD-INSPECT
// owns them). A building's pane shows its health, fuel and feed bars and
// its status lines; the Bills tab (benches) and the Pen tab (pen markers)
// open above the pane; Move, Copy, Deconstruct and the kind's own actions
// are command buttons. Every existing id keeps its handler
// (applyBuildingClick, applyFrameClick), so the commands carry no Run or
// Cmds; the trough filter is new and carries Options.

// buildingView is the read model of building id.
func (a *App) buildingView(id entity.EntityID) (game.BuildingView, bool) {
	if a.renderer == nil {
		return game.BuildingView{}, false
	}
	bs := a.shell.World().Buildings
	for i := range bs { // by index: the views are large
		if bs[i].ID == id {
			return bs[i], true
		}
	}
	return game.BuildingView{}, false
}

// frameView is the read model of blueprint id.
func (a *App) frameView(id entity.EntityID) (game.FrameView, bool) {
	if a.renderer == nil {
		return game.FrameView{}, false
	}
	fs := a.shell.World().Frames
	for i := range fs {
		if fs[i].ID == id {
			return fs[i], true
		}
	}
	return game.FrameView{}, false
}

// buildingCommands are building id's command buttons: the kind's own first
// (trough filter, bed owners, prisoner mark, casket, the power toggle and
// thermostat), then Move, Copy and Deconstruct. A steam vent is natural: nothing to do. A pen marker only
// comes down; its settings are the Pen tab.
func (a *App) buildingCommands(id entity.EntityID) []cmdButton {
	b, ok := a.buildingView(id)
	if !ok || b.Kind == steamVentKind {
		return nil
	}
	var out []cmdButton
	if !isPenMarker(b) {
		out = append(out, a.troughCommands(b)...)
		out = append(out, a.bedCommands(b)...)
		out = append(out, a.casketCommands(b)...)
		out = append(out, a.powerCommands(b)...) // cmds_power.go (PLAY-021)
		out = append(out, a.moveCommands(b)...)
	}
	if !b.Deconstruct {
		out = append(out, cmdButton{ID: "bldg.deconstruct", Label: "Deconstruct", Icon: "order_deconstruct",
			Desc: "Have a colonist take this building down."})
	}
	return out
}

// moveCommands are Move, Copy and, while a move is pending, Cancel move.
func (a *App) moveCommands(b game.BuildingView) []cmdButton {
	mv := cmdButton{ID: "bldg.move", Label: "Move", Icon: "move",
		Desc: "Pick a new spot; it is reinstalled there, keeping its bills, fuel and owners."}
	if !movableKind(b.Kind, b.Floor) {
		mv.Disabled, mv.Reason = true, "can't be moved"
	}
	cp := cmdButton{ID: "bldg.copy", Label: "Copy", Icon: "copy",
		Desc: "Open the Architect with this building's tool, material and facing."}
	if p := a.lockedBy("build:" + b.Kind); p != "" {
		cp.Disabled, cp.Reason = true, "needs research: "+p
	} else if !hasBuildTool(b.Kind) {
		cp.Disabled, cp.Reason = true, "not buildable"
	}
	out := []cmdButton{mv, cp}
	if b.MovingTo != nil {
		out = append(out, cmdButton{ID: "bldg.cancelmove", Label: "Cancel move", Icon: "order_cancel",
			Desc: "Leave it where it stands."})
	}
	return out
}

// buildingPane is building id's pane.
func (a *App) buildingPane(id entity.EntityID) paneSpec {
	b, ok := a.buildingView(id)
	if !ok {
		return paneSpec{}
	}
	p := paneSpec{Title: buildingTitle(b), Subtitle: fmt.Sprintf("tile (%d, %d)", b.X, b.Y), Close: "bldg.close"}
	if b.HPPct > 0 {
		p.Bars = append(p.Bars, paneBar{Label: "Health", Pct: bldgPct(b.HPPct), Color: healthBarColor(b.HPPct)})
	}
	if isPenMarker(b) {
		p.Title = "Pen marker"
		if pv, ok := a.penOf(b); ok {
			p.Lines = append(p.Lines, penStatusLine(pv))
			p.Lines = append(p.Lines, penFoodLines(pv)...)
			p.Tabs = append(p.Tabs, a.penTab(b))
		}
		return p
	}
	p.Bars = append(p.Bars, a.buildingBars(b)...)
	p.Lines = a.buildingLines(b)
	if b.Bench {
		p.Tabs = append(p.Tabs, a.billsTab(b))
	}
	return p
}

// buildingBars are the fuel, feed and battery charge bars. Any fuel
// burner has the Fuel bar: a campfire, a torch lamp, a wood-fired generator.
func (a *App) buildingBars(b game.BuildingView) []paneBar {
	var out []paneBar
	if b.Fuel {
		out = append(out, paneBar{Label: "Fuel", Pct: bldgPct(b.FuelPct), Color: uiWarning})
	}
	if isTrough(b) && b.TroughCap > 0 {
		out = append(out, paneBar{Label: "Feed", Pct: bldgPct(b.TroughFill * 100 / b.TroughCap), Color: uiAccent})
	}
	return append(out, powerBars(b)...)
}

// buildingLines are the status lines: what the legacy panel said, one per
// line.
func (a *App) buildingLines(b game.BuildingView) []string {
	var out []string
	add := func(ss ...string) {
		for _, s := range ss {
			if s != "" {
				out = append(out, s)
			}
		}
	}
	if b.Deconstruct {
		add("Marked for deconstruction")
	}
	w := a.shell.World()
	add(roomLabelAt(w, b.X, b.Y))
	add(fuelLine(b))
	add(turretLine(b))
	add(powerLines(b, w)...)
	if b.DryQty > 0 {
		add(fmt.Sprintf("Drying %d %s, %d%%", b.DryQty, b.DryKind, b.DryPct))
	}
	add(graveLines(b)...)
	add(ventLine(b), carpetLine(b), bridgeLine(b), swapLine(b), movingLine(b))
	add(a.bedLines(b)...)
	add(a.troughLines(b)...)
	return out
}

// bldgPct limits n to 0..100.
func bldgPct(n int) int { return max(0, min(100, n)) }

// healthBarColor is the building health colour: red under 35, amber under
// 70, else green (the map's damage bars use the same cuts).
func healthBarColor(pct int) color.RGBA {
	switch {
	case pct < 35:
		return uiDanger
	case pct < 70:
		return uiWarning
	}
	return uiGood
}

// ---- the blueprint -------------------------------------------------------

// frameCommands are blueprint id's command buttons: Cancel blueprint.
func (a *App) frameCommands(id entity.EntityID) []cmdButton {
	if _, ok := a.frameView(id); !ok {
		return nil
	}
	return []cmdButton{{ID: "frame.cancel", Label: "Cancel", Icon: "order_cancel",
		Desc: "Cancel this blueprint."}}
}

// framePane is blueprint id's pane: what is being built, its status,
// materials, work and who is on it.
func (a *App) framePane(id entity.EntityID) paneSpec {
	f, ok := a.frameView(id)
	if !ok {
		return paneSpec{}
	}
	in := buildInfo(f, a.shell.World())
	p := paneSpec{Title: in.Title, Subtitle: in.Foot, Close: "frame.close"}
	p.Lines = append(p.Lines, "Status: "+in.Status)
	if in.Material != "" {
		p.Bars = append(p.Bars, paneBar{Label: "Materials", Pct: bldgFracPct(in.MatFrac), Color: uiAccent})
		p.Lines = append(p.Lines, "Materials  "+in.Material)
		if in.Stock != "" {
			p.Lines = append(p.Lines, "In stock  "+in.Stock)
		}
	}
	if in.Work != "" {
		p.Bars = append(p.Bars, paneBar{Label: "Work", Pct: bldgFracPct(in.WorkFrac), Color: uiGood})
		p.Lines = append(p.Lines, "Work  "+in.Work)
	}
	p.Lines = append(p.Lines, in.People)
	return p
}

// bldgFracPct is a 0..1 share as a percentage.
func bldgFracPct(f float64) int { return bldgPct(int(math.Round(f * 100))) }

// ---- expanded tab bodies -------------------------------------------------

// A tab body is laid out once in content space (origin: the top-left of
// its scrolling view), then declared with the widget kit shifted by the
// scroll offset. Rows that are not wholly inside the view are skipped, so
// nothing is live or drawn outside the panel.

const (
	bodyGap       = 8  // between a tab's list and the button under it
	tabScrollStep = 52 // px per wheel notch
)

// tabLabel is a line of text in a tab body.
type tabLabel struct {
	text string
	x, y float32
	w    float32 // widest it may be (0: unbounded); longer text is cut
	st   textStyle
}

// tabFill is a filled rectangle in a tab body.
type tabFill struct {
	r rect
	c color.RGBA
}

// tabBody is a tab's content: buttons by legacy id, text, fills, and the
// content height.
type tabBody struct {
	buttons []button
	labels  []tabLabel
	fills   []tabFill
	h       float32
}

// declareTabBody puts body in a scroll view over outer and declares its
// widgets; click runs with the id of a clicked button.
func (a *App) declareTabBody(scrollID string, outer rect, body tabBody, click func(id string)) {
	area := a.uiScrollArea(scrollID, outer, body.h, scrollOpts{Step: tabScrollStep})
	ox, oy := area.View.X, area.View.Y-area.Off
	inView := func(y, h float32) bool { return oy+y >= area.View.Y && oy+y+h <= area.View.Bottom() }
	u := a.uiSt()
	for _, f := range body.fills {
		if !inView(f.r.Y, f.r.H) {
			continue
		}
		r, c := rect{X: ox + f.r.X, Y: oy + f.r.Y, W: f.r.W, H: f.r.H}, f.c
		u.queue(func(dst *ebiten.Image) { fillRect(dst, r, c) })
	}
	for _, l := range body.labels {
		if !inView(l.y, lineStep(l.st)) {
			continue
		}
		text := l.text
		if l.w > 0 {
			text = fitText(text, l.st, l.w)
		}
		a.uiLabel(ox+l.x, oy+l.y, text, l.st)
	}
	for _, bt := range body.buttons {
		if !inView(bt.y, bt.h) {
			continue
		}
		r := rect{X: ox + bt.x, Y: oy + bt.y, W: bt.w, H: bt.h}
		if a.uiButton(bt.id, r, bt.label, btnOpts{Selected: bt.selected, Disabled: bt.disabled, Reason: bt.reason, Tip: bt.tip}) {
			click(bt.id)
		}
	}
}
