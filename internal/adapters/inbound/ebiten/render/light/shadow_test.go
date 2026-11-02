package light

import (
	"math"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/world/roof"
)

// testLight is a hand-built light: shadows fall away from a light at
// azimuth az (screen space, 0 = east) and elevation el (radians), clamped
// at 3 tiles as LightAt does, with the given alpha.
func testLight(az, el, alpha float64) State {
	lx, ly := math.Cos(az)*math.Cos(el), 0.5*math.Sin(az)*math.Cos(el)
	n := math.Hypot(lx, ly)
	return State{
		SunL:      [3]float32{float32(lx), float32(ly), float32(math.Sin(el))},
		ShadowDir: [2]float64{-lx / n, -ly / n},
		ShadowLen: func(h float64) float64 {
			return math.Min(math.Max(h/math.Tan(el), 0.25*h), litShadowMaxLen)
		},
		ShadowAlpha: alpha,
	}
}

func quadCentre(q [4][2]float64) (float64, float64) {
	return (q[0][0] + q[1][0] + q[2][0] + q[3][0]) / 4, (q[0][1] + q[1][1] + q[2][1] + q[3][1]) / 4
}

// quadExtent is the quad's length along its texture-x axis (the shadow).
func quadExtent(q [4][2]float64) float64 {
	return math.Hypot(q[1][0]-q[0][0], q[1][1]-q[0][1])
}

// quadWidth is the quad's width across the shadow.
func quadWidth(q [4][2]float64) float64 {
	return math.Hypot(q[2][0]-q[0][0], q[2][1]-q[0][1])
}

// TestColonistShadowFollowsTheSun: the blob is centred away from the
// light, starts at the feet, is longer at dawn than at noon (clamped) and
// keeps its width; a scaled foot scales the blob.
func TestColonistShadowFollowsTheSun(t *testing.T) {
	t.Parallel()
	foot := Foot{X: 5.5, Y: 5.5, Scale: 1}
	for _, az := range []float64{0.3, 1.2, 2.5, 4} {
		l := testLight(az, 0.5, 0.3)
		cx, cy := quadCentre(ColonistQuad(foot, l))
		dx, dy := cx-foot.X, cy-foot.Y
		if dot := dx*l.ShadowDir[0] + dy*l.ShadowDir[1]; dot <= 0 {
			t.Fatalf("az %.1f: blob centre (%.2f,%.2f) is not on the shadow side %v", az, dx, dy, l.ShadowDir)
		}
		// the blob's near end is at the feet: half its depth behind them
		if q := ColonistQuad(foot, l); !near(quadExtent(q)/2-math.Hypot(dx, dy), colonistD/2, 1e-9) {
			t.Fatalf("az %.1f: blob does not start at the feet", az)
		}
	}
	dawn := testLight(0.1, 0.12, 0.3)
	noon := testLight(math.Pi/2, 65*math.Pi/180, 0.5)
	qd, qn := ColonistQuad(foot, dawn), ColonistQuad(foot, noon)
	if ld, ln := quadExtent(qd), quadExtent(qn); !(ld > 1.5*ln) || ld > colonistD+colonistMaxLen+1e-9 {
		t.Fatalf("blob length dawn %.2f noon %.2f (clamp %.2f)", ld, ln, colonistD+colonistMaxLen)
	}
	if !near(quadWidth(qd), colonistW, 1e-9) || !near(quadWidth(qn), colonistW, 1e-9) {
		t.Fatalf("blob width dawn %.2f noon %.2f, want %.2f", quadWidth(qd), quadWidth(qn), colonistW)
	}
	big := ColonistQuad(Foot{X: 5.5, Y: 5.5, Scale: 2}, noon)
	if !near(quadWidth(big), 2*colonistW, 1e-9) {
		t.Fatalf("a scaled foot's blob is %.2f wide", quadWidth(big))
	}
	// no light direction: a round blob at the feet
	flat := ColonistQuad(foot, State{})
	if cx, cy := quadCentre(flat); !near(cx, foot.X, 1e-9) || !near(cy, foot.Y, 1e-9) {
		t.Fatalf("no light: blob at (%.2f,%.2f)", cx, cy)
	}
}

// TestColonistShadowFadesWithTheSun: darker at noon than at dawn, a
// faint blob under the moon, none without a light.
func TestColonistShadowFadesWithTheSun(t *testing.T) {
	t.Parallel()
	const day = 172 // summer
	noon := ColonistAlpha(LightAt(12, day, "clear", 0), day)
	dawn := ColonistAlpha(LightAt(6.8, day, "clear", 0), day)
	moon := ColonistAlpha(LightAt(0, day, "clear", 0), day)
	if !(noon > dawn && dawn > moon && moon > 0) {
		t.Fatalf("alpha noon %.3f dawn %.3f moon %.3f", noon, dawn, moon)
	}
	if moon > 0.2 {
		t.Fatalf("the moon's blob is not faint: %.3f", moon)
	}
	if a := ColonistAlpha(State{}, day); a != 0 {
		t.Fatalf("no light: alpha %.3f", a)
	}
	storm := ColonistAlpha(LightAt(12, day, "storm", 1), day)
	if !(storm < noon) {
		t.Fatalf("a storm does not fade the shadow: %.3f vs %.3f", storm, noon)
	}
}

// TestOnlyColonistsOutdoorsCast: a colonist on open ground casts; under a
// built or natural roof, as a prisoner, visitor, ally, guest, raider, a
// body or carried, nothing does. A planned roof is no roof yet.
func TestOnlyColonistsOutdoorsCast(t *testing.T) {
	t.Parallel()
	w := game.WorldView{TerrainW: 10, RoofCells: make([]uint8, 100)}
	w.RoofCells[3*10+3] = uint8(roof.Built)
	w.RoofCells[3*10+4] = uint8(roof.Natural)
	w.RoofCells[3*10+5] = uint8(roof.PlanBuild)
	col := game.PawnView{ID: 1, X: 1, Y: 1}
	if !Casts(&col, &w) {
		t.Fatal("a colonist outdoors casts no shadow")
	}
	for _, c := range []struct {
		name string
		edit func(*game.PawnView)
		want bool
	}{
		{"under a built roof", func(p *game.PawnView) { p.X, p.Y = 3, 3 }, false},
		{"under a natural roof", func(p *game.PawnView) { p.X, p.Y = 4, 3 }, false},
		{"under a planned roof", func(p *game.PawnView) { p.X, p.Y = 5, 3 }, true},
		{"lying outdoors", func(p *game.PawnView) { p.Lying = true }, true},
		{"prisoner", func(p *game.PawnView) { p.Prisoner = true }, false},
		{"visitor", func(p *game.PawnView) { p.Visitor = true }, false},
		{"ally", func(p *game.PawnView) { p.Ally = true }, false},
		{"guest", func(p *game.PawnView) { p.GuestFaction = "Hill Folk" }, false},
		{"raider", func(p *game.PawnView) { p.Hostile = true }, false},
		{"body", func(p *game.PawnView) { p.Embodied = true }, false},
		{"carried", func(p *game.PawnView) { p.CarriedBy = 9 }, false},
	} {
		p := col
		c.edit(&p)
		if got := Casts(&p, &w); got != c.want {
			t.Errorf("%s: casts %v, want %v", c.name, got, c.want)
		}
	}
	// no roof data: every tile is open sky
	bare := game.WorldView{TerrainW: 10}
	if !Casts(&col, &bare) {
		t.Fatal("no roof data: a colonist casts no shadow")
	}
}
