// Package explosion is the pure blast model (DEF-2): what a grenade,
// launcher shell or (later) IED, mortar shell or rocket does where it
// bursts. It answers which cells a blast reaches, how hard it hits at a
// distance, and what it leaves behind (fire, an EMP stun, a smoke cloud).
// Applying it (line of sight from the centre, creatures, buildings, the
// game's fire) is the combat cluster's job.
package explosion

import "rimworld-go/internal/domain/combat/damage"

// Kind is what a blast does.
type Kind uint8

const (
	None  Kind = iota
	Bomb       // shrapnel and blast: frag grenades
	Flame      // burning fuel: molotovs, incendiary shells; sets cells alight
	EMP        // an electromagnetic pulse: stuns machines, harms no flesh
	Smoke      // a cloud that spoils aim through it, harms nothing
	numKinds
)

var names = [numKinds]string{"", "bomb", "flame", "EMP", "smoke"}

// String is the player-facing name ("" for None).
func (k Kind) String() string {
	if k >= numKinds {
		return ""
	}
	return names[k]
}

// kinds: the damage type each kind deals and its damage against buildings
// (permille of the blast's damage). wiki: Damage (bomb, flame, EMP types),
// Frag grenades (bomb damage x2 against buildings; the x4/x8 against
// impassable things and walls is left for DEF-3's structures).
var kinds = [numKinds]struct {
	typ      damage.Type
	building int
	flesh    bool
}{
	Bomb:  {damage.Bomb, 2000, true},
	Flame: {damage.Heat, 1000, true},
	EMP:   {damage.EMP, 0, false},
	Smoke: {damage.Blunt, 0, false},
}

// Def is one blast: its kind, reach and punch. Radius is in thousandths
// of a tile (wiki radii such as 1.9 tiles: 1900); Damage and Pen
// (armor penetration, permille) are at the centre.
type Def struct {
	Kind   Kind
	Radius int
	Damage int
	Pen    int
}

// Zero reports a weapon without a blast.
func (d Def) Zero() bool { return d.Kind == None }

// Type is the damage type the blast deals.
func (d Def) Type() damage.Type {
	if d.Kind >= numKinds {
		return damage.Blunt
	}
	return kinds[d.Kind].typ
}

// Flesh reports a blast that wounds creatures (EMP and smoke do not).
func (d Def) Flesh() bool { return d.Kind < numKinds && kinds[d.Kind].flesh }

// Ignites reports a blast that sets the cells it reaches alight.
func (d Def) Ignites() bool { return d.Kind == Flame }

// Reach is the radius in whole tiles, rounded up: the square a caller
// scans around the centre.
func (d Def) Reach() int { return (d.Radius + 999) / 1000 }

// Within reports whether the cell (dx, dy) from the centre lies inside the
// blast: its centre within Radius (Euclidean).
func (d Def) Within(dx, dy int) bool {
	return int64(dx*dx+dy*dy)*1_000_000 <= int64(d.Radius)*int64(d.Radius)
}

// Dist is the Euclidean distance of (dx, dy) in thousandths of a tile
// (integer square root, so the same everywhere).
func Dist(dx, dy int) int { return isqrt(int64(dx*dx+dy*dy) * 1_000_000) }

// RimShare is the damage left at the blast's rim (permille). original
// tuning: the wiki publishes no falloff, so a blast hits full at its centre
// and loses damage linearly to half at the rim: standing at the edge of a
// frag blast still hurts, standing on it hurts twice as much.
const RimShare = 500

// At is the damage a creature dist thousandths of a tile from the centre
// takes: Damage at the centre, falling linearly to RimShare at Radius,
// 0 beyond it; at least 1 inside a damaging blast.
func (d Def) At(dist int) int {
	if d.Damage <= 0 || dist > d.Radius || dist < 0 {
		return 0
	}
	f := 1000
	if d.Radius > 0 {
		f -= (1000 - RimShare) * dist / d.Radius
	}
	return max(d.Damage*f/1000, 1)
}

// Building is the damage a building dist thousandths of a tile from the
// centre takes: At scaled by the kind's damage against structures.
func (d Def) Building(dist int) int {
	if d.Kind >= numKinds || kinds[d.Kind].building == 0 {
		return 0
	}
	return d.At(dist) * kinds[d.Kind].building / 1000
}

// StunPerDamage is the stun an EMP inflicts on a machine, ticks per point
// of EMP damage (0.25 s). wiki: Damage (EMP)
const StunPerDamage = 15

// StunTicks is how long a machine dist from the centre stays stunned (0
// for a blast that is not an EMP).
func (d Def) StunTicks(dist int) int {
	if d.Kind != EMP {
		return 0
	}
	return d.At(dist) * StunPerDamage
}

// SmokeTicks is how long a smoke cloud lasts (about a minute). wiki: Smoke
// shell
const SmokeTicks = 3600

// forcedMiss is the forced-miss radius multiplier by distance to the
// target (permille): explosives land on the spot up close and scatter
// further out. wiki: Combat (marked "needs verification")
var forcedMiss = [...]struct{ upTo, f int }{{8, 0}, {24, 500}, {48, 800}}

// ForcedMiss is the scatter radius (whole tiles, rounded) of an explosive
// aimed dist tiles away with a forced-miss radius of base thousandths of a
// tile (grenades and launchers: 1900). wiki: Weapons, Combat
func ForcedMiss(base, dist int) int {
	f := 1000
	for _, p := range forcedMiss {
		if dist <= p.upTo {
			f = p.f
			break
		}
	}
	return (base*f/1000 + 500) / 1000
}

// Cloud is a smoke cloud on the map until tick Until: shots that pass
// through it aim worse (attack.SmokeFactor). All fields are exported for
// the save (gob).
type Cloud struct {
	X, Y   int
	Radius int // thousandths of a tile
	Until  uint64
}

// Crosses reports whether the shot from (ax, ay) to (bx, by) passes
// through c: the segment comes within the cloud's radius of its centre
// (fixed point, thousandths of a tile).
func (c Cloud) Crosses(ax, ay, bx, by int) bool {
	px, py := int64(c.X-ax)*1000, int64(c.Y-ay)*1000
	vx, vy := int64(bx-ax), int64(by-ay)
	var t int64 // the closest point's share of the segment, permille
	if l2 := vx*vx + vy*vy; l2 > 0 {
		t = min(max((px*vx+py*vy)/l2, 0), 1000)
	}
	dx, dy := px-vx*t, py-vy*t
	r := int64(c.Radius)
	return dx*dx+dy*dy <= r*r
}

func isqrt(n int64) int {
	if n <= 0 {
		return 0
	}
	x := int64(1) << ((bitLen(n) + 1) / 2)
	for {
		y := (x + n/x) / 2
		if y >= x {
			return int(x)
		}
		x = y
	}
}

func bitLen(n int64) int {
	b := 0
	for n > 0 {
		b++
		n >>= 1
	}
	return b
}
