package lighting

import (
	"testing"

	"rimworld-go/internal/domain/sim/clock"
)

func TestOutdoorNoonIsBrightest(t *testing.T) {
	t.Parallel()
	noon := Outdoor(clock.Tick(12 * clock.TicksPerHour))
	morning := Outdoor(clock.Tick(8 * clock.TicksPerHour))
	night := Outdoor(clock.Tick(2 * clock.TicksPerHour))
	dusk := Outdoor(clock.Tick(18 * clock.TicksPerHour))

	if noon < 0.99 {
		t.Fatalf("noon should be ~1, got %v", noon)
	}
	if !(night < dusk && dusk < morning && morning < noon) {
		t.Fatalf("expected night < dusk < morning < noon, got %v %v %v %v", night, dusk, morning, noon)
	}
}

func TestOutdoorNightHasFloor(t *testing.T) {
	t.Parallel()
	if got := Outdoor(0); got < 0.1 {
		t.Fatalf("night light %v is too dark to read tiles", got)
	}
}

func TestOutdoorIsDeterministic(t *testing.T) {
	t.Parallel()
	t0 := clock.Tick(8*clock.TicksPerHour + 250)
	if Outdoor(t0) != Outdoor(t0) {
		t.Fatal("Outdoor must be a pure function of tick")
	}
}
