package ebiten

import (
	"bytes"
	"math"
	"rimworld-go/internal/adapters/inbound/ebiten/render/light"
	"rimworld-go/internal/adapters/inbound/ebiten/render/overlay"
	worldpkg "rimworld-go/internal/adapters/inbound/ebiten/render/world"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/application/game"
)

func TestDirtPrintsThenSmudge(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		amount float64
		prints int
	}{{0.01, 1}, {0.09, 2}, {0.17, 3}, {0.249, 3}, {0.25, 0}, {0.6, 0}, {1, 0}} {
		if got := overlay.DirtPrintCount(c.amount); got != c.prints {
			t.Errorf("amount %v: %d prints, want %d", c.amount, got, c.prints)
		}
	}
	// prints draw one tile wide; the smudge grows from 0.55 to a full tile
	if l := overlay.DirtLookOf(game.DirtView{X: 1, Y: 1, Amount: 0.1}); l.Size != 1 || l.Tex >= overlay.DirtPrintN {
		t.Fatalf("a print group: %+v", l)
	}
	a := overlay.DirtLookOf(game.DirtView{X: 1, Y: 1, Amount: 0.25})
	b := overlay.DirtLookOf(game.DirtView{X: 1, Y: 1, Amount: 0.6})
	c := overlay.DirtLookOf(game.DirtView{X: 1, Y: 1, Amount: 1})
	if a.Tex < overlay.DirtPrintN || math.Abs(a.Size-0.55) > 1e-9 || !(a.Size < b.Size && b.Size < c.Size) || math.Abs(c.Size-1) > 1e-9 {
		t.Fatalf("smudges grow: %v %v %v", a.Size, b.Size, c.Size)
	}
}

func TestDirtTexturesDeterministic(t *testing.T) {
	t.Parallel()
	for i := 0; i < overlay.DirtTexN; i++ {
		a, b := overlay.DirtPixels(i), overlay.DirtPixels(i)
		if !bytes.Equal(a.Pix, b.Pix) {
			t.Fatalf("texture %d differs between builds", i)
		}
		opaque := 0
		for y := 0; y < overlay.DirtPx; y++ {
			for x := 0; x < overlay.DirtPx; x++ {
				al := a.NRGBAAt(x, y).A
				if al > 0 {
					opaque++
				}
				if (x == 0 || y == 0 || x == overlay.DirtPx-1 || y == overlay.DirtPx-1) && al != 0 {
					t.Fatalf("texture %d: border pixel (%d,%d) not transparent", i, x, y)
				}
			}
		}
		if opaque < 60 {
			t.Fatalf("texture %d is nearly empty (%d px)", i, opaque)
		}
	}
	// more prints cover more; variants differ
	if bytes.Equal(overlay.DirtPixels(overlay.DirtTexIndex(1, 0)).Pix, overlay.DirtPixels(overlay.DirtTexIndex(1, 1)).Pix) ||
		bytes.Equal(overlay.DirtPixels(overlay.DirtTexIndex(0, 0)).Pix, overlay.DirtPixels(overlay.DirtTexIndex(0, 1)).Pix) {
		t.Fatal("variants must differ")
	}
	cover := func(tex int) (n int) {
		img := overlay.DirtPixels(tex)
		for y := 0; y < overlay.DirtPx; y++ {
			for x := 0; x < overlay.DirtPx; x++ {
				if img.NRGBAAt(x, y).A > 0 {
					n++
				}
			}
		}
		return
	}
	if !(cover(overlay.DirtTexIndex(1, 0)) < cover(overlay.DirtTexIndex(3, 0))) {
		t.Fatal("three prints cover more than one")
	}
	if overlay.DirtTexIndex(-1, -3) < overlay.DirtPrintN || overlay.DirtTexIndex(9, 7) >= overlay.DirtPrintN {
		t.Fatal("texture index clamps")
	}
}

func TestOffscreenDirtSkipped(t *testing.T) {
	t.Parallel()
	r := stainRenderer()
	tile := r.cam.Tile()
	w := game.WorldView{Dirt: []game.DirtView{
		{X: 2, Y: 2, Amount: 0.5},
		{X: 9, Y: 5, Amount: 0.1},
		{X: 50, Y: 50, Amount: 1},
		{X: 3, Y: 40, Amount: 1},
		{X: 4, Y: 4, Amount: 0}, // cleaned
	}}
	r.view.W = w
	var b overlay.StainBatch
	if n := r.buildDirt(&b, w, 320, 320, tile); n != 2 || len(b.Verts) != 8 || len(b.Idx) != 12 {
		t.Fatalf("on screen: %d quads, %d verts, %d indices", n, len(b.Verts), len(b.Idx))
	}
	for _, v := range b.Verts {
		if v.Custom3 != 0 || v.Custom2 != 0 {
			t.Fatalf("dirt is lit flat and carries no figure blood: %+v", v)
		}
	}
	screen := ebiten.NewImage(320, 320)
	r.drawDirt(screen, w, 320, 320, tile) // one batch, no panic
	r.drawDirt(screen, game.WorldView{}, 320, 320, tile)
}

func TestDirtLitByTheirTile(t *testing.T) {
	t.Parallel()
	r := stainRenderer()
	w := roomWorld()
	w.Dirt = []game.DirtView{{X: 0, Y: 0, Amount: 0.5}}
	r.view.W = w
	var b overlay.StainBatch
	r.buildDirt(&b, w, 320, 320, r.cam.Tile())
	sky, lamp := r.lightTile(0, 0)
	if len(b.Verts) != 4 || b.Verts[0].Custom0 != sky || b.Verts[0].Custom1 != lamp {
		t.Fatalf("dirt lit as its tile (%v, %v): %+v", sky, lamp, b.Verts)
	}
}

// TestDirtDrawOrder: dirt lies on the floors and under the blood, and
// both under the zones.
func TestDirtDrawOrder(t *testing.T) {
	t.Parallel()
	r := stainRenderer()
	var order []string
	r.view.Trace = &render.Trace{Layer: func(n string) { order = append(order, n) }}
	w := game.WorldView{
		Dirt:   []game.DirtView{{X: 2, Y: 2, Amount: 0.5}},
		Stains: []game.StainView{{X: 2, Y: 2, Kind: "blood", Amount: 0.5}},
	}
	r.view.W = w
	r.drawFloorLayers(ebiten.NewImage(320, 320), w, 320, 320, r.cam.Tile())
	if got := strings.Join(order, ","); got != "floors,dirt,stains" {
		t.Fatalf("layer order = %s, want floors,dirt,stains", got)
	}
	// without dirt or blood only the floors are traced
	order = nil
	r.drawFloorLayers(ebiten.NewImage(320, 320), game.WorldView{}, 320, 320, r.cam.Tile())
	if got := strings.Join(order, ","); got != "floors" {
		t.Fatalf("empty world traces %s", got)
	}
}

func TestRoomLabelCleanliness(t *testing.T) {
	t.Parallel()
	w := game.WorldView{TerrainW: 4, RoomGrid: []int{0, 1, 2, 3}, Rooms: []game.RoomView{
		{ID: 0, Role: "bedroom", Label: "decent", Score: 12, Roofed: 1, Cleanliness: "clean"},
		{ID: 1, Role: "bedroom", Label: "decent", Score: 9, Roofed: 1, Cleanliness: "dirty", FilthPenalty: 3},
		{ID: 2, Role: "barn", Label: "dull", Score: 3, Roofed: 1, Cleanliness: "slightly dirty"},
		{ID: 3, Role: "bedroom", Label: "awful", Score: -2, Roofed: 1, Cleanliness: "filthy", FilthPenalty: 10, Lit: true},
	}}
	for x, want := range []string{
		"Room: bedroom, decent (12), roofed 100%",
		"Room: bedroom, decent (9), dirty (-3), roofed 100%",
		"Room: barn, dull (3), slightly dirty, roofed 100%",
		"Room: bedroom, awful (-2), filthy (-10), lit, roofed 100%",
	} {
		if got := worldpkg.RoomLabelAt(w, x, 0); got != want {
			t.Errorf("room %d: %q, want %q", x, got, want)
		}
	}
	// a view that predates cleanliness keeps the old label
	old := game.WorldView{TerrainW: 1, RoomGrid: []int{0}, Rooms: []game.RoomView{{Role: "bedroom", Label: "decent", Score: 5, Roofed: 1}}}
	if got := worldpkg.RoomLabelAt(old, 0, 0); got != "Room: bedroom, decent (5), roofed 100%" {
		t.Errorf("no cleanliness: %q", got)
	}
}

func TestCarpetDrawnUnderPawns(t *testing.T) {
	t.Parallel()
	// carpets lie in the floor-marking pass, never in the object pass that
	// sorts furniture with pawns
	for kind, want := range map[string]bool{"carpet": true, "sleeping spot": true, "bed": false, "table": false, "floor": false} {
		if worldpkg.OnFloor(kind) != want {
			t.Errorf("onFloor(%q) = %v", kind, !want)
		}
	}
	for stuff, key := range map[string]string{
		"straw carpet": "item/carpet_straw_rolled", "cloth carpet": "item/carpet_cloth_rolled",
		"leather carpet": "item/carpet_leather_rolled", "wood carpet": "item/carpet_wood_rolled",
		"cloth": "item/cloth_bolt", "straw": "item/straw_bundle",
	} {
		if got := worldpkg.ItemSprite(stuff); got != key {
			t.Errorf("itemSprite(%q) = %q, want %q", stuff, got, key)
		}
	}
}

// TestCarpetFramePass: a carpet and its frame draw in the floor pass
// without panicking, with or without art.
func TestCarpetFramePass(t *testing.T) {
	art := testArt(t)
	g := game.NewColony(game.Config{Seed: 7, Width: 40, Height: 30})
	r := New(&carpetSim{Game: g})
	r.art = art
	screen := ebiten.NewImage(640, 400)
	tile := r.cam.Tile()
	r.pipe().Env.SetView(r.liveView(), 640, 400, tile)
	r.drawArt(screen, 640, 400, tile)
}

// carpetSim adds a built carpet and a carpet frame to the colony's view.
type carpetSim struct{ *game.Game }

func (s *carpetSim) World() game.WorldView {
	w := s.Game.World()
	w.Buildings = append(w.Buildings, game.BuildingView{Kind: "carpet", Stuff: "straw carpet", X: 3, Y: 3, W: 1, H: 1})
	w.Frames = append(w.Frames, game.FrameView{Kind: "carpet", Stuff: "cloth carpet", X: 4, Y: 3, W: 1, H: 1})
	return w
}

// dirtSim is a colony whose world view carries hand-built dirt.
type dirtSim struct {
	*game.Game
	dirt []game.DirtView
}

func (s *dirtSim) World() game.WorldView {
	w := s.Game.World()
	w.Dirt = append(w.Dirt, s.dirt...)
	return w
}

// benchDirt is n dirty tiles on a w×h map, every amount.
func benchDirt(n, w, h int) []game.DirtView {
	out := make([]game.DirtView, n)
	for i := range out {
		out[i] = game.DirtView{X: (i * 7) % w, Y: (i * 13 / w) % h, Amount: 0.02 + float64(i%25)/26}
	}
	return out
}

// BenchmarkDrawDirt is the dirt pass alone with 1000 dirty tiles on
// screen (gate: at most 0.3 ms).
func BenchmarkDrawDirt(b *testing.B) {
	g := game.NewColony(game.Config{Seed: 7, Width: 80, Height: 50})
	r := New(&dirtSim{Game: g, dirt: benchDirt(1000, 80, 50)})
	r.cam.Zoom = 0.6
	screen := ebiten.NewImage(1280, 800)
	tile := r.cam.Tile()
	r.pipe().Env.SetView(r.liveView(), 1280, 800, tile)
	r.view.W = r.game.World()
	r.lit.Light = light.LightAt(12, summerDay, "clear", 0)
	for i := 0; i < 5; i++ {
		r.drawDirt(screen, r.view.W, 1280, 800, tile)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.drawDirt(screen, r.view.W, 1280, 800, tile)
	}
}
