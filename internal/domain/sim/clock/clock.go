package clock

import "fmt"

// Tick is the authoritative simulation time unit.
type Tick uint64

// Clock tracks authoritative simulation time. It never moves backward.
type Clock struct {
	tick Tick
}

// StartHour is the default colony-start hour (morning).
const StartHour = 8

// New returns a Clock starting at tick 0 (midnight of day 0).
func New() *Clock {
	return &Clock{}
}

// NewAt returns a Clock starting at t.
func NewAt(t Tick) *Clock {
	return &Clock{tick: t}
}

// StartingMorning is the tick for StartHour on day 0.
func StartingMorning() Tick {
	return Tick(StartHour * TicksPerHour)
}

// Now returns the current tick.
func (c *Clock) Now() Tick {
	return c.tick
}

// Advance moves the clock forward by n ticks. Negative n is a no-op.
func (c *Clock) Advance(n int) {
	if n <= 0 {
		return
	}
	c.tick += Tick(n)
}

// Date returns the calendar date for the clock's current tick.
func (c *Clock) Date() Date {
	return DateAt(c.tick)
}

// Calendar constants define the original rimworld-go time convention.
// A day has 24 hours, a quadrum (quarter-year) has 15 days, and a year
// has 4 quadrums.
const (
	TicksPerHour    = 1000
	HoursPerDay     = 24
	TicksPerDay     = TicksPerHour * HoursPerDay
	DaysPerQuadrum  = 15
	TicksPerQuadrum = TicksPerDay * DaysPerQuadrum
	QuadrumsPerYear = 4
	TicksPerYear    = TicksPerQuadrum * QuadrumsPerYear
)

// Date is a human-readable calendar date derived from an authoritative
// tick count. Year, Quadrum, Day, and Hour are 0-based. Minute is 0–59.
type Date struct {
	Year    int
	Quadrum int
	Day     int
	Hour    int
	Minute  int
}

// DateAt converts an authoritative tick into a calendar Date.
func DateAt(t Tick) Date {
	totalHours := int(t) / TicksPerHour
	hour := totalHours % HoursPerDay
	totalDays := totalHours / HoursPerDay
	day := totalDays % DaysPerQuadrum
	totalQuadrums := totalDays / DaysPerQuadrum
	quadrum := totalQuadrums % QuadrumsPerYear
	year := totalQuadrums / QuadrumsPerYear
	minute := int(t%TicksPerHour) * 60 / TicksPerHour
	return Date{Year: year, Quadrum: quadrum, Day: day, Hour: hour, Minute: minute}
}

// Period returns the named part of the day for this date.
func (d Date) Period() Period {
	return PeriodAt(d.Hour)
}

// HourOfDay is the fractional hour in [0, 24) for tick t.
func HourOfDay(t Tick) float64 {
	return float64(t%TicksPerDay) / float64(TicksPerHour)
}

// String renders the date with period and clock time.
func (d Date) String() string {
	return fmt.Sprintf("Year %d, Quadrum %d, Day %d, %s %02d:%02d",
		d.Year, d.Quadrum, d.Day, d.Period(), d.Hour, d.Minute)
}
