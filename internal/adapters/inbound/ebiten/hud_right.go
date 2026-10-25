package ebiten

import ebiten "github.com/hajimehoshi/ebiten/v2"

// The right column (PLAY-018: HUD-RIGHT owns it): alerts below the
// learning helper, the letter stack above the global controls, and the
// messages under the colonist bar, in the layout's Alerts, Letters and
// Messages rects. Until migrated: the legacy alert list, banner and
// centred messages.

// drawRightColumn draws the alerts, letters and messages.
func (a *App) drawRightColumn(screen *ebiten.Image, sw, sh int) {
	a.drawAlerts(screen, sw)
	a.drawGatheringBanner(screen, sw) // gathering_ui.go (ENG-003)
	a.drawMessages(screen)
}

// rightColumnButtons are the alerts', letters' and messages' buttons.
func (a *App) rightColumnButtons(sw int) []button {
	return append(a.alertButtons(sw), a.messageButtons(sw)...)
}

// rightColumnChrome is the column's legacy area (none: the legacy alerts
// and messages never blocked the map).
func (a *App) rightColumnChrome(sw int) []rect { return nil }

// alertCount, letterCount and messageCount are the rows the layout
// reserves (0 while drawn the legacy way).
func (a *App) alertCount() int   { return 0 }
func (a *App) letterCount() int  { return 0 }
func (a *App) messageCount() int { return 0 }

// applyLetterClick handles a "letter." click.
func (a *App) applyLetterClick(id string) {}

// rightClickRightColumn answers a right-click over the column (dismiss a
// letter, cycle an alert back).
func (a *App) rightClickRightColumn(mx, my int) bool { return false }

// rightColumnUI declares the column's widget-kit parts (stub).
func (a *App) rightColumnUI(sw, sh int) {}
