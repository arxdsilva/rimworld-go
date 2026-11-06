package windows

import (
	"strconv"
	"strings"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/application/game/gearview"
	"rimworld-go/internal/domain/sim/command"
)

// Apparel policies (OUT-001…005, MAIN_UI_SCREENS Assign): an Apparel column
// in the Assign window cycles each colonist through the policies, a Clear
// forced button releases the garments the player chose, and the Apparel
// policies window (apparel_policy_editor.go) edits them. It reads
// WorldView.Gear.Policies and PawnView.Gear.ApparelPolicy and sends
// SetApparelPolicy, ClearForcedApparel and EditApparelPolicy; nothing here
// changes the simulation.

// Button ids (journeys press them by these).
const (
	AssignApparelEditID = "assign.apparelpolicies"
	assignOutfitPrefix  = "assign.apparel."
	assignClearPrefix   = "assign.clearforced."
)

// AssignApparelID is a colonist's apparel policy button id; AssignClearID
// the id of their Clear forced button.
func AssignApparelID(id uint64) string { return assignOutfitPrefix + strconv.FormatUint(id, 10) }
func AssignClearID(id uint64) string   { return assignClearPrefix + strconv.FormatUint(id, 10) }

// assignCols are the Assign window's column offsets and widths, worked
// out from the body width: the columns share what the window has left over
// after the fixed Food column and shrink together, never below half.
type assignCols struct {
	nameW                         float32
	drugX, drugW, careX, careW    float32
	foodX, outX, outW, clrX, clrW float32
}

const (
	assignColGap  = 10
	assignOutW    = 130
	assignClearW  = 90
	assignFlexW   = assignNameW + assignDrugW + assignCareW + assignOutW + assignClearW
	assignFixedW  = assignFoodW + 5*assignColGap
	assignMinFrac = 0.5
)

func newAssignCols(w float32) assignCols {
	s := min(1, max(assignMinFrac, (w-assignFixedW)/assignFlexW))
	var c assignCols
	c.nameW = assignNameW * s
	c.drugX, c.drugW = c.nameW, assignDrugW*s
	c.careX, c.careW = c.drugX+c.drugW+assignColGap, assignCareW*s
	c.foodX = c.careX + c.careW + assignColGap
	c.outX, c.outW = c.foodX+assignFoodW+assignColGap, assignOutW*s
	c.clrX, c.clrW = c.outX+c.outW+assignColGap, assignClearW*s
	return c
}

// cell is the column's rectangle in row rr, cut to the row; ok is false
// when nothing of it is left.
func cell(rr gfx.Rect, x, w float32) (gfx.Rect, bool) {
	r := gfx.Rect{X: rr.X + x, Y: rr.Y, W: min(w, rr.W-x), H: rr.H}
	return r, r.W >= 24
}

// outfitPolicyIndex is policy id's place in ps, or -1.
func outfitPolicyIndex(ps []gearview.Policy, id int) int {
	for i, p := range ps {
		if int(p.ID) == id {
			return i
		}
	}
	return -1
}

// NextApparelPolicy is the id of the policy after id in ps (wrapping); the
// first when id is unknown.
func NextApparelPolicy(ps []gearview.Policy, id uint32) (next int, ok bool) {
	if len(ps) == 0 {
		return 0, false
	}
	for i, p := range ps {
		if p.ID == id {
			return int(ps[(i+1)%len(ps)].ID), true
		}
	}
	return int(ps[0].ID), true
}

// outfitHead is the Apparel header: a button that opens the policy editor.
// It reports the window was replaced.
func (w Assign) outfitHead(head gfx.Rect, cols assignCols) bool {
	r, ok := cell(head, cols.outX, cols.outW)
	if !ok {
		return false
	}
	r.Y -= 2
	if w.kit().Button(AssignApparelEditID, r, w.sh.T("Apparel"), uikit.BtnOpts{Tip: w.sh.T("Apparel policies\nMake and edit which garments, materials, wear and quality each colonist may put on.")}) {
		w.OpenApparelPolicies()
		return true
	}
	return false
}

// outfitCells declares a colonist's Apparel button and Clear forced button.
func (w Assign) outfitCells(p game.PawnView, rr gfx.Rect, cols assignCols) {
	if br, ok := cell(rr, cols.outX, cols.outW); ok {
		w.apparelButton(p, br)
	}
	if cr, ok := cell(rr, cols.clrX, cols.clrW); ok {
		o := uikit.BtnOpts{Tip: w.sh.T("Clear forced\nLet this colonist choose again what to wear: every garment you picked by hand is released.")}
		if !anyForced(p.ApparelForced) {
			o.Disabled, o.Reason = true, w.sh.T("Nothing this colonist wears was picked by hand.")
		}
		if w.kit().Button(AssignClearID(uint64(p.ID)), cr, w.sh.T("Clear forced"), o) {
			w.sh.Enqueue(command.Command{Kind: command.ClearForcedApparel, Pawn: uint64(p.ID)})
		}
	}
}

func anyForced(f []bool) bool {
	for _, b := range f {
		if b {
			return true
		}
	}
	return false
}

// apparelButton is a colonist's apparel policy button.
func (w Assign) apparelButton(p game.PawnView, br gfx.Rect) {
	id := AssignApparelID(uint64(p.ID))
	ps := w.sh.World().Gear.Policies
	label := p.Gear.ApparelPolicyName
	if label == "" {
		label = "-"
	}
	next, ok := NextApparelPolicy(ps, p.Gear.ApparelPolicy)
	if !ok {
		w.kit().Button(id, br, "-", uikit.BtnOpts{Disabled: true, Reason: w.sh.T("Apparel policies are not available in this game.")})
		return
	}
	if w.kit().Button(id, br, gfx.Ellipsize(label, gfx.Body, float64(br.W)-12), uikit.BtnOpts{Tip: w.sh.T("Apparel policy\nWhich garments this colonist may wear. A click picks the next policy; the Apparel header opens the policy editor.")}) {
		w.sh.Enqueue(command.Command{Kind: command.SetApparelPolicy, Pawn: uint64(p.ID), Index: next})
		w.armCell(paintOutfit, id, next)
	}
}

// hasName reports n is in list.
func hasName(list []string, n string) bool {
	for _, s := range list {
		if s == n {
			return true
		}
	}
	return false
}

// trimName is the rename field's normaliser.
func trimName(s string) string { return strings.TrimSpace(s) }
