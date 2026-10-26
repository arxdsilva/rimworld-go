package ebiten

// PLAY-021 power overlay (POWERUI owns this file): each net's conduits in
// its own colour (hidden ones included), the link lines, producers and
// consumers, unpowered and disconnected devices and battery charge. It is
// a global toggle (powerButtonID, not in globalToggleList yet) and shows
// by itself while a power tool is placed.

// powerButtonID is the power overlay's global toggle.
const powerButtonID = "hud.power"

// syncPowerForce keeps the power overlay forced on while a power tool is
// the selected tool of an open Architect (beside syncRoofForce).
func (a *App) syncPowerForce() {
	// TODO(POWERUI)
}
