package ebiten

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/globemath"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/planet"
	"rimworld-go/internal/domain/world/site"
)

var (
	globePlanetMu sync.Mutex
	globePlanets  = map[int]*planet.Planet{}
)

// globeTestPlanet is the seed "amber-hollow" world at the coverage, built
// once per test binary.
func globeTestPlanet(tb testing.TB, coverage int) *planet.Planet {
	tb.Helper()
	globePlanetMu.Lock()
	defer globePlanetMu.Unlock()
	if p, ok := globePlanets[coverage]; ok {
		return p
	}
	p := planet.Build(planet.Params{SeedText: "amber-hollow", Coverage: coverage, Rain: 2, Temp: 2, Pop: 1,
		Factions: faction.DefaultSlots()})
	globePlanets[coverage] = p
	return p
}

// testGlobe is a fresh view on the 30 % world at 1280x720.
func testGlobe(t testing.TB) *globeView {
	t.Helper()
	g := newGlobeView(globeTestPlanet(t, 30), nil)
	g.Sw, g.Sh = 1280, 720
	g.Reduced = true
	return g
}

// screenOf is where tile i's centre is on screen and whether it faces the eye.
func screenOf(g *globeView, i int) (x, y float64, front bool) {
	c := g.P.Center[i]
	sx, sy, z := g.Cam.Project(g.Viewport(), globemath.Vec3{float64(c[0]), float64(c[1]), float64(c[2])})
	return sx, sy, z > globemath.HorizonZ(g.Cam.Dist)+0.05
}

func centreNow(g *globeView, i int) {
	c := g.P.Center[i]
	g.Cam.PanX, g.Cam.PanY = 0, 0
	g.Cam.CentreOn(globemath.Vec3{float64(c[0]), float64(c[1]), float64(c[2])}, 0)
}

func landTile(t testing.TB, g *globeView) int {
	t.Helper()
	for i := 0; i < g.P.TileCount(); i++ {
		if ok, _ := g.P.ValidLanding(i); ok {
			return i
		}
	}
	t.Fatal("no landable tile")
	return -1
}

// globeClick feeds a press frame then a release frame at (x, y).
func globeClick(g *globeView, x, y int, blocked bool) (clicked int) {
	in := globeInput{W: 1280, H: 720, MX: x, MY: y, Blocked: blocked}
	in.Press[mbLeft], in.Down[mbLeft] = true, true
	g.Update(in, true)
	in.Press[mbLeft], in.Down[mbLeft], in.Release[mbLeft] = false, false, true
	c, _ := g.Update(in, true)
	return c
}

func TestGlobePaletteMatchesYAML(t *testing.T) {
	t.Parallel()
	path, err := locateUIPaletteYAML()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(path), "world.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, _ := strings.Cut(line, ":")
		key := strings.TrimPrefix(strings.TrimSpace(k), "world.")
		want, err := parseHexRGBA(strings.Trim(strings.TrimSpace(v), `"`))
		if err != nil {
			t.Fatalf("%s: %v", key, err)
		}
		got, ok := worldPalette[key]
		if !ok {
			t.Fatalf("yaml has %q, Go has no such colour", key)
		}
		if got != want {
			t.Fatalf("%s: yaml %v, Go %v", key, want, got)
		}
		seen++
	}
	if seen != len(worldPalette) {
		t.Fatalf("yaml has %d colours, Go has %d", seen, len(worldPalette))
	}
	for id := biome.ID(0); id < biome.NumBiomes; id++ {
		if _, ok := worldPalette[biome.DefOf(id).Key]; !ok {
			t.Fatalf("no globe colour for biome %s", biome.DefOf(id).Key)
		}
	}
}

func TestGlobeCornersAreSharedAcrossTiles(t *testing.T) {
	t.Parallel()
	p := globeTestPlanet(t, 100)
	ids, pos := dedupeCorners(p.Corners)
	// A closed hex sphere has 2T-4 corners (Euler), each in three tiles.
	if want := 2*p.TileCount() - 4; len(pos) != want {
		t.Fatalf("%d unique corners, want %d", len(pos), want)
	}
	uses := make([]int, len(pos))
	for _, id := range ids {
		uses[id]++
	}
	for id, n := range uses {
		if n != 3 {
			t.Fatalf("corner %d is used %d times", id, n)
		}
	}
	// On a partial world, boundary corners are used less, never more.
	q := globeTestPlanet(t, 30)
	_, qpos := dedupeCorners(q.Corners)
	if len(qpos) >= len(q.Corners) || len(qpos) < len(q.Corners)/3 {
		t.Fatalf("30%%: %d corners from %d copies", len(qpos), len(q.Corners))
	}
}

func TestGlobeColoursByWaterThenBiome(t *testing.T) {
	t.Parallel()
	g := testGlobe(t)
	p := g.P
	var ocean, lake, landMtn, seaIce = -1, -1, -1, -1
	for i := 0; i < p.TileCount(); i++ {
		switch {
		case p.Water[i] == planet.Ocean && ocean < 0:
			ocean = i
		case p.Water[i] == planet.Lake && lake < 0:
			lake = i
		case p.Water[i] == planet.Land && p.Hill[i] == site.Mountainous && landMtn < 0:
			landMtn = i
		case p.Biome[i] == biome.SeaIce && p.Water[i] == planet.Land && seaIce < 0:
			seaIce = i
		}
	}
	if ocean < 0 {
		t.Fatal("test world has no ocean")
	}
	// Water tiles store Biome 0 (temperate forest); they must not take its colour.
	if p.Biome[ocean] != biome.TemperateForest {
		t.Fatalf("ocean tile biome is %v, expected the placeholder 0", p.Biome[ocean])
	}
	if got, want := g.Base[ocean], oceanColour(p.Elev[ocean]); got != want {
		t.Fatalf("ocean colour %v, want depth shading %v", got, want)
	}
	if g.Base[ocean] == palRGB("temperate_forest") {
		t.Fatal("ocean drawn as temperate forest")
	}
	if lake >= 0 && g.Base[lake] != palRGB("lake") {
		t.Fatalf("lake colour %v", g.Base[lake])
	}
	// Ocean depth: deeper is darker.
	if d, s := oceanColour(-4000), oceanColour(-50); d[2] >= s[2] {
		t.Fatalf("deep %v not darker than shallow %v", d, s)
	}
	// Land mixes towards the mountain tint by hilliness (jitter is within 3 %).
	if landMtn >= 0 {
		want := mixRGB(palRGB(biome.DefOf(p.Biome[landMtn]).Key), palRGB("mountain_tint"), 0.40)
		for c := 0; c < 3; c++ {
			if r := g.Base[landMtn][c] / want[c]; r < 0.969 || r > 1.031 {
				t.Fatalf("mountain tile channel %d ratio %v", c, r)
			}
		}
	}
	if seaIce >= 0 {
		want := palRGB("sea_ice")
		for c := 0; c < 3; c++ {
			if r := g.Base[seaIce][c] / want[c]; r < 0.969 || r > 1.031 {
				t.Fatalf("sea ice drawn as water: ratio %v", r)
			}
		}
	}
	// Every hilliness has a mix.
	for _, h := range []site.Hills{site.HillsDefault, site.Flat, site.SmallHills, site.LargeHills, site.Mountainous} {
		if int(h) >= len(hillMix) {
			t.Fatalf("no hill mix for %v", h)
		}
	}
	if hillMix[site.Flat] != 0 || hillMix[site.SmallHills] != 0.12 || hillMix[site.LargeHills] != 0.25 || hillMix[site.Mountainous] != 0.40 {
		t.Fatalf("hill mixes %v", hillMix)
	}
}

func TestGlobeModesRecolourOnly(t *testing.T) {
	t.Parallel()
	g := testGlobe(t)
	var land, ocean = -1, -1
	for i := 0; i < g.P.TileCount(); i++ {
		if g.P.Water[i] == planet.Land && land < 0 && g.P.Biome[i] != biome.SeaIce {
			land = i
		}
		if g.P.Water[i] == planet.Ocean && ocean < 0 {
			ocean = i
		}
	}
	biomeLand, biomeOcean := g.Base[land], g.Base[ocean]
	nv := g.Verts
	for _, m := range []globeMode{globeTemperature, globeRainfall, globeElevation} {
		g.SetMode(m)
		if g.Mode() != m {
			t.Fatalf("mode %v not set", m)
		}
		if g.Base[land] == biomeLand {
			t.Fatalf("%v: land colour unchanged", m)
		}
		if g.Base[ocean][2] >= biomeOcean[2] && m != globeElevation {
			// data views dim the water so the land reads
			t.Fatalf("%v: ocean not dimmed", m)
		}
		if m.String() == "" {
			t.Fatal("empty mode name")
		}
	}
	g.SetMode(globeBiome)
	if g.Base[land] != biomeLand || g.Base[ocean] != biomeOcean {
		t.Fatal("switching back does not restore the biome colours")
	}
	g.SetMode(numGlobeModes) // ignored
	if g.Mode() != globeBiome {
		t.Fatal("invalid mode accepted")
	}
	if &nv[0] != &g.Verts[0] {
		t.Fatal("recolour reallocated the vertex buffer")
	}
	for _, c := range []float32{-40, 0, 40} {
		if v := ramp(tempRamp, c); v[0] < 0 || v[0] > 1 {
			t.Fatalf("ramp(%v) = %v", c, v)
		}
	}
}

func TestGlobePickTileRoundTrip(t *testing.T) {
	t.Parallel()
	g := testGlobe(t)
	rng := rand.New(rand.NewSource(3))
	n := g.P.TileCount()
	checked := 0
	for trial := 0; trial < 60; trial++ {
		g.Cam.Lat = (rng.Float64()*2 - 1) * 1.3
		g.Cam.Lon = (rng.Float64()*2 - 1) * math.Pi
		g.Cam.SetDist(globemath.MinDist + rng.Float64()*(globemath.MaxDist-globemath.MinDist))
		g.Cam.PanX, g.Cam.PanY = (rng.Float64()-0.5)*200, (rng.Float64()-0.5)*100
		for k := 0; k < 40; k++ {
			i := rng.Intn(n)
			x, y, front := screenOf(g, i)
			if !front {
				continue
			}
			if got := g.PickTile(int(math.Round(x)), int(math.Round(y))); got != i {
				// A rounded pixel can land in a neighbour when tiles are tiny; the
				// exact point must always pick the tile.
				w, ok := g.Cam.Pick(g.Viewport(), x, y)
				if !ok {
					t.Fatalf("tile %d centre not pickable", i)
				}
				if got := g.P.TileAt([3]float32{float32(w[0]), float32(w[1]), float32(w[2])}); got != i {
					t.Fatalf("tile %d picked as %d", i, got)
				}
			}
			checked++
		}
	}
	if checked < 300 {
		t.Fatalf("only %d tiles exercised", checked)
	}
}

func TestGlobeDragDoesNotSelect(t *testing.T) {
	t.Parallel()
	g := testGlobe(t)
	centreNow(g, landTile(t, g))
	lon0 := g.Cam.Lon
	in := globeInput{W: 1280, H: 720, MX: 640, MY: 360}
	in.Press[mbLeft], in.Down[mbLeft] = true, true
	g.Update(in, true)
	in.Press[mbLeft] = false
	for step := 1; step <= 10; step++ { // 100 px right over 10 frames
		in.MX = 640 + step*10
		if c, _ := g.Update(in, true); c != globeNone {
			t.Fatalf("clicked %d while dragging", c)
		}
	}
	in.Down[mbLeft], in.Release[mbLeft] = false, true
	if c, _ := g.Update(in, true); c != globeNone || g.Selected() != -1 {
		t.Fatalf("drag release selected %d / %d", c, g.Selected())
	}
	if g.Cam.Lon >= lon0 { // dragging right turns the surface right: lon decreases
		t.Fatalf("drag did not rotate: lon %v -> %v", lon0, g.Cam.Lon)
	}
	// A 3 px wobble is still a click.
	g2 := testGlobe(t)
	i := landTile(t, g2)
	centreNow(g2, i)
	in2 := globeInput{W: 1280, H: 720, MX: 640, MY: 360}
	in2.Press[mbLeft], in2.Down[mbLeft] = true, true
	g2.Update(in2, true)
	in2.Press[mbLeft], in2.MX = false, 643
	g2.Update(in2, true)
	in2.Down[mbLeft], in2.Release[mbLeft] = false, true
	if c, _ := g2.Update(in2, true); c < 0 {
		t.Fatalf("3 px wobble was not a click: %d", c)
	}
}

func TestGlobeClickSelectsTileUnderCursor(t *testing.T) {
	t.Parallel()
	g := testGlobe(t)
	i := landTile(t, g)
	centreNow(g, i)
	x, y, front := screenOf(g, i)
	if !front {
		t.Fatal("centred tile not facing")
	}
	if got := globeClick(g, int(math.Round(x)), int(math.Round(y)), false); got != i {
		t.Fatalf("clicked %d, want %d", got, i)
	}
	if g.Selected() != i {
		t.Fatalf("selected %d, want %d", g.Selected(), i)
	}
	// A neighbour is a different tile.
	nb := int(g.P.Neighbours(i)[0])
	nx, ny, _ := screenOf(g, nb)
	if got := globeClick(g, int(math.Round(nx)), int(math.Round(ny)), false); got != nb || g.Selected() != nb {
		t.Fatalf("neighbour click gave %d (selected %d), want %d", got, g.Selected(), nb)
	}
	// Blocked pointer (over a panel): nothing happens.
	g.Select(-1)
	if got := globeClick(g, int(x), int(y), true); got != globeNone || g.Selected() != -1 {
		t.Fatalf("blocked click selected %d", got)
	}
	// Right-click clears.
	g.Select(i)
	in := globeInput{W: 1280, H: 720, MX: 10, MY: 10}
	in.Press[mbRight] = true
	g.Update(in, true)
	if g.Selected() != -1 {
		t.Fatal("right-click did not clear")
	}
	// Off the globe: no click.
	if got := globeClick(g, 2, 2, false); got != globeNone {
		t.Fatalf("click in space gave %d", got)
	}
}

func TestGlobeClickOnUncoveredSphere(t *testing.T) {
	t.Parallel()
	g := testGlobe(t)
	pole := g.P.Pole
	g.Cam.CentreOn(globemath.Vec3{-float64(pole[0]), -float64(pole[1]), -float64(pole[2])}, 0)
	g.Select(5)
	if got := globeClick(g, 640, 360, false); got != globeUncovered {
		t.Fatalf("click on the far side of a 30%% world gave %d", got)
	}
	if g.Selected() != -1 {
		t.Fatal("uncovered click kept the selection")
	}
	in := globeInput{W: 1280, H: 720, MX: 640, MY: 360}
	if _, h := g.Update(in, true); h != globeUncovered {
		t.Fatalf("hover over uncovered = %d", h)
	}
}

func TestGlobeHoverFollowsPointerAndPanelBlocks(t *testing.T) {
	t.Parallel()
	g := testGlobe(t)
	i := landTile(t, g)
	centreNow(g, i)
	x, y, _ := screenOf(g, i)
	in := globeInput{W: 1280, H: 720, MX: int(math.Round(x)), MY: int(math.Round(y))}
	if _, h := g.Update(in, true); h != i || g.Hovered() != i {
		t.Fatalf("hover %d, want %d", h, i)
	}
	in.Blocked = true
	if _, h := g.Update(in, true); h != globeNone {
		t.Fatalf("blocked hover = %d", h)
	}
}

func TestGlobeDoubleClickCentres(t *testing.T) {
	t.Parallel()
	g := testGlobe(t)
	i := landTile(t, g)
	centreNow(g, i)
	nb := int(g.P.Neighbours(i)[1])
	// Look at a tile slightly off-centre by clicking its neighbour twice.
	x, y, _ := screenOf(g, nb)
	globeClick(g, int(x), int(y), false)
	if g.Cam.Flying() {
		t.Fatal("a single click flew")
	}
	globeClick(g, int(x), int(y), false)
	if g.Selected() != nb {
		t.Fatalf("selected %d", g.Selected())
	}
	// Reduced motion: the centring is instant.
	cx, cy, _ := screenOf(g, nb)
	if math.Abs(cx-640) > 1 || math.Abs(cy-360) > 1 {
		t.Fatalf("double-click left the tile at (%v, %v)", cx, cy)
	}
	// Slow second click: no centring.
	g2 := testGlobe(t)
	j := landTile(t, g2)
	centreNow(g2, j)
	nb2 := int(g2.P.Neighbours(j)[1])
	x2, y2, _ := screenOf(g2, nb2)
	globeClick(g2, int(x2), int(y2), false)
	for k := 0; k < globeDoubleFrames+2; k++ {
		g2.Update(globeInput{W: 1280, H: 720, MX: 5, MY: 5}, true)
	}
	globeClick(g2, int(x2), int(y2), false)
	if cx, _, _ := screenOf(g2, nb2); math.Abs(cx-640) < 1 {
		t.Fatal("slow second click centred")
	}
}

func TestGlobeCentreFliesUnlessReducedMotion(t *testing.T) {
	t.Parallel()
	g := testGlobe(t)
	i := landTile(t, g)
	g.Reduced = false
	g.Center(i)
	if !g.Cam.Flying() {
		t.Fatal("no flight")
	}
	for k := 0; k < globeFlyFrames+1; k++ {
		g.Update(globeInput{W: 1280, H: 720}, false)
	}
	if x, y, _ := screenOf(g, i); math.Abs(x-640) > 1 || math.Abs(y-360) > 1 {
		t.Fatalf("flight ended at (%v, %v)", x, y)
	}
	g.Reduced = true
	g.Cam.Lon += 1
	g.Center(i)
	if g.Cam.Flying() {
		t.Fatal("reduced motion still flies")
	}
	g.Center(-1) // ignored
	g.Center(g.P.TileCount())
}

func TestGlobeKeysWheelAndHome(t *testing.T) {
	t.Parallel()
	g := testGlobe(t)
	i := landTile(t, g)
	centreNow(g, i)
	lon, lat := g.Cam.Lon, g.Cam.Lat
	g.Update(globeInput{W: 1280, H: 720, MX: 5, MY: 5, Held: []string{"D"}}, true)
	d1 := g.Cam.Lon - lon
	if d1 <= 0 {
		t.Fatalf("D turned lon by %v", d1)
	}
	lon = g.Cam.Lon
	g.Update(globeInput{W: 1280, H: 720, MX: 5, MY: 5, Held: []string{"ArrowRight"}, Shift: true}, true)
	if d3 := g.Cam.Lon - lon; d3 < 2.9*d1 || d3 > 3.1*d1 {
		t.Fatalf("shift step %v, plain %v", d3, d1)
	}
	g.Update(globeInput{W: 1280, H: 720, MX: 5, MY: 5, Held: []string{"W"}}, true)
	if g.Cam.Lat <= lat {
		t.Fatal("W did not look north")
	}
	g.Update(globeInput{W: 1280, H: 720, MX: 5, MY: 5, Held: []string{"S", "A"}}, true)
	// Zoom: wheel, keys, clamps.
	d0 := g.Cam.Dist
	g.Update(globeInput{W: 1280, H: 720, MX: 5, MY: 5, Wheel: 1}, true)
	if g.Cam.Dist >= d0 {
		t.Fatal("wheel up did not zoom in")
	}
	g.Update(globeInput{W: 1280, H: 720, MX: 5, MY: 5, Wheel: 1, Blocked: true}, true)
	d1z := g.Cam.Dist
	g.Update(globeInput{W: 1280, H: 720, MX: 5, MY: 5, Held: []string{"PageDown"}}, true)
	if g.Cam.Dist <= d1z {
		t.Fatal("PageDown did not zoom out")
	}
	for k := 0; k < 300; k++ {
		g.Update(globeInput{W: 1280, H: 720, MX: 5, MY: 5, Held: []string{"PageUp", "Equal"}}, true)
	}
	if g.Cam.Dist != globemath.MinDist {
		t.Fatalf("zoom in stopped at %v", g.Cam.Dist)
	}
	for k := 0; k < 300; k++ {
		g.Update(globeInput{W: 1280, H: 720, MX: 5, MY: 5, Held: []string{"Minus"}, Wheel: -3}, true)
	}
	if g.Cam.Dist != globemath.MaxDist {
		t.Fatalf("zoom out stopped at %v", g.Cam.Dist)
	}
	// Home centres the selection and resets the pan.
	g.Select(i)
	g.Cam.PanX = 100
	g.Update(globeInput{W: 1280, H: 720, MX: 5, MY: 5, Keys: []string{"Home"}}, true)
	if x, y, _ := screenOf(g, i); math.Abs(x-640) > 1 || math.Abs(y-360) > 1 {
		t.Fatalf("Home left the tile at (%v, %v)", x, y)
	}
}

func TestGlobeMiddleDragPans(t *testing.T) {
	t.Parallel()
	g := testGlobe(t)
	in := globeInput{W: 1280, H: 720, MX: 400, MY: 300}
	in.Press[mbMiddle], in.Down[mbMiddle] = true, true
	g.Update(in, true)
	in.Press[mbMiddle], in.MX, in.MY = false, 460, 280
	g.Update(in, true)
	if g.Cam.PanX != 60 || g.Cam.PanY != -20 {
		t.Fatalf("pan %v,%v", g.Cam.PanX, g.Cam.PanY)
	}
	in.Down[mbMiddle], in.MX = false, 9000
	g.Update(in, true)
	if g.Cam.PanX != 60 {
		t.Fatal("pan continued after release")
	}
	// Clamped.
	in.Press[mbMiddle], in.Down[mbMiddle] = true, true
	g.Update(in, true)
	in.Press[mbMiddle], in.MX = false, 9000
	g.Update(in, true)
	if g.Cam.PanX > 0.4*1280+1 {
		t.Fatalf("pan %v not clamped", g.Cam.PanX)
	}
}

func TestGlobeInertiaAfterDrag(t *testing.T) {
	t.Parallel()
	run := func(reduced bool) (spun bool) {
		g := testGlobe(t)
		in := globeInput{W: 1280, H: 720, MX: 600, MY: 360}
		in.Press[mbLeft], in.Down[mbLeft] = true, true
		g.Update(in, reduced)
		in.Press[mbLeft] = false
		for k := 1; k <= 6; k++ {
			in.MX = 600 + k*12
			g.Update(in, reduced)
		}
		in.Down[mbLeft], in.Release[mbLeft] = false, true
		g.Update(in, reduced)
		in.Release[mbLeft] = false
		lon := g.Cam.Lon
		g.Update(in, reduced)
		first := math.Abs(g.Cam.Lon - lon)
		lon = g.Cam.Lon
		g.Update(in, reduced)
		second := math.Abs(g.Cam.Lon - lon)
		if reduced {
			return first != 0
		}
		if first == 0 || math.Abs(second/first-0.92) > 0.01 {
			t.Fatalf("inertia steps %v then %v, want 8%% decay", first, second)
		}
		return true
	}
	if !run(false) {
		t.Fatal("no inertia")
	}
	if run(true) {
		t.Fatal("inertia with Reduced Motion")
	}
}

func TestGlobeHighlightExpires(t *testing.T) {
	t.Parallel()
	g := testGlobe(t)
	i := landTile(t, g)
	centreNow(g, i)
	g.Highlight([]int{i, -3, 1 << 20}, 3)
	g.Build()
	withHL := len(g.Ov)
	if withHL == 0 {
		t.Fatal("highlight drew nothing")
	}
	for k := 0; k < 3; k++ {
		g.Update(globeInput{W: 1280, H: 720, MX: 2, MY: 2}, true)
	}
	g.Build()
	if len(g.Ov) >= withHL {
		t.Fatalf("highlight still drawn: %d >= %d", len(g.Ov), withHL)
	}
}

func TestGlobeSelectBounds(t *testing.T) {
	t.Parallel()
	g := testGlobe(t)
	g.Select(3)
	if g.Selected() != 3 {
		t.Fatal("select")
	}
	g.Select(g.P.TileCount())
	if g.Selected() != -1 {
		t.Fatal("out of range select kept")
	}
}

// TestGlobeBuildStaysInsideTheSilhouette checks no vertex lands outside
// the sphere's outline (a tile folded over the horizon would).
func TestGlobeBuildStaysInsideTheSilhouette(t *testing.T) {
	t.Parallel()
	g := newGlobeView(globeTestPlanet(t, 100), nil)
	g.Sw, g.Sh = 1280, 720
	rng := rand.New(rand.NewSource(11))
	for trial := 0; trial < 12; trial++ {
		g.Cam.Lat = (rng.Float64()*2 - 1) * 1.4
		g.Cam.Lon = (rng.Float64()*2 - 1) * math.Pi
		g.Cam.SetDist(globemath.MinDist + rng.Float64()*(globemath.MaxDist-globemath.MinDist))
		g.Build()
		if g.NV == 0 || g.NI == 0 || g.NI%3 != 0 {
			t.Fatalf("empty or ragged mesh: %d verts %d indices", g.NV, g.NI)
		}
		vp := g.Viewport()
		r := globemath.SilhouetteRadius(vp, g.Cam.Dist)
		for k := 0; k < g.NV; k++ {
			v := g.Verts[k]
			if math.IsNaN(float64(v.DstX)) || math.IsNaN(float64(v.DstY)) {
				t.Fatal("NaN vertex")
			}
			if d := math.Hypot(float64(v.DstX)-vp.CX, float64(v.DstY)-vp.CY); d > r+1.5 {
				t.Fatalf("vertex %d at %.1f px from the centre, silhouette %.1f (dist %.2f)", k, d, r, g.Cam.Dist)
			}
		}
	}
}

// TestGlobeHasNoHoles samples points inside the silhouette and requires a
// triangle under each: no cracks between tiles, none at the pentagons or
// the poles, none along the horizon.
func TestGlobeHasNoHoles(t *testing.T) {
	t.Parallel()
	p := globeTestPlanet(t, 100)
	g := newGlobeView(p, nil)
	g.Sw, g.Sh = 1280, 720
	var pent []int
	for i := 0; i < p.TileCount() && len(pent) < 3; i++ {
		if p.CornerOff[i+1]-p.CornerOff[i] == 5 {
			pent = append(pent, i)
		}
	}
	views := []globemath.Vec3{{0, 0, 1}, {0, 0, -1}, {1, 0, 0}, {0.3, -0.8, 0.5}}
	for _, i := range pent {
		c := p.Center[i]
		views = append(views, globemath.Vec3{float64(c[0]), float64(c[1]), float64(c[2])})
	}
	rng := rand.New(rand.NewSource(5))
	for vi, v := range views {
		for _, dist := range []float64{1.4, 2.9, 4.0} {
			g.Cam.SetDist(dist)
			g.Cam.CentreOn(v, 0)
			g.Build()
			vp := g.Viewport()
			r := globemath.SilhouetteRadius(vp, g.Cam.Dist)
			misses := 0
			for s := 0; s < 400; s++ {
				a, d := rng.Float64()*2*math.Pi, math.Sqrt(rng.Float64())*r*0.985
				x, y := vp.CX+d*math.Cos(a), vp.CY+d*math.Sin(a)
				if !coveredByMesh(g, float32(x), float32(y)) {
					misses++
				}
			}
			if misses > 0 {
				t.Fatalf("view %d dist %.1f: %d of 400 sample points have no triangle", vi, dist, misses)
			}
		}
	}
}

func coveredByMesh(g *globeView, x, y float32) bool {
	for k := 0; k+2 < g.NI; k += 3 {
		a, b, c := g.Verts[g.Idx[k]], g.Verts[g.Idx[k+1]], g.Verts[g.Idx[k+2]]
		d1 := (x-b.DstX)*(a.DstY-b.DstY) - (a.DstX-b.DstX)*(y-b.DstY)
		d2 := (x-c.DstX)*(b.DstY-c.DstY) - (b.DstX-c.DstX)*(y-c.DstY)
		d3 := (x-a.DstX)*(c.DstY-a.DstY) - (c.DstX-a.DstX)*(y-a.DstY)
		neg := d1 < -1e-3 || d2 < -1e-3 || d3 < -1e-3
		pos := d1 > 1e-3 || d2 > 1e-3 || d3 > 1e-3
		if !(neg && pos) {
			return true
		}
	}
	return false
}

func TestGlobeCullsTheFarSide(t *testing.T) {
	t.Parallel()
	g := newGlobeView(globeTestPlanet(t, 100), nil)
	g.Sw, g.Sh = 1280, 720
	g.Build()
	tiles := g.NV / 7 // roughly: one centre + 5 or 6 corners per tile
	if n := g.P.TileCount(); tiles > n*6/10 || tiles < n/10 {
		t.Fatalf("%d of %d tiles drawn", tiles, n)
	}
}

func TestGlobeNoAllocSteadyState(t *testing.T) {
	g := newGlobeView(globeTestPlanet(t, 100), nil)
	g.Sw, g.Sh = 1280, 720
	g.Select(50)
	g.Build() // warm: the first build sizes nothing, but be safe
	allocs := testing.AllocsPerRun(20, func() {
		g.Cam.Lon += 0.01
		g.Build()
	})
	if allocs > 1 {
		t.Fatalf("%v allocations per frame build", allocs)
	}
}

func TestGlobeOverlayDrawsRiversAndRoads(t *testing.T) {
	t.Parallel()
	g := newGlobeView(globeTestPlanet(t, 100), nil)
	g.Sw, g.Sh = 1280, 720
	if len(g.Rivers) == 0 {
		t.Skip("test world has no rivers")
	}
	// Look at a river tile.
	for i, s := range g.P.River {
		if s != site.NoRiver && g.P.RiverTo[i] >= 0 {
			centreNow(g, i)
			break
		}
	}
	g.Cam.SetDist(1.6)
	g.Build()
	if len(g.Ov) < 8 || len(g.Oi)%3 != 0 {
		t.Fatalf("overlay has %d vertices, %d indices", len(g.Ov), len(g.Oi))
	}
	for _, i := range g.Oi {
		if int(i) >= len(g.Ov) {
			t.Fatalf("overlay index %d beyond %d vertices", i, len(g.Ov))
		}
	}
}

func TestBakeStarfield(t *testing.T) {
	t.Parallel()
	a := bakeStarfield(42, 640, 360, 1, 0)
	b := bakeStarfield(42, 640, 360, 1, 0)
	c := bakeStarfield(43, 640, 360, 1, 0)
	if string(a.Pix) != string(b.Pix) {
		t.Fatal("same seed, different sky")
	}
	if string(a.Pix) == string(c.Pix) {
		t.Fatal("different seeds, same sky")
	}
	stars := 0
	for i := 3; i < len(a.Pix); i += 4 {
		if a.Pix[i] != 0 {
			stars++
		}
	}
	if stars < 100 || stars > 640*360/20 {
		t.Fatalf("%d lit pixels", stars)
	}
	// The sky fade dims the bottom half and leaves the top almost alone
	// (same star positions, so the comparison is exact in expectation).
	f := bakeStarfield(42, 640, 360, 1, 0.9)
	sum := func(img *image.NRGBA, y0, y1 int) (n int) {
		for y := y0; y < y1; y++ {
			for x := 0; x < 640; x++ {
				n += int(img.NRGBAAt(x, y).A)
			}
		}
		return n
	}
	if sum(f, 180, 360)*10 > sum(a, 180, 360)*6 {
		t.Fatalf("bottom half not dimmed: %d vs %d", sum(f, 180, 360), sum(a, 180, 360))
	}
	if sum(f, 0, 180)*10 < sum(a, 0, 180)*7 {
		t.Fatalf("top half dimmed too much: %d vs %d", sum(f, 0, 180), sum(a, 0, 180))
	}
}

func TestBakeSphere(t *testing.T) {
	t.Parallel()
	img := bakeSphere(globeSphereBakeSize, globeSphereBakeR, palRGB("uncovered"), palRGB("atmosphere_rim"))
	c := globeSphereBakeSize / 2
	if a := img.NRGBAAt(c, c).A; a != 255 {
		t.Fatalf("centre alpha %d", a)
	}
	if a := img.NRGBAAt(0, 0).A; a != 0 {
		t.Fatalf("corner alpha %d", a)
	}
	inner := img.NRGBAAt(c+globeSphereBakeR+4, c).A
	outer := img.NRGBAAt(c+globeSphereBakeR+40, c).A
	if inner == 0 || outer >= inner {
		t.Fatalf("halo alpha %d then %d should fade", inner, outer)
	}
	// The sun is upper left: the disc is brighter there.
	ul, lr := img.NRGBAAt(c-100, c-100), img.NRGBAAt(c+100, c+100)
	if int(ul.R)+int(ul.G)+int(ul.B) <= int(lr.R)+int(lr.G)+int(lr.B) {
		t.Fatalf("lighting: upper left %v, lower right %v", ul, lr)
	}
}

func TestGlobeShadeBounds(t *testing.T) {
	t.Parallel()
	for _, v := range [][3]float32{{0, 0, 1}, {1, 0, 0}, {-1, 0, 0}, {0, 0, -1}, {-0.42, 0.42, 0.8}} {
		k1, k2 := globeShade(v[0], v[1], v[2])
		if k1 < 0 || k1 > 1.001 || k2 < 0 || k2 > 0.46 {
			t.Fatalf("shade%v = %v, %v", v, k1, k2)
		}
	}
	if a, _ := globeShade(-0.4, 0.4, 0.82); a < 0.95 {
		t.Fatalf("sun-facing shade %v", a)
	}
}

func TestGlobeAccentsAreDistinct(t *testing.T) {
	t.Parallel()
	seen := map[color.NRGBA]faction.Type{}
	for _, ty := range []faction.Type{faction.Settlers, faction.Clans, faction.Drifters, faction.Freebooters, faction.Swarm, faction.Player} {
		c := globeAccent(ty)
		if o, dup := seen[c]; dup {
			t.Fatalf("%v and %v share %v", ty, o, c)
		}
		seen[c] = ty
	}
}

func TestGlobeMarksMatchSettlements(t *testing.T) {
	t.Parallel()
	g := newGlobeView(globeTestPlanet(t, 100), nil)
	if want := len(g.P.SettlementTiles()); len(g.Marks) != want {
		t.Fatalf("%d marks for %d settlements", len(g.Marks), want)
	}
	for _, m := range g.Marks {
		if !strings.HasPrefix(m.Key, "ui_icon/settlement_") || m.Label == "" {
			t.Fatalf("bad mark %+v", m)
		}
	}
}

func TestGlobeDrawsHeadless(t *testing.T) {
	g := testGlobe(t)
	g.Select(landTile(t, g))
	screen := ebiten.NewImage(1280, 720)
	g.Draw(screen, 1280, 720)
	g.SetMode(globeRainfall)
	g.Draw(screen, 1280, 720)
	g.SetArea(0, 0, 900, 720)
	g.Cam.SetDist(1.5)
	g.Draw(screen, 1280, 720)
}

func BenchmarkGlobeFrame(b *testing.B) {
	for _, dist := range []float64{1.5, 2.9, 4.0} {
		b.Run(fmt.Sprintf("dist%.1f", dist), func(b *testing.B) {
			g := newGlobeView(globeTestPlanet(b, 100), nil)
			g.Sw, g.Sh = 1280, 720
			g.Cam.SetDist(dist)
			g.Select(100)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				g.Cam.Lon += 0.003
				g.Build()
			}
		})
	}
}

// TestGlobeDrawsEveryFacingTile looks straight at a partial world, where
// every tile faces the eye: none may be dropped by the buffer sizing.
func TestGlobeDrawsEveryFacingTile(t *testing.T) {
	t.Parallel()
	for _, cov := range []int{30, 100} {
		g := newGlobeView(globeTestPlanet(t, cov), nil)
		g.Sw, g.Sh = 1280, 720
		for _, dist := range []float64{globemath.MinDist, globemath.MaxDist} {
			g.Cam.SetDist(dist)
			if cov == 30 {
				c := g.P.Pole
				g.Cam.CentreOn(globemath.Vec3{float64(c[0]), float64(c[1]), float64(c[2])}, 0)
			}
			g.Build()
			want, tiles := 0, 0
			cd := g.CamDir
			thresh := g.Zh - globeCullSlack
			for i := range g.P.Center {
				c := g.P.Center[i]
				if c[0]*cd[0]+c[1]*cd[1]+c[2]*cd[2] >= thresh {
					want += int(g.P.CornerOff[i+1] - g.P.CornerOff[i])
					tiles++
				}
			}
			if g.NI/3 != want {
				t.Fatalf("coverage %d dist %.2f: %d triangles for %d facing tiles (want %d)", cov, dist, g.NI/3, tiles, want)
			}
		}
	}
}

func TestGlobeInputFromFrame(t *testing.T) {
	t.Parallel()
	raw := rawInput{Mx: 30, My: 40, WheelY: 2, Held: []string{"W"}, Pressed: []string{"Home"}}
	raw.Press[mbLeft], raw.Down[mbLeft], raw.Release[mbRight] = true, true, true
	f := newFrame(raw)
	in := globeInputFrom(&f, 1280, 720, true)
	if in.MX != 30 || in.MY != 40 || in.Wheel != 2 || !in.Press[mbLeft] || !in.Down[mbLeft] || !in.Release[mbRight] ||
		!in.Blocked || in.W != 1280 || in.H != 720 || len(in.Held) != 1 || len(in.Keys) != 1 || in.Shift {
		t.Fatalf("globeInputFrom = %+v", in)
	}
}
