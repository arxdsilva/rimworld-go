package body

import "rimworld-go/internal/domain/economy/item"

// ArtificialParts counts the installed implants.
func (b *Body) ArtificialParts() int {
	n := 0
	for _, k := range b.Implants {
		if k.IsImplant() {
			n++
		}
	}
	return n
}

// Fits reports implant kind k can replace part p.
func Fits(k item.Kind, p Part) bool {
	switch k {
	case item.PegLeg, item.ProstheticLeg:
		return p == LegL || p == LegR
	case item.ProstheticArm:
		return p == ArmL || p == ArmR
	case item.WoodenHand:
		return p == HandL || p == HandR
	case item.Denture:
		return p == Jaw
	case item.HearingTrumpet:
		return p == EarL || p == EarR
	case item.ProstheticFoot:
		return p == FootL || p == FootR
	case item.GlassEye:
		return p == EyeL || p == EyeR
	case item.WoodenNose:
		return p == Nose
	}
	return false
}

// Install puts implant k on missing part p; it reports success.
func (b *Body) Install(p Part, k item.Kind) bool {
	if p >= NumParts || !Fits(k, p) || !b.isMissing(p) || b.Implants[p].IsImplant() || b.replaced(p) {
		return false
	}
	b.Implants[p] = k
	return true
}

// Remove takes the implant off p and returns it.
func (b *Body) Remove(p Part) (item.Kind, bool) {
	if p >= NumParts || !b.Implants[p].IsImplant() {
		return 0, false
	}
	k := b.Implants[p]
	b.Implants[p] = 0
	return k, true
}

// blockedAbove reports a strict ancestor of p missing or carrying an implant,
// so nothing can be fitted on p.
func (b *Body) blockedAbove(p Part) bool {
	for q := defs[p].Parent; q != p; q = defs[q].Parent {
		if b.isMissing(q) || b.Implants[q].IsImplant() {
			return true
		}
		p = q
	}
	return false
}

// CanReplace reports p can take an implant at all: it has no implant and no
// ancestor is missing or replaced (the part itself may be present or missing).
func (b *Body) CanReplace(p Part) bool {
	return p < NumParts && !defs[p].Vital && !b.Implants[p].IsImplant() && !b.blockedAbove(p)
}

// InstallOptions lists the parts implant kind k could be fitted on now,
// present or missing.
func (b *Body) InstallOptions(k item.Kind) []Part {
	var out []Part
	for p := Part(0); p < NumParts; p++ {
		if Fits(k, p) && b.CanReplace(p) {
			out = append(out, p)
		}
	}
	return out
}

// MissingUncovered counts missing parts no implant stands in for: the ones
// that should still weigh on the pawn's mood.
func (b *Body) MissingUncovered() int {
	n := 0
	for p := Part(0); p < NumParts; p++ {
		if b.isMissing(p) && !b.Implants[p].IsImplant() && !b.replaced(p) {
			n++
		}
	}
	return n
}
