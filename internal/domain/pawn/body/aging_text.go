package body

import (
	"fmt"
	"strings"
)

// Health tab text for the aging conditions (MED-08; moved here from the
// game package in TEMP-6): the onset message's words, each condition's
// row label and hover card, and the headline.

// AgingOnsetText names a new chronic condition for the message ("a bad
// back", "a cataract in the left eye").
func AgingOnsetText(k CondKind, p Part) string {
	switch k {
	case CondBadBack:
		return "a bad back"
	case CondCataract:
		return "a cataract in the " + p.Label()
	case CondFrail:
		return "frailty"
	case CondHearingLoss:
		return "hearing loss in the " + p.Label()
	case CondArteryBlockage:
		return "an artery blockage"
	}
	return DefOfCond(k).Label
}

// AgingLabel is an aging condition's row name: "Bad back", "Cataract
// (left eye)", "Artery blockage (major)", "Carcinoma (stomach), growing".
func AgingLabel(c Condition, d CondDef, salt uint64) string {
	l := strings.ToUpper(d.Label[:1]) + d.Label[1:]
	switch c.Kind {
	case CondCataract, CondHearingLoss:
		return l + " (" + c.Part.Label() + ")"
	case CondCarcinoma:
		return l + " (" + c.Part.Label() + "), " + CarcinomaGrowth(c, salt).String()
	case CondArteryBlockage, CondHeartAttack:
		if int(c.Stage) < len(d.Stages) {
			return l + " (" + d.Stages[c.Stage].Label + ")"
		}
	}
	return l
}

// AgingTip is an aging condition's hover card: what it does and how it
// ends.
func AgingTip(c Condition, label string, salt uint64) string {
	switch c.Kind {
	case CondBadBack:
		return label + "\nMoving −30 %, manipulation −10 %. It comes with age and stays; a spinal brace in place of the spine ends it."
	case CondCataract:
		return label + "\nThe " + c.Part.Label() + " sees half as well (sight −25 %). It stays; replacing the eye (an optic implant) ends it."
	case CondFrail:
		return label + "\nMoving and manipulation −30 %. It comes with age and stays."
	case CondDementia:
		return label + "\nConsciousness −15 %, talking and hearing −25 %; now and then wanders off confused. It stays."
	case CondHearingLoss:
		return label + "\nThe " + c.Part.Label() + " hears half as well (hearing −25 %). It stays; a cochlear implant in its place ends it."
	case CondArteryBlockage:
		mtb := heartAttackMTBAt(c.Stage)
		return fmt.Sprintf("%s\nSeverity %d%%; it slowly grows and stops the heart at 100%%. A heart attack about once in %d days at this stage. A pump or valve heart, or a new heart, ends it.",
			label, int(c.Sev)/10, mtb)
	case CondCarcinoma:
		d := DefOfCond(c.Kind)
		pain := int32(0)
		if int(c.Stage) < len(d.Stages) {
			pain = d.Stages[c.Stage].PainAdd
		}
		t := fmt.Sprintf("%s\nSeverity %d%%, pain +%d%%. ", label, int(c.Sev)/10, pain/10)
		switch CarcinomaGrowth(c, salt) {
		case Growing:
			t += "Growing: at 100 % it destroys the " + c.Part.Label() + "."
		case Stable:
			t += "Stable for now."
		default:
			t += "In remission: shrinking, it will fade away."
		}
		return t + "\nExcise it: a doctor with Medicine 10 and 4 industrial medicine or better; a failed excision kills one time in four."
	case CondHeartAttack:
		return fmt.Sprintf("%s\nSeverity %d%%: at 60 %% the patient blacks out, at 100 %% the heart stops. Each tend (one an hour) has a 65 %% × quality chance to ease it by 30 %%; below 0 it is over.",
			label, int(c.Sev)/10)
	}
	return label
}

// heartAttackMTBAt is the mean days between heart attacks at artery
// blockage stage s (for the tooltip).
func heartAttackMTBAt(s uint8) int32 {
	b := Body{Conds: []Condition{{Kind: CondArteryBlockage, Part: Heart, Sev: 1, Stage: s}}}
	return b.HeartAttackMTB()
}

// AgingHeadline is the Health tab's headline when the body's conditions
// are all aging ones or a heart attack is under way: the heart attack
// first, then a carcinoma and an artery blockage; the permanent kinds
// keep the plain state ("Healthy", "Injured").
func AgingHeadline(b *Body, state string, now uint64) string {
	if c, ok := b.Cond(CondHeartAttack); ok {
		if now >= c.TendUntil {
			return "Heart attack: needs tending"
		}
		return "Heart attack: tended"
	}
	if state != "Healthy" {
		return state
	}
	for _, k := range []CondKind{CondCarcinoma, CondArteryBlockage} {
		if _, ok := b.Cond(k); ok {
			d := DefOfCond(k)
			return strings.ToUpper(d.Label[:1]) + d.Label[1:]
		}
	}
	return state
}

// AgingHeads reports b's headline as AgingHeadline's: a heart attack, or
// nothing but aging conditions.
func AgingHeads(b *Body) bool {
	if _, ok := b.Cond(CondHeartAttack); ok {
		return true
	}
	for _, c := range b.Conds {
		if !IsAging(c.Kind) {
			return false
		}
	}
	return true
}
