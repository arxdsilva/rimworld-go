package ebiten

import (
	"fmt"
	"image/color"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

// Item inspection (UI-011, MAIN_UI §10): what the stack is and its
// condition (itemPane), and Forbid, the haul mark and per-colonist Equip /
// Wear / Eat / Haul orders (itemCommands, cmds_thing.go).

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
	if it.PoisonPermille > 0 {
		out = append(out, "Poison chance "+poisonPct(it.PoisonPermille))
	}
	if isCarpetItem(it) {
		out = append(out, fmt.Sprintf("Catches %d%% of tracked dirt", it.Catch),
			"Install from Architect > Furniture > Carpet")
	}
	if it.Forbidden {
		out = append(out, "Forbidden: colonists leave it alone")
	}
	switch {
	case it.Unclaimed && it.HaulMarked:
		out = append(out, "Found here. Marked: colonists will haul it.")
	case it.Unclaimed:
		out = append(out, "Found here. Colonists won't haul it until you mark it.")
	}
	if stored {
		return append(out, "In a stockpile")
	}
	return append(out, "Not stored")
}

// poisonPct is a food-poisoning chance in permille as a percent ("2%",
// "0.5%").
func poisonPct(permille int) string {
	if permille%10 == 0 {
		return fmt.Sprintf("%d%%", permille/10)
	}
	return fmt.Sprintf("%.1f%%", float64(permille)/10)
}

// itemVerb is one colonist action on a stack: a command whose options are
// the colonists able to do it.
type itemVerb struct {
	id, label, icon, desc string
	match                 func(game.OrderOption) bool
}

// itemVerbs are the actions for it: the verb of its category (Equip, Wear
// or Eat), then Haul when it is not in a stockpile.
func itemVerbs(it game.ItemView, stored bool) []itemVerb {
	var out []itemVerb
	switch itemCategory(it) {
	case "weapon":
		out = append(out, itemVerb{"cmd.item.equip", "Equip", "equip",
			"Have a colonist pick it up and wield it. A click sends the nearest; right-click picks who.", orderIs(command.OrderEquip)})
	case "apparel":
		out = append(out, itemVerb{"cmd.item.wear", "Wear", "wear",
			"Have a colonist put it on. A click sends the nearest; right-click picks who.", orderIs(command.OrderEquip)})
	case "food":
		out = append(out, itemVerb{"cmd.item.eat", "Eat", "eat",
			"Have a colonist eat it now. A click sends the nearest; right-click picks who.", orderIs(command.OrderEat)})
	}
	if !stored {
		out = append(out, itemVerb{"cmd.item.haul", "Haul", "order_haul",
			"Have a colonist carry it to a stockpile. A click sends the nearest; right-click picks who.", orderIs(command.OrderHaul)})
	}
	return out
}

// drawItemIcon draws the stack's icon, size px square at (x, y): its
// ui_icon, else its stone sprite. Nothing without art.
func (a *App) drawItemIcon(dst *ebiten.Image, it game.ItemView, x, y, size float64) {
	if a.renderer == nil || a.renderer.art == nil {
		return
	}
	key := "ui_icon/" + strings.ReplaceAll(it.Kind, " ", "_") // "cloth carpet" -> cloth_carpet
	cs := noCS
	if !a.renderer.art.Has(key) {
		key, cs = a.renderer.stoneItem(itemSpriteSpecies(it.Kind, it.Animal, it.X, it.Y), it.Kind, noCS)
	}
	if img := a.renderer.art.Image(key, color.NRGBA{}); img != nil {
		drawIconCS(dst, img, x, y, size, cs)
	}
}

// drawItemMark draws the selection mark on the stack's tile; the
// panel itself is the inspect pane (itemPane).
func (a *App) drawItemMark(screen *ebiten.Image) {
	it, ok := a.selectedItem()
	if !ok || a.workOpen || a.archOpen {
		return
	}
	if a.renderer.art != nil {
		a.renderer.drawSprite(screen, "overlay/selection", color.NRGBA{}, float64(it.X), float64(it.Y), a.shell.TilePx(), a.pw, a.ph) // physical, like the map
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
		a.shell.Enqueue(command.Command{Kind: command.SetForbidden, Target: uint64(it.ID), Enabled: !it.Forbidden})
		return
	}
	if id == "item.haulmark" {
		if it.Unclaimed { // colony stacks have no mark
			a.shell.Enqueue(command.Command{Kind: command.SetHaulMark, Target: uint64(it.ID), Enabled: !it.HaulMarked})
		}
		return
	}
}
