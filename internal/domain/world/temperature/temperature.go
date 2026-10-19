package temperature

import (
	"fmt"

	"rimworld-go/internal/domain/sim/clock"
)

// Temp is air temperature in tenths of a degree Celsius (215 = 21.5 C).
type Temp int

// C builds a Temp from whole degrees.
func C(deg int) Temp { return Temp(deg * 10) }

// String renders whole degrees, e.g. "-3C" or "18C".
func (t Temp) String() string {
	d := int(t)
	if d < 0 {
		d -= 5
	} else {
		d += 5
	}
	return fmt.Sprintf("%dC", d/10)
}

// Temperate-climate calibration. The year is a triangle wave between a
// mid-summer peak and a mid-winter trough; each day adds a swing that is
// coolest just before dawn and warmest mid-afternoon. The colony lands in
// late spring, so the first crops get a mild start and winter arrives
// around day 34.
const (
	yearMean      = 90  // 9 C annual mean
	yearAmplitude = 150 // +-15 C: summer days ~30 C, winter nights ~-12 C
	peakDay       = 15  // day of year (0-based) with the warmest mean
	daySwing      = 60  // +-6 C between night and afternoon
	coldestHour   = 5
	warmestHour   = 15
	// IndoorBonus is how much warmer an enclosed, unheated room keeps than
	// the day's outdoor mean (body heat, shelter from wind); rooms also
	// don't swing between day and night.
	IndoorBonus = 30
)

// DailyMean is the outdoor mean for the day containing tick t.
func DailyMean(t clock.Tick) Temp {
	day := int(t/clock.TicksPerDay) % clock.DaysPerYear
	half := clock.DaysPerYear / 2
	dist := (day - peakDay + clock.DaysPerYear) % clock.DaysPerYear
	if dist > half {
		dist = clock.DaysPerYear - dist
	}
	// dist in [0, half]: +amplitude at the peak, -amplitude half a year away
	return Temp(yearMean + yearAmplitude - 2*yearAmplitude*dist/half)
}

// Outdoor is the outdoor air temperature at tick t.
func Outdoor(t clock.Tick) Temp {
	return DailyMean(t) + diurnal(int(t%clock.TicksPerDay))
}

// Indoor is the temperature of an enclosed room with no heating or cooling.
func Indoor(t clock.Tick) Temp { return DailyMean(t) + IndoorBonus }

// diurnal is the offset from the daily mean at tick-of-day tod: -daySwing at
// coldestHour rising linearly to +daySwing at warmestHour and back.
func diurnal(tod int) Temp {
	cold := coldestHour * clock.TicksPerHour
	warm := warmestHour * clock.TicksPerHour
	rise := warm - cold
	fall := clock.TicksPerDay - rise
	p := (tod - cold + clock.TicksPerDay) % clock.TicksPerDay
	if p <= rise {
		return Temp(-daySwing + 2*daySwing*p/rise)
	}
	return Temp(daySwing - 2*daySwing*(p-rise)/fall)
}

// Season names the time of year, centred on the temperature curve: summer
// is the 15 days around the warmest day, winter the 15 around the coldest.
func Season(t clock.Tick) string {
	day := int(t/clock.TicksPerDay) % clock.DaysPerYear
	q := clock.DaysPerQuadrum
	switch (day - peakDay + q/2 + clock.DaysPerYear) % clock.DaysPerYear / q {
	case 0:
		return "Summer"
	case 1:
		return "Fall"
	case 2:
		return "Winter"
	default:
		return "Spring"
	}
}
