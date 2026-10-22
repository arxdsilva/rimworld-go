package incident

import (
	"testing"

	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/world/biome"
)

func TestPickDeterministic(t *testing.T) {
	now := uint64(20 * clock.TicksPerDay)
	var st State
	total := Total(Nature, st, now, biome.TemperateForest, nil)
	if total == 0 {
		t.Fatal("no nature events on day 20")
	}
	seen := map[ID]bool{}
	for roll := 0; roll < total; roll++ {
		a, ok1 := Pick(Nature, st, now, biome.TemperateForest, roll, nil)
		b, ok2 := Pick(Nature, st, now, biome.TemperateForest, roll, nil)
		if !ok1 || !ok2 || a != b || DefOf(a).Category != Nature {
			t.Fatalf("roll %d: %v %v / %v %v", roll, a, ok1, b, ok2)
		}
		seen[a] = true
	}
	if len(seen) != 5 {
		t.Fatalf("picked %v, want all 5 nature events", seen)
	}
}

func TestCooldownMinDayAndBiome(t *testing.T) {
	now := uint64(20 * clock.TicksPerDay)
	var st State
	st.Fired(HerdCrossing, now-2*clock.TicksPerDay)
	if DefOf(HerdCrossing).Allowed(st, now, biome.TemperateForest) {
		t.Fatal("herd crossing fired 2 days ago: still cooling down")
	}
	if !DefOf(HerdCrossing).Allowed(st, now+9*clock.TicksPerDay, biome.TemperateForest) {
		t.Fatal("after the cooldown it may fire again")
	}
	if DefOf(StrayJoins).Allowed(State{}, clock.TicksPerDay, biome.TemperateForest) {
		t.Fatal("too early")
	}
	if DefOf(HerdCrossing).Allowed(State{}, now, biome.SeaIce) || DefOf(TimbergnawGnawing).Allowed(State{}, now, biome.Desert) {
		t.Fatal("biome limits")
	}
	if _, ok := Pick(Nature, State{}, now, biome.TemperateForest, 0, func(ID) bool { return false }); ok {
		t.Fatal("caller veto")
	}
}

func TestNextNatureAt(t *testing.T) {
	now := uint64(3*clock.TicksPerDay + 17*clock.TicksPerHour)
	for gap := NatureGapMin; gap <= NatureGapMax; gap++ {
		for hour := 0; hour < clock.HoursPerDay; hour++ {
			at := NextNatureAt(now, gap, hour)
			if at <= now || at/clock.TicksPerDay != uint64(3+gap) || at%clock.TicksPerDay != uint64(hour*clock.TicksPerHour) {
				t.Fatalf("gap %d hour %d: tick %d", gap, hour, at)
			}
		}
	}
	if at := NextNatureAt(now, 0, 99); at <= now || at%clock.TicksPerDay != uint64(23*clock.TicksPerHour) {
		t.Fatalf("clamped: %d", at)
	}
}
