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
