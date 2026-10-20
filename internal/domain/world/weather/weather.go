// Package weather is the deterministic outdoor weather model (SIM-016):
// what falls from the sky, how hard, the wind, and what it leaves on the
// ground. It is pure: callers pass the seed, time and temperature in.
package weather

// Kind is the sky's state.
type Kind uint8

const (
	Clear Kind = iota
	Cloudy
	Rain
	Storm
	Snow
	Blizzard
)

var names = [...]string{"clear", "cloudy", "rain", "storm", "snow", "blizzard"}

func (k Kind) String() string {
	if int(k) < len(names) {
		return names[k]
	}
	return "clear"
}

// Falling reports rain or snow coming down.
func (k Kind) Falling() bool { return k == Rain || k == Storm || k == Snow || k == Blizzard }

// Frozen reports snow rather than rain.
func (k Kind) Frozen() bool { return k == Snow || k == Blizzard }

// State is the weather now. Intensity is 0..1 (how hard it falls or
// blows), WindDir radians (0 = blowing towards +x, screen east),
// WindSpeed tiles per second; SnowDepth and Wetness (0..1) are what has
// built up on open ground.
type State struct {
	Kind      Kind
	Target    Kind // the block's kind the state is easing towards
	Intensity float64
	WindDir   float64
	WindSpeed float64
	SnowDepth float64
	Wetness   float64
}

// Next is the weather for 6-hour block b, given the previous state, the
// day's mean outdoor temperature (°C) and the season.
func Next(seed, block uint64, prev State, meanTempC float64, season string) State {
	return prev
}

// Step advances a state by dt ticks at outdoor temperature tempC: easing
// towards the block's target, building or melting snow and puddles.
func (s State) Step(dt int, tempC float64, season string) State {
	return s
}
