package temperature

import (
	"testing"

	"rimworld-go/internal/domain/sim/clock"
)

func at(day, hour int) clock.Tick {
	return clock.Tick(day*clock.TicksPerDay + hour*clock.TicksPerHour)
}

func TestSeasonsAndDays(t *testing.T) {
	if got := DailyMean(at(peakDay, 0)); got != yearMean+yearAmplitude {
		t.Errorf("peak mean = %v", got)
	}
	if got := DailyMean(at(peakDay+30, 0)); got != yearMean-yearAmplitude {
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
