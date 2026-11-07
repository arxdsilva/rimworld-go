package windows

import (
	"strconv"
	"strings"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/adapters/inbound/ebiten/lang"
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
// SetDefaultCare. The Food column and its policies are in foodpolicy.go, the
// Apparel column and Clear forced in apparel_policy.go.

const (
	AssignModalID = "colony.assign"
	assignW       = 880
	AssignRowH    = 30
	assignNameW   = 140
	assignHeadH   = 24
	assignDrugW   = 130
	assignCareW   = 120 // MED-03: the medical care column
)

// careTip, drugTip: the Medical and Drugs columns' tips, on header and cells.
func (w Assign) careTip() string {
	return w.sh.T("Medical care\nHow much care this colonist gets: no care, care without medicine, herbal or less, industrial or less, or the best medicine there is. A click picks the next level.")
}

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
var AssignPresets = []drug.Preset{drug.NoDrugs, drug.SocialDrinking, drug.Unrestricted, drug.MoodLift}

func (w Assign) drugTip() string {
	return w.sh.T("Drug policy\nNo drugs: never takes any, even when hooked.\nSocial drinking: a beer now and then when bored.\nUnrestricted: any recreational drug when bored.\nMood lift: a beer only while mood is below 50 and not already recovering.\nA colonist hooked on a drug seeks it when the craving starts, unless set to no drugs. Abstainers never take drugs for fun.")
}

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
		ID: AssignModalID, Title: w.sh.T("Assign"), W: assignW, H: assignHeight(0), Anchor: w.Anchor, Frameless: true,
		Draw: func(body gfx.Rect) { w.body(body) },
	})
}

// assignHeight is the window's outer height for n colonists (UIS-063): the
// inner margin, the header row, a row per colonist, the default care and
// button rows, and the inner margin (reference: 70 above the first row, 60
// under the last).
func assignHeight(n int) float32 {
	return float32(2*hudlayout.WindowPad + assignHeadH + n*AssignRowH + 2*AssignRowH + 10)
}

// size is the window's outer size: assignW wide, as tall as its colonists.
func (w Assign) size() (float32, float32) {
	return assignW, assignHeight(len(w.rows()))
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
	cols := newAssignCols(body.W)
	hs := gfx.WithColor(gfx.Label, uikit.Heading)
	w.kit().Label(head.X, head.Y, w.sh.T("Colonist"), hs)
	w.kit().Label(head.X+cols.drugX, head.Y, w.sh.T("Drugs"), hs)
	w.kit().Tip("assign.head.drugs", gfx.Rect{X: head.X + cols.drugX, Y: head.Y, W: 120, H: head.H}, w.drugTip())
	w.kit().Label(head.X+cols.careX, head.Y, w.sh.T("Medical"), hs)
	w.kit().Tip("assign.head.care", gfx.Rect{X: head.X + cols.careX, Y: head.Y, W: 120, H: head.H}, w.careTip())
	w.kit().Label(head.X+cols.foodX, head.Y, w.sh.T("Food"), hs)
	w.kit().Tip("assign.head.food", gfx.Rect{X: head.X + cols.foodX, Y: head.Y, W: 120, H: head.H}, w.foodTip())
	if w.outfitHead(head, cols) { // GEAR-3: the Apparel header opens the policy editor
		return
	}
	rest, foot := rest.SplitV(rest.H - 2*AssignRowH) // MED-03: the default care row
	w.defaultCare(foot)
	if w.kit().Button(AssignFoodEditID, gfx.Rect{X: foot.Right() - 250, Y: foot.Y, W: 120, H: AssignRowH - 4}, w.sh.T("Food policies"),
		uikit.BtnOpts{Tip: w.sh.T("Food policies\nMake and edit what each colonist, prisoner and animal may eat.")}) {
		w.OpenFoodPolicies()
		return
	}
	if w.kit().Button("assign.areas", gfx.Rect{X: foot.Right() - 120, Y: foot.Y, W: 120, H: AssignRowH - 4}, w.sh.T("Manage areas"),
		uikit.BtnOpts{Tip: w.sh.T("Allowed areas\nMake, rename, invert and delete areas. Pick a colonist's area in the Schedule tab, an animal's in the Animals tab.")}) {
		w.sh.ManageAreas() // the Architect's area manager
	}
	rows := w.rows()
	if len(rows) == 0 {
		w.kit().Label(rest.X, rest.Y+6, w.sh.T("No colonists."), uikit.DimText())
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
		w.kit().Label(rr.X, float32(gfx.CenterTop(float64(rr.Y), float64(rr.H), gfx.Body)), gfx.Ellipsize(r.name, gfx.Body, float64(cols.nameW)-12), gfx.Body)
		w.careButton(r, rr, cols) // MED-03
		w.foodButton(r.pawn, rr, rr.X+cols.foodX)
		w.outfitCells(r.pawn, rr, cols) // GEAR-3
		br := gfx.Rect{X: rr.X + cols.drugX, Y: rr.Y, W: min(cols.drugW, rr.W-cols.drugX), H: rr.H}
		id := AssignDrugID(r.id)
		if !r.known {
			w.kit().Button(id, br, "-", uikit.BtnOpts{Disabled: true, Reason: w.sh.T("Drug policies are not available in this game.")})
			continue
		}
		if w.kit().Button(id, br, PresetLabel(r.policy), uikit.BtnOpts{Tip: w.drugTip()}) {
			next := NextPreset(r.policy)
			w.sh.Enqueue(command.Command{Kind: command.SetDrugPolicy, Pawn: uint64(r.id), Mode: uint8(next)})
			w.armCell(paintDrug, id, int(next))
		}
	}
}

// careButton is a colonist's medical care button (MED-03).
func (w Assign) careButton(r assignRow, rr gfx.Rect, cols assignCols) {
	cr := gfx.Rect{X: rr.X + cols.careX, Y: rr.Y, W: min(cols.careW, rr.W-cols.careX), H: rr.H}
	if w.kit().Button(AssignCareID(r.id), cr, r.care.Label(), uikit.BtnOpts{Tip: w.careTip()}) {
		next := NextCare(r.care)
		w.sh.Enqueue(command.Command{Kind: command.SetMedicalCare, Pawn: uint64(r.id), Mode: uint8(next)})
		w.armCell(paintCare, AssignCareID(r.id), int(next))
	}
}

// defaultCare is the default care row: "Default care" and a button
// per group showing its level; a click picks the next level.
func (w Assign) defaultCare(r gfx.Rect) {
	w.kit().Label(r.X, r.Y+6, w.sh.T("Default medical care"), gfx.WithColor(gfx.Label, uikit.Heading))
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
		tip := lang.Tf(w.sh, "Default care: %s\nNow %s. Anyone in this group without a level of its own gets it. A click picks the next level.", g.Label(), cur.Label())
		if !known {
			w.kit().Button(AssignDefCareID(g), br, g.Label(), uikit.BtnOpts{Disabled: true, Reason: w.sh.T("Default care is not available in this game.")})
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

// Painting by drag (CTL-015): the first click on a column's cell arms the
// painter with the value it set; holding the button and moving over the
// same column's other cells gives them that value too.
const (
	paintDrug = iota + 1 // the painted column, kept above the value in Value
	paintCare
	paintFood
	paintOutfit
	assignColShift = 16
	assignValMask  = 1<<assignColShift - 1
)

// armCell starts a paint of value v over column col, from cell id.
func (w Assign) armCell(col int, id string, v int) { w.paint.Arm(id, col<<assignColShift|v) }

// assignCell splits a cell id into its column and colonist.
func assignCell(id string) (col int, pawn uint64, ok bool) {
	for c, prefix := range [...]string{paintDrug: "assign.drug.", paintCare: "assign.care.", paintFood: assignFoodPrefix, paintOutfit: assignOutfitPrefix} {
		if prefix != "" && strings.HasPrefix(id, prefix) {
			n, err := strconv.ParseUint(id[len(prefix):], 10, 64)
			return c, n, err == nil
		}
	}
	return 0, 0, false
}

// Drag keeps painting while the mouse is held over the armed column.
func (w Assign) Drag() {
	if !w.Open || w.Mode != AssignModalID || !w.paint.Armed || !w.live() {
		return
	}
	cell := ""
	if col, _, ok := assignCell(w.kit().Hot); ok && col == w.paint.Value>>assignColShift {
		cell = w.kit().Hot
	}
	if id, ok := w.paint.Entered(w.kit().Host().Frame().Down(inputframe.Left), cell); ok {
		w.paintCell(id)
	}
}

// paintCell gives the colonist of cell id the painted value, unless it
// already has it.
func (w Assign) paintCell(id string) {
	col, pawn, ok := assignCell(id)
	if !ok {
		return
	}
	v := w.paint.Value & assignValMask
	for _, p := range w.sh.Pawns() {
		if uint64(p.ID) != pawn || p.Status == "dead" {
			continue
		}
		switch col {
		case paintDrug:
			rd, known := w.sh.Env().Game.(drugPolicyReader)
			if known && int(rd.DrugPolicy(p.ID)) != v {
				w.sh.Enqueue(command.Command{Kind: command.SetDrugPolicy, Pawn: pawn, Mode: uint8(v)})
			}
		case paintCare:
			if int(p.CareLevel) != v {
				w.sh.Enqueue(command.Command{Kind: command.SetMedicalCare, Pawn: pawn, Mode: uint8(v)})
			}
		case paintFood:
			if pol, ok := foodPolicyByID(w.sh.World().FoodPolicies, v); ok && pol.Name != p.FoodPolicy {
				w.sh.Enqueue(command.Command{Kind: command.SetFoodPolicy, Pawn: pawn, Index: v})
			}
		case paintOutfit:
			if int(p.Gear.ApparelPolicy) != v && outfitPolicyIndex(w.sh.World().Gear.Policies, v) >= 0 {
				w.sh.Enqueue(command.Command{Kind: command.SetApparelPolicy, Pawn: pawn, Index: v})
			}
		}
		return
	}
}
