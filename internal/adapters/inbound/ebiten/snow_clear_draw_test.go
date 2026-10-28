package ebiten

import (
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

// snowClearWorld is a 10x10 map whose snow clearing area is the 3x2 block
// at (2,3)..(4,4).
func snowClearWorld(ver uint64) game.WorldView {
	w := game.WorldView{TerrainW: 10, SnowClear: make([]uint8, 100), SnowClearVer: ver}
	for y := 3; y <= 4; y++ {
		for x := 2; x <= 4; x++ {
			w.SnowClear[y*10+x] = 1
		}
	}
	return w
}

func TestSnowClearOverlayOnlyWithToolHeld(t *testing.T) {
	t.Parallel()
	r := stainRenderer()
	tile := r.tilePx()
	screen := ebiten.NewImage(320, 320)
	w := snowClearWorld(1)
	r.drawSnowClearOverlay(screen, w, 320, 320, tile)
	if r.overlays().SnowDraw.Verts != 0 || r.overlays().SnowDraw.Builds != 0 {
		t.Fatal("no tool held: nothing drawn")
	}
	r.overlays().SnowClear.Forced = true
	r.drawSnowClearOverlay(screen, w, 320, 320, tile)
	// one fill rect per row run (2) plus the 1 px outline: 10 edge strips
	if got := r.overlays().SnowDraw.Verts; got != 4*(2+10) {
		t.Fatalf("held: %d verts, want %d", got, 4*(2+10))
	}
	for i := 0; i < 3; i++ {
		r.drawSnowClearOverlay(screen, w, 320, 320, tile)
	}
	if r.overlays().SnowDraw.Builds != 1 {
		t.Fatalf("same version rebuilt %d times", r.overlays().SnowDraw.Builds)
	}
	w2 := snowClearWorld(2)
	w2.SnowClear[8*10+8] = 1
	r.drawSnowClearOverlay(screen, w2, 320, 320, tile)
	if r.overlays().SnowDraw.Builds != 2 || r.overlays().SnowDraw.Verts != 4*(2+10+1+4) {
		t.Fatalf("new version: %d builds, %d verts", r.overlays().SnowDraw.Builds, r.overlays().SnowDraw.Verts)
	}
	r.drawSnowClearOverlay(screen, game.WorldView{}, 320, 320, tile)
	if r.overlays().SnowDraw.Verts != 0 {
		t.Fatal("no area: nothing drawn")
	}
}

func TestSnowClearToolsInZones(t *testing.T) {
	t.Parallel()
	found := 0
	for _, tl := range archTools {
		if !snowClearToolID(tl.id) {
			continue
		}
		found++
		if tl.category != "zones" || tl.cmd != command.DesignateSnowClear || tl.mode != placeArea || tl.icon != "snow_clear" {
			t.Fatalf("tool %+v", tl)
		}
		if (tl.id == "snow_clear") != tl.enable {
			t.Fatalf("%s paints %v", tl.id, tl.enable)
		}
	}
	if found != 2 {
		t.Fatalf("%d snow clearing tools, want 2", found)
	}
}

func TestSnowClearOverlayForcedByTool(t *testing.T) {
	t.Parallel()
	a := &App{renderer: stainRenderer()}
	a.archOpen, a.archTool = true, "snow_clear"
	a.syncSnowClearForce()
	if !a.renderer.overlays().SnowClear.Forced {
		t.Fatal("the tool in hand shows the area")
	}
	a.archTool = "home"
	a.syncSnowClearForce()
	if a.renderer.overlays().SnowClear.Forced {
		t.Fatal("another tool hides it")
	}
	(&App{}).syncSnowClearForce() // menus: no renderer
}
