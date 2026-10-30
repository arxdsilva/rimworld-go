package animal

import (
	"rimworld-go/internal/domain/combat/armor"
	"rimworld-go/internal/domain/combat/weapon"
)

// Mech is a mechanoid's numbers (wave E contract, ENG-003: wiki numbers
// for the vanilla roles, our own designs and names). buildDef applies them
// (applyMech, DEF-4): hit points, pace, weapon, no meat or leather.
type Mech struct {
	Role          string    // the combat role it fills
	HealthPct     int       // health scale, percent of a standard body
	BodySize      int       // body size, tenths
	Speed         int       // move speed, hundredths of a cell per second
	Armor         armor.Set // sharp, blunt, heat (permille)
	CombatPower   int       // raid points it costs
	Weapon        weapon.Kind
	EMPVulnerable bool // an EMP blast stuns it (EMPStunPerDamage)
}

// EMP on mechanoids. wiki: Damage (EMP stun 0.25 s per point), Mechanoid
// (2200 ticks of adapted immunity after a stun).
const (
	EMPStunPerDamage = 15 // ticks of stun per point of EMP damage
	EMPAdaptTicks    = 2200
)

// mechs is each mechanoid's row. wiki: Scyther (Harrow), Lancer
// (Spindle), Centipede (Bulwark, the gunner variant's minigun), Pikeman
// (Quill).
var mechs = [numSpecies]Mech{
	Harrow: {Role: "blade", HealthPct: 132, BodySize: 10, Speed: 470,
		Armor: armor.Set{Sharp: 400, Blunt: 200, Heat: 2000}, CombatPower: 150, Weapon: weapon.MechBlades, EMPVulnerable: true},
	Spindle: {Role: "sniper", HealthPct: 72, BodySize: 10, Speed: 470,
		Armor: armor.Set{Sharp: 400, Blunt: 200, Heat: 2000}, CombatPower: 190, Weapon: weapon.PulseLance, EMPVulnerable: true},
	Bulwark: {Role: "heavy gunner", HealthPct: 432, BodySize: 30, Speed: 190,
		Armor: armor.Set{Sharp: 720, Blunt: 220, Heat: 2000}, CombatPower: 400, Weapon: weapon.Minigun, EMPVulnerable: true},
	Quill: {Role: "skirmisher", HealthPct: 85, BodySize: 10, Speed: 250,
		Armor: armor.Set{Sharp: 400, Blunt: 200, Heat: 2000}, CombatPower: 110, Weapon: weapon.NeedleCaster, EMPVulnerable: true},
}

// Salvage is what shredding a mechanoid's wreck gives (DEF-4).
type Salvage struct {
	Steel, Plasteel, Components int
}

// wrecks is each mechanoid's shredding yield. wiki: Scyther, Lancer,
// Pikeman (15 steel each), Centipede (30 steel and 10 plasteel); the wiki
// lists no components for any of them.
var wrecks = [numSpecies]Salvage{
	Harrow:  {Steel: 15},
	Spindle: {Steel: 15},
	Bulwark: {Steel: 30, Plasteel: 10},
	Quill:   {Steel: 15},
}

// WreckYield is what shredding one wreck of s gives (zero for an animal).
// A wreck is an item.CorpseAnimal stack whose Animal is a mechanoid
// (IsMech): it never rots, is never eaten and never butchered for meat.
func WreckYield(s Species) Salvage {
	if !IsMech(s) {
		return Salvage{}
	}
	return wrecks[s]
}

// Mechanoid combat. wiki: Lancer, Centipede (shooting accuracy 96 %, a
// Shooting skill of 8), Scyther (melee hit chance 62 %).
const (
	MechShootingSkill = 8
	MechMeleeHit      = 62
)

// A human's numbers the mechanoid rows scale against: hit points
// (pawn/health.MaxHP), move speed (wiki: Human, 4.6 cells a second) and
// the colonist's ticks per cell (game.TicksPerCell).
const (
	humanHP    = 100
	humanSpeed = 460
	humanStep  = 15
)

// IsMech reports a mechanoid species.
func IsMech(s Species) bool { return s < numSpecies && roster[s].Behaviour == Mechanoid }

// applyMech gives def d mechanoid m's numbers: health scaled from a
// human's, pace from its move speed, its weapon's damage and cooldown,
// and no meat or leather (it leaves a wreck).
func applyMech(d *Def, m Mech) {
	d.Mech = true
	d.MaxHP = humanHP * m.HealthPct / 100
	d.StepTicks = (humanStep*humanSpeed + m.Speed/2) / m.Speed
	d.FleeStep = (d.StepTicks*fleePermill + 500) / 1000
	d.Meat, d.Leather = 0, 0
	d.TameChance, d.Revenge, d.TameRevenge = 0, 0, 0
	wd := weapon.DefOf(m.Weapon)
	d.Attack, d.Damage, d.Cooldown, d.Accuracy = m.Weapon, wd.Damage, wd.Cooldown, wd.Accuracy
	if !wd.Ranged {
		d.Accuracy = MechMeleeHit
	}
}

// Layer is m's plating as an armor layer over its whole body.
func (m Mech) Layer() armor.Layer {
	return armor.Layer{Set: m.Armor, Covers: armor.Head | armor.Torso | armor.Legs}
}

// MechOf is s's mechanoid row; ok is false for an animal.
func MechOf(s Species) (Mech, bool) {
	if !IsMech(s) {
		return Mech{}, false
	}
	return mechs[s], true
}
