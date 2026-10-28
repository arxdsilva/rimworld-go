package light

import (
	"image/color"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/camera"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/world/roof"
)

// fakeCasters is the shadow pass's renderer stand-in: fixed casters, and
// every figure one sprite drawn through the cast it is handed.
type fakeCasters struct {
	objects   []Caster
	creatures []Creature[string]
	sprite    *ebiten.Image
	white     *ebiten.Image
	drawn     map[string]int // figure casts by creature name (Fig)
	alphas    []float32
}

func (f *fakeCasters) Objects(game.WorldView, []game.PawnView) []Caster { return f.objects }

func (f *fakeCasters) Creatures(game.WorldView, []game.PawnView) []Creature[string] {
	return f.creatures
}

func (f *fakeCasters) CastFigure(c *Creature[string], cast *Cast, dst *ebiten.Image, tile float64) {
	f.drawn[c.Fig]++
	f.alphas = append(f.alphas, cast.Alpha)
	var geo ebiten.GeoM
	geo.Translate(c.WX*tile, c.WY*tile)
	cast.Silhouette(f.sprite, geo)
}

func (f *fakeCasters) VisibleTiles() (x0, y0, x1, y1 int) { return 0, 0, 19, 11 }

func (f *fakeCasters) White() *ebiten.Image { return f.white }

// TestShadowPassWithoutTheRenderer runs the whole pass on hand-built
// casters at night: the moon and a campfire cast the open-sky creature's
// silhouettes, the roofed one far from any light casts none, both get a
// contact pool, and the layer is composited once at half size on a 2×
// display; with shadows off, or nothing casting, nothing is drawn.
func TestShadowPassWithoutTheRenderer(t *testing.T) {
	t.Parallel()
	const mw, mh = 20, 12
	w := game.WorldView{TerrainW: mw, Terrain: make([]uint8, mw*mh), RoomGrid: make([]int, mw*mh), RoofCells: make([]uint8, mw*mh), DayOfYear: 15}
	for i := range w.RoomGrid {
		w.RoomGrid[i] = -1
	}
	for x := 12; x < 18; x++ { // a roofed run in rows 1..3
		for y := 1; y <= 3; y++ {
			w.RoofCells[y*mw+x] = uint8(roof.Built)
		}
	}
	w.Buildings = []game.BuildingView{{ID: 1, Kind: "campfire", X: 5, Y: 5, W: 1, H: 1, Burning: true, Glow: true, GlowRadius: 5}}
	v := &render.View{W: w, Cam: camera.Camera{Zoom: 0.5, Dev: 2, Placed: true}}
	lit := &Lit{Light: LightAt(23, 15, "clear", 0)}
	if lit.Light.ShadowAlpha <= 0 {
		t.Fatal("fixture: no moon shadows at 23:00")
	}
	host := &fakeCasters{
		objects: []Caster{
			{Spec: ShadowSpec{Kind: ShadowBox, W: 1, H: 1, D: 1}, X: 2, Y: 2},
			{Spec: ShadowSpec{Kind: ShadowBlob, W: 0.7, H: 2, D: 0.4}, X: 9.5, Y: 8.5},
		},
		creatures: []Creature[string]{
			{Fig: "by the fire", WX: 6, WY: 6, Px: 6.5, Py: 6.9, W: 0.55, H: 1.7, Opaque: 250, PPT: 128, Sky: true, Tx: 6, Ty: 6},
			{Fig: "indoors", WX: 15, WY: 2, Px: 15.5, Py: 2.9, W: 0.55, H: 1.7, Opaque: 250, PPT: 128, Tx: 15, Ty: 2},
		},
		sprite: ebiten.NewImage(16, 32),
		white:  ebiten.NewImage(1, 1),
		drawn:  map[string]int{},
	}
	host.sprite.Fill(color.White) // any coverage will do: pixels are never read back
	var st Shadows[string]
	screen := ebiten.NewImage(640, 400)
	st.Draw(v, lit, host, Rect{0, 0, mw, mh}, screen, w, 640, 400, v.Cam.Tile())

	if host.drawn["by the fire"] != 2 || host.drawn["indoors"] != 0 || st.Casts != 2 {
		t.Fatalf("silhouettes %v (%d cast), want the moon and the campfire for the open-sky creature only", host.drawn, st.Casts)
	}
	for _, a := range host.alphas {
		if a <= 0 || a > 1 {
			t.Fatalf("silhouette alpha %v", a)
		}
	}
	if pl := st.PointLights(v, lit, Rect{0, 0, mw, mh}); len(pl) != 1 || pl[0].ID != 1 || pl[0].H != CampfireLightH {
		t.Fatalf("point lights %+v, want the campfire", pl)
	}
	if st.Composites != 1 || len(st.Contacts.Verts) != 0 {
		t.Fatalf("composites %d, contact batch %d vertices left: want one composite, the pools flushed", st.Composites, len(st.Contacts.Verts))
	}
	if b := st.Layer.Bounds(); b.Dx() != 320 || b.Dy() != 200 {
		t.Fatalf("layer %v at device scale 2, want 320x200", b)
	}
	if r1, ok := st.BlurOpts.Uniforms["R1"].(float32); ok && r1 <= 0 {
		t.Fatalf("blur radius %v", r1)
	}

	// a far view culls the campfire's light: only the moon is left
	if pl := st.PointLights(v, lit, Rect{100, 100, 120, 112}); len(pl) != 0 {
		t.Fatalf("a light out of view: %+v", pl)
	}
	// nothing casting: nothing composited
	empty := &fakeCasters{sprite: host.sprite, white: host.white, drawn: map[string]int{}}
	st.Draw(v, lit, empty, Rect{0, 0, mw, mh}, screen, w, 640, 400, v.Cam.Tile())
	if st.Composites != 1 {
		t.Fatal("composited a layer with no casters")
	}
	// shadows off: the layers are freed and nothing is drawn
	st.SetOff(true)
	if st.Layer != nil || st.Scratch != nil {
		t.Fatal("shadows off kept the screen-sized layers")
	}
	st.Draw(v, lit, host, Rect{0, 0, mw, mh}, screen, w, 640, 400, v.Cam.Tile())
	if st.Composites != 1 || host.drawn["by the fire"] != 2 {
		t.Fatal("shadows off still drew")
	}
	st.SetOff(false)
	st.Draw(v, lit, host, Rect{0, 0, mw, mh}, screen, w, 640, 400, v.Cam.Tile())
	if st.Composites != 2 {
		t.Fatalf("shadows back on: composites %d, want 2", st.Composites)
	}
}

// TestShadowSpecCachePerCatalog: SpecOf caches a key's caster per
// catalog and starts afresh with a new one.
func TestShadowSpecCachePerCatalog(t *testing.T) {
	t.Parallel()
	var st Shadows[string]
	if s := st.SpecOf(nil, "structure/wall_wood_05"); s.Kind != ShadowBox {
		t.Fatalf("wall without art: %+v, want the box fallback", s)
	}
	if s := st.SpecOf(nil, "terrain/soil_00"); s.Kind != ShadowNone {
		t.Fatalf("terrain: %+v, want none", s)
	}
	if len(st.specs) != 2 {
		t.Fatalf("cached %d specs, want 2", len(st.specs))
	}
}
