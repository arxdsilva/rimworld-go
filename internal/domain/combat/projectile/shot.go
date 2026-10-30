package projectile

import (
	"rimworld-go/internal/domain/combat/damage"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/sim/entity"
)

// Milli is the fixed-point scale of a shot's progress: thousandths of a
// cell.
const Milli = 1000

// Shot is a projectile in flight. It walks the Bresenham line from its
// origin to its destination (the same cells line of sight walks), Speed
// thousandths of a cell per tick, one Chebyshev step per cell, so a
// diagonal shot covers its tiles as fast as a straight one. All fields are
// exported for the save (gob).
type Shot struct {
	FromX, FromY int // origin tile (the shooter's)
	ToX, ToY     int // destination tile
	// X, Y is the cell reached, Cell how many cells it has walked (0 at
	// the origin), Steps the cells from origin to destination, Err the
	// line's error term.
	X, Y, Cell, Steps, Err int
	// Progress is thousandths of a cell travelled; Speed what one tick
	// adds.
	Progress, Speed int
	// Owner fired it at Target. Aimed: the aim held and it is headed at
	// Target's tile. Cover: the target's cover caught it, and the
	// destination is that building's tile.
	Owner, Target entity.EntityID
	Aimed         bool
	Cover         entity.EntityID
	// Raider: the owner's side, for friendly fire.
	Raider bool
	// Weapon fired it: Damage, Pen (armor penetration, permille) and
	// Type are the weapon's.
	Weapon weapon.Kind
	Damage int
	Pen    int
	Type   damage.Type
	// Fired is the tick it left the muzzle.
	Fired uint64
	// Hit is the firing weapon's hit (CBT-2, append only): what strikes
	// on landing, whatever the owner wields by then. Zero in older saves
	// (Weapon, Damage, Pen and Type stand in).
	Hit damage.Hit
	// FromBuilding (CBT-3, append only): a turret fired it; Owner is then
	// the turret's building id.
	FromBuilding bool
}

// Launch sets s on its way from (fx, fy) to (tx, ty) at speed thousandths
// of a cell per tick (at least 1).
func (s *Shot) Launch(fx, fy, tx, ty, speed int) {
	s.FromX, s.FromY, s.ToX, s.ToY = fx, fy, tx, ty
	s.X, s.Y, s.Cell, s.Progress = fx, fy, 0, 0
	s.Speed = max(speed, 1)
	dx, dy := abs(tx-fx), abs(ty-fy)
	s.Steps, s.Err = max(dx, dy), dx-dy
}

// Fly adds one tick of travel and returns how many new cells the shot
// enters this tick (Step walks them one at a time).
func (s *Shot) Fly() int {
	s.Progress += s.Speed
	return min(s.Progress/Milli, s.Steps) - s.Cell
}

// Step moves the shot into the next cell of its line and reports whether
// that cell is the destination.
func (s *Shot) Step() (x, y int, last bool) {
	dx, dy := abs(s.ToX-s.FromX), abs(s.ToY-s.FromY)
	e2 := 2 * s.Err
	if e2 > -dy {
		s.Err -= dy
		s.X += sign(s.ToX - s.FromX)
	}
	if e2 < dx {
		s.Err += dx
		s.Y += sign(s.ToY - s.FromY)
	}
	s.Cell++
	return s.X, s.Y, s.Cell >= s.Steps
}

// Blast reports an explosive's shot (a grenade or a launcher's shell,
// DEF-2): it arcs to its destination and bursts there.
func (s *Shot) Blast() bool { return weapon.IsExplosive(s.Weapon) }

// Arrived reports a shot standing on its destination (a zero-length shot
// arrives at once).
func (s *Shot) Arrived() bool { return s.Cell >= s.Steps }

// View is a projectile in flight for presentation.
type View struct {
	// X, Y is where it is now in thousandths of a tile from the map
	// origin (a tile's corner; add 500 for its centre); VX, VY is what it
	// moves per tick, for interpolation between ticks.
	X, Y, VX, VY int
	// FromX, FromY and ToX, ToY are its origin and destination tiles.
	FromX, FromY, ToX, ToY int
	// Weapon is the weapon's name; Owner fired it at Target.
	Weapon        string
	Owner, Target entity.EntityID
	// Fired is the tick it left the muzzle.
	Fired uint64
	// Hit is the firing weapon's hit (CBT-2, append only): what strikes
	// on landing, whatever the owner wields by then. Zero in older saves
	// (Weapon, Damage, Pen and Type stand in).
	Hit damage.Hit
}

// View is s for presentation: its point on the straight segment from
// origin to destination.
func (s *Shot) View() View {
	v := View{FromX: s.FromX, FromY: s.FromY, ToX: s.ToX, ToY: s.ToY,
		Weapon: weapon.DefOf(s.Weapon).Name, Owner: s.Owner, Target: s.Target, Fired: s.Fired}
	v.X, v.Y = s.FromX*Milli, s.FromY*Milli
	if s.Steps > 0 {
		p := min(s.Progress, s.Steps*Milli)
		v.X += (s.ToX - s.FromX) * p / s.Steps
		v.Y += (s.ToY - s.FromY) * p / s.Steps
		v.VX, v.VY = (s.ToX-s.FromX)*s.Speed/s.Steps, (s.ToY-s.FromY)*s.Speed/s.Steps
	}
	return v
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func sign(v int) int {
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	}
	return 0
}
