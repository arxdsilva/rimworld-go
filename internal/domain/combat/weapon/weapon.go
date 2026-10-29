package weapon

import (
	"rimworld-go/internal/domain/combat/attack"
	"rimworld-go/internal/domain/combat/damage"
	"rimworld-go/internal/domain/economy/item"
)

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
	// Animal natural weapons (SIM-019); Kick and Bite above are too.
	Claw
	Gore
	Headbutt
	Peck
	// SentryGun (PLAY-024) is a sentry turret's mounted gun: never carried,
	// fired by the building itself.
	SentryGun
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

	// Warmup is the aim time in ticks before an attack: a gun aims before
	// every burst, a melee weapon only before its first blow (legacy
	// weapons: Cooldown/3).
	Warmup int
	// Burst is shots per trigger pull and BurstTicks the gap between
	// them; legacy weapons fire one shot.
	Burst      int
	BurstTicks int
	// ArmorPen is the armor penetration in permille; 0 means the wiki's
	// calculated value (Penetration).
	ArmorPen int
	// DamageType picks the armor category that resists the hit.
	DamageType damage.Type
	// Bands is the vanilla accuracy curve. The zero value means the
	// legacy formula Accuracy - Falloff*(dist-1), floored at
	// MinHitChance (melee and the sentry gun).
	Bands attack.Bands
	// Speed is a projectile's speed in thousandths of a tile per tick
	// (0: the attack lands at once). wiki: Weapons (cells per second / 60)
	Speed int
}

// The colony's weapons carry the wiki's numbers (ENG-003): the knife,
// club and spear their main attack (blade, head, point), and the rifle
// the bolt-action rifle's, the closest vanilla rifle to a slow single-shot
// gun. Melee has no warmup (wiki: Weapons). Fists and the natural weapons
// (no published numbers) keep their original tuning, with warmup
// Cooldown/3, and armor penetration from the wiki's calculated rule
// (Penetration). wiki: Weapons, Knife, Club, Spear, Bolt-action rifle
var defs = [numKinds]Def{
	Fists: {Name: "fists", Range: 1, Damage: 4, Cooldown: 90, Accuracy: 80, Warmup: 30, Burst: 1, DamageType: damage.Blunt},
	Club:  {Name: "club", Range: 1, Damage: 14, Cooldown: 120, Accuracy: 85, Burst: 1, ArmorPen: 210, DamageType: damage.Blunt},
	Rifle: {Name: "rifle", Ranged: true, Range: 36, Damage: 18, Cooldown: 90, Warmup: 102, Burst: 1, ArmorPen: 270,
		DamageType: damage.Sharp, Bands: attack.DefaultBands(650, 800, 900, 800), Speed: 70 * 1000 / 60},
	Kick:  {Name: "hooves", Range: 1, Damage: 3, Cooldown: 120, Accuracy: 70, Warmup: 40, Burst: 1, DamageType: damage.Blunt},
	Bite:  {Name: "bite", Range: 1, Damage: 9, Cooldown: 100, Accuracy: 80, Warmup: 33, Burst: 1, DamageType: damage.Sharp},
	Knife: {Name: "knife", Range: 1, Damage: 12, Cooldown: 90, Accuracy: 90, Burst: 1, ArmorPen: 180, DamageType: damage.Sharp},
	Spear: {Name: "spear", Range: 1, Damage: 23, Cooldown: 156, Accuracy: 85, Burst: 1, ArmorPen: 500, DamageType: damage.Sharp},
	// SIM-019 S1 tunes these; damage is scaled by the animal's size.
	Claw:     {Name: "claws", Range: 1, Damage: 7, Cooldown: 110, Accuracy: 75, Warmup: 36, Burst: 1, DamageType: damage.Sharp},
	Gore:     {Name: "horns", Range: 1, Damage: 9, Cooldown: 130, Accuracy: 70, Warmup: 43, Burst: 1, DamageType: damage.Sharp},
	Headbutt: {Name: "head", Range: 1, Damage: 6, Cooldown: 130, Accuracy: 75, Warmup: 43, Burst: 1, DamageType: damage.Blunt},
	Peck:     {Name: "beak", Range: 1, Damage: 2, Cooldown: 100, Accuracy: 80, Warmup: 33, Burst: 1, DamageType: damage.Sharp},
	// PLAY-024 (our tuning, PLAY024_PLAN.md): quick, light bursts at
	// medium range. It fires instantly until DEF-3 gives turrets
	// projectiles.
	SentryGun: {Name: "sentry gun", Ranged: true, Range: 10, Damage: 9, Cooldown: 60, Accuracy: 75, Falloff: 3, Warmup: 20, Burst: 1, DamageType: damage.Sharp},
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

// Legacy reports whether d uses the pre-DEF-1 linear accuracy formula.
func (d Def) Legacy() bool { return d.Bands.Zero() }

// HitPermille is the hit chance in permille at dist with neutral
// shooter, size, light and weather factors and no cover. Legacy weapons
// reproduce HitChance exactly (percent times ten).
func HitPermille(d Def, dist int) int {
	if d.Legacy() {
		return HitChance(d, dist) * 10
	}
	const n = 1000
	return attack.HitChance(d.Bands, dist, attack.PerfectShooter, 0, n, n, n, n)
}

// PenPerDamage is the calculated armor penetration: 1.5 percent per point
// of damage (permille per point). wiki: Damage (calculated AP)
const PenPerDamage = 15

// Penetration is d's armor penetration (permille) for a hit of dmg: its
// own, or the calculated value when it has none.
func Penetration(d Def, dmg int) int {
	if d.ArmorPen > 0 {
		return d.ArmorPen
	}
	return max(dmg, 0) * PenPerDamage
}

// ForItem is the weapon an item of kind k is when wielded (fists for
// anything that is not a weapon).
func ForItem(k item.Kind) Kind {
	switch k {
	case item.Club:
		return Club
	case item.Knife:
		return Knife
	case item.Spear:
		return Spear
	case item.Rifle:
		return Rifle
	}
	return Fists
}
