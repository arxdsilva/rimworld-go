package ebiten

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

// Item panel (UI-011, MAIN_UI §10): what the stack is and its condition,
// and per-colonist Equip / Wear / Haul / Eat orders.

// itemRect is the item panel's frame (the inspector's top-left slot).
func itemRect() button { return button{x: hudPad, y: hudTopH + hudPad, w: 300, h: 200} }

// maxItemChips is how many colonist chips fit in one action row.
const maxItemChips = 3

var (
	weaponKinds = map[string]bool{"club": true, "rifle": true, "knife": true, "spear": true}
	apparelKind = map[string]bool{"parka": true, "wool parka": true, "tuque": true, "duster": true}
	foodKinds   = map[string]bool{"food": true, "berries": true, "potatoes": true, "meat": true, "meal": true,
		"corn": true, "wheat": true, "beans": true, "lettuce": true, "radishes": true, "pumpkins": true}
)

// isCarpetItem is a crafted carpet ("cloth carpet"), installed from the
// Architect.
func isCarpetItem(it game.ItemView) bool { return strings.HasSuffix(it.Kind, " carpet") }

func isHumanBody(it game.ItemView) bool { return it.Kind == "human corpse" || it.Kind == "bones" }

// itemCategory is the header's category word.
func itemCategory(it game.ItemView) string {
	switch {
	case weaponKinds[it.Kind]:
		return "weapon"
	case apparelKind[it.Kind]:
		return "apparel"
	case foodKinds[it.Kind]:
		return "food"
	case isHumanBody(it) || strings.HasSuffix(it.Kind, " corpse"):
		return "body"
	case isCarpetItem(it):
		return "furnishing"
	}
	return "material"
}

// itemTitle is "Rifle", "Wood ×25", "Body of Ada" or "Bones of Ada".
func itemTitle(it game.ItemView) string {
	switch {
	case it.Kind == "human corpse":
		if it.OfName != "" {
			return "Body of " + it.OfName
		}
		return "Body"
	case it.Kind == "bones":
		if it.OfName != "" {
			return "Bones of " + it.OfName
		}
		return "Bones"
	}
	t := capitalize(it.Kind)
	if it.Qty > 1 {
		t += fmt.Sprintf(" ×%d", it.Qty)
	}
	return t
}

// inStockpile reports whether (x, y) lies in any stockpile.
func inStockpile(w game.WorldView, x, y int) bool {
	for _, z := range w.Zones {
		if z.Kind != game.ZoneStockpile || x < z.MinX || x > z.MaxX || y < z.MinY || y > z.MaxY {
			continue
		}
		for _, c := range z.Cells {
			if c[0] == x && c[1] == y {
				return true
			}
		}
	}
	return false
}

// itemLines are the body lines: condition, then storage.
func itemLines(it game.ItemView, stored bool) []string {
	var out []string
	switch {
	case it.Rotten:
		out = append(out, "Rotten")
	case it.Dried:
		out = append(out, "Dried")
	case it.FreshDays >= 0:
		out = append(out, fmt.Sprintf("Fresh for %.0f days", it.FreshDays))
	}
	if isCarpetItem(it) {
		out = append(out, fmt.Sprintf("Catches %d%% of tracked dirt", it.Catch),
			"Install from Architect > Furniture > Carpet")
	}
	if it.Forbidden {
		out = append(out, "Forbidden: colonists leave it alone")
	}
	if stored {
		return append(out, "In a stockpile")
	}
	return append(out, "Not stored")
}

// itemChipRow is one labelled row of colonist chips.
type itemChipRow struct {
	Verb  string // "Equip:", "Wear:", "Eat:", "Haul:"
	Chips []colonistAction
}

// itemRows builds the action rows for it: the verb row for its category,
// then Haul when it is not in a stockpile.
func (a *App) itemRows(it game.ItemView, stored bool) []itemChipRow {
	var rows []itemChipRow
	add := func(verb string, m func(game.OrderOption) bool) {
		if acts := a.colonistActions(it.X, it.Y, m); len(acts) > 0 {
			if len(acts) > maxItemChips {
				acts = acts[:maxItemChips]
			}
			rows = append(rows, itemChipRow{verb, acts})
		}
	}
	equip := orderIs(command.OrderEquip)
	switch itemCategory(it) {
	case "weapon":
		add("Equip:", equip)
	case "apparel":
		add("Wear:", equip)
	case "food":
		add("Eat:", orderIs(command.OrderEat))
	}
	if !stored {
		add("Haul:", orderIs(command.OrderHaul))
	}
	return rows
}

// itemPanelButtons lays out the close button and the chips; chip n gets
// the id "item.act.<n>", counted row by row.
func itemPanelButtons(r button, rows []itemChipRow) []button {
	out := []button{{id: "item.close", label: "X", x: r.x + r.w - 28, y: r.y + 6, w: 22, h: 22}}
	n := 0
	for i, row := range rows {
		y := r.y + r.h - 30 - float32(len(rows)-1-i)*26
		for j, c := range row.Chips {
			out = append(out, button{id: "item.act." + strconv.Itoa(n), label: chipLabel(c),
				x: r.x + 56 + float32(j)*84, y: y, w: 82, h: 20, disabled: c.Disabled, reason: c.Reason, quiet: true})
			n++
		}
	}
	return out
}

func flattenChips(rows []itemChipRow) []colonistAction {
	var out []colonistAction
	for _, r := range rows {
		out = append(out, r.Chips...)
	}
	return out
}

// itemForbidButton toggles the stack's forbidden flag (SetForbidden).
func itemForbidButton(r button, it game.ItemView) button {
	label := "Forbid"
	if it.Forbidden {
		label = "Allow"
	}
	return button{id: "item.forbid", label: label, x: r.x + r.w - 96, y: r.y + 34, w: 84, h: 22}
}

// itemButtons are the item panel's buttons (ids "item.*").
func (a *App) itemButtons() []button {
	it, ok := a.selectedItem()
	if !ok || a.workOpen || a.archOpen {
		return nil
	}
	stored := inStockpile(a.renderer.world(), it.X, it.Y)
	r := itemRect()
	return append(itemPanelButtons(r, a.itemRows(it, stored)), itemForbidButton(r, it))
}

// drawItemInspector draws the item panel when an item is selected.
func (a *App) drawItemInspector(screen *ebiten.Image) {
	it, ok := a.selectedItem()
	if !ok || a.workOpen || a.archOpen {
		return
	}
	w := a.renderer.world()
	stored := inStockpile(w, it.X, it.Y)
	r := itemRect()
	drawPanel(screen, r.x, r.y, r.w, r.h)
	if a.renderer.art != nil {
		key := "ui_icon/" + strings.ReplaceAll(it.Kind, " ", "_") // "cloth carpet" -> cloth_carpet
		cs := noCS
		if !a.renderer.art.has(key) {
			key, cs = a.renderer.stoneItem(itemSpriteSpecies(it.Kind, it.Animal, it.X, it.Y), it.Kind, noCS)
		}
		if img := a.renderer.art.image(key, color.NRGBA{}); img != nil {
			drawIconCS(screen, img, float64(r.x)+12, float64(r.y)+10, 40, cs)
		}
	}
	x, y := int(r.x), int(r.y)
	drawLabel(screen, x+60, y+12, itemTitle(it))
	drawLabel(screen, x+60, y+30, itemCategory(it))
	for i, l := range itemLines(it, stored) {
		drawLabel(screen, x+12, y+62+i*14, l)
	}
	rows := a.itemRows(it, stored)
	for i, row := range rows {
		drawLabel(screen, x+12, y+int(r.h)-26-(len(rows)-1-i)*26, row.Verb)
	}
	mx, my := a.currentCursor()
	a.drawButtons(screen, append(itemPanelButtons(r, rows), itemForbidButton(r, it)), mx, my)
	if a.renderer.art != nil {
		a.renderer.drawSprite(screen, "overlay/selection", color.NRGBA{}, float64(it.X), float64(it.Y), a.renderer.tilePx(), a.sw, a.sh)
	}
}

// applyItemClick handles an "item.*" button.
func (a *App) applyItemClick(id string) {
	it, ok := a.selectedItem()
	if !ok {
		return
	}
	if id == "item.close" {
		a.clearItem()
		return
	}
	if id == "item.forbid" {
		a.renderer.enqueue(command.Command{Kind: command.SetForbidden, Target: uint64(it.ID), Enabled: !it.Forbidden})
		return
	}
	n, err := strconv.Atoi(strings.TrimPrefix(id, "item.act."))
	if err != nil || !strings.HasPrefix(id, "item.act.") {
		return
	}
	acts := flattenChips(a.itemRows(it, inStockpile(a.renderer.world(), it.X, it.Y)))
	if n >= 0 && n < len(acts) && !acts[n].Disabled {
		a.renderer.enqueue(acts[n].Cmd)
	}
}
