package combat

import (
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/combat/cover"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/sim/entity"
)

// Combat is the combat cluster's root: the game builds one with New and
// keeps one field. Its components are exported so the game reaches each by
// name (g.combat.Turrets.Phase); a feature's entry points go on its
// component, never here.
type Combat struct {
	// Sight is line of sight, reach and cover.
	Sight *Sight
	// Strikes resolves attacks and runs attack and hunt jobs (strike.go).
	Strikes *Strikes
	// Guard is a colonist meeting an attacker: fight or flee (defend.go).
	Guard *Guard
	// Traps and Breach are the spike traps and raiders breaking in
	// (defense.go).
	Traps  *Traps
	Breach *Breach
	// Turrets are the sentry turrets (turret.go, PLAY-024).
	Turrets *Turrets
	// Shots are the projectiles in flight (shot.go, DEF-1).
	Shots *Shots
	// Blasts are the explosions and their smoke (blast.go, DEF-2).
	Blasts *Blasts
	// DEF-3: Doors (door.go), Mortars (mortar.go), Rearms (rearm.go: the
	// haul that loads a turret's barrel or a mortar's shells), Orders
	// (orders.go: the defence commands and the buildings' read model) and
	// Hunting and Tiles (hunt.go, tiles.go: assigning hunts and finding
	// free ground, moved from the game).
	Doors   *Doors
	Mortars *Mortars
	Rearms  *Rearms
	Orders  *Orders
	Hunting *Hunting
	Tiles   *Tiles
}

// New builds the combat cluster on host h with the extra roles r.
func New(h Host, r Roles) *Combat {
	c := &Combat{Sight: &Sight{g: h, f: r.Field}, Breach: &Breach{h: h, b: r.Breach}}
	c.Doors = &Doors{h: h, pw: r.Power}
	c.Sight.doors = c.Doors
	c.Turrets = newTurrets(h, r.Power, c.Sight)
	c.Strikes = &Strikes{h: h, sight: c.Sight, turrets: c.Turrets, hunts: r.Hunts}
	c.Blasts = &Blasts{h: h, sight: c.Sight, st: c.Strikes, turrets: c.Turrets}
	c.Shots = &Shots{h: h, st: c.Strikes, blasts: c.Blasts}
	c.Strikes.shots, c.Strikes.blasts, c.Turrets.st = c.Shots, c.Blasts, c.Strikes
	c.Traps = &Traps{h: h, blasts: c.Blasts}
	c.Guard = &Guard{h: h, fl: r.Flight}
	c.Mortars = &Mortars{h: h, f: r.Field, turrets: c.Turrets, shots: c.Shots}
	c.Rearms = &Rearms{h: h, sup: r.Supply, br: r.Breach, turrets: c.Turrets, mortars: c.Mortars}
	c.Orders = &Orders{h: h, turrets: c.Turrets, mortars: c.Mortars, doors: c.Doors, rearms: c.Rearms}
	c.Hunting, c.Tiles = &Hunting{h: h, hu: r.Hunts}, &Tiles{f: r.Field}
	c.Turrets.after = [...]phaser{c.Traps, c.Doors, c.Mortars, c.Rearms}
	c.Turrets.saved = saved{traps: c.Traps, doors: c.Doors, mortars: c.Mortars, rearms: c.Rearms}
	return c
}

// phaser is a defence stepped once a tick after the turrets (Turrets.Phase,
// the game's threat phase): IED fuses, doors, mortars and rearm hauls.
type phaser interface{ phase(now uint64) }

// SwitchMargin: a fighter (or a raider choosing between a colonist and a
// turret) only swaps targets for one this many tiles closer than the one it
// has, so it never flaps between two.
const SwitchMargin = 3

// Sight answers what a shot can reach: rock, walls and closed doors stop
// bullets, and what stands beside a target gives it cover (DEF-3: the
// cover domain's values for buildings, rock, trees and chunks).
type Sight struct {
	g     sightGround
	f     Field // nil: no natural cover but rock (tests)
	doors *Doors
	// chunks is the loose stone chunks' tiles, rebuilt at most once in
	// chunkEvery ticks (an item scan), chunkAt the tick it was built.
	chunks  map[[2]int]struct{}
	chunkAt uint64
	chunkOK bool
	chunkB  [][2]int
}

// sightGround is the map and the clock (the chunk cache's age).
type sightGround interface {
	Ground
	Now() uint64
}

// chunkEvery is how long the chunk tiles are trusted: chunks are mined,
// hauled and cut slowly, and cover needs no tick-exact list. original
// tuning.
const chunkEvery = 250

// blocks: rock, walls and closed doors stop bullets (an open door lets
// them through, DEF-3).
func (s *Sight) blocks(x, y int) bool {
	if s.g.Rock(x, y) {
		return true
	}
	if id, k := s.g.Object(x, y); id != 0 {
		return k == construction.Wall || k.IsDoor() && !s.doors.isOpen(id)
	}
	return false
}

// Line walks the Bresenham line between two tiles, endpoints excluded,
// and reports whether nothing on it blocks sight.
func (s *Sight) Line(ax, ay, bx, by int) bool {
	dx, dy := bx-ax, by-ay
	sx, sy := 1, 1
	if dx < 0 {
		dx, sx = -dx, -1
	}
	if dy < 0 {
		dy, sy = -dy, -1
	}
	err := dx - dy
	x, y := ax, ay
	for {
		if x == bx && y == by {
			return true
		}
		if !(x == ax && y == ay) && s.blocks(x, y) {
			return false
		}
		e2 := 2 * err
		if e2 > -dy {
			err -= dy
			x += sx
		}
		if e2 < dx {
			err += dx
			y += sy
		}
	}
}

// CanHit reports whether an attacker at (fx, fy) wielding d can attack the
// target at (tx, ty) from where it stands: in range, and in sight for a
// gun beyond touching distance.
func (s *Sight) CanHit(fx, fy int, d weapon.Def, tx, ty int) bool {
	dist := weapon.Distance(fx, fy, tx, ty)
	if !weapon.InRange(d, dist) {
		return false
	}
	return !d.Ranged || dist <= 1 || s.Line(fx, fy, tx, ty)
}

// Cover is the cover (permille) the target at (tx, ty) has against a
// shot from (ax, ay): the best of what stands on the cells beside it,
// facing the shooter (cover.BestAt); none point blank.
func (s *Sight) Cover(ax, ay, tx, ty int) int {
	_, _, _, p := s.coverBy(ax, ay, tx, ty)
	return p
}

// coverBy is Cover with the building that gives it (0: natural cover or
// none) and the cover's tile.
func (s *Sight) coverBy(ax, ay, tx, ty int) (id entity.EntityID, x, y, p int) {
	if weapon.Distance(ax, ay, tx, ty) <= 1 {
		return 0, 0, 0, 0
	}
	k, p, x, y := cover.BestAt(s, ax, ay, tx, ty)
	if k == cover.None {
		return 0, 0, 0, 0
	}
	if bid, bk := s.g.Object(x, y); bid != 0 && cover.OfBuilding(bk) == k {
		id = bid
	}
	return id, x, y, p
}

// CoverAt is the cover kind on (x, y) (cover.Grid): a building's, else
// rock, a grown tree, cactus or bush, or a stone chunk.
func (s *Sight) CoverAt(x, y int) cover.Kind {
	if id, k := s.g.Object(x, y); id != 0 {
		if c := cover.OfBuilding(k); c != cover.None {
			return c
		}
	}
	if s.g.Rock(x, y) {
		return cover.Rock
	}
	if s.f == nil {
		return cover.None
	}
	if sp, grown, ok := s.f.Plant(x, y); ok {
		if c := cover.OfPlant(sp, grown); c != cover.None {
			return c
		}
	}
	if s.chunk(x, y) {
		return cover.Chunk
	}
	return cover.None
}

// chunk reports a loose stone chunk on (x, y), from the cached tiles.
func (s *Sight) chunk(x, y int) bool {
	if now := s.g.Now(); !s.chunkOK || now >= s.chunkAt+chunkEvery || now < s.chunkAt {
		s.chunkB = s.f.AppendChunks(s.chunkB[:0])
		if s.chunks == nil {
			s.chunks = map[[2]int]struct{}{}
		}
		clear(s.chunks)
		for _, c := range s.chunkB {
			s.chunks[c] = struct{}{}
		}
		s.chunkAt, s.chunkOK = now, true
	}
	_, ok := s.chunks[[2]int{x, y}]
	return ok
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

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
