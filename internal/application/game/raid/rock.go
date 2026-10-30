package raid

// RockHP is the damage a rock tile takes before a sapper opens it up.
const RockHP = 90

// Rock is the rock tiles sappers are hacking through (the save's
// RockDamage).
type Rock struct {
	h      Host
	damage map[[2]int]int
}

// Dig: a sapper hacks at a rock tile; it opens up at zero.
func (r *Rock) Dig(x, y, dmg int) {
	if !r.h.Rock(x, y) {
		return
	}
	k := [2]int{x, y}
	r.damage[k] += dmg
	if r.damage[k] < RockHP {
		return
	}
	delete(r.damage, k)
	r.h.Mine(x, y)
}

// Export is the damage for the save (the live map, never nil).
func (r *Rock) Export() map[[2]int]int { return r.damage }

// Import restores saved damage (nil, an older save: kept as it is).
func (r *Rock) Import(m map[[2]int]int) {
	if m != nil {
		r.damage = m
	}
}
