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

func pickerOptions(h game.HealthView) []inspector.HealthOption {
	return inspector.PickerOptions(h)
}

func healthOptions(h game.HealthView) []inspector.HealthOption {
	return inspector.HealthOptions(h)
}

func healthOptionTip(o inspector.HealthOption) string {
	return inspector.HealthOptionTip(inspector.HealthOption{Label: o.Label, Op: o.Op, Part: o.Part, Item: o.Item, Reason: o.Reason})
}

func itemVerbs(it game.ItemView, stored bool) []inspector.ItemVerb {
	in := inspector.ItemVerbs(it, stored)
	out := make([]inspector.ItemVerb, len(in))
	for i, v := range in {
		out[i] = v
	}
	return out
}
