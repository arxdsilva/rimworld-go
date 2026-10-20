package weapon

// Kind is an attack profile.
type Kind uint8

const (
	Fists Kind = iota
	Club
	Rifle
	Kick // dustbuck hooves
	Bite // ridgewolf jaws
	// Knife stabs fast for little damage; Spear thrusts slow and hard.
	Knife
	Spear
	numKinds
)

// Def is one attack profile. Range is in tiles (Chebyshev); melee
// weapons reach adjacent tiles only. Cooldown is ticks between attacks.
type Def struct {
	Name     string
	Ranged   bool
	Range    int
	Damage   int
	Cooldown int
	// Accuracy is the hit chance in percent at point blank; ranged
	// weapons lose Falloff percent per tile of distance.
	Accuracy int
	Falloff  int
}

var defs = [numKinds]Def{
	Fists: {Name: "fists", Range: 1, Damage: 4, Cooldown: 90, Accuracy: 80},
	Club:  {Name: "club", Range: 1, Damage: 11, Cooldown: 110, Accuracy: 85},
	Rifle: {Name: "rifle", Ranged: true, Range: 12, Damage: 14, Cooldown: 150, Accuracy: 95, Falloff: 4},
	Kick:  {Name: "hooves", Range: 1, Damage: 3, Cooldown: 120, Accuracy: 70},
	Bite:  {Name: "bite", Range: 1, Damage: 9, Cooldown: 100, Accuracy: 80},
	Knife: {Name: "knife", Range: 1, Damage: 8, Cooldown: 70, Accuracy: 90},
	Spear: {Name: "spear", Range: 1, Damage: 13, Cooldown: 125, Accuracy: 85},
}

// DefOf returns the profile for k (fists for unknown kinds).
func DefOf(k Kind) Def {
	if k >= numKinds {
		return defs[Fists]
	}
	return defs[k]
}

// MinHitChance keeps long shots possible.
const MinHitChance = 10

// HitChance is the percent chance to hit a target dist tiles away.
func HitChance(d Def, dist int) int {
	c := d.Accuracy
	if d.Ranged && dist > 1 {
		c -= d.Falloff * (dist - 1)
	}
	if c < MinHitChance {
		c = MinHitChance
	}
	if c > 100 {
		c = 100
	}
	return c
}

// InRange reports whether a target dist tiles away (Chebyshev) can be
// attacked. Line of sight is the caller's concern.
func InRange(d Def, dist int) bool {
	if dist < 1 {
		return true
	}
	return dist <= d.Range
}

// Distance is the Chebyshev tile distance used for ranges.
func Distance(ax, ay, bx, by int) int {
	dx, dy := ax-bx, ay-by
	if dx < 0 {
		dx = -dx
	}
	if dy < 0 {
		dy = -dy
	}
	if dx > dy {
		return dx
	}
	return dy
}
