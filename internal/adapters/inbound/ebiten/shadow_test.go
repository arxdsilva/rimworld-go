package ebiten

import (
	"math"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

// testLight is a hand-built light: shadows fall away from a light at
// azimuth az (screen space, 0 = east) and elevation el (radians), with
// the ART §6.1 length clamp and the given alpha.
func testLight(az, el, alpha float64) lightState {
	lx, ly := math.Cos(az)*math.Cos(el), 0.5*math.Sin(az)*math.Cos(el)
	n := math.Hypot(lx, ly)
	return lightState{
		SunL:      [3]float32{float32(lx), float32(ly), float32(math.Sin(el))},
		ShadowDir: [2]float64{-lx / n, -ly / n},
		ShadowLen: func(h float64) float64 {
			return math.Min(math.Max(h/math.Tan(el), 0.25*h), shadowMaxLen)
		},
		ShadowAlpha: alpha,
	}
}

func quadCentre(q [4][2]float64) (float64, float64) {
	return (q[0][0] + q[1][0] + q[2][0] + q[3][0]) / 4, (q[0][1] + q[1][1] + q[2][1] + q[3][1]) / 4
}

// quadExtent is the quad's length along its texture-x axis (the shadow).
func quadExtent(q [4][2]float64) float64 {
	return math.Hypot(q[1][0]-q[0][0], q[1][1]-q[0][1])
}

func hullCentroid(pts [8][2]float64, n int) (float64, float64) {
	var x, y float64
	for i := 0; i < n; i++ {
		x += pts[i][0]
		y += pts[i][1]
	}
	return x / float64(n), y / float64(n)
}

var pawnBlob = shadowSpec{kind: shadowBlob, w: 0.55, h: 1.7, d: 0.35}
var wallBox = shadowSpec{kind: shadowBox, w: 1, h: 1, d: 1}

func TestShadowPointsAwayFromSun(t *testing.T) {
	t.Parallel()
	morning := testLight(0.3, 0.4, 0.3)         // sun in the east, a little south
	evening := testLight(math.Pi-0.3, 0.4, 0.3) // sun in the west
	cx, _ := quadCentre(blobQuad(5, 5, pawnBlob, morning))
	ex, _ := quadCentre(blobQuad(5, 5, pawnBlob, evening))
	if !(cx < 5 && ex > 5) {
		t.Fatalf("blob centres: morning %.2f (want west of 5), evening %.2f (want east)", cx, ex)
	}
	_, my := quadCentre(blobQuad(5, 5, pawnBlob, morning))
	if my >= 5 {
		t.Fatalf("a southern sun casts north (up the screen); centre y %.2f", my)
	}
	pts, n := boxHull(5, 5, 1, 1, wallBox, morning)
	hx, _ := hullCentroid(pts, n)
	if !(hx < 5.5) {
		t.Fatalf("box shadow centroid x %.2f, want west of the footprint centre", hx)
	}
	// the footprint stays inside the shadow
	for _, c := range [][2]float64{{5, 5}, {6, 5}, {5, 6}, {6, 6}} {
		if !inConvex(pts, n, c[0], c[1]) {
			t.Fatalf("footprint corner %v outside the hull %v", c, pts[:n])
		}
	}
}

// inConvex reports p inside (or on) the counter-clockwise hull.
func inConvex(pts [8][2]float64, n int, x, y float64) bool {
	for i := 0; i < n; i++ {
		a, b := pts[i], pts[(i+1)%n]
		if (b[0]-a[0])*(y-a[1])-(b[1]-a[1])*(x-a[0]) < -1e-9 {
			return false
		}
	}
	return true
}

func TestShadowLongAtDawnShortAtNoon(t *testing.T) {
	t.Parallel()
	dawn := testLight(0.1, 0.12, 0.1)
	noon := testLight(math.Pi/2, 65*math.Pi/180, 0.32)
	ld, ln := quadExtent(blobQuad(0, 0, pawnBlob, dawn)), quadExtent(blobQuad(0, 0, pawnBlob, noon))
	if !(ld > 2*ln) {
		t.Fatalf("blob length dawn %.2f vs noon %.2f", ld, ln)
	}
	if ld > shadowMaxLen+1.2 {
		t.Fatalf("dawn shadow %.2f exceeds the clamp", ld)
	}
	area := func(l lightState) float64 {
		pts, n := boxHull(0, 0, 1, 1, wallBox, l)
		a := 0.0
		for i := 0; i < n; i++ {
			p, q := pts[i], pts[(i+1)%n]
			a += p[0]*q[1] - q[0]*p[1]
		}
		return a / 2
	}
	if ad, an := area(dawn), area(noon); !(ad > an && an > 1) {
		t.Fatalf("box shadow area dawn %.2f noon %.2f", ad, an)
	}
}

func TestMoonShadowsFaint(t *testing.T) {
	t.Parallel()
	day := shadowCompositeScale(testLight(1, 1, 0.32))
	moon := shadowCompositeScale(testLight(1, 0.5, 0.12))
	if !(moon[3] > 0 && moon[3] < day[3]/2) {
		t.Fatalf("moon alpha %.3f vs day %.3f", moon[3], day[3])
	}
	// the composite is the dark cool shadow colour, premultiplied
	if !(day[2] > day[0] && math.Abs(float64(day[0])-18.0/255*0.32) < 1e-4) {
		t.Fatalf("composite colour %v", day)
	}
}

func TestStormFadesShadows(t *testing.T) {
	t.Parallel()
	clear := shadowCompositeScale(testLight(1, 1, 0.32))
	storm := shadowCompositeScale(testLight(1, 1, 0.32*(1-0.6)))
	if !(storm[3] < clear[3]) || math.Abs(float64(storm[3])-0.128) > 1e-4 {
		t.Fatalf("storm alpha %.3f clear %.3f", storm[3], clear[3])
	}
	// no shadows at all: the pass draws nothing
	r := shadowRenderer(t, shadowWorld(nil))
	r.lit.light = testLight(1, 1, 0)
	r.drawShadows(ebiten.NewImage(64, 64), shadowWorld(nil), 64, 64, r.tilePx())
	if r.env.shadow.composites != 0 {
		t.Fatal("composited a shadow layer with zero shadow alpha")
	}
}

func TestBoxVsBlobByMetadata(t *testing.T) {
	t.Parallel()
	meta := func(kind string, fw, fd, ch int) *artMeta {
		m := &artMeta{Footprint: [2]int{fw, fd}, Canvas: [2]int{160, ch}}
		m.Shadow.Kind = kind
		if kind != "" {
			m.Shadow.W, m.Shadow.H, m.Shadow.D = 0.8, 2, 0.6
		}
		return m
	}
	cases := []struct {
		key  string
		m    *artMeta
		want shadowKind
		h    float64
	}{
		{"structure/wall_wood_05", meta("box", 1, 1, 224), shadowBox, 2},
		{"plant/tree_oak_00", meta("blob", 1, 1, 400), shadowBlob, 2},
		{"item/wood_pile", meta("none", 1, 1, 200), shadowNone, 0},
		// fallbacks by family while the art has no shadow block
		{"structure/wall_wood_05", meta("", 1, 1, 224), shadowBox, 1},
		{"structure/floor_wood", meta("", 1, 1, 128), shadowNone, 0},
		{"furniture/table_2x1", meta("", 2, 1, 198), shadowBox, 1},
		{"rock/rock_wall_00", nil, shadowBox, 1},
		{"plant/tree_oak_00", meta("", 1, 1, 512), shadowBlob, 512.0 / artTilePx * 0.8},
		{"pawn/body_south", meta("", 1, 1, 250), shadowBlob, 1.7},
		{"animal/dustbuck_east", meta("", 1, 1, 256), shadowBlob, 256.0 / artTilePx * 0.7},
		{"item/wood_pile", meta("", 1, 1, 128), shadowBlob, 0.7},
		{"overlay/stockpile", meta("", 1, 1, 128), shadowNone, 0},
		{"weather/raindrop", nil, shadowNone, 0},
		{"terrain/soil_00", nil, shadowNone, 0},
	}
	for _, c := range cases {
		s := shadowSpecFor(c.key, c.m)
		if s.kind != c.want || math.Abs(s.h-c.h) > 1e-9 {
			t.Errorf("%s (kind %q): got %+v, want kind %d h %.3f", c.key, kindOf(c.m), s, c.want, c.h)
		}
	}
	if s := shadowSpecFor("furniture/table_2x1", meta("", 2, 1, 198)); s.w != 2 || s.d != 1 {
		t.Errorf("box fallback footprint %+v, want 2x1", s)
	}
	if s := shadowSpecFor("plant/bush_00", meta("", 1, 1, 200)); math.Abs(s.w-0.7) > 1e-9 || s.d != 0.4 {
		t.Errorf("plant fallback %+v", s)
	}
}

func kindOf(m *artMeta) string {
	if m == nil {
		return "<nil>"
	}
	return m.Shadow.Kind
}

// --- casters, roofs and the layer ------------------------------------------------

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
	r.env.view = envRect{0, 0, 20, 12}
	return r
}

func TestNoShadowsUnderRoof(t *testing.T) {
	t.Parallel()
	w := shadowWorld(map[int]uint8{3*20 + 4: roofBuilt, 3*20 + 6: roofNatural, 3*20 + 8: roofPlanBuild})
	w.Items = []game.ItemView{{Kind: "wood", X: 4, Y: 3}, {Kind: "wood", X: 6, Y: 3}, {Kind: "wood", X: 8, Y: 3}, {Kind: "wood", X: 10, Y: 3}}
	w.Buildings = []game.BuildingView{{Kind: "wall", X: 4, Y: 3, W: 1, H: 1}, {Kind: "wall", X: 11, Y: 3, W: 1, H: 1},
		{Kind: "floor", Floor: true, X: 12, Y: 3, W: 1, H: 1}}
	pawns := []game.PawnView{{ID: 1, X: 4, Y: 3}, {ID: 2, X: 9, Y: 3}, {ID: 3, X: 13, Y: 3, Embodied: true}}
	r := shadowRenderer(t, w)
	got := map[[2]int]int{}
	for _, c := range r.shadowCasters(w, pawns) {
		got[[2]int{int(math.Floor(c.x)), int(math.Floor(c.y))}]++
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
	// pawns cast silhouettes instead (with art), roofed or not; the
	// embodied one is a corpse item now
	r.art.sprites["pawn/body_south"] = fakeSprite([2]int{160, 297}, pt{16, 153}, nil)
	sky := map[int]bool{}
	for _, c := range r.creatureCasters(w, pawns) {
		sky[int(c.pawn.ID)] = c.sky
	}
	if len(sky) != 2 || sky[1] || !sky[2] {
		t.Fatalf("creature casters (id: open sky) %v", sky)
	}
	// off-screen casters (beyond the longest shadow) are skipped
	r.env.view = envRect{0, 0, 2, 2}
	far := game.WorldView{TerrainW: w.TerrainW, Items: []game.ItemView{{Kind: "wood", X: 1, Y: 1}, {Kind: "wood", X: 19, Y: 11}}}
	if n := len(r.shadowCasters(far, nil)); n != 1 {
		t.Fatalf("got %d casters, want only the one in view", n)
	}
}

func TestShadowsClippedAtRoofedTiles(t *testing.T) {
	t.Parallel()
	w := shadowWorld(map[int]uint8{
		2*20 + 3: roofBuilt, 2*20 + 4: roofBuilt, 2*20 + 5: roofNatural, // one run of three
		2*20 + 7: roofBuilt,     // a run of one
		5*20 + 0: roofPlanBuild, // planned only: not clipped
	})
	runs := roofRuns(w, 0, 0, 19, 11, nil)
	want := [][3]int{{2, 3, 6}, {2, 7, 8}}
	if len(runs) != len(want) {
		t.Fatalf("runs %v, want %v", runs, want)
	}
	for i := range want {
		if runs[i] != want[i] {
			t.Fatalf("runs %v, want %v", runs, want)
		}
	}
	if n := len(roofRuns(shadowWorld(nil), 0, 0, 19, 11, nil)); n != 0 {
		t.Fatalf("no roof data clipped %d runs", n)
	}
	// a full frame: the shadow is drawn and the roofed run cleared
	r := shadowRenderer(t, w)
	r.lit.light = testLight(0.4, 0.4, 0.3)
	w.Buildings = []game.BuildingView{{Kind: "wall", X: 2, Y: 2, W: 1, H: 1}}
	w.Plants = []game.PlantView{{Species: "tree", X: 9, Y: 2, Growth: 60}}
	w.Items = []game.ItemView{{Kind: "wood", X: 10, Y: 3}}
	w.Animals = []game.AnimalView{{ID: 7, Species: "dustbuck", X: 12, Y: 4}}
	r.drawPawns = []game.PawnView{{ID: 1, X: 11, Y: 3}}
	if n := len(r.shadowCasters(w, r.drawPawns)); n != 3 {
		t.Fatalf("got %d box and blob casters, want 3", n)
	}
	r.drawShadows(ebiten.NewImage(640, 400), w, 640, 400, r.tilePx())
	if r.env.shadow.composites != 1 {
		t.Fatalf("composites %d, want 1", r.env.shadow.composites)
	}
}

func TestWallShadowsMergeSingleAlpha(t *testing.T) {
	t.Parallel()
	// the layer merges by max, so overlapping shadows keep one alpha
	if shadowBlend.BlendOperationAlpha != ebiten.BlendOperationMax || shadowBlend.BlendOperationRGB != ebiten.BlendOperationMax {
		t.Fatalf("shadow layer blend %+v is not max", shadowBlend)
	}
	w := shadowWorld(nil)
	for x := 2; x < 12; x++ {
		w.Buildings = append(w.Buildings, game.BuildingView{Kind: "wall", X: x, Y: 4, W: 1, H: 1})
	}
	r := shadowRenderer(t, w)
	r.lit.light = testLight(0.3, 0.3, 0.32)
	casters := r.shadowCasters(w, nil)
	if len(casters) != 10 {
		t.Fatalf("got %d wall casters, want one box per wall tile", len(casters))
	}
	var b triBatch
	for _, c := range casters {
		if c.spec.kind != shadowBox {
			t.Fatalf("wall caster %+v is not a box", c)
		}
		r.addBox(&b, c, r.lit.light, r.tilePx(), 0.32)
	}
	// every vertex is at the shadow alpha (core) or clear (feather edge):
	// overlapping walls max to exactly the core alpha, never more; boxes
	// carry no height (red 0) for the penumbra blur
	for _, v := range b.verts {
		if v.ColorA != 0 && v.ColorA != 0.32 || v.ColorR != 0 {
			t.Fatalf("vertex alpha %.3f red %.3f", v.ColorA, v.ColorR)
		}
	}
	// the whole layer is composited once, at the light's shadow alpha
	r.drawShadows(ebiten.NewImage(640, 400), w, 640, 400, r.tilePx())
	if r.env.shadow.composites != 1 {
		t.Fatalf("composited %d times", r.env.shadow.composites)
	}
	if cs := shadowCompositeScale(r.lit.light); math.Abs(float64(cs[3])-0.32) > 1e-6 {
		t.Fatalf("composite alpha %.3f", cs[3])
	}
	// and the layer is reused across frames, reallocated only on resize
	layer := r.env.shadow.layer
	r.drawShadows(ebiten.NewImage(640, 400), w, 640, 400, r.tilePx())
	if r.env.shadow.layer != layer {
		t.Fatal("layer reallocated without a resize")
	}
	r.drawShadows(ebiten.NewImage(320, 200), w, 320, 200, r.tilePx())
	if r.env.shadow.layer.Bounds().Dx() != 320 {
		t.Fatal("layer not resized with the screen")
	}
}

func TestBoxSkirtPointsOutward(t *testing.T) {
	t.Parallel()
	for _, az := range []float64{0.2, 1.4, 2.0, 2.9, 3.5, 4.8} {
		l := testLight(az, 0.35, 0.3)
		pts, n := boxHull(0, 0, 1, 1, wallBox, l)
		if n < 4 || n > 6 {
			t.Fatalf("az %.1f: hull of %d points", az, n)
		}
		cx, cy := hullCentroid(pts, n)
		for i := 0; i < n; i++ {
			p, q := pts[i], pts[(i+1)%n]
			dx, dy := q[0]-p[0], q[1]-p[1]
			nx, ny := dy, -dx
			mx, my := (p[0]+q[0])/2-cx, (p[1]+q[1])/2-cy
			if nx*mx+ny*my <= 0 {
				t.Fatalf("az %.1f: edge %d normal points inward", az, i)
			}
		}
	}
}
