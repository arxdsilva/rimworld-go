package ebiten

import (
	"strconv"
	"strings"

	"rimworld-go/internal/domain/pawn/drug"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// The Assign tab (MAIN_UI_SCREENS §21, ENG-003 M2): one row per living
// colonist with its drug policy. A click cycles the policy (no drugs,
// social drinking, unrestricted) and emits SetDrugPolicy; the window only
// reads the policy back. Food, apparel, medical and area columns come with
// their systems.

const (
	assignModalID = "colony.assign"
	assignW       = 560
	assignH       = 480
	assignRowH    = 30
	assignNameW   = 220
	assignHeadH   = 24
)

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
}

// assignRows lists the living colonists and their drug policies.
func (a *App) assignRows() []assignRow {
	rd, known := a.renderer.game.(drugPolicyReader)
	var out []assignRow
	for _, p := range a.renderer.snapshot() {
		if p.Status == "dead" {
			continue
		}
		r := assignRow{id: p.ID, name: pawnLabel(p), known: known}
		if known {
			r.policy = drug.Preset(rd.DrugPolicy(p.ID))
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
		br := rect{rr.X + assignNameW, rr.Y, min(200, rr.W-assignNameW), rr.H}
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
