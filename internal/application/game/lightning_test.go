package game

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/world/roof"
	"rimworld-go/internal/domain/world/weather"
)

// stormStrikes runs the lightning schedule tick by tick for hours under
// kind and returns every strike.
func stormStrikes(g *Game, kind weather.Kind, hours int) []StrikeView {
	g.SetWeather(kind, 1)
	var out []StrikeView
	for i := 0; i < hours*clock.TicksPerHour; i++ {
		n := len(g.fire.rt.strikes)
		g.stepLightning()
		if len(g.fire.rt.strikes) > n {
			out = append(out, g.fire.rt.strikes[len(g.fire.rt.strikes)-1])
		}
		g.clock.Advance(1)
	}
	return out
}

func TestLightningStrikesOnlyInStorms(t *testing.T) {
	t.Parallel()
	for _, k := range []weather.Kind{weather.Clear, weather.Cloudy, weather.Rain, weather.Snow, weather.Blizzard} {
		g := fireGame(t, 40, 40)
		if s := stormStrikes(g, k, 24); len(s) != 0 {
			t.Errorf("%v: %d strikes", k, len(s))
		}
	}
}

func TestLightningEveryOneToThreeHours(t *testing.T) {
	t.Parallel()
	g := fireGame(t, 40, 40)
	s := stormStrikes(g, weather.Storm, 48)
	if len(s) < 20 || len(s) > 26 {
		t.Fatalf("%d strikes in 48 storm hours, want about 24", len(s))
	}
	for i := 1; i < len(s); i++ {
		if gap := s[i].Tick - s[i-1].Tick; gap < clock.TicksPerHour || gap > 3*clock.TicksPerHour {
			t.Fatalf("strikes %d ticks apart", gap)
		}
	}
	// the read model keeps only the last hour
	if n := len(g.fire.rt.strikes); n > 1 {
		t.Fatalf("%d strikes kept", n)
	}
}

func TestLightningDeterministic(t *testing.T) {
	t.Parallel()
	a, b := fireGame(t, 40, 40), fireGame(t, 40, 40)
	sa, sb := stormStrikes(a, weather.Storm, 24), stormStrikes(b, weather.Storm, 24)
	if !slices.Equal(sa, sb) || len(sa) == 0 {
		t.Fatalf("same seed, different strikes:\n%v\n%v", sa, sb)
	}
	c := New(Config{Seed: 8, Width: 40, Height: 40})
	if sc := stormStrikes(c, weather.Storm, 24); slices.Equal(sa, sc) {
		t.Fatal("another seed strikes the same cells at the same ticks")
	}
}

func TestLightningIgnitesFuelOutdoors(t *testing.T) {
	t.Parallel()
	g := fireGame(t, 40, 40)
	grassRect(g, 0, 0, 39, 39)
	g.roofs = make([]roof.Cell, 40*40)
	for y := 0; y < 40; y++ {
		for x := 0; x < 20; x++ {
			g.roofs[y*40+x] = roof.Built // the west half is roofed
		}
	}
	g.SetWeather(weather.Storm, 0.2) // light enough not to drench the fire at once
	lit := 0
	for i := 0; i < 12*clock.TicksPerHour; i++ {
		n := len(g.fire.rt.strikes)
		g.stepLightning()
		if len(g.fire.rt.strikes) > n {
			s := g.fire.rt.strikes[len(g.fire.rt.strikes)-1]
			if s.X < 20 {
				t.Fatalf("a strike under a roof at %d,%d", s.X, s.Y)
			}
			if g.fireAt(s.X, s.Y) != lightningFireSize {
				t.Fatalf("a strike on grass at %d,%d lit no fire", s.X, s.Y)
			}
			lit++
			g.douseFire(s.X, s.Y, 100)
		}
		g.clock.Advance(1)
	}
	if lit == 0 || !hasMessage(g, "Lightning started a fire") {
		t.Fatal("lightning fires are announced")
	}
	if w := g.World(); len(w.Strikes) == 0 && len(g.fire.rt.strikes) != 0 {
		t.Fatal("strikes reach the read model")
	}
	// on bare ground a strike lights nothing
	h := fireGame(t, 40, 40)
	for _, s := range stormStrikes(h, weather.Storm, 12) {
		if h.fireAt(s.X, s.Y) != 0 {
			t.Fatal("a strike on bare ground lit a fire")
		}
	}
}
