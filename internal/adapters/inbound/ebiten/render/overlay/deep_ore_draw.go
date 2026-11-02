package overlay

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/application/game"
)

// The deep ore overlay (AX-4, MAIN_UI_SCREENS map overlays): every deep
// ore cell a ground-penetrating scanner found (WorldView.AX.DeepOre) wears
// the overlay/deep_ore marker multiplied by its ore's colour. It shows
// while a deep drill or a scanner is selected or its tool is in hand
// (Set.DeepOre.Forced), or with the player's toggle. Presentation only;
// no allocation per frame.

// DeepOreKey is the marker sprite (AR2's art, metadata ore_colour).
const DeepOreKey = "overlay/deep_ore"

// OreColour is the marker colour of a deep ore (an item kind name); a
// muted grey for one without its own.
func OreColour(ore string) sprite.ColorScale {
	switch ore {
	case "steel":
		return sprite.ColorScale{0.62, 0.68, 0.78, 0.8}
	case "silver":
		return sprite.ColorScale{0.9, 0.92, 0.96, 0.8}
	case "plasteel":
		return sprite.ColorScale{0.45, 0.8, 0.85, 0.8}
	case "uranium":
		return sprite.ColorScale{0.5, 0.85, 0.35, 0.8}
	case "jade":
		return sprite.ColorScale{0.3, 0.7, 0.5, 0.8}
	case "gold":
		return sprite.ColorScale{1, 0.82, 0.3, 0.8}
	}
	return sprite.ColorScale{0.6, 0.6, 0.6, 0.7}
}

// DrawDeepOre draws the overlay when it is shown.
func (o *Set) DrawDeepOre(v *render.View, screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	if !o.DeepOre.Shown() || v.Art == nil || len(w.AX.DeepOre) == 0 {
		return
	}
	x0, y0, x1, y1 := visibleTiles(v, sw, sh, tile)
	for _, d := range w.AX.DeepOre {
		if d.X < x0 || d.X > x1 || d.Y < y0 || d.Y > y1 {
			continue
		}
		o.Wiring.drawUnlitSprite(v, screen, DeepOreKey, float64(d.X), float64(d.Y), tile, sw, sh, OreColour(d.Ore))
	}
	v.LayerDrawn("deep ore")
}

// DeepOreKind reports a building kind that shows the overlay while
// selected: the deep drill and the ground-penetrating scanner.
func DeepOreKind(kind string) bool {
	return kind == "deep drill" || kind == "ground-penetrating scanner"
}

// DeepOreTool reports an Architect tool that shows the overlay while in
// hand (architect catalog ids).
func DeepOreTool(id string) bool { return id == "deep_drill" || id == "deep_scanner" }
