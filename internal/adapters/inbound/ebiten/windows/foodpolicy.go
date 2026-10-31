package windows

import (
	"strconv"
	"strings"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

// Food policies (FOOD-5, MAIN_UI_SCREENS Assign): a Food column in the
// Assign window cycles each colonist through the policies, and the Food
// policies window edits them: pick one, make a copy, rename or delete it,
// and tick the foods it allows (the Foods branch of the storage filter
// tree). It reads WorldView.FoodPolicies and PawnView.FoodPolicy and sends
// SetFoodPolicy and EditFoodPolicy; nothing here changes the simulation.

const (
	FoodPolicyModalID = "colony.foodpolicy"
	foodPolicyW       = 640
	foodPolicyH       = 500
	foodListW         = 190
	foodRowH          = 24
	assignFoodW       = 150
	foodNameMax       = 24
)

// Button ids (journeys press them by these).
const (
	AssignFoodEditID  = "assign.food.edit"
	FoodNewID         = "foodpolicy.new"
	FoodDeleteID      = "foodpolicy.delete"
	FoodNameID        = "foodpolicy.name"
	FoodBackID        = "foodpolicy.back"
	FoodAllowAllID    = "foodpolicy.allow_all"
	FoodClearAllID    = "foodpolicy.clear_all"
	foodSelPrefix     = "foodpolicy.sel."
	foodFilterPrefix  = "foodpolicy.filter."
	foodExpandPrefix  = "foodpolicy.expand."
	assignFoodPrefix  = "assign.food."
	foodPolicyRoot    = "Foods"
	assignFoodTip     = "Food policy\nWhat this colonist may eat. A click picks the next policy; Food policies edits them."
	foodPolicyNewName = "New policy"
)

// AssignFoodID is a colonist's food policy button id.
func AssignFoodID(id uint64) string { return assignFoodPrefix + strconv.FormatUint(id, 10) }

// FoodSelectID is policy id's row in the editor.
func FoodSelectID(id int) string { return foodSelPrefix + strconv.Itoa(id) }

// foodState is the editor's own state.
type foodState struct {
	sel  int             // the policy being edited (a policy id)
	name string          // the rename field
	open map[string]bool // expanded category paths
	last int             // the policy whose name the field holds
}

// NextFoodPolicy is the id of the policy after the one named cur in
// policies (wrapping); the first when cur is unknown.
func NextFoodPolicy(policies []game.FoodPolicyView, cur string) (id int, ok bool) {
	if len(policies) == 0 {
		return 0, false
	}
	for i, p := range policies {
		if p.Name == cur {
			return policies[(i+1)%len(policies)].ID, true
		}
	}
	return policies[0].ID, true
}

// foodButton is a colonist's food policy button in the Assign row.
func (w Assign) foodButton(p game.PawnView, rr gfx.Rect, x float32) {
	br := gfx.Rect{X: x, Y: rr.Y, W: min(assignFoodW, rr.Right()-x), H: rr.H}
	id := AssignFoodID(uint64(p.ID))
	policies := w.sh.World().FoodPolicies
	label := p.FoodPolicy
	if label == "" {
		label = "-"
	}
	next, ok := NextFoodPolicy(policies, p.FoodPolicy)
	if !ok {
		w.kit().Button(id, br, label, uikit.BtnOpts{Disabled: true, Reason: "Food policies are not available in this game."})
		return
	}
	if w.kit().Button(id, br, gfx.Ellipsize(label, gfx.Body, float64(br.W)-12), uikit.BtnOpts{Tip: assignFoodTip}) {
		w.sh.Enqueue(command.Command{Kind: command.SetFoodPolicy, Pawn: uint64(p.ID), Index: next})
	}
}

// OpenFoodPolicies opens the Food policies window.
func (w Assign) OpenFoodPolicies() {
	if !w.live() {
		return
	}
	w.food.open = map[string]bool{}
	w.OpenKit(uikit.Modal{
		ID: FoodPolicyModalID, Title: "Food policies", W: foodPolicyW, H: foodPolicyH, Anchor: w.Anchor,
		Draw: func(body gfx.Rect) { w.foodBody(body) },
	})
}

// foodPolicyByID finds policy id.
func foodPolicyByID(ps []game.FoodPolicyView, id int) (game.FoodPolicyView, bool) {
	for _, p := range ps {
		if p.ID == id {
			return p, true
		}
	}
	return game.FoodPolicyView{}, false
}

// foodBody declares the policy list, its buttons, the name field and the
// allowed-foods tree.
func (w Assign) foodBody(body gfx.Rect) {
	st := w.food
	ps := w.sh.World().FoodPolicies
	if len(ps) == 0 {
		w.kit().Label(body.X, body.Y, "Food policies are not available in this game.", uikit.DimText())
		return
	}
	cur, ok := foodPolicyByID(ps, st.sel)
	if !ok {
		cur, st.sel = ps[0], ps[0].ID
	}
	if st.last != cur.ID { // a draft may be empty while it is edited: only a new selection reloads it
		st.name, st.last = cur.Name, cur.ID
	}
	if w.kit().Button(FoodBackID, gfx.Rect{X: body.Right() - 80, Y: body.Y - 2, W: 80, H: 24}, "Assign", uikit.BtnOpts{Tip: "Back to the Assign window."}) {
		w.Launch()
		return
	}
	list := gfx.Rect{X: body.X, Y: body.Y + 28, W: foodListW, H: body.H - 28 - 34}
	for i, p := range ps {
		r := gfx.Rect{X: list.X, Y: list.Y + float32(i)*(foodRowH+2), W: list.W, H: foodRowH}
		if r.Bottom() > list.Bottom() {
			break
		}
		if w.kit().Button(FoodSelectID(p.ID), r, gfx.Ellipsize(p.Name, gfx.Body, float64(r.W)-12), uikit.BtnOpts{Selected: p.ID == cur.ID, Tip: "Edit " + p.Name}) {
			st.sel = p.ID
		}
	}
	by := body.Bottom() - 28
	if w.kit().Button(FoodNewID, gfx.Rect{X: body.X, Y: by, W: 92, H: 26}, "Copy", uikit.BtnOpts{Tip: "New policy\nMake a policy as a copy of the selected one."}) {
		w.sh.Enqueue(command.Command{Kind: command.EditFoodPolicy, Mode: command.FoodPolicyCreate, Index: cur.ID, Name: foodPolicyNewName})
	}
	del := uikit.BtnOpts{Tip: "Delete policy\nIts users fall back to the default policy."}
	if len(ps) <= 1 || cur.ID == ps[0].ID {
		del.Disabled, del.Reason = true, "the default policy stays"
	}
	if w.kit().Button(FoodDeleteID, gfx.Rect{X: body.X + 98, Y: by, W: 92, H: 26}, "Delete", del) {
		w.sh.Enqueue(command.Command{Kind: command.EditFoodPolicy, Mode: command.FoodPolicyDelete, Index: cur.ID})
		st.sel = ps[0].ID
	}
	right := gfx.Rect{X: body.X + foodListW + 12, Y: body.Y + 28, W: body.W - foodListW - 12, H: body.H - 28}
	field := gfx.Rect{X: right.X, Y: body.Y, W: right.W - 90, H: 24}
	if _, committed := w.kit().TextField(FoodNameID, field, &st.name, uikit.TextFieldSpec{MaxLen: foodNameMax, Placeholder: "Name", Normalize: strings.TrimSpace}); committed {
		if st.name == "" { // an empty name is refused on commit: the old one comes back
			st.name = cur.Name
		} else {
			w.sh.Enqueue(command.Command{Kind: command.EditFoodPolicy, Mode: command.FoodPolicyRename, Index: cur.ID, Name: st.name})
		}
	}
	w.foodTree(right, cur)
}

// foodRows are the Foods branch of the filter tree for policy p, with the
// children of a collapsed category left out.
func foodRows(p game.FoodPolicyView, open map[string]bool) []game.FilterRow {
	var out []game.FilterRow
	hide := -1
	for _, r := range game.StorageFilterRows(game.ZoneStockpile, p.Allowed) {
		if r.Path != foodPolicyRoot && !strings.HasPrefix(r.Path, foodPolicyRoot+"/") {
			continue
		}
		if hide >= 0 {
			if r.Depth > hide {
				continue
			}
			hide = -1
		}
		out = append(out, r)
		if !r.Leaf && !open[r.Path] {
			hide = r.Depth
		}
	}
	return out
}

// foodTree declares the scrolling allowed-foods tree of policy p.
func (w Assign) foodTree(r gfx.Rect, p game.FoodPolicyView) {
	st := w.food
	w.kit().Label(r.X, r.Y+2, "Allowed foods", uikit.DimText())
	allow := gfx.Rect{X: r.Right() - 170, Y: r.Y, W: 82, H: 24}
	clear := gfx.Rect{X: r.Right() - 84, Y: r.Y, W: 82, H: 24}
	if w.kit().Button(FoodAllowAllID, allow, "Allow all", uikit.BtnOpts{Tip: "Allow every food."}) {
		w.sh.Enqueue(command.Command{Kind: command.EditFoodPolicy, Mode: command.FoodPolicyAllow, Index: p.ID, Name: foodPolicyRoot, Enabled: true})
	}
	if w.kit().Button(FoodClearAllID, clear, "Clear all", uikit.BtnOpts{Tip: "Forbid every food."}) {
		w.sh.Enqueue(command.Command{Kind: command.EditFoodPolicy, Mode: command.FoodPolicyAllow, Index: p.ID, Name: foodPolicyRoot, Enabled: false})
	}
	list := gfx.Rect{X: r.X, Y: r.Y + 28, W: r.W, H: r.H - 28}
	rows := foodRows(p, st.open)
	area := w.kit().ScrollArea("foodpolicy.list", list, float32(len(rows))*foodRowH, uikit.ScrollOpts{Step: foodRowH * 3, Focusable: true})
	for i, row := range rows {
		y := area.View.Y + float32(i)*foodRowH - area.Off
		if y < area.View.Y || y+foodRowH > area.View.Bottom() {
			continue
		}
		x := area.View.X + float32(row.Depth*22)
		if !row.Leaf {
			mark := "+"
			if st.open[row.Path] {
				mark = "-"
			}
			if w.kit().Button(foodExpandPrefix+row.Path, gfx.Rect{X: x, Y: y, W: 22, H: foodRowH}, mark, uikit.BtnOpts{Tip: "Show or hide the foods in " + row.Label}) {
				st.open[row.Path] = !st.open[row.Path]
			}
			x += 26
		}
		on, label := row.On, row.Label
		if row.Partial {
			label += "  (some)"
		}
		if w.kit().Check(foodFilterPrefix+row.Path, gfx.Rect{X: x, Y: y, W: area.View.Right() - x, H: foodRowH}, label, &on, uikit.CheckOpts{}) {
			w.sh.Enqueue(command.Command{Kind: command.EditFoodPolicy, Mode: command.FoodPolicyAllow, Index: p.ID, Name: row.Path, Enabled: !row.On})
		}
	}
}
