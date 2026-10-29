package ebiten

import (
	globepkg "rimworld-go/internal/adapters/inbound/ebiten/globe"
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"sync"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/globemath"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/world/planet"
)

var (
	globePlanetMu sync.Mutex
	globePlanets  = map[int]*planet.Planet{}
)

// globeTestPlanet is the seed "amber-hollow" world at the coverage, built
// once per test binary (the globe package's tests keep their own copy).
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
func testGlobe(t testing.TB) *globepkg.View {
	t.Helper()
	g := globepkg.New(globeTestPlanet(t, 30), nil)
	g.Sw, g.Sh = 1280, 720
	g.Reduced = true
	return g
}

func centreNow(g *globepkg.View, i int) {
	c := g.P.Center[i]
	g.Cam.PanX, g.Cam.PanY = 0, 0
	g.Cam.CentreOn(globemath.Vec3{float64(c[0]), float64(c[1]), float64(c[2])}, 0)
}

func landTile(t testing.TB, g *globepkg.View) int {
	t.Helper()
	for i := 0; i < g.P.TileCount(); i++ {
		if ok, _ := g.P.ValidLanding(i); ok {
			return i
		}
	}
	t.Fatal("no landable tile")
	return -1
}

func TestGlobeInputFromFrame(t *testing.T) {
	t.Parallel()
	raw := inputframe.Raw{Mx: 30, My: 40, WheelY: 2, Held: []string{"W"}, Pressed: []string{"Home"}}
	raw.Press[inputframe.Left], raw.Down[inputframe.Left], raw.Release[inputframe.Right] = true, true, true
	f := inputframe.NewFrame(raw)
	in := globeInputFrom(&f, 1280, 720, true)
	if in.MX != 30 || in.MY != 40 || in.Wheel != 2 || !in.Press[inputframe.Left] || !in.Down[inputframe.Left] || !in.Release[inputframe.Right] ||
		!in.Blocked || in.W != 1280 || in.H != 720 || len(in.Held) != 1 || len(in.Keys) != 1 || in.Shift {
		t.Fatalf("globeInputFrom = %+v", in)
	}
}
