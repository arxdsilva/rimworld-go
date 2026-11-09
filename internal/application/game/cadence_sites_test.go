package game

import (
	"bytes"
	"testing"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/pkg/randx"
)

// RaceOn reports the race detector to the external tests (the day-tick
// budget skips under it).
const RaceOn = raceOn

// TestStaggeredSitesKeepTheirRate: spread over the ticks of their period
// (game/cadence), incubation and the pens' daily leave roll give every
// entity exactly what the old all-at-once rule gave over the same two
// whole days. The world is held still (the clock stays put, so the
// temperature does too) and only the sites run, once for every tick of
// the window.
func TestStaggeredSitesKeepTheirRate(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 3, Width: 120, Height: 30})
	bareGround(g)
	warmDay(t, g)
	g.paintHome(5, 5, 8, 8, true)
	egg, _ := g.SpawnStack(item.FertileEgg, 2, 20, 9)
	st, _ := g.stacks.Get(egg)
	st.Animal, st.Tame = uint8(animal.Chicken), true
	g.putStack(egg, st)
	cow := tameAt(t, g, "cow", "Bess", 100, 15)
	g.pen.since = map[entity.EntityID]uint64{cow: 0} // unpenned for days, far from Home
	if air := int(g.temperatureAt(20, 9)); air < animal.HatchMin || air > animal.HatchMax {
		t.Fatalf("not a warm day: %d", air)
	}
	rng0, _ := g.rng.MarshalBinary()

	const from, span = 10 * clock.TicksPerDay, 2 * clock.TicksPerDay
	for now := uint64(from); now < from+span; now++ {
		g.incubate(now)
		g.penUnpenned(now)
		g.animals.Ref(cow).Leaving = false // it may roll again tomorrow
	}

	periods := span / climateInterval
	if got := g.stacks.Ref(egg).Hatch; got != periods*climateInterval {
		t.Errorf("the egg incubated %d, want %d", got, periods*climateInterval)
	}
	if n := draws(t, rng0, g.rng); n != span/clock.TicksPerDay { // old: every midnight
		t.Errorf("%d leave rolls in two days, want %d", n, span/clock.TicksPerDay)
	}
}

// draws counts the IntN draws that took a stream from state from to r's.
func draws(t *testing.T, from []byte, r *randx.Rand) int {
	t.Helper()
	want, _ := r.MarshalBinary()
	c := randx.New(0)
	if err := c.UnmarshalBinary(from); err != nil {
		t.Fatal(err)
	}
	for n := 0; n < 100; n++ {
		if b, _ := c.MarshalBinary(); bytes.Equal(b, want) {
			return n
		}
		c.IntN(100)
	}
	t.Fatal("the stream moved more than 100 draws")
	return -1
}
