package sprite

import (
	"image/color"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/camera"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/adapters/inbound/ebiten/render/light"
	"rimworld-go/internal/application/game"
)

// drawRig is a painter over hand-built one-tile sprites (pixels, no
// files) with lighting off, so a lit draw takes the plain path, and a
// view of 32 px tiles. draws lists the images Lit drew.
type drawRig struct {
	p     *Painter
	v     *render.View
	draws []*ebiten.Image
}

func newDrawRig(t *testing.T, keys ...string) *drawRig {
	t.Helper()
	c := &art.Catalog{Sprites: map[string]*art.Sprite{}}
	for _, k := range keys {
		c.Sprites[k] = &art.Sprite{
			Meta: art.Meta{Canvas: [2]int{32, 32}, Footprint: [2]int{1, 1}},
			Img:  ebiten.NewImage(32, 32),
		}
	}
	rig := &drawRig{v: &render.View{Cam: camera.Camera{Zoom: 32.0 / camera.TileSize, Dev: 1}, Art: c}}
	lit := &light.Lit{Off: true}
	lit.OnDraw = func(img *ebiten.Image, _ float32) { rig.draws = append(rig.draws, img) }
	rig.p = NewPainter(c, lit)
	return rig
}

func TestPainterDrawsLitSpritesAndSkipsTheRest(t *testing.T) {
	t.Parallel()
	rig := newDrawRig(t, "building/table", "terrain/soil_edge_01", "overlay/designation_haul")
	dst := ebiten.NewImage(256, 256)
	img := rig.p.Art.Image("building/table", color.NRGBA{})

	rig.p.Draw(rig.v, Pass{}, dst, "building/table", color.NRGBA{}, 2, 2, 32, 256, 256)
	if len(rig.draws) != 1 || rig.draws[0] != img {
		t.Fatalf("a world sprite draws lit once: %d draws", len(rig.draws))
	}
	rig.p.DrawCS(rig.v, Pass{}, dst, "terrain/soil_edge_01", color.NRGBA{}, 2, 2, 32, 256, 256, BlueprintCS)
	if len(rig.draws) != 2 {
		t.Fatalf("a ground edge draws lit (as ground): %d draws", len(rig.draws))
	}
	rig.p.DrawScaled(rig.v, Pass{}, dst, "overlay/designation_haul", color.NRGBA{}, 2, 2, 32, 256, 256, GhostOKCS, 0.5)
	if len(rig.draws) != 2 {
		t.Fatal("overlays (zones, marks) stay unlit")
	}
	rig.p.Draw(rig.v, Pass{}, dst, "building/missing", color.NRGBA{}, 2, 2, 32, 256, 256)
	rig.p.Draw(rig.v, Pass{}, dst, "building/table", color.NRGBA{}, 40, 40, 32, 256, 256)
	rig.p.Draw(rig.v, Pass{}, dst, "building/table", color.NRGBA{}, -3, 2, 32, 256, 256)
	if len(rig.draws) != 2 {
		t.Fatalf("missing and off-screen sprites draw nothing: %d draws", len(rig.draws))
	}
	rig.p.Draw(rig.v, Pass{Baking: true}, dst, "building/table", color.NRGBA{}, 2, 2, 32, 256, 256)
	if len(rig.draws) != 2 {
		t.Fatal("a bake draws the plain albedo, not a lit draw")
	}
	// the camera places the sprite: scrolled two tiles, (9, 2) is on screen
	rig.v.Cam.OffsetX = 64
	rig.p.Draw(rig.v, Pass{}, dst, "building/table", color.NRGBA{}, 9, 2, 32, 256, 256)
	if len(rig.draws) != 3 {
		t.Fatal("the view's camera positions the sprite")
	}
}

func TestPainterDrawsLinkedFamilies(t *testing.T) {
	t.Parallel()
	rig := newDrawRig(t, "structure/wall_stone_06", "rock/rock_wall_15")
	rig.p.Art.Tints = map[string]map[string]color.NRGBA{"stone_tint": {"granite": {0x69, 0x5f, 0x61, 255}}}
	dst := ebiten.NewImage(256, 256)
	tint := rig.p.StuffTint("granite blocks")
	if !rig.p.DrawLinked(rig.v, Pass{}, dst, "structure/wall_stone", 2, 2, art.WallE|art.WallS, CornerSE, tint, GhostOKCS, 32, 256, 256) {
		t.Fatal("a stone wall with art draws")
	}
	if rig.p.DrawLinked(rig.v, Pass{}, dst, "structure/no_such_family", 2, 2, 0, 0, color.NRGBA{}, NoCS, 32, 256, 256) {
		t.Fatal("an unknown family draws nothing")
	}
	if len(rig.draws) != 1 {
		t.Fatalf("one linked draw, got %d", len(rig.draws))
	}
	// no tint mask: the stone tint multiplies the colour scale instead
	d, ok := rig.p.LinkedSprite("rock/rock_wall", 3, 4, 15, 0, tint, NoCS)
	if !ok || d.key != "rock/rock_wall_15" || d.tint != (color.NRGBA{}) || d.cs != TintCS(tint) || d.x != 3 || d.y != 4 {
		t.Fatalf("linked rock = %+v %v", d, ok)
	}
	rig.p.DrawLinkedSprite(rig.v, Pass{}, dst, d, 32, 256, 256)
	if len(rig.draws) != 2 {
		t.Fatal("a prepared linked sprite draws")
	}
	w := game.WorldView{TerrainW: 2, StoneType: []uint8{0, 200}}
	if rig.p.StoneTintAt(&w, 0, 0) != tint || rig.p.StoneTintAt(&w, 1, 0) != tint || rig.p.StoneTintAt(&w, 5, 0) != tint {
		t.Fatal("unknown or missing stone data tints as granite")
	}
}

func TestColorScales(t *testing.T) {
	t.Parallel()
	if ItemCS(game.ItemView{Dried: true}) != DriedCS || ItemCS(game.ItemView{Rotten: true}) != RottenCS || ItemCS(game.ItemView{}) != NoCS {
		t.Fatal("ItemCS")
	}
	if LerpCS(NoCS, DriedCS, 0) != NoCS || LerpCS(ColorScale{}, DriedCS, 1) != DriedCS {
		t.Fatal("LerpCS ends")
	}
	if mid := LerpCS(ColorScale{0, 0, 0, 0}, ColorScale{1, 1, 1, 1}, 0.5); mid != (ColorScale{0.5, 0.5, 0.5, 0.5}) {
		t.Fatalf("LerpCS mid = %v", mid)
	}
	if !DoorAxisEW("south") || !DoorAxisEW("") || DoorAxisEW("east") || DoorAxisEW("west") {
		t.Fatal("DoorAxisEW")
	}
	if !Visible(-10, -10, 11, 11, 4, 4) || Visible(-10, 0, 9, 4, 4, 4) || Visible(4, 0, 1, 1, 4, 4) {
		t.Fatal("Visible")
	}
}

func TestSpriteKeys(t *testing.T) {
	t.Parallel()
	for n, want := range map[int]string{0: "00", 7: "07", 10: "10", 15: "15", 99: "99"} {
		if got := TwoDigits(n); got != want {
			t.Fatalf("TwoDigits(%d) = %q", n, got)
		}
	}
	if TileHash(3, 4) != TileHash(3, 4) || TileHash(3, 4) == TileHash(4, 3) {
		t.Fatal("TileHash is stable and not symmetric")
	}
	if GrassTint("temperate_forest") != NoCS || GrassTint("no such biome") != NoCS || GrassTint("desert") == NoCS {
		t.Fatal("GrassTint")
	}
	if !IsTreePlant("pine") || !IsTreePlant("jungle_tree") || IsTreePlant("bush") {
		t.Fatal("IsTreePlant")
	}
	if BiomeItemSprite("ember oil") != "item/ember_oil" || BiomeItemSprite("wood") != "" {
		t.Fatal("BiomeItemSprite")
	}
	if !IsFenceKind(FenceKind) || !IsFenceKind(GateKind) || IsFenceKind("wall") {
		t.Fatal("IsFenceKind")
	}
	if FenceSprite("wood", 5) != "structure/fence_wood_05" || FenceSprite("slate blocks", 15) != "structure/fence_stone_15" {
		t.Fatal("FenceSprite")
	}
	if WallPrefix("granite blocks") != "structure/wall_stone" || WallPrefix("steel") != "structure/wall_steel" || WallPrefix("wood") != "structure/wall_wood" {
		t.Fatal("WallPrefix")
	}
	if !IsRockWallName("rock_smooth") || IsRockWallName("stone") {
		t.Fatal("IsRockWallName")
	}
	if len(LinkedKeys("p")) != 47 {
		t.Fatalf("%d linked keys, want 47", len(LinkedKeys("p")))
	}
}

func TestDrawFenceVector(t *testing.T) {
	t.Parallel()
	dst := ebiten.NewImage(64, 64)
	for _, kind := range []string{FenceKind, GateKind} {
		for _, open := range []bool{false, true} {
			DrawFenceVector(dst, 0, 0, 32, kind, 15, open, open, 170) // every rail and both gate states
		}
	}
}
