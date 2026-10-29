package world

import (
	"image/color"
	"math"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/adapters/inbound/ebiten/render/creature"
	"rimworld-go/internal/adapters/inbound/ebiten/render/env"
	"rimworld-go/internal/adapters/inbound/ebiten/render/motion"
	"rimworld-go/internal/adapters/inbound/ebiten/render/overlay"
	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/application/game"
)

// The parts' shared primitives: the sprite pass, the camera transforms,
// the painter and creature draws, the frame's culls as the layers read
// them, the ground tile and the art keys that need the catalog.

// art is the catalog the painter draws from.
func (b *base) art() *art.Catalog { return b.Paint.Art }

// Pass is the Pass sprites draw in: the shadow Pass's cast, a bake, a
// figure's blood.
func (b *base) Pass() sprite.Pass {
	return sprite.Pass{Cast: *b.Cast, Baking: b.LOD.Baking, Blood: *b.Blood}
}

// lightView is the view a light draw reads: the camera the renderer draws
// with, unless baking (a bake's draws are unlit).
func (b *base) lightView() *render.View {
	v := b.v
	v.Art = b.Paint.Art
	if !b.LOD.Baking {
		v.Cam = *b.Cam
	}
	return v
}

// worldToScreenF is a world point in map pixels on the renderer's camera
// (the bake's while baking).
func (b *base) worldToScreenF(wx, wy float64) (float32, float32) {
	tile := b.Cam.Tile()
	return float32(wx*tile - b.Cam.OffsetX), float32(wy*tile - b.Cam.OffsetY)
}

// worldToScreen is a tile's top-left in map pixels.
func (b *base) worldToScreen(tx, ty int) (float32, float32) {
	return b.worldToScreenF(float64(tx), float64(ty))
}

// visibleTiles is the on-map tile range in view (render/env).
func (b *base) visibleTiles() (x0, y0, x1, y1 int) { return b.Env.VisibleTiles(b.v) }

// setView sets the environment's visible rectangle for a destination of
// sw × sh at tile.
func (b *base) setView(sw, sh int, tile float64) { b.Env.SetView(b.v, sw, sh, tile) }

// layer marks the start of a ground layer for the draw-order test.
func (b *base) layer(name string) { b.v.LayerDrawn(name) }

func (b *base) drawSprite(dst *ebiten.Image, key string, tint color.NRGBA, wx, wy, tile float64, sw, sh int) {
	b.Paint.Draw(b.v, b.Pass(), dst, key, tint, wx, wy, tile, sw, sh)
}

func (b *base) drawSpriteCS(dst *ebiten.Image, key string, tint color.NRGBA, wx, wy, tile float64, sw, sh int, cs sprite.ColorScale) {
	b.Paint.DrawCS(b.v, b.Pass(), dst, key, tint, wx, wy, tile, sw, sh, cs)
}

func (b *base) drawSpriteScaled(dst *ebiten.Image, key string, tint color.NRGBA, wx, wy, tile float64, sw, sh int, cs sprite.ColorScale, k float64) {
	b.Paint.DrawScaled(b.v, b.Pass(), dst, key, tint, wx, wy, tile, sw, sh, cs, k)
}

func (b *base) DrawSpritePose(dst *ebiten.Image, key string, tint color.NRGBA, wx, wy, tile float64, ps motion.Pose) {
	b.Creatures.DrawSpritePose(b.v, b.Pass(), dst, key, tint, wx, wy, tile, ps)
}

func (b *base) drawLinkedSprite(dst *ebiten.Image, d sprite.LinkedDraw, tile float64, sw, sh int) {
	b.Paint.DrawLinkedSprite(b.v, b.Pass(), dst, d, tile, sw, sh)
}

func (b *base) stuffTint(stuff string) color.NRGBA { return b.Paint.StuffTint(stuff) }

func (b *base) stoneTintAt(w *game.WorldView, x, y int) color.NRGBA {
	return b.Paint.StoneTintAt(w, x, y)
}

// interpWorld is where a pawn is drawn: its motion at the frame's animT
// (ART §27.1), or its tile before the first sync.
func (b *base) interpWorld(p game.PawnView) (float64, float64) {
	m, ok := b.Motion.MotionOf(p.ID, b.v.AnimT)
	if !ok {
		return float64(p.X), float64(p.Y)
	}
	return m.X, m.Y
}

// OverlayChunks is the frame's chunk lists and view as the overlays walk
// them.
func (b *base) OverlayChunks() overlay.Chunks {
	l := b.LOD
	return overlay.Chunks{Views: l.Views, VX0: l.VX0, VY0: l.VY0, VX1: l.VX1, VY1: l.VY1, ViewSet: l.ViewSet,
		Live: l.Tier == Live, WholeMap: l.Tier == Map, Baking: l.Baking}
}

// CreatureCull is the frame's culling for the creature layer.
func (b *base) CreatureCull() creature.Cull {
	l := b.LOD
	return creature.Cull{X0: l.VX0, Y0: l.VY0, X1: l.VX1, Y1: l.VY1, ViewSet: l.ViewSet,
		MapTier: l.Tier == Map, MotionPad: MotionViewPad, MinTile: CreatureMinTile}
}

// MotionViewPad: animals within this many tiles of the last frame's view
// keep a motion track (the renderer's syncAnimals).
const MotionViewPad = env.ContactViewPad + 2

// DrawTile draws an opaque, tileable 1x1 sprite snapped to whole pixels
// with a 1 px overlap, so fractional zoom never shows seams between tiles.
// It is ground (terrain, floors, water): lit as the ground (Lit.DrawGround).
func (b *base) DrawTile(dst *ebiten.Image, key string, wx, wy, tile float64, sw, sh int, cs sprite.ColorScale) {
	sx, sy := b.worldToScreenF(wx, wy)
	x0, y0 := math.Floor(float64(sx)), math.Floor(float64(sy))
	size := math.Ceil(float64(sx)+tile) - x0 + 1
	if !sprite.Visible(float32(x0), float32(y0), float32(size), float32(size), sw, sh) {
		return
	}
	img := b.art().Image(key, color.NRGBA{})
	if img == nil {
		return
	}
	k := size / float64(img.Bounds().Dx())
	geo := ebiten.GeoM{}
	geo.Scale(k, k)
	geo.Translate(x0, y0)
	b.Lit.DrawGround(b.lightView(), dst, img, geo, cs, wx, wy, *b.Cast, b.LOD.Baking, *b.Blood)
}

// drawGroundTile draws tile (x, y)'s ground: its terrain tile (water
// through render/env), the soft edges of higher-priority neighbours and
// water edges. Stone ground (rough, smooth, under rock) takes its stone's
// tint.
func (b *base) drawGroundTile(dst *ebiten.Image, w *game.WorldView, x, y int, tile float64, sw, sh int) {
	none := color.NRGBA{}
	code, ok := env.TerrainCode(*w, x, y)
	if !ok || int(code) >= len(TerrainNames) {
		code, ok = 0, false
	}
	key, tcs := b.tileKeyOf(code, int(sprite.TileHash(x, y)%SoilVariants))
	if !ok {
		key, tcs = b.TerrainTileKey("soil", int(sprite.TileHash(x, y)%SoilVariants))
	} else if stoneGroundCode[code] {
		tcs = sprite.MulCS(tcs, sprite.TintCS(b.stoneTintAt(w, x, y)))
	}
	if ok && IsWaterCode[code] {
		b.Lit.Calls++
		b.Env.DrawWaterTile(b.v, dst, TerrainNames[code], x, y, tile, sw, sh, tcs, *b.Cast != nil, b.Stamp)
	} else {
		b.DrawTile(dst, key, float64(x), float64(y), tile, sw, sh, tcs)
	}
	pri := int(TerrainPriByCode[code])
	var nbPri [4]int // N, E, S, W; -100 off the map
	higher := false
	for i, n := range [...]struct{ dx, dy int }{{0, -1}, {1, 0}, {0, 1}, {-1, 0}} {
		nbPri[i] = -100
		if nc, ok := env.TerrainCode(*w, x+n.dx, y+n.dy); ok {
			nbPri[i] = int(TerrainPriByCode[nc])
			higher = higher || nbPri[i] > pri
		}
	}
	for p := max(pri+1, 0); higher && p < len(TerrainByPri); p++ {
		mask, sx, sy := 0, -1, -1
		for i, n := range [...]struct{ dx, dy, bit int }{{0, -1, art.WallN}, {1, 0, art.WallE}, {0, 1, art.WallS}, {-1, 0, art.WallW}} {
			if nbPri[i] == p {
				mask |= n.bit
				sx, sy = x+n.dx, y+n.dy
			}
		}
		if mask == 0 || p == 0 { // soil has no edge art: it is the plain ground
			continue
		}
		ek, ecs := TerrainEdgeKeys[p][mask], sprite.NoCS
		if !b.art().Has(ek) && TerrainByPri[p] == "stone_smooth" { // older art: the rough edge
			ek = TerrainEdgeKeys[TerrainPriority["stone"]][mask]
		}
		if n := TerrainByPri[p]; n == "stone" || n == "stone_smooth" { // stone edges take the neighbouring stone's tint
			ecs = sprite.TintCS(b.stoneTintAt(w, sx, sy))
		}
		b.drawSpriteCS(dst, ek, none, float64(x), float64(y), tile, sw, sh, ecs)
	}
	if ok && IsWaterCode[code] {
		b.Env.DrawWaterEdges(b.v, b.Pass(), dst, TerrainNames[code], nbPri, TerrainPriority["shallow_water"], x, y, tile, sw, sh, b.Stamp)
	}
}

// rockDraw is the linked sprite of the rock wall at (x, y): its stone's
// tint, its ore vein's sprite.
func (b *base) rockDraw(w *game.WorldView, x, y int) (sprite.LinkedDraw, bool) {
	i := y*w.TerrainW + x
	ore := ""
	if i < len(w.Ore) && int(w.Ore[i]) < len(sprite.OreNames) {
		ore = sprite.OreNames[w.Ore[i]]
	}
	var stone uint8
	if i < len(w.StoneType) {
		stone = w.StoneType[i]
	}
	prefix := sprite.RockPrefix(TerrainNames[w.Terrain[i]], ore)
	return b.Paint.LinkedSprite(prefix, x, y, b.Links.Mask(x, y), b.Links.Corners(x, y), b.Paint.StoneTint(stone), sprite.NoCS)
}

// PlantsOn appends the indexes into plants of the plants on tiles
// x0..x1 × y0..y1: from the simulation's chunk read model when plants is
// its current list (the same shared backing array), else by scanning it (a
// world handed in directly, as tests do).
func (b *base) PlantsOn(plants []game.PlantView, x0, y0, x1, y1 int, buf []int32) []int32 {
	if out, ok := b.src.PlantsIn(plants, x0, y0, x1, y1, buf); ok {
		return out
	}
	for i := range plants {
		if p := &plants[i]; p.X >= x0 && p.X <= x1 && p.Y >= y0 && p.Y <= y1 {
			buf = append(buf, int32(i))
		}
	}
	return buf
}

// ChunkView is this frame's view of chunk (cx, cy) (fetched when outside
// the frame's range).
func (b *base) ChunkView(cx, cy int) game.ChunkView {
	l := b.LOD
	if cx >= l.cx0 && cx <= l.cx1 && cy >= l.cy0 && cy <= l.cy1 {
		if i := (cy-l.cy0)*(l.cx1-l.cx0+1) + cx - l.cx0; i < len(l.Views) {
			return l.Views[i]
		}
	}
	return b.src.ChunkLayer(cx, cy)
}

// AnimalPads are the near/inView pads of a cell-based animal cull
// (PLAY-022 A.3) at a camera tile size (AnimalPadsOf over the catalog).
func (b *base) AnimalPads(tile float64) (pad, padS int) { return AnimalPadsOf(b.art(), tile) }

// drawUnlit is a lit draw while baking: the plain sprite, colour-scaled.
func (b *base) drawUnlit(dst, img *ebiten.Image, geo ebiten.GeoM, cs sprite.ColorScale) {
	op := &ebiten.DrawImageOptions{GeoM: geo, Filter: ebiten.FilterLinear}
	if cs != sprite.NoCS && cs != (sprite.ColorScale{}) {
		op.ColorScale.Scale(cs[0]*cs[3], cs[1]*cs[3], cs[2]*cs[3], cs[3])
	}
	dst.DrawImage(img, op)
}

// ColonyCloths lists the cloth tints colonists and their beds may use; the
// raider tint is held back so hostiles stand out.
func (b *base) ColonyCloths() []string {
	var cloths []string
	for _, c := range b.art().TintNames("cloth") {
		if c != RaiderCloth {
			cloths = append(cloths, c)
		}
	}
	return cloths
}

// RaiderCloth is the tint reserved for raider clothing.
const RaiderCloth = "raider"
