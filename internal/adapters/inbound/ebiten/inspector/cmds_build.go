package inspector

import (
	"fmt"
	"image/color"
	"math"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/render/env"
	"rimworld-go/internal/adapters/inbound/ebiten/render/overlay"
	"rimworld-go/internal/adapters/inbound/ebiten/render/world"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
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
func (a *Pane) BuildingView(id entity.EntityID) (game.BuildingView, bool) {
	if a.sim() == nil {
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
func (a *Pane) frameView(id entity.EntityID) (game.FrameView, bool) {
	if a.sim() == nil {
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
func (a Building) BuildingCommands(id entity.EntityID) []Button {
	b, ok := a.BuildingView(id)
	if !ok || b.Kind == env.SteamVentKind {
		return nil
	}
	var out []Button
	if !isPenMarker(b) {
		out = append(out, a.troughCommands(b)...)
		out = append(out, a.bedCommands(b)...)
		out = append(out, a.CasketCommands(b)...)
		out = append(out, a.defenceCommands(b)...) // cmds_defence.go (DEF-3)
		if forbiddable(b) {
			out = append(out, forbidCommand(b))
		}
		out = append(out, a.powerCommands(b)...) // cmds_power.go (PLAY-021)
		out = append(out, a.MoveCommands(b)...)
	}
	if !b.Deconstruct {
		out = append(out, Button{ID: "bldg.deconstruct", Label: "Deconstruct", Icon: "order_deconstruct",
			Desc: "Have a colonist take this building down."})
	}
	return out
}

// moveCommands are Move, Copy and, while a move is pending, Cancel move.
func (a Building) MoveCommands(b game.BuildingView) []Button {
	mv := Button{ID: "bldg.move", Label: "Move", Icon: "move",
		Desc: "Pick a new spot; it is reinstalled there, keeping its bills, fuel and owners."}
	if movable, _ := a.shell.MoveState(b); !movable {
		mv.Disabled, mv.Reason = true, "can't be moved"
	}
	cp := Button{ID: "bldg.copy", Label: "Copy", Icon: "copy",
		Desc: "Open the Architect with this building's tool, material and facing."}
	if p := a.shell.LockedBy("build:" + b.Kind); p != "" {
		cp.Disabled, cp.Reason = true, "needs research: "+p
	} else if !a.shell.Buildable(b.Kind) {
		cp.Disabled, cp.Reason = true, "not buildable"
	}
	out := []Button{mv, cp}
	if b.MovingTo != nil {
		out = append(out, Button{ID: "bldg.cancelmove", Label: "Cancel move", Icon: "order_cancel",
			Desc: "Leave it where it stands."})
	}
	return out
}

// buildingPane is building id's pane.
func (a Building) BuildingPane(id entity.EntityID) Spec {
	b, ok := a.BuildingView(id)
	if !ok {
		return Spec{}
	}
	p := Spec{Title: BuildingTitle(b), Subtitle: fmt.Sprintf("tile (%d, %d)", b.X, b.Y), Close: "bldg.close"}
	if b.HPPct > 0 {
		p.Bars = append(p.Bars, Bar{Label: "Health", Pct: bldgPct(b.HPPct), Color: healthBarColor(b.HPPct)})
	}
	if isPenMarker(b) {
		p.Title = "Pen marker"
		if pv, ok := a.PenOf(b); ok {
			p.Lines = append(p.Lines, PenStatusLine(pv))
			p.Lines = append(p.Lines, PenFoodLines(pv)...)
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
func (a Building) buildingBars(b game.BuildingView) []Bar {
	var out []Bar
	if b.Fuel {
		out = append(out, Bar{Label: "Fuel", Pct: bldgPct(b.FuelPct), Color: uikit.Warning})
	}
	if IsTrough(b) && b.TroughCap > 0 {
		out = append(out, Bar{Label: "Feed", Pct: bldgPct(b.TroughFill * 100 / b.TroughCap), Color: uikit.Accent})
	}
	return append(out, powerBars(b)...)
}

// buildingLines are the status lines: what the legacy panel said, one per
// line.
func (a Building) buildingLines(b game.BuildingView) []string {
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
	add(world.RoomLabelAt(w, b.X, b.Y))
	add(fuelLine(b))
	add(overlay.TurretLine(b))
	add(PowerLines(b, w)...)
	if b.DryQty > 0 {
		add(fmt.Sprintf("Drying %d %s, %d%%", b.DryQty, b.DryKind, b.DryPct))
	}
	add(GraveLines(b)...)
	add(env.VentLine(b), CarpetLine(b), overlay.BridgeLine(b), SwapLine(b))
	if _, moving := a.shell.MoveState(b); moving != "" {
		add(moving)
	}
	add(defenceLines(b)...) // cmds_defence.go (DEF-3, PRD-3)
	add(a.bedLines(b)...)
	add(bedStatLines(b)...)
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
		return uikit.Danger
	case pct < 70:
		return uikit.Warning
	}
	return uikit.Good
}

// ---- the blueprint -------------------------------------------------------

// frameCommands are blueprint id's command buttons: Cancel blueprint.
func (a Thing) FrameCommands(id entity.EntityID) []Button {
	if _, ok := a.frameView(id); !ok {
		return nil
	}
	return []Button{{ID: "frame.cancel", Label: "Cancel", Icon: "order_cancel",
		Desc: "Cancel this blueprint."}}
}

// framePane is blueprint id's pane: what is being built, its status,
// materials, work and who is on it.
func (a Thing) FramePane(id entity.EntityID) Spec {
	f, ok := a.frameView(id)
	if !ok {
		return Spec{}
	}
	in := BuildInfo(f, a.shell.World())
	p := Spec{Title: in.Title, Subtitle: in.Foot, Close: "frame.close"}
	p.Lines = append(p.Lines, "Status: "+in.Status)
	if in.Material != "" {
		p.Bars = append(p.Bars, Bar{Label: "Materials", Pct: bldgFracPct(in.MatFrac), Color: uikit.Accent})
		p.Lines = append(p.Lines, "Materials  "+in.Material)
		if in.Stock != "" {
			p.Lines = append(p.Lines, "In stock  "+in.Stock)
		}
	}
	if in.Work != "" {
		p.Bars = append(p.Bars, Bar{Label: "Work", Pct: bldgFracPct(in.WorkFrac), Color: uikit.Good})
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
	Text string
	x, y float32
	w    float32 // widest it may be (0: unbounded); longer text is cut
	st   gfx.TextStyle
}

// tabFill is a filled rectangle in a tab body.
type tabFill struct {
	r gfx.Rect
	c color.RGBA
}

// TabBody is a tab's content: buttons by legacy id, text, fills, and the
// content height.
type TabBody struct {
	Buttons []uikit.Button
	Labels  []tabLabel
	Fills   []tabFill
	H       float32
}

// declareTabBody puts body in a scroll view over outer and declares its
// widgets; click runs with the id of a clicked button.
func (a Building) declareTabBody(scrollID string, outer gfx.Rect, body TabBody, click func(id string)) {
	area := a.kit().ScrollArea(scrollID, outer, body.H, uikit.ScrollOpts{Step: tabScrollStep})
	ox, oy := area.View.X, area.View.Y-area.Off
	inView := func(y, h float32) bool { return oy+y >= area.View.Y && oy+y+h <= area.View.Bottom() }
	u := a.kit()
	for _, f := range body.Fills {
		if !inView(f.r.Y, f.r.H) {
			continue
		}
		r, c := gfx.Rect{X: ox + f.r.X, Y: oy + f.r.Y, W: f.r.W, H: f.r.H}, f.c
		u.Queue(func(dst *ebiten.Image) { uikit.FillRect(dst, r, c) })
	}
	for _, l := range body.Labels {
		if !inView(l.y, gfx.LineStep(l.st)) {
			continue
		}
		text := l.Text
		if l.w > 0 {
			text = uikit.FitText(text, l.st, l.w)
		}
		a.kit().Label(ox+l.x, oy+l.y, text, l.st)
	}
	for _, bt := range body.Buttons {
		if !inView(bt.Y, bt.H) {
			continue
		}
		r := gfx.Rect{X: ox + bt.X, Y: oy + bt.Y, W: bt.W, H: bt.H}
		if a.kit().Button(bt.ID, r, bt.Label, uikit.BtnOpts{Selected: bt.Selected, Disabled: bt.Disabled, Reason: bt.Reason, Tip: bt.Tip}) {
			click(bt.ID)
		}
	}
}
