package ebiten

import (
	"fmt"
	"strconv"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

// Building selection and the bench bills panel (MAIN_UI_SCREENS §10
// inspector, UI checklist §14.1 workbench inspector). Bills are typed
// AddBill/SetBill/RemoveBill commands; the panel only reads BillView.

const billRowH = 26

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
	a.selBuilding, a.hasBuilding = b.ID, true
	return true
}

func (a *App) clearBuilding() { a.hasBuilding = false }

func buildingTitle(b game.BuildingView) string {
	name := strings.ToUpper(b.Kind[:1]) + b.Kind[1:]
	if b.Stuff == "stone" {
		name = "Stone " + b.Kind
	}
	return name
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
	if !b.Deconstruct {
		out = append(out, button{id: "bldg.deconstruct", label: "Deconstruct", x: r.x + r.w - 130, y: r.y + r.h - 32, w: 120, h: 24})
	}
	if !b.Bench {
		return out
	}
	y := r.y + 64
	for i, bl := range b.Bills {
		ix := strconv.Itoa(i)
		x := r.x + r.w - 150
		out = append(out,
			button{id: "bill.minus." + ix, label: "-", x: x, y: y, w: 22, h: 22},
			button{id: "bill.plus." + ix, label: "+", x: x + 24, y: y, w: 22, h: 22},
			button{id: "bill.mode." + ix, label: "M", x: x + 48, y: y, w: 22, h: 22},
			button{id: "bill.pause." + ix, label: "||", x: x + 72, y: y, w: 26, h: 22, selected: bl.Suspended},
			button{id: "bill.del." + ix, label: "x", x: x + 100, y: y, w: 22, h: 22},
		)
		y += billRowH
	}
	y += 8
	for _, rc := range a.recipesFor(b.Kind) {
		out = append(out, button{id: "bill.add." + strconv.Itoa(int(rc.ID)), label: "+ " + rc.Label,
			x: r.x + 12, y: y, w: 230, h: 22})
		y += 26
	}
	return out
}

func (a *App) drawBuildingInspector(screen *ebiten.Image) {
	b, ok := a.selectedBuilding()
	if !ok || a.workOpen || a.archOpen {
		return
	}
	r := buildingRect()
	drawPanel(screen, r.x, r.y, r.w, r.h)
	drawLabel(screen, int(r.x)+12, int(r.y)+10, buildingTitle(b))
	state := fmt.Sprintf("tile (%d, %d)", b.X, b.Y)
	if b.Deconstruct {
		state += "   marked for deconstruction"
	}
	if lbl := roomLabelAt(a.renderer.world(), b.X, b.Y); lbl != "" {
		state += "   " + lbl
	}
	drawLabel(screen, int(r.x)+12, int(r.y)+28, state)
	if b.Bench {
		drawLabel(screen, int(r.x)+12, int(r.y)+46, "Bills")
		y := int(r.y) + 64
		for _, bl := range b.Bills {
			line := billLine(bl)
			if bl.Status != "" {
				vector.FillRect(screen, r.x+8, float32(y), 4, 22, uiBtnDisabled, false)
			}
			drawLabel(screen, int(r.x)+16, y+4, line)
			y += billRowH
		}
		if len(b.Bills) == 0 {
			drawLabel(screen, int(r.x)+16, y+4, "No bills yet - add one below.")
		}
	}
	mx, my := a.currentCursor()
	a.drawButtons(screen, a.buildingButtons(), mx, my)
	a.drawBuildingSelection(screen, b)
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
	if bl.Status != "" {
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
		if bl.Target > 1 {
			set.Count = bl.Target - 1
		}
	case "plus":
		set.Count = bl.Target + 1
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
