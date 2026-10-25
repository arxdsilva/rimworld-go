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

// Building selection and the bench bills panel (MAIN_UI_SCREENS §10
// inspector, UI checklist §14.1 workbench inspector). Bills are typed
// AddBill/SetBill/RemoveBill commands; the panel only reads BillView.

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
	if !a.hasBuilding || a.renderer == nil {
		return game.BuildingView{}, false
	}
	for _, b := range a.renderer.world().Buildings {
		if b.ID == a.selBuilding {
			return b, true
		}
	}
	return game.BuildingView{}, false
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
		if !b.Floor {
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

func buildingRect() button {
	return button{x: hudPad, y: hudTopH + hudPad, w: 380, h: 330}
}

func (a *App) buildingButtons() []button {
	b, ok := a.selectedBuilding()
	if !ok || a.workOpen || a.archOpen {
		return nil
	}
	r := buildingRect()
	out := []button{{id: "bldg.close", label: "X", x: r.x + r.w - 28, y: r.y + 6, w: 22, h: 22}}
	if b.Kind == steamVentKind {
		return out // natural: no deconstruct, move or copy
	}
	if !b.Deconstruct {
		out = append(out, button{id: "bldg.deconstruct", label: "Deconstruct", x: r.x + r.w - 130, y: r.y + r.h - 32, w: 120, h: 24})
	}
	out = append(out, a.moveCopyButtons(b, r)...)
	out = append(out, a.troughButtons(b, r)...)
	out = append(out, a.bedButtons(b, r)...)
	if !b.Bench {
		return out
	}
	y := r.y + 64
	for i, bl := range b.Bills {
		ix := strconv.Itoa(i)
		x := r.x + r.w - 150
		out = append(out,
			button{id: "bill.minus." + ix, label: "-", x: x, y: y, w: 22, h: 22, tip: "Fewer (Ctrl x10, Shift x100, both x1000)"},
			button{id: "bill.plus." + ix, label: "+", x: x + 24, y: y, w: 22, h: 22, tip: "More (Ctrl x10, Shift x100, both x1000)"},
			button{id: "bill.mode." + ix, label: "M", x: x + 48, y: y, w: 22, h: 22},
			button{id: "bill.pause." + ix, label: "||", x: x + 72, y: y, w: 26, h: 22, selected: bl.Suspended},
			button{id: "bill.del." + ix, label: "x", x: x + 100, y: y, w: 22, h: 22},
		)
		y += billRowH
	}
	// One "Add bill" button opens a pick list, so every recipe is
	// reachable however many the bench has and none crowds Move/Copy.
	out = append(out, button{id: billPickBtn, label: "+ Add bill", x: r.x + 132, y: r.y + r.h - 62, w: 130, h: 22,
		tip: "Add bill\nPick a recipe for this bench."})
	return out
}

func (a *App) drawBuildingInspector(screen *ebiten.Image) {
	b, ok := a.selectedBuilding()
	if !ok || a.workOpen || a.archOpen {
		return
	}
	r := buildingRect()
	drawPanel(screen, r.x, r.y, r.w, r.h)
	drawFitSmall(screen, buildingTitle(b), float64(r.x)+12, float64(r.y)+8, float64(r.w)-48, tsLabel)
	state := fmt.Sprintf("tile (%d, %d)", b.X, b.Y)
	if b.Deconstruct {
		state += "   marked for deconstruction"
	}
	if lbl := roomLabelAt(a.renderer.world(), b.X, b.Y); lbl != "" {
		state += "   " + lbl
	}
	if b.Fuel {
		switch {
		case b.FuelPct == 0:
			state += "   out of fuel"
		case b.Burning:
			state += fmt.Sprintf("   burning, fuel %d%%", b.FuelPct)
		default:
			state += fmt.Sprintf("   banked (warm enough), fuel %d%%", b.FuelPct)
		}
	}
	if b.DryQty > 0 {
		state += fmt.Sprintf("   drying %d %s, %d%%", b.DryQty, b.DryKind, b.DryPct)
	}
	drawFitSmall(screen, state, float64(r.x)+12, float64(r.y)+27, float64(r.w)-24, tsBody)
	for i, ln := range graveLines(b) {
		drawLabel(screen, int(r.x)+12, int(r.y)+46+i*18, ln)
	}
	if ln := ventLine(b); ln != "" {
		drawFitSmall(screen, ln, float64(r.x)+12, float64(r.y)+46, float64(r.w)-24, tsBody)
	}
	if ln := carpetLine(b); ln != "" {
		drawLabel(screen, int(r.x)+12, int(r.y)+46, ln)
	}
	if b.Bench {
		drawText(screen, "Bills", float64(r.x)+12, float64(r.y)+44, tsLabel)
		y := int(r.y) + 64
		for _, bl := range b.Bills {
			line := billLine(bl)
			if bl.Status != "" {
				uiFillRect(screen, r.x+8, float32(y), 4, 22, uiBtnDisabled, false)
			}
			drawFitSmall(screen, line, float64(r.x)+16, float64(y)+2, float64(r.w)-150-22, tsBody)
			y += billRowH
		}
		if len(b.Bills) == 0 {
			drawLabel(screen, int(r.x)+16, y+4, "No bills yet - add one below.")
		}
	}
	a.drawTroughSection(screen, b, r)
	a.drawBedSection(screen, b, r)
	if ln := swapLine(b); ln != "" {
		drawFitSmall(screen, ln, float64(r.x)+12, float64(r.y+r.h)-108, float64(r.w)-24, tsBody)
	}
	if ln := movingLine(b); ln != "" {
		drawFitSmall(screen, ln, float64(r.x)+134, float64(r.y+r.h)-88, float64(r.w)-146, tsBody)
	}
	mx, my := a.currentCursor()
	a.drawButtons(screen, a.buildingButtons(), mx, my)
	a.drawBuildingSelection(screen, b)
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
	if a.applyTroughClick(id, b) || a.applyBedClick(id, b) {
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

// moveCopyButtons are the Move / Copy / Cancel-move buttons along the
// panel's bottom edge.
func (a *App) moveCopyButtons(b game.BuildingView, r button) []button {
	y := r.y + r.h - 32
	mv := button{id: "bldg.move", label: "Move", x: r.x + 12, y: y, w: 56, h: 24}
	if !movableKind(b.Kind, b.Floor) {
		mv.disabled, mv.reason, mv.quiet = true, "can't be moved", true
	}
	cp := button{id: "bldg.copy", label: "Copy", x: r.x + 72, y: y, w: 56, h: 24}
	if p := a.lockedBy("build:" + b.Kind); p != "" {
		cp.disabled, cp.reason, cp.quiet = true, "needs research: "+p, true
	} else if !hasBuildTool(b.Kind) {
		cp.disabled, cp.reason, cp.quiet = true, "not buildable", true
	}
	out := []button{mv, cp}
	if b.MovingTo != nil {
		out = append(out, button{id: "bldg.cancelmove", label: "Cancel move", x: r.x + 12, y: y - 30, w: 112, h: 24})
	}
	return out
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

const bedRowY, bedRowH = 46, 28

func (a *App) bedButtons(b game.BuildingView, r button) []button {
	n := bedSlots(b)
	if n == 0 {
		return nil
	}
	var out []button
	for slot := range n {
		out = append(out, button{id: bedOwnerID(slot), label: "Set owner", x: r.x + r.w - 112, y: r.y + bedRowY + float32(slot*bedRowH),
			w: 100, h: 22, tip: "Choose who sleeps here"})
	}
	pr := button{id: "bldg.bed.prisoners", label: "For prisoners", x: r.x + 12, y: r.y + bedRowY + float32(n*bedRowH) + 6, w: 130, h: 22}
	if rd, ok := a.renderer.game.(bedPrisonReader); ok {
		pr.selected = rd.BedForPrisoners(b.ID)
		pr.tip = "Only prisoners sleep here"
	} else {
		pr.disabled, pr.reason, pr.quiet = true, "Prisoner beds are not available in this game.", true
	}
	return append(out, pr)
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

func (a *App) drawBedSection(screen *ebiten.Image, b game.BuildingView, r button) {
	n := bedSlots(b)
	for slot := range n {
		who := "nobody"
		if id := bedOwner(b, slot); id != 0 {
			if who = a.pawnName(id); who == "" {
				who = "someone"
			}
		}
		label := "Owner: " + who
		if n > 1 {
			label = fmt.Sprintf("Owner %d: %s", slot+1, who)
		}
		drawFitSmall(screen, label, float64(r.x)+12, float64(r.y)+bedRowY+float64(slot*bedRowH)+2, float64(r.w)-140, tsBody)
	}
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
