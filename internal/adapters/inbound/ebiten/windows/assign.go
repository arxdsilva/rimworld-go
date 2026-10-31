package windows

import (
	"strconv"
	"strings"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
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
	AssignModalID = "colony.assign"
	assignW       = 880
	assignH       = 480
	AssignRowH    = 30
	assignNameW   = 180
	assignHeadH   = 24
	assignDrugW   = 170
	assignCareW   = 150 // MED-03: the medical care column
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

// NextCare is the care level after p, wrapping from the best to none.
func NextCare(p medcare.Policy) medcare.Policy { return (p + 1) % medcare.NumPolicies }

// AssignCareID is a colonist's care button id; assignDefCareID group g's
// default care button id.
func AssignCareID(id entity.EntityID) string {
	return "assign.care." + strconv.FormatUint(uint64(id), 10)
}

func AssignDefCareID(g medcare.Group) string { return "assign.defcare." + strconv.Itoa(int(g)) }

// AssignPresets is the cycle order of the drug policy button.
var AssignPresets = []drug.Preset{drug.NoDrugs, drug.SocialDrinking, drug.Unrestricted}

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

// Toggle opens the window, or closes it when it is the open one.
func (w Assign) Toggle() { w.toggleKit(AssignModalID, w.Launch) }

// Launch opens the Assign window: a main-tab window in the Window rect
// (window.go). Its X, Escape and a click on the open tab close it.
func (w Assign) Launch() {
	if !w.live() {
		return
	}
	w.OpenKit(uikit.Modal{
		ID: AssignModalID, Title: "Assign", W: assignW, H: assignH, Anchor: w.Anchor,
		Draw: func(body gfx.Rect) { w.body(body) },
	})
}

// assignRow is one colonist's row.
type assignRow struct {
	id     entity.EntityID
	name   string
	policy drug.Preset
	known  bool // the simulation reports policies
	care   medcare.Policy
	pawn   game.PawnView
}

// rows lists the living colonists and their drug policies.
func (w Assign) rows() []assignRow {
	rd, known := w.sh.Env().Game.(drugPolicyReader)
	var out []assignRow
	for _, p := range w.sh.Pawns() {
		if p.Status == "dead" {
			continue
		}
		r := assignRow{id: p.ID, name: w.sh.PawnLabel(p), known: known, care: medcare.Policy(p.CareLevel), pawn: p}
		if known {
			r.policy = drug.Preset(rd.DrugPolicy(p.ID))
		}
		out = append(out, r)
	}
	return out
}

// PresetLabel is the policy's button label ("Social drinking").
func PresetLabel(p drug.Preset) string {
	l := p.Label()
	if l == "" {
		return l
	}
	return strings.ToUpper(l[:1]) + l[1:]
}

// NextPreset is the policy after p in the cycle.
func NextPreset(p drug.Preset) drug.Preset {
	for i, q := range AssignPresets {
		if q == p {
			return AssignPresets[(i+1)%len(AssignPresets)]
		}
	}
	return drug.SocialDrinking
}

// body declares the header and the visible rows (the list scrolls;
// only rows in view get widgets).
func (w Assign) body(body gfx.Rect) {
	if !w.live() {
		return
	}
	head, rest := body.SplitV(assignHeadH)
	hs := gfx.WithColor(gfx.Label, uikit.Heading)
	w.kit().Label(head.X, head.Y, "Colonist", hs)
	w.kit().Label(head.X+assignNameW, head.Y, "Drugs", hs)
	w.kit().Tip("assign.head.drugs", gfx.Rect{X: head.X + assignNameW, Y: head.Y, W: 120, H: head.H}, assignDrugTip)
	careX := head.X + assignNameW + assignDrugW + 10
	w.kit().Label(careX, head.Y, "Medical", hs)
	w.kit().Tip("assign.head.care", gfx.Rect{X: careX, Y: head.Y, W: 120, H: head.H}, assignCareTip)
	foodX := careX + assignCareW + 10
	w.kit().Label(foodX, head.Y, "Food", hs)
	w.kit().Tip("assign.head.food", gfx.Rect{X: foodX, Y: head.Y, W: 120, H: head.H}, assignFoodTip)
	rest, foot := rest.SplitV(rest.H - 2*AssignRowH) // MED-03: the default care row
	w.defaultCare(foot)
	if w.kit().Button(AssignFoodEditID, gfx.Rect{X: head.Right() - 260, Y: head.Y - 2, W: 130, H: head.H}, "Food policies",
		uikit.BtnOpts{Tip: "Food policies\nMake and edit what each colonist, prisoner and animal may eat."}) {
		w.OpenFoodPolicies()
		return
	}
	if w.kit().Button("assign.areas", gfx.Rect{X: head.Right() - 120, Y: head.Y - 2, W: 120, H: head.H}, "Manage areas",
		uikit.BtnOpts{Tip: "Allowed areas\nMake, rename, invert and delete areas. Pick a colonist's area in the Schedule tab, an animal's in the Animals tab."}) {
		w.sh.ManageAreas() // the Architect's area manager
	}
	rows := w.rows()
	if len(rows) == 0 {
		w.kit().Label(rest.X, rest.Y+6, "No colonists.", uikit.DimText())
		return
	}
	area := w.kit().ScrollArea("assign.rows", rest, float32(len(rows)*AssignRowH), uikit.ScrollOpts{Step: AssignRowH * 2})
	y := area.View.Y - area.Off
	for _, r := range rows {
		rr := gfx.Rect{X: area.View.X, Y: y, W: area.View.W, H: AssignRowH - 4}
		y += AssignRowH
		if rr.Y < area.View.Y || rr.Bottom() > area.View.Bottom() {
			continue // outside the view: no widgets
		}
		w.kit().Label(rr.X, rr.Y+(rr.H-float32(gfx.LineH(gfx.Body)))/2, gfx.Ellipsize(r.name, gfx.Body, assignNameW-12), gfx.Body)
		w.careButton(r, rr) // MED-03
		w.foodButton(r.pawn, rr, rr.X+assignNameW+assignDrugW+10+assignCareW+10)
		br := gfx.Rect{X: rr.X + assignNameW, Y: rr.Y, W: min(assignDrugW, rr.W-assignNameW), H: rr.H}
		id := AssignDrugID(r.id)
		if !r.known {
			w.kit().Button(id, br, "-", uikit.BtnOpts{Disabled: true, Reason: "Drug policies are not available in this game."})
			continue
		}
		if w.kit().Button(id, br, PresetLabel(r.policy), uikit.BtnOpts{Tip: assignDrugTip}) {
			w.sh.Enqueue(command.Command{Kind: command.SetDrugPolicy, Pawn: uint64(r.id), Mode: uint8(NextPreset(r.policy))})
		}
	}
}

// careButton is a colonist's medical care button (MED-03).
func (w Assign) careButton(r assignRow, rr gfx.Rect) {
	cr := gfx.Rect{X: rr.X + assignNameW + assignDrugW + 10, Y: rr.Y, W: min(assignCareW, rr.W-assignNameW-assignDrugW-10), H: rr.H}
	if w.kit().Button(AssignCareID(r.id), cr, r.care.Label(), uikit.BtnOpts{Tip: assignCareTip}) {
		w.sh.Enqueue(command.Command{Kind: command.SetMedicalCare, Pawn: uint64(r.id), Mode: uint8(NextCare(r.care))})
	}
}

// defaultCare is the default care row: "Default care" and a button
// per group showing its level; a click picks the next level.
func (w Assign) defaultCare(r gfx.Rect) {
	w.kit().Label(r.X, r.Y+6, "Default medical care", gfx.WithColor(gfx.Label, uikit.Heading))
	rd, known := w.sh.Env().Game.(defaultCareReader)
	var def [medcare.NumGroups]uint8
	if known {
		def = rd.DefaultCare()
	}
	y := r.Y + AssignRowH
	bw := (r.W - 8) / float32(medcare.NumGroups)
	for g := medcare.Group(0); g < medcare.NumGroups; g++ {
		br := gfx.Rect{X: r.X + float32(g)*bw, Y: y, W: bw - 6, H: AssignRowH - 4}
		cur := medcare.Policy(def[g])
		tip := "Default care: " + g.Label() + "\nNow " + cur.Label() + ". Anyone in this group without a level of its own gets it. A click picks the next level."
		if !known {
			w.kit().Button(AssignDefCareID(g), br, g.Label(), uikit.BtnOpts{Disabled: true, Reason: "Default care is not available in this game."})
			continue
		}
		if w.kit().Button(AssignDefCareID(g), br, g.Label()+": "+w.sh.CareShort(cur), uikit.BtnOpts{Tip: tip}) {
			w.sh.Enqueue(command.Command{Kind: command.SetDefaultCare, Index: int(g), Mode: uint8(NextCare(cur))})
		}
	}
}

// AssignDrugID is a colonist's drug policy button id.
func AssignDrugID(id entity.EntityID) string {
	return "assign.drug." + strconv.FormatUint(uint64(id), 10)
}
