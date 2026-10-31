package plant

import (
	"testing"

	"rimworld-go/internal/domain/sim/clock"
)

// TestColdThresholdPerPlant (TEMP-2, wiki: Plants): each plant's cold
// threshold is fixed by its id, between -10 and -18 °C, and spread over
// the whole range.
func TestColdThresholdPerPlant(t *testing.T) {
	t.Parallel()
	if ColdWarmest != -100 || ColdColdest != -180 || LeafRegrow != 60000 {
		t.Fatalf("wiki numbers: %d %d %d", ColdWarmest, ColdColdest, LeafRegrow)
	}
	seen := map[int]bool{}
	lo, hi := 0, -1000
	for id := uint64(1); id <= 2000; id++ {
		th := ColdThreshold(id)
		if th > -100 || th < -180 || th != ColdThreshold(id) {
			t.Fatalf("plant %d: threshold %d", id, th)
		}
		seen[th] = true
		lo, hi = min(lo, th), max(hi, th)
	}
	if len(seen) != 81 || lo != -180 || hi != -100 {
		t.Fatalf("%d thresholds from %d to %d over 2000 plants, want all 81 from -180 to -100", len(seen), lo, hi)
	}
}

// TestColdKillsCropsAtOnce: a crop survives air at its own threshold and
// dies in the first window below it, without the hours of frost damage;
// below -18 °C no crop survives.
func TestColdKillsCropsAtOnce(t *testing.T) {
	t.Parallel()
	for id := uint64(1); id <= 50; id++ {
		th := ColdThreshold(id)
		p, _ := Plant{Species: Rice}.Expose(id, th, 0, step)
		if p.Dead() || p.Leafless(step) {
			t.Fatalf("rice %d at its threshold %d: %+v", id, th, p)
		}
		q, started := Plant{Species: Rice}.Expose(id, th-1, 0, step)
		if !q.Dead() || q.DamageHeat || started {
			t.Fatalf("rice %d just below %d: %+v started %v", id, th, q, started)
		}
		if r, _ := (Plant{Species: Corn}).Expose(id, -181, 0, step); !r.Dead() {
			t.Fatalf("corn %d survived -18.1 C", id)
		}
	}
}

// TestColdResistantCropLosesLeaves: healroot (wiki: cold-resistant) loses
// its leaves below its threshold instead of dying, and dies of the damage
// only below -29 °C.
func TestColdResistantCropLosesLeaves(t *testing.T) {
	t.Parallel()
	p, _ := Plant{Species: Healroot}.Expose(7, -250, 0, step)
	if p.Dead() || p.Damage != 0 || !p.Leafless(step) {
		t.Fatalf("healroot at -25 C: %+v", p)
	}
	if lim, ok := p.ColdLimit(); !ok || lim != -290 {
		t.Fatalf("healroot cold limit %d %v, want -290", lim, ok)
	}
	q, _ := exposeFor(Plant{Species: Healroot}, -291, 0, 10*clock.TicksPerHour)
	if !q.Dead() {
		t.Fatalf("healroot below -29 C: %+v", q)
	}
	if _, ok := (Plant{Species: Rice}).ColdLimit(); ok {
		t.Fatal("a plain crop has no damage limit (it dies at once)")
	}
}

// TestLeavesRegrowInADay: a wild plant below its threshold loses its
// leaves (no damage) and gets them back 24 h after the hour of the last
// cold window; below -19 °C every plant that lives is leafless.
func TestLeavesRegrowInADay(t *testing.T) {
	t.Parallel()
	const id = 11
	th := ColdThreshold(id)
	p, _ := Plant{Species: Tree}.Expose(id, th-1, 0, step)
	if p.Damage != 0 || p.Bare != clock.TicksPerHour+LeafRegrow || !p.Leafless(LeafRegrow) || p.Leafless(clock.TicksPerHour+LeafRegrow) {
		t.Fatalf("tree below its threshold: %+v", p)
	}
	if q, _ := p.Expose(id, 50, step, 2*step); q.Bare != p.Bare {
		t.Fatalf("warm air brought the leaves back early: %+v", q)
	}
	for s := Species(0); s < numSpecies; s++ {
		for seed := uint64(1); seed <= 20; seed++ {
			q, _ := Plant{Species: s}.Expose(seed, -190, 0, step)
			if !q.Dead() && !q.Leafless(step) {
				t.Fatalf("%v (%d) kept its leaves at -19 C", s, seed)
			}
		}
	}
	// growth resumes only above 0 °C once the leaves are back
	if TempFactor(0) != 0 || TempFactor(1) == 0 {
		t.Fatalf("growth at 0 / 0.1 C: %d %d", TempFactor(0), TempFactor(1))
	}
}

// TestLeafLossNoFlap: air hovering either side of a plant's threshold
// keeps it leafless (the timer is re-armed, never cleared); it comes back
// into leaf exactly once, a day after the cold stops.
func TestLeafLossNoFlap(t *testing.T) {
	t.Parallel()
	const id = 23
	th := ColdThreshold(id)
	p := Plant{Species: Pine}
	flips, was := 0, false
	at := uint64(0)
	for i := 0; i < 2*clock.TicksPerDay/step; i++ { // two days hovering
		p, _ = p.Expose(id, th-1+2*(i%2), at, at+step)
		at += step
		if l := p.Leafless(at); l != was {
			flips++
			was = l
		}
	}
	for i := 0; i < 2*clock.TicksPerDay/step; i++ { // two mild days
		p, _ = p.Expose(id, 80, at, at+step)
		at += step
		if l := p.Leafless(at); l != was {
			flips++
			was = l
		}
	}
	if flips != 2 || p.Leafless(at) || p.Damage != 0 {
		t.Fatalf("hovering at the threshold: %d flips, %+v", flips, p)
	}
}

// TestWildPlantsSurviveTheirHomeWinters (TEMP-2): every wild plant's cold
// limit (original tuning) lies below the wiki winter minimum of the
// biomes it grows wild in, with a blizzard's 5 °C on top, so it only
// loses its leaves at home; a jungle tree still dies in a -25 C winter.
func TestWildPlantsSurviveTheirHomeWinters(t *testing.T) {
	t.Parallel()
	home := map[Species]int{ // the coldest home winter, tenths
		Pine: -500, Shrub: -500, Moss: -500, Reeds: -350, Bramble: -350, Dandelion: -250, Duskmoss: -500,
		Cactus: -250, Agave: -250, BallCactus: -250, JungleTree: 0, ElephantEar: 0, FireLily: 0,
	}
	for s, winter := range home {
		if k := DefOf(s).ColdKill; k != 0 && k >= winter-50 {
			t.Errorf("%v dies at %d in its own %d winter", s, k, winter)
		}
	}
	if k := DefOf(JungleTree).ColdKill; k <= -250 || k == 0 {
		t.Errorf("a jungle tree should die in a temperate winter (limit %d)", k)
	}
}
