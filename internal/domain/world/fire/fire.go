package fire

import "math"

// Fire is one burning cell. Size is 1..MaxSize; a fire at 0 is out.
type Fire struct {
	X, Y int
	Size uint8
	Age  uint32 // fire steps it has burned
}

const (
	// MaxSize is the largest a fire grows.
	MaxSize = 100
	// StepTicks is how often fires update.
	StepTicks = 30
)

// Per-step rules (our numbers).
const (
	// Grow is the size a fire gains each step while its cell has fuel;
	// Starve what it loses once nothing is left to burn.
	Grow   = 4
	Starve = 6
	// Drench is what hard rain or snow takes off an outdoor fire each step.
	Drench = 12
	// DouseIntensity is the weather intensity from which it does.
	DouseIntensity = 0.3
	// SpreadSize is the size from which a fire spreads to its neighbours.
	SpreadSize = 35
	// IgniteSize is the size a fire starts at when it spreads or a spark
	// catches.
	IgniteSize = 10
	// spreadPermille: chance = flammability x size/100 x 0.06.
	spreadPermille = 60
	downwindPct    = 150 // a neighbour downwind catches half again as often
	shieldPct      = 20  // a roofed neighbour of an outdoor fire: a fifth as often
	// downwindCos: a neighbour within about 45 degrees of the wind's heading
	// is downwind; stillWind: weaker wind (tiles per second) never biases.
	downwindCos = 0.7
	stillWind   = 0.2
)

// Fuel is what a burning cell has to burn, summed by the game.
type Fuel struct {
	// Flammability is the best flammability on the cell in permille
	// (plants, grass, buildings, items); 0 means nothing burns there.
	Flammability int
}

// Burns reports whether a cell with this fuel can burn.
func (f Fuel) Burns() bool { return f.Flammability > 0 }

// With is f with one more thing of flammability fl on the cell: the most
// flammable thing sets the cell's fuel.
func (f Fuel) With(fl int) Fuel {
	if fl > f.Flammability {
		f.Flammability = fl
	}
	return f
}

// Next is a fire's size after one step: it grows while fuel remains,
// starves without, and hard rain or snow (doused) beats it down. 0 is out.
func Next(size int, fuel Fuel, doused bool) int {
	if fuel.Burns() {
		size += Grow
	} else {
		size -= Starve
	}
	if doused {
		size -= Drench
	}
	return max(0, min(MaxSize, size))
}

// Doused reports a fire beaten down by the weather: rain or snow falling
// at DouseIntensity or harder on a cell with no roof over it.
func Doused(falling bool, intensity float64, roofed bool) bool {
	return falling && !roofed && intensity >= DouseIntensity
}

// SpreadChance is the chance, in parts per million, that a fire of size
// sets a neighbouring cell with fuel to alight in one step. downwind: the
// neighbour lies downwind; shielded: it is roofed while the fire is not.
func SpreadChance(size int, to Fuel, downwind, shielded bool) int {
	if size < SpreadSize || !to.Burns() {
		return 0
	}
	// flammability (permille) x size (percent) x 0.06, in ppm
	c := to.Flammability * size * spreadPermille / 100
	if downwind {
		c = c * downwindPct / 100
	}
	if shielded {
		c = c * shieldPct / 100
	}
	return c
}

// Downwind reports whether the neighbour at offset (dx, dy) lies
// downwind of its fire, the wind blowing (windX, windY) tiles per second.
func Downwind(dx, dy int, windX, windY float64) bool {
	w := math.Hypot(windX, windY)
	d := math.Hypot(float64(dx), float64(dy))
	if w < stillWind || d == 0 {
		return false
	}
	return (float64(dx)*windX+float64(dy)*windY)/(w*d) >= downwindCos
}

// BuildingDamage is the hit points a fire of size takes off a burning
// building each step.
func BuildingDamage(size int) int {
	if size <= 0 {
		return 0
	}
	return 1 + size/12
}

// CreatureDamage is the injury a fire of size deals each step to a
// creature standing in it.
func CreatureDamage(size int) int {
	if size <= 0 {
		return 0
	}
	return 1 + size/25
}

// AdjacentDamage is the injury a creature beside a fire of size takes
// each step (only big fires singe their neighbours).
func AdjacentDamage(size int) int {
	if size < adjacentSize {
		return 0
	}
	return 1
}

// adjacentSize: fires this big singe creatures on the cells around them.
const adjacentSize = 60
