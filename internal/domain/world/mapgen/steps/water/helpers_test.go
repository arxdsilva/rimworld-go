package water

import (
	"math"
	"slices"
	"testing"

	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/internal/domain/world/site"
)

// fixture is a bare context: every input field zero, no rock.
func fixture(seed uint64, w, h int, s site.Site) *mapgen.Ctx {
	if s.Biome == 0 && s.Hills == 0 {
		s.Hills = site.Flat
	}
	return mapgen.NewCtx(mapgen.Config{Seed: seed, W: w, H: h, Site: s, Biome: biome.DefOf(s.Biome)})
}

func run(t testing.TB, c *mapgen.Ctx, steps ...mapgen.Step) {
	t.Helper()
	for _, s := range steps {
		if err := s.Run(c); err != nil {
			t.Fatalf("%s: %v", s.Name(), err)
		}
	}
}

// all runs the four water steps in pipeline order.
func all(t testing.TB, c *mapgen.Ctx) {
	run(t, c, Coast{}, River{}, Lakes{}, Wetness{})
}

func dirs(ds ...site.Dir8) site.Dir8Set {
	var s site.Dir8Set
	for _, d := range ds {
		s |= 1 << d
	}
	return s
}

func sameFields(a, b *mapgen.Fields) bool {
	return slices.Equal(a.Water, b.Water) && slices.Equal(a.CoastDist, b.CoastDist) &&
		slices.Equal(a.RiverDist, b.RiverDist) && slices.Equal(a.Wetness, b.Wetness) &&
		slices.Equal(a.River, b.River) && slices.Equal(a.Lake, b.Lake) &&
		slices.Equal(a.Coast, b.Coast) && slices.Equal(a.Rock, b.Rock) &&
		slices.Equal(a.RiverLine, b.RiverLine)
}

// onBorder reports whether cell i lies on map edge d (corners count for
// both cardinal edges; a diagonal edge is its corner's two edge stretches).
func onBorder(c *mapgen.Ctx, i int, d site.Dir8) bool {
	x, y := c.XY(i)
	n, e, s, w := y == 0, x == c.W-1, y == c.H-1, x == 0
	q := min(c.W, c.H) / 3
	switch d {
	case 0:
		return n
	case 2:
		return e
	case 4:
		return s
	case 6:
		return w
	case 1:
		return (n && x >= c.W-1-q) || (e && y <= q)
	case 3:
		return (s && x >= c.W-1-q) || (e && y >= c.H-1-q)
	case 5:
		return (s && x <= q) || (w && y >= c.H-1-q)
	default:
		return (n && x <= q) || (w && y <= q)
	}
}

func count(c *mapgen.Ctx, f func(i int) bool) int {
	n := 0
	for i := 0; i < c.W*c.H; i++ {
		if f(i) {
			n++
		}
	}
	return n
}

func dist2(c *mapgen.Ctx, i, j int) float64 {
	x0, y0 := c.XY(i)
	x1, y1 := c.XY(j)
	return math.Hypot(float64(x0-x1), float64(y0-y1))
}
