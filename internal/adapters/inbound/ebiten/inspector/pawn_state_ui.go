package inspector

import (
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// The single readers of the SIM-063/066 read-model fields.

// mentalTarget is the pawn a targeting break (insulting spree, murderous
// rage) is aimed at, 0 when the break has no target.
func mentalTarget(p game.PawnView) entity.EntityID { return p.MentalTarget }

// onMedicalBed reports lying in a medical bed.
func onMedicalBed(p game.PawnView) bool { return p.OnMedicalBed }

// guestFaction is the faction a rescued visitor belongs to, "" when it has
// none (a stranger).
func guestFaction(p game.PawnView) string { return p.GuestFaction }

// mentalBreakLine names the break and, for a targeting break, its target.
func mentalBreakLine(p game.PawnView, name func(entity.EntityID) string) string {
	if p.MentalBreak == "" {
		return ""
	}
	ln := "MENTAL BREAK: " + p.MentalBreak
	if t := mentalTarget(p); t != 0 {
		ln += ": " + carryName(name, t)
	}
	return ln
}

// bedStateLine is the bed-rest state (UI 10.4): a medical bed, or plain
// bed rest while resting in bed to heal. "" when neither applies.
func bedStateLine(p game.PawnView) string {
	switch {
	case p.Status == "dead":
		return ""
	case onMedicalBed(p) && p.InBed:
		return "Resting in a medical bed"
	case p.BedRest:
		return "On bed rest"
	}
	return ""
}

// guestLine is the guest identity of a visitor: "Guest of <faction>", or
// "Stranger" when it belongs to none.
func guestLine(p game.PawnView) string {
	if !p.Visitor || p.Prisoner {
		return ""
	}
	if f := guestFaction(p); f != "" {
		return "Guest of " + f
	}
	return "Stranger"
}
