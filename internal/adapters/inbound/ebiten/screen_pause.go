package ebiten

import ebiten "github.com/hajimehoshi/ebiten/v2"

func (a *App) pauseButtons(sw, sh int) []button {
	cx := float32(sw)/2 - 140
	y := float32(sh)/2 - 140
	return stackButtons(cx, y, 280, 36, 8, []buttonSpec{
		{id: "pause.resume", label: "Resume"},
		{id: "pause.save", label: "Save", disabled: true, reason: "not implemented"},
		{id: "pause.load", label: "Load", disabled: true, reason: "not implemented"},
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
	drawLabel(screen, int(px)+24, int(py)+20, "Paused")
	mx, my := a.currentCursor()
	a.drawButtons(screen, a.pauseButtons(sw, sh), mx, my)
}
