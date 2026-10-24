package ebiten

import (
	"strconv"
	"strings"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/pawn/drug"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// The Assign tab (MAIN_UI_SCREENS §21, ENG-003 M2): one row per living
// colonist with its drug policy. A click cycles the policy (no drugs,
// social drinking, unrestricted) and emits SetDrugPolicy; the window only
// reads the policy back. The Area column cycles the allowed area
// (AssignArea). Food, apparel and medical columns come with their systems.

const (
	assignModalID = "colony.assign"
	assignW       = 720
	assignH       = 480
	assignRowH    = 30
	assignNameW   = 220
	assignHeadH   = 24
	assignDrugW   = 200
	assignAreaW   = 160
)

const assignAreaTip = "Allowed area\n" +
	"Where the colonist may go: work, meals and free time outside it are left alone. " +
	"Drafted colonists, direct orders, sleep and medical care ignore it. A click cycles the areas."

// assignPresets is the cycle order of the drug policy button.
var assignPresets = []drug.Preset{drug.NoDrugs, drug.SocialDrinking, drug.Unrestricted}

const assignDrugTip = "Drug policy\n" +
	"No drugs: never takes any, even when hooked.\n" +
	"Social drinking: a beer now and then when bored.\n" +
	"Unrestricted: any recreational drug when bored.\n" +
	"A colonist hooked on a drug seeks it when the craving starts, unless set to no drugs. Abstainers never take drugs for fun."

// drugPolicyReader is the read model the tab needs (consumer-owned; the
// game implements it). A simulation without it shows the column disabled.
type drugPolicyReader interface {
	DrugPolicy(id entity.EntityID) uint8
}

// assignTabUI makes the HUD's Assign tab open the window: the legacy button
// is drawn by the HUD, this widget answers its click.
func (a *App) assignTabUI(sw, sh int) {
	for _, b := range a.hudTabButtons(sw, sh) {
		if b.id != "tab.assign" || b.disabled {
			continue
		}
		a.uiSt().add(uiWidget{id: b.id, label: b.label, r: rect{b.x, b.y, b.w, b.h}, click: a.toggleAssign})
	}
}

// toggleAssign opens the window, or closes it when it is on top.
func (a *App) toggleAssign() {
	if a.uiModalTop() == assignModalID {
		a.uiCloseTop()
		return
	}
	a.openAssign()
}

// openAssign opens the Assign window.
func (a *App) openAssign() {
	if a.renderer == nil {
		return
	}
	a.uiPushModal(modal{
		id: assignModalID, title: "Assign", w: assignW, h: assignH,
		draw:    func(a *App, body rect) { a.assignBody(body) },
		buttons: []modalButton{{id: "close", label: "Close", style: btnPrimary, closes: true}},
	})
}

// assignRow is one colonist's row.
type assignRow struct {
	id     entity.EntityID
	name   string
	policy drug.Preset
	known  bool // the simulation reports policies
	area   int  // allowed area, -1 unrestricted
	areaOK bool // the simulation reports areas
}

// assignRows lists the living colonists and their drug policies (and
// allowed areas).
func (a *App) assignRows() []assignRow {
	rd, known := a.renderer.game.(drugPolicyReader)
	ar, areaOK := a.renderer.game.(areaReader)
	var out []assignRow
	for _, p := range a.renderer.snapshot() {
		if p.Status == "dead" {
			continue
		}
		r := assignRow{id: p.ID, name: pawnLabel(p), known: known}
		if known {
			r.policy = drug.Preset(rd.DrugPolicy(p.ID))
		}
		r.area, r.areaOK = -1, areaOK
		if areaOK {
			r.area = ar.AreaOf(p.ID)
		}
		out = append(out, r)
	}
	return out
}

// presetLabel is the policy's button label ("Social drinking").
func presetLabel(p drug.Preset) string {
	l := p.Label()
	if l == "" {
		return l
	}
	return strings.ToUpper(l[:1]) + l[1:]
}

// nextPreset is the policy after p in the cycle.
func nextPreset(p drug.Preset) drug.Preset {
	for i, q := range assignPresets {
		if q == p {
			return assignPresets[(i+1)%len(assignPresets)]
		}
	}
	return drug.SocialDrinking
}

// assignBody declares the header and the visible rows (the list scrolls;
// only rows in view get widgets).
func (a *App) assignBody(body rect) {
	if a.renderer == nil {
		return
	}
	head, rest := body.splitV(assignHeadH)
	hs := withColor(tsLabel, uiHeading)
	a.uiLabel(head.X, head.Y, "Colonist", hs)
	a.uiLabel(head.X+assignNameW, head.Y, "Drugs", hs)
	a.uiTip("assign.head.drugs", rect{head.X + assignNameW, head.Y, 120, head.H}, assignDrugTip)
	areaX := head.X + assignNameW + assignDrugW + 12
	a.uiLabel(areaX, head.Y, "Area", hs)
	a.uiTip("assign.head.area", rect{areaX, head.Y, 60, head.H}, assignAreaTip)
	if a.uiButton("assign.areas", rect{head.right() - 120, head.Y - 2, 120, head.H}, "Manage areas",
		btnOpts{Tip: "Allowed areas\nMake, rename and pick the areas the Architect paints."}) {
		a.openAreas() // areas_ui.go
	}
	areas := a.renderer.game.Areas()
	rows := a.assignRows()
	if len(rows) == 0 {
		a.uiLabel(rest.X, rest.Y+6, "No colonists.", dimText())
		return
	}
	area := a.uiScrollArea("assign.rows", rest, float32(len(rows)*assignRowH), scrollOpts{Step: assignRowH * 2})
	y := area.View.Y - area.Off
	for _, r := range rows {
		rr := rect{area.View.X, y, area.View.W, assignRowH - 4}
		y += assignRowH
		if rr.Y < area.View.Y || rr.bottom() > area.View.bottom() {
			continue // outside the view: no widgets
		}
		a.uiLabel(rr.X, rr.Y+(rr.H-float32(lineH(tsBody)))/2, ellipsize(r.name, tsBody, assignNameW-12), tsBody)
		a.assignAreaButton(r, rect{rr.X + assignNameW + assignDrugW + 12, rr.Y, assignAreaW, rr.H}, areas)
		br := rect{rr.X + assignNameW, rr.Y, min(assignDrugW, rr.W-assignNameW), rr.H}
		id := assignDrugID(r.id)
		if !r.known {
			a.uiButton(id, br, "-", btnOpts{Disabled: true, Reason: "Drug policies are not available in this game."})
			continue
		}
		if a.uiButton(id, br, presetLabel(r.policy), btnOpts{Tip: assignDrugTip}) {
			a.renderer.enqueue(command.Command{Kind: command.SetDrugPolicy, Pawn: uint64(r.id), Mode: uint8(nextPreset(r.policy))})
		}
	}
}

// assignDrugID is a colonist's drug policy button id.
func assignDrugID(id entity.EntityID) string {
	return "assign.drug." + strconv.FormatUint(uint64(id), 10)
}

// assignAreaButton is a colonist's allowed-area button: a click cycles
// unrestricted and the areas, emitting AssignArea.
func (a *App) assignAreaButton(r assignRow, br rect, areas []game.AreaView) {
	id := "assign.area." + strconv.FormatUint(uint64(r.id), 10)
	switch {
	case !r.areaOK:
		a.uiButton(id, br, "-", btnOpts{Disabled: true, Reason: "Allowed areas are not available in this game."})
	case len(areas) == 0:
		a.uiButton(id, br, "Unrestricted", btnOpts{Disabled: true, Reason: areasEmpty})
	default:
		if a.uiButton(id, br, ellipsize(areaLabel(areas, r.area), tsBody, float64(br.W-12)), btnOpts{Tip: assignAreaTip}) {
			a.renderer.enqueue(command.Command{Kind: command.AssignArea, Pawn: uint64(r.id), Index: nextArea(areas, r.area)})
		}
	}
}
