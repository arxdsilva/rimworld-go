package inspector

import (
	"fmt"
	"strconv"
	"strings"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/render/overlay"
	"rimworld-go/internal/adapters/inbound/ebiten/selection"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// Building selection and the bench's Bills tab (MAIN_UI_SCREENS §10
// inspector, UI checklist §14.1 workbench inspector). PLAY-018: the pane
// and the command buttons are cmds_build.go; this file keeps the selection,
// the click handlers, the Bills tab opened above the pane, and the bed
// owner picker. Bills are typed AddBill/SetBill/RemoveBill commands; the
// tab only reads BillView.

const BillRowH = 26

const (
	BillPickBtn  = "bill.pick"
	BillPickID   = "bill.picker"
	BillPickList = "bill.picker.list"
)

// openBillPicker opens the recipe pick list for bench b; locked recipes
// are listed but cannot be picked.
func (a Building) openBillPicker(b game.BuildingView) {
	recipes := a.RecipesFor(b.Kind)
	sel := -1
	a.kit().Push(uikit.Modal{
		ID: BillPickID, Title: "Add bill", W: 360, H: 380,
		Draw: func(r gfx.Rect) {
			if len(recipes) == 0 {
				a.kit().Label(r.X, r.Y, "This bench has no recipes.", uikit.DimText())
				return
			}
			a.kit().List(BillPickList, r, len(recipes), &sel, uikit.ListSpec{
				RowH:  26,
				Label: func(i int) string { return recipes[i].Label },
				Disabled: func(i int) string {
					if p := a.shell.LockedBy("recipe:" + recipes[i].Name); p != "" {
						return "Needs research: " + p
					}
					return ""
				},
			})
		},
		Buttons: []uikit.ModalButton{
			{ID: "bill.picker.ok", Label: "Add", Style: uikit.BtnPrimary, Closes: true,
				Disabled: func() string {
					if sel < 0 || sel >= len(recipes) {
						return "Pick a recipe first."
					}
					return ""
				},
				OnClick: func() {
					if sel >= 0 && sel < len(recipes) {
						a.ApplyBuildingClick("bill.add." + strconv.Itoa(int(recipes[sel].ID)))
					}
				}},
			{ID: "bill.picker.cancel", Label: "Cancel", Closes: true},
		},
	})
}

// billMaxCount caps a bill's target.
const BillMaxCount = 9999

// qtyStep is a numeric control's step for the held modifiers (UI-016):
// x10 with Ctrl, x100 with Shift, x1000 with both.
func qtyStep(ctrl, shift bool) int {
	switch {
	case ctrl && shift:
		return 1000
	case shift:
		return 100
	case ctrl:
		return 10
	}
	return 1
}

func (a Building) qtyStep() int { return qtyStep(a.env().Ctrl, a.env().Shift) }

// recipe.Mode values (mirrored; the adapter only sends the numbers).
const (
	BillModeDo      = 0
	BillModeForever = 1
	BillModeUntil   = 2
)

func (a *Pane) SelectedBuilding() (game.BuildingView, bool) {
	if !a.sel.HasBuilding {
		return game.BuildingView{}, false
	}
	return a.BuildingView(a.sel.SelBuilding)
}

// buildingAt is the completed object-layer building covering (x, y),
// falling back to a floor.
func BuildingAt(w game.WorldView, x, y int) (game.BuildingView, bool) {
	var floor game.BuildingView
	found := false
	for _, b := range w.Buildings {
		bw, bh := max(b.W, 1), max(b.H, 1)
		if x < b.X || y < b.Y || x >= b.X+bw || y >= b.Y+bh {
			continue
		}
		if !b.Floor && b.Kind != overlay.BridgeKind { // a floor or bridge: only when nothing stands on it
			return b, true
		}
		floor, found = b, true
	}
	return floor, found
}

func (a Building) SelectBuildingAt(tx, ty int) bool {
	b, ok := BuildingAt(a.shell.World(), tx, ty)
	if !ok {
		a.sel.HasBuilding = false
		return false
	}
	a.sel.Select(selection.Cand{Kind: selection.Building, ID: b.ID})
	return true
}

func (a Building) ClearBuilding() { a.sel.HasBuilding = false }

func BuildingTitle(b game.BuildingView) string {
	if b.Ruin {
		return ruinTitle(b)
	}
	if b.Kind == overlay.BridgeKind {
		return "Wooden bridge"
	}
	if t := overlay.CasketTitle(b); t != "" {
		return t
	}
	if b.Kind == "carpet" && strings.HasSuffix(b.Stuff, " carpet") {
		return strings.ToUpper(b.Stuff[:1]) + b.Stuff[1:] // "Straw carpet"
	}
	return materialTitle(b.Kind, b.Stuff) // "Wall", "Granite wall", "Steel door"
}

// recipeBenchName is the bench kind whose recipes a bench of kind works: an
// electric bench takes its base bench's (construction.RecipeBench), any
// other kind is its own.
func RecipeBenchName(kind string) string {
	if k, ok := construction.Parse(kind); ok {
		return construction.RecipeBench(k).String()
	}
	return kind
}

// recipesFor lists the recipes a bench of kind can be given as bills.
func (a Building) RecipesFor(kind string) []game.RecipeView {
	var out []game.RecipeView
	bench := RecipeBenchName(kind)
	for _, r := range game.Recipes() {
		if r.Bench == bench {
			out = append(out, r)
		}
	}
	return out
}

// ---- the Bills tab -------------------------------------------------------

const (
	billsPanelW = 520 // the Bills tab's panel
	billBtnH    = 22
	billBtnsW   = 122 // minus, plus, mode, pause and delete
	billAddW    = 130
)

// billsPanelH is the Bills tab's wanted panel height for n bills: the
// list, the Add bill button under it and the panel's inset all round.
func billsPanelH(n int) float32 {
	return 2*PaneInset + float32(max(n, 1))*BillRowH + bodyGap + billBtnH
}

// billsTab is the Bills tab of bench b.
func (a Building) billsTab(b game.BuildingView) Tab {
	id := b.ID
	return Tab{ID: "inspector.bills", Label: "Bills", WantW: billsPanelW, WantH: billsPanelH(len(b.Bills)),
		Declare: func(r gfx.Rect) { a.BillsTabUI(id, r) }}
}

// billsLayout splits the Bills tab's inner rectangle r: the scrolling list
// of bills and the Add bill button under it.
func billsLayout(r gfx.Rect) (list, add gfx.Rect) {
	list, foot := r.SplitV(-(billBtnH + bodyGap))
	return list, gfx.Rect{X: foot.X, Y: foot.Y + bodyGap, W: billAddW, H: billBtnH}
}

// billsBody lays out bench b's bills, viewW wide: a line of text and the
// five buttons (fewer, more, mode, pause, delete) per bill.
func BillsBody(b game.BuildingView, viewW float32) TabBody {
	var body TabBody
	if len(b.Bills) == 0 {
		body.Labels = []tabLabel{{Text: "No bills yet - add one below.", x: 8, y: 4, w: viewW - 8, st: gfx.Body}}
		body.H = BillRowH
		return body
	}
	x := viewW - billBtnsW
	for i, bl := range b.Bills {
		y := float32(i) * BillRowH
		ix := strconv.Itoa(i)
		if bl.Status != "" {
			body.Fills = append(body.Fills, tabFill{gfx.Rect{X: 0, Y: y, W: 4, H: billBtnH}, uikit.BtnDisabled})
		}
		body.Labels = append(body.Labels, tabLabel{Text: BillLine(bl), x: 8, y: y + 2, w: x - 8 - bodyGap, st: gfx.Body})
		body.Buttons = append(body.Buttons,
			uikit.Button{ID: "bill.minus." + ix, Label: "-", X: x, Y: y, W: 22, H: billBtnH, Tip: "Fewer (Ctrl x10, Shift x100, both x1000)"},
			uikit.Button{ID: "bill.plus." + ix, Label: "+", X: x + 24, Y: y, W: 22, H: billBtnH, Tip: "More (Ctrl x10, Shift x100, both x1000)"},
			uikit.Button{ID: "bill.mode." + ix, Label: "M", X: x + 48, Y: y, W: 22, H: billBtnH},
			uikit.Button{ID: "bill.pause." + ix, Label: "||", X: x + 72, Y: y, W: 26, H: billBtnH, Selected: bl.Suspended},
			uikit.Button{ID: "bill.del." + ix, Label: "x", X: x + 100, Y: y, W: 22, H: billBtnH},
		)
	}
	body.H = float32(len(b.Bills)) * BillRowH
	return body
}

// billsTabUI declares the Bills tab of bench id in r, the expanded panel's
// inner rectangle. One "+ Add bill" button opens a pick list, so every
// recipe is reachable however many the bench has.
func (a Building) BillsTabUI(id entity.EntityID, r gfx.Rect) {
	b, ok := a.BuildingView(id)
	if !ok || !b.Bench {
		return
	}
	list, add := billsLayout(r)
	a.declareTabBody("bill.rows", list, BillsBody(b, list.W-uikit.ScrollBarW), a.ApplyBuildingClick)
	if a.kit().Button(BillPickBtn, add, "+ Add bill", uikit.BtnOpts{Tip: "Add bill\nPick a recipe for this bench."}) {
		a.ApplyBuildingClick(BillPickBtn)
	}
}

// swapLine is the inspector text while an in-place material swap is
// pending on the building. "" otherwise.
func SwapLine(b game.BuildingView) string {
	if b.SwapTo == "" {
		return ""
	}
	return fmt.Sprintf("Changing to %s (%d%%)", b.SwapTo, b.SwapPct)
}

// carpetLine is a carpet's inspector text: the share of tracked dirt it
// wipes off boots. "" for other buildings.
func CarpetLine(b game.BuildingView) string {
	if b.Kind != "carpet" {
		return ""
	}
	return fmt.Sprintf("Catches %d%% of tracked dirt", b.Catch)
}

// graveLines is a grave's inspector text: who lies there and the
// epitaph, or "Empty grave". Nil for other buildings.
func GraveLines(b game.BuildingView) []string {
	if b.Kind != "grave" {
		return nil
	}
	if b.Occupant == "" {
		return []string{"Empty grave"}
	}
	lines := []string{"Here lies " + b.Occupant}
	if b.Epitaph != "" {
		lines = append(lines, b.Epitaph)
	}
	return lines
}

func BillLine(bl game.BillView) string {
	var count string
	switch bl.ModeID {
	case BillModeForever:
		count = "forever"
	case BillModeUntil:
		count = fmt.Sprintf("until %d", bl.Target)
	default:
		count = fmt.Sprintf("%d/%d", bl.Done, bl.Target)
	}
	line := fmt.Sprintf("%s  %s", strings.TrimPrefix(strings.TrimPrefix(bl.Label, "Cook "), "Make "), count)
	if bl.Waiting && bl.Status == game.BillHaveEnough {
		line += fmt.Sprintf("  (Have enough - resumes at %d)", bl.ResumeAt)
	} else if bl.Status != "" {
		line += "  (" + bl.Status + ")"
	}
	return line
}

// applyBuildingClick handles bldg.* and bill.* ids.
func (a Building) ApplyBuildingClick(id string) {
	b, ok := a.SelectedBuilding()
	if !ok || a.sim() == nil {
		return
	}
	if id == "bldg.close" {
		a.ClearBuilding()
		return
	}
	if id == "bldg.deconstruct" {
		a.shell.Enqueue(command.Command{Kind: command.DesignateDeconstruct, X: b.X, Y: b.Y,
			X2: b.X + max(b.W, 1) - 1, Y2: b.Y + max(b.H, 1) - 1})
		return
	}
	if a.applyBedClick(id, b) || a.ApplyCasketClick(id, b) || a.applyPenClick(id, b) {
		return
	}
	switch id {
	case "bldg.move":
		a.shell.Move(b, false)
		return
	case "bldg.copy":
		a.shell.Do(ports.PresetBuildTool{Kind: b.Kind, Stuff: b.Stuff, Facing: b.Facing})
		return
	case "bldg.cancelmove":
		a.shell.Move(b, true)
		return
	}
	if id == BillPickBtn {
		a.openBillPicker(b)
		return
	}
	parts := strings.Split(id, ".")
	if len(parts) != 3 || parts[0] != "bill" {
		return
	}
	n, err := strconv.Atoi(parts[2])
	if err != nil {
		return
	}
	if parts[1] == "add" {
		a.shell.Enqueue(command.Command{Kind: command.AddBill, X: b.X, Y: b.Y, Recipe: uint8(n),
			Mode: BillModeDo, Count: 1})
		return
	}
	if n < 0 || n >= len(b.Bills) {
		return
	}
	bl := b.Bills[n]
	set := command.Command{Kind: command.SetBill, X: b.X, Y: b.Y, Index: n, Mode: bl.ModeID,
		Count: bl.Target, Enabled: bl.Suspended}
	switch parts[1] {
	case "minus":
		set.Count = max(1, bl.Target-a.qtyStep())
	case "plus":
		set.Count = min(BillMaxCount, bl.Target+a.qtyStep())
	case "mode":
		set.Mode = (bl.ModeID + 1) % 3
	case "pause":
		set.Enabled = !bl.Suspended
	case "del":
		a.shell.Enqueue(command.Command{Kind: command.RemoveBill, X: b.X, Y: b.Y, Index: n})
		return
	default:
		return
	}
	a.shell.Enqueue(set)
}

// Bed owners and the prisoner mark (MAIN_UI_SCREENS §10 bed inspector,
// ENG-003 M5): one owner row per sleeping slot with a picker that sends
// AssignBedOwner, and a "For prisoners" toggle that sends
// SetBedForPrisoners. The rows read BuildingView's claimants back.

const BedOwnerModalID = "bed.owner"

// bedMedicalID is the Medical toggle's id: a medical bed is kept for the
// sick and injured (PLAY-020) and has no owner.
const BedMedicalID = "bldg.bed.medical"

// bedPrisonReader is the read model for the prisoner mark (consumer-owned;
// the game implements it). Without it the toggle is disabled.
type bedPrisonReader interface {
	BedForPrisoners(bed entity.EntityID) bool
}

// bedSlots is how many sleepers bed kind b holds (0: not a bed).
func bedSlots(b game.BuildingView) int {
	switch b.Kind {
	case "double bed":
		return 2
	case "bed", "hospital bed", "sleeping spot":
		return 1
	}
	return 0
}

// bedOwner is who holds slot of b (0: nobody).
func BedOwner(b game.BuildingView, slot int) entity.EntityID {
	if slot == 1 {
		return b.Claimant2
	}
	return b.Claimant
}

func BedOwnerID(slot int) string { return "bldg.bed.owner." + strconv.Itoa(slot) }

// bedOwnerName is who holds slot of b ("nobody" when free).
func (a Building) bedOwnerName(b game.BuildingView, slot int) string {
	id := BedOwner(b, slot)
	if id == 0 {
		return "nobody"
	}
	if who := a.PawnName(id); who != "" {
		return who
	}
	return "someone"
}

// bedLines are a bed's owner lines, the prisoner mark and the medical mark.
func (a Building) bedLines(b game.BuildingView) []string {
	n := bedSlots(b)
	var out []string
	for slot := range n {
		label := "Owner: " + a.bedOwnerName(b, slot)
		if n > 1 {
			label = fmt.Sprintf("Owner %d: %s", slot+1, a.bedOwnerName(b, slot))
		}
		out = append(out, label)
	}
	if rd, ok := a.sim().(bedPrisonReader); ok && n > 0 && rd.BedForPrisoners(b.ID) {
		out = append(out, "For prisoners")
	}
	if n > 0 && b.Medical {
		out = append(out, "Medical bed")
	}
	return out
}

// bedCommands are a bed's owner buttons (one per sleeping slot; each opens
// the owner picker, and none works on a medical bed), the "For prisoners"
// toggle and the "Medical" toggle.
func (a Building) bedCommands(b game.BuildingView) []Button {
	n := bedSlots(b)
	if n == 0 {
		return nil
	}
	var out []Button
	for slot := range n {
		label := "Owner"
		if n > 1 {
			label = fmt.Sprintf("Owner %d", slot+1)
		}
		owner := Button{ID: BedOwnerID(slot), Label: label,
			Desc: "Choose who sleeps here. Now: " + a.bedOwnerName(b, slot) + "."}
		if b.Medical {
			owner.Disabled, owner.Reason = true, "Medical beds have no owner"
		}
		out = append(out, owner)
	}
	pr := Button{ID: "bldg.bed.prisoners", Label: "Prisoners", Toggle: true,
		Desc: "Only prisoners sleep here."}
	if rd, ok := a.sim().(bedPrisonReader); ok {
		pr.On = rd.BedForPrisoners(b.ID)
	} else {
		pr.Disabled, pr.Reason = true, "Prisoner beds are not available in this game."
	}
	med := Button{ID: BedMedicalID, Label: "Medical", Icon: "hospital_bed", Toggle: true, On: b.Medical,
		Desc: "Only the sick and injured rest here. Healthy colonists sleep elsewhere."}
	return append(out, pr, med)
}

// pawnName is the colonist's label in the snapshot ("" when unknown).
func (a Building) PawnName(id entity.EntityID) string {
	for _, p := range a.shell.Pawns() {
		if p.ID == id {
			return a.shell.PawnLabel(p)
		}
	}
	return ""
}

// applyBedClick opens the owner picker or flips the prisoner mark.
func (a Building) applyBedClick(id string, b game.BuildingView) bool {
	if bedSlots(b) == 0 {
		return false
	}
	if id == "bldg.bed.prisoners" {
		rd, ok := a.sim().(bedPrisonReader)
		if ok {
			a.shell.Enqueue(command.Command{Kind: command.SetBedForPrisoners, Target: uint64(b.ID), Enabled: !rd.BedForPrisoners(b.ID)})
		}
		return true
	}
	if id == BedMedicalID {
		a.shell.Enqueue(command.Command{Kind: command.SetBedMedical, Target: uint64(b.ID), Enabled: !b.Medical})
		return true
	}
	s, ok := strings.CutPrefix(id, "bldg.bed.owner.")
	if !ok {
		return false
	}
	if slot, err := strconv.Atoi(s); err == nil && slot >= 0 && slot < bedSlots(b) {
		a.openBedOwnerPicker(b.ID, slot)
	}
	return true
}

// openBedOwnerPicker lists the living colonists (and nobody) for slot.
func (a Building) openBedOwnerPicker(bed entity.EntityID, slot int) {
	a.kit().Push(uikit.Modal{
		ID: BedOwnerModalID, Title: "Bed owner", W: 320, H: 420,
		Draw:    func(body gfx.Rect) { a.bedOwnerBody(body, bed, slot) },
		Buttons: []uikit.ModalButton{{ID: "close", Label: "Cancel", Style: uikit.BtnNormal, Closes: true}},
	})
}

// bedOwnerPickID is the picker button for owner (0: nobody).
func BedOwnerPickID(owner entity.EntityID) string {
	return "bed.owner.pick." + strconv.FormatUint(uint64(owner), 10)
}

func (a Building) bedOwnerBody(body gfx.Rect, bed entity.EntityID, slot int) {
	if a.sim() == nil {
		return
	}
	rows := []struct {
		id   entity.EntityID
		name string
	}{{0, "Nobody"}}
	for _, p := range a.shell.Pawns() {
		if p.Status != "dead" {
			rows = append(rows, struct {
				id   entity.EntityID
				name string
			}{p.ID, a.shell.PawnLabel(p)})
		}
	}
	area := a.kit().ScrollArea("bed.owner.rows", body, float32(len(rows)*bedRowH), uikit.ScrollOpts{Step: bedRowH * 2})
	y := area.View.Y - area.Off
	for _, row := range rows {
		rr := gfx.Rect{X: area.View.X, Y: y, W: area.View.W, H: bedRowH - 4}
		y += bedRowH
		if rr.Y < area.View.Y || rr.Bottom() > area.View.Bottom() {
			continue
		}
		if a.kit().Button(BedOwnerPickID(row.id), rr, row.name, uikit.BtnOpts{}) {
			a.assignBedOwner(bed, slot, row.id)
		}
	}
}

// assignBedOwner sends AssignBedOwner and closes the picker.
func (a Building) assignBedOwner(bed entity.EntityID, slot int, owner entity.EntityID) {
	a.shell.Enqueue(command.Command{Kind: command.AssignBedOwner, Target: uint64(bed), Pawn: uint64(owner), Index: slot})
	if a.modalTop() == BedOwnerModalID {
		a.kit().CloseTop()
	}
}
