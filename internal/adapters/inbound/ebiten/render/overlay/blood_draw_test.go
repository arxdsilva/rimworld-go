package overlay

import (
	"math"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/render/light"
	"rimworld-go/internal/application/game"
)

func TestStainShapeByAmount(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		amount   float64
		shape    StainShape
		min, max float64
	}{
		{0.05, StainDrip, 0.5, 0.5},
		{0.19, StainDrip, 0.5, 0.5},
		{0.2, StainSplat, 0.55, 0.55},
		{0.4, StainSplat, 0.55, 0.9},
		{0.599, StainSplat, 0.89, 0.9},
		{0.6, StainPool, 0.8, 0.8},
		{0.8, StainPool, 0.8, 1.1},
		{1, StainPool, 1.1, 1.1},
	} {
		shape, size := StainShapeOf(c.amount)
		if shape != c.shape || size < c.min-1e-9 || size > c.max+1e-9 {
			t.Errorf("amount %v: shape %v size %v, want %v in [%v, %v]", c.amount, shape, size, c.shape, c.min, c.max)
		}
	}
	_, a := StainShapeOf(0.3)
	_, b := StainShapeOf(0.5)
	if a >= b {
		t.Fatalf("splats grow with amount: %v then %v", a, b)
	}
}

func TestStainAlphaIsAmount(t *testing.T) {
	t.Parallel()
	for _, a := range []float64{0, 0.1, 0.5, 1} {
		l := StainLookOf(game.StainView{X: 3, Y: 4, Kind: "blood", Amount: a})
		if math.Abs(float64(l.Cs[3])-a*0.85) > 1e-6 {
			t.Fatalf("amount %v: alpha %v", a, l.Cs[3])
		}
	}
	if l := StainLookOf(game.StainView{Amount: 3}); l.Cs[3] > 0.85+1e-6 {
		t.Fatalf("alpha capped at 0.85: %v", l.Cs[3])
	}
}

func rgb8(c [3]float32) [3]int {
	return [3]int{int(math.Round(float64(c[0]) * 255)), int(math.Round(float64(c[1]) * 255)), int(math.Round(float64(c[2]) * 255))}
}

func TestStainDriesDarker(t *testing.T) {
	t.Parallel()
	if c := rgb8(StainColour("blood", 0)); c != [3]int{0x7a, 0x14, 0x14} {
		t.Fatalf("fresh blood %x", c)
	}
	if c := rgb8(StainColour("blood", StainDryTicks)); c != [3]int{0x4a, 0x1d, 0x1a} {
		t.Fatalf("dried blood %x", c)
	}
	if StainColour("blood", 10*StainDryTicks) != StainColour("blood", StainDryTicks) {
		t.Fatal("dry after a day")
	}
	prev := float32(2)
	for age := uint64(0); age <= StainDryTicks; age += StainDryTicks / 8 {
		l := light.Lum3(StainColour("blood", age))
		if l > prev+1e-6 {
			t.Fatalf("darkens as it dries: %v after %v at %d", l, prev, age)
		}
		prev = l
	}
	// a stain's look carries the colour (the tint), premultiplied later
	l := StainLookOf(game.StainView{Kind: "blood", Amount: 0.5, AgeTicks: StainDryTicks})
	if rgb8([3]float32{l.Cs[0], l.Cs[1], l.Cs[2]}) != [3]int{0x4a, 0x1d, 0x1a} {
		t.Fatalf("look colour %v", l.Cs)
	}
}

func TestIchorColour(t *testing.T) {
	t.Parallel()
	if c := rgb8(StainColour("ichor", 0)); c != [3]int{0xa8, 0xa0, 0x30} {
		t.Fatalf("fresh ichor %x", c)
	}
	if c := rgb8(StainColour("ichor", StainDryTicks)); c != [3]int{0x5c, 0x5a, 0x1c} {
		t.Fatalf("dried ichor %x", c)
	}
}

func TestCustom3Encoding(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		cov   float64
		wet   int
		ichor bool
	}{{0.1, 0, false}, {0.75, 15, false}, {0.5, 7, true}, {0.95, 15, true}, {0.01, 1, false}} {
		w := EncodeBlood(c.cov, c.wet, c.ichor)
		if want := float64(32*b2i(c.ichor)+2*c.wet) + c.cov; math.Abs(float64(w)-want) > 1e-5 {
			t.Fatalf("%+v: w %v, want %v", c, w, want)
		}
		cov, wet, ichor := DecodeBlood(w)
		if math.Abs(cov-c.cov) > 1e-5 || wet != c.wet || ichor != c.ichor {
			t.Fatalf("%+v round trip: %v %v %v", c, cov, wet, ichor)
		}
	}
	if EncodeBlood(0, 15, true) != 0 || EncodeBlood(-1, 3, false) != 0 {
		t.Fatal("no coverage: no blood")
	}
	if cov, wet, _ := DecodeBlood(EncodeBlood(2, 99, false)); cov >= 1 || wet != 15 {
		t.Fatalf("clamped: %v %v", cov, wet)
	}
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

func TestCoverageFromHealthAndWound(t *testing.T) {
	t.Parallel()
	near := func(a, b float64) bool { return math.Abs(a-b) < 1e-9 }
	if c := PawnBloodCoverage(game.PawnView{Health: 100}); c != 0 {
		t.Fatalf("healthy: %v", c)
	}
	if c := PawnBloodCoverage(game.PawnView{Health: 50}); !near(c, 0.4) {
		t.Fatalf("half health: %v", c)
	}
	if c := PawnBloodCoverage(game.PawnView{Health: 50, Wounded: true}); !near(c, 0.6) {
		t.Fatalf("open wound: %v", c)
	}
	if c := PawnBloodCoverage(game.PawnView{Health: 0, Wounded: true}); !near(c, 0.75) {
		t.Fatalf("capped: %v", c)
	}
	if c := AnimalBloodCoverage(game.AnimalView{Health: 70}); !near(c, 0.3) {
		t.Fatalf("animal: %v", c)
	}
	if c := AnimalBloodCoverage(game.AnimalView{Health: 5}); !near(c, 0.75) {
		t.Fatalf("animal capped: %v", c)
	}
	if !IsHiveSpecies("hive_beetle") || !IsHiveSpecies("hive_brute") || !IsHiveSpecies("hive_crawler") || IsHiveSpecies("muffalo") {
		t.Fatal("hive species")
	}
}

func TestBloodMaskGrowsWithCoverage(t *testing.T) {
	t.Parallel()
	share := func(cov float32) float64 {
		var sum float64
		n := 0
		for y := 0; y < 96; y++ {
			for x := 0; x < 96; x++ {
				sum += float64(BloodMask([2]float32{float32(x) + 0.5, float32(y) + 0.5}, cov))
				n++
			}
		}
		return sum / float64(n)
	}
	if s := share(0); s != 0 {
		t.Fatalf("no coverage: %v", s)
	}
	prev := 0.0
	for _, c := range []float32{0.1, 0.2, 0.35, 0.5, 0.75, 0.95} {
		s := share(c)
		if s < prev {
			t.Fatalf("coverage %v: share %v below %v", c, s, prev)
		}
		prev = s
	}
	if s := share(0.2); s <= 0 || s > 0.3 {
		t.Fatalf("light wounds: a few blotches, got %v", s)
	}
	if s := share(0.75); s < 0.5 {
		t.Fatalf("near death: mostly covered, got %v", s)
	}
	for x := float32(0); x < 64; x += 0.7 {
		if m := BloodMask([2]float32{x, x * 1.3}, 0.5); m < 0 || m > 1 {
			t.Fatalf("mask %v", m)
		}
	}
}
