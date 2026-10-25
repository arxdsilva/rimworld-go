package landform

import (
	"testing"

	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/internal/domain/world/roof"
	"rimworld-go/internal/domain/world/site"
)

// maskCtx is a w×h map whose rock mask is drawn from rows ('#' rock,
// 'c' cave under rock, '~' deep water, 'b' beach, '.' open).
func maskCtx(hills site.Hills, rows ...string) *mapgen.Ctx {
	c := newCtx(len(rows[0]), len(rows), 1, site.Site{Hills: hills})
	for y, r := range rows {
		for x, ch := range r {
			i := c.Idx(x, y)
			switch ch {
			case '#':
				c.F.Rock[i] = true
			case 'c':
				c.F.Rock[i], c.F.Cave[i] = true, true
			case '~':
				c.F.Rock[i], c.F.Water[i] = true, mapgen.Deep
			case 'b':
				c.F.Rock[i], c.F.Water[i] = true, mapgen.Beach
			}
		}
	}
	return c
}

func maskString(c *mapgen.Ctx) []string {
	out := make([]string, c.H)
	for y := range out {
		b := make([]byte, c.W)
		for x := range b {
			b[x] = '.'
			if c.F.Rock[c.Idx(x, y)] {
				b[x] = '#'
			}
		}
		out[y] = string(b)
	}
	return out
}

func wantMask(t *testing.T, c *mapgen.Ctx, want ...string) {
	t.Helper()
	got := maskString(c)
	for y := range want {
		if got[y] != want[y] {
			t.Fatalf("rock mask:\n%s\nwant:\n%s", join(got), join(want))
		}
	}
}

func join(rows []string) string {
	s := ""
	for _, r := range rows {
		s += r + "\n"
	}
	return s
}

// Caves, water and water bands are cut out of the rock.
func TestRocksCutCavesAndWater(t *testing.T) {
	c := maskCtx(site.LargeHills,
		"..........",
		".########.",
		".##cc~b##.",
		".########.",
		".########.",
		"..........",
	)
	runSteps(t, c, Rocks{})
	for i := range c.F.Rock {
		if c.F.Rock[i] && (c.F.Cave[i] || c.F.Water[i] != mapgen.Dry) {
			t.Fatalf("cell %d stays rock under a cave or water", i)
		}
	}
	if c.F.Rock[c.Idx(2, 2)] != true {
		t.Fatal("plain rock was removed")
	}
}

// Tiny bodies (≤ 5 cells) go; 6+ cell outcrops stay (also on flat maps).
func TestRocksDropTinyBodies(t *testing.T) {
	for _, h := range []site.Hills{site.Flat, site.Mountainous} {
		c := maskCtx(h,
			"............",
			".##.....###.",
			".##.....###.",
			"............",
			"............",
		)
		runSteps(t, c, Rocks{})
		wantMask(t, c,
			"............",
			"........###.",
			"........###.",
			"............",
			"............",
		)
	}
}

// Holes of ≤ 3 open cells inside rock fill; larger ones, holes at the map
// edge, cave and water cells stay open.
func TestRocksFillSmallHoles(t *testing.T) {
	c := maskCtx(site.LargeHills,
		"..............................",
		".#######.########.#######.###.",
		".###.###.##...###.##..###.#c#.",
		".#######.########.##..###.#~#.",
		".#######.########.#######.###.",
		"..............................",
	)
	runSteps(t, c, Rocks{})
	wantMask(t, c,
		"..............................",
		".#######.########.#######.###.",
		".#######.########.##..###.#.#.",
		".#######.########.##..###.#.#.",
		".#######.########.#######.###.",
		"..............................",
	)
	edge := maskCtx(site.LargeHills,
		"##.###",
		"######",
		"######",
	)
	runSteps(t, edge, Rocks{})
	wantMask(t, edge, "##.###", "######", "######")
}

// One-cell spikes are trimmed, twice.
func TestRocksTrimSpikes(t *testing.T) {
	c := maskCtx(site.LargeHills,
		"....................",
		".####.........####..",
		".#########....####..",
		".####........#####..",
		".####.........####..",
		"....................",
	)
	runSteps(t, c, Rocks{})
	// The left spike (5 long) loses two cells from the tip; the right spike
	// (1 long) goes in the first pass.
	wantMask(t, c,
		"....................",
		".####.........####..",
		".#######......####..",
		".####.........####..",
		".####.........####..",
		"....................",
	)
}

// RockDepth is the Chebyshev depth of the final rock.
func TestRocksDepth(t *testing.T) {
	c := maskCtx(site.LargeHills,
		".........",
		".#######.",
		".#######.",
		".#######.",
		".#######.",
		".#######.",
		".........",
	)
	runSteps(t, c, Rocks{})
	want := mapgen.ChebDepth(c.W, c.H, c.F.Rock)
	for i := range want {
		if c.F.RockDepth[i] != want[i] {
			t.Fatalf("RockDepth[%d] = %d, want %d", i, c.F.RockDepth[i], want[i])
		}
	}
	if c.F.RockDepth[c.Idx(4, 3)] != 3 {
		t.Fatalf("centre depth %d, want 3", c.F.RockDepth[c.Idx(4, 3)])
	}
}

// Natural roofs from depth 2, thick from 7; none on open cells or on cave
// mouths open to the sky; roofs elsewhere are left alone.
func TestRocksRoofs(t *testing.T) {
	const n = 25
	c := newCtx(n+4, n+4, 1, site.Site{Hills: site.Mountainous})
	fillRect(c, c.F.Rock, 2, 2, n+2, n+2, true)
	// A cave corridor from the west face to the middle.
	fillRect(c, c.F.Cave, 2, 14, 15, 15, true)
	c.R.Roof[c.Idx(0, 0)] = roof.Built
	runSteps(t, c, Rocks{})
	solid := make([]bool, len(c.F.Rock))
	for i := range solid {
		solid[i] = c.F.Rock[i] || c.F.Cave[i]
	}
	d := mapgen.ChebDepth(c.W, c.H, solid)
	thin, thick := 0, 0
	for i, r := range c.R.Roof {
		switch {
		case i == c.Idx(0, 0):
			if r != roof.Built {
				t.Fatal("roof on an open cell was overwritten")
			}
		case !solid[i] || d[i] < 2:
			if r != 0 {
				t.Fatalf("cell %d (depth %d) roofed %v", i, d[i], r)
			}
		case d[i] < 7:
			if r != roof.Natural {
				t.Fatalf("cell %d (depth %d) roof %v, want thin", i, d[i], r)
			}
			thin++
		default:
			if r != roof.Natural|roof.NaturalThick {
				t.Fatalf("cell %d (depth %d) roof %v, want thick", i, d[i], r)
			}
			thick++
		}
	}
	if thin == 0 || thick == 0 {
		t.Fatalf("thin %d thick %d roofs", thin, thick)
	}
	if mouth := c.Idx(2, 14); c.R.Roof[mouth] != 0 {
		t.Fatal("cave mouth open to the sky is roofed")
	}
	if inner := c.Idx(14, 14); c.R.Roof[inner] == 0 {
		t.Fatal("cave under rock has no roof")
	}
}
