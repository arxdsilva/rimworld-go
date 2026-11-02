package globe

import (
	"encoding/hex"
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

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/globemath"
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
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
		Factions: faction.DefaultSlots(), Version: planet.CurrentVersion})
	globePlanets[coverage] = p
	return p
}

// testGlobe is a fresh view on the 30 % world at 1280x720.
func testGlobe(t testing.TB) *View {
	t.Helper()
	g := New(globeTestPlanet(t, 30), nil)
	g.Sw, g.Sh = 1280, 720
	g.Reduced = true
	return g
}

// screenOf is where tile i's centre is on screen and whether it faces the eye.
func screenOf(g *View, i int) (x, y float64, front bool) {
	c := g.P.Center[i]
	sx, sy, z := g.Cam.Project(g.Viewport(), globemath.Vec3{float64(c[0]), float64(c[1]), float64(c[2])})
	return sx, sy, z > globemath.HorizonZ(g.Cam.Dist)+0.05
}

func centreNow(g *View, i int) {
	c := g.P.Center[i]
	g.Cam.PanX, g.Cam.PanY = 0, 0
	g.Cam.CentreOn(globemath.Vec3{float64(c[0]), float64(c[1]), float64(c[2])}, 0)
}

func landTile(t testing.TB, g *View) int {
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
func globeClick(g *View, x, y int, blocked bool) (clicked int) {
	in := Input{W: 1280, H: 720, MX: x, MY: y, Blocked: blocked}
	in.Press[inputframe.Left], in.Down[inputframe.Left] = true, true
	g.Update(in, true)
	in.Press[inputframe.Left], in.Down[inputframe.Left], in.Release[inputframe.Left] = false, false, true
	c, _ := g.Update(in, true)
	return c
}

// locateWorldYAML finds assets/palettes/world.yaml above the working directory.
func locateWorldYAML() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	const rel = "assets/palettes/world.yaml"
	for i := 0; i < 12; i++ {
		cand := filepath.Join(dir, rel)
		if st, err := os.Stat(cand); err == nil && !st.IsDir() {
			return cand, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("palette %q not found from cwd", rel)
}

func TestGlobePaletteMatchesYAML(t *testing.T) {
	t.Parallel()
	path, err := locateWorldYAML()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
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
		got, ok := WorldPalette[key]
		if !ok {
			t.Fatalf("yaml has %q, Go has no such colour", key)
		}
		if got != want {
			t.Fatalf("%s: yaml %v, Go %v", key, want, got)
		}
		seen++
	}
	if seen != len(WorldPalette) {
		t.Fatalf("yaml has %d colours, Go has %d", seen, len(WorldPalette))
	}
	for id := biome.ID(0); id < biome.NumBiomes; id++ {
		if _, ok := WorldPalette[biome.DefOf(id).Key]; !ok {
			t.Fatalf("no globe colour for biome %s", biome.DefOf(id).Key)
		}
	}
}

func TestGlobeCornersAreSharedAcrossTiles(t *testing.T) {
	t.Parallel()
	p := globeTestPlanet(t, 100)
	ids, pos := DedupeCorners(p.Corners)
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
	_, qpos := DedupeCorners(q.Corners)
	if len(qpos) >= len(q.Corners) || len(qpos) < len(q.Corners)/3 {
		t.Fatalf("30%%: %d corners from %d copies", len(qpos), len(q.Corners))
	}
}

func TestGlobeColoursByWaterThenBiome(t *testing.T) {
	t.Parallel()
	g := testGlobe(t)
	p := g.P
	var ocean, shallow, lake, landMtn, seaIce = -1, -1, -1, -1, -1
	for i := 0; i < p.TileCount(); i++ {
		switch {
		case p.Water[i] == planet.Ocean && coastalShallow(p, i) && shallow < 0:
			shallow = i
		case p.Water[i] == planet.Ocean && !coastalShallow(p, i) && ocean < 0:
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
	if got, want := g.Base[ocean], OceanColour(p.Elev[ocean]); got != want {
		t.Fatalf("ocean colour %v, want depth shading %v", got, want)
	}
	if g.Base[ocean] == PalRGB("temperate_forest") {
		t.Fatal("ocean drawn as temperate forest")
	}
	if lake >= 0 && g.Base[lake] != PalRGB("lake") {
		t.Fatalf("lake colour %v", g.Base[lake])
	}
	// Ocean depth: deeper is darker.
	if d, s := OceanColour(-4000), OceanColour(-50); d[2] >= s[2] {
		t.Fatalf("deep %v not darker than shallow %v", d, s)
	}
	// The shallow band: coastal ocean shallower than 200 m.
	if shallow < 0 {
		t.Fatal("test world has no shallow coastal ocean")
	}
	if g.Base[shallow] != PalRGB("shallow") || p.Elev[shallow] <= -200 {
		t.Fatalf("shallow tile %d (elev %d) colour %v", shallow, p.Elev[shallow], g.Base[shallow])
	}
	// Mountains keep their biome colour: relief and light make them read,
	// not a tint (jitter is within 3 %).
	if landMtn >= 0 {
		want := PalRGB(biome.DefOf(p.Biome[landMtn]).Key)
		for c := 0; c < 3; c++ {
			if r := g.Base[landMtn][c] / want[c]; r < 0.969 || r > 1.031 {
				t.Fatalf("mountain tile channel %d ratio %v", c, r)
			}
		}
	}
	if seaIce >= 0 {
		want := PalRGB("sea_ice")
		for c := 0; c < 3; c++ {
			if r := g.Base[seaIce][c] / want[c]; r < 0.969 || r > 1.031 {
				t.Fatalf("sea ice drawn as water: ratio %v", r)
			}
		}
	}
	// Every hilliness has a Hilliness-mode colour.
	for _, h := range []site.Hills{site.HillsDefault, site.Flat, site.SmallHills, site.LargeHills, site.Mountainous} {
		if int(h) >= len(HillKey) {
			t.Fatalf("no hilliness colour for %v", h)
		}
		if _, ok := WorldPalette[HillKey[h]]; !ok {
			t.Fatalf("hilliness %v colour %q not in the palette", h, HillKey[h])
		}
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
	for _, m := range []Mode{ModeTemperature, ModeRainfall, ModeElevation, ModeHilliness} {
		g.SetMode(m)
		if g.Mode() != m {
			t.Fatalf("mode %v not set", m)
		}
		if g.Base[land] == biomeLand {
			t.Fatalf("%v: land colour unchanged", m)
		}
		if g.Base[ocean][2] >= biomeOcean[2] && m != ModeElevation {
			// data views dim the water so the land reads
			t.Fatalf("%v: ocean not dimmed", m)
		}
		if m.String() == "" {
			t.Fatal("empty mode name")
		}
	}
	g.SetMode(ModeBiome)
	if g.Base[land] != biomeLand || g.Base[ocean] != biomeOcean {
		t.Fatal("switching back does not restore the biome colours")
	}
	g.SetMode(NumModes) // ignored
	if g.Mode() != ModeBiome {
		t.Fatal("invalid mode accepted")
	}
	if &nv[0] != &g.Verts[0] {
		t.Fatal("recolour reallocated the vertex buffer")
	}
	for _, c := range []float32{-40, 0, 40} {
		if v := Ramp(TempRamp, c); v[0] < 0 || v[0] > 1 {
			t.Fatalf("Ramp(%v) = %v", c, v)
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
	in := Input{W: 1280, H: 720, MX: 640, MY: 360}
	in.Press[inputframe.Left], in.Down[inputframe.Left] = true, true
	g.Update(in, true)
	in.Press[inputframe.Left] = false
	for step := 1; step <= 10; step++ { // 100 px right over 10 frames
		in.MX = 640 + step*10
		if c, _ := g.Update(in, true); c != ModeNone {
			t.Fatalf("clicked %d while dragging", c)
		}
	}
	in.Down[inputframe.Left], in.Release[inputframe.Left] = false, true
	if c, _ := g.Update(in, true); c != ModeNone || g.Selected() != -1 {
		t.Fatalf("drag release selected %d / %d", c, g.Selected())
	}
	if g.Cam.Lon >= lon0 { // dragging right turns the surface right: lon decreases
		t.Fatalf("drag did not rotate: lon %v -> %v", lon0, g.Cam.Lon)
	}
	// A 3 px wobble is still a click.
	g2 := testGlobe(t)
	i := landTile(t, g2)
	centreNow(g2, i)
	in2 := Input{W: 1280, H: 720, MX: 640, MY: 360}
	in2.Press[inputframe.Left], in2.Down[inputframe.Left] = true, true
	g2.Update(in2, true)
	in2.Press[inputframe.Left], in2.MX = false, 643
	g2.Update(in2, true)
	in2.Down[inputframe.Left], in2.Release[inputframe.Left] = false, true
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
	if got := globeClick(g, int(x), int(y), true); got != ModeNone || g.Selected() != -1 {
		t.Fatalf("blocked click selected %d", got)
	}
	// Right-click clears.
	g.Select(i)
	in := Input{W: 1280, H: 720, MX: 10, MY: 10}
	in.Press[inputframe.Right] = true
	g.Update(in, true)
	if g.Selected() != -1 {
		t.Fatal("right-click did not clear")
	}
	// Off the globe: no click.
	if got := globeClick(g, 2, 2, false); got != ModeNone {
		t.Fatalf("click in space gave %d", got)
	}
}

func TestGlobeClickOnUncoveredSphere(t *testing.T) {
	t.Parallel()
	g := testGlobe(t)
	pole := g.P.Pole
	g.Cam.CentreOn(globemath.Vec3{-float64(pole[0]), -float64(pole[1]), -float64(pole[2])}, 0)
	g.Select(5)
	if got := globeClick(g, 640, 360, false); got != ModeUncovered {
		t.Fatalf("click on the far side of a 30%% world gave %d", got)
	}
	if g.Selected() != -1 {
		t.Fatal("uncovered click kept the selection")
	}
	in := Input{W: 1280, H: 720, MX: 640, MY: 360}
	if _, h := g.Update(in, true); h != ModeUncovered {
		t.Fatalf("hover over uncovered = %d", h)
	}
}

func TestGlobeHoverFollowsPointerAndPanelBlocks(t *testing.T) {
	t.Parallel()
	g := testGlobe(t)
	i := landTile(t, g)
	centreNow(g, i)
	x, y, _ := screenOf(g, i)
	in := Input{W: 1280, H: 720, MX: int(math.Round(x)), MY: int(math.Round(y))}
	if _, h := g.Update(in, true); h != i || g.Hovered() != i {
		t.Fatalf("hover %d, want %d", h, i)
	}
	in.Blocked = true
	if _, h := g.Update(in, true); h != ModeNone {
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
	for k := 0; k < DoubleFrames+2; k++ {
		g2.Update(Input{W: 1280, H: 720, MX: 5, MY: 5}, true)
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
	for k := 0; k < FlyFrames+1; k++ {
		g.Update(Input{W: 1280, H: 720}, false)
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
	g.Update(Input{W: 1280, H: 720, MX: 5, MY: 5, Held: []string{"D"}}, true)
	d1 := g.Cam.Lon - lon
	if d1 <= 0 {
		t.Fatalf("D turned lon by %v", d1)
	}
	lon = g.Cam.Lon
	g.Update(Input{W: 1280, H: 720, MX: 5, MY: 5, Held: []string{"ArrowRight"}, Shift: true}, true)
	if d3 := g.Cam.Lon - lon; d3 < 2.9*d1 || d3 > 3.1*d1 {
		t.Fatalf("shift step %v, plain %v", d3, d1)
	}
	g.Update(Input{W: 1280, H: 720, MX: 5, MY: 5, Held: []string{"W"}}, true)
	if g.Cam.Lat <= lat {
		t.Fatal("W did not look north")
	}
	g.Update(Input{W: 1280, H: 720, MX: 5, MY: 5, Held: []string{"S", "A"}}, true)
	// Zoom: wheel, keys, clamps.
	d0 := g.Cam.Dist
	g.Update(Input{W: 1280, H: 720, MX: 5, MY: 5, Wheel: 1}, true)
	if g.Cam.Dist >= d0 {
		t.Fatal("wheel up did not zoom in")
	}
	g.Update(Input{W: 1280, H: 720, MX: 5, MY: 5, Wheel: 1, Blocked: true}, true)
	d1z := g.Cam.Dist
	g.Update(Input{W: 1280, H: 720, MX: 5, MY: 5, Held: []string{"PageDown"}}, true)
	if g.Cam.Dist <= d1z {
		t.Fatal("PageDown did not zoom out")
	}
	for k := 0; k < 300; k++ {
		g.Update(Input{W: 1280, H: 720, MX: 5, MY: 5, Held: []string{"PageUp", "Equal"}}, true)
	}
	if g.Cam.Dist != globemath.MinDist {
		t.Fatalf("zoom in stopped at %v", g.Cam.Dist)
	}
	for k := 0; k < 300; k++ {
		g.Update(Input{W: 1280, H: 720, MX: 5, MY: 5, Held: []string{"Minus"}, Wheel: -3}, true)
	}
	if g.Cam.Dist != globemath.MaxDist {
		t.Fatalf("zoom out stopped at %v", g.Cam.Dist)
	}
	// Home centres the selection and resets the pan.
	g.Select(i)
	g.Cam.PanX = 100
	g.Update(Input{W: 1280, H: 720, MX: 5, MY: 5, Keys: []string{"Home"}}, true)
	if x, y, _ := screenOf(g, i); math.Abs(x-640) > 1 || math.Abs(y-360) > 1 {
		t.Fatalf("Home left the tile at (%v, %v)", x, y)
	}
}

func TestGlobeMiddleDragPans(t *testing.T) {
	t.Parallel()
	g := testGlobe(t)
	in := Input{W: 1280, H: 720, MX: 400, MY: 300}
	in.Press[inputframe.Middle], in.Down[inputframe.Middle] = true, true
	g.Update(in, true)
	in.Press[inputframe.Middle], in.MX, in.MY = false, 460, 280
	g.Update(in, true)
	if g.Cam.PanX != 60 || g.Cam.PanY != -20 {
		t.Fatalf("pan %v,%v", g.Cam.PanX, g.Cam.PanY)
	}
	in.Down[inputframe.Middle], in.MX = false, 9000
	g.Update(in, true)
	if g.Cam.PanX != 60 {
		t.Fatal("pan continued after release")
	}
	// Clamped.
	in.Press[inputframe.Middle], in.Down[inputframe.Middle] = true, true
	g.Update(in, true)
	in.Press[inputframe.Middle], in.MX = false, 9000
	g.Update(in, true)
	if g.Cam.PanX > 0.4*1280+1 {
		t.Fatalf("pan %v not clamped", g.Cam.PanX)
	}
}

func TestGlobeInertiaAfterDrag(t *testing.T) {
	t.Parallel()
	run := func(reduced bool) (spun bool) {
		g := testGlobe(t)
		in := Input{W: 1280, H: 720, MX: 600, MY: 360}
		in.Press[inputframe.Left], in.Down[inputframe.Left] = true, true
		g.Update(in, reduced)
		in.Press[inputframe.Left] = false
		for k := 1; k <= 6; k++ {
			in.MX = 600 + k*12
			g.Update(in, reduced)
		}
		in.Down[inputframe.Left], in.Release[inputframe.Left] = false, true
		g.Update(in, reduced)
		in.Release[inputframe.Left] = false
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
		g.Update(Input{W: 1280, H: 720, MX: 2, MY: 2}, true)
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
// the outline of the sphere of radius ReliefMax, the highest relief (a tile
// folded over the horizon would).
func TestGlobeBuildStaysInsideTheSilhouette(t *testing.T) {
	t.Parallel()
	g := New(globeTestPlanet(t, 100), nil)
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
		// The silhouette of a sphere of radius R seen from D is
		// f R / sqrt(D^2 - R^2), the unit sphere's seen from D/R.
		r := globemath.SilhouetteRadius(vp, g.Cam.Dist/ReliefMax)
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
	g := New(p, nil)
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

func coveredByMesh(g *View, x, y float32) bool {
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
	g := New(globeTestPlanet(t, 100), nil)
	g.Sw, g.Sh = 1280, 720
	g.Build()
	tiles := g.NV / 18 // roughly: six faces of three vertices per tile
	if n := g.P.TileCount(); tiles > n*6/10 || tiles < n/10 {
		t.Fatalf("%d of %d tiles drawn", tiles, n)
	}
}

func TestGlobeNoAllocSteadyState(t *testing.T) {
	g := New(globeTestPlanet(t, 100), nil)
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
	g := New(globeTestPlanet(t, 100), nil)
	g.Sw, g.Sh = 1280, 720
	if len(g.geom.Rivers) == 0 {
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
	a := BakeStarfield(42, 640, 360, 1, 0)
	b := BakeStarfield(42, 640, 360, 1, 0)
	c := BakeStarfield(43, 640, 360, 1, 0)
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
	f := BakeStarfield(42, 640, 360, 1, 0.9)
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
	img := BakeSphere(SphereBakeSize, SphereBakeR, PalRGB("uncovered"), PalRGB("atmosphere_rim"))
	c := SphereBakeSize / 2
	if a := img.NRGBAAt(c, c).A; a != 255 {
		t.Fatalf("centre alpha %d", a)
	}
	if a := img.NRGBAAt(0, 0).A; a != 0 {
		t.Fatalf("corner alpha %d", a)
	}
	inner := img.NRGBAAt(c+SphereBakeR+4, c).A
	outer := img.NRGBAAt(c+SphereBakeR+40, c).A
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
		k1, k2 := Shade(v[0], v[1], v[2])
		if k1 < 0 || k1 > 1.001 || k2 < 0 || k2 > 0.46 {
			t.Fatalf("shade%v = %v, %v", v, k1, k2)
		}
	}
	if a, _ := Shade(-0.4, 0.4, 0.82); a < 0.95 {
		t.Fatalf("sun-facing shade %v", a)
	}
}

func TestGlobeAccentsAreDistinct(t *testing.T) {
	t.Parallel()
	seen := map[color.NRGBA]faction.Type{}
	for _, ty := range []faction.Type{faction.Settlers, faction.Clans, faction.Drifters, faction.Freebooters, faction.Swarm, faction.Player} {
		c := Accent(ty)
		if o, dup := seen[c]; dup {
			t.Fatalf("%v and %v share %v", ty, o, c)
		}
		seen[c] = ty
	}
}

func TestGlobeMarksMatchSettlements(t *testing.T) {
	t.Parallel()
	g := New(globeTestPlanet(t, 100), nil)
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
	g.SetMode(ModeRainfall)
	g.Draw(screen, 1280, 720)
	g.SetArea(0, 0, 900, 720)
	g.Cam.SetDist(1.5)
	g.Draw(screen, 1280, 720)
}

func BenchmarkGlobeFrame(b *testing.B) {
	for _, dist := range []float64{1.5, 2.9, 4.0} {
		b.Run(fmt.Sprintf("dist%.1f", dist), func(b *testing.B) {
			g := New(globeTestPlanet(b, 100), nil)
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
		g := New(globeTestPlanet(t, cov), nil)
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
			thresh := g.Zh - CullSlack
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

func parseHexRGBA(s string) (color.RGBA, error) {
	s = strings.TrimPrefix(s, "#")
	if len(s) != 8 {
		return color.RGBA{}, fmt.Errorf("need #RRGGBBAA, got %q", s)
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return color.RGBA{}, err
	}
	return color.RGBA{R: b[0], G: b[1], B: b[2], A: b[3]}, nil
}

// reliefTile is a mountainous land tile of the full test world.
func reliefTile(t testing.TB, g *View) int {
	t.Helper()
	best, bestH := -1, float32(-1)
	for i := 0; i < g.P.TileCount(); i++ {
		if isLand(g.P, i) && g.P.Hill[i] == site.Mountainous && g.geom.centreH[i] > bestH {
			best, bestH = i, g.geom.centreH[i]
		}
	}
	if best < 0 {
		t.Fatal("test world has no mountains")
	}
	return best
}

func radius(v [3]float32) float32 {
	return float32(math.Sqrt(float64(v[0]*v[0] + v[1]*v[1] + v[2]*v[2])))
}

// TestGlobeReliefPeaksAboveCorners: every mountain's centre stands above
// its lowest corner and, in nine of ten, above the mean of its corners;
// a good share stand above all of them (peaks). A corner between two
// mountains is lifted toward their peaks to form a ridge, so a low peak
// may sit under a high saddle. Every vertex stays within [1, ReliefMax]
// and water stays at radius 1.
func TestGlobeReliefPeaksAboveCorners(t *testing.T) {
	t.Parallel()
	g := New(globeTestPlanet(t, 100), nil)
	p, geo := g.P, g.geom
	n, aboveMean, aboveAll := 0, 0, 0
	for i := 0; i < p.TileCount(); i++ {
		if !isLand(p, i) || p.Hill[i] != site.Mountainous {
			continue
		}
		n++
		rc := radius(geo.centreDisp[i])
		var mean, hi float32
		lo := float32(9)
		m := float32(p.CornerOff[i+1] - p.CornerOff[i])
		for o := p.CornerOff[i]; o < p.CornerOff[i+1]; o++ {
			rk := radius(geo.cornerDisp[geo.cornerID[o]])
			mean += rk / m
			lo, hi = min(lo, rk), max(hi, rk)
		}
		if rc <= lo {
			t.Fatalf("mountain %d: centre radius %v not above its lowest corner %v", i, rc, lo)
		}
		if rc > mean {
			aboveMean++
		}
		if rc > hi {
			aboveAll++
		}
	}
	if n == 0 {
		t.Fatal("no mountains")
	}
	if aboveMean*10 < n*9 || aboveAll*10 < n*3 {
		t.Fatalf("of %d mountains %d stand above their corners' mean and %d above every corner", n, aboveMean, aboveAll)
	}
	for i := 0; i < p.TileCount(); i++ {
		r := radius(geo.centreDisp[i])
		if r > ReliefMax+1e-5 || r < 1-1e-5 {
			t.Fatalf("tile %d radius %v outside [1, ReliefMax]", i, r)
		}
		if p.Water[i] == planet.Ocean && math.Abs(float64(r-1)) > 1e-5 {
			t.Fatalf("ocean tile %d radius %v, want flat 1", i, r)
		}
	}
	for c, ts := range geo.cornerTiles {
		for _, tl := range ts {
			if tl >= 0 && p.Water[tl] == planet.Ocean && math.Abs(float64(radius(geo.cornerDisp[c])-1)) > 1e-5 {
				t.Fatalf("corner %d on the ocean is at radius %v: coasts must meet at sea level", c, radius(geo.cornerDisp[c]))
			}
		}
	}
	if ReliefMax < 1.025 || ReliefMax > 1.035 {
		t.Fatalf("ReliefMax %v", ReliefMax)
	}
}

// TestGlobeSunFacingSlopeBrighter: with the sun upper left, a face that
// tilts toward it is drawn brighter than one tilting away, and flat ground
// facing the eye shades as the sphere did.
func TestGlobeSunFacingSlopeBrighter(t *testing.T) {
	t.Parallel()
	g := New(globeTestPlanet(t, 100), nil)
	g.Sw, g.Sh = 1280, 720
	g.Reduced = true
	i := reliefTile(t, g)
	centreNow(g, i)
	g.Cam.SetDist(1.5)
	g.setupFrame()
	p, geo := g.P, g.geom
	// Light the tile's faces as Build does and compare the most sun-facing
	// with the least, by the direction each face leans.
	lo, hi := float32(2), float32(-1)
	var loLean, hiLean float32
	c := p.Center[i]
	for o := p.CornerOff[i]; o < p.CornerOff[i+1]; o++ {
		n := geo.faceN[o]
		d := max(0, n[0]*g.sun[0]+n[1]*g.sun[1]+n[2]*g.sun[2])
		s := lightAmbient + lightDiffuse*d
		// lean: the face normal's tilt off the radial, along the sun's
		// direction on the ground
		dot := n[0]*c[0] + n[1]*c[1] + n[2]*c[2]
		tx, ty, tz := n[0]-dot*c[0], n[1]-dot*c[1], n[2]-dot*c[2]
		lean := tx*g.sun[0] + ty*g.sun[1] + tz*g.sun[2]
		if s < lo {
			lo, loLean = s, lean
		}
		if s > hi {
			hi, hiLean = s, lean
		}
	}
	if hi-lo < 0.1 {
		t.Fatalf("mountain faces light %v..%v: slopes do not read", lo, hi)
	}
	if hiLean <= 0 || loLean >= hiLean {
		t.Fatalf("the bright face leans %v toward the sun, the dark one %v", hiLean, loLean)
	}
	// The drawn mesh agrees: the tile's centre vertices (one per face, at
	// the screen centre) carry different light.
	g.Build()
	minB, maxB := float32(9), float32(-1)
	for k := 0; k+2 < g.NV; k += 3 {
		v := g.Verts[k] // a face's first vertex is its tile centre
		if math.Abs(float64(v.DstX)-640) < 2 && math.Abs(float64(v.DstY)-360) < 2 {
			b := v.ColorR + v.ColorG + v.ColorB
			minB, maxB = min(minB, b), max(maxB, b)
		}
	}
	if maxB <= minB {
		t.Fatalf("drawn faces of the peak all share a brightness (%v..%v)", minB, maxB)
	}
	// Flat ground facing the eye straight on: within rounding of the old
	// sphere shading (ambient + diffuse on the sphere normal).
	k1, _ := Shade(0, 0, 1)
	flat := float32(lightAmbient + lightDiffuse*globeSun[2])
	if math.Abs(float64(k1-flat)) > 1e-4 {
		t.Fatalf("flat shade %v, sphere shade %v", flat, k1)
	}
}

// TestGlobeSharedCornersIdentical: tiles sharing a corner place it at the
// same relief point, and every drawn copy of a corner lands on the same
// screen position, so the relief cannot crack.
func TestGlobeSharedCornersIdentical(t *testing.T) {
	t.Parallel()
	g := New(globeTestPlanet(t, 100), nil)
	g.Sw, g.Sh = 1280, 720
	p, geo := g.P, g.geom
	for i := 0; i < p.TileCount(); i++ {
		for o := p.CornerOff[i]; o < p.CornerOff[i+1]; o++ {
			for _, nb := range p.Neighbours(i) {
				for q := p.CornerOff[nb]; q < p.CornerOff[nb+1]; q++ {
					a, b := p.Corners[o], p.Corners[q]
					if math.Abs(float64(a[0]-b[0]))+math.Abs(float64(a[1]-b[1]))+math.Abs(float64(a[2]-b[2])) > 1e-4 {
						continue
					}
					if geo.cornerDisp[geo.cornerID[o]] != geo.cornerDisp[geo.cornerID[q]] {
						t.Fatalf("tiles %d and %d place a shared corner apart", i, nb)
					}
				}
			}
		}
	}
	i := reliefTile(t, g)
	centreNow(g, i)
	g.Cam.SetDist(1.4)
	g.Build()
	seen := map[[2]float32]int{}
	for k := 0; k < g.NV; k++ {
		seen[[2]float32{g.Verts[k].DstX, g.Verts[k].DstY}]++
	}
	// Each corner of the centred tile is in two faces of each of its three tiles.
	for o := p.CornerOff[i]; o < p.CornerOff[i+1]; o++ {
		sx, sy, _ := g.cc.at(geo.cornerID[o], &g.xform, geo)
		if n := seen[[2]float32{sx, sy}]; n != 6 {
			t.Fatalf("corner drawn %d times at one position, want 6 (3 tiles x 2 faces)", n)
		}
	}
}

// TestGlobeSnowOnHighColdPeaks: in Biome mode a peak above its snowline
// turns toward the snow colour, a warm lowland does not, and the ice sheet
// is all snow.
func TestGlobeSnowOnHighColdPeaks(t *testing.T) {
	t.Parallel()
	g := New(globeTestPlanet(t, 100), nil)
	p, geo := g.P, g.geom
	snow := PalRGB("snow")
	dist := func(a, b rgb) float32 {
		return gfx.Abs32(a[0]-b[0]) + gfx.Abs32(a[1]-b[1]) + gfx.Abs32(a[2]-b[2])
	}
	high, low, ice := 0, 0, 0
	for i := 0; i < p.TileCount(); i++ {
		if !(p.Water[i] == planet.Land) {
			continue
		}
		if p.Biome[i] == biome.IceSheet {
			if geo.ccol[i] != snow {
				t.Fatalf("ice sheet tile %d not snow: %v", i, geo.ccol[i])
			}
			ice++
			continue
		}
		if !isLand(p, i) {
			continue
		}
		sl := geo.snowAt[i]
		switch {
		case geo.centreH[i] >= sl+snowBandM:
			if geo.ccol[i] != snow {
				t.Fatalf("peak %d at %v m (snowline %v) is %v, not snow", i, geo.centreH[i], sl, geo.ccol[i])
			}
			high++
		case geo.centreH[i] < sl && p.Temp[i] > 150:
			if dist(geo.ccol[i], g.Base[i]) != 0 {
				t.Fatalf("warm lowland %d snowed", i)
			}
			low++
		}
	}
	if high == 0 || low == 0 || ice == 0 {
		t.Fatalf("snowy peaks %d, warm lowland %d, ice sheet %d", high, low, ice)
	}
	// Snow is a Biome-mode colour; data views show their data.
	g.SetMode(ModeTemperature)
	for i := 0; i < p.TileCount(); i++ {
		if isLand(p, i) && geo.centreH[i] >= geo.snowAt[i]+snowBandM && p.Biome[i] != biome.IceSheet {
			if geo.ccol[i] != g.Base[i] {
				t.Fatalf("temperature mode snowed tile %d", i)
			}
			break
		}
	}
}

func TestGlobeGridAndBare(t *testing.T) {
	t.Parallel()
	g := New(globeTestPlanet(t, 100), nil)
	g.Sw, g.Sh = 1280, 720
	centreNow(g, landTile(t, g))
	g.Cam.SetDist(2.0)
	g.Build()
	plain := len(g.Ov)
	if g.Grid() {
		t.Fatal("grid on by default")
	}
	g.SetGrid(true)
	g.Build()
	if !g.Grid() || len(g.Ov) <= plain+400 {
		t.Fatalf("grid adds %d overlay vertices", len(g.Ov)-plain)
	}
	g.Cam.SetDist(gridDist + 0.3) // too far out: no grid
	g.Build()
	far := len(g.Ov)
	g.SetGrid(false)
	g.Build()
	if len(g.Ov) != far {
		t.Fatalf("grid drawn beyond Dist %v", gridDist)
	}
	if len(g.geom.Coast) == 0 {
		t.Fatal("no coastline built")
	}
	g.SetGrid(true)
	g.Bare()
	if g.Grid() || len(g.geom.Coast)+len(g.geom.Rivers)+len(g.geom.Roads)+len(g.Marks) != 0 {
		t.Fatal("Bare kept lines or marks")
	}
}

// TestGlobeRoadsByClass: every road class has a style; a highway is an
// asphalt road 1.6x as wide; paths only show from zoom 1.0; the test world
// draws every class.
func TestGlobeRoadsByClass(t *testing.T) {
	t.Parallel()
	g := New(globeTestPlanet(t, 30), nil)
	g.Sw, g.Sh = 1280, 720
	leg := RoadLegend()
	for c, e := range leg {
		if e.Label == "" || e.Width <= 0 {
			t.Fatalf("road class %d legend %+v", c, e)
		}
		if _, ok := WorldPalette[roadStyles[c].key]; !ok {
			t.Fatalf("road class %d colour %q not in the palette", c, roadStyles[c].key)
		}
	}
	if a, h := roadStyles[planet.AncientAsphaltRoad], roadStyles[planet.AncientHighway]; h.body != a.body*highwayWide || !h.centre || h.key != a.key {
		t.Fatalf("highway %+v vs asphalt %+v", h, a)
	}
	seen := map[uint8]bool{}
	path := -1
	for k, s := range g.geom.Roads {
		seen[s.w] = true
		if s.w == uint8(planet.Path) && path < 0 {
			path = k
		}
	}
	if len(seen) != int(planet.NumRoadClasses) || path < 0 {
		t.Fatalf("road classes on the test world: %v", seen)
	}
	// Look at a path and draw only it.
	r := g.P.Roads[path]
	centreNow(g, int(r[0]))
	g.Cam.SetDist(1.5)
	g.setupFrame()
	g.geom.Roads = g.geom.Roads[path : path+1]
	g.Ov, g.Oi = g.Ov[:0], g.Oi[:0]
	g.geom.strokeRoads(&g.xform, &g.ovMesh, pathZoom-0.1, 1)
	if len(g.Ov) != 0 {
		t.Fatalf("path drawn below zoom %v", pathZoom)
	}
	g.geom.strokeRoads(&g.xform, &g.ovMesh, pathZoom, 1)
	if len(g.Ov) != 4*len(roadDots) {
		t.Fatalf("path drew %d vertices, want %d dots", len(g.Ov), len(roadDots))
	}
}

// TestGlobeRiversTaperAndEndAtShore: a river into the sea or a lake ends at
// the shore (no point at a water tile's centre); sources taper; creeks only
// show from zoom 1.0.
func TestGlobeRiversTaperAndEndAtShore(t *testing.T) {
	t.Parallel()
	g := New(globeTestPlanet(t, 100), nil)
	g.Sw, g.Sh = 1280, 720
	p, geo := g.P, g.geom
	mouths, tapers := 0, 0
	for i := range p.River {
		if p.River[i] != site.NoRiver && p.RiverTo[i] >= 0 && p.Water[p.RiverTo[i]] != planet.Land && p.Water[i] == planet.Land {
			mouths++
		}
	}
	for _, s := range geo.Rivers {
		for _, e := range [2][3]float32{s.a, s.b} {
			d := norm3(e)
			ti := p.TileAt(d)
			if ti >= 0 && p.Water[ti] != planet.Land {
				c := p.Center[ti]
				if d[0]*c[0]+d[1]*c[1]+d[2]*c[2] > 0.99999 {
					t.Fatalf("a river runs to the centre of water tile %d", ti)
				}
			}
		}
		if s.wa < s.wb {
			tapers++
		}
		if s.wb != riverWidth[s.w] {
			t.Fatalf("river size %d width %v", s.w, s.wb)
		}
	}
	if mouths == 0 || tapers == 0 {
		t.Fatalf("%d river mouths, %d tapered sources", mouths, tapers)
	}
	// Creeks hide below zoom 1.0.
	creek := -1
	for k, s := range geo.Rivers {
		if s.w == uint8(site.Creek) {
			creek = k
			break
		}
	}
	if creek < 0 {
		t.Fatal("no creek")
	}
	centreNow(g, p.TileAt(norm3(geo.Rivers[creek].a)))
	g.Cam.SetDist(1.5)
	g.setupFrame()
	geo.Rivers = geo.Rivers[creek : creek+1]
	g.Ov, g.Oi = g.Ov[:0], g.Oi[:0]
	g.geom.strokeRivers(&g.xform, &g.ovMesh, creekZoom-0.1, 1)
	if len(g.Ov) != 0 {
		t.Fatal("creek drawn below zoom 1.0")
	}
	g.geom.strokeRivers(&g.xform, &g.ovMesh, creekZoom, 1)
	if len(g.Ov) != 4 {
		t.Fatalf("creek drew %d vertices", len(g.Ov))
	}
	for _, e := range RiverLegend() {
		if e.Label == "" || e.Width <= 0 || e.Colour != WorldPalette["river"] {
			t.Fatalf("river legend %+v", e)
		}
	}
	for k, e := range HillLegend() {
		if e.Colour != WorldPalette[HillKey[site.Flat+site.Hills(k)]] || e.Label == "" {
			t.Fatalf("hill legend %+v", e)
		}
	}
}

// TestGlobeLabelsByZoomFacingAndCollision: oceans are named when zoomed
// out, ranges and lakes in the middle band, settlements close in; every
// label faces the eye, none overlap, at most 24.
func TestGlobeLabelsByZoomFacingAndCollision(t *testing.T) {
	t.Parallel()
	g := New(globeTestPlanet(t, 100), nil)
	g.Sw, g.Sh = 1280, 720
	if len(g.geom.feats) == 0 {
		t.Fatal("the v1 test world has no features")
	}
	rng := rand.New(rand.NewSource(7))
	kinds := map[string]bool{}
	for trial := 0; trial < 40; trial++ {
		g.Cam.Lat = (rng.Float64()*2 - 1) * 1.2
		g.Cam.Lon = (rng.Float64()*2 - 1) * math.Pi
		g.Cam.SetDist([]float64{1.5, 1.8, 2.6, 3.5}[trial%4])
		g.Build()
		ls := g.Labels()
		if len(ls) > maxLabels {
			t.Fatalf("%d labels", len(ls))
		}
		for a := range ls {
			for b := a + 1; b < len(ls); b++ {
				p, q := ls[a], ls[b]
				if p.X < q.X+q.W && q.X < p.X+p.W && p.Y < q.Y+q.H && q.Y < p.Y+p.H {
					t.Fatalf("labels %q and %q overlap", p.Text, q.Text)
				}
			}
			switch l := ls[a]; {
			case l.Settlement:
				if g.Cam.Dist > settleLabelD {
					t.Fatalf("settlement %q named at Dist %v", l.Text, g.Cam.Dist)
				}
				kinds["settlement"] = true
			case l.Ocean:
				if g.Cam.Dist < 2.4 {
					t.Fatalf("ocean %q named at Dist %v", l.Text, g.Cam.Dist)
				}
				kinds["ocean"] = true
			default:
				if g.Cam.Dist < 1.6 || g.Cam.Dist > 3.2 {
					t.Fatalf("feature %q named at Dist %v", l.Text, g.Cam.Dist)
				}
				kinds["feature"] = true
			}
		}
		for _, f := range g.geom.feats {
			for _, l := range ls {
				if l.Text == f.text && !l.Settlement {
					c := g.P.Center[f.tile]
					if c[0]*g.CamDir[0]+c[1]*g.CamDir[1]+c[2]*g.CamDir[2] <= labelFacing {
						t.Fatalf("%q labelled while turned away", f.text)
					}
				}
			}
		}
	}
	if !kinds["ocean"] || !kinds["feature"] || !kinds["settlement"] {
		t.Fatalf("label kinds seen: %v", kinds)
	}
	if s := spaced("the Salt Deep"); s != "T H E   S A L T   D E E P" {
		t.Fatalf("spaced %q", s)
	}
	g.Bare()
	g.Build()
	if len(g.Labels()) != 0 {
		t.Fatal("a bare globe still names things")
	}
}
