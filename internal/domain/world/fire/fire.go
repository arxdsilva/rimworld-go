package fire

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

// Fuel is what a burning cell has to burn, summed by the game.
type Fuel struct {
	// Flammability is the best flammability on the cell in permille
	// (plants, grass, buildings, items); 0 means nothing burns there.
	Flammability int
}

// Burns reports whether a cell with this fuel can burn.
func (f Fuel) Burns() bool { return f.Flammability > 0 }
