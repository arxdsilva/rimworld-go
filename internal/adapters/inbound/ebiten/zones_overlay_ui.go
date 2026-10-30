package ebiten

import "rimworld-go/internal/domain/sim/command"

// toggleZonesOverlay flips the global zones overlay (default: every zone shown; the toggle hides them all) (STO-3; Shift+Z and
// the Zones button): the HUD flag, the overlay's draw flag and the typed
// SetZoneOverlay command are set together.
func toggleZonesOverlay(a *App) {
	a.chrome.ZonesOff = !a.chrome.ZonesOff
	if r := a.renderer; r != nil {
		r.overlays().Zones.Hide = a.chrome.ZonesOff
	}
	a.shell.Enqueue(command.Command{Kind: command.SetZoneOverlay, Enabled: !a.chrome.ZonesOff})
}
