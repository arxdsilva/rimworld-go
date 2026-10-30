package armor

import "rimworld-go/internal/domain/combat/damage"

// Landed is a hit after armor, on the part it struck: Hit is what got
// through (Amount left; a sharp form armor halved turns FormBlunt),
// Outcome the worst a layer did to it, and Part the body part struck
// (body.Part) when Placed. A creature with no part table (a mechanoid,
// an animal whose body picks its own part) is not Placed.
type Landed struct {
	damage.Hit
	Outcome Outcome
	Part    uint8
	Placed  bool
}

// Stopped reports that nothing got through.
func (l Landed) Stopped() bool { return l.Amount <= 0 }

// Absorb runs hit through sets listed outermost first, drawing one roll
// in [0, RollMax) from roll per set (none when sets is empty): each set
// rates the hit's original type at its full penetration, and a sharp hit
// any layer halved lands as FormBlunt. wiki: Apparel (layer order),
// Damage ("sharp halved becomes blunt")
func Absorb(sets []Set, hit damage.Hit, roll func(int) int) Landed {
	return absorb(sets, hit, roll, nil)
}

// absorb is Absorb writing the damage that reached set k, before it
// acted, to reached[k] (0 for sets it never reached; reached may be
// shorter than sets).
func absorb(sets []Set, hit damage.Hit, roll func(int) int, reached []int) Landed {
	l := Landed{Hit: hit}
	cat := damage.Resisted(hit.Type())
	halved := false
	for k, s := range sets {
		if l.Amount <= 0 {
			break
		}
		if k < len(reached) {
			reached[k] = l.Amount
		}
		o, d := Resolve(l.Amount, Permille(hit.Pen), s.Of(cat), roll(RollMax))
		l.Amount = d
		switch o {
		case Deflected:
			l.Outcome = Deflected
		case Halved:
			halved = true
			l.Outcome = max(l.Outcome, Halved)
		}
	}
	if halved && hit.Type() == damage.Sharp {
		l.Form = damage.FormBlunt
	}
	return l
}
