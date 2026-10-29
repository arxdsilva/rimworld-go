package needs

import (
	"testing"

	"rimworld-go/internal/domain/pawn/schedule"
	"rimworld-go/internal/domain/sim/clock"
)

func TestDecay_ReachesZero(t *testing.T) {
	p := Pair{Hunger: 3, Rest: 3}
	for i := 0; i < HungerPeriod*5; i++ {
		p = Decay(p, uint64(i+1))
	}
	if p.Hunger != 0 {
		t.Fatalf("hunger = %d", p.Hunger)
	}
}

func TestSleepTick_RestoresRest(t *testing.T) {
	p := Pair{Hunger: Full, Rest: 10}
	p = SleepTick(p, SleepPeriod)
	if p.Rest != 10+SleepGain {
		t.Fatalf("rest = %d", p.Rest)
	}
	if p = SleepTick(p, SleepPeriod+1); p.Rest != 10+SleepGain {
		t.Fatalf("rest between periods = %d, want unchanged", p.Rest)
	}
}

// A normal pawn sleeps from tired (20%) to full in eight in-game hours.
func TestSleepTakesEightHoursFrom20To100(t *testing.T) {
	p := Pair{Hunger: Full, Rest: CriticalRest}
	ticks := 0
	for now := uint64(1); p.Rest < Full; now++ {
		p = SleepTick(p, now)
		ticks++
	}
	if want := 8 * clock.TicksPerHour; ticks < want-SleepPeriod || ticks > want {
		t.Fatalf("20%%->100%% took %d ticks, want %d (8 h)", ticks, want)
	}
}

func TestSleepAllowed(t *testing.T) {
	for a, want := range map[schedule.Activity]bool{
		schedule.Work: false, schedule.Anything: true, schedule.Recreation: true, schedule.Sleep: true,
	} {
		if SleepAllowed(a) != want {
			t.Errorf("SleepAllowed(%v) = %v, want %v", a, !want, want)
		}
	}
}

func TestPercent(t *testing.T) {
	if Percent(Full) != 100 || Percent(0) != 0 {
		t.Fatal("percent bounds")
	}
}

// TestSleepTickAt: rest comes back in proportion to the bed's rest
// effectiveness, exactly over a long sleep; 1000 is the old SleepTick.
func TestSleepTickAt(t *testing.T) {
	for _, re := range []int{0, 800, 950, 1000, 1250, 1600} {
		p := Pair{Hunger: Full}
		for now := uint64(1); now <= 5000; now++ {
			p = SleepTickAt(p, now, re)
		}
		if want := 5000 * SleepGain * re / (SleepPeriod * 1000); p.Rest != want {
			t.Errorf("re %d: rest %d after 5000 ticks, want %d", re, p.Rest, want)
		}
		if p.Hunger != Full-5000/HungerPeriod {
			t.Errorf("re %d: hunger %d", re, p.Hunger)
		}
	}
	for now := uint64(0); now < 50; now++ {
		a, b := SleepTickAt(Pair{Rest: 10, Hunger: 10}, now, 1000), SleepTick(Pair{Rest: 10, Hunger: 10}, now)
		if a != b {
			t.Fatalf("tick %d: %+v vs %+v", now, a, b)
		}
	}
	if p := SleepTickAt(Pair{Rest: Full}, 7, 1600); p.Rest != Full {
		t.Fatalf("clamped: %d", p.Rest)
	}
}
