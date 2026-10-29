package ebiten

import (
	"rimworld-go/internal/adapters/inbound/ebiten/inspector"
	"rimworld-go/internal/adapters/inbound/ebiten/wizard"
	"rimworld-go/internal/application/game"
)

// bioSkillTip is a Bio skill row's hover card.
func bioSkillTip(s game.SkillView, week int) string {
	return inspector.BioSkillTip(wizard.SkillTip(s), s, week)
}

func pickerOptions(h game.HealthView) []healthOption {
	return toHealthOptions(inspector.PickerOptions(h))
}

func healthOptions(h game.HealthView) []healthOption {
	return toHealthOptions(inspector.HealthOptions(h))
}

func healthOptionTip(o healthOption) string {
	return inspector.HealthOptionTip(inspector.HealthOption{Label: o.label, Op: o.op, Part: o.part, Item: o.item, Reason: o.reason})
}

func itemVerbs(it game.ItemView, stored bool) []itemVerb {
	in := inspector.ItemVerbs(it, stored)
	out := make([]itemVerb, len(in))
	for i, v := range in {
		out[i] = itemVerb{v.ID, v.Label, v.Icon, v.Desc, v.Match}
	}
	return out
}
