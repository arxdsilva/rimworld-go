package ebiten

import (
	"errors"
	"math"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/world/weather"
)

const (
	summerDay = 15
	winterDay = 45
)

var flatN = [3]float32{0, 0, 1}

func lumOf(c [3]float32) float64 { return float64(lum3(c)) }

func TestSunRisesEastSetsWest(t *testing.T) {
	t.Parallel()
	dawn := lightAt(6.5, summerDay, "clear", 0)
	dusk := lightAt(17.5, summerDay, "clear", 0)
	if dawn.SunL[0] <= 0 {
		t.Fatalf("06:30 sun should be in the east (L.x > 0), got %v", dawn.SunL)
	}
	if dusk.SunL[0] >= 0 {
		t.Fatalf("17:30 sun should be in the west (L.x < 0), got %v", dusk.SunL)
	}
	noon := lightAt(12, summerDay, "clear", 0)
	if noon.SunL[1] <= 0 || math.Abs(float64(noon.SunL[0])) > 1e-4 {
		t.Fatalf("noon sun should be due south (+y), got %v", noon.SunL)
	}
	for _, l := range [][3]float32{dawn.SunL, dusk.SunL, noon.SunL} {
		if n := math.Sqrt(float64(l[0]*l[0] + l[1]*l[1] + l[2]*l[2])); math.Abs(n-1) > 1e-4 {
			t.Fatalf("light vector not unit: %v (|L| = %v)", l, n)
		}
	}
}

func TestSunHighestAtNoonAndLowerInWinter(t *testing.T) {
	t.Parallel()
	z := func(h float64, day int) float32 { return lightAt(h, day, "clear", 0).SunL[2] }
	if !(z(12, summerDay) > z(9, summerDay) && z(12, summerDay) > z(15, summerDay)) {
		t.Fatalf("sun not highest at noon: 9h %v 12h %v 15h %v", z(9, summerDay), z(12, summerDay), z(15, summerDay))
	}
	if z(12, winterDay) >= z(12, summerDay) {
		t.Fatalf("winter noon sun %v should be lower than summer %v", z(12, winterDay), z(12, summerDay))
	}
	if seasonMaxEl(summerDay) != 65 || seasonMaxEl(winterDay) != 32 || seasonMaxEl(3) != 50 ||
		seasonMaxEl(55) != 50 || seasonMaxEl(30) != 50 || seasonMaxEl(8) != 65 || seasonMaxEl(38) != 32 {
		t.Fatal("season elevations off the plan's table")
	}
	w := litFactor(ptr(lightAt(12, winterDay, "clear", 0)), flatN, 1, 0)
	s := litFactor(ptr(lightAt(12, summerDay, "clear", 0)), flatN, 1, 0)
	if lumOf(w) >= lumOf(s) || lumOf(w) < 0.85 {
		t.Fatalf("winter noon %v should be a little dimmer than summer %v", w, s)
	}
}

func ptr(ls lightState) *lightState { return &ls }

func TestNoSunAtNightMoonUp(t *testing.T) {
	t.Parallel()
	for _, h := range []float64{0, 2, 4, 20, 23} {
		ls := lightAt(h, summerDay, "clear", 0)
		if ls.SunL != ([3]float32{}) {
			t.Fatalf("%vh: sun up at night: %v", h, ls.SunL)
		}
		if ls.MoonL[2] <= 0 {
			t.Fatalf("%vh: moon should be up: %v", h, ls.MoonL)
		}
	}
	day := lightAt(12, summerDay, "clear", 0)
	if day.MoonL != ([3]float32{}) {
		t.Fatalf("moon up at noon: %v", day.MoonL)
	}
	if m := lightAt(2, summerDay, "clear", 0); m.MoonL[0] >= 0 {
		t.Fatalf("02:00 moon should be setting in the west, got %v", m.MoonL)
	}
}

func TestNoonFlatNormalIsAboutOne(t *testing.T) {
	t.Parallel()
	f := litFactor(ptr(lightAt(12, summerDay, "clear", 0)), flatN, 1, 0)
	for i, v := range f {
		if v < 0.97 || v > 1.03 {
			t.Fatalf("summer noon flat channel %d = %.3f, want 0.97..1.03 (%v)", i, v, f)
		}
	}
}

func TestDawnIsWarm(t *testing.T) {
	t.Parallel()
	for _, h := range []float64{6.5, 7, 17.5} {
		ls := lightAt(h, summerDay, "clear", 0)
		if ls.SunColor[0] <= ls.SunColor[2] {
			t.Fatalf("%vh sun colour not warm: %v", h, ls.SunColor)
		}
		f := litFactor(&ls, flatN, 1, 0)
		if f[0] <= f[2] {
			t.Fatalf("%vh flat light not warm (R <= B): %v", h, f)
		}
		if lumOf(f) >= 0.9 {
			t.Fatalf("%vh should be dimmer than noon, got %v", h, f)
		}
	}
}

func TestNightIsCoolAndDim(t *testing.T) {
	t.Parallel()
	ls := lightAt(0, summerDay, "clear", 0)
	f := litFactor(&ls, flatN, 1, 0)
	if f[2] <= f[0] {
		t.Fatalf("midnight not cool (B <= R): %v", f)
	}
	if l := lumOf(f); l < 0.25 || l > 0.35 {
		t.Fatalf("midnight open sky brightness %.3f, want 0.25..0.35 (%v)", l, f)
	}
	lamp := litFactor(&ls, flatN, 0, 1)
	if l := lumOf(lamp); l < 0.7 || l > 0.9 {
		t.Fatalf("midnight lamp-lit room brightness %.3f, want 0.7..0.9 (%v)", l, lamp)
	}
	if lamp[0] <= lamp[2] {
		t.Fatalf("lamp light not warm: %v", lamp)
	}
}

func TestStormDimsSun(t *testing.T) {
	t.Parallel()
	clear := lightAt(12, summerDay, "clear", 0)
	for _, w := range []string{"cloudy", "rain", "storm", "snow", "blizzard"} {
		ls := lightAt(12, summerDay, w, 1)
		if ls.SunColor[0] >= clear.SunColor[0] {
			t.Fatalf("%s should dim the sun: %v vs clear %v", w, ls.SunColor, clear.SunColor)
		}
		if lumOf(litFactor(&ls, flatN, 1, 0)) >= lumOf(litFactor(&clear, flatN, 1, 0)) {
			t.Fatalf("%s noon should be darker than clear", w)
		}
		// ambient goes toward grey: the channel spread shrinks
		spread := func(a [3]float32) float32 { return max(a[0], a[1], a[2]) - min(a[0], a[1], a[2]) }
		if n := lightAt(0, summerDay, w, 1); spread(n.SkyAmbient) >= spread(lightAt(0, summerDay, "clear", 0).SkyAmbient) {
			t.Fatalf("%s night ambient should be greyer: %v", w, n.SkyAmbient)
		}
	}
	light, hard := lightAt(12, summerDay, "storm", 0.2), lightAt(12, summerDay, "storm", 1)
	if hard.SunColor[0] >= light.SunColor[0] {
		t.Fatalf("a harder storm should dim more: %v vs %v", hard.SunColor, light.SunColor)
	}
}

// TestRendererSunUsesDomainFactor: the renderer dims the sun by exactly
// weather.SunFactor, the factor solar panels make power by.
func TestRendererSunUsesDomainFactor(t *testing.T) {
	t.Parallel()
	clear := lightAt(12, summerDay, "clear", 0)
	for k := weather.Clear; k <= weather.Blizzard; k++ {
		for _, in := range []float64{0, 0.2, 0.6, 1} {
			want := float64(weather.SunFactor(k, in)) / 1000
			if got := weatherSun(k.String(), in); got != want {
				t.Fatalf("%s at %.1f: renderer sun %.4f, domain %.4f", k, in, got, want)
			}
			ls := lightAt(12, summerDay, k.String(), in)
			if d := float64(ls.SunColor[0]) - float64(clear.SunColor[0])*want; math.Abs(d) > 1e-5 {
				t.Fatalf("%s at %.1f: sun red %.5f, want clear %.5f x %.3f", k, in, ls.SunColor[0], clear.SunColor[0], want)
			}
		}
	}
}

// roomWorld is a 7×4 map: outdoors at x 0 and 6, a lit room (id 0)
// under a built roof at x 1..2, an inner wall at x 3, an unlit room (id
// 1) under natural roof at x 4 and an outer wall at x 5.
func roomWorld() game.WorldView {
	const w, h = 7, 4
	v := game.WorldView{TerrainW: w, Terrain: make([]uint8, w*h), RoomGrid: make([]int, w*h), RoofCells: make([]uint8, w*h),
		Rooms: []game.RoomView{{ID: 0, Lit: true}, {ID: 1}}}
	for i := range v.RoomGrid {
		v.RoomGrid[i] = -1
	}
	for y := 0; y < h; y++ {
		v.RoomGrid[y*w+1], v.RoomGrid[y*w+2] = 0, 0
		v.RoofCells[y*w+1], v.RoofCells[y*w+2] = 1, 1 // built
		v.RoomGrid[y*w+3] = -2                        // inner wall
		v.RoofCells[y*w+3] = 1
		v.RoomGrid[y*w+4] = 1
		v.RoofCells[y*w+4] = 2 // natural
		v.RoomGrid[y*w+5] = -2 // outer wall
		v.RoofCells[y*w+5] = 1
	}
	return v
}

func TestIndoorAmbientWhenRoofed(t *testing.T) {
	t.Parallel()
	r := &Renderer{}
	r.view.W = roomWorld()
	for _, c := range []struct {
		x, y int
		sky  float32
		what string
	}{{0, 0, 1, "outdoors"}, {1, 1, 0, "built roof"}, {3, 2, 0, "inner wall between roofed rooms"}, {4, 3, 0, "natural roof"},
		{5, 1, 1, "outer wall (its open face)"}, {6, 0, 1, "outdoors"}, {-1, 9, 1, "off map"}} {
		if sky, _ := r.lightTile(c.x, c.y); sky != c.sky {
			t.Fatalf("%s (%d,%d): sky %v, want %v", c.what, c.x, c.y, sky, c.sky)
		}
	}
	ls := lightAt(12, summerDay, "clear", 0)
	in, out := litFactor(&ls, flatN, 0, 0), litFactor(&ls, flatN, 1, 0)
	if in != ls.IndoorAmbient {
		t.Fatalf("roofed flat light %v should be the indoor ambient %v", in, ls.IndoorAmbient)
	}
	if lumOf(in) >= lumOf(out) {
		t.Fatalf("indoors %v should be darker than outdoors %v at noon", in, out)
	}
	// a new frame's view rebuilds the grid
	w := roomWorld()
	w.RoofCells = make([]uint8, len(w.RoofCells))
	r.view.W = w
	if sky, _ := r.lightTile(1, 1); sky != 1 {
		t.Fatalf("roof removed: sky %v, want 1", sky)
	}
}

func TestLampAddsWarmth(t *testing.T) {
	t.Parallel()
	r := &Renderer{}
	w := roomWorld()
	// PLAY-020 §3b: a room is no longer lit flat at 1 because the sim calls
	// it lit; its lamp lights it, full on the lamp's tile and its
	// neighbours, falling off with distance, and stops at its walls.
	w.Buildings = []game.BuildingView{{ID: 1, Kind: "lamp", Glow: true, GlowRadius: 6, X: 1, Y: 0, W: 1, H: 1},
		{Kind: "campfire", X: 6, Y: 0, Burning: true, Glow: true, GlowRadius: 5}, {Kind: "campfire", X: 0, Y: 3}}
	r.view.W = w
	if l := levelAt(r, 1, 0); l != 1 {
		t.Fatalf("on the lamp %v, want 1", l)
	}
	if l := levelAt(r, 1, 1); l != 1 {
		t.Fatalf("next to the lamp %v, want 1", l)
	}
	if l := levelAt(r, 2, 3); !(l > 0 && l < 1) {
		t.Fatalf("across the room from the lamp %v, want dimmer but lit", l)
	}
	if _, l := r.lightTile(4, 3); l != 0 {
		t.Fatalf("the room beyond the wall, out of the campfire's reach: lamp %v, want 0", l)
	}
	if _, l := r.lightTile(6, 0); l != 1 {
		t.Fatalf("on the campfire lamp %v, want 1", l)
	}
	if l, want := levelAt(r, 3, 1), max(levelAt(r, 2, 0), levelAt(r, 2, 1), levelAt(r, 2, 2)); math.Abs(float64(l-want)) > 2e-3 || l <= 0 {
		t.Fatalf("a lit room's wall glows with its brightest floor neighbour: %v, want %v", l, want)
	}
	if _, l := r.lightTile(6, 3); math.Abs(float64(l)-(1-3/3.5)) > 1e-5 {
		t.Fatalf("3 tiles from the campfire lamp %v, want %v", l, 1-3/3.5)
	}
	if _, l := r.lightTile(0, 2); l != 0 {
		t.Fatalf("a banked campfire lights nothing (and the room lamp not the outdoors), got %v", l)
	}
	ls := lightAt(22, summerDay, "clear", 0)
	dark, lit := litFactor(&ls, flatN, 0, 0), litFactor(&ls, flatN, 0, 1)
	if lumOf(lit) <= lumOf(dark)+0.3 {
		t.Fatalf("lamp should brighten a dark room: %v -> %v", dark, lit)
	}
	if lit[0]-dark[0] <= lit[2]-dark[2] {
		t.Fatalf("lamp light should be warm: %v -> %v", dark, lit)
	}
	noon := lightAt(12, summerDay, "clear", 0)
	if lumOf(litFactor(&noon, flatN, 0, 1)) >= lumOf(litFactor(&noon, flatN, 1, 0)) {
		t.Fatal("a lit room at noon should stay below the open sky")
	}
}

func TestLightGridNoAllocAfterWarmup(t *testing.T) {
	r := &Renderer{}
	views := [2]game.WorldView{roomWorld(), roomWorld()}
	for i := range views { // a room lamp and an outdoor one: the lamp paths too
		views[i].Buildings = []game.BuildingView{{ID: 1, Kind: "lamp", Glow: true, GlowRadius: 6, X: 1, Y: 1, W: 1, H: 1}, {ID: 2, Kind: "lamp", Glow: true, GlowRadius: 6, X: 6, Y: 2, W: 1, H: 1}}
	}
	r.view.W = views[0]
	r.lightTile(0, 0)
	i := 0
	allocs := testing.AllocsPerRun(50, func() {
		i++
		r.view.W = views[i%2] // a new frame
		r.lightTile(1, 1)
		r.lightTile(4, 3)
	})
	if allocs != 0 {
		t.Fatalf("light grid allocates %v per frame", allocs)
	}
}

func TestNeutralNormalFallback(t *testing.T) {
	t.Parallel()
	if n := decodeNormal([4]float32{0.9, 0.1, 0.2, 1}, false); n != flatN {
		t.Fatalf("no normal map should be flat, got %v", n)
	}
	if n := decodeNormal([4]float32{}, true); n != flatN {
		t.Fatalf("a transparent texel should be flat, got %v", n)
	}
	// (128, 128, 255) is the neutral normal, also premultiplied at half alpha
	for _, a := range []float32{1, 0.5} {
		n := decodeNormal([4]float32{128.0 / 255 * a, 128.0 / 255 * a, a, a}, true)
		if math.Abs(float64(n[2])-1) > 1e-3 || math.Abs(float64(n[0])) > 0.01 || math.Abs(float64(n[1])) > 0.01 {
			t.Fatalf("alpha %v: neutral texel decoded to %v", a, n)
		}
	}
	// green up in the map is screen up (-y)
	n := decodeNormal([4]float32{0.5, 1, 0.5, 1}, true)
	if n[1] >= -0.9 {
		t.Fatalf("green-up should decode to -y, got %v", n)
	}
	// a normal facing the dawn sun (east, +x) is brighter than flat; one
	// facing west gets no sun
	ls := lightAt(7, summerDay, "clear", 0)
	east, west := decodeNormal([4]float32{1, 0.5, 0.75, 1}, true), decodeNormal([4]float32{0, 0.5, 0.75, 1}, true)
	fe, ff, fw := litFactor(&ls, east, 1, 0), litFactor(&ls, flatN, 1, 0), litFactor(&ls, west, 1, 0)
	if !(fe[0] > ff[0] && ff[0] > fw[0]) {
		t.Fatalf("east %v flat %v west %v at 07:00", fe, ff, fw)
	}
	if fw != ls.SkyAmbient {
		t.Fatalf("a west-facing normal at dawn gets only ambient: %v vs %v", fw, ls.SkyAmbient)
	}
}

func TestShaderCompiles(t *testing.T) {
	t.Parallel()
	s, err := ebiten.NewShader(litKage)
	if err != nil {
		t.Fatalf("lit.kage: %v", err)
	}
	s.Deallocate()
}

func TestCustomAttributesPerVertex(t *testing.T) {
	t.Parallel()
	var vs [4]ebiten.Vertex
	geo := ebiten.GeoM{}
	geo.Scale(2, 2)
	geo.Translate(10, 20)
	litQuad(&vs, 3, 4, 8, 6, geo, colorScale{1, 0.5, 0.25, 0.5}, 0, 0.75, 9)
	wantDst := [4][2]float32{{10, 20}, {26, 20}, {10, 32}, {26, 32}}
	wantSrc := [4][2]float32{{3, 4}, {11, 4}, {3, 10}, {11, 10}}
	for i, v := range vs {
		if [2]float32{v.DstX, v.DstY} != wantDst[i] || [2]float32{v.SrcX, v.SrcY} != wantSrc[i] {
			t.Fatalf("vertex %d at dst (%v,%v) src (%v,%v)", i, v.DstX, v.DstY, v.SrcX, v.SrcY)
		}
		if v.ColorR != 0.5 || v.ColorG != 0.25 || v.ColorB != 0.125 || v.ColorA != 0.5 {
			t.Fatalf("vertex %d colour not premultiplied: %+v", i, v)
		}
		if v.Custom0 != 0 || v.Custom1 != 0.75 || v.Custom2 != 9 {
			t.Fatalf("vertex %d custom (sky, lamp, normal) = (%v, %v, %v)", i, v.Custom0, v.Custom1, v.Custom2)
		}
	}
	for _, i := range litIndices {
		if int(i) >= len(vs) {
			t.Fatalf("index %d out of range", i)
		}
	}
}

func TestDrawLitFlatAndNormalPaths(t *testing.T) {
	sprite := ebiten.NewImage(8, 6)
	normal := ebiten.NewImage(8, 6)
	plain := ebiten.NewImage(4, 4)
	r := &Renderer{art: &artCatalog{Normals: map[*ebiten.Image]*ebiten.Image{sprite: normal}}}
	// PLAY-020 §3b: lamp light comes from a lamp, not from the room's Lit
	// flag, so the lit room gets its lamp at (1, 1) (its tile: lamp 1).
	w := roomWorld()
	w.Buildings = []game.BuildingView{{ID: 1, Kind: "lamp", Glow: true, GlowRadius: 6, X: 1, Y: 1, W: 1, H: 1}}
	r.view.W = w
	r.lit.light = lightAt(12, summerDay, "clear", 0)
	dst := ebiten.NewImage(64, 64)

	r.drawLit(dst, plain, ebiten.GeoM{}, noCS, 1, 1)
	if r.lit.err != nil {
		t.Fatal(r.lit.err)
	}
	if v := r.lit.verts[0]; v.Custom2 != 0 || v.Custom0 != 0 || v.Custom1 != 1 {
		t.Fatalf("flat sprite in a lit roofed room: custom (%v, %v, %v)", v.Custom0, v.Custom1, v.Custom2)
	}
	if r.lit.opts.Images[0] != plain {
		t.Fatal("a sprite without a normal map draws from itself")
	}
	if got := r.lit.uniforms[4]; got[0] != r.lit.light.SkyAmbient[0] {
		t.Fatalf("uniform SkyAmbient %v, want %v", got, r.lit.light.SkyAmbient)
	}

	r.drawLit(dst, sprite, ebiten.GeoM{}, colorScale{}, 0, 0)
	pair := r.lit.opts.Images[0]
	if pair == sprite || pair.Bounds().Dx() != 2*8+litGap || pair.Bounds().Dy() != 6 {
		t.Fatalf("normal-mapped sprite should draw from its pair image, got %v", pair.Bounds())
	}
	if v := r.lit.verts[3]; v.Custom2 != 8+litGap || v.SrcX != 8 || v.SrcY != 6 || v.Custom0 != 1 || v.ColorA != 1 {
		t.Fatalf("normal path vertex %+v", v)
	}
	r.drawLit(dst, sprite, ebiten.GeoM{}, noCS, 0, 0)
	if r.lit.opts.Images[0] != pair {
		t.Fatal("pair image should be built once")
	}

	// drawLit adds no allocation of its own over Ebiten's draw call
	sh := r.litShader()
	pair2, _, _ := r.litSource(sprite)
	ebitenOnly := testing.AllocsPerRun(500, func() {
		r.lit.opts.Images[0] = pair2
		dst.DrawTrianglesShader32(r.lit.verts[:], litIndices[:], sh, &r.lit.opts)
		r.lit.opts.Images[0] = plain
		dst.DrawTrianglesShader32(r.lit.verts[:], litIndices[:], sh, &r.lit.opts)
	})
	allocs := testing.AllocsPerRun(500, func() {
		r.drawLit(dst, sprite, ebiten.GeoM{}, noCS, 0, 0)
		r.drawLit(dst, plain, ebiten.GeoM{}, noCS, 2, 1)
	})
	// +1: Ebiten's own sync.Pool (invokeUsageCallbacks) jitters under GC and -race
	if allocs > ebitenOnly+1 {
		t.Fatalf("drawLit allocates %v per call pair, Ebiten alone %v", allocs, ebitenOnly)
	}
}

func TestGroundPlanesLitFlat(t *testing.T) {
	soil, floor, chair, tinted := ebiten.NewImage(8, 8), ebiten.NewImage(8, 8), ebiten.NewImage(8, 8), ebiten.NewImage(8, 8)
	n := ebiten.NewImage(8, 8)
	art := &artCatalog{
		Sprites: map[string]*artSprite{"terrain/soil_00": {Img: soil}, "structure/floor_wood": {}, "furniture/chair": {Img: chair}},
		Tinted:  map[tintKey]*ebiten.Image{{Key: "structure/floor_wood"}: floor, {Key: "furniture/bed"}: tinted},
		Normals: map[*ebiten.Image]*ebiten.Image{soil: n, floor: n, chair: n, tinted: n},
	}
	r := &Renderer{art: art}
	for _, c := range []struct {
		img  *ebiten.Image
		flat bool
		what string
	}{{soil, true, "terrain"}, {floor, true, "tinted floor"}, {chair, false, "furniture"}, {tinted, false, "tinted furniture"}} {
		for i := 0; i < 2; i++ { // built, then cached
			src, off, _ := r.litSource(c.img)
			if (off == 0) != c.flat || (src == c.img) != c.flat {
				t.Fatalf("%s (pass %d): offset %v, own image %v; want flat %v", c.what, i, off, src == c.img, c.flat)
			}
		}
	}
	if (*artCatalog)(nil).groundPlane(soil) || art.groundPlane(n) {
		t.Fatal("unknown images are not ground planes")
	}
}

func TestShadowFields(t *testing.T) {
	t.Parallel()
	near := func(a, b float64) bool { return math.Abs(a-b) < 1e-6 }
	unit := func(d [2]float64) bool { return near(math.Hypot(d[0], d[1]), 1) }

	noon := lightAt(12, summerDay, "clear", 0)
	if !unit(noon.ShadowDir) || !near(noon.ShadowDir[0], 0) || noon.ShadowDir[1] >= 0 {
		t.Fatalf("noon shadows fall north (-y): %v", noon.ShadowDir)
	}
	// physical strength: the sun's share of a flat ground's light
	s := float64(lum3(noon.SunColor) * noon.SunL[2])
	a := float64(lum3(noon.SkyAmbient))
	if want := litShadowGain * s / (a + s); !near(noon.ShadowAlpha, want) || noon.ShadowAlpha < 0.4 {
		t.Fatalf("summer noon alpha %v, want the sun's share %v (>= 0.4)", noon.ShadowAlpha, want)
	}
	if l := noon.ShadowLen(1); !near(l, 1/math.Tan(65*math.Pi/180)) {
		t.Fatalf("noon shadow of h 1: %v", l)
	}
	if l := noon.ShadowLen(0.4); !near(l, 0.4/math.Tan(65*math.Pi/180)) {
		t.Fatalf("noon shadow of h 0.4: %v", l)
	}

	dawn := lightAt(6.5, summerDay, "clear", 0)
	if !unit(dawn.ShadowDir) || dawn.ShadowDir[0] >= 0 {
		t.Fatalf("dawn shadows fall west: %v", dawn.ShadowDir)
	}
	if l := dawn.ShadowLen(1); !near(l, 3) {
		t.Fatalf("dawn shadow is long (clamped to 3), got %v", l)
	}
	if want := float64(litShadowFloor); !near(dawn.ShadowAlpha, want) {
		t.Fatalf("06:30 alpha %v, want the floor %v", dawn.ShadowAlpha, want)
	}
	// legible (>= the floor) from 30 min after sunrise to 30 min before
	// sunset, in every season, strongest at noon
	for _, day := range []int{summerDay, winterDay, 3, 30} {
		prev := 0.0
		for h := 6.5; h <= 12; h += 0.25 {
			a := lightAt(h, day, "clear", 0).ShadowAlpha
			b := lightAt(24-h, day, "clear", 0).ShadowAlpha // mirror: 17.5 .. 12
			// morning and evening sun colours differ slightly: mirror within 3%
			if a < litShadowFloor-1e-6 || math.Abs(a-b) > 0.03*math.Max(a, b) || a < prev-1e-9 {
				t.Fatalf("day %d %vh: alpha %v (mirror %v, before %v)", day, h, a, b, prev)
			}
			prev = a
		}
	}
	// fading in over the first half hour, out over the last
	in1, in2 := lightAt(6.1, summerDay, "clear", 0).ShadowAlpha, lightAt(6.3, summerDay, "clear", 0).ShadowAlpha
	if !(in1 < in2 && in2 < litShadowFloor && in1 >= 0) {
		t.Fatalf("dawn fade-in: 06:06 %v, 06:18 %v", in1, in2)
	}
	if out := lightAt(17.8, summerDay, "clear", 0).ShadowAlpha; out >= litShadowFloor || out <= 0 {
		t.Fatalf("dusk fade-out at 17:48: %v", out)
	}

	dusk := lightAt(17.5, summerDay, "clear", 0)
	if !unit(dusk.ShadowDir) || dusk.ShadowDir[0] <= 0 {
		t.Fatalf("dusk shadows fall east: %v", dusk.ShadowDir)
	}

	mid := lightAt(0, summerDay, "clear", 0)
	if mid.ShadowAlpha < litMoonFloor-1e-6 || mid.ShadowAlpha > litMoonCap+1e-6 || !unit(mid.ShadowDir) || mid.ShadowDir[1] >= 0 {
		t.Fatalf("midnight moon shadow: alpha %v dir %v", mid.ShadowAlpha, mid.ShadowDir)
	}
	if l := mid.ShadowLen(1); !near(l, 1/math.Tan(40*math.Pi/180)) {
		t.Fatalf("moon shadow of h 1: %v", l)
	}
	if l := mid.ShadowLen(0.01); !near(l, 0.01/math.Tan(40*math.Pi/180)) {
		t.Fatalf("short caster: %v", l)
	}
	// the 0.25·h floor: a tall caster under a high summer sun at noon
	if l := lightAt(12, summerDay, "clear", 0).ShadowLen(2); l < 0.5 {
		t.Fatalf("shadow below 0.25·h: %v", l)
	}

	for _, h := range []float64{6, 18, 6.02, 17.98} {
		if ls := lightAt(h, summerDay, "clear", 0); ls.ShadowAlpha != 0 || ls.ShadowLen(1) != 0 {
			t.Fatalf("%vh: no light up enough, alpha %v", h, ls.ShadowAlpha)
		}
	}

	winter := lightAt(12, winterDay, "clear", 0)
	// a low winter sun lights the ground less: fainter but longer shadows
	if winter.ShadowAlpha >= noon.ShadowAlpha || winter.ShadowAlpha < litShadowFloor-1e-6 || winter.ShadowLen(1) <= noon.ShadowLen(1) {
		t.Fatalf("winter noon: alpha %v (summer %v), length %v longer than summer", winter.ShadowAlpha, noon.ShadowAlpha, winter.ShadowLen(1))
	}

	// clouds scatter the light: storms fade shadows most, rain less
	storm := lightAt(12, summerDay, "storm", 1)
	rain := lightAt(12, summerDay, "rain", 0.5)
	if !(storm.ShadowAlpha < rain.ShadowAlpha && rain.ShadowAlpha < noon.ShadowAlpha) {
		t.Fatalf("storm %v < rain %v < clear %v", storm.ShadowAlpha, rain.ShadowAlpha, noon.ShadowAlpha)
	}
	if bl := lightAt(0, summerDay, "blizzard", 1); bl.ShadowAlpha >= mid.ShadowAlpha {
		t.Fatalf("blizzard moon alpha %v, clear %v", bl.ShadowAlpha, mid.ShadowAlpha)
	}
}

func TestDrawLitFallsBackWhenShaderFails(t *testing.T) {
	r := &Renderer{}
	r.lit.err = errors.New("no shader")
	dst, img := ebiten.NewImage(16, 16), ebiten.NewImage(4, 4)
	r.drawLit(dst, img, ebiten.GeoM{}, colorScale{1, 0.5, 0.5, 0.5}, 0, 0)
	r.drawLit(dst, img, ebiten.GeoM{}, noCS, 0, 0)
	if r.lit.shader != nil || r.lit.opts.Images[0] != nil {
		t.Fatal("a failed shader should take the unlit DrawImage path")
	}
}

// BenchmarkDrawArtLitAB is BenchmarkDrawArt's frame with the lit draw
// (lit) and with the unlit DrawImage fallback (unlit), side by side, so
// the lighting cost reads apart from machine load and other changes.
func BenchmarkDrawArtLitAB(b *testing.B) {
	root, err := LocateArt()
	if err != nil {
		b.Skip("generated art not present:", err)
	}
	art, err := loadArtCatalog(root)
	if err != nil {
		b.Fatal(err)
	}
	g := game.NewColony(game.Config{Seed: 7, Width: 80, Height: 50})
	g.Step(600)
	for _, mode := range []string{"unlit", "lit"} {
		b.Run(mode, func(b *testing.B) {
			r := New(g)
			r.art = art
			r.cam.Zoom = 0.6
			if mode == "unlit" {
				r.lit.err = errors.New("benchmark: unlit")
			}
			screen := ebiten.NewImage(1280, 800)
			tile := r.tilePx()
			frame := func() {
				r.present++
				r.advance()
				r.setView(1280, 800, tile)
				r.drawArt(screen, 1280, 800, tile)
			}
			for i := 0; i < 5; i++ {
				frame()
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				frame()
			}
		})
	}
}

// TestLitRoomWallCornersGlow: a lit room's wall corners touch its floor
// only diagonally; they glow with the rest of its walls. PLAY-020 §3b: the
// room is lit by a lamp (at (1, 1)) with a falloff, no longer flat 1, so
// each corner glows with its diagonal floor tile's light.
func TestLitRoomWallCornersGlow(t *testing.T) {
	t.Parallel()
	const w, h = 4, 4 // a wall ring around a 2x2 lit room
	v := game.WorldView{TerrainW: w, Terrain: make([]uint8, w*h), RoomGrid: make([]int, w*h), RoofCells: make([]uint8, w*h),
		Rooms: []game.RoomView{{ID: 0, Lit: true}}, Buildings: []game.BuildingView{{ID: 1, Kind: "lamp", Glow: true, GlowRadius: 6, X: 1, Y: 1, W: 1, H: 1}}}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if x == 0 || y == 0 || x == w-1 || y == h-1 {
				v.RoomGrid[y*w+x] = -2
			} else {
				v.RoofCells[y*w+x] = 1
			}
		}
	}
	r := &Renderer{}
	r.view.W = v
	for _, c := range [][4]int{{0, 0, 1, 1}, {3, 0, 2, 1}, {0, 3, 1, 2}, {3, 3, 2, 2}} {
		if l, want := levelAt(r, c[0], c[1]), levelAt(r, c[2], c[3]); math.Abs(float64(l-want)) > 2e-3 || l < 0.9 {
			t.Fatalf("corner %v lamp %v, want its diagonal floor's %v", c[:2], l, want)
		}
	}
	if s, _ := r.lightTile(0, 0); s != 1 {
		t.Fatalf("a corner on the map edge keeps open sky, got %v", s)
	}
}
