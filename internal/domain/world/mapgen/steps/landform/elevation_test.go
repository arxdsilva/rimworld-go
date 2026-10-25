package landform

import (
	"math"
	"testing"

	"rimworld-go/internal/domain/world/site"
)

// Elevation is rank-normalised: quantiles are shares of the map.
func TestElevationRankNormalised(t *testing.T) {
	c := newCtx(100, 80, 1, site.Site{Hills: site.LargeHills})
	runSteps(t, c, Elevation{})
	lo, hi, above := float32(2), float32(-1), 0
	for _, e := range c.F.Elevation {
		lo, hi = min(lo, e), max(hi, e)
		if e >= 0.8 {
			above++
		}
	}
	if lo != 0 || hi != 1 {
		t.Fatalf("elevation range [%v, %v], want [0, 1]", lo, hi)
	}
	if share := float64(above) / float64(len(c.F.Elevation)); math.Abs(share-0.2) > 0.005 {
		t.Fatalf("share above 0.8 = %.3f, want 0.2", share)
	}
}

func TestElevationDeterministicPerSeed(t *testing.T) {
	s := site.Site{Hills: site.Mountainous}
	a, b, o := newCtx(80, 80, 5, s), newCtx(80, 80, 5, s), newCtx(80, 80, 6, s)
	runSteps(t, a, Elevation{})
	runSteps(t, b, Elevation{})
	runSteps(t, o, Elevation{})
	diff := 0
	for i := range a.F.Elevation {
		if a.F.Elevation[i] != b.F.Elevation[i] || a.F.MountainBias[i] != b.F.MountainBias[i] {
			t.Fatal("same seed, different elevation")
		}
		if a.F.Elevation[i] != o.F.Elevation[i] {
			diff++
		}
	}
	if diff < len(a.F.Elevation)/2 {
		t.Fatalf("other seed changed only %d cells", diff)
	}
}

// Flat maps have no mountain bias; hilly ones do.
func TestElevationBiasByHills(t *testing.T) {
	for h := site.HillsDefault; h <= site.Mountainous; h++ {
		c := newCtx(60, 60, 2, site.Site{Hills: h})
		runSteps(t, c, Elevation{})
		sum := 0.0
		for _, b := range c.F.MountainBias {
			if b < 0 || b > 1 {
				t.Fatalf("hills %d: bias %v outside [0, 1]", h, b)
			}
			sum += float64(b)
		}
		if h == site.Flat && sum != 0 {
			t.Fatalf("flat map has mountain bias %v", sum)
		}
		if h != site.Flat && sum == 0 {
			t.Fatalf("hills %d has no mountain bias", h)
		}
	}
}

// biasCentroid is the MountainBias-weighted centroid relative to the map
// centre, in cells.
func biasCentroid(w, h int, b []float32) (float64, float64) {
	var sx, sy, sw float64
	for i, v := range b {
		x, y := i%w, i/w
		sx += float64(v) * (float64(x) + 0.5 - float64(w)/2)
		sy += float64(v) * (float64(y) + 0.5 - float64(h)/2)
		sw += float64(v)
	}
	return sx / sw, sy / sw
}

// The bias leans toward Uphill (0 N is y = 0) and its front is never a
// straight line.
func TestElevationBiasFollowsUphill(t *testing.T) {
	for d := site.Dir8(0); d < 8; d++ {
		c := newCtx(150, 150, 9, site.Site{Hills: site.LargeHills, Uphill: d, HasUphill: true})
		runSteps(t, c, Elevation{})
		cx, cy := biasCentroid(150, 150, c.F.MountainBias)
		vx, vy := dirVec(d)
		if dot := cx*vx + cy*vy; dot < 20 {
			t.Fatalf("uphill %d: bias centroid (%.1f, %.1f) not toward the edge (dot %.1f)", d, cx, cy, dot)
		}
	}
	// North front: the row where the bias crosses 0.5 varies by column.
	c := newCtx(200, 200, 4, site.Site{Hills: site.Mountainous, Uphill: 0, HasUphill: true})
	runSteps(t, c, Elevation{})
	var rows []float64
	for x := 0; x < 200; x++ {
		for y := 0; y < 200; y++ {
			if c.F.MountainBias[y*200+x] < 0.5 {
				rows = append(rows, float64(y))
				break
			}
		}
	}
	mean, sd := 0.0, 0.0
	for _, r := range rows {
		mean += r
	}
	mean /= float64(len(rows))
	for _, r := range rows {
		sd += (r - mean) * (r - mean)
	}
	sd = math.Sqrt(sd / float64(len(rows)))
	if sd < 2 {
		t.Fatalf("bias front is too straight: row sd %.2f", sd)
	}
}

// Without a world uphill, hilly maps draw their own direction.
func TestElevationDrawsDirectionWithoutUphill(t *testing.T) {
	seen := map[[2]int]bool{}
	for seed := uint64(0); seed < 16; seed++ {
		c := newCtx(80, 80, seed, site.Site{Hills: site.Mountainous})
		runSteps(t, c, Elevation{})
		cx, cy := biasCentroid(80, 80, c.F.MountainBias)
		seen[[2]int{sign(cx, 5), sign(cy, 5)}] = true
	}
	if len(seen) < 3 {
		t.Fatalf("only %d bias directions over 16 seeds", len(seen))
	}
}

func sign(v, dead float64) int {
	switch {
	case v > dead:
		return 1
	case v < -dead:
		return -1
	}
	return 0
}

func TestDirVecUnit(t *testing.T) {
	want := [8][2]float64{{0, -1}, {1, -1}, {1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1}}
	for d := site.Dir8(0); d < 8; d++ {
		x, y := dirVec(d)
		if math.Abs(math.Hypot(x, y)-1) > 1e-9 || sign(x, 0) != int(want[d][0]) || sign(y, 0) != int(want[d][1]) {
			t.Fatalf("dirVec(%d) = (%v, %v)", d, x, y)
		}
	}
}
