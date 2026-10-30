package weapon

import (
	"rimworld-go/internal/domain/combat/attack"
	"rimworld-go/internal/domain/combat/damage"
	"rimworld-go/internal/domain/combat/explosion"
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
	// DEF-2 (append only, saved in shots and logs): the vanilla catalogue.
	Mace
	Gladius
	Longsword
	ShortBow
	RecurveBow
	Greatbow
	Pila
	Revolver
	Autopistol
	MachinePistol
	AssaultRifle
	SniperRifle
	PumpShotgun
	ChainShotgun
	HeavySMG
	LMG
	Minigun
	FragGrenade
	Molotov
	EMPGrenade
	IncendiaryLauncher
	EMPLauncher
	// DEF-3 (append only, saved in shots and logs): the heavy turrets'
	// guns and the mortar, one kind per shell it lobs (all named
	// "mortar"). Never carried: fired by their buildings.
	Autocannon
	SlugTurret
	MortarHE
	MortarIncendiary
	MortarEMP
	MortarSmoke
	// Wave E contract (append only): the mechanoids' built-in weapons,
	// never carried or dropped. The heavy gunner mounts the minigun.
	MechBlades
	PulseLance
	NeedleCaster
	// STO/CBT contract (append only, saved in shots and logs): the carried
	// gear CBT-3 adds, one per item of the same name (item.RocketLauncher
	// to item.Ikwa, ForItem). No def rows yet: CBT-3 writes them with the
	// wiki numbers and flips item.IsWeapon, so nobody wields one before.
	RocketLauncher
	TripleRocketLauncher
	SmokeLauncher
	ChargeRifle
	ChargeLance
	Ikwa
	numKinds
)

// lastCatalogue is the last carried weapon of the DEF-2 catalogue (the
// kinds after it are building and mechanoid guns).
const lastCatalogue = EMPLauncher

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
	// Blast is what an explosive's shot does where it lands (grenades and
	// launchers, DEF-2); the zero value is no blast. An explosive never
	// rolls to hit: it lands on its target, scattered by ForcedMiss.
	Blast explosion.Def
	// ForcedMiss is an explosive's scatter radius at long range,
	// thousandths of a tile (explosion.ForcedMiss).
	ForcedMiss int
	// MinRange is the nearest a target may be (tiles; 0: none): the heavy
	// turrets and the mortar cannot shoot what is too close (DEF-3).
	MinRange int
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

	// DEF-2 melee, steel at Normal quality, main attack (head, point,
	// blade); no warmup. Accuracy is our flat melee base, as the club's.
	// wiki: Mace, Gladius, Longsword
	Mace:      {Name: "mace", Range: 1, Damage: 16, Cooldown: 120, Accuracy: 85, Burst: 1, ArmorPen: 240, DamageType: damage.Blunt},
	Gladius:   {Name: "gladius", Range: 1, Damage: 16, Cooldown: 120, Accuracy: 88, Burst: 1, ArmorPen: 240, DamageType: damage.Sharp},
	Longsword: {Name: "longsword", Range: 1, Damage: 23, Cooldown: 156, Accuracy: 85, Burst: 1, ArmorPen: 340, DamageType: damage.Sharp},

	// DEF-2 ranged: damage, AP, warmup, cooldown, range (tiles, the
	// wiki's x.9 floored), burst x gap, accuracy touch/short/medium/long
	// and projectile speed. wiki: Weapons
	ShortBow:      ranged("short bow", 11, 160, 81, 99, 22, 1, 0, 750, 650, 450, 250, 44),
	RecurveBow:    ranged("recurve bow", 14, 210, 87, 99, 25, 1, 0, 700, 780, 650, 350, 56),
	Greatbow:      ranged("greatbow", 17, 150, 120, 90, 29, 1, 0, 650, 850, 750, 500, 49),
	Pila:          ranged("pila", 25, 100, 240, 150, 18, 1, 0, 800, 710, 500, 320, 26),
	Revolver:      ranged("revolver", 12, 180, 18, 96, 25, 1, 0, 800, 750, 550, 400, 55),
	Autopistol:    ranged("autopistol", 10, 150, 18, 60, 25, 1, 0, 800, 700, 400, 300, 55),
	MachinePistol: ranged("machine pistol", 6, 90, 30, 54, 19, 3, 7, 900, 650, 350, 150, 55),
	AssaultRifle:  ranged("assault rifle", 11, 160, 60, 102, 30, 3, 10, 600, 700, 650, 550, 70),
	SniperRifle:   ranged("sniper rifle", 25, 380, 210, 90, 44, 1, 0, 500, 700, 880, 900, 100),
	PumpShotgun:   ranged("pump shotgun", 18, 140, 54, 75, 15, 1, 0, 800, 870, 770, 640, 55),
	ChainShotgun:  ranged("chain shotgun", 18, 140, 72, 81, 12, 3, 10, 570, 640, 550, 450, 55),
	HeavySMG:      ranged("heavy smg", 12, 180, 54, 99, 22, 3, 11, 850, 650, 350, 200, 48),
	LMG:           ranged("lmg", 12, 160, 108, 96, 25, 6, 7, 400, 480, 350, 260, 46),
	Minigun:       ranged("minigun", 10, 150, 150, 90, 30, 25, 5, 200, 250, 250, 180, 70),

	// DEF-2 explosives: thrown grenades (12 cells/s) and launchers
	// (40 cells/s), forced-miss radius 1.9. wiki: Weapons, Frag grenades,
	// Molotov cocktail, EMP grenade, Incendiary launcher, EMP launcher
	FragGrenade:        explosive("frag grenade", explosion.Bomb, 1900, 50, 100, 90, 160, 12, 12),
	Molotov:            explosive("molotov cocktail", explosion.Flame, 1100, 10, 0, 90, 160, 12, 12),
	EMPGrenade:         explosive("EMP grenade", explosion.EMP, 3500, 50, 0, 90, 160, 12, 12),
	IncendiaryLauncher: explosive("incendiary launcher", explosion.Flame, 1100, 10, 0, 210, 210, 23, 40),
	EMPLauncher:        explosive("emp launcher", explosion.EMP, 1100, 50, 0, 210, 210, 23, 40),

	// DEF-3 heavy turrets: damage, AP, no warmup, cooldown, range and
	// minimum range (the wiki's x.9 floored), burst x gap, accuracy
	// touch/short/medium/long (the slug turret's odd curve as published).
	// The turrets fire at once (no projectile), like the sentry gun.
	// wiki: Autocannon turret, Uranium slug turret
	Autocannon: turretGun("autocannon", 27, 400, 210, 32, 8, 3, 17, 280, 720, 660, 500),
	SlugTurret: turretGun("slug turret", 55, 820, 240, 45, 11, 1, 0, 220, 330, 440, 950),

	// DEF-3 mortar: range 500 (wider than any map), minimum range 29.9,
	// warmup 240, cooldown 1680, shells at 41 cells/s, forced-miss radius
	// 9 tiles (scaled by the crew's skill, Mortars). Blasts: HE 50 bomb
	// r 2.9, incendiary 10 flame r 2.9, EMP 50 r 8.9, smoke r 7.2. The HE
	// shell's armor penetration is not in the notes: the frag grenade's
	// 10 % (original assumption). wiki: Mortar, High-explosive shell,
	// Incendiary shell, EMP shell, Smoke shell
	MortarHE:         mortar(explosion.Def{Kind: explosion.Bomb, Radius: 2900, Damage: 50, Pen: 100}),
	MortarIncendiary: mortar(explosion.Def{Kind: explosion.Flame, Radius: 2900, Damage: 10}),
	MortarEMP:        mortar(explosion.Def{Kind: explosion.EMP, Radius: 8900, Damage: 50}),
	MortarSmoke:      mortar(explosion.Def{Kind: explosion.Smoke, Radius: 7200}),

	// Wave E mechanoid weapons (ENG-003, our names). Blades: 20 sharp, 30
	// % AP, 2 s (wiki: Scyther), our flat melee accuracy. The lance and
	// the needle caster as ranged rows (wiki: Charge lance, Needle gun);
	// projectile speeds are ours (the wiki gives none).
	MechBlades:   {Name: "edge blades", Range: 1, Damage: 20, Cooldown: 120, Accuracy: 85, Burst: 1, ArmorPen: 300, DamageType: damage.Sharp},
	PulseLance:   ranged("pulse lance", 30, 450, 102, 162, 32, 1, 0, 650, 850, 850, 750, 70),
	NeedleCaster: ranged("needle caster", 15, 350, 150, 126, 44, 1, 0, 600, 800, 900, 850, 70),
}

// turretGun is a heavy turret's gun: name, damage, armor penetration
// (permille), cooldown, range and minimum range (tiles), burst shots and
// the ticks between them, accuracy at touch/short/medium/long (permille).
func turretGun(name string, dmg, pen, cooldown, rng, minRng, burst, gap, t, sh, m, l int) Def {
	return Def{Name: name, Ranged: true, Range: rng, MinRange: minRng, Damage: dmg, ArmorPen: pen, Cooldown: cooldown,
		Burst: burst, BurstTicks: gap, DamageType: damage.Sharp, Bands: attack.DefaultBands(t, sh, m, l)}
}

// The mortar's numbers. wiki: Mortar
const (
	MortarRange      = 500
	MortarMinRange   = 29
	mortarWarmup     = 240
	mortarCooldown   = 1680
	mortarCellsPerS  = 41
	MortarForcedMiss = 9000 // thousandths of a tile, before the crew's skill
)

// mortar is the mortar lobbing a shell that bursts as b.
func mortar(b explosion.Def) Def {
	return Def{Name: "mortar", Ranged: true, Range: MortarRange, MinRange: MortarMinRange, Damage: b.Damage, ArmorPen: b.Pen,
		Warmup: mortarWarmup, Cooldown: mortarCooldown, Burst: 1, Accuracy: 100, DamageType: b.Type(),
		Speed: mortarCellsPerS * 1000 / 60, Blast: b, ForcedMiss: MortarForcedMiss}
}

// ForShell is the mortar weapon that lobs shell kind k (ok false: no
// shell).
func ForShell(k item.Kind) (Kind, bool) {
	if !k.IsShell() {
		return Fists, false
	}
	return MortarHE + Kind(k-item.MortarShells), true
}

// ranged is a banded gun or bow: name, damage, armor penetration
// (permille), warmup and cooldown (ticks), range (tiles), burst shots and
// the ticks between them, accuracy at touch/short/medium/long (permille)
// and projectile speed (cells per second). Bows and the pila deal sharp
// damage like bullets (the wiki's arrow type).
func ranged(name string, dmg, pen, warmup, cooldown, rng, burst, gap, t, sh, m, l, cps int) Def {
	return Def{Name: name, Ranged: true, Range: rng, Damage: dmg, ArmorPen: pen, Warmup: warmup, Cooldown: cooldown,
		Burst: burst, BurstTicks: gap, DamageType: damage.Sharp, Bands: attack.DefaultBands(t, sh, m, l), Speed: cps * 1000 / 60}
}

// explosiveMiss is the grenades' and launchers' forced-miss radius
// (thousandths of a tile). wiki: Weapons
const explosiveMiss = 1900

// explosive is a grenade or launcher: its blast (kind, radius, damage,
// penetration), warmup, cooldown, range and projectile speed (cells per
// second). It has no accuracy: it always flies at its target.
func explosive(name string, k explosion.Kind, radius, dmg, pen, warmup, cooldown, rng, cps int) Def {
	b := explosion.Def{Kind: k, Radius: radius, Damage: dmg, Pen: pen}
	return Def{Name: name, Ranged: true, Range: rng, Damage: dmg, ArmorPen: pen, Warmup: warmup, Cooldown: cooldown,
		Burst: 1, Accuracy: 100, DamageType: b.Type(), Speed: cps * 1000 / 60, Blast: b, ForcedMiss: explosiveMiss}
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
		return d.MinRange == 0
	}
	return dist <= d.Range && dist >= d.MinRange
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
// anything that is not a weapon). The DEF-2 catalogue lies in the same
// order in both enums.
func ForItem(k item.Kind) Kind {
	switch {
	case k == item.Club:
		return Club
	case k == item.Knife:
		return Knife
	case k == item.Spear:
		return Spear
	case k == item.Rifle:
		return Rifle
	case k >= item.Mace && k <= item.EMPLauncher:
		return Mace + Kind(k-item.Mace)
	case k.IsContractWeapon(): // same order in both enums
		return RocketLauncher + Kind(k-item.RocketLauncher)
	}
	return Fists
}

// Explosive reports a weapon whose shot bursts where it lands.
func (d Def) Explosive() bool { return !d.Blast.Zero() }

// IsExplosive is DefOf(k).Explosive without copying the profile (a shot
// asks every cell it flies).
func IsExplosive(k Kind) bool { return k < numKinds && !defs[k].Blast.Zero() }

// View is a weapon's numbers for the gear tab (read model): Damage and
// ArmorPen (permille) per hit, Warmup and Cooldown in ticks, Burst shots
// per trigger pull, Range in tiles, Accuracy in permille at the touch,
// short, medium and long band (zero for melee and explosives); an
// explosive's Blast kind ("bomb", "flame", "EMP") and BlastRadius
// (tenths of a tile).
type View struct {
	Name             string
	Damage, ArmorPen int
	Warmup, Cooldown int
	Burst, Range     int
	Accuracy         [4]int
	Melee            bool
	Blast            string
	BlastRadius      int
}

// Stats is weapon k's View.
func Stats(k Kind) View {
	d := DefOf(k)
	v := View{Name: d.Name, Damage: d.Damage, ArmorPen: Penetration(d, d.Damage), Warmup: d.Warmup, Cooldown: d.Cooldown,
		Burst: attack.Burst{Shots: d.Burst}.Count(), Range: d.Range, Melee: !d.Ranged,
		Blast: d.Blast.Kind.String(), BlastRadius: d.Blast.Radius / 100}
	if d.Ranged && !d.Explosive() {
		for i, dist := range [4]int{3, 12, 25, 40} {
			v.Accuracy[i] = HitPermille(d, dist)
			if !d.Legacy() {
				v.Accuracy[i] = d.Bands.Accuracy(dist)
			}
		}
	}
	return v
}
