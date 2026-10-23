package ebiten

import ebiten "github.com/hajimehoshi/ebiten/v2"

func (a *App) pauseButtons(sw, sh int) []button {
	cx := float32(sw)/2 - 140
	y := float32(sh)/2 - 140
	save := buttonSpec{id: "pause.save", label: "Save", disabled: !a.canSave(), reason: "saving unavailable"}
	load := buttonSpec{id: "pause.load", label: "Load", disabled: !a.canLoad(), reason: "loading unavailable"}
	if a.commitment() { // saves.go: one timeline, saved by the game itself
		save = buttonSpec{id: "pause.savequit", label: "Save & Quit to Menu", disabled: !a.canSave(), reason: "saving unavailable"}
		load = buttonSpec{id: "pause.load", label: "Load", disabled: true, reason: commitLoadReason}
	}
	return stackButtons(cx, y, 280, 36, 8, []buttonSpec{
		{id: "pause.resume", label: "Resume"},
		save,
		load,
		{id: "pause.options", label: "Options"},
		{id: "pause.help", label: "Help", disabled: true, reason: "not implemented"},
		{id: "pause.menu", label: "Return to Main Menu"},
		{id: "pause.quit", label: "Quit"},
	})
}

func (a *App) drawPause(screen *ebiten.Image, sw, sh int) {
	drawDim(screen, sw, sh)
	pw, ph := float32(360), float32(360)
	px := float32(sw)/2 - pw/2
	py := float32(sh)/2 - ph/2
	drawPanel(screen, px, py, pw, ph)
	drawText(screen, "Paused", float64(px)+24, float64(py)+16, withColor(tsLarge, uiHeading))
	mx, my := a.currentCursor()
	a.drawButtons(screen, a.pauseButtons(sw, sh), mx, my)
}
