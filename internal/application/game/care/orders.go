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

// pawnVerbs are the pawn-targeted Prioritize orders in menu order
// (SIM-062: rescue, capture, arrest, tend), with their labels. The
// drafted "Tend <name>" (an EmergencyTend) closes the list.
var pawnVerbs = [...]struct {
	o     command.Order
	label string
}{
	{command.OrderRescue, "Rescue "},
	{command.OrderCapture, "Capture "},
	{command.OrderArrest, "Arrest "},
	{command.OrderTendPatient, "Prioritize tending "},
}

// PawnOrders are the care entries of doer's menu at (x, y), for every
// pawn standing or lying there: "Rescue", "Capture", "Arrest",
// "Prioritize tending" (undrafted) for each target that judge offers,
// each kind in roster order (colonists, then raiders, prisoners and
// visitors), then, when drafted, "Tend <name>" for each wounded colonist
// or prisoner there that no one carries (CanEmergencyTend). An entry that
// can't happen now is disabled with the reason.
func (c *Care) PawnOrders(doer entity.EntityID, x, y int) []Option {
	var here []entity.EntityID
	at := func(ids []entity.EntityID) {
		for _, pid := range ids {
			if px, py, ok := c.h.Pos(pid); ok && px == x && py == y {
				here = append(here, pid)
			}
		}
	}
	at(c.h.PawnIDs())
	at(c.h.RaiderIDs())
	if len(here) == 0 {
		return nil
	}
	var out []Option
	for _, v := range pawnVerbs {
		for _, pid := range here {
			offer, why := c.judge(doer, v.o, pid)
			if !offer {
				continue
			}
			out = append(out, Option{Label: v.label + c.personName(pid), Disabled: why != "", Reason: why,
				Cmd: command.Command{Kind: command.Prioritize, Order: v.o, Pawn: uint64(doer), Target: uint64(pid), X: x, Y: y}})
		}
	}
	if c.h.Drafted(doer) {
		for _, pid := range here {
			out = c.tendOption(out, doer, pid)
		}
	}
	return out
}

// tendOption appends drafted doer's "Tend <name>" for pid when it is a
// colonist or prisoner that needs tending and no one carries.
func (c *Care) tendOption(out []Option, doer, pid entity.EntityID) []Option {
	if !c.h.IsColonist(pid) && !c.h.IsPrisoner(pid) || c.Carry.Carrier(pid) != 0 || !c.h.NeedsTending(pid) {
		return out
	}
	o := Option{Label: "Tend " + c.h.Name(pid),
		Cmd: command.Command{Kind: command.EmergencyTend, Pawn: uint64(doer), Target: uint64(pid)}}
	if why := c.Doctor.CanEmergencyTend(doer, pid); why != "" {
		o.Disabled, o.Reason = true, why
	}
	return append(out, o)
}

// personName names anyone: a colonist by the roster, anyone else by the
// prison's register.
func (c *Care) personName(id entity.EntityID) string {
	if c.h.IsColonist(id) {
		return c.h.Name(id)
	}
	return c.pr.PersonName(id)
}
