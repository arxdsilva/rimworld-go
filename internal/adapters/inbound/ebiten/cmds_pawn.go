package ebiten

import "rimworld-go/internal/domain/sim/entity"

// A colonist's commands and pane (PLAY-018: PAWN-INSPECT owns them).

// pawnCommands are colonist id's command buttons (Draft, clear orders,
// capture/arrest, equip, ...).
func (a *App) pawnCommands(id entity.EntityID) []cmdButton { return nil }

// pawnPane is colonist id's pane: name, status bars, area chip, lines and
// the Needs/Health/Bio/Social/Gear/Log (Prisoner) tabs.
func (a *App) pawnPane(id entity.EntityID) paneSpec { return paneSpec{} }
