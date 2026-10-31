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
	// ENG-003 added visitors, wanderer joins and party; PLAY-021 the solar
	// flare and the short circuit (the game vetoes those two without power
	// devices; this pure pick has no veto). MED-04 added flu, plague, gut
	// worms and muscle parasites (malaria and sleeping sickness keep to
	// warm and tropical lands). PLAY-024 added the trader caravan and the
	// orbital trader (the game vetoes both until it can host them).
	// FOOD-2 added crop blight (vetoed without blightable crops).
	if len(seen) != 17 {
		t.Fatalf("picked %v, want all 17 temperate nature events", seen)
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

func TestDiseaseWeightsFollowFrequency(t *testing.T) {
	now := uint64(20 * clock.TicksPerDay)
	var st State
	full := Total(Nature, st, now, biome.TemperateForest, nil)
	off := TotalAt(Nature, st, now, biome.TemperateForest, 0, nil)
	double := TotalAt(Nature, st, now, biome.TemperateForest, 200, nil)
	var dis int
	for _, d := range All() {
		if d.Disease && d.Allowed(st, now, biome.TemperateForest) {
			dis += d.Weight
		}
	}
	if dis == 0 || off != full-dis || double != full+dis {
		t.Fatalf("totals: full %d, off %d, double %d, disease weight %d", full, off, double, dis)
	}
	for roll := 0; roll < off; roll++ {
		if id, ok := PickAt(Nature, st, now, biome.TemperateForest, 0, roll, nil); !ok || DefOf(id).Disease {
			t.Fatalf("frequency 0 picked %v", id)
		}
	}
	if w := DefOf(Plague).WeightAt(10); w != 1 {
		t.Fatalf("a low frequency keeps a disease possible: weight %d", w)
	}
	if DefOf(HerdCrossing).WeightAt(0) != DefOf(HerdCrossing).Weight {
		t.Fatal("other events ignore the disease frequency")
	}
}

func TestDiseaseBiomesAndDays(t *testing.T) {
	now := uint64(20 * clock.TicksPerDay)
	for b := biome.ID(0); b < biome.NumBiomes; b++ {
		tropical := b == biome.TropicalRainforest || b == biome.TropicalSwamp
		if got := DefOf(SleepingSickness).Allowed(State{}, now, b); got != tropical {
			t.Fatalf("sleeping sickness in %v: %v", b, got)
		}
	}
	if DefOf(Malaria).Allowed(State{}, now, biome.TemperateForest) || !DefOf(Malaria).Allowed(State{}, now, biome.TropicalSwamp) {
		t.Fatal("malaria keeps to warm lands")
	}
	if DefOf(Flu).Allowed(State{}, 14*clock.TicksPerDay, biome.TemperateForest) || !DefOf(Flu).Allowed(State{}, 15*clock.TicksPerDay, biome.TemperateForest) {
		t.Fatal("diseases from day 15")
	}
}
