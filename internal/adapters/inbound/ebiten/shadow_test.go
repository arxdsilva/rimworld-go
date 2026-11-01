package ebiten

import (
	"math"
	artpkg "rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/render/env"
	lightpkg "rimworld-go/internal/adapters/inbound/ebiten/render/light"
	"rimworld-go/internal/adapters/inbound/ebiten/render/overlay"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

// testLight is a hand-built light: shadows fall away from a light at
// azimuth az (screen space, 0 = east) and elevation el (radians), with
// the ART §6.1 length clamp and the given alpha.
func testLight(az, el, alpha float64) lightpkg.State {
	lx, ly := math.Cos(az)*math.Cos(el), 0.5*math.Sin(az)*math.Cos(el)
	n := math.Hypot(lx, ly)
	return lightpkg.State{
		SunL:      [3]float32{float32(lx), float32(ly), float32(math.Sin(el))},
		ShadowDir: [2]float64{-lx / n, -ly / n},
		ShadowLen: func(h float64) float64 {
			return math.Min(math.Max(h/math.Tan(el), 0.25*h), lightpkg.ShadowMaxLen)
		},
		ShadowAlpha: alpha,
	}
}

func TestStormFadesShadows(t *testing.T) {
	t.Parallel()
	clear := lightpkg.ShadowCompositeScale(testLight(1, 1, 0.32))
	storm := lightpkg.ShadowCompositeScale(testLight(1, 1, 0.32*(1-0.6)))
	if !(storm[3] < clear[3]) || math.Abs(float64(storm[3])-0.128) > 1e-4 {
		t.Fatalf("storm alpha %.3f clear %.3f", storm[3], clear[3])
	}
	// no shadows at all: the pass draws nothing
	r := shadowRenderer(t, shadowWorld(nil))
	r.lit.Light = testLight(1, 1, 0)
	r.drawShadows(ebiten.NewImage(64, 64), shadowWorld(nil), 64, 64, r.cam.Tile())
	if r.env.Shadow.Composites != 0 {
		t.Fatal("composited a shadow layer with zero shadow alpha")
	}
}

func kindOf(m *artpkg.Meta) string {
	if m == nil {
		return "<nil>"
	}
	return m.Shadow.Kind
}

// shadowWorld is a 20x12 open map with the given roof cells set (index ->
// flags); every tile is outdoors.
func shadowWorld(roofs map[int]uint8) game.WorldView {
	w := game.WorldView{TerrainW: 20, Terrain: make([]uint8, 20*12), RoomGrid: make([]int, 20*12)}
	for i := range w.RoomGrid {
		w.RoomGrid[i] = -1
	}
	if roofs != nil {
		w.RoofCells = make([]uint8, 20*12)
		for i, c := range roofs {
			w.RoofCells[i] = c
		}
	}
	return w
}

func shadowRenderer(t *testing.T, w game.WorldView) *Renderer {
	t.Helper()
	r := New(&stubSim{w: 20, h: 12})
	r.art = envArt()
	r.env.View = env.Rect{X0: 0, Y0: 0, X1: 20, Y1: 12}
	return r
}

func TestNoShadowsUnderRoof(t *testing.T) {
	t.Parallel()
	w := shadowWorld(map[int]uint8{3*20 + 4: overlay.RoofBuilt, 3*20 + 6: overlay.RoofNatural, 3*20 + 8: overlay.RoofPlanBuild})
	w.Items = []game.ItemView{{Kind: "wood", X: 4, Y: 3}, {Kind: "wood", X: 6, Y: 3}, {Kind: "wood", X: 8, Y: 3}, {Kind: "wood", X: 10, Y: 3}}
	w.Buildings = []game.BuildingView{{Kind: "wall", X: 4, Y: 3, W: 1, H: 1}, {Kind: "wall", X: 11, Y: 3, W: 1, H: 1},
		{Kind: "floor", Floor: true, X: 12, Y: 3, W: 1, H: 1}}
	pawns := []game.PawnView{{ID: 1, X: 4, Y: 3}, {ID: 2, X: 9, Y: 3}, {ID: 3, X: 13, Y: 3, Embodied: true}}
	r := shadowRenderer(t, w)
	got := map[[2]int]int{}
	for _, c := range r.shadowCasters(w, pawns) {
		got[[2]int{int(math.Floor(c.X)), int(math.Floor(c.Y))}]++
	}
	for _, roofedTile := range [][2]int{{4, 3}, {6, 3}} {
		if got[roofedTile] != 0 {
			t.Fatalf("caster under a roof at %v: %v", roofedTile, got)
		}
	}
	for _, open := range [][2]int{{8, 3}, {10, 3}, {11, 3}} { // a planned roof is not a roof yet
		if got[open] == 0 {
			t.Fatalf("no caster on open ground at %v: %v", open, got)
		}
	}
	if got[[2]int{12, 3}] != 0 || got[[2]int{9, 3}] != 0 || got[[2]int{13, 3}] != 0 {
		t.Fatalf("floors or pawns cast a box or blob: %v", got)
	}
	// off-screen casters (beyond the longest shadow) are skipped
	r.env.View = env.Rect{X0: 0, Y0: 0, X1: 2, Y1: 2}
	far := game.WorldView{TerrainW: w.TerrainW, Items: []game.ItemView{{Kind: "wood", X: 1, Y: 1}, {Kind: "wood", X: 19, Y: 11}}}
	if n := len(r.shadowCasters(far, nil)); n != 1 {
		t.Fatalf("got %d casters, want only the one in view", n)
	}
}

func TestShadowsClippedAtRoofedTiles(t *testing.T) {
	t.Parallel()
	w := shadowWorld(map[int]uint8{
		2*20 + 3: overlay.RoofBuilt, 2*20 + 4: overlay.RoofBuilt, 2*20 + 5: overlay.RoofNatural, // one run of three
		2*20 + 7: overlay.RoofBuilt,     // a run of one
		5*20 + 0: overlay.RoofPlanBuild, // planned only: not clipped
	})
	runs := lightpkg.RoofRuns(w, 0, 0, 19, 11, nil)
	want := [][3]int{{2, 3, 6}, {2, 7, 8}}
	if len(runs) != len(want) {
		t.Fatalf("runs %v, want %v", runs, want)
	}
	for i := range want {
		if runs[i] != want[i] {
			t.Fatalf("runs %v, want %v", runs, want)
		}
	}
	if n := len(lightpkg.RoofRuns(shadowWorld(nil), 0, 0, 19, 11, nil)); n != 0 {
		t.Fatalf("no roof data clipped %d runs", n)
	}
	// a full frame: the shadow is drawn and the roofed run cleared
	r := shadowRenderer(t, w)
	r.lit.Light = testLight(0.4, 0.4, 0.3)
	w.Buildings = []game.BuildingView{{Kind: "wall", X: 2, Y: 2, W: 1, H: 1}}
	w.Plants = []game.PlantView{{Species: "tree", X: 9, Y: 2, Growth: 60}}
	w.Items = []game.ItemView{{Kind: "wood", X: 10, Y: 3}}
	w.Animals = []game.AnimalView{{ID: 7, Species: "dustbuck", X: 12, Y: 4}}
	r.view.Pawns = []game.PawnView{{ID: 1, X: 11, Y: 3}}
	if n := len(r.shadowCasters(w, r.view.Pawns)); n != 3 {
		t.Fatalf("got %d box and blob casters, want 3", n)
	}
	r.drawShadows(ebiten.NewImage(640, 400), w, 640, 400, r.cam.Tile())
	if r.env.Shadow.Composites != 1 {
		t.Fatalf("composites %d, want 1", r.env.Shadow.Composites)
	}
}

func TestWallShadowsMergeSingleAlpha(t *testing.T) {
	t.Parallel()
	// the layer merges by max, so overlapping shadows keep one alpha
	if lightpkg.ShadowBlend.BlendOperationAlpha != ebiten.BlendOperationMax || lightpkg.ShadowBlend.BlendOperationRGB != ebiten.BlendOperationMax {
		t.Fatalf("shadow layer blend %+v is not max", lightpkg.ShadowBlend)
	}
	w := shadowWorld(nil)
	for x := 2; x < 12; x++ {
		w.Buildings = append(w.Buildings, game.BuildingView{Kind: "wall", X: x, Y: 4, W: 1, H: 1})
	}
	r := shadowRenderer(t, w)
	r.lit.Light = testLight(0.3, 0.3, 0.32)
	casters := r.shadowCasters(w, nil)
	if len(casters) != 10 {
		t.Fatalf("got %d wall casters, want one box per wall tile", len(casters))
	}
	var b gfx.TriBatch
	for _, c := range casters {
		if c.Spec.Kind != lightpkg.ShadowBox {
			t.Fatalf("wall caster %+v is not a box", c)
		}
		r.addBox(&b, c, r.lit.Light, r.cam.Tile(), 0.32)
	}
	// every vertex is at the shadow alpha (core) or clear (feather edge):
	// overlapping walls max to exactly the core alpha, never more; boxes
	// carry no height (red 0) for the penumbra blur
	for _, v := range b.Verts {
		if v.ColorA != 0 && v.ColorA != 0.32 || v.ColorR != 0 {
			t.Fatalf("vertex alpha %.3f red %.3f", v.ColorA, v.ColorR)
		}
	}
	// the whole layer is composited once, at the light's shadow alpha
	r.drawShadows(ebiten.NewImage(640, 400), w, 640, 400, r.cam.Tile())
	if r.env.Shadow.Composites != 1 {
		t.Fatalf("composited %d times", r.env.Shadow.Composites)
	}
	if cs := lightpkg.ShadowCompositeScale(r.lit.Light); math.Abs(float64(cs[3])-0.32) > 1e-6 {
		t.Fatalf("composite alpha %.3f", cs[3])
	}
	// and the layer is reused across frames, reallocated only on resize
	layer := r.env.Shadow.Layer
	r.drawShadows(ebiten.NewImage(640, 400), w, 640, 400, r.cam.Tile())
	if r.env.Shadow.Layer != layer {
		t.Fatal("layer reallocated without a resize")
	}
	r.drawShadows(ebiten.NewImage(320, 200), w, 320, 200, r.cam.Tile())
	if r.env.Shadow.Layer.Bounds().Dx() != 320 {
		t.Fatal("layer not resized with the screen")
	}
}

func TestShadowLayerHalfResolution(t *testing.T) {
	t.Parallel()
	w := shadowWorld(nil)
	w.Buildings = []game.BuildingView{{Kind: "wall", X: 4, Y: 4, W: 1, H: 1}}
	run := func(dev float64) (*Renderer, float32) {
		r := shadowRenderer(t, w)
		r.cam.Dev = dev
		r.lit.Light = testLight(0.3, 0.3, 0.32)
		r.drawShadows(ebiten.NewImage(641, 401), w, 641, 401, r.cam.Tile())
		if r.env.Shadow.Composites != 1 {
			t.Fatalf("dev %v: composites %d, want 1", dev, r.env.Shadow.Composites)
		}
		return r, r.env.Shadow.BlurOpts.Uniforms["R0"].(float32)
	}
	r1, r1dev1 := run(1)
	if b := r1.env.Shadow.Layer.Bounds(); b.Dx() != 641 || b.Dy() != 401 {
		t.Fatalf("dev 1 layer %v, want 641x401", b)
	}
	r2, r1dev2 := run(2)
	for _, img := range []*ebiten.Image{r2.env.Shadow.Layer, r2.env.Shadow.Scratch} {
		if b := img.Bounds(); b.Dx() != 321 || b.Dy() != 201 {
			t.Fatalf("dev 2 layer %v, want 321x201", b)
		}
	}
	if r1dev1 <= 0 {
		t.Skip("blur shader unavailable")
	}
	// at dev 2 the same zoom is twice the tile px, then halved by the layer
	if math.Abs(float64(r1dev2-r1dev1)) > 1e-4 {
		t.Fatalf("R0 dev2 %v vs dev1 %v: radii are px at the layer's zoom", r1dev2, r1dev1)
	}
}

func TestShadowHalfResolutionKeepsWorldPlacement(t *testing.T) {
	t.Parallel()
	w := shadowWorld(nil)
	w.Buildings = []game.BuildingView{{Kind: "wall", X: 2, Y: 2, W: 1, H: 1}}
	r := shadowRenderer(t, w)
	r.cam.Dev = 2
	r.lit.Light = testLight(1.2, 0.9, 0.5) // high sun: a short shadow hugging the wall
	screen := ebiten.NewImage(640, 400)
	tile := r.cam.Tile()
	r.drawShadows(screen, w, 640, 400, tile)
	cx, cy := r.worldToScreenF(2.5, 2.5)
	// pixels cannot be read without a running game, so check the geometry
	// the layer is drawn from: the wall's box at layer scale is the screen
	// box halved, and still covers the half-scaled wall centre
	st := &r.env.Shadow
	box := func(k float32) (lo, hi [2]float32) {
		var b gfx.TriBatch
		b.K = k
		for _, c := range r.shadowCasters(w, nil) {
			r.addBox(&b, c, r.lit.Light, tile, 0.5)
		}
		if len(b.Verts) == 0 {
			t.Fatal("no box vertices")
		}
		lo, hi = [2]float32{1e9, 1e9}, [2]float32{-1e9, -1e9}
		for _, v := range b.Verts {
			lo[0], lo[1] = min(lo[0], v.DstX), min(lo[1], v.DstY)
			hi[0], hi[1] = max(hi[0], v.DstX), max(hi[1], v.DstY)
		}
		return
	}
	flo, fhi := box(0)
	hlo, hhi := box(0.5)
	for i := 0; i < 2; i++ {
		if math.Abs(float64(hlo[i]-flo[i]/2)) > 1e-3 || math.Abs(float64(hhi[i]-fhi[i]/2)) > 1e-3 {
			t.Fatalf("half-scale box %v..%v is not half of %v..%v", hlo, hhi, flo, fhi)
		}
	}
	if lx, ly := float32(cx)/2, float32(cy)/2; lx < hlo[0] || lx > hhi[0] || ly < hlo[1] || ly > hhi[1] {
		t.Fatalf("wall centre (%.1f,%.1f) outside the layer box %v..%v", lx, ly, hlo, hhi)
	}
	if b := st.Layer.Bounds(); b.Dx() != 320 || b.Dy() != 200 {
		t.Fatalf("layer %v, want 320x200", b)
	}
	_ = screen
}

// TestCreaturesCastNoShadow: pawns and animals (with art, standing in the
// sun, by a lit lamp at night and in a lamp-lit roofed room at noon) cast
// nothing (user decision, 2026-10-31): alone they leave the shadow pass
// idle (no layer, no composite), and beside a wall and a lamp the pass
// lists the same casters and draws the same as without them, with no lit
// draw of a figure.
func TestCreaturesCastNoShadow(t *testing.T) {
	t.Parallel()
	creatureArt := func(r *Renderer) {
		r.art.Sprites["pawn/body_south"] = fakeSprite([2]int{160, 297}, pt{16, 153}, nil)
		for _, f := range []string{"south", "east", "north"} {
			r.art.Sprites["animal/dustbuck_"+f] = fakeSprite([2]int{128, 128}, pt{64, 110}, nil)
		}
	}
	pawns := []game.PawnView{{ID: 1, X: 4, Y: 3}, {ID: 2, X: 9, Y: 6}, {ID: 3, X: 15, Y: 2}, {ID: 4, X: 6, Y: 8, Lying: true}}
	animals := []game.AnimalView{{ID: 7, Species: "dustbuck", X: 5, Y: 5}, {ID: 8, Species: "dustbuck", X: 10, Y: 6}}
	roofs := map[int]uint8{}
	for x := 13; x < 18; x++ { // a roofed room around (15, 2)
		for y := 1; y <= 3; y++ {
			roofs[y*20+x] = overlay.RoofBuilt
		}
	}
	for _, c := range []struct {
		name  string
		light lightpkg.State
	}{
		{"noon sun", lightpkg.LightAt(12, 15, "clear", 0)},
		{"low sun", testLight(0.4, 0.3, 0.32)},
		{"moon and lamps", lightpkg.LightAt(23, 15, "clear", 0)},
	} {
		if c.light.ShadowAlpha <= 0 {
			t.Fatalf("%s: fixture casts no shadows at all", c.name)
		}
		// creatures alone: the pass never runs
		w := shadowWorld(roofs)
		w.Animals = animals
		r := shadowRenderer(t, w)
		creatureArt(r)
		r.lit.Light = c.light
		r.view.Pawns = pawns
		lit := 0
		r.lit.OnDraw = func(*ebiten.Image, float32) { lit++ }
		r.drawShadows(ebiten.NewImage(640, 400), w, 640, 400, r.cam.Tile())
		if r.env.Shadow.Composites != 0 || r.env.Shadow.Layer != nil || lit != 0 {
			t.Fatalf("%s: creatures alone drew a shadow layer (composites %d, lit draws %d)", c.name, r.env.Shadow.Composites, lit)
		}

		// beside a wall and lamps (outdoors and in the roofed room): the
		// same casters and composites as the same world without creatures
		frame := func(withCreatures bool) (casters []lightpkg.Caster, composites, litDraws int) {
			w := shadowWorld(roofs)
			w.Buildings = []game.BuildingView{{Kind: "wall", X: 2, Y: 2, W: 1, H: 1}, lampBuilding(20, 8, 5), lampBuilding(21, 14, 2)}
			var ps []game.PawnView
			if withCreatures {
				w.Animals, ps = animals, pawns
			}
			r := shadowRenderer(t, w)
			creatureArt(r)
			r.lit.Light = c.light
			r.view.W, r.view.Pawns = w, ps
			n := 0
			r.lit.OnDraw = func(*ebiten.Image, float32) { n++ }
			r.drawShadows(ebiten.NewImage(640, 400), w, 640, 400, r.cam.Tile())
			return append([]lightpkg.Caster(nil), r.env.Shadow.Objects...), r.env.Shadow.Composites, n
		}
		bare, bareN, _ := frame(false)
		got, gotN, litDraws := frame(true)
		if len(bare) == 0 || bareN != 1 {
			t.Fatalf("%s: fixture: %d casters, %d composites without creatures", c.name, len(bare), bareN)
		}
		if len(got) != len(bare) || gotN != bareN || litDraws != 0 {
			t.Fatalf("%s: with creatures %d casters, %d composites, %d lit draws; without %d casters, %d composites",
				c.name, len(got), gotN, litDraws, len(bare), bareN)
		}
		for i := range bare {
			if got[i] != bare[i] {
				t.Fatalf("%s: caster %d is %+v with creatures, %+v without", c.name, i, got[i], bare[i])
			}
		}
	}
}
