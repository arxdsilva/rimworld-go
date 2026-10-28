package ebiten

import (
	"fmt"
	"image/color"
	"math"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/application/game"
)

// Burial presentation (SIM-017, ART §28): human corpses drawn from the
// deceased's own look, graves with headstones, the funeral pyre.

// corpseCarrySprite is the bundle shown in a hauler's arms while a body
// is carried (the wrapped body; no dedicated art yet).
const corpseCarrySprite = "item/leather_hides"

// pyreFlameScale enlarges the campfire flame for a pyre.
const pyreFlameScale = 1.9

// corpsePlan is how a human corpse lies: its standing south figure laid
// on its side (lyingPose).
type corpsePlan struct {
	Ps pose
}

// corpsePlanFor lays the deceased flat on a side that is stable per body,
// tinted dead (or sickly green once rotten).
func corpsePlanFor(c *artCatalog, it game.ItemView) corpsePlan {
	side := 1.0
	if tileHash(int(it.ID), it.X+it.Y)&1 == 1 {
		side = -1
	}
	cs := deadCS
	if it.Rotten {
		cs = rottenCS
	}
	ps := lyingPose(c, side)
	ps.Cs = cs
	return corpsePlan{Ps: ps}
}

// deceasedOf finds the pawn a corpse belongs to (still in the snapshot).
func (r *Renderer) deceasedOf(it game.ItemView) (game.PawnView, bool) {
	for _, p := range r.view.Pawns {
		if p.ID == it.Of && it.Of != 0 {
			return p, true
		}
	}
	return game.PawnView{}, false
}

// corpseFlies are the positions (tiles) of the flies circling a rotting
// human corpse; deterministic in the item ID and the frame.
func corpseFlies(it game.ItemView, present uint64) [][2]float64 {
	if it.Kind != "human corpse" || !it.Rotten {
		return nil
	}
	n := 2 + int(it.ID%3)
	out := make([][2]float64, n)
	t := float64(present)
	for i := range out {
		a := t*(0.11+0.02*float64(i)) + float64(i)*2*math.Pi/float64(n) + float64(it.ID)*1.7
		rad := 0.3 + 0.1*math.Sin(t*0.07+float64(i)+float64(it.ID))
		out[i] = [2]float64{float64(it.X) + 0.5 + math.Cos(a)*rad, float64(it.Y) + 0.45 + math.Sin(a)*rad*0.6}
	}
	return out
}

// bonesSprite is the bones variant for a tile.
func bonesSprite(x, y int) string { return artKey("item", "bones", int(tileHash(x, y)%2)) }

// itemSpriteAt is itemSprite with the per-tile variants of corpses and
// bones (animal corpses v0..v2, bones 00..01). It has no species for an
// "animal corpse"; use itemSpriteSpecies with ItemView.Animal for those.
func itemSpriteAt(kind string, x, y int) string {
	return itemSpriteSpecies(kind, "", x, y)
}

// itemSpriteSpecies is itemSpriteAt for a stack that names its species. The
// read model names a carcass by species ("cobra corpse", Animal "cobra"):
// it draws item/corpse_<species>_v<0..2>; an unnamed "animal corpse" draws
// the deer's. The former dustbuck and ridgewolf corpse kinds keep their own
// art.
func itemSpriteSpecies(kind, species string, x, y int) string {
	if species != "" && strings.HasSuffix(kind, " corpse") && kind != "human corpse" {
		return fmt.Sprintf("item/corpse_%s_v%d", species, tileHash(x, y)%3)
	}
	switch kind {
	case "bones":
		return bonesSprite(x, y)
	case "animal corpse":
		if species == "" {
			species = "deer"
		}
		return fmt.Sprintf("item/corpse_%s_v%d", species, tileHash(x, y)%3)
	case "dustbuck corpse", "ridgewolf corpse":
		sp := kind[:len(kind)-len(" corpse")]
		return fmt.Sprintf("item/corpse_%s_v%d", sp, tileHash(x, y)%3)
	}
	return itemSprite(kind)
}

// drawCorpseItem draws a human corpse (or bones) lying on its tile.
func (r *Renderer) drawCorpseItem(screen *ebiten.Image, it game.ItemView, tile float64, sw, sh int) {
	wx, wy := float64(it.X), float64(it.Y)
	if it.Kind == "bones" {
		r.drawSpriteScaled(screen, bonesSprite(it.X, it.Y), color.NRGBA{}, wx, wy, tile, sw, sh, noCS, 1)
		return
	}
	p, ok := r.deceasedOf(it)
	if !ok {
		r.drawSpriteScaled(screen, itemSprite(it.Kind), color.NRGBA{}, wx, wy, tile, sw, sh, itemCS(it), 1)
		return
	}
	r.drawFigure(screen, p, wx, wy, tile, corpsePlanFor(r.art, it).Ps)
	for _, f := range corpseFlies(it, r.present) {
		fx, fy := r.worldToScreenF(f[0], f[1])
		vector.FillCircle(screen, fx, fy, float32(math.Max(1.2, tile*0.025)), color.RGBA{24, 22, 20, 255}, true)
	}
}

// graveSprite is the dug or filled ground sprite, south or east.
func graveSprite(b game.BuildingView) string {
	state, face := "dug", "south"
	if b.Occupant != "" {
		state = "filled"
	}
	if b.Facing == "east" || b.Facing == "west" {
		face = "east"
	}
	return "furniture/grave_" + state + "_" + face
}

// headstoneSprite is the headstone for a filled grave ("" when empty).
func headstoneSprite(b game.BuildingView) string {
	if b.Occupant == "" {
		return ""
	}
	return artKey("furniture", "headstone", max(0, min(b.Headstone, 3)))
}

// headstoneTile is the grave's head end: top for south, left for east,
// right for west, bottom for north.
func headstoneTile(b game.BuildingView) (float64, float64) {
	w, h := max(b.W, 1), max(b.H, 1)
	switch b.Facing {
	case "west":
		return float64(b.X + w - 1), float64(b.Y)
	case "north":
		return float64(b.X), float64(b.Y + h - 1)
	}
	return float64(b.X), float64(b.Y)
}

// drawGrave draws a dug or filled grave and its headstone.
func (r *Renderer) drawGrave(screen *ebiten.Image, b game.BuildingView, tile float64, sw, sh int) {
	r.drawSprite(screen, graveSprite(b), color.NRGBA{}, float64(b.X), float64(b.Y), tile, sw, sh)
	if key := headstoneSprite(b); key != "" {
		hx, hy := headstoneTile(b)
		r.drawSprite(screen, key, color.NRGBA{}, hx, hy, tile, sw, sh)
	}
}

// pyreSprite is the cold or burning pyre.
func pyreSprite(b game.BuildingView) string {
	if b.Burning {
		return "furniture/pyre_burning"
	}
	return "furniture/pyre"
}

// pyreCentre is where the pyre's flames rise, in tiles.
func pyreCentre(b game.BuildingView) (float64, float64) {
	return float64(b.X) + float64(max(b.W, 1))/2, float64(b.Y) + 0.5 - fireHeight
}

// emitPyre sends embers and smoke up from a burning pyre.
func (r *Renderer) emitPyre(b game.BuildingView) {
	if !b.Burning {
		return
	}
	x, y := pyreCentre(b)
	seed := uint64(b.ID)*104729 + r.present
	if (r.present+uint64(b.ID))%4 == 0 {
		r.burst(partEmber, x, y-0.2, 1, seed)
	}
	if (r.present+uint64(b.ID))%7 == 0 {
		r.burst(partSmoke, x, y-0.45, 1, seed^0x5bd1e995)
	}
}

// drawFlame is the campfire flame and ground glow (fire.go) scaled by k.
func (r *Renderer) drawFlame(screen *ebiten.Image, cx, cy, t, tile, k float64) {
	sx, sy := r.worldToScreenF(cx, cy)
	drawFlameScreen(screen, sx, sy, t, tile, k)
}

// drawFlameScreen draws the flame at screen point (sx, sy), tile pixels
// per tile; shared by the colony and the main-menu diorama.
func drawFlameScreen(screen *ebiten.Image, sx, sy float32, t, tile, k float64) {
	f := 0.75 + 0.25*math.Sin(t*0.31)*math.Sin(t*0.13+2)
	vector.FillCircle(screen, sx, sy+float32(tile*0.12), float32(tile*0.75*k), fade(color.RGBA{255, 150, 60, 255}, 0.10*f), true)
	vector.FillCircle(screen, sx, sy+float32(tile*0.12), float32(tile*0.42*k), fade(color.RGBA{255, 170, 80, 255}, 0.16*f), true)
	for i, c := range []color.RGBA{{205, 60, 30, 255}, {245, 130, 40, 255}, {255, 205, 90, 255}} {
		base := float32(tile * (0.17 - 0.04*float64(i)) * f * k)
		for j := 0; j < 4; j++ {
			h := float64(j) / 4
			sway := math.Sin(t*0.22+h*3+float64(i)) * 0.03 * h * k
			px, py := sx+float32(sway*tile), sy-float32(h*0.32*f*k*tile)
			vector.FillCircle(screen, px, py, base*float32(1-h*0.7), fade(c, 0.9), true)
		}
	}
}

// drawPyre draws the funeral pyre, burning while it cremates.
func (r *Renderer) drawPyre(screen *ebiten.Image, b game.BuildingView, tile float64, sw, sh int) {
	r.drawSprite(screen, pyreSprite(b), color.NRGBA{}, float64(b.X), float64(b.Y), tile, sw, sh)
	if !b.Burning {
		return
	}
	cx, cy := pyreCentre(b)
	r.drawFlame(screen, cx, cy, float64(r.present)+float64(b.ID)*17, tile, pyreFlameScale)
}
