package body

import "rimworld-go/internal/domain/economy/item"

// ArtificialParts counts the installed implants.
func (b *Body) ArtificialParts() int {
	n := 0
	for _, k := range b.Fitted {
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
	case item.ServoArm: // MED-05: the servo tier and the valve heart
		return p == ArmL || p == ArmR
	case item.ServoLeg:
		return p == LegL || p == LegR
	case item.OpticImplant:
		return p == EyeL || p == EyeR
	case item.CochlearImplant:
		return p == EarL || p == EarR
	case item.SpinalBrace:
		return p == Spine
	case item.PumpHeart, item.ValveHeart:
		return p == Heart
	case item.SynthStomach:
		return p == Stomach
	}
	return false
}

// OrganOf is the natural organ item part p is (0: not a harvestable
// organ). MED-05 adds the kinds; MED-06's harvest and install operations
// use it. Organs are not implants: Fits and Install never take them.
func OrganOf(p Part) item.Kind {
	switch p {
	case KidneyL, KidneyR:
		return item.Kidney
	case LungL, LungR:
		return item.Lung
	case Heart:
		return item.Heart
	case Liver:
		return item.Liver
	}
	return 0
}

// FitsOrgan reports natural organ k can take part p's place.
func FitsOrgan(k item.Kind, p Part) bool { return k.IsOrgan() && OrganOf(p) == k }

// HeartAttackImmune reports an artificial heart (pump or valve), which
// never has a heart attack (MED-08 reads it).
func (b *Body) HeartAttackImmune() bool {
	k := b.Implant(Heart)
	return b.fitted(Heart) && (k == item.PumpHeart || k == item.ValveHeart)
}

// GutWormImmune reports a synth stomach, which gut worms cannot live in
// (MED-04 reads it).
func (b *Body) GutWormImmune() bool {
	return b.fitted(Stomach) && b.Implant(Stomach) == item.SynthStomach
}

// SpineBraced reports a spinal brace, which ends a bad back (MED-08
// reads it).
func (b *Body) SpineBraced() bool { return b.fitted(Spine) && b.Implant(Spine) == item.SpinalBrace }

// Install puts implant k on missing part p; it reports success.
func (b *Body) Install(p Part, k item.Kind) bool {
	if p >= NumParts || !Fits(k, p) || !b.isMissing(p) || b.fitted(p) || b.replaced(p) {
		return false
	}
	b.SetImplant(p, k)
	return true
}

// Remove takes the implant off p and returns it.
func (b *Body) Remove(p Part) (item.Kind, bool) {
	if p >= NumParts || !b.fitted(p) {
		return 0, false
	}
	k := b.Implant(p)
	b.SetImplant(p, 0)
	return k, true
}

// blockedAbove reports a strict ancestor of p missing or carrying an implant,
// so nothing can be fitted on p.
func (b *Body) blockedAbove(p Part) bool {
	for q := defs[p].Parent; q != p; q = defs[q].Parent {
		if b.isMissing(q) || b.fitted(q) {
			return true
		}
		p = q
	}
	return false
}

// CanReplace reports p can take an implant at all: it has no implant and no
// ancestor is missing or replaced (the part itself may be present or
// missing). Of the vital parts only the heart can, with an artificial
// heart (MED-05).
func (b *Body) CanReplace(p Part) bool {
	return p < NumParts && (!defs[p].Vital || p == Heart) && !b.fitted(p) && !b.blockedAbove(p)
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
		if b.isMissing(p) && !b.fitted(p) && !b.replaced(p) {
			n++
		}
	}
	return n
}
