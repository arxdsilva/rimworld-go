package ebiten

import (
	"strconv"
	"strings"

	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/domain/pawn/drug"
	"rimworld-go/internal/domain/pawn/medcare"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// The Assign tab (MAIN_UI_SCREENS §21, ENG-003 M2): one row per living
// colonist with its drug policy and its medical care level (MED-03). A
// click cycles the policy (no drugs, social drinking, unrestricted) and
// emits SetDrugPolicy, or the care level (SetMedicalCare); the window only
// reads them back. Under the rows, the default care level of each group
// (colonists, prisoners, guests, animals) cycles the same way and emits
// SetDefaultCare. Food and apparel columns come with their systems.

const (
	assignModalID = "colony.assign"
	assignW       = 720
	assignH       = 480
	assignRowH    = 30
	assignNameW   = 220
	assignHeadH   = 24
	assignDrugW   = 200
	assignCareW   = 170 // MED-03: the medical care column
)

const assignCareTip = "Medical care\n" +
	"How much care this colonist gets: no care, care without medicine, herbal or less, industrial or less, or the best medicine there is. A click picks the next level."

// defaultCareReader is the read model of the default care row
// (consumer-owned; the game implements it): each group's default care
// level, in medcare.Group order. A simulation without it shows the row
// disabled.
type defaultCareReader interface {
	DefaultCare() [medcare.NumGroups]uint8
}

// nextCare is the care level after p, wrapping from the best to none.
func nextCare(p medcare.Policy) medcare.Policy { return (p + 1) % medcare.NumPolicies }

// assignCareID is a colonist's care button id; assignDefCareID group g's
// default care button id.
func assignCareID(id entity.EntityID) string {
	return "assign.care." + strconv.FormatUint(uint64(id), 10)
}

func assignDefCareID(g medcare.Group) string { return "assign.defcare." + strconv.Itoa(int(g)) }

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
		if b.id != ids.TabAssign || b.disabled {
			continue
		}
		a.uiSt().add(uiWidget{id: b.id, label: b.label, r: rect{X: b.x, Y: b.y, W: b.w, H: b.h}, click: a.toggleAssign})
	}
}

// toggleAssign opens the window, or closes it when it is the open one.
func (a *App) toggleAssign() { a.toggleKitWindow(assignModalID, a.openAssign) }

// openAssign opens the Assign window: a main-tab window in the Window rect
// (hud_window.go). Its X, Escape and a click on the open tab close it.
func (a *App) openAssign() {
	if a.renderer == nil {
		return
	}
	a.uiPushModal(modal{
		id: assignModalID, title: "Assign", w: assignW, h: assignH,
		modeless: true, anchor: (*App).windowAnchor,
		draw: func(a *App, body rect) { a.assignBody(body) },
	})
}

// assignRow is one colonist's row.
type assignRow struct {
	id     entity.EntityID
	name   string
	policy drug.Preset
	known  bool // the simulation reports policies
	care   medcare.Policy
}

// assignRows lists the living colonists and their drug policies.
func (a *App) assignRows() []assignRow {
	rd, known := a.renderer.game.(drugPolicyReader)
	var out []assignRow
	for _, p := range a.shell.Pawns() {
		if p.Status == "dead" {
			continue
		}
		r := assignRow{id: p.ID, name: pawnLabel(p), known: known, care: medcare.Policy(p.CareLevel)}
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
	head, rest := body.SplitV(assignHeadH)
	hs := withColor(tsLabel, uiHeading)
	a.uiLabel(head.X, head.Y, "Colonist", hs)
	a.uiLabel(head.X+assignNameW, head.Y, "Drugs", hs)
	a.uiTip("assign.head.drugs", rect{X: head.X + assignNameW, Y: head.Y, W: 120, H: head.H}, assignDrugTip)
	careX := head.X + assignNameW + assignDrugW + 10
	a.uiLabel(careX, head.Y, "Medical", hs)
	a.uiTip("assign.head.care", rect{X: careX, Y: head.Y, W: 120, H: head.H}, assignCareTip)
	rest, foot := rest.SplitV(rest.H - 2*assignRowH) // MED-03: the default care row
	a.assignDefaultCare(foot)
	if a.uiButton("assign.areas", rect{X: head.Right() - 120, Y: head.Y - 2, W: 120, H: head.H}, "Manage areas",
		btnOpts{Tip: "Allowed areas\nMake, rename, invert and delete areas. Pick a colonist's area in the Schedule tab, an animal's in the Animals tab."}) {
		a.openAreaManager() // areas_ui.go
	}
	rows := a.assignRows()
	if len(rows) == 0 {
		a.uiLabel(rest.X, rest.Y+6, "No colonists.", dimText())
		return
	}
	area := a.uiScrollArea("assign.rows", rest, float32(len(rows)*assignRowH), scrollOpts{Step: assignRowH * 2})
	y := area.View.Y - area.Off
	for _, r := range rows {
		rr := rect{X: area.View.X, Y: y, W: area.View.W, H: assignRowH - 4}
		y += assignRowH
		if rr.Y < area.View.Y || rr.Bottom() > area.View.Bottom() {
			continue // outside the view: no widgets
		}
		a.uiLabel(rr.X, rr.Y+(rr.H-float32(lineH(tsBody)))/2, ellipsize(r.name, tsBody, assignNameW-12), tsBody)
		a.assignCareButton(r, rr) // MED-03
		br := rect{X: rr.X + assignNameW, Y: rr.Y, W: min(assignDrugW, rr.W-assignNameW), H: rr.H}
		id := assignDrugID(r.id)
		if !r.known {
			a.uiButton(id, br, "-", btnOpts{Disabled: true, Reason: "Drug policies are not available in this game."})
			continue
		}
		if a.uiButton(id, br, presetLabel(r.policy), btnOpts{Tip: assignDrugTip}) {
			a.shell.Enqueue(command.Command{Kind: command.SetDrugPolicy, Pawn: uint64(r.id), Mode: uint8(nextPreset(r.policy))})
		}
	}
}

// assignCareButton is a colonist's medical care button (MED-03).
func (a *App) assignCareButton(r assignRow, rr rect) {
	cr := rect{X: rr.X + assignNameW + assignDrugW + 10, Y: rr.Y, W: min(assignCareW, rr.W-assignNameW-assignDrugW-10), H: rr.H}
	if a.uiButton(assignCareID(r.id), cr, r.care.Label(), btnOpts{Tip: assignCareTip}) {
		a.shell.Enqueue(command.Command{Kind: command.SetMedicalCare, Pawn: uint64(r.id), Mode: uint8(nextCare(r.care))})
	}
}

// assignDefaultCare is the default care row: "Default care" and a button
// per group showing its level; a click picks the next level.
func (a *App) assignDefaultCare(r rect) {
	a.uiLabel(r.X, r.Y+6, "Default medical care", withColor(tsLabel, uiHeading))
	rd, known := a.renderer.game.(defaultCareReader)
	var def [medcare.NumGroups]uint8
	if known {
		def = rd.DefaultCare()
	}
	y := r.Y + assignRowH
	bw := (r.W - 8) / float32(medcare.NumGroups)
	for g := medcare.Group(0); g < medcare.NumGroups; g++ {
		br := rect{X: r.X + float32(g)*bw, Y: y, W: bw - 6, H: assignRowH - 4}
		cur := medcare.Policy(def[g])
		tip := "Default care: " + g.Label() + "\nNow " + cur.Label() + ". Anyone in this group without a level of its own gets it. A click picks the next level."
		if !known {
			a.uiButton(assignDefCareID(g), br, g.Label(), btnOpts{Disabled: true, Reason: "Default care is not available in this game."})
			continue
		}
		if a.uiButton(assignDefCareID(g), br, g.Label()+": "+careShort(cur), btnOpts{Tip: tip}) {
			a.shell.Enqueue(command.Command{Kind: command.SetDefaultCare, Index: int(g), Mode: uint8(nextCare(cur))})
		}
	}
}

// assignDrugID is a colonist's drug policy button id.
func assignDrugID(id entity.EntityID) string {
	return "assign.drug." + strconv.FormatUint(uint64(id), 10)
}
