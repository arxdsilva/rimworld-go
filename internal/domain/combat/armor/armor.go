package armor

import "rimworld-go/internal/domain/combat/damage"

// Permille is a rating or probability in thousandths; ratings above 1000
// are legal (armor can exceed 100 percent).
type Permille int

// MaxRating is the armor rating cap, 200 percent. wiki: Apparel
const MaxRating Permille = 2000

// Set is the armor ratings of a creature or garment.
type Set struct {
	Sharp, Blunt, Heat Permille
}

// Of returns the rating that resists category c (0 for NoArmor).
func (s Set) Of(c damage.Category) Permille {
	switch c {
	case damage.ArmorSharp:
		return s.Sharp
	case damage.ArmorBlunt:
		return s.Blunt
	case damage.ArmorHeat:
		return s.Heat
	}
	return 0
}

// Add sums two sets rating by rating, capped at MaxRating. It is for
// stat display; hits are resolved layer by layer with ResolveLayers.
// wiki: Apparel
func (s Set) Add(o Set) Set {
	c := func(a, b Permille) Permille {
		if a+b > MaxRating {
			return MaxRating
		}
		return a + b
	}
	return Set{c(s.Sharp, o.Sharp), c(s.Blunt, o.Blunt), c(s.Heat, o.Heat)}
}

// Outcome is how armor treated one hit.
type Outcome uint8

const (
	Passed    Outcome = iota // full damage
	Halved                   // damage reduced to half
	Deflected                // no damage
)

// RollMax is the exclusive upper bound of the caller's roll.
const RollMax = 1000

// Resolve applies one layer: with A = rating - penetration (rating
// capped at MaxRating), a roll below A/2 deflects, a roll below A halves
// the damage, any other roll passes it whole. roll is uniform in
// [0, RollMax) and comes from the caller's deterministic RNG. Halved
// damage rounds up and is at least 1 while amount > 0.
// wiki: Apparel (the wiki's 0..100 roll and "<= A" edge become the
// half-open permille form).
func Resolve(amount int, penetration, rating Permille, roll int) (Outcome, int) {
	if amount <= 0 {
		return Passed, 0
	}
	if rating > MaxRating {
		rating = MaxRating
	}
	e := int(rating - penetration)
	if e <= 0 {
		return Passed, amount
	}
	switch {
	case roll < e/2:
		return Deflected, 0
	case roll < e:
		return Halved, (amount + 1) / 2
	}
	return Passed, amount
}

// Result is a hit after armor: the damage left and its (possibly
// converted) type.
type Result struct {
	Outcome Outcome // worst outcome met on any layer (Deflected > Halved > Passed)
	Damage  int
	Type    damage.Type
}

// ResolveType resolves one layer for damage type t. A halved sharp hit
// becomes blunt. wiki: Damage ("sharp halved becomes blunt")
func ResolveType(s Set, t damage.Type, amount int, penetration Permille, roll int) Result {
	o, d := Resolve(amount, penetration, s.Of(damage.Resisted(t)), roll)
	if o == Halved && t == damage.Sharp {
		t = damage.Blunt
	}
	return Result{Outcome: o, Damage: d, Type: t}
}

// ResolveLayers runs a hit through layers listed outermost first, one
// roll per layer (missing rolls pass the layer). The damage left by a
// layer enters the next with the full penetration, each layer rating
// the original damage type; the sharp-to-blunt conversion applies to the
// final result. wiki: Apparel (layer order), Damage
func ResolveLayers(layers []Set, t damage.Type, amount int, penetration Permille, rolls []int) Result {
	res := Result{Outcome: Passed, Damage: amount, Type: t}
	cat := damage.Resisted(t)
	halved := false
	for i, l := range layers {
		if res.Damage <= 0 || i >= len(rolls) {
			break
		}
		o, d := Resolve(res.Damage, penetration, l.Of(cat), rolls[i])
		res.Damage = d
		switch o {
		case Deflected:
			res.Outcome = Deflected
		case Halved:
			halved = true
			if res.Outcome == Passed {
				res.Outcome = Halved
			}
		}
	}
	if halved && t == damage.Sharp {
		res.Type = damage.Blunt
	}
	return res
}
