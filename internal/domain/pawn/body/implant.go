// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package body

import "rimworld-go/internal/domain/economy/item"

// ArtificialParts counts the installed implants.
func (b *Body) ArtificialParts() int {
	n := 0
	for _, r := range b.Replacements {
		if r.Kind.IsReplacement() {
			n++
		}
	}
	return n + len(b.Implants)
}

// Body part and implant rules are defined once for surgery, efficiency,
// medical skill gates and removal yields. Bitmasks use the saved Part IDs.
type artificialDef struct {
	Parts      uint32
	Efficiency int32
	MinMedical uint8
	Legacy     bool
	Removable  bool
	Return     item.Kind
	ReturnQty  uint8
}

const (
	arms  = 1<<ArmL | 1<<ArmR
	hands = 1<<HandL | 1<<HandR
	legs  = 1<<LegL | 1<<LegR
	feet  = 1<<FootL | 1<<FootR
	eyes  = 1<<EyeL | 1<<EyeR
	ears  = 1<<EarL | 1<<EarR
)

var artificialDefs = [item.NumKinds]artificialDef{
	// wiki: Peg leg (part compatibility and efficiency); original tuning: medical threshold and recovered item.
	item.PegLeg: {Parts: legs, Efficiency: 600, MinMedical: 3, Return: item.Wood, ReturnQty: 1},
	// wiki: Prosthetic leg (part compatibility and efficiency); original tuning: medical threshold and recovered item.
	item.ProstheticLeg: {Parts: legs, Efficiency: 850, MinMedical: 4, Return: item.ProstheticLeg, ReturnQty: 1},
	// wiki: Prosthetic arm (part compatibility and efficiency); original tuning: medical threshold and recovered item.
	item.ProstheticArm: {Parts: arms, Efficiency: 500, MinMedical: 4, Return: item.ProstheticArm, ReturnQty: 1},
	// wiki: Wooden hand (part compatibility and efficiency); original tuning: medical threshold and recovered item.
	item.WoodenHand: {Parts: hands, Efficiency: 600, MinMedical: 3, Return: item.Wood, ReturnQty: 1},
	// wiki: Denture (part compatibility and efficiency); original tuning: medical threshold and recovered item.
	item.Denture: {Parts: 1 << Jaw, Efficiency: 800, MinMedical: 3},
	// original tuning: retired fictional part; fitting, efficiency, skill and recovery.
	item.HearingTrumpet: {Parts: ears, Legacy: true, Efficiency: 600, MinMedical: 4, Return: item.HearingTrumpet, ReturnQty: 1},
	// wiki: Wooden foot (part compatibility and efficiency); original tuning: medical threshold and recovered item.
	item.ProstheticFoot: {Parts: feet, Efficiency: 800, MinMedical: 3, Return: item.Wood, ReturnQty: 1},
	// original tuning: retired fictional part; fitting, efficiency, skill and recovery.
	item.GlassEye: {Parts: eyes, Legacy: true, Efficiency: 500, MinMedical: 4, Return: item.GlassEye, ReturnQty: 1},
	// original tuning: retired fictional part; fitting, efficiency, skill and recovery.
	item.WoodenNose: {Parts: 1 << Nose, Legacy: true, Efficiency: 1000, MinMedical: 3, Return: item.Wood, ReturnQty: 1},
	// wiki: Bionic arm (part compatibility and efficiency); original tuning: medical threshold and recovered item.
	item.ServoArm: {Parts: arms, Efficiency: 1250, MinMedical: 5, Return: item.ServoArm, ReturnQty: 1},
	// wiki: Bionic leg (part compatibility and efficiency); original tuning: medical threshold and recovered item.
	item.ServoLeg: {Parts: legs, Efficiency: 1250, MinMedical: 5, Return: item.ServoLeg, ReturnQty: 1},
	// wiki: Bionic eye (part compatibility and efficiency); original tuning: medical threshold and recovered item.
	item.OpticImplant: {Parts: eyes, Efficiency: 1250, MinMedical: 5, Return: item.OpticImplant, ReturnQty: 1},
	// wiki: Bionic ear (part compatibility and efficiency); original tuning: medical threshold and recovered item.
	item.CochlearImplant: {Parts: ears, Efficiency: 1250, MinMedical: 5, Return: item.CochlearImplant, ReturnQty: 1},
	// wiki: Bionic spine (part compatibility and efficiency); original tuning: medical threshold and recovered item.
	item.SpinalBrace: {Parts: 1 << Spine, Efficiency: 1000, MinMedical: 5, Return: item.SpinalBrace, ReturnQty: 1},
	// wiki: Bionic heart (part compatibility and efficiency); original tuning: medical threshold and recovered item.
	item.PumpHeart: {Parts: 1 << Heart, Efficiency: 1250, MinMedical: 5, Return: item.PumpHeart, ReturnQty: 1},
	// wiki: Bionic stomach (part compatibility and efficiency); original tuning: medical threshold and recovered item.
	item.SynthStomach: {Parts: 1 << Stomach, Efficiency: 1250, MinMedical: 5, Return: item.SynthStomach, ReturnQty: 1},
	// wiki: Prosthetic heart (part compatibility and efficiency); original tuning: medical threshold and recovered item.
	item.ValveHeart: {Parts: 1 << Heart, Efficiency: 800, MinMedical: 4, Return: item.ValveHeart, ReturnQty: 1},
	// wiki: Archotech arm (part compatibility and efficiency); original tuning: medical threshold and recovered item.
	item.ApexArm: {Parts: arms, Efficiency: 1500, MinMedical: 5, Return: item.ApexArm, ReturnQty: 1},
	// wiki: Archotech leg (part compatibility and efficiency); original tuning: medical threshold and recovered item.
	item.ApexLeg: {Parts: legs, Efficiency: 1500, MinMedical: 5, Return: item.ApexLeg, ReturnQty: 1},
	// wiki: Archotech eye (part compatibility and efficiency); original tuning: medical threshold and recovered item.
	item.ApexEye: {Parts: eyes, Efficiency: 1500, MinMedical: 5, Return: item.ApexEye, ReturnQty: 1},
	// wiki: Power claw (part compatibility and efficiency); original tuning: medical threshold and recovered item.
	item.ServoClaw: {Parts: hands, Efficiency: 1000, MinMedical: 4, Return: item.ServoClaw, ReturnQty: 1},
	// wiki: Bionic jaw (part compatibility and efficiency); original tuning: medical threshold and recovered item.
	item.ServoJaw: {Parts: 1 << Jaw, Efficiency: 1250, MinMedical: 5, Return: item.ServoJaw, ReturnQty: 1},
	// wiki: Bionic tongue (part compatibility and efficiency); original tuning: medical threshold and recovered item.
	item.SynthTongue: {Parts: 1 << Tongue, Efficiency: 1000, MinMedical: 5, Return: item.SynthTongue, ReturnQty: 1},
	// wiki: Cochlear implant (part compatibility and efficiency); original tuning: medical threshold and recovered item.
	item.AcousticImplant: {Parts: ears, Efficiency: 650, MinMedical: 4, Return: item.AcousticImplant, ReturnQty: 1},
	// wiki: Joywire (brain attachment, mood +30, consciousness -20%, irreversible); original tuning: medical threshold and recovered item.
	item.BlissCircuit: {Parts: 1 << Brain, MinMedical: 5},
	// wiki: Painstopper (brain attachment, pain suppression, removable); original tuning: medical threshold and recovered item.
	item.NerveDampener: {Parts: 1 << Brain, MinMedical: 5, Removable: true, Return: item.NerveDampener, ReturnQty: 1},
	// wiki: Death acidifier (torso attachment, irreversible, dissolves worn and wielded gear); original tuning: medical threshold and recovered item.
	item.AcidFailsafe: {Parts: 1 << Torso, MinMedical: 5},
}

func artificial(k item.Kind) artificialDef {
	if int(k) >= len(artificialDefs) {
		return artificialDef{}
	}
	return artificialDefs[k]
}

func FitsReplacement(k item.Kind, p Part) bool {
	return k.IsReplacement() && p < NumParts && artificial(k).Parts&bit(p) != 0
}

func FitsImplant(k item.Kind, p Part) bool {
	return k.IsAddedImplant() && p < NumParts && artificial(k).Parts&bit(p) != 0
}

func ImplantEfficiency(k item.Kind) int32 { return artificial(k).Efficiency }

// ListedArtificial excludes retired, non-Core parts from new surgery
// choices while existing saved parts remain removable and usable.
func ListedArtificial(k item.Kind) bool { return !artificial(k).Legacy }

// HasDamagedReplacement reports structural injuries needing hourly recovery.
func HasDamagedReplacement(b *Body) bool {
	for _, r := range b.Replacements {
		if r.Damage > 0 {
			return true
		}
	}
	return false
}

// RemovalYield reports the item or raw material returned by a successful
// extraction. Simple wooden parts collapse to a single piece of wood.
func RemovalYield(k item.Kind) (item.Kind, int) {
	d := artificial(k)
	return d.Return, int(d.ReturnQty)
}

// MedicalSkill is the minimum doctor skill required to install a part.
func MedicalSkill(k item.Kind) int { return int(artificial(k).MinMedical) }

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
	k := b.ReplacementAt(Heart)
	return b.hasReplacement(Heart) && (k == item.PumpHeart || k == item.ValveHeart)
}

// GutWormImmune reports a synth stomach, which gut worms cannot live in
// (MED-04 reads it).
func (b *Body) GutWormImmune() bool {
	return b.hasReplacement(Stomach) && b.ReplacementAt(Stomach) == item.SynthStomach
}

// SpineBraced reports a spinal brace, which ends a bad back (MED-08
// reads it).
func (b *Body) SpineBraced() bool {
	return b.hasReplacement(Spine) && b.ReplacementAt(Spine) == item.SpinalBrace
}

// InstallReplacement fits artificial part k to a missing natural part.
func (b *Body) InstallReplacement(p Part, k item.Kind) bool {
	if b.Anatomy != Humanlike || p >= NumParts || !FitsReplacement(k, p) || !b.isMissing(p) || b.hasReplacement(p) || b.coveredByReplacement(p) {
		return false
	}
	b.SetReplacement(p, k)
	return true
}

// RemoveReplacement detaches the artificial part at p.
func (b *Body) RemoveReplacement(p Part) (item.Kind, bool) {
	if p >= NumParts || !b.hasReplacement(p) {
		return 0, false
	}
	k := b.ReplacementAt(p)
	b.SetReplacement(p, 0)
	return k, true
}

// blockedAbove reports a strict ancestor of p missing or carrying an implant,
// so nothing can be fitted on p.
func (b *Body) blockedAbove(p Part) bool {
	t := b.table()
	for q := t[p].Parent; q != p; q = t[q].Parent {
		if b.isMissing(q) || b.hasReplacement(q) {
			return true
		}
		p = q
	}
	return false
}

// CanReplace reports whether p can receive a replacement part, including
// a replacement upgrade. Vital replacement surgery targets the heart only;
// replacing the tongue requires an intact natural jaw.
func (b *Body) CanReplace(p Part) bool {
	return b.Anatomy == Humanlike && p < NumParts && (!defs[p].Vital || p == Heart) &&
		!b.blockedAbove(p) && (p != Tongue || !b.isMissing(Jaw) && !b.hasReplacement(Jaw))
}

// InstallOptions lists the parts implant kind k could be fitted on now,
// present or missing.
func (b *Body) InstallOptions(k item.Kind) []Part {
	var out []Part
	for p := Part(0); p < NumParts; p++ {
		if FitsReplacement(k, p) && b.CanReplace(p) {
			out = append(out, p)
		}
	}
	return out
}

// MissingUncovered counts missing parts no implant stands in for: the ones
// that should still weigh on the pawn's mood.
func (b *Body) MissingUncovered() int {
	n := 0
	for p := Part(0); p < b.numParts(); p++ {
		if b.isMissing(p) && !b.hasReplacement(p) && !b.coveredByReplacement(p) {
			n++
		}
	}
	return n
}

// Implant is an additive device on an intact body part.
type Implant struct {
	Part Part
	Kind item.Kind
}

func HasImplant(b *Body, p Part, k item.Kind) bool {
	for _, imp := range b.Implants {
		if imp.Part == p && imp.Kind == k {
			return true
		}
	}
	return false
}

func CanAddImplant(b *Body, p Part, k item.Kind) bool {
	return b.Anatomy == Humanlike && p < NumParts && FitsImplant(k, p) &&
		!b.isMissing(p) && !b.blockedAbove(p) && !b.hasReplacement(p) && !HasImplant(b, p, k)
}

// AddImplant adds a device without replacing the underlying natural part.
// The slice has canonical part-and-kind order for deterministic saves.
func AddImplant(b *Body, p Part, k item.Kind) bool {
	if !CanAddImplant(b, p, k) {
		return false
	}
	at := len(b.Implants)
	for i, imp := range b.Implants {
		if imp.Part > p || imp.Part == p && imp.Kind > k {
			at = i
			break
		}
	}
	b.Implants = append(b.Implants, Implant{})
	copy(b.Implants[at+1:], b.Implants[at:])
	b.Implants[at] = Implant{p, k}
	return true
}

// RemoveImplant removes a reversible additive implant.
func DetachImplant(b *Body, p Part, k item.Kind) bool {
	if !artificial(k).Removable {
		return false
	}
	for i, imp := range b.Implants {
		if imp.Part == p && imp.Kind == k {
			b.Implants = append(b.Implants[:i:i], b.Implants[i+1:]...)
			if len(b.Implants) == 0 {
				b.Implants = nil
			}
			return true
		}
	}
	return false
}

func dropImplantsUnder(b *Body, p Part) {
	if len(b.Implants) == 0 {
		return
	}
	kept := b.Implants[:0]
	for _, imp := range b.Implants {
		if !b.under(imp.Part, p) {
			kept = append(kept, imp)
		}
	}
	clear(b.Implants[len(kept):])
	b.Implants = kept
	if len(kept) == 0 {
		b.Implants = nil
	}
}
