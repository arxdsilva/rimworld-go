package light

import (
	"math"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/camera"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/world/roof"
)

// fakeCasters is the shadow pass's renderer stand-in: fixed casters.
type fakeCasters struct {
	objects []Caster
	white   *ebiten.Image
}

func (f *fakeCasters) Objects(game.WorldView, []game.PawnView) []Caster { return f.objects }

func (f *fakeCasters) VisibleTiles() (x0, y0, x1, y1 int) { return 0, 0, 19, 11 }

func (f *fakeCasters) White() *ebiten.Image { return f.white }

func near(a, b, eps float64) bool { return math.Abs(a-b) <= eps }

// TestShadowPassWithoutTheRenderer runs the whole pass on hand-built
// casters by moonlight: the layer is composited once at half size on a 2×
// display, with a roofed run cleared; with shadows off, or nothing
// casting, nothing is drawn.
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
		white: ebiten.NewImage(1, 1),
	}
	var st Shadows
	screen := ebiten.NewImage(640, 400)
	st.Draw(v, lit, host, screen, w, 640, 400, v.Cam.Tile())
	if st.Composites != 1 {
		t.Fatalf("composites %d, want one", st.Composites)
	}
	if b := st.Layer.Bounds(); b.Dx() != 320 || b.Dy() != 200 {
		t.Fatalf("layer %v at device scale 2, want 320x200", b)
	}
	if r0, ok := st.BlurOpts.Uniforms["R0"].(float32); !ok || r0 <= 0 {
		t.Fatalf("blur radius %v", st.BlurOpts.Uniforms["R0"])
	}
	if _, ok := st.BlurOpts.Uniforms["R1"]; ok {
		t.Fatal("the blur still sets a height term (creature silhouettes are gone)")
	}

	// nothing casting: nothing composited
	empty := &fakeCasters{white: host.white}
	st.Draw(v, lit, empty, screen, w, 640, 400, v.Cam.Tile())
	if st.Composites != 1 {
		t.Fatal("composited a layer with no casters")
	}
	// shadows off: the layers are freed and nothing is drawn
	st.SetOff(true)
	if st.Layer != nil || st.Scratch != nil {
		t.Fatal("shadows off kept the screen-sized layers")
	}
	st.Draw(v, lit, host, screen, w, 640, 400, v.Cam.Tile())
	if st.Composites != 1 {
		t.Fatal("shadows off still drew")
	}
	st.SetOff(false)
	st.Draw(v, lit, host, screen, w, 640, 400, v.Cam.Tile())
	if st.Composites != 2 {
		t.Fatalf("shadows back on: composites %d, want 2", st.Composites)
	}
}

// TestShadowSpecCachePerCatalog: SpecOf caches a key's caster per
// catalog and starts afresh with a new one.
func TestShadowSpecCachePerCatalog(t *testing.T) {
	t.Parallel()
	var st Shadows
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

func TestBlurShaderCompiles(t *testing.T) {
	t.Parallel()
	if _, err := ebiten.NewShader(shadowBlurKage); err != nil {
		t.Fatalf("shadow_blur.kage: %v", err)
	}
	if r := BlurRadius(1); !near(r, 0.6, 1e-9) {
		t.Fatalf("radius %v", r)
	}
	if lo, hi := ShadowTint(0), ShadowTint(1); lo != [3]float32{18, 20, 34} || hi != [3]float32{18, 20, 30} {
		t.Fatalf("tint %v %v", lo, hi)
	}
}

func TestSunElevationFromLight(t *testing.T) {
	t.Parallel()
	for _, el := range []float64{0.1, 0.5, 1.1} {
		l := State{SunL: F32v(LightDir(el, 0.7))}
		if got := SunElevation(l); !near(got, el, 1e-5) {
			t.Fatalf("elevation %v, want %v", got, el)
		}
	}
	if SunElevation(LightAt(2, 15, "clear", 0)) != 0 {
		t.Fatal("the sun is up at 02:00")
	}
}
