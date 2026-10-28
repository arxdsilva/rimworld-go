package architect

import (
	"image/color"
	"strconv"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

// Area pickers (PLAY-015 §1.3): the Architect's "Expand/Clear allowed
// area" submenus and the allowed-area dropdowns of the Schedule tab, the
// pawn inspector and the Animals tab. All are one small centred list
// (a modal) of areas with colour swatches and a "Manage allowed areas..."
// row. An area is named by its stable id (AreaView.ID); a pawn's
// assignment is a ref: command.AreaUnrestricted, command.AreaHome or an id.

// Areas is the allowed areas' side of the Architect: the pickers, the
// manager, the area tools' armed area and the selectors legacy panels
// host.
type Areas struct {
	*Tool
	paint int // the area id the Architect's area tools paint
}

const (
	// AreaPickID is the area picker's modal id.
	AreaPickID   = "colony.areapick"
	areaPickW    = 380
	areaPickRowH = 30
	areaSwatchW  = 14
	areaManage   = "Manage allowed areas..."
)

// areaRef normalises a read-model ref: 0 (never an id) is unrestricted.
func areaRef(ref int) int {
	if ref == 0 {
		return command.AreaUnrestricted
	}
	return ref
}

// areaByID finds the area with id.
func areaByID(areas []game.AreaView, id int) (game.AreaView, bool) {
	for _, ar := range areas {
		if int(ar.ID) == id {
			return ar, true
		}
	}
	return game.AreaView{}, false
}

// AreaRefLabel is the dropdown text for a ref: "Unrestricted", "Home" or
// the area's name (a ref to a vanished area reads Unrestricted).
func AreaRefLabel(areas []game.AreaView, ref int) string {
	switch ref = areaRef(ref); ref {
	case command.AreaHome:
		return "Home"
	case command.AreaUnrestricted:
		return "Unrestricted"
	}
	if ar, ok := areaByID(areas, ref); ok {
		return ar.Name
	}
	return "Unrestricted"
}

// AreaTint is area ar's colour (opaque); an uncoloured area is grey.
func AreaTint(ar game.AreaView) color.RGBA {
	c := ar.Color
	if c[0]|c[1]|c[2] == 0 {
		return color.RGBA{150, 150, 150, 255}
	}
	return color.RGBA{c[0], c[1], c[2], 255}
}

// AreaPickOpt is one row of a picker.
type AreaPickOpt struct {
	Ref    int
	Label  string
	Swatch color.RGBA
	Has    bool // draw the swatch
}

// AreaPickOpts lists the rows: with base, Unrestricted and Home first;
// then every area.
func AreaPickOpts(areas []game.AreaView, base bool) []AreaPickOpt {
	var out []AreaPickOpt
	if base {
		out = append(out, AreaPickOpt{Ref: command.AreaUnrestricted, Label: "Unrestricted"},
			AreaPickOpt{Ref: command.AreaHome, Label: "Home"})
	}
	for _, ar := range areas {
		out = append(out, AreaPickOpt{Ref: int(ar.ID), Label: ar.Name, Swatch: AreaTint(ar), Has: true})
	}
	return out
}

// OpenPicker opens the list; pick gets the chosen ref (the modal is
// closed first). base adds the Unrestricted and Home rows; cur marks the
// current choice.
func (a *Areas) OpenPicker(title string, cur int, base bool, pick func(ref int)) {
	if a.sim() == nil {
		return
	}
	reset := func() { a.sh.Overlays().Area.Hover = 0 }
	a.sh.Kit().Push(uikit.Modal{
		ID: AreaPickID, Title: title, W: areaPickW, H: 460,
		Draw: func(body gfx.Rect) { a.pickBody(body, cur, base, pick) },
		Buttons: []uikit.ModalButton{
			{ID: "close", Label: "Close", Style: uikit.BtnPrimary, Closes: true, OnClick: reset},
		},
		OnClose: reset,
	})
}

// pickBody declares the rows (hovering an area row previews it on the
// map) and the Manage row.
func (a *Areas) pickBody(body gfx.Rect, cur int, base bool, pick func(ref int)) {
	k := a.sh.Kit()
	ov := a.sh.Overlays()
	opts := AreaPickOpts(a.sim().Areas(), base)
	list, foot := body.SplitV(-areaPickRowH - 8)
	if len(opts) == 0 {
		k.Label(list.X, list.Y+4, areasEmpty, uikit.DimText())
	}
	sa := k.ScrollArea("areapick.list", list, float32(len(opts)*areaPickRowH), uikit.ScrollOpts{Step: areaPickRowH * 2})
	y := sa.View.Y - sa.Off
	hover := 0
	cur = areaRef(cur)
	for _, o := range opts {
		rr := gfx.Rect{X: sa.View.X, Y: y, W: sa.View.W - 2, H: areaPickRowH - 4}
		y += areaPickRowH
		if rr.Y < sa.View.Y || rr.Bottom() > sa.View.Bottom() {
			continue
		}
		id := "areapick.row." + strconv.Itoa(o.Ref)
		if k.Button(id, rr, o.Label, uikit.BtnOpts{Selected: o.Ref == cur}) {
			k.CloseModal(AreaPickID)
			ov.Area.Hover = 0
			pick(o.Ref)
			return
		}
		if o.Has {
			sw := o.Swatch
			k.Queue(func(dst *ebiten.Image) {
				gfx.FillRect(dst, rr.X+8, rr.Y+(rr.H-areaSwatchW)/2, areaSwatchW, areaSwatchW, sw, false)
				gfx.StrokeRect(dst, rr.X+8, rr.Y+(rr.H-areaSwatchW)/2, areaSwatchW, areaSwatchW, 1, uikit.PanelBorder, false)
			})
			if k.IsHot(id) && o.Ref > 0 {
				hover = o.Ref
			}
		}
	}
	ov.Area.Hover = hover
	if k.Button("areapick.manage", foot.Inset(0), areaManage, uikit.BtnOpts{}) {
		k.CloseModal(AreaPickID)
		ov.Area.Hover = 0
		a.OpenManager()
	}
}

// Dropdown is the allowed-area selector shared by the Schedule tab, the
// pawn inspector and the Animals tab: a button in r showing ref's name
// that opens the picker; pick gets the chosen ref (emit AssignArea from
// it). id is the button's widget id.
func (a *Areas) Dropdown(id string, r gfx.Rect, ref int, pick func(ref int)) {
	sim := a.sim()
	if sim == nil {
		return
	}
	k := a.sh.Kit()
	areas := sim.Areas()
	label := AreaRefLabel(areas, ref)
	if k.Button(id, r, label+"  v", uikit.BtnOpts{Tip: AssignAreaTip}) {
		a.OpenPicker("Allowed area", ref, true, pick)
	}
	if ar, ok := areaByID(areas, areaRef(ref)); ok {
		sw := AreaTint(ar)
		k.Queue(func(dst *ebiten.Image) {
			gfx.FillRect(dst, r.X+5, r.Y+(r.H-10)/2, 10, 10, sw, false)
		})
	}
}

// AssignAreaTip is the allowed-area selector's hover card.
const AssignAreaTip = "Allowed area\n" +
	"Where it may go: Unrestricted, Home, or a painted area. Outside it, work and wandering are avoided. " +
	"Drafted colonists and direct orders ignore it."

// Architect submenus: the zone cards "Expand allowed area" and "Clear
// allowed area" open the list of areas; choosing one arms the tool for it.

// areaToolPaints reports whether id is an allowed-area tool, and whether it
// paints (else it clears).
func areaToolPaints(id string) (paint, ok bool) {
	switch id {
	case "allowed_area":
		return true, true
	case "allowed_area_clear":
		return false, true
	}
	return false, false
}

// toolPick is SelectTool's hook: for an allowed-area tool it opens the
// submenu instead of arming at once. It reports it took the click.
func (a *Areas) toolPick(id string) bool {
	paint, ok := areaToolPaints(id)
	if !ok || a.sim() == nil {
		return false
	}
	if a.ToolID == id {
		a.ToolID = "" // clicking the armed card toggles it off
		a.sh.SyncOverlays()
		return true
	}
	title := "Clear allowed area"
	if paint {
		title = "Expand allowed area"
	}
	a.OpenPicker(title, 0, false, func(ref int) { a.arm(id, ref) })
	return true
}

// arm arms Architect tool id (an allowed-area tool) on area ref.
func (a *Areas) arm(id string, ref int) {
	a.Cat, a.ToolID, a.Drag, a.Style = "zones", id, false, StyleDefault
	a.paint = ref
	a.sh.SyncOverlays()
}

// PaintIndex is the id of the area the Architect's area tools paint now:
// the one chosen in the submenu.
func (a *Areas) PaintIndex() int { return a.paint }

// SyncOverlay points the area overlay at the area the armed Architect
// tool paints (none when no area tool is armed); the pickers set hover.
func (a *Areas) SyncOverlay() {
	if a.sim() == nil {
		return
	}
	id := 0
	if _, ok := areaToolPaints(a.ToolID); ok && a.Open {
		id = a.paint
	}
	a.sh.Overlays().Area.Armed = id
}
