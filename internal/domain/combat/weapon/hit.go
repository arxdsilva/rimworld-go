package weapon

import "rimworld-go/internal/domain/combat/damage"

// hitTraits is what a weapon's hit carries beyond its def: the damage form
// (0 keeps the def's type: a sharp gun shoots bullets, a sharp blade
// cuts) and the stopping power in permille of body size (0: the default
// for its kind).
type hitTraits struct {
	form     damage.Form
	stopping int
}

// defaultStopping is a projectile's stopping power when its weapon names
// none. wiki: Weapons (stopping power 0.5 for most projectiles)
const defaultStopping = 500

// traits per weapon. wiki: Weapons (stop pwr column; arrow damage for the
// bows and the pila), Damage types (the natural weapons' forms); the
// knife cuts with its edge here and the beak scratches (original tuning:
// one tool per weapon until CBT-2 adds tools).
var traits = [numKinds]hitTraits{
	Bite: {form: damage.FormBite}, Spear: {form: damage.FormStab}, Ikwa: {form: damage.FormStab},
	Claw: {form: damage.FormScratch}, Gore: {form: damage.FormStab}, Peck: {form: damage.FormScratch},
	ShortBow: {damage.FormArrow, 500}, RecurveBow: {damage.FormArrow, 1000}, Greatbow: {damage.FormArrow, 1500}, Pila: {damage.FormArrow, 2500},
	Revolver: {stopping: 1000}, Rifle: {stopping: 1500}, SniperRifle: {stopping: 1500},
	PumpShotgun: {stopping: 3000}, ChainShotgun: {stopping: 3000}, LMG: {stopping: 1000}, ChargeLance: {stopping: 1500},
}

// HitOf is the hit weapon k with profile d (DefOf(k), or an animal's
// natural weapon scaled to its owner) deals at full damage, before armor:
// its name, form, damage, penetration and stopping power. Melee blows
// never stagger.
func HitOf(k Kind, d Def) damage.Hit {
	var t hitTraits
	if k < numKinds {
		t = traits[k]
	}
	h := damage.Hit{Weapon: d.Name, Form: t.form, Amount: d.Damage, Pen: Penetration(d, d.Damage), Stopping: t.stopping}
	if t.form == 0 {
		h.Form = damage.FormOf(d.DamageType)
		if d.DamageType == damage.Sharp && d.Ranged {
			h.Form = damage.FormBullet
		}
	}
	if d.Ranged && h.Stopping == 0 && h.Form.Type() != damage.EMP && h.Form != damage.FormFlame {
		h.Stopping = defaultStopping
	}
	if !d.Ranged {
		h.Stopping = 0
	}
	return h
}
