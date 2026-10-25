package landform

import (
	"math"
	"sync"
	"testing"

	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/internal/domain/world/site"
)

// Rock is everything at or above RockTau; the bands sit below it in order
// and hold their shares.
func TestRockMaskThresholds(t *testing.T) {
	c := newCtx(200, 200, 3, site.Site{Hills: site.LargeHills})
	runSteps(t, c, Elevation{}, RockMask{})
	f := c.F
	if !(f.RockTau > f.RoughTau && f.RoughTau > f.GravelTau && f.GravelTau > 0) {
		t.Fatalf("taus out of order: rock %v rough %v gravel %v", f.RockTau, f.RoughTau, f.GravelTau)
	}
	rough, gravel := 0, 0
	for i, e := range f.Elevation {
		if f.Rock[i] != (e >= f.RockTau) {
			t.Fatalf("cell %d: rock %v but elevation %v vs tau %v", i, f.Rock[i], e, f.RockTau)
		}
		switch {
		case e >= f.RockTau:
		case e >= f.RoughTau:
			rough++
		case e >= f.GravelTau:
			gravel++
		}
	}
	n := float64(len(f.Elevation))
	if s := float64(rough) / n; math.Abs(s-0.06) > 0.005 {
		t.Fatalf("rough band share %.3f, want 0.06", s)
	}
	if s := float64(gravel) / n; math.Abs(s-0.08) > 0.005 {
		t.Fatalf("gravel band share %.3f, want 0.08", s)
	}
}

var shareTarget = map[site.Hills]float64{site.Flat: 0.04, site.SmallHills: 0.10, site.LargeHills: 0.22, site.Mountainous: 0.42}

// genMaps runs the chain without caves on 200×200 maps of seeds
// from..from+n-1 in parallel (the steps never fail; a panic still fails
// the test).
func genMaps(from, n int, s func(seed uint64) site.Site) []*mapgen.Ctx {
	out := make([]*mapgen.Ctx, n)
	var wg sync.WaitGroup
	for k := range n {
		wg.Go(func() {
			seed := uint64(from + k)
			c := newCtx(200, 200, seed, s(seed))
			for _, st := range []mapgen.Step{Elevation{}, RockMask{}, Rocks{}} {
				_ = st.Run(c)
			}
			out[k] = c
		})
	}
	wg.Wait()
	return out
}

// shares is the final rock share of seeds 0..n-1 for s.
func shares(n int, s site.Site) []float64 {
	out := make([]float64, n)
	for k, c := range genMaps(0, n, func(uint64) site.Site { return s }) {
		out[k] = rockShare(c)
	}
	return out
}

// Every seed lands near its hilliness profile, and the profiles rise.
func TestRockShareByHills(t *testing.T) {
	t.Parallel()
	prev := 0.0
	for _, h := range []site.Hills{site.Flat, site.SmallHills, site.LargeHills, site.Mountainous} {
		sum, lo, hi := 0.0, 1.0, 0.0
		for seed, sh := range shares(20, site.Site{Hills: h}) {
			if math.Abs(sh-shareTarget[h]) > 0.35*shareTarget[h] {
				t.Errorf("hills %d seed %d: rock share %.3f, target %.2f", h, seed, sh, shareTarget[h])
			}
			sum += sh
			lo, hi = min(lo, sh), max(hi, sh)
		}
		mean := sum / 20
		if mean <= prev {
			t.Errorf("hills %d: mean share %.3f not above the previous %.3f", h, mean, prev)
		}
		if hi-lo < 0.1*shareTarget[h] {
			t.Errorf("hills %d: shares %.3f..%.3f barely vary between seeds", h, lo, hi)
		}
		prev = mean
	}
}

// The legacy default hilliness behaves like small hills.
func TestRockShareDefaultIsSmallHills(t *testing.T) {
	t.Parallel()
	sum := 0.0
	for _, sh := range shares(10, site.Site{}) {
		sum += sh
	}
	if m := sum / 10; math.Abs(m-0.10) > 0.03 {
		t.Fatalf("default hills mean share %.3f, want ~0.10", m)
	}
}

// largest returns the largest 4-connected rock component's size, its
// centroid relative to the map centre and the total rock.
func largest(c *mapgen.Ctx) (size int, cx, cy float64, total int) {
	label, sizes := mapgen.Components(c.W, c.H, c.F.Rock, false)
	best := -1
	for k, s := range sizes {
		total += s
		if best < 0 || s > sizes[best] {
			best = k
		}
	}
	if best < 0 {
		return 0, 0, 0, 0
	}
	for i, l := range label {
		if int(l) == best {
			x, y := c.XY(i)
			cx += float64(x) + 0.5 - float64(c.W)/2
			cy += float64(y) + 0.5 - float64(c.H)/2
		}
	}
	return sizes[best], cx / float64(sizes[best]), cy / float64(sizes[best]), total
}

// Mountainous maps are one dominant mass on the uphill side.
func TestMountainousOneMassUphill(t *testing.T) {
	t.Parallel()
	maps := genMaps(0, 12, func(seed uint64) site.Site {
		return site.Site{Hills: site.Mountainous, Uphill: site.Dir8(seed % 8), HasUphill: true}
	})
	for seed, c := range maps {
		d := site.Dir8(seed % 8)
		size, cx, cy, total := largest(c)
		if float64(size) < 0.55*float64(total) {
			t.Errorf("seed %d: largest rock body %d of %d", seed, size, total)
		}
		vx, vy := dirVec(d)
		if cx*vx+cy*vy <= 0 {
			t.Errorf("seed %d uphill %d: mass centroid (%.1f, %.1f) not uphill", seed, d, cx, cy)
		}
	}
	// Without a world uphill it is still one dominant mass.
	for k, c := range genMaps(20, 6, func(uint64) site.Site { return site.Site{Hills: site.Mountainous} }) {
		seed := 20 + k
		size, _, _, total := largest(c)
		if float64(size) < 0.55*float64(total) {
			t.Errorf("seed %d (no uphill): largest rock body %d of %d", seed, size, total)
		}
	}
}

// Flat maps get several scattered outcrops, not one blob.
func TestFlatScatteredOutcrops(t *testing.T) {
	t.Parallel()
	sum := 0
	for _, c := range genMaps(0, 10, func(uint64) site.Site { return site.Site{Hills: site.Flat} }) {
		_, sizes := mapgen.Components(200, 200, c.F.Rock, false)
		sum += len(sizes)
	}
	if avg := float64(sum) / 10; avg < 3 {
		t.Fatalf("flat maps average %.1f outcrops, want >= 3", avg)
	}
}

// Sea ice has no rock at all.
func TestRockMaskSeaIce(t *testing.T) {
	c := newCtx(100, 100, 1, site.Site{Hills: site.Mountainous, Biome: biome.SeaIce})
	runSteps(t, c, Elevation{}, RockMask{})
	if c.F.RockTau <= 1 || c.F.RoughTau <= 1 || c.F.GravelTau <= 1 {
		t.Fatalf("sea ice taus %v %v %v must be above 1", c.F.RockTau, c.F.RoughTau, c.F.GravelTau)
	}
	if !(c.F.RockTau > c.F.RoughTau && c.F.RoughTau > c.F.GravelTau) {
		t.Fatal("sea ice taus out of order")
	}
	if rockShare(c) != 0 {
		t.Fatal("sea ice has rock")
	}
}

func TestQuantilesSorted(t *testing.T) {
	v := []float32{5, 1, 4, 2, 3}
	q := quantiles(v, 0, 0.5, 1)
	if q[0] != 1 || q[1] != 3 || q[2] != 5 {
		t.Fatalf("quantiles = %v", q)
	}
	if q := quantiles(nil, 0.5); q[0] != 0 {
		t.Fatalf("empty quantile = %v", q)
	}
}
