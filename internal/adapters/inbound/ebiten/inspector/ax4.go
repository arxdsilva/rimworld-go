package inspector

import (
	"fmt"

	"rimworld-go/internal/application/game"
)

// AX-4 power and industry lines, read from BuildingView.AX and
// FrameView.AX (game/axview): a deep drill's cycle and the ore it brings
// up, a scanner's scan, a moisture pump's dried radius, a watermill's
// river flow, and a blueprint's secondary materials.

// ax4Lines are a deep drill's, scanner's, pump's or watermill's lines.
func ax4Lines(b game.BuildingView) []string {
	switch b.Kind {
	case "deep drill":
		ore := "Brings up: stone chunks (no deep ore found in reach)"
		if b.AX.Ore != "" {
			ore = "Brings up: " + b.AX.Ore
		}
		return []string{fmt.Sprintf("Drilling  %d%%", b.AX.Cycle/10), ore}
	case "ground-penetrating scanner":
		return []string{fmt.Sprintf("Scanning  %d%%", b.AX.Cycle/10), "A researcher works it to find deep ore"}
	case "moisture pump":
		return []string{fmt.Sprintf("Dried radius  %d / 7 tiles", b.AX.Radius)}
	case "watermill generator":
		if b.AX.Flow <= 0 {
			return []string{"River flow  none: the wheel is out of the water"}
		}
		return []string{fmt.Sprintf("River flow  %d%%", b.AX.Flow/10)}
	}
	return nil
}

// frameExtraLines are a blueprint's secondary materials ("Steel  3 / 9").
func frameExtraLines(f game.FrameView) []string {
	var out []string
	for i, k := range f.AX.Extra {
		if k != "" {
			out = append(out, fmt.Sprintf("%s  %d / %d", Capitalize(k), f.AX.ExtraGot[i], f.AX.Need[i]))
		}
	}
	return out
}
