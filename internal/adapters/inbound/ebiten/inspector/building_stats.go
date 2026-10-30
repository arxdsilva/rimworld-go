package inspector

import (
	"fmt"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/command"
)

// Bed stats and the building Forbid toggle (BED-1), read from
// BuildingView (Comfort and RestEffect in permille, Beauty in thousandths).

// BuildingForbidID is the Forbid toggle's id.
const BuildingForbidID = "bldg.forbid"

// bedStatLines are a sleeping place's comfort, rest effectiveness, beauty
// and linked facilities: colonist beds and animal beds alike (the view
// fills RestEffect for every sleeping place).
func bedStatLines(b game.BuildingView) []string {
	if bedSlots(b) == 0 && b.RestEffect == 0 {
		return nil
	}
	out := []string{
		fmt.Sprintf("Comfort: %d%%", b.Comfort/10),
		fmt.Sprintf("Rest effectiveness: %d%%", b.RestEffect/10),
		fmt.Sprintf("Beauty: %+.1f", float64(b.Beauty)/1000),
	}
	if b.Facilities&game.FacilityEndTable != 0 {
		out = append(out, "Linked: End table")
	}
	if b.Facilities&game.FacilityDresser != 0 {
		out = append(out, "Linked: Dresser")
	}
	return out
}

// forbiddable reports a building the simulation leaves alone when it is
// forbidden: the sleeping places, colonists' and pets' (BED-1), and doors (CBT-5:
// a forbidden door stays shut to colonists).
// Other buildings get the toggle when their users honour it.
func forbiddable(b game.BuildingView) bool {
	k, ok := construction.Parse(b.Kind)
	return ok && (k.Sleeps() || k.AnimalSleeps() || k.IsDoor())
}

// forbidCommand is the Forbid toggle of building b: SetForbidden for the
// building, the command an item stack uses.
func forbidCommand(b game.BuildingView) Button {
	c := Button{ID: BuildingForbidID, Label: "Forbid", Icon: "order_forbid", Toggle: true, On: b.Forbidden,
		Desc: "Colonists leave it alone: no using, no work on it.",
		Cmds: []command.Command{{Kind: command.SetForbidden, Target: uint64(b.ID), Enabled: !b.Forbidden}}}
	if k, ok := construction.Parse(b.Kind); ok && k.IsDoor() { // a door: Forbid shuts it to colonists, Allow opens it again
		c.Desc = "Forbid: colonists keep out and path around this door.\nAllow: colonists may use it again."
		if b.Forbidden {
			c.Label = "Allow"
		}
	}
	return c
}
