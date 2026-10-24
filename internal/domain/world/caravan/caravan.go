package caravan

// State is where a caravan is in its trip (append only, saved).
type State uint8

const (
	Travelling State = iota
	Arrived
	Returning
	Home
	Forming
	Visiting
)

// BaseTilesPerDay is the speed, in permille of a tile per day, of a
// caravan whose slowest member moves at 1000 permille.
const BaseTilesPerDay int32 = 3000

// Caravan is a group of colonists (and animals) on the world map.
// Progress is permille of the way to the next tile of Path.
type Caravan struct {
	ID         uint64
	Members    []uint64
	Animals    []uint64
	Tile       int
	Path       []int
	Progress   int
	Dest       int
	State      State
	FoodDays   int32
	VisitUntil uint64
	// Slowest is the slowest member's Moving capacity, permille (0: 1000).
	Slowest int32
}

// SpeedFor is tiles per day, in permille, for a slowest Moving capacity.
func SpeedFor(slowestMovingPermille int32) int32 {
	return BaseTilesPerDay * max(slowestMovingPermille, 0) / 1000
}

// Speed is the caravan's tiles per day, in permille.
func (c *Caravan) Speed() int32 {
	s := c.Slowest
	if s == 0 {
		s = 1000
	}
	return SpeedFor(s)
}

// Advance moves the caravan hours along Path and reports whether it
// reached a tile. Path is the remaining tiles to enter. It sets Arrived
// when the path ends, and spends FoodDays while moving.
func Advance(c *Caravan, hours int) bool {
	if c.State != Travelling && c.State != Returning || hours <= 0 {
		return false
	}
	if len(c.Path) == 0 {
		c.State = Arrived
		return false
	}
	// permille of a tile covered: speed (permille tiles/day) * hours / 24.
	gain := int64(c.Speed()) * int64(hours) / 24
	if c.FoodDays > 0 {
		c.FoodDays = max(c.FoodDays-int32((int64(hours)*1000+23)/24), 0)
	}
	reached := false
	c.Progress += int(gain)
	for c.Progress >= 1000 && len(c.Path) > 0 {
		c.Progress -= 1000
		c.Tile = c.Path[0]
		c.Path = c.Path[1:]
		reached = true
	}
	if len(c.Path) == 0 {
		c.Progress = 0
		c.State = Arrived
	}
	return reached
}

// Step advances the caravan by ticks (rounded down to whole hours of 2500
// ticks); it reports whether it reached a tile.
func (c *Caravan) Step(ticks int) bool {
	return Advance(c, ticks/2500)
}

// IsArrived reports whether the caravan has reached the end of its path.
func IsArrived(c *Caravan) bool { return c.State == Arrived }
