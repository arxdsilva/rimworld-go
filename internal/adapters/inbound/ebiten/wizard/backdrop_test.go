package wizard

import (
	"sync"
	"testing"

	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/world/planet"
)

var (
	testPlanetMu sync.Mutex
	testPlanets  = map[int]*planet.Planet{}
)

// testPlanet is the seed "amber-hollow" world at the coverage, built once
// per test binary.
func testPlanet(tb testing.TB, coverage int) *planet.Planet {
	tb.Helper()
	testPlanetMu.Lock()
	defer testPlanetMu.Unlock()
	if p, ok := testPlanets[coverage]; ok {
		return p
	}
	p := planet.Build(planet.Params{SeedText: "amber-hollow", Coverage: coverage, Rain: 2, Temp: 2, Pop: 1,
		Factions: faction.DefaultSlots()})
	testPlanets[coverage] = p
	return p
}

func TestBackdropShowcaseBuildsAcrossFrames(t *testing.T) {
	ResetBackdrop()
	t.Cleanup(ResetBackdrop)
	b := &Backdrop
	calls := 0
	for b.DiscPlanet(nil) == nil {
		calls++
		if calls > 20000 {
			t.Fatal("showcase never finished")
		}
	}
	if calls < 5 {
		t.Fatalf("showcase built in %d calls: not spread over frames", calls)
	}
	p := b.DiscPlanet(nil)
	if p.TileCount() == 0 || p.Params.SeedText != BackdropShowcase.SeedText {
		t.Fatalf("showcase planet %+v", p.Params)
	}
	if b.DiscPlanet(nil) != p {
		t.Fatal("showcase rebuilt")
	}
	t.Logf("showcase generated in %d frames", calls)
}

func TestBackdropPrefersTheGivenPlanet(t *testing.T) {
	ResetBackdrop()
	t.Cleanup(ResetBackdrop)
	p := testPlanet(t, 30)
	if got := Backdrop.DiscPlanet(p); got != p {
		t.Fatal("ignored the wizard's planet")
	}
	if Backdrop.ShowBuilder != nil {
		t.Fatal("started the showcase although a planet exists")
	}
}

func TestBackdropDiscFitsItsRender(t *testing.T) {
	t.Parallel()
	// The halo (1.28 radii) must fit in the offscreen render.
	if r := DiscSilhouette(); 1.28*r > BackdropDiscArea/2 {
		t.Fatalf("silhouette %.1f px: halo clipped in a %d px render", r, BackdropDiscArea)
	}
}
