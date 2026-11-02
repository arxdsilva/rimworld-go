package inspector

import (
	"strings"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/application/game/floors"
)

// floorTitle names an AX-3 floor of kind in stuff and colour: "Granite
// tile", "Slate flagstone", "Crimson carpet floor", "Sterile tile"; ""
// for any other kind (the wood floor keeps materialTitle).
func floorTitle(kind, stuff, colour string) string {
	switch kind {
	case "stone tile":
		return stuffAdjective(stuff) + " tile"
	case "flagstone":
		return stuffAdjective(stuff) + " flagstone"
	case "carpet floor":
		if colour == "" {
			return "Carpet floor"
		}
		return Capitalize(colour) + " carpet floor"
	case "straw matting", "concrete", "paved tile", "steel tile", "silver tile", "gold tile", "sterile tile":
		return Capitalize(kind)
	}
	return ""
}

// ruinedFloorTitle is a ruined AX-3 floor's title ("Ruined granite tile";
// "" for any other kind).
func ruinedFloorTitle(b game.BuildingView) string {
	t := floorTitle(b.Kind, b.Stuff, b.AX.Colour)
	if t == "" {
		return ""
	}
	return "Ruined " + strings.ToLower(t[:1]) + t[1:]
}

// FloorLines are a floor's pane lines: what a cell gives its room and a
// carpet floor's colour (nil for anything but a floor).
func FloorLines(b game.BuildingView) []string {
	l := floors.Line(b.Kind, b.Stuff)
	if l == "" {
		return nil
	}
	if b.AX.Colour != "" {
		return []string{l, "Colour: " + Capitalize(b.AX.Colour)}
	}
	return []string{l}
}
