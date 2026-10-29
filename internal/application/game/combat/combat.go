package combat

import (
	"rimworld-go/internal/domain/building/construction"
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
}

// New builds the combat cluster on host h with the extra roles r.
func New(h Host, r Roles) *Combat {
	c := &Combat{Sight: &Sight{g: h}, Traps: &Traps{h: h}, Breach: &Breach{h: h, b: r.Breach}}
	c.Turrets = newTurrets(h, r.Power, c.Sight)
	c.Strikes = &Strikes{h: h, sight: c.Sight, turrets: c.Turrets, hunts: r.Hunts}
	c.Blasts = &Blasts{h: h, sight: c.Sight, st: c.Strikes, turrets: c.Turrets}
	c.Shots = &Shots{h: h, st: c.Strikes, blasts: c.Blasts}
	c.Strikes.shots, c.Strikes.blasts = c.Shots, c.Blasts
	c.Guard = &Guard{h: h, fl: r.Flight}
	return c
}

// SwitchMargin: a fighter (or a raider choosing between a colonist and a
// turret) only swaps targets for one this many tiles closer than the one it
// has, so it never flaps between two.
const SwitchMargin = 3

// Sight answers what a shot can reach: rock, walls and closed doors stop
// bullets, and the buildings beside a target give it cover.
type Sight struct{ g Ground }

// blocks: rock, walls and (closed) doors stop bullets.
func (s *Sight) blocks(x, y int) bool {
	if s.g.Rock(x, y) {
		return true
	}
	if id, k := s.g.Object(x, y); id != 0 {
		return k == construction.Wall || k == construction.Door
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

// Cover is the cover (percent) the target at (tx, ty) has against a shot
// from (ax, ay): the best of the buildings on the cells beside it, facing
// the shooter.
func (s *Sight) Cover(ax, ay, tx, ty int) int {
	_, _, _, pct := s.coverBy(ax, ay, tx, ty)
	return pct
}

// coverBy is Cover with the building that gives it and its tile (id 0:
// no cover).
func (s *Sight) coverBy(ax, ay, tx, ty int) (id entity.EntityID, x, y, pct int) {
	if weapon.Distance(ax, ay, tx, ty) <= 1 {
		return 0, 0, 0, 0
	}
	sx, sy := sign(ax-tx), sign(ay-ty)
	cells := [3][2]int{{tx + sx, ty + sy}}
	n := 1
	if sx != 0 && sy != 0 {
		cells[1], cells[2], n = [2]int{tx + sx, ty}, [2]int{tx, ty + sy}, 3
	}
	for _, c := range cells[:n] {
		if bid, k := s.g.Object(c[0], c[1]); bid != 0 && construction.CoverPct(k) > pct {
			id, x, y, pct = bid, c[0], c[1], construction.CoverPct(k)
		}
	}
	return id, x, y, pct
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
