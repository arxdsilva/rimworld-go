package hud

import (
	"rimworld-go/internal/adapters/inbound/ebiten/lang"
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

// AllyAlertsIn raises "Allies on the map (N)" for the standing fighters of
// every group, or nothing when none stands.
func AllyAlertsIn(tr lang.Tr, groups []game.AllyGroupView) []Alert {
	n := 0
	var targets [][2]int
	for _, gr := range groups {
		n += gr.Standing
		targets = append(targets, gr.Spots...)
	}
	if n == 0 {
		return nil
	}
	label := lang.Tf(tr, "Allies on the map (%d)", n)
	if len(groups) == 1 && groups[0].Leaving {
		label = lang.Tf(tr, "Allies leaving (%d)", n)
	}
	return []Alert{{Key: "allies", Label: label, Targets: targets}}
}

// AllyAlertsFromIn reads the aid groups when src has them.
func AllyAlertsFromIn(tr lang.Tr, src any) []Alert {
	if s, ok := src.(allySource); ok {
		return AllyAlertsIn(tr, s.Allies())
	}
	return nil
}
