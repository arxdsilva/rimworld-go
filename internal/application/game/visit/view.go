// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package visit

import (
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// View is a detached, read-only representation of a visiting party.
type View struct {
	ID           uint64
	Kind         string
	Phase        string
	Leader       entity.EntityID
	Faction      int
	Members      []entity.EntityID
	ExitX, ExitY int
	CampX, CampY int
	HoursLeft    int
	LeaveReason  string
}

func (k Kind) String() string {
	switch k {
	case KindTraveler:
		return "traveler"
	case KindSmallTrader:
		return "small trader"
	case KindCaravan:
		return "trade caravan"
	case KindAid:
		return "military aid"
	case KindQuest:
		return "quest guest"
	case KindManaged:
		return "managed guest"
	default:
		return "visitor"
	}
}

func (g Group) View(now uint64) View {
	v := View{ID: g.ID, Kind: g.Kind.String(), Leader: g.Leader, Faction: g.Faction,
		ExitX: g.ExitX, ExitY: g.ExitY, CampX: g.CampX, CampY: g.CampY}
	switch g.Phase {
	case PhaseLeaving:
		v.Phase = "leaving"
	case PhaseEntering:
		v.Phase = "entering"
	default:
		v.Phase = "staying"
	}
	if g.LeaveAt > now {
		v.HoursLeft = int((g.LeaveAt - now + clock.TicksPerHour - 1) / clock.TicksPerHour)
	}
	switch g.Reason {
	case LeaveThreat:
		v.LeaveReason = "danger"
	case LeaveLoss:
		v.LeaveReason = "party loss"
	case LeaveCaptured:
		v.LeaveReason = "captured"
	case LeaveHostile:
		v.LeaveReason = "hostile"
	case LeaveDismissed:
		v.LeaveReason = "dismissed"
	case LeaveQuest:
		v.LeaveReason = "quest"
	case LeaveRecall:
		v.LeaveReason = "recalled"
	default:
		v.LeaveReason = "scheduled"
	}
	for _, m := range g.Members {
		if !m.Departed {
			v.Members = append(v.Members, m.ID)
		}
	}
	return v
}
