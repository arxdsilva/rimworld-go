package temperature

import (
	"testing"

	"rimworld-go/internal/domain/sim/clock"
)

func at(day, hour int) clock.Tick {
	return clock.Tick(day*clock.TicksPerDay + hour*clock.TicksPerHour)
}

func TestSeasonsAndDays(t *testing.T) {
	if got := DailyMean(at(peakDay, 0)); got != Temp(Temperate.YearMean+Temperate.Amplitude) {
		t.Errorf("peak mean = %v", got)
	}
	if got := DailyMean(at(peakDay+30, 0)); got != Temp(Temperate.YearMean-Temperate.Amplitude) {
		t.Errorf("trough mean = %v", got)
	}
	if Outdoor(at(3, warmestHour)) <= Outdoor(at(3, coldestHour)) {
		t.Error("afternoon should be warmer than dawn")
	}
	if m := DailyMean(at(0, 0)); m < C(5) || m > C(15) {
		t.Errorf("start of game should be mild spring, got %v", m)
	}
	if Outdoor(at(peakDay+30, coldestHour)) >= 0 {
		t.Error("mid-winter nights should freeze")
	}
	if Indoor(at(10, 4)) <= Outdoor(at(10, 4)) {
		t.Error("rooms should be warmer than a spring night outside")
	}
}

func TestTempString(t *testing.T) {
	for in, want := range map[Temp]string{215: "22C", -34: "-3C", 0: "0C", 94: "9C"} {
		if got := in.String(); got != want {
			t.Errorf("%d: got %q want %q", in, got, want)
		}
	}
}

func TestSeasonNames(t *testing.T) {
	for day, want := range map[int]string{0: "Spring", peakDay: "Summer", peakDay + 15: "Fall", peakDay + 30: "Winter", 59: "Spring"} {
		if got := Season(at(day, 12)); got != want {
			t.Errorf("day %d: got %s want %s", day, got, want)
		}
	}
}

// The temperate climate is the original curve; a colder one is shifted down.
func TestClimateCurves(t *testing.T) {
	cold := Climate{YearMean: -80, Amplitude: 180, PeakDay: 15, DaySwing: 50}
	for _, tick := range []clock.Tick{0, 7 * clock.TicksPerDay, 40*clock.TicksPerDay + 3*clock.TicksPerHour} {
		if Outdoor(tick) != Temperate.Outdoor(tick) {
			t.Fatalf("Outdoor(%d) differs from Temperate.Outdoor", tick)
		}
	}
	if low, high := cold.Range(); low != -310 || high != 150 {
		t.Fatalf("cold range %v..%v", low, high)
	}
	if g, c := Temperate.GrowingDays(30), cold.GrowingDays(30); g <= c || c == 0 {
		t.Fatalf("growing days temperate %d, cold %d", g, c)
	}
}

func TestSeasonAtSouthernShift(t *testing.T) {
	north := Temperate
	south := Temperate
	south.PeakDay = (Temperate.PeakDay + 30) % clock.DaysPerYear // 45
	for d := 0; d < clock.DaysPerYear; d++ {
		tick := at(d, 12)
		if got, want := north.SeasonAt(tick), Season(tick); got != want {
			t.Fatalf("day %d: northern SeasonAt %q, legacy Season %q", d, got, want)
		}
	}
	opposite := map[string]string{"Summer": "Winter", "Winter": "Summer", "Spring": "Fall", "Fall": "Spring"}
	for d := 0; d < clock.DaysPerYear; d++ {
		tick := at(d, 6)
		if got, want := south.SeasonAt(tick), opposite[north.SeasonAt(tick)]; got != want {
			t.Fatalf("day %d: south %q, want %q (north %q)", d, got, want, north.SeasonAt(tick))
		}
	}
	if got := south.SeasonAt(at(45, 0)); got != "Summer" {
		t.Errorf("southern peak day 45 is %q, want Summer", got)
	}
	if got := south.SeasonAt(at(15, 0)); got != "Winter" {
		t.Errorf("southern day 15 is %q, want Winter", got)
	}
	// A negative or out-of-year PeakDay wraps like the calendar.
	if (Climate{PeakDay: -45}).SeasonAt(at(15, 0)) != "Summer" {
		t.Error("PeakDay -45 should wrap to day 15")
	}
}
