package ebiten

import ebiten "github.com/hajimehoshi/ebiten/v2"

// Main-tab windows (PLAY-018: WINDOWS owns them): Work, Schedule,
// Research, Animals, Wildlife, Assign, History, Factions and World open
// bottom-left above the tab strip, in the layout's Window rect.

// drawWindows draws the open main-tab window (legacy: the Work panel).
func (a *App) drawWindows(screen *ebiten.Image, sw, sh int) {
	if a.workOpen {
		a.drawWork(screen, sw, sh)
	}
}

// windowWant is the open window's wanted size (0, 0: none, or still
// drawn the legacy way).
func (a *App) windowWant() (float32, float32) { return 0, 0 }

// windowChrome is the open window's legacy area.
func (a *App) windowChrome(sw, sh int) []rect {
	if a.workOpen {
		r := workRect(sw, sh)
		return []rect{{r.x, r.y, r.w, r.h}}
	}
	return nil
}

// applyWindowClick handles a "win." click.
func (a *App) applyWindowClick(id string) {}

// toggleWildlifeTab opens or closes the Wildlife window (tab.wildlife).
func (a *App) toggleWildlifeTab() {}

// mainWindowUI declares the open window's widget-kit parts (stub).
func (a *App) mainWindowUI(sw, sh int) {}
