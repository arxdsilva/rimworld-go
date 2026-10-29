package care

import (
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// Option is one entry of a pawn's right-click menu, field for field the
// game's OrderOption (which converts it with OrderOption(o)). When
// Disabled, Reason says why; Cmd is still the command it would issue.
type Option struct {
	Label    string
	Cmd      command.Command
	Disabled bool
	Reason   string
}

// PawnOrders are the care entries of pawn's menu at (x, y): when drafted,
// "Tend <name>" for each wounded colonist or prisoner there that no one
// carries, disabled with the reason it can't happen (CanEmergencyTend).
// None when not drafted.
func (c *Care) PawnOrders(pawn entity.EntityID, x, y int) []Option {
	if !c.h.Drafted(pawn) {
		return nil
	}
	var out []Option
	add := func(pid entity.EntityID) {
		if px, py, ok := c.h.Pos(pid); !ok || px != x || py != y || c.Carry.Carrier(pid) != 0 || !c.h.NeedsTending(pid) {
			return
		}
		o := Option{Label: "Tend " + c.h.Name(pid),
			Cmd: command.Command{Kind: command.EmergencyTend, Pawn: uint64(pawn), Target: uint64(pid)}}
		if why := c.Doctor.CanEmergencyTend(pawn, pid); why != "" {
			o.Disabled, o.Reason = true, why
		}
		out = append(out, o)
	}
	for _, pid := range c.h.PawnIDs() {
		add(pid)
	}
	for _, pid := range c.h.RaiderIDs() {
		if c.h.IsPrisoner(pid) {
			add(pid)
		}
	}
	return out
}
