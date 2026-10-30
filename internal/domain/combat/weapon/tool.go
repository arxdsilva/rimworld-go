package weapon

import "rimworld-go/internal/domain/combat/damage"

// Tool is one melee attack a weapon or a body offers (CBT-2): a knife's
// handle, blade and point, a rifle's stock, a human's fist or teeth.
// Damage per blow, Pen the armor penetration (permille), Cooldown the
// ticks after the blow, Form the damage it deals and Chance its weight
// when a blow picks its tool (Pick).
type Tool struct {
	Name     string
	Damage   int
	Pen      int
	Cooldown int
	Form     damage.Form
	Chance   int
}

// tool is a Tool from the wiki's columns: name, damage, form, armor
// penetration (percent), cooldown (ticks) and chance (permille).
func tool(name string, dmg int, f damage.Form, pen, cooldown, chance int) Tool {
	return Tool{Name: name, Damage: dmg, Pen: pen * 10, Cooldown: cooldown, Form: f, Chance: chance}
}

// The melee weapons' tools at Normal steel (the club's at steel too): the
// wiki's damage, form, armor penetration, cooldown and chance; a poke is
// blunt. The longsword's chances are not published: the gladius's split
// (original assumption). The wiki's tables also give the wielder's fists
// a share; a blow here always uses the weapon (their share is spread over
// its tools). The main attack comes first. wiki: Knife, Club, Spear, Mace, Gladius, Longsword, Ikwa
var (
	knifeTools = []Tool{
		tool("blade", 12, damage.FormCut, 18, 90, 750),
		tool("handle", 9, damage.FormBlunt, 13, 120, 63),
		tool("point", 13, damage.FormStab, 20, 120, 63),
	}
	clubTools = []Tool{
		tool("head", 14, damage.FormBlunt, 21, 120, 750),
		tool("handle", 9, damage.FormBlunt, 13, 120, 83),
	}
	spearTools = []Tool{
		tool("point", 23, damage.FormStab, 50, 156, 750),
		tool("shaft", 13, damage.FormBlunt, 20, 156, 63),
		tool("shaft", 13, damage.FormBlunt, 20, 156, 63),
	}
	maceTools = []Tool{
		tool("head", 16, damage.FormBlunt, 24, 120, 750),
		tool("handle", 9, damage.FormBlunt, 13, 120, 83),
	}
	gladiusTools = []Tool{
		tool("point", 16, damage.FormStab, 24, 120, 375),
		tool("edge", 16, damage.FormCut, 24, 120, 375),
		tool("handle", 9, damage.FormBlunt, 13, 120, 83),
	}
	longswordTools = []Tool{
		tool("blade", 23, damage.FormCut, 34, 156, 375),
		tool("point", 23, damage.FormStab, 34, 156, 375),
		tool("handle", 9, damage.FormBlunt, 13, 120, 83),
	}
	// The ikwa's head and edge (15 stab and cut, 22 %) and its handle
	// (9 blunt, a blow and a poke). wiki: Ikwa
	ikwaTools = []Tool{
		tool("head", 15, damage.FormStab, 22, 120, 375),
		tool("edge", 15, damage.FormCut, 22, 120, 375),
		tool("handle", 9, damage.FormBlunt, 13, 120, 63),
		tool("handle", 9, damage.FormBlunt, 13, 120, 63),
	}
	// A gun's stock and barrel, a pistol's grip, a bow's limb, the pila's
	// shaft: 9 blunt, 13 %, 2 s each, chosen evenly. wiki: Assault rifle
	// (stock, barrel), the other ranged weapons' pages alike
	gunTools    = []Tool{tool("stock", 9, damage.FormBlunt, 13, 120, 500), tool("barrel", 9, damage.FormBlunt, 13, 120, 500)}
	pistolTools = []Tool{tool("grip", 9, damage.FormBlunt, 13, 120, 500), tool("barrel", 9, damage.FormBlunt, 13, 120, 500)}
	bowTools    = []Tool{tool("limb", 9, damage.FormBlunt, 13, 120, 1000)}
	shaftTools  = []Tool{tool("shaft", 9, damage.FormBlunt, 13, 120, 1000)}
	// A human's bare hands: the fists keep their original tuning (the
	// fists row), the head and teeth the wiki's numbers and chance factors
	// (a fist 1 each, head 0.2, teeth 0.07). wiki: Human
	fistTools = []Tool{
		{Name: "fist", Damage: defs[Fists].Damage, Pen: Penetration(defs[Fists], defs[Fists].Damage), Cooldown: defs[Fists].Cooldown, Form: damage.FormBlunt, Chance: 2000},
		tool("head", 5, damage.FormBlunt, 7, 120, 200),
		tool("teeth", 8, damage.FormBite, 12, 120, 70),
	}
)

// ToolsOf is the melee tools a humanlike wielding k fights with (a slice
// the caller must not change): the weapon's, a gun's stock, its bare
// hands for fists, a grenade or a mechanoid's or building's gun. A weapon
// with one attack (the mechanoids' blades) has none: its def is its tool.
func ToolsOf(k Kind) []Tool {
	switch k {
	case Knife:
		return knifeTools
	case Club:
		return clubTools
	case Spear:
		return spearTools
	case Mace:
		return maceTools
	case Gladius:
		return gladiusTools
	case Longsword:
		return longswordTools
	case Ikwa:
		return ikwaTools
	case Revolver, Autopistol, MachinePistol:
		return pistolTools
	case ShortBow, RecurveBow, Greatbow:
		return bowTools
	case Pila:
		return shaftTools
	case Fists, FragGrenade, Molotov, EMPGrenade:
		return fistTools
	}
	if k < numKinds && k != SentryGun && defs[k].Ranged && (k <= lastCatalogue || k >= RocketLauncher) {
		return gunTools
	}
	return nil
}

// Stock reports whether a wielder of k fights a target beside it with a
// tool rather than shooting it point-blank: a carried gun, bow or
// grenade.
func Stock(k Kind) bool { return k < numKinds && defs[k].Ranged && ToolsOf(k) != nil }

// natural is a creature's second attack beside its roster attack: paws
// beside jaws, a kick beside horns. original tuning: the roster gives one
// attack per species; the second hits for secondShare of the first and is
// picked a quarter of the time.
var natural = [numKinds]Kind{
	Bite: Claw, Claw: Bite, Gore: Kick, Kick: Headbutt, Headbutt: Kick, Peck: Claw,
}

const (
	secondShare  = 60 // percent of the primary's damage
	primaryOdds  = 750
	secondaryOdd = 250
)

// NaturalTools is a creature's attacks with its roster attack primary
// (d: that attack's profile scaled to the creature) and, for the natural
// weapons, a second one (n is how many are set).
func NaturalTools(primary Kind, d Def) (ts [2]Tool, n int) {
	h := HitOf(primary, d)
	ts[0] = Tool{Name: d.Name, Damage: d.Damage, Pen: h.Pen, Cooldown: d.Cooldown, Form: h.Form, Chance: primaryOdds}
	if primary >= numKinds || natural[primary] == Fists {
		ts[0].Chance = 1000
		return ts, 1
	}
	k := natural[primary]
	sd := defs[k]
	sd.Damage = max(1, d.Damage*secondShare/100)
	sh := HitOf(k, sd)
	ts[1] = Tool{Name: sd.Name, Damage: sd.Damage, Pen: sh.Pen, Cooldown: sd.Cooldown, Form: sh.Form, Chance: secondaryOdd}
	return ts, 2
}

// Pick chooses a blow's tool from ts by their chances, one draw from roll
// when there is a choice (none for one tool). ts must not be empty.
func Pick(ts []Tool, roll func(int) int) Tool {
	if len(ts) == 1 {
		return ts[0]
	}
	sum := 0
	for _, t := range ts {
		sum += max(t.Chance, 0)
	}
	if sum <= 0 {
		return ts[0]
	}
	r := roll(sum)
	for _, t := range ts {
		if r -= max(t.Chance, 0); r < 0 {
			return t
		}
	}
	return ts[len(ts)-1]
}

// Melee is d as a melee blow with tool t: adjacent reach, no aim, one
// blow, the tool's damage, penetration, cooldown and type. It keeps d's
// name (the weapon the blow is dealt with).
func (t Tool) Melee(d Def) Def {
	return Def{Name: d.Name, Range: 1, Damage: t.Damage, Cooldown: t.Cooldown, Accuracy: d.Accuracy, Burst: 1,
		ArmorPen: t.Pen, DamageType: t.Form.Type()}
}

// Hit is a blow with t of amount (after the melee damage factor).
func (t Tool) Hit(weapon string, amount int) damage.Hit {
	return damage.Hit{Weapon: weapon, Form: t.Form, Amount: amount, Pen: t.Pen}
}
