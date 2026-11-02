package inspector

import (
	"fmt"
	"strings"

	"rimworld-go/internal/application/game"
)

// AX-6 recreation lines, read from BuildingView.AX and PawnView.AX
// (game/axview): a recreation piece's joy kind and who uses it, and a
// colonist's recreation status (what it enjoys now, its tolerances).

// ax6Lines are a recreation piece's lines ("Recreation  cerebral",
// "In use  1 colonist").
func ax6Lines(b game.BuildingView) []string {
	if b.AX.Joy == "" {
		return nil
	}
	use := "Free"
	switch {
	case b.AX.Users == 1:
		use = "In use  1 colonist"
	case b.AX.Users > 1:
		use = fmt.Sprintf("In use  %d colonists", b.AX.Users)
	}
	return []string{"Recreation  " + b.AX.Joy, use}
}

// JoyRows are a colonist's recreation status for its Needs tab: what it
// enjoys now, then its tolerance per joy kind ("cerebral 42% (bored)").
func JoyRows(p game.PawnView) []string {
	var out []string
	if p.AX.Joy != "" {
		out = append(out, "Enjoying  "+p.AX.Joy)
	}
	if len(p.AX.Tolerance) == 0 {
		return out
	}
	parts := make([]string, 0, len(p.AX.Tolerance))
	for _, t := range p.AX.Tolerance {
		s := fmt.Sprintf("%s %d%%", t.Kind, t.Pct)
		if t.Bored {
			s += " (bored)"
		}
		parts = append(parts, s)
	}
	return append(out, "Tolerance  "+strings.Join(parts, ", "))
}
