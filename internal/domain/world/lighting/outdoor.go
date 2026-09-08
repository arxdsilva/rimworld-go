package lighting

import "rimworld-go/internal/domain/sim/clock"

// Level is outdoor illumination in [0, 1]. Night stays slightly above
// zero so unlit outdoor tiles remain readable.
type Level float64

// Outdoor is the unobstructed outdoor light level at tick t.
func Outdoor(t clock.Tick) Level {
	return outdoorAtHour(clock.HourOfDay(t))
}

func outdoorAtHour(h float64) Level {
	stops := [...][2]float64{
		{0, 0.12},
		{5, 0.12},
		{6.5, 0.45},
		{8, 0.85},
		{12, 1.00},
		{16, 0.92},
		{18, 0.50},
		{20, 0.20},
		{22, 0.12},
		{24, 0.12},
	}
	if h <= stops[0][0] {
		return Level(stops[0][1])
	}
	for i := 1; i < len(stops); i++ {
		if h <= stops[i][0] {
			span := stops[i][0] - stops[i-1][0]
			if span <= 0 {
				return Level(stops[i][1])
			}
			t := (h - stops[i-1][0]) / span
			return Level(stops[i-1][1] + (stops[i][1]-stops[i-1][1])*t)
		}
	}
	return Level(stops[len(stops)-1][1])
}
