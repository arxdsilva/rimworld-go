package creature

import (
	"fmt"
	"image/color"
	"math"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/adapters/inbound/ebiten/render/motion"
	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/application/game"
)

// Burial presentation (SIM-017, ART §28): human corpses drawn from the
// deceased's own look, graves with headstones, the funeral pyre.

// CorpseCarrySprite is the bundle shown in a hauler's arms while a body
// is carried (the wrapped body; no dedicated art yet).
const CorpseCarrySprite = "item/leather_hides"

// PyreFlameScale enlarges the campfire flame for a pyre.
const PyreFlameScale = 1.9

// CorpsePlan is how a human corpse lies: its standing south figure laid
// on its side (lyingPose).
type CorpsePlan struct {
	Ps motion.Pose
}

// CorpsePlanFor lays the deceased flat on a side that is stable per body,
// tinted dead (or sickly green once rotten).
func CorpsePlanFor(c *art.Catalog, it game.ItemView) CorpsePlan {
	side := 1.0
	if sprite.TileHash(int(it.ID), it.X+it.Y)&1 == 1 {
		side = -1
	}
	cs := DeadCS
	if it.Rotten {
		cs = sprite.RottenCS
	}
	ps := LyingPose(c, side)
	ps.Cs = cs
	return CorpsePlan{Ps: ps}
}

// DeceasedOf finds the pawn a corpse belongs to (still in the snapshot).
func (c *Set) DeceasedOf(v *render.View, it game.ItemView) (game.PawnView, bool) {
	for _, p := range v.Pawns {
		if p.ID == it.Of && it.Of != 0 {
			return p, true
		}
	}
	return game.PawnView{}, false
}

// CorpseFlies are the positions (tiles) of the flies circling a rotting
// human corpse; deterministic in the item ID and the frame.
func CorpseFlies(it game.ItemView, present uint64) [][2]float64 {
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

// BonesSprite is the bones variant for a tile.
func BonesSprite(x, y int) string { return artKey("item", "bones", int(sprite.TileHash(x, y)%2)) }

// CorpseSprite is the per-tile sprite of a carcass or bones (animal
// corpses v0..v2, bones 00..01); false for any other kind, which draws
// its item sprite. The read model names a carcass by species ("cobra
// corpse", Animal "cobra"): it draws item/corpse_<species>_v<0..2>; an
// unnamed "animal corpse" draws the deer's. The former dustbuck and
// ridgewolf corpse kinds keep their own art.
func CorpseSprite(kind, species string, x, y int) (string, bool) {
	if species != "" && (strings.HasSuffix(kind, " corpse") || strings.HasSuffix(kind, " wreck")) && kind != "human corpse" { // a mechanoid wreck draws like a corpse (DEF-4)
		return fmt.Sprintf("item/corpse_%s_v%d", species, sprite.TileHash(x, y)%3), true
	}
	switch kind {
	case "bones":
		return BonesSprite(x, y), true
	case "animal corpse":
		if species == "" {
			species = "deer"
		}
		return fmt.Sprintf("item/corpse_%s_v%d", species, sprite.TileHash(x, y)%3), true
	case "dustbuck corpse", "ridgewolf corpse":
		sp := kind[:len(kind)-len(" corpse")]
		return fmt.Sprintf("item/corpse_%s_v%d", sp, sprite.TileHash(x, y)%3), true
	}
	return "", false
}

// artKey is a variant's sprite key: family/name_NN.
func artKey(family, name string, variant int) string {
	return family + "/" + name + "_" + sprite.TwoDigits(variant)
}

// DrawCorpseItem draws a human corpse (or bones) lying on its tile.
func (c *Set) DrawCorpseItem(v *render.View, pass sprite.Pass, screen *ebiten.Image, it game.ItemView, tile float64, sw, sh int) {
	wx, wy := float64(it.X), float64(it.Y)
	if it.Kind == "bones" {
		c.Paint.DrawScaled(v, pass, screen, BonesSprite(it.X, it.Y), color.NRGBA{}, wx, wy, tile, sw, sh, sprite.NoCS, 1)
		return
	}
	p, ok := c.DeceasedOf(v, it)
	if !ok {
		c.Paint.DrawScaled(v, pass, screen, c.Host.ItemSprite(it.Kind), color.NRGBA{}, wx, wy, tile, sw, sh, sprite.ItemCS(it), 1)
		return
	}
	c.DrawFigure(v, pass, screen, p, wx, wy, tile, CorpsePlanFor(v.Art, it).Ps)
	for _, f := range CorpseFlies(it, v.Present) {
		fx, fy := v.WorldToScreenF(f[0], f[1])
		vector.FillCircle(screen, fx, fy, float32(math.Max(1.2, tile*0.025)), color.RGBA{24, 22, 20, 255}, true)
	}
}

// GraveSprite is the dug or filled ground sprite, south or east.
func GraveSprite(b game.BuildingView) string {
	state, face := "dug", "south"
	if b.Occupant != "" {
		state = "filled"
	}
	if b.Facing == "east" || b.Facing == "west" {
		face = "east"
	}
	return "furniture/grave_" + state + "_" + face
}

// HeadstoneSprite is the headstone for a filled grave ("" when empty).
func HeadstoneSprite(b game.BuildingView) string {
	if b.Occupant == "" {
		return ""
	}
	return artKey("furniture", "headstone", max(0, min(b.Headstone, 3)))
}

// HeadstoneTile is the grave's head end: top for south, left for east,
// right for west, bottom for north.
func HeadstoneTile(b game.BuildingView) (float64, float64) {
	w, h := max(b.W, 1), max(b.H, 1)
	switch b.Facing {
	case "west":
		return float64(b.X + w - 1), float64(b.Y)
	case "north":
		return float64(b.X), float64(b.Y + h - 1)
	}
	return float64(b.X), float64(b.Y)
}

// DrawGrave draws a dug or filled grave and its headstone.
func (c *Set) DrawGrave(v *render.View, pass sprite.Pass, screen *ebiten.Image, b game.BuildingView, tile float64, sw, sh int) {
	c.Paint.Draw(v, pass, screen, GraveSprite(b), color.NRGBA{}, float64(b.X), float64(b.Y), tile, sw, sh)
	if key := HeadstoneSprite(b); key != "" {
		hx, hy := HeadstoneTile(b)
		c.Paint.Draw(v, pass, screen, key, color.NRGBA{}, hx, hy, tile, sw, sh)
	}
}

// PyreSprite is the cold or burning pyre.
func PyreSprite(b game.BuildingView) string {
	if b.Burning {
		return "furniture/pyre_burning"
	}
	return "furniture/pyre"
}

// fireHeight lifts a flame (tiles) to sit on the logs: the campfire's
// (fire.go), which the pyre shares.
const fireHeight = 0.22

// pyreCentre is where the pyre's flames rise, in tiles.
func pyreCentre(b game.BuildingView) (float64, float64) {
	return float64(b.X) + float64(max(b.W, 1))/2, float64(b.Y) + 0.5 - fireHeight
}

// EmitPyre sends embers and smoke up from a burning pyre.
func (c *Set) EmitPyre(v *render.View, b game.BuildingView) {
	if !b.Burning {
		return
	}
	x, y := pyreCentre(b)
	seed := uint64(b.ID)*104729 + v.Present
	if (v.Present+uint64(b.ID))%4 == 0 {
		c.Burst(v, PartEmber, x, y-0.2, 1, seed)
	}
	if (v.Present+uint64(b.ID))%7 == 0 {
		c.Burst(v, PartSmoke, x, y-0.45, 1, seed^0x5bd1e995)
	}
}

// DrawFlame is the campfire flame and ground glow (fire.go) scaled by k.
func (c *Set) DrawFlame(v *render.View, screen *ebiten.Image, cx, cy, t, tile, k float64) {
	sx, sy := v.WorldToScreenF(cx, cy)
	DrawFlameScreen(screen, sx, sy, t, tile, k)
}

// DrawFlameScreen draws the flame at screen point (sx, sy), tile pixels
// per tile; shared by the colony and the main-menu diorama.
func DrawFlameScreen(screen *ebiten.Image, sx, sy float32, t, tile, k float64) {
	f := 0.75 + 0.25*math.Sin(t*0.31)*math.Sin(t*0.13+2)
	vector.FillCircle(screen, sx, sy+float32(tile*0.12), float32(tile*0.75*k), Fade(color.RGBA{255, 150, 60, 255}, 0.10*f), true)
	vector.FillCircle(screen, sx, sy+float32(tile*0.12), float32(tile*0.42*k), Fade(color.RGBA{255, 170, 80, 255}, 0.16*f), true)
	for i, c := range []color.RGBA{{205, 60, 30, 255}, {245, 130, 40, 255}, {255, 205, 90, 255}} {
		base := float32(tile * (0.17 - 0.04*float64(i)) * f * k)
		for j := 0; j < 4; j++ {
			h := float64(j) / 4
			sway := math.Sin(t*0.22+h*3+float64(i)) * 0.03 * h * k
			px, py := sx+float32(sway*tile), sy-float32(h*0.32*f*k*tile)
			vector.FillCircle(screen, px, py, base*float32(1-h*0.7), Fade(c, 0.9), true)
		}
	}
}

// DrawPyre draws the funeral pyre, burning while it cremates.
func (c *Set) DrawPyre(v *render.View, pass sprite.Pass, screen *ebiten.Image, b game.BuildingView, tile float64, sw, sh int) {
	c.Paint.Draw(v, pass, screen, PyreSprite(b), color.NRGBA{}, float64(b.X), float64(b.Y), tile, sw, sh)
	if !b.Burning {
		return
	}
	cx, cy := pyreCentre(b)
	c.DrawFlame(v, screen, cx, cy, float64(v.Present)+float64(b.ID)*17, tile, PyreFlameScale)
}
