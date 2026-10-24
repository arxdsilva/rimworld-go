package caravan

// State is where a caravan is in its trip (append only, saved).
type State uint8

const (
	Travelling State = iota
	Arrived
	Returning
	Home
)

// Caravan is a group of colonists (and animals) on the world map.
type Caravan struct {
	ID       uint64
	Members  []uint64
	Animals  []uint64
	Tile     int
	Path     []int
	Progress int // permille of the way to the next tile
	Dest     int
	State    State
}

// Speed is the caravan's tiles per day, in permille.
func (c *Caravan) Speed() int32 {
	return 0 // TODO(M9)
}

// Step advances the caravan by ticks; it reports whether it reached a tile.
func (c *Caravan) Step(ticks int) bool {
	return false // TODO(M9)
}
