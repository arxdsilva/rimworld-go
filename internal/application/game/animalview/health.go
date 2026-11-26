// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package animalview

import (
	"fmt"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/infection"
)

// partsOf lists body b's hurt and lost parts in part order; a lost
// part's children go with it.
func partsOf(b *body.Body) []Part {
	var out []Part
	for _, ps := range b.Parts() {
		if ps.Missing {
			if par := b.PartDef(ps.Part).Parent; par != ps.Part && b.PartMissing(par) {
				continue
			}
		} else if ps.HP >= ps.MaxHP {
			continue
		}
		out = append(out, Part{Label: ps.Label, HP: ps.HP, MaxHP: ps.MaxHP, Missing: ps.Missing,
			Bleeding: b.PartBleedPerDay(ps.Part) > 0})
	}
	return out
}

// FillHealth fills an animal view's health block (MED-09): the
// headline, bleeding, blood loss, the worst infection, whether a tend is
// due, and the care level.
func FillHealth(av *View, a animal.State, now uint64, careLevel uint8, tended bool) {
	av.CareLevel = careLevel
	av.BleedDeathHours = -1
	av.HealthState = "Healthy"
	av.MovingPct = 100
	if a.Injury > 0 {
		av.HealthState = "Injured"
	}
	b := a.Body
	if b == nil {
		av.NeedsTend = a.Tame && a.Injury > 0 && !tended
		return
	}
	av.Parts = partsOf(b)
	av.MovingPct = int(b.Capacity(body.Moving)) / 10
	av.NeedsTend = a.Tame && b.TendDue(now)
	av.BleedPerDay = int(animal.BleedPerDay(b, a.Species))
	av.BleedDeathHours = animal.BleedDeathHours(b, a.Species)
	av.BloodLossPct = int(b.BloodLoss) / 10
	av.BloodStage = body.BloodStageLabel(b.BloodStage)
	for _, c := range b.Conds {
		if c.Kind == body.CondInfection && (av.InfectionPct == 0 || int(c.Sev)/10 > av.InfectionPct) {
			av.InfectionPct = max(int(c.Sev)/10, 1)
			av.ImmunityPct = int(c.Immunity) / 10
			av.InfectionLosing = infection.Losing(c, body.DefOfCond(c.Kind), now < c.TendUntil, 1000)
		}

	}
	av.Asthma = AsthmaOf(b)
	if len(av.Asthma) > 0 {
		av.BreathingPct = int(b.Capacity(body.Breathing)) / 10
	}
	if av.BleedPerDay > 0 {
		av.HealthState = fmt.Sprintf("Bleeding %d%%/day · death in %dh", av.BleedPerDay/10, av.BleedDeathHours)
	}
}
