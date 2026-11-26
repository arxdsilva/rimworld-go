// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package prison

import (
	"rimworld-go/internal/application/game/tabview"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/prisoner"
	"rimworld-go/internal/domain/sim/entity"
)

// View returns a detached tab state for held prisoners.
func (p *Prison) View(id entity.EntityID, prisonerBed bool) (tabview.Prisoner, bool) {
	st := p.st[id]
	if st == nil {
		return tabview.Prisoner{}, false
	}
	v := tabview.Prisoner{
		Mode:            uint8(st.Mode),
		Resistance:      int((st.Resistance + prisoner.ResistanceUnit - 1) / prisoner.ResistanceUnit),
		ForPrisonersBed: prisonerBed,
	}
	v.RecruitChanceMin, v.RecruitChanceMax = prisoner.RecruitChance(0), prisoner.RecruitChance(20)
	if w := p.h.Held(id).Warden; w != 0 {
		if vital, ok := p.h.Vital(w); ok && vital.Status == health.Alive {
			v.Warden = p.h.PersonName(w)
			v.RecruitChanceMin = prisoner.RecruitChance(p.h.SocialLevel(w))
			v.RecruitChanceMax = v.RecruitChanceMin
		}
	}
	return v, true
}
