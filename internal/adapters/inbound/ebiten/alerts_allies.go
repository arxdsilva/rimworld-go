package ebiten

import (
	"fmt"

	"rimworld-go/internal/application/game"
)

// Allies alert (PLAY-024 Phase 4): fighters sent as military aid are on
// the map. Informational (warning tier, like the trade alerts); clicking
// it cycles through the standing fighters.

// allySource is the aid groups on the map.
type allySource interface {
	Allies() []game.AllyGroupView
}

var _ allySource = (*game.Game)(nil)

func init() {
	alertExplanations["allies"] = "Fighters sent by a friendly faction are helping defend the colony. They fight raiders and hostile animals on their own and go home once all is quiet, within a day. Turrets and colonists never shoot them."
}

// allyAlerts raises "Allies on the map (N)" for the standing fighters of
// every group, or nothing when none stands.
func allyAlerts(groups []game.AllyGroupView) []alert {
	n := 0
	var targets [][2]int
	for _, gr := range groups {
		n += gr.Standing
		targets = append(targets, gr.Spots...)
	}
	if n == 0 {
		return nil
	}
	label := fmt.Sprintf("Allies on the map (%d)", n)
	if len(groups) == 1 && groups[0].Leaving {
		label = fmt.Sprintf("Allies leaving (%d)", n)
	}
	return []alert{{key: "allies", label: label, targets: targets}}
}

// allyAlertsFrom reads the aid groups when src has them.
func allyAlertsFrom(src any) []alert {
	if s, ok := src.(allySource); ok {
		return allyAlerts(s.Allies())
	}
	return nil
}
