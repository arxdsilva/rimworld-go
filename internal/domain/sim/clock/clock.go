package clock

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
