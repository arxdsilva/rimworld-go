package plant

import (
	"math/rand/v2"
	"testing"

	"rimworld-go/internal/domain/sim/clock"
)

// growTicks is the per-tick reference GrowSpan integrates: Grow's step at
// every growing-hour tick of t0+1 .. t1.
func growTicks(rate int, t0, t1 uint64) int {
	if rate <= 0 {
		return 0
	}
	f := uint64(rate)
	n := 0
	for t := t0 + 1; t <= t1; t++ {
		if GrowingHour(clock.DateAt(clock.Tick(t)).Hour) {
			n += int(t*f/100 - (t-1)*f/100)
		}
	}
	return n
}

// TestGrowSpanTelescopes: spans add up exactly (any split of a window
// grows the same as the whole window) and equal the per-tick steps.
func TestGrowSpanTelescopes(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewPCG(1, 2))
	for i := 0; i < 400; i++ {
		rate := r.IntN(260) - 10 // includes zero and negative rates
		a := r.Uint64N(20 * clock.TicksPerDay)
		b := a + r.Uint64N(3*clock.TicksPerDay)
		c := b + r.Uint64N(3*clock.TicksPerDay)
		whole := GrowSpan(rate, a, c)
		if got := GrowSpan(rate, a, b) + GrowSpan(rate, b, c); got != whole {
			t.Fatalf("rate %d: span(%d,%d)+span(%d,%d) = %d, span(%d,%d) = %d", rate, a, b, b, c, got, a, c, whole)
		}
		if i%8 == 0 {
			if want := growTicks(rate, a, c); whole != want {
				t.Fatalf("rate %d (%d,%d]: span %d, per tick %d", rate, a, c, whole, want)
			}
		}
	}
	// windows of 250 ticks across a whole day add up to the day
	day := uint64(3 * clock.TicksPerDay)
	sum := 0
	for t0 := day; t0 < day+clock.TicksPerDay; t0 += 250 {
		sum += GrowSpan(137, t0, t0+250)
	}
	if want := GrowSpan(137, day, day+clock.TicksPerDay); sum != want || want != growTicks(137, day, day+clock.TicksPerDay) {
		t.Fatalf("a day in 250-tick windows: %d, whole day %d", sum, want)
	}
	// nothing grows at night
	night := day + 21*clock.TicksPerHour
	if n := GrowSpan(100, night, night+2*clock.TicksPerHour); n != 0 {
		t.Fatalf("night growth %d", n)
	}
	// the same growth as stepping Grow tick by tick (below the cap)
	p := Plant{Species: Tree}
	q := p
	for t := day + 6*clock.TicksPerHour - 100; t <= day+6*clock.TicksPerHour+900; t++ {
		if GrowingHour(clock.DateAt(clock.Tick(t)).Hour) {
			p = p.Grow(140, t)
		}
	}
	q = q.AddGrowth(GrowSpan(140, day+6*clock.TicksPerHour-101, day+6*clock.TicksPerHour+900))
	if p.Growth != q.Growth || p.Growth == 0 {
		t.Fatalf("Grow per tick %d, AddGrowth(GrowSpan) %d", p.Growth, q.Growth)
	}
	if ripe := (Plant{Species: Radish}).AddGrowth(1 << 30); ripe.Growth != DefOf(Radish).GrowTicks {
		t.Fatalf("AddGrowth caps at ripe: %d", ripe.Growth)
	}
}
