package body

// The Health tab's words for a body (moved from the game package's body.go
// unchanged, AX-1 budget payer; aging_text.go is the precedent): an
// operation's label, a part's open wounds, the conditions' headline and a
// condition row's name.

import (
	"fmt"
	"strings"
)

// OperationLabel is how the Health tab lists op on body b: fitting an
// implant where the part is missing installs it, where it is still there
// replaces it (SIM-049).
func OperationLabel(b *Body, op Operation) string {
	part := op.Part.Label()
	switch op.Kind {
	case InstallImplant:
		if op.Part < NumParts && b.Missing&(1<<op.Part) == 0 {
			return fmt.Sprintf("Replace %s with %s", part, op.Item)
		}
		return fmt.Sprintf("Install %s (%s)", op.Item, part)
	case RemoveImplant:
		return fmt.Sprintf("Remove implant (%s)", part)
	case Amputate:
		return fmt.Sprintf("Amputate %s", part)
	case HarvestOrgan: // MED-00 placeholders until MED-06
		return fmt.Sprintf("Harvest %s", part)
	case InstallOrgan:
		return fmt.Sprintf("Install organ (%s)", part)
	case Euthanize:
		return "Euthanize"
	case Anesthetize:
		return "Anesthetize"
	case ExciseCarcinoma:
		return fmt.Sprintf("Excise carcinoma (%s)", part)
	}
	return "Operation (" + part + ")"
}

// woundKindLabels name WoundKind values for the Health tab.
var woundKindLabels = [...]string{Cut: "cut", Bruise: "bruise", Bite: "bite", Scratch: "scratch",
	Burn: "burn", Surgical: "surgical cut", Stab: "stab", Gunshot: "gunshot", Frostbite: "frostbite"}

// WoundKindLabel names a wound kind ("gunshot").
func WoundKindLabel(k WoundKind) string {
	if int(k) < len(woundKindLabels) {
		return woundKindLabels[k]
	}
	return "wound"
}

// PartTendTip lists p's open wounds with their tend quality, one a line
// ("" when it has none).
func PartTendTip(b *Body, p Part) string {
	var lines []string
	for _, w := range b.Wounds {
		if w.Part != p || w.Permanent {
			continue
		}
		t := fmt.Sprintf("%s, %d HP: ", WoundKindLabel(w.Kind), w.Dmg)
		if w.Tended {
			t += fmt.Sprintf("tended %d%%", int(w.TendQ)/10)
		} else {
			t += "untended"
		}
		lines = append(lines, t)
	}
	return strings.Join(lines, "\n")
}

// ConditionHeadline is the Health tab's headline for b's conditions when
// nothing bleeds: "Infection: winning/losing" while a wound festers, else
// its first disease's ("Flu: losing"; a parasite: "Gut worms: needs
// tending").
func ConditionHeadline(b *Body, losing bool) string {
	name := ""
	cure, sedated := false, false
	for _, c := range b.Conds {
		if c.Kind == CondAnesthetic { // no race to win or lose (MED-06)
			sedated = true
			continue
		}
		if IsAging(c.Kind) { // the aging headline's (MED-08)
			continue
		}
		if c.Kind == CondInfection {
			name, cure = "Infection", false
			break
		}
		if name == "" {
			d := DefOfCond(c.Kind)
			name, cure = strings.ToUpper(d.Label[:1])+d.Label[1:], d.CureTendSum > 0
		}
	}
	switch {
	case name == "" && sedated:
		return "Under anesthetic"
	case name == "":
		return "Infection: winning"
	case losing:
		return name + ": losing"
	case cure:
		return name + ": needs tending"
	}
	return name + ": winning"
}

// ConditionLabel is a condition row's name: kind, stage and part
// ("Infection (major), left arm").
func ConditionLabel(c Condition, d CondDef) string {
	l := d.Label
	if l != "" {
		l = strings.ToUpper(l[:1]) + l[1:]
	}
	if int(c.Stage) < len(d.Stages) && d.Stages[c.Stage].Label != "" {
		l += " (" + d.Stages[c.Stage].Label + ")"
	}
	if c.Part != Torso || c.Kind == CondInfection {
		l += ", " + c.Part.Label()
	}
	return l
}
