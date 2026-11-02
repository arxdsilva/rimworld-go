package caravan

import "rimworld-go/internal/domain/animal"

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
	// Slowest is the slowest walking member's Moving capacity, permille
	// (0: 1000); Load the cargo's share of what the caravan can carry,
	// permille; Mount its riding speed factor, permille (0: none). MAP-09.
	Slowest, Load, Mount int32
}

// Movement (MAP-09, wiki: Caravan): at full speed a tile takes 0.06 days
// per point of its movement difficulty (roads already halve it), and
// caravans travel only from 06:00 to 22:00.
const (
	TicksPerDifficulty = 3600  // 0.06 days × 60 000 ticks (wiki: Caravan)
	MoveFrom, MoveTo   = 6, 22 // hours of the day (wiki: Caravan)
	ticksPerHour       = 2500
	estimateHours      = 24 * 1000 // Estimate gives up after a thousand days
)

// LoadFactor is the speed factor (permille) of a load (permille of what
// the caravan can carry): ×2 empty down to ×0.5 full (wiki: Caravan),
// linear between (original tuning).
func LoadFactor(load int32) int32 {
	return 2000 - 1500*min(max(load, 0), 1000)/1000
}

// MountFactor is the riding speed factor (permille) of a mount species,
// 0 for an animal nobody rides. The range 1.3-2.0 is the wiki's
// (Caravan); each species' place in it is original tuning.
func MountFactor(s animal.Species) int32 {
	switch s {
	case animal.Horse:
		return 2000
	case animal.Dromedary:
		return 1600
	case animal.Elephant:
		return 1500
	case animal.Yak:
		return 1400
	case animal.Donkey:
		return 1300
	}
	return 0
}

// Speed is the caravan's speed factor, permille: its slowest walker
// times its load factor times its mounts.
func (c *Caravan) Speed() int32 {
	s := c.Slowest
	if s == 0 {
		s = 1000
	}
	v := int64(s) * int64(LoadFactor(c.Load)) / 1000
	if c.Mount > 0 {
		v = v * int64(c.Mount) / 1000
	}
	return int32(max(v, 1))
}

// TileTicks is the moving time across a tile of milli-difficulty dm
// (1000 = difficulty 1) at speed (permille).
func TileTicks(dm int, speed int32) int {
	if dm <= 0 {
		dm = 1000 // impassable tiles are never on a route
	}
	return max(1, int(int64(TicksPerDifficulty)*int64(dm)/int64(max(speed, 1))))
}

// TilesPerDay is how many difficulty-1 tiles a day speed covers, nights
// off, for the player's summary.
func TilesPerDay(speed int32) float64 {
	return float64((MoveTo-MoveFrom)*ticksPerHour) / float64(TileTicks(1000, speed))
}

// Moving reports whether caravans travel at hour of the day.
func Moving(hour int) bool { return hour >= MoveFrom && hour < MoveTo }

// Advance moves the caravan one hour along Path, the hour starting at
// hour of the day; dm is a tile's movement difficulty in thousandths. It
// spends an hour of food on the road, sets Arrived when the path ends and
// reports whether it reached a tile.
func Advance(c *Caravan, hour int, dm func(tile int) int) bool {
	if c.State != Travelling && c.State != Returning {
		return false
	}
	if len(c.Path) == 0 {
		c.State = Arrived
		return false
	}
	if c.FoodDays > 0 {
		c.FoodDays = max(c.FoodDays-(1000+23)/24, 0)
	}
	return walk(c, hour, c.Speed(), dm)
}

// walk is Advance's movement at speed.
func walk(c *Caravan, hour int, speed int32, dm func(tile int) int) bool {
	if !Moving(hour) {
		return false
	}
	reached := false
	c.Progress += 1000 * ticksPerHour / TileTicks(dm(c.Path[0]), speed)
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

// Estimate is how many ticks a caravan at speed (permille) needs to
// cross path from the start of hour (of the day), resting at night, in
// whole hours as Advance moves it; -1 when it would not get there.
func Estimate(path []int, speed int32, hour int, dm func(tile int) int) int {
	if len(path) == 0 {
		return 0
	}
	if speed <= 0 {
		return -1
	}
	c := Caravan{Path: path}
	for h := 0; h < estimateHours; h++ {
		walk(&c, (hour+h)%24, speed, dm)
		if len(c.Path) == 0 {
			return (h + 1) * ticksPerHour
		}
	}
	return -1
}

// IsArrived reports whether the caravan has reached the end of its path.
func IsArrived(c *Caravan) bool { return c.State == Arrived }

// Label is the caravan inspector's state line.
func (s State) Label() string {
	switch s {
	case Forming:
		return "Forming"
	case Travelling:
		return "Travelling"
	case Arrived:
		return "Waiting at destination"
	case Visiting:
		return "Visiting"
	case Returning:
		return "Returning home"
	}
	return "Home"
}
