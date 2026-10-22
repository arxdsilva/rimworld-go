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
