package clock

import "fmt"

// Tick is the authoritative simulation time unit.
type Tick uint64

// Clock tracks authoritative simulation time. It never moves backward.
type Clock struct {
	tick Tick
}

// New returns a Clock starting at tick 0.
func New() *Clock {
	return &Clock{}
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
// tick count. All fields are 0-based.
type Date struct {
	Year    int
	Quadrum int
	Day     int
	Hour    int
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
	return Date{Year: year, Quadrum: quadrum, Day: day, Hour: hour}
}

// String renders the date as "Year Y, Quadrum Q, Day D, Hour H".
func (d Date) String() string {
	return fmt.Sprintf("Year %d, Quadrum %d, Day %d, Hour %d", d.Year, d.Quadrum, d.Day, d.Hour)
}
