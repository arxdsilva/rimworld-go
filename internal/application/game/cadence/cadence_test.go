package cadence

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

func TestPeriodsMatchTheClock(t *testing.T) {
	if hour != clock.TicksPerHour || day != clock.TicksPerDay {
		t.Fatalf("hour %d day %d, clock %d %d", hour, day, clock.TicksPerHour, clock.TicksPerDay)
	}
}

// Each yields exactly the ids Due accepts, ascending, for every tick of a
// period and several offsets and ends.
func TestEachIsDue(t *testing.T) {
	for _, every := range []uint64{1, 7, 250, 2500} {
		for _, off := range []uint64{0, 1, 83, every - 1, day - 125} {
			for _, end := range []entity.EntityID{0, 1, 2, 300, 5001} {
				for now := uint64(0); now < 2*every+3; now++ {
					var want []entity.EntityID
					for id := entity.EntityID(1); id < end; id++ {
						if Due(now, every, off, id) {
							want = append(want, id)
						}
					}
					got := slices.Collect(Each(now, every, off, end))
					if !slices.Equal(got, want) {
						t.Fatalf("Each(%d, %d, %d, %d) = %v, want %v", now, every, off, end, got, want)
					}
				}
			}
		}
	}
}

// Every id is visited exactly once per period, so over 120 000 ticks each
// keeps its exact rate, and no tick visits more than end/every+1 ids.
func TestEachExactRate(t *testing.T) {
	const ticks, end = 120000, 1000
	for _, every := range []uint64{250, 2500, 60000} {
		visits := make([]int, end)
		bound := int(end/every) + 1
		for now := uint64(0); now < ticks; now++ {
			n := 0
			for id := range Each(now, every, OffEgg, end) {
				visits[id]++
				n++
			}
			if n > bound {
				t.Fatalf("every %d: tick %d visits %d ids, bound %d", every, now, n, bound)
			}
		}
		for id := 1; id < end; id++ {
			if want := int(ticks / every); visits[id] != want {
				t.Fatalf("every %d: id %d visited %d times, want %d", every, id, visits[id], want)
			}
		}
	}
}

func TestEachStops(t *testing.T) {
	n := 0
	for range Each(0, 1, 0, 100) {
		n++
		if n == 3 {
			break
		}
	}
	if n != 3 {
		t.Fatalf("broke after %d", n)
	}
}

func TestNext(t *testing.T) {
	for _, every := range []uint64{1, 250, 2500} {
		for now := uint64(0); now < 3*every; now += 17 {
			for _, id := range []entity.EntityID{0, 1, 999} {
				n := Next(now, every, OffPot, id)
				if n < now || n >= now+every || !Due(n, every, OffPot, id) {
					t.Fatalf("Next(%d, %d, id %d) = %d", now, every, id, n)
				}
				for t2 := now; t2 < n; t2++ {
					if Due(t2, every, OffPot, id) {
						t.Fatalf("Next(%d, %d, id %d) = %d, but %d is due", now, every, id, n, t2)
					}
				}
			}
		}
	}
}

// TestCalendarSlots: no two Heavy whole-map passes share a tick, and the
// daily work inside an hourly pass falls on one of that pass's ticks.
func TestCalendarSlots(t *testing.T) {
	cal := Calendar()
	for now := uint64(0); now < day; now++ {
		var heavy []string
		for _, s := range cal {
			if !s.PerEntity && s.Cost == Heavy && Due(now, s.Every, s.Offset, 0) {
				heavy = append(heavy, s.Name)
			}
		}
		if len(heavy) > 1 {
			t.Fatalf("tick %d runs %v", now, heavy)
		}
	}
	byName := map[string]Site{}
	for _, s := range cal {
		if _, dup := byName[s.Name]; dup {
			t.Fatalf("site %q twice", s.Name)
		}
		byName[s.Name] = s
	}
	for inner, outer := range map[string]string{"aging": "medical", "rollover": "story", "pen-day": "pen"} {
		in, out := byName[inner], byName[outer]
		if in.Every%out.Every != 0 || in.Offset%out.Every != out.Offset%out.Every {
			t.Fatalf("%s (every %d, off %d) is not on %s's ticks (every %d, off %d)", inner, in.Every, in.Offset, outer, out.Every, out.Offset)
		}
	}
}
