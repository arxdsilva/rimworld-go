package ebiten

import (
	"rimworld-go/internal/domain/sim/command"
)

// The Home auto-expand toggle (PLAY-015): a house-with-arrows button
// among the global controls' overlay toggles. Selected while Home grows
// by itself; a click emits SetHomeAuto with the opposite state.

const homeAutoButtonID = "hud.homeauto" // the toggle (hud_global.go places it)

// toggleHomeAuto asks the sim to flip automatic Home expansion.
func (a *App) toggleHomeAuto() {
	if a.renderer == nil {
		return
	}
	a.renderer.enqueue(command.Command{Kind: command.SetHomeAuto, Enabled: !a.renderer.world().HomeAuto})
}
