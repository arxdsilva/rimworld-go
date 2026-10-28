package ebiten

import (
	"image/color"
	"strconv"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

// Area pickers (PLAY-015 §1.3): the Architect's "Expand/Clear allowed
// area" submenus and the allowed-area dropdowns of the Schedule tab, the
// pawn inspector and the Animals tab. All are one small centred list
// (a modal) of areas with colour swatches and a "Manage allowed areas..."
// row. An area is named by its stable id (AreaView.ID); a pawn's
// assignment is a ref: command.AreaUnrestricted, command.AreaHome or an id.

const (
	areaPickID   = "colony.areapick"
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

// areaRefLabel is the dropdown text for a ref: "Unrestricted", "Home" or
// the area's name (a ref to a vanished area reads Unrestricted).
func areaRefLabel(areas []game.AreaView, ref int) string {
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

// areaTint is area ar's colour (opaque); an uncoloured area is grey.
func areaTint(ar game.AreaView) color.RGBA {
	c := ar.Color
	if c[0]|c[1]|c[2] == 0 {
		return color.RGBA{150, 150, 150, 255}
	}
	return color.RGBA{c[0], c[1], c[2], 255}
}

// areaPickOpt is one row of a picker.
type areaPickOpt struct {
	ref    int
	label  string
	swatch color.RGBA
	has    bool // draw the swatch
}

// areaPickOpts lists the rows: with base, Unrestricted and Home first;
// then every area.
func areaPickOpts(areas []game.AreaView, base bool) []areaPickOpt {
	var out []areaPickOpt
	if base {
		out = append(out, areaPickOpt{ref: command.AreaUnrestricted, label: "Unrestricted"},
			areaPickOpt{ref: command.AreaHome, label: "Home"})
	}
	for _, ar := range areas {
		out = append(out, areaPickOpt{ref: int(ar.ID), label: ar.Name, swatch: areaTint(ar), has: true})
	}
	return out
}

// openAreaPicker opens the list; pick gets the chosen ref (the modal is
// closed first). base adds the Unrestricted and Home rows; cur marks the
// current choice.
func (a *App) openAreaPicker(title string, cur int, base bool, pick func(ref int)) {
	if a.renderer == nil {
		return
	}
	reset := func() { a.renderer.overlays().Area.Hover = 0 }
	a.uiPushModal(modal{
		ID: areaPickID, Title: title, W: areaPickW, H: 460,
		Draw: func(a *App, body rect) { a.areaPickBody(body, cur, base, pick) },
		Buttons: []modalButton{
			{ID: "close", Label: "Close", Style: btnPrimary, Closes: true, OnClick: reset},
		},
		OnClose: reset,
	})
}

// areaPickBody declares the rows (hovering an area row previews it on the
// map) and the Manage row.
func (a *App) areaPickBody(body rect, cur int, base bool, pick func(ref int)) {
	areas := a.renderer.game.Areas()
	opts := areaPickOpts(areas, base)
	list, foot := body.SplitV(-areaPickRowH - 8)
	if len(opts) == 0 {
		a.uiLabel(list.X, list.Y+4, areasEmpty, dimText())
	}
	sa := a.uiScrollArea("areapick.list", list, float32(len(opts)*areaPickRowH), scrollOpts{Step: areaPickRowH * 2})
	y := sa.View.Y - sa.Off
	hover := 0
	cur = areaRef(cur)
	for _, o := range opts {
		rr := rect{X: sa.View.X, Y: y, W: sa.View.W - 2, H: areaPickRowH - 4}
		y += areaPickRowH
		if rr.Y < sa.View.Y || rr.Bottom() > sa.View.Bottom() {
			continue
		}
		id := "areapick.row." + strconv.Itoa(o.ref)
		if a.uiButton(id, rr, o.label, btnOpts{Selected: o.ref == cur}) {
			a.uiCloseModal(areaPickID)
			a.renderer.overlays().Area.Hover = 0
			pick(o.ref)
			return
		}
		if o.has {
			sw := o.swatch
			a.uiSt().Queue(func(dst *ebiten.Image) {
				uiFillRect(dst, rr.X+8, rr.Y+(rr.H-areaSwatchW)/2, areaSwatchW, areaSwatchW, sw, false)
				uiStrokeRect(dst, rr.X+8, rr.Y+(rr.H-areaSwatchW)/2, areaSwatchW, areaSwatchW, 1, uiPanelBorder, false)
			})
			if a.uiHot(id) && o.ref > 0 {
				hover = o.ref
			}
		}
	}
	a.renderer.overlays().Area.Hover = hover
	if a.uiButton("areapick.manage", foot.Inset(0), areaManage, btnOpts{}) {
		a.uiCloseModal(areaPickID)
		a.renderer.overlays().Area.Hover = 0
		a.openAreaManager()
	}
}

// areaDropdown is the allowed-area selector shared by the Schedule tab,
// the pawn inspector and the Animals tab: a button in r showing ref's name
// that opens the picker; pick gets the chosen ref (emit AssignArea from
// it). id is the button's widget id.
func (a *App) areaDropdown(id string, r rect, ref int, pick func(ref int)) {
	if a.renderer == nil {
		return
	}
	areas := a.renderer.game.Areas()
	label := areaRefLabel(areas, ref)
	if a.uiButton(id, r, label+"  v", btnOpts{Tip: assignAreaTip}) {
		a.openAreaPicker("Allowed area", ref, true, pick)
	}
	if ar, ok := areaByID(areas, areaRef(ref)); ok {
		sw := areaTint(ar)
		a.uiSt().Queue(func(dst *ebiten.Image) {
			uiFillRect(dst, r.X+5, r.Y+(r.H-10)/2, 10, 10, sw, false)
		})
	}
}

const assignAreaTip = "Allowed area\n" +
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

// areaToolPick is selectArchTool's hook: for an allowed-area tool it opens
// the submenu instead of arming at once. It reports it took the click.
func (a *App) areaToolPick(id string) bool {
	paint, ok := areaToolPaints(id)
	if !ok || a.renderer == nil {
		return false
	}
	if a.archTool == id {
		a.archTool = "" // clicking the armed card toggles it off
		a.syncRoofForce()
		return true
	}
	title := "Clear allowed area"
	if paint {
		title = "Expand allowed area"
	}
	a.openAreaPicker(title, 0, false, func(ref int) { a.armAreaTool(id, ref) })
	return true
}

// armAreaTool arms Architect tool id (an allowed-area tool) on area ref.
func (a *App) armAreaTool(id string, ref int) {
	a.archCat, a.archTool, a.archDrag, a.archStyle = "zones", id, false, styleDefault
	a.setAreaPaint(ref)
	a.syncRoofForce()
}
