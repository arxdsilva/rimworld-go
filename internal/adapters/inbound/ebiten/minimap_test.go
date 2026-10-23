package ebiten

import (
	"math"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

func TestMinimapClickMovesCamera(t *testing.T) {
	t.Parallel()
	const sw, sh = 1280, 800
	v := colonyViewport(sw, sh)
	r := New(&stubSim{w: 200, h: 200, world: game.WorldView{TerrainW: 200, Terrain: make([]uint8, 200*200)}})
	r.cam.zoom = 1
	r.clampCamera(v)
	if !r.minimapOn() {
		t.Fatal("a 200² map should show the minimap")
	}
	bx, by, mx, my, mw, mh := r.minimapBox(v)
	if bx+minimapSide != v.x1-minimapMargin || by+minimapSide != v.y1-minimapMargin-minimapAbove { // right edge, above the cell readout
		t.Fatalf("box (%v,%v) not at the right edge above the readout in %+v", bx, by, v)
	}
	// a click a quarter of the way in centres the camera on tile (50, 150)
	cx, cy := mx+mw*0.25, my+mh*0.75
	if !r.minimapClick(int(cx), int(cy), v) {
		t.Fatal("click on the minimap not taken")
	}
	tile := r.tilePx()
	centreX := (r.cam.offsetX + (v.x0+v.x1)/2) / tile
	centreY := (r.cam.offsetY + (v.y0+v.y1)/2) / tile
	if math.Abs(centreX-50) > 1.5 || math.Abs(centreY-150) > 1.5 {
		t.Fatalf("camera centred on (%.1f, %.1f), want (50, 150)", centreX, centreY)
	}
	if r.minimapClick(int(bx)-5, int(by)+10, v) {
		t.Fatal("a click beside the minimap was taken")
	}
	r.toggleMinimap()
	if r.minimapClick(int(cx), int(cy), v) {
		t.Fatal("a hidden minimap took a click")
	}
	small := New(&stubSim{w: 150, h: 150})
	if small.minimapOn() || small.minimapClick(int(cx), int(cy), v) {
		t.Fatal("a 150² map has no minimap")
	}
}

func TestMinimapPollsLayer(t *testing.T) {
	t.Parallel()
	w := game.WorldView{TerrainW: 160, Terrain: make([]uint8, 160*160)}
	sim := &stubSim{w: 160, h: 160, world: w, pawns: []game.PawnView{{ID: 1, X: 10, Y: 10}, {ID: 2, X: 20, Y: 20, Hostile: true}}}
	r := New(sim)
	screen := ebiten.NewImage(1280, 800)
	v := colonyViewport(1280, 800)
	r.drawMinimap(screen, v)
	if r.mini.polls != 1 || r.mini.img == nil || r.mini.w != 160 {
		t.Fatalf("first draw: polls %d img %v", r.mini.polls, r.mini.img != nil)
	}
	r.present += minimapPoll
	r.drawMinimap(screen, v)
	if r.mini.polls != 1 {
		t.Fatal("an unchanged layer was rebuilt")
	}
	t2 := append([]uint8(nil), w.Terrain...)
	t2[0] = 4
	sim.world.Terrain, sim.world.StructVer = t2, 9
	r.drawMinimap(screen, v) // within the poll interval: not asked yet
	if r.mini.polls != 1 {
		t.Fatal("polled before the interval")
	}
	r.present += minimapPoll
	r.drawMinimap(screen, v)
	if r.mini.polls != 2 {
		t.Fatalf("a changed ground was not picked up: polls %d", r.mini.polls)
	}
	if c := minimapColor(game.MinimapStructure | 1); c != minimapStructureClr {
		t.Fatal("structures take the structure colour")
	}
}
