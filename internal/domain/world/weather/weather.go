// Package weather is the deterministic outdoor weather model (SIM-016):
// what falls from the sky, how hard, the wind, and what it leaves on the
// ground. It is pure: callers pass the seed, time and temperature in.
package weather

import (
	"math"
	"strings"
)

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
// built up on open ground. Goal is the intensity the state is easing
// towards (the block's target intensity, jitter included).
type State struct {
	Kind      Kind
	Target    Kind // the block's kind the state is easing towards
	Intensity float64
	WindDir   float64
	WindSpeed float64
	SnowDepth float64
	Wetness   float64
	Goal      float64
}

const (
	blockTicks = 2500 * 6 // one weather block: 6 hours
	easeTicks  = 2500.0   // ticks to ease across the full 0..1 range
	stepTicks  = 250.0    // the reference build-up interval
	nearGoal   = 0.05
)

var targetIntensity = [...]float64{0, 0.15, 0.6, 1.0, 0.5, 1.0}

var windRange = [...][2]float64{
	{0.4, 1.2}, {0.8, 1.6}, {1.2, 2.2}, {2.5, 4.0}, {0.6, 1.4}, {3.0, 4.5},
}

// rows are the Markov rows from Clear, Cloudy, Rain and Storm to
// Clear, Cloudy, Rain and Storm.
var rows = [4][4]float64{
	{.62, .28, .10, 0},
	{.30, .38, .24, .08},
	{.12, .38, .38, .12},
	{.05, .40, .45, .10},
}

// Row is the chance of each next kind (Clear, Cloudy, Rain, Storm) after
// from in season, renormalised. Snow follows Rain's row, Blizzard Storm's.
func Row(from Kind, season string) [4]float64 {
	idx := int(from)
	switch from {
	case Snow:
		idx = int(Rain)
	case Blizzard:
		idx = int(Storm)
	}
	r := rows[idx]
	f := 1.0
	switch {
	case strings.EqualFold(season, "spring"), strings.EqualFold(season, "fall"):
		f = 1.3
	case strings.EqualFold(season, "summer"):
		f = 0.7
	}
	r[Rain] *= f
	r[Storm] *= f
	sum := r[0] + r[1] + r[2] + r[3]
	for i := range r {
		r[i] /= sum
	}
	return r
}

func mix(x uint64) uint64 {
	x += 0x9e3779b97f4a7c15
	x = (x ^ x>>30) * 0xbf58476d1ce4e5b9
	x = (x ^ x>>27) * 0x94d049bb133111eb
	return x ^ x>>31
}

// unit is a deterministic value in [0,1) from (seed, block, salt).
func unit(seed, block, salt uint64) float64 {
	h := mix(mix(mix(seed)^block) ^ salt)
	return float64(h>>11) / (1 << 53)
}

func clamp01(v float64) float64 { return math.Max(0, math.Min(1, v)) }

func windSpeed(k Kind, intensity float64) float64 {
	r := windRange[k]
	return r[0] + (r[1]-r[0])*clamp01(intensity)
}

// Next is the weather for 6-hour block b, given the previous state, the
// day's mean outdoor temperature (°C) and the season. It picks the block's
// kind, target intensity and wind direction; rain halves the snow lying.
func Next(seed, block uint64, prev State, meanTempC float64, season string) State {
	row := Row(prev.Target, season)
	u := unit(seed, block, 1)
	k := Storm
	acc := 0.0
	for i, p := range row {
		acc += p
		if u < acc {
			k = Kind(i)
			break
		}
	}
	if row[k] == 0 { // rounding landed on an impossible kind
		k = Cloudy
	}
	switch {
	case meanTempC <= 0 && k == Rain:
		k = Snow
	case meanTempC <= 0 && k == Storm:
		k = Blizzard
	}
	s := prev
	s.Target = k
	s.Goal = clamp01(targetIntensity[k] + (unit(seed, block, 2)-0.5)*0.3)
	if k == Rain || k == Storm {
		s.SnowDepth /= 2
	}
	day := float64(block) * blockTicks / (2500 * 24)
	s.WindDir = unit(seed, 0, 3)*2*math.Pi + 0.6*math.Sin(2*math.Pi*day/9) + (unit(seed, block, 4)-0.5)*0.8
	s.WindSpeed = windSpeed(s.Kind, s.Intensity)
	return s
}

// Step advances a state by dt ticks at outdoor temperature tempC: easing
// towards the block's target, building or melting snow and puddles.
func (s State) Step(dt int, tempC float64, season string) State {
	k := float64(dt) / stepTicks
	if s.Kind != s.Target && (s.Kind == Clear || s.Kind == Cloudy) {
		s.Kind = s.Target
	}
	step := float64(dt) / easeTicks
	if d := s.Goal - s.Intensity; math.Abs(d) <= step+1e-9 {
		s.Intensity = s.Goal
	} else if d > 0 {
		s.Intensity += step
	} else {
		s.Intensity -= step
	}
	if s.Kind != s.Target && math.Abs(s.Goal-s.Intensity) <= nearGoal {
		s.Kind = s.Target
	}
	s.WindSpeed = windSpeed(s.Kind, s.Intensity)
	raining := s.Kind == Rain || s.Kind == Storm
	if s.Kind.Frozen() {
		s.SnowDepth += 0.012 * s.Intensity * k
	}
	s.SnowDepth -= 0.004 * math.Max(0, tempC) * k
	if raining {
		s.Wetness += 0.02 * s.Intensity * k
	} else {
		dry := 0.006
		if strings.EqualFold(season, "summer") {
			dry *= 2
		}
		s.Wetness -= dry * k
	}
	s.SnowDepth, s.Wetness = clamp01(s.SnowDepth), clamp01(s.Wetness)
	return s
}
