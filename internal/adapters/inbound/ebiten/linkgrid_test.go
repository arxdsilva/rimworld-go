package ebiten

import (
	"image/color"
	artpkg "rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/render/overlay"
	spritepkg "rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

// The grid's own tests are in render/sprite; these check the renderer's
// kinds (linkKinds) and the painter wrappers over the real art.

func TestLinkKindsClassify(t *testing.T) {
	t.Parallel()
	for kind, want := range map[string]spritepkg.LinkCell{
		"wall": spritepkg.WallLink, "door": spritepkg.WallLink, overlay.CoolerKind: spritepkg.WallLink,
		spritepkg.FenceKind: spritepkg.FenceLink, spritepkg.GateKind: spritepkg.FenceLink, "bed": {},
	} {
		if got := (overlay.LinkKinds{}).Building(kind); got != want {
			t.Fatalf("Building(%q) = %+v, want %+v", kind, got, want)
		}
	}
	for kind, want := range map[string]spritepkg.LinkCell{
		overlay.ConduitKind: spritepkg.ConduitLink, overlay.HiddenConduitKind: spritepkg.ConduitLink,
		overlay.PowerSwitchKind: {Flags: spritepkg.LinkConduit}, overlay.BatteryKind: {Flags: spritepkg.LinkDevice},
		overlay.CoolerKind: {Flags: spritepkg.LinkDevice}, "wall": {},
	} {
		if got := (overlay.LinkKinds{}).Power(kind); got != want {
			t.Fatalf("Power(%q) = %+v, want %+v", kind, got, want)
		}
	}
	// the renderer's grid links with these kinds: a fence joins the wall
	// beside it, a conduit the battery beside it
	w := game.WorldView{TerrainW: 3, Terrain: make([]uint8, 6), StructVer: 1, Buildings: []game.BuildingView{
		{Kind: spritepkg.FenceKind, X: 0, Y: 0, W: 1, H: 1}, {Kind: "wall", X: 1, Y: 0, W: 1, H: 1},
		{Kind: overlay.ConduitKind, X: 0, Y: 1, W: 1, H: 1}, {Kind: overlay.BatteryKind, X: 1, Y: 1, W: 1, H: 1},
	}}
	var g linkGrid
	g.Sync(w, 3, 2)
	if g.Mask(0, 0) != artpkg.WallE || g.Mask(1, 0) != 0 {
		t.Fatalf("fence beside a wall: masks %d %d", g.Mask(0, 0), g.Mask(1, 0))
	}
	if g.ConduitMask(0, 1) != artpkg.WallE {
		t.Fatalf("conduit beside a battery: mask %d", g.ConduitMask(0, 1))
	}
}

func TestGhostAndFrameWallsDrawWithArt(t *testing.T) {
	t.Parallel()
	art := testArt(t)
	r := testStoneArt()
	r.art, r.game = art, &stubSim{w: 8, h: 8}
	if !r.drawLinked(ebiten.NewImage(256, 256), "structure/wall_stone", 2, 2, artpkg.WallE|artpkg.WallS, spritepkg.CornerSE, r.stuffTint("granite blocks"), spritepkg.GhostOKCS, 32, 256, 256) {
		t.Fatal("a granite ghost wall has art")
	}
	if r.drawLinked(ebiten.NewImage(256, 256), "structure/no_such_family", 2, 2, 0, 0, color.NRGBA{}, spritepkg.NoCS, 32, 256, 256) {
		t.Fatal("unknown family draws nothing")
	}
}
