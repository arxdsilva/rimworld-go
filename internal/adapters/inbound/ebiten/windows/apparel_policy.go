// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

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

// Declared Assign column ids, in order (QA-002).
const (
	assignColColonist = iota
	assignColDrugs
	assignColMedical
	assignColFood
	assignColApparel
	assignColClear
	assignNumCols
)

// assignColumns declares the Assign window's columns: the header reads its
// titles here, and the column test its ids. Clear forced has no title.
func assignColumns(sh interface{ T(string) string }) [assignNumCols]uikit.TableCol {
	return [assignNumCols]uikit.TableCol{
		{ID: "colonist", Title: sh.T("Colonist")},
		{ID: "drugs", Title: sh.T("Drugs")},
		{ID: "medical", Title: sh.T("Medical")},
		{ID: "food", Title: sh.T("Food")},
		{ID: "apparel", Title: sh.T("Apparel")},
		{ID: "clear"},
	}
}

// assignCells appends the cells of an Assign row (or of its header) laid in
// row, in declared order: the one place a column's rectangle is worked out.
// A column with nothing of it left in the row (under 24 px for Apparel and
// Clear forced) gets no cell.
func assignCells(dst []uikit.TableCell, row gfx.Rect) []uikit.TableCell {
	c := newAssignCols(row.W)
	place := func(dst []uikit.TableCell, col string, x, w, minW float32) []uikit.TableCell {
		r := gfx.Rect{X: row.X + x, Y: row.Y, W: min(w, row.W-x), H: row.H}
		if r.W < minW || r.W <= 0 {
			return dst
		}
		return append(dst, uikit.TableCell{Col: col, R: r})
	}
	dst = place(dst, "colonist", 0, c.nameW, 0)
	dst = place(dst, "drugs", c.drugX, c.drugW, 0)
	dst = place(dst, "medical", c.careX, c.careW, 0)
	dst = place(dst, "food", c.foodX, assignFoodW, 0)
	dst = place(dst, "apparel", c.outX, c.outW, 24)
	return place(dst, "clear", c.clrX, c.clrW, 24)
}

// assignCellRect is column col's rectangle among cells; ok is false when the
// row has no room for it.
func assignCellRect(cells []uikit.TableCell, col string) (gfx.Rect, bool) {
	for _, c := range cells {
		if c.Col == col {
			return c.R, true
		}
	}
	return gfx.Rect{}, false
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

// outfitHead is the Apparel header: a button that opens the policy editor,
// on the header cell hc. It reports the window was replaced.
func (w Assign) outfitHead(hc []uikit.TableCell, title string) bool {
	r, ok := assignCellRect(hc, "apparel")
	if !ok {
		return false
	}
	r.Y -= 2
	if w.kit().Button(AssignApparelEditID, r, title, uikit.BtnOpts{Tip: w.sh.T("Apparel policies\nMake and edit which garments, materials, wear and quality each colonist may put on.")}) {
		w.OpenApparelPolicies()
		return true
	}
	return false
}

// outfitCells declares a colonist's Apparel button and Clear forced button
// on their row cells rc.
func (w Assign) outfitCells(p game.PawnView, rc []uikit.TableCell) {
	if br, ok := assignCellRect(rc, "apparel"); ok {
		w.apparelButton(p, br)
	}
	if cr, ok := assignCellRect(rc, "clear"); ok {
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
