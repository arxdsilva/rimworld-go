package ebiten

import (
	"fmt"
	"strconv"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// Building selection and the bench's Bills tab (MAIN_UI_SCREENS §10
// inspector, UI checklist §14.1 workbench inspector). PLAY-018: the pane
// and the command buttons are cmds_build.go; this file keeps the selection,
// the click handlers, the Bills tab opened above the pane, and the bed
// owner picker. Bills are typed AddBill/SetBill/RemoveBill commands; the
// tab only reads BillView.

const billRowH = 26

const (
	billPickBtn  = "bill.pick"
	billPickID   = "bill.picker"
	billPickList = "bill.picker.list"
)

// openBillPicker opens the recipe pick list for bench b; locked recipes
// are listed but cannot be picked.
func (a *App) openBillPicker(b game.BuildingView) {
	recipes := a.recipesFor(b.Kind)
	sel := -1
	a.uiPushModal(modal{
		id: billPickID, title: "Add bill", w: 360, h: 380,
		draw: func(a *App, r rect) {
			if len(recipes) == 0 {
				a.uiLabel(r.X, r.Y, "This bench has no recipes.", dimText())
				return
			}
			a.uiList(billPickList, r, len(recipes), &sel, listSpec{
				RowH:  26,
				Label: func(i int) string { return recipes[i].Label },
				Disabled: func(i int) string {
					if p := a.lockedBy("recipe:" + recipes[i].Name); p != "" {
						return "Needs research: " + p
					}
					return ""
				},
			})
		},
		buttons: []modalButton{
			{id: "bill.picker.ok", label: "Add", style: btnPrimary, closes: true,
				disabled: func() string {
					if sel < 0 || sel >= len(recipes) {
						return "Pick a recipe first."
					}
					return ""
				},
				onClick: func() {
					if sel >= 0 && sel < len(recipes) {
						a.applyBuildingClick("bill.add." + strconv.Itoa(int(recipes[sel].ID)))
					}
				}},
			{id: "bill.picker.cancel", label: "Cancel", closes: true},
		},
	})
}

// billMaxCount caps a bill's target.
const billMaxCount = 9999

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

func (a *App) qtyStep() int { return qtyStep(a.in.ctrl(), a.in.shift()) }

// recipe.Mode values (mirrored; the adapter only sends the numbers).
const (
	billModeDo      = 0
	billModeForever = 1
	billModeUntil   = 2
)

func (a *App) selectedBuilding() (game.BuildingView, bool) {
	if !a.hasBuilding {
		return game.BuildingView{}, false
	}
	return a.buildingView(a.selBuilding)
}

// buildingAt is the completed object-layer building covering (x, y),
// falling back to a floor.
func buildingAt(w game.WorldView, x, y int) (game.BuildingView, bool) {
	var floor game.BuildingView
	found := false
	for _, b := range w.Buildings {
		bw, bh := max(b.W, 1), max(b.H, 1)
		if x < b.X || y < b.Y || x >= b.X+bw || y >= b.Y+bh {
			continue
		}
		if !b.Floor && b.Kind != bridgeKind { // a floor or bridge: only when nothing stands on it
			return b, true
		}
		floor, found = b, true
	}
	return floor, found
}

func (a *App) selectBuildingAt(tx, ty int) bool {
	b, ok := buildingAt(a.renderer.world(), tx, ty)
	if !ok {
		a.hasBuilding = false
		return false
	}
	a.selectCand(selCand{selBuilding, b.ID})
	return true
}

func (a *App) clearBuilding() { a.hasBuilding = false }

func buildingTitle(b game.BuildingView) string {
	if b.Ruin {
		return ruinTitle(b)
	}
	if b.Kind == bridgeKind {
		return "Wooden bridge"
	}
	if t := casketTitle(b); t != "" {
		return t
	}
	if b.Kind == "carpet" && strings.HasSuffix(b.Stuff, " carpet") {
		return strings.ToUpper(b.Stuff[:1]) + b.Stuff[1:] // "Straw carpet"
	}
	return materialTitle(b.Kind, b.Stuff) // "Wall", "Granite wall", "Steel door"
}

func (a *App) recipesFor(kind string) []game.RecipeView {
	var out []game.RecipeView
	for _, r := range game.Recipes() {
		if r.Bench == kind {
			out = append(out, r)
		}
	}
	return out
}

// drawBuildingMark marks the selected building on the map. The pane
// (cmds_build.go) is the building's panel now.
func (a *App) drawBuildingMark(screen *ebiten.Image) {
	b, ok := a.selectedBuilding()
	if !ok || a.workOpen || a.archOpen {
		return
	}
	a.drawBuildingSelection(screen, b)
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
	return 2*paneInset + float32(max(n, 1))*billRowH + bodyGap + billBtnH
}

// billsTab is the Bills tab of bench b.
func (a *App) billsTab(b game.BuildingView) paneTab {
	id := b.ID
	return paneTab{ID: "inspector.bills", Label: "Bills", WantW: billsPanelW, WantH: billsPanelH(len(b.Bills)),
		Declare: func(r rect) { a.billsTabUI(id, r) }}
}

// billsLayout splits the Bills tab's inner rectangle r: the scrolling list
// of bills and the Add bill button under it.
func billsLayout(r rect) (list, add rect) {
	list, foot := r.splitV(-(billBtnH + bodyGap))
	return list, rect{foot.X, foot.Y + bodyGap, billAddW, billBtnH}
}

// billsBody lays out bench b's bills, viewW wide: a line of text and the
// five buttons (fewer, more, mode, pause, delete) per bill.
func billsBody(b game.BuildingView, viewW float32) tabBody {
	var body tabBody
	if len(b.Bills) == 0 {
		body.labels = []tabLabel{{text: "No bills yet - add one below.", x: 8, y: 4, w: viewW - 8, st: tsBody}}
		body.h = billRowH
		return body
	}
	x := viewW - billBtnsW
	for i, bl := range b.Bills {
		y := float32(i) * billRowH
		ix := strconv.Itoa(i)
		if bl.Status != "" {
			body.fills = append(body.fills, tabFill{rect{0, y, 4, billBtnH}, uiBtnDisabled})
		}
		body.labels = append(body.labels, tabLabel{text: billLine(bl), x: 8, y: y + 2, w: x - 8 - bodyGap, st: tsBody})
		body.buttons = append(body.buttons,
			button{id: "bill.minus." + ix, label: "-", x: x, y: y, w: 22, h: billBtnH, tip: "Fewer (Ctrl x10, Shift x100, both x1000)"},
			button{id: "bill.plus." + ix, label: "+", x: x + 24, y: y, w: 22, h: billBtnH, tip: "More (Ctrl x10, Shift x100, both x1000)"},
			button{id: "bill.mode." + ix, label: "M", x: x + 48, y: y, w: 22, h: billBtnH},
			button{id: "bill.pause." + ix, label: "||", x: x + 72, y: y, w: 26, h: billBtnH, selected: bl.Suspended},
			button{id: "bill.del." + ix, label: "x", x: x + 100, y: y, w: 22, h: billBtnH},
		)
	}
	body.h = float32(len(b.Bills)) * billRowH
	return body
}

// billsTabUI declares the Bills tab of bench id in r, the expanded panel's
// inner rectangle. One "+ Add bill" button opens a pick list, so every
// recipe is reachable however many the bench has.
func (a *App) billsTabUI(id entity.EntityID, r rect) {
	b, ok := a.buildingView(id)
	if !ok || !b.Bench {
		return
	}
	list, add := billsLayout(r)
	a.declareTabBody("bill.rows", list, billsBody(b, list.W-scrollBarW), a.applyBuildingClick)
	if a.uiButton(billPickBtn, add, "+ Add bill", btnOpts{Tip: "Add bill\nPick a recipe for this bench."}) {
		a.applyBuildingClick(billPickBtn)
	}
}

// swapLine is the inspector text while an in-place material swap is
// pending on the building. "" otherwise.
func swapLine(b game.BuildingView) string {
	if b.SwapTo == "" {
		return ""
	}
	return fmt.Sprintf("Changing to %s (%d%%)", b.SwapTo, b.SwapPct)
}

// carpetLine is a carpet's inspector text: the share of tracked dirt it
// wipes off boots. "" for other buildings.
func carpetLine(b game.BuildingView) string {
	if b.Kind != "carpet" {
		return ""
	}
	return fmt.Sprintf("Catches %d%% of tracked dirt", b.Catch)
}

// graveLines is a grave's inspector text: who lies there and the
// epitaph, or "Empty grave". Nil for other buildings.
func graveLines(b game.BuildingView) []string {
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

func billLine(bl game.BillView) string {
	var count string
	switch bl.ModeID {
	case billModeForever:
		count = "forever"
	case billModeUntil:
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

func (a *App) drawBuildingSelection(screen *ebiten.Image, b game.BuildingView) {
	tile := a.renderer.tilePx()
	sx, sy := a.renderer.worldToScreen(b.X, b.Y)
	w, h := float32(float64(max(b.W, 1))*tile), float32(float64(max(b.H, 1))*tile)
	vector.StrokeRect(screen, sx+1, sy+1, w-2, h-2, 2, uiAccent, false)
	if b.HasSpot {
		px, py := a.renderer.worldToScreen(b.SpotX, b.SpotY)
		vector.StrokeRect(screen, px+tile32(tile)*0.3, py+tile32(tile)*0.3, tile32(tile)*0.4, tile32(tile)*0.4, 1, uiAccent, false)
	}
}

func tile32(t float64) float32 { return float32(t) }

// applyBuildingClick handles bldg.* and bill.* ids.
func (a *App) applyBuildingClick(id string) {
	b, ok := a.selectedBuilding()
	if !ok || a.renderer == nil {
		return
	}
	if id == "bldg.close" {
		a.clearBuilding()
		return
	}
	if id == "bldg.deconstruct" {
		a.renderer.enqueue(command.Command{Kind: command.DesignateDeconstruct, X: b.X, Y: b.Y,
			X2: b.X + max(b.W, 1) - 1, Y2: b.Y + max(b.H, 1) - 1})
		return
	}
	if a.applyBedClick(id, b) || a.applyCasketClick(id, b) || a.applyPenClick(id, b) {
		return
	}
	switch id {
	case "bldg.move":
		a.startMove(b)
		return
	case "bldg.copy":
		a.presetArchTool(b.Kind, b.Stuff, b.Facing)
		return
	case "bldg.cancelmove":
		if f, ok := pendingMoveFrame(a.renderer.world(), b.ID); ok {
			a.renderer.enqueue(moveCancelRect(f))
		}
		return
	}
	if id == billPickBtn {
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
		a.renderer.enqueue(command.Command{Kind: command.AddBill, X: b.X, Y: b.Y, Recipe: uint8(n),
			Mode: billModeDo, Count: 1})
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
		set.Count = min(billMaxCount, bl.Target+a.qtyStep())
	case "mode":
		set.Mode = (bl.ModeID + 1) % 3
	case "pause":
		set.Enabled = !bl.Suspended
	case "del":
		a.renderer.enqueue(command.Command{Kind: command.RemoveBill, X: b.X, Y: b.Y, Index: n})
		return
	default:
		return
	}
	a.renderer.enqueue(set)
}

func hasBuildTool(kind string) bool {
	for _, t := range allArchTools() {
		if t.build == kind {
			return true
		}
	}
	return false
}

// Bed owners and the prisoner mark (MAIN_UI_SCREENS §10 bed inspector,
// ENG-003 M5): one owner row per sleeping slot with a picker that sends
// AssignBedOwner, and a "For prisoners" toggle that sends
// SetBedForPrisoners. The rows read BuildingView's claimants back.

const bedOwnerModalID = "bed.owner"

// bedMedicalID is the Medical toggle's id: a medical bed is kept for the
// sick and injured (PLAY-020) and has no owner.
const bedMedicalID = "bldg.bed.medical"

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
func bedOwner(b game.BuildingView, slot int) entity.EntityID {
	if slot == 1 {
		return b.Claimant2
	}
	return b.Claimant
}

func bedOwnerID(slot int) string { return "bldg.bed.owner." + strconv.Itoa(slot) }

// bedOwnerName is who holds slot of b ("nobody" when free).
func (a *App) bedOwnerName(b game.BuildingView, slot int) string {
	id := bedOwner(b, slot)
	if id == 0 {
		return "nobody"
	}
	if who := a.pawnName(id); who != "" {
		return who
	}
	return "someone"
}

// bedLines are a bed's owner lines, the prisoner mark and the medical mark.
func (a *App) bedLines(b game.BuildingView) []string {
	n := bedSlots(b)
	var out []string
	for slot := range n {
		label := "Owner: " + a.bedOwnerName(b, slot)
		if n > 1 {
			label = fmt.Sprintf("Owner %d: %s", slot+1, a.bedOwnerName(b, slot))
		}
		out = append(out, label)
	}
	if rd, ok := a.renderer.game.(bedPrisonReader); ok && n > 0 && rd.BedForPrisoners(b.ID) {
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
func (a *App) bedCommands(b game.BuildingView) []cmdButton {
	n := bedSlots(b)
	if n == 0 {
		return nil
	}
	var out []cmdButton
	for slot := range n {
		label := "Owner"
		if n > 1 {
			label = fmt.Sprintf("Owner %d", slot+1)
		}
		owner := cmdButton{ID: bedOwnerID(slot), Label: label,
			Desc: "Choose who sleeps here. Now: " + a.bedOwnerName(b, slot) + "."}
		if b.Medical {
			owner.Disabled, owner.Reason = true, "Medical beds have no owner"
		}
		out = append(out, owner)
	}
	pr := cmdButton{ID: "bldg.bed.prisoners", Label: "Prisoners", Toggle: true,
		Desc: "Only prisoners sleep here."}
	if rd, ok := a.renderer.game.(bedPrisonReader); ok {
		pr.On = rd.BedForPrisoners(b.ID)
	} else {
		pr.Disabled, pr.Reason = true, "Prisoner beds are not available in this game."
	}
	med := cmdButton{ID: bedMedicalID, Label: "Medical", Icon: "hospital_bed", Toggle: true, On: b.Medical,
		Desc: "Only the sick and injured rest here. Healthy colonists sleep elsewhere."}
	return append(out, pr, med)
}

// pawnName is the colonist's label in the snapshot ("" when unknown).
func (a *App) pawnName(id entity.EntityID) string {
	for _, p := range a.renderer.snapshot() {
		if p.ID == id {
			return pawnLabel(p)
		}
	}
	return ""
}

// applyBedClick opens the owner picker or flips the prisoner mark.
func (a *App) applyBedClick(id string, b game.BuildingView) bool {
	if bedSlots(b) == 0 {
		return false
	}
	if id == "bldg.bed.prisoners" {
		rd, ok := a.renderer.game.(bedPrisonReader)
		if ok {
			a.renderer.enqueue(command.Command{Kind: command.SetBedForPrisoners, Target: uint64(b.ID), Enabled: !rd.BedForPrisoners(b.ID)})
		}
		return true
	}
	if id == bedMedicalID {
		a.renderer.enqueue(command.Command{Kind: command.SetBedMedical, Target: uint64(b.ID), Enabled: !b.Medical})
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
func (a *App) openBedOwnerPicker(bed entity.EntityID, slot int) {
	a.uiPushModal(modal{
		id: bedOwnerModalID, title: "Bed owner", w: 320, h: 420,
		draw:    func(a *App, body rect) { a.bedOwnerBody(body, bed, slot) },
		buttons: []modalButton{{id: "close", label: "Cancel", style: btnNormal, closes: true}},
	})
}

// bedOwnerPickID is the picker button for owner (0: nobody).
func bedOwnerPickID(owner entity.EntityID) string {
	return "bed.owner.pick." + strconv.FormatUint(uint64(owner), 10)
}

func (a *App) bedOwnerBody(body rect, bed entity.EntityID, slot int) {
	if a.renderer == nil {
		return
	}
	rows := []struct {
		id   entity.EntityID
		name string
	}{{0, "Nobody"}}
	for _, p := range a.renderer.snapshot() {
		if p.Status != "dead" {
			rows = append(rows, struct {
				id   entity.EntityID
				name string
			}{p.ID, pawnLabel(p)})
		}
	}
	area := a.uiScrollArea("bed.owner.rows", body, float32(len(rows)*assignRowH), scrollOpts{Step: assignRowH * 2})
	y := area.View.Y - area.Off
	for _, row := range rows {
		rr := rect{area.View.X, y, area.View.W, assignRowH - 4}
		y += assignRowH
		if rr.Y < area.View.Y || rr.bottom() > area.View.bottom() {
			continue
		}
		if a.uiButton(bedOwnerPickID(row.id), rr, row.name, btnOpts{}) {
			a.assignBedOwner(bed, slot, row.id)
		}
	}
}

// assignBedOwner sends AssignBedOwner and closes the picker.
func (a *App) assignBedOwner(bed entity.EntityID, slot int, owner entity.EntityID) {
	a.renderer.enqueue(command.Command{Kind: command.AssignBedOwner, Target: uint64(bed), Pawn: uint64(owner), Index: slot})
	if a.uiModalTop() == bedOwnerModalID {
		a.uiCloseTop()
	}
}
