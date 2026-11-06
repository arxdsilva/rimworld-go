package hud

import (
	"rimworld-go/internal/adapters/inbound/ebiten/lang"
	"rimworld-go/internal/application/game"
)

// NotRecoveringAlertKey is the "Not recovering" alert (AIL-004): a downed
// colonist whose hypothermia or heatstroke can't heal in the air where it
// lies (PawnView.ExposureNote set).
const NotRecoveringAlertKey = "notrecovering"

// NotRecoveringAlertIn names the downed colonists whose exposure is not
// healing where they lie; colonists is the colonist list (no raiders or
// prisoners). The sim keeps the note set while the air holds, so the alert
// stays up until they are moved or the air changes.
func NotRecoveringAlertIn(tr lang.Tr, colonists []game.PawnView) (Alert, bool) {
	var who []game.PawnView
	for _, p := range colonists {
		if p.Status != "dead" && p.ExposureNote != "" {
			who = append(who, p)
		}
	}
	if len(who) == 0 {
		return Alert{}, false
	}
	a := Alert{Key: NotRecoveringAlertKey, Danger: true, Label: lang.Tf(tr, "%s is not recovering", who[0].Name)}
	if len(who) > 1 {
		a.Label = lang.Tf(tr, "%d colonists are not recovering", len(who))
	}
	for _, p := range who {
		a.Targets = append(a.Targets, [2]int{p.X, p.Y})
	}
	return a, true
}
