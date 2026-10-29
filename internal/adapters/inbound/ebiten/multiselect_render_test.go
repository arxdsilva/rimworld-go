package ebiten

import (
	"image/color"
	"rimworld-go/internal/adapters/inbound/ebiten/architect"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	spritepkg "rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	worldpkg "rimworld-go/internal/adapters/inbound/ebiten/render/world"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

// drawItemIcon draws the stack's icon, size px square at (x, y): its
// ui_icon, else its stone sprite. Nothing without art.
func (a *App) drawItemIcon(dst *ebiten.Image, it game.ItemView, x, y, size float64) {
	if a.renderer == nil || a.renderer.art == nil {
		return
	}
	key := "ui_icon/" + strings.ReplaceAll(it.Kind, " ", "_") // "cloth carpet" -> cloth_carpet
	cs := spritepkg.NoCS
	if !a.renderer.art.Has(key) {
		key, cs = a.renderer.pipe().StoneItem(worldpkg.ItemSpriteSpecies(it.Kind, it.Animal, it.X, it.Y), it.Kind, spritepkg.NoCS)
	}
	if img := a.renderer.art.Image(key, color.NRGBA{}); img != nil {
		architect.DrawIconCS(dst, img, x, y, size, cs)
	}
}

// drawPlantIcon draws the plant's icon, size px square at (x, y). Nothing
// without art.
func (a *App) drawPlantIcon(dst *ebiten.Image, p game.PlantView, x, y, size float64) {
	if a.renderer == nil || a.renderer.art == nil {
		return
	}
	if img := a.renderer.art.Image(a.plantIconKey(p), color.NRGBA{}); img != nil {
		gfx.Icon(dst, img, x, y, size)
	}
}
