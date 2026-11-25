// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package temperature

import (
	"fmt"

	"rimworld-go/internal/domain/sim/clock"
)

// Temp is air temperature in tenths of a degree Celsius (215 = 21.5 C).
type Temp int

// C builds a Temp from whole degrees.
func C(deg int) Temp { return Temp(deg * 10) }

// Whole is t in whole degrees, rounded to the nearest (half away from 0).
func (t Temp) Whole() int {
	d := int(t)
	if d < 0 {
		d -= 5
	} else {
		d += 5
	}
	return d / 10
}

// String renders whole degrees, e.g. "-3C" or "18C".
func (t Temp) String() string { return fmt.Sprintf("%dC", t.Whole()) }

// Climate is a map's outdoor temperature curve (SIM-023: one per biome). The
// year is a triangle wave between a mid-summer peak and a mid-winter trough;
// each day adds a swing that is coolest just before dawn and warmest
// mid-afternoon. All values are tenths of a degree, PeakDay a day of year.
type Climate struct {
	YearMean  int
	Amplitude int
	PeakDay   int
	DaySwing  int
	Calendar  clock.CalendarPolicy
}

// Temperate is the temperate-forest calibration (the original map, TEMP-2
// wiki: Biomes): summer afternoons up to 35 C and winter nights down to
// -25 C, a 5 C annual mean (inside the wiki's 0..25 C average), +-24 C
// over the year and +-6 C between night and afternoon (the day swing is
// original tuning: unpublished). The colony lands in late spring at a
// 5 C daily mean.
var Temperate = Climate{YearMean: 50, Amplitude: 240, PeakDay: 15, DaySwing: 60}

const (
	peakDay     = 15 // the calendar's seasons centre on this day (Season)
	coldestHour = 5
	warmestHour = 15
)

// WithCalendar preserves the peak's seasonal phase across policies.
func (c Climate) WithCalendar(p clock.CalendarPolicy) Climate {
	if c.Calendar == p {
		return c
	}
	old := c.Calendar.YearDays()
	peak := (c.PeakDay%old + old) % old
	c.PeakDay = int(int64(peak) * int64(p.YearDays()) / int64(old))
	c.Calendar = p
	return c
}

// DailyMean is the outdoor mean for the day containing tick t.
func (c Climate) DailyMean(t clock.Tick) Temp {
	year := c.Calendar.YearDays()
	day := int(t/clock.TicksPerDay) % year
	half := year / 2
	peak := (c.PeakDay%year + year) % year
	dist := (day - peak + year) % year
	if dist > half {
		dist = year - dist
	}
	// dist in [0, half]: +amplitude at the peak, -amplitude half a year away
	return Temp(c.YearMean + c.Amplitude - 2*c.Amplitude*dist/half)
}

// Outdoor is the outdoor air temperature at tick t.
func (c Climate) Outdoor(t clock.Tick) Temp {
	return c.DailyMean(t) + c.diurnal(int(t%clock.TicksPerDay))
}

// Indoor is what an enclosed room with no heating or cooling settles
// around: the day's outdoor mean (TEMP-1: its walls and roof smooth the
// day's swing; there is no shelter bonus in the wiki's model).
func (c Climate) Indoor(t clock.Tick) Temp { return c.DailyMean(t) }

// Range is the coldest and warmest outdoor air of the year (winter night,
// summer afternoon).
func (c Climate) Range() (low, high Temp) {
	return Temp(c.YearMean - c.Amplitude - c.DaySwing), Temp(c.YearMean + c.Amplitude + c.DaySwing)
}

// GrowingDays counts the days of the year whose mean is above min.
func (c Climate) GrowingDays(min Temp) int {
	y := c.Calendar.YearDays()
	half := y / 2
	if Temp(c.YearMean+c.Amplitude) <= min {
		return 0
	}
	if Temp(c.YearMean-c.Amplitude) > min {
		return y
	}
	if c.Amplitude <= 0 {
		return 0
	}
	// Temperature at distance d is peak - floor(2*A*d/half).
	// Count the peak and the pairs at distances whose floored drop fits.
	drop := int64(c.YearMean + c.Amplitude - int(min))
	d := (drop*int64(half) - 1) / int64(2*c.Amplitude)
	return 1 + 2*int(d)
}

// diurnal is the offset from the daily mean at tick-of-day tod: -DaySwing
// at coldestHour rising linearly to +DaySwing at warmestHour and back.
func (c Climate) diurnal(tod int) Temp {
	cold := coldestHour * clock.TicksPerHour
	warm := warmestHour * clock.TicksPerHour
	rise := warm - cold
	fall := clock.TicksPerDay - rise
	p := (tod - cold + clock.TicksPerDay) % clock.TicksPerDay
	if p <= rise {
		return Temp(-c.DaySwing + 2*c.DaySwing*p/rise)
	}
	return Temp(c.DaySwing - 2*c.DaySwing*(p-rise)/fall)
}

// DailyMean, Outdoor and Indoor are the temperate climate's curve, for
// callers without a map (tests, tools).
func DailyMean(t clock.Tick) Temp { return Temperate.DailyMean(t) }

// Outdoor is Temperate.Outdoor.
func Outdoor(t clock.Tick) Temp { return Temperate.Outdoor(t) }

// Indoor is Temperate.Indoor.
func Indoor(t clock.Tick) Temp { return Temperate.Indoor(t) }

// Season names the time of year, centred on the temperature curve: summer
// is the 15 days around the warmest day, winter the 15 around the coldest.
// It is the legacy (northern, PeakDay 15) calendar; a map with its own
// climate uses Climate.SeasonAt.
func Season(t clock.Tick) string { return seasonAt(t, peakDay) }

// SeasonAt is Season for this climate: the seasons centre on c.PeakDay, so
// a southern-hemisphere site (PeakDay 45) has winter when the north has
// summer (NG-1 §4.2).
func (c Climate) SeasonAt(t clock.Tick) string { return seasonAtPolicy(t, c.PeakDay, c.Calendar) }

func seasonAt(t clock.Tick, peak int) string {
	return seasonAtPolicy(t, peak, clock.CalendarPolicy{})
}
func SeasonPolicy(t clock.Tick, p clock.CalendarPolicy) string {
	return Temperate.WithCalendar(p).SeasonAt(t)
}
func seasonAtPolicy(t clock.Tick, peak int, p clock.CalendarPolicy) string {
	year, q := p.YearDays(), p.QuadrumDays()
	day := int(t/clock.TicksPerDay) % year
	peak = (peak%year + year) % year
	switch (day - peak + q/2 + year) % year / q {
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
