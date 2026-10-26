package ebiten

import ebiten "github.com/hajimehoshi/ebiten/v2"

// PLAY-020 zone selection, pane, commands and the Expand and Shrink tools
// (contract stubs: ZONEUI implements them; app.go, hud.go, hud_inspect.go
// and hud_commands.go call these).

// handleZoneTool runs a selected zone's Expand or Shrink tool; true while
// one is active (the map mouse is the tool's).
func (a *App) handleZoneTool() bool { return false }

// escapeZoneTool ends an active zone tool; true when it took the Esc.
func (a *App) escapeZoneTool() bool { return false }

// clearZone drops the zone selection.
func (a *App) clearZone() { a.hasZone, a.zoneSel = false, 0 }

// applyZoneClick handles a "zone." click id.
func (a *App) applyZoneClick(id string) {}

// drawZoneSelection marks the selected zone on the map.
func (a *App) drawZoneSelection(screen *ebiten.Image) {}

// zonePane is the selected zone's inspect pane.
func (a *App) zonePane(id uint16) paneSpec { return paneSpec{} }

// zoneCommands are the selected zone's command buttons.
func (a *App) zoneCommands(id uint16) []cmdButton { return nil }
