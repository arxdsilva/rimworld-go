package ebiten

import (
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/adapters/inbound/ebiten/render/overlay"
	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/application/game"
)

// The sprite painter, the linked-building grid and keys, the stone tints
// and the fence sprites live in render/sprite (ARCH-004 wave 4.2). These
// aliases and one-line wrappers keep the root's call sites unchanged until
// the world pipeline moves (wave 4.4) and repoints them (SOLID_GO.md §11).

type (
	colorScale  = sprite.ColorScale
	linkedDraw  = sprite.LinkedDraw
	linkFlag    = sprite.LinkFlag
	linkCell    = sprite.LinkCell
	plannedCell = sprite.PlannedCell
	// linkGrid classifies building kinds with the renderer's names
	// (overlay.LinkKinds, render/overlay/power_draw.go).
	linkGrid = sprite.LinkGrid[overlay.LinkKinds]
)

const (
	defaultStone = sprite.DefaultStone
	linkMapEdge  = sprite.LinkMapEdge
	linkRock     = sprite.LinkRock
	linkWall     = sprite.LinkWall
	linkFence    = sprite.LinkFence
	linkConduit  = sprite.LinkConduit
	linkDevice   = sprite.LinkDevice
	cornerNE     = sprite.CornerNE
	cornerSE     = sprite.CornerSE
	cornerSW     = sprite.CornerSW
	cornerNW     = sprite.CornerNW
	fenceKind    = sprite.FenceKind
	gateKind     = sprite.GateKind
)

var (
	noCS         = sprite.NoCS
	blueprintCS  = sprite.BlueprintCS
	ghostOKCS    = sprite.GhostOKCS
	ghostBadCS   = sprite.GhostBadCS
	ghostSameCS  = sprite.GhostSameCS
	driedCS      = sprite.DriedCS
	rottenCS     = sprite.RottenCS
	stoneNames   = sprite.StoneNames
	oreNames     = sprite.OreNames
	linkFallback = sprite.LinkFallback
	wallLink     = sprite.WallLink
	fenceLink    = sprite.FenceLink
	conduitLink  = sprite.ConduitLink
)

func itemCS(it game.ItemView) colorScale           { return sprite.ItemCS(it) }
func lerpCS(a, b colorScale, t float64) colorScale { return sprite.LerpCS(a, b, t) }
func tintCS(t color.NRGBA) colorScale              { return sprite.TintCS(t) }
func mulCS(a, b colorScale) colorScale             { return sprite.MulCS(a, b) }
func doorAxisEW(facing string) bool                { return sprite.DoorAxisEW(facing) }
func isStoneStuff(stuff string) bool               { return sprite.IsStoneStuff(stuff) }
func stuffStone(stuff string) string               { return sprite.StuffStone(stuff) }
func stuffMaterial(stuff string) string            { return sprite.StuffMaterial(stuff) }
func wallPrefix(stuff string) string               { return sprite.WallPrefix(stuff) }
func rockPrefix(terrain, ore string) string        { return sprite.RockPrefix(terrain, ore) }
func isRockWallName(n string) bool                 { return sprite.IsRockWallName(n) }
func isRockWallCode(c uint8) bool                  { return sprite.IsRockWallCode(c) }
func linkedKey(prefix string, mask, corners int) string {
	return sprite.LinkedKey(prefix, mask, corners)
}
func linkedKeys(prefix string) []string         { return sprite.LinkedKeys(prefix) }
func isFenceKind(kind string) bool              { return sprite.IsFenceKind(kind) }
func fenceSprite(stuff string, mask int) string { return sprite.FenceSprite(stuff, mask) }
func tileHash(x, y int) uint64                  { return sprite.TileHash(x, y) }
func grassTint(biomeKey string) colorScale      { return sprite.GrassTint(biomeKey) }
func isTreePlant(species string) bool           { return sprite.IsTreePlant(species) }
func biomeItemSprite(kind string) string        { return sprite.BiomeItemSprite(kind) }

func drawFenceVector(screen *ebiten.Image, sx, sy, tile float32, kind string, mask int, open, stone bool, alpha uint8) {
	sprite.DrawFenceVector(screen, sx, sy, tile, kind, mask, open, stone, alpha)
}

// painter is the renderer's sprite painter over its live catalog, light
// layer (fire light hooked in) and stone cache. The lead's New gives the
// renderer its own (sprite.NewPainter); until then it is built per call,
// on the stack.
func (r *Renderer) painter() sprite.Painter {
	return sprite.Painter{Art: r.art, Lit: r.litLayer(), Stone: &r.stoneArt.StoneCache}
}

// spritePass is the pass the renderer draws sprites in: the shadow
// pass's cast, a bake, a figure's blood.
func (r *Renderer) spritePass() sprite.Pass {
	return sprite.Pass{Cast: r.cast, Baking: r.lod.Baking, Blood: r.figureBlood}
}

// spriteView is the view a sprite draw reads (lightView) with the
// renderer's live camera, which a bake moves; the caller puts frame back
// after the draw, so the view keeps the frame's camera.
func (r *Renderer) spriteView() (v *render.View, frame camera) {
	v = r.lightView()
	frame, v.Cam = v.Cam, r.cam
	return v, frame
}

func (r *Renderer) drawSprite(screen *ebiten.Image, key string, tint color.NRGBA, wx, wy, tile float64, sw, sh int) {
	v, frame := r.spriteView()
	p := r.painter()
	p.Draw(v, r.spritePass(), screen, key, tint, wx, wy, tile, sw, sh)
	v.Cam = frame
}

func (r *Renderer) drawSpriteCS(screen *ebiten.Image, key string, tint color.NRGBA, wx, wy, tile float64, sw, sh int, cs colorScale) {
	v, frame := r.spriteView()
	p := r.painter()
	p.DrawCS(v, r.spritePass(), screen, key, tint, wx, wy, tile, sw, sh, cs)
	v.Cam = frame
}

// drawSpriteScaled draws a sprite scaled by k about its footprint's
// bottom-centre (sprite.Painter.DrawScaled).
func (r *Renderer) drawSpriteScaled(screen *ebiten.Image, key string, tint color.NRGBA, wx, wy, tile float64, sw, sh int, cs colorScale, k float64) {
	v, frame := r.spriteView()
	p := r.painter()
	p.DrawScaled(v, r.spritePass(), screen, key, tint, wx, wy, tile, sw, sh, cs, k)
	v.Cam = frame
}

func (r *Renderer) drawLinkedSprite(screen *ebiten.Image, d linkedDraw, tile float64, sw, sh int) {
	v, frame := r.spriteView()
	p := r.painter()
	p.DrawLinkedSprite(v, r.spritePass(), screen, d, tile, sw, sh)
	v.Cam = frame
}

// drawLinked draws a linked family sprite now; false when there is no art
// (sprite.Painter.DrawLinked).
func (r *Renderer) drawLinked(screen *ebiten.Image, prefix string, x, y, mask, corners int, tint color.NRGBA, cs colorScale, tile float64, sw, sh int) bool {
	v, frame := r.spriteView()
	p := r.painter()
	ok := p.DrawLinked(v, r.spritePass(), screen, prefix, x, y, mask, corners, tint, cs, tile, sw, sh)
	v.Cam = frame
	return ok
}

func (r *Renderer) linkedSprite(prefix string, x, y, mask, corners int, tint color.NRGBA, cs colorScale) (linkedDraw, bool) {
	p := r.painter()
	return p.LinkedSprite(prefix, x, y, mask, corners, tint, cs)
}

func (r *Renderer) linkKey(prefix string, mask, corners int) string {
	p := r.painter()
	return p.LinkKey(prefix, mask, corners)
}

func (r *Renderer) stoneTint(code uint8) color.NRGBA {
	p := r.painter()
	return p.StoneTint(code)
}

func (r *Renderer) stoneTintAt(w game.WorldView, x, y int) color.NRGBA {
	p := r.painter()
	return p.StoneTintAt(&w, x, y)
}

func (r *Renderer) stuffTint(stuff string) color.NRGBA {
	p := r.painter()
	return p.StuffTint(stuff)
}
