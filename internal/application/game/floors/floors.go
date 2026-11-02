// Package floors is the floors' read model for the interface (AX-3): what
// one cell of a built floor gives its room, by kind and material name, for
// the Architect's floor cards and the floor's pane. The numbers are the
// room tally's (room.FloorStats), so the interface never drifts from the
// simulation.
package floors

import (
	"fmt"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/world/room"
)

// Stats is the beauty (tenths) and cleanliness (hundredths) a cell of a
// kind floor of stuff gives its room; ok is false for anything but a floor.
func Stats(kind, stuff string) (beautyTenths, cleanHundredths int, ok bool) {
	k, found := construction.Parse(kind)
	if !found || construction.DefOf(k).Layer != construction.LayerFloor {
		return 0, 0, false
	}
	s, _ := item.ParseKind(stuff)
	b, c := room.FloorStats(k, s)
	return b, c, true
}

// Line is a floor's stats line, "Beauty +1   Cleanliness +0.6" ("" for
// anything but a floor).
func Line(kind, stuff string) string {
	b, c, ok := Stats(kind, stuff)
	if !ok {
		return ""
	}
	return fmt.Sprintf("Beauty %s   Cleanliness %s", signed(b, 10), signed(c, 100))
}

// signed is v/unit with a sign and no trailing zeros: "+1", "-0.1", "0".
func signed(v, unit int) string {
	s := fmt.Sprintf("%+g", float64(v)/float64(unit))
	if v == 0 {
		s = "0"
	}
	return s
}
