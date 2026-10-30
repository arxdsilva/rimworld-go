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

// Target is what a blast strikes besides creatures (CBT-5): its damage
// is scaled by the kind of thing it lands on.
type Target uint8

const (
	Passable   Target = iota // a building creatures walk through: doors, fences, furniture spots
	Impassable               // a building that blocks movement: benches, turrets, batteries
	Wall                     // a wall
	Plant                    // a plant or tree
	numTargets
)

// kinds: the damage type each kind deals, its damage against each target
// (permille of the blast's damage) and whether it wounds creatures. wiki:
// Damage types (bomb x2 against passable buildings, x4 against
// impassable ones and plants, x8 in all against walls); flame x1 on
// buildings and nothing on plants directly (the fire it lights burns
// them); EMP and smoke harm no structure.
var kinds = [numKinds]struct {
	typ     damage.Type
	against [numTargets]int
	flesh   bool
}{
	Bomb:  {damage.Bomb, [numTargets]int{Passable: 2000, Impassable: 4000, Wall: 8000, Plant: 4000}, true},
	Flame: {damage.Heat, [numTargets]int{Passable: 1000, Impassable: 1000, Wall: 1000}, true},
	EMP:   {damage.EMP, [numTargets]int{}, false},
	Smoke: {damage.Blunt, [numTargets]int{}, false},
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

// Against is the damage target t, dist thousandths of a tile from the
// centre, takes: At scaled by the kind's multiplier against it (0: the
// blast leaves it whole).
func (d Def) Against(dist int, t Target) int {
	if d.Kind >= numKinds || t >= numTargets || kinds[d.Kind].against[t] == 0 {
		return 0
	}
	return d.At(dist) * kinds[d.Kind].against[t] / 1000
}

// Structural reports a blast that harms buildings at all.
func (d Def) Structural() bool { return d.Against(0, Passable) > 0 }

// StunPerDamage is the stun an EMP inflicts on a machine, ticks per point
// of EMP damage (half a second: 50 points stun for 1500 ticks). A machine
// that has shaken off a stun is adapted for EMPAdaptTicks after it: a
// pulse then leaves it running. wiki: Damage types (EMP)
const (
	StunPerDamage = 30
	EMPAdaptTicks = 2200
)

// Adapted reports a machine stunned until tick until (0: never) that has
// recovered and still shrugs off pulses at tick now.
func Adapted(until, now uint64) bool {
	return until != 0 && now >= until && now < until+EMPAdaptTicks
}

// StunTicks is how long a machine dist from the centre stays stunned (0
// for a blast that is not an EMP).
func (d Def) StunTicks(dist int) int {
	if d.Kind != EMP {
		return 0
	}
	return d.At(dist) * StunPerDamage
}

// Item is the damage a loose stack dist thousandths of a tile from the
// centre takes (CBT-5): a bomb's At, worn off its hit points; flame
// reaches stacks through the fire it lights, EMP and smoke not at all.
func (d Def) Item(dist int) int {
	if d.Kind != Bomb {
		return 0
	}
	return d.At(dist)
}

// LooseHP is the hit points of a stack with no condition of its own
// (food, textiles, wood, corpses) that burns: one blast dealing at least
// this much destroys it, a weaker one leaves it whole. Stacks that do not
// burn (stone, metal) shrug blasts off. original tuning: the game keeps
// no hit points on raw stacks, so one meal-sized figure stands for all.
const LooseHP = 50

// Plant hit points (original tuning: the wiki lists them per plant and
// they are not transcribed): a bush, crop or grass tuft, and a tree.
const (
	PlantHP = 100
	TreeHP  = 300
)

// PlantPct is the share of its life (percent, the plant's damage scale)
// a plant dist from the centre loses: Against(Plant) over its hit points
// (trunk: a tree's).
func (d Def) PlantPct(dist int, trunk bool) int {
	hp := PlantHP
	if trunk {
		hp = TreeHP
	}
	return d.Against(dist, Plant) * 100 / hp
}

// Lights reports a blast that sets off the explosives it reaches (a
// bomb's shrapnel or a flame's heat); EMP and smoke leave them be.
func (d Def) Lights() bool { return d.Kind == Bomb || d.Kind == Flame }

// Cook-off (CBT-5): a stack of shells or grenades a blast reaches, or a
// fire burns over, goes off in turn CookOffFuse ticks later as one of its
// own blasts; no more than CookOffsPerTick stacks go off in one tick (the
// rest wait a tick), so a packed stockpile chains over a few ticks
// instead of in one. original tuning.
const (
	CookOffFuse     = 30
	CookOffsPerTick = 8
)

// CreatureFlammability is the chance (permille) a flame blast sets a
// creature it reaches on fire: its cell is lit even with nothing there to
// burn. original tuning.
const CreatureFlammability = 700

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
