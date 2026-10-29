package ebiten

import (
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/options"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

func (a *App) pauseButtons(sw, sh int) []uikit.Button {
	cx := float32(sw)/2 - 140
	y := float32(sh)/2 - 140
	save := uikit.ButtonSpec{ID: "pause.save", Label: "Save", Disabled: !a.canSave(), Reason: "saving unavailable"}
	load := uikit.ButtonSpec{ID: "pause.load", Label: "Load", Disabled: !a.canLoad(), Reason: "loading unavailable"}
	if a.commitment() { // saves.go: one timeline, saved by the game itself
		save = uikit.ButtonSpec{ID: "pause.savequit", Label: "Save & Quit to Menu", Disabled: !a.canSave(), Reason: "saving unavailable"}
		load = uikit.ButtonSpec{ID: "pause.load", Label: "Load", Disabled: true, Reason: options.CommitLoadReason}
	}
	return uikit.StackButtons(cx, y, 280, 36, 8, []uikit.ButtonSpec{
		{ID: "pause.resume", Label: "Resume"},
		save,
		load,
		{ID: "pause.options", Label: "Options"},
		{ID: "pause.help", Label: "Help", Disabled: true, Reason: "not implemented"},
		{ID: "pause.menu", Label: "Return to Main Menu"},
		{ID: "pause.quit", Label: "Quit"},
	})
}

func (a *App) drawPause(screen *ebiten.Image, sw, sh int) {
	drawDim(screen, sw, sh)
	pw, ph := float32(360), float32(360)
	px := float32(sw)/2 - pw/2
	py := float32(sh)/2 - ph/2
	uikit.DrawPanel(screen, px, py, pw, ph)
	gfx.Text(screen, "Paused", float64(px)+24, float64(py)+16, gfx.WithColor(gfx.Large, uikit.Heading))
	mx, my := a.currentCursor()
	a.drawButtons(screen, a.pauseButtons(sw, sh), mx, my)
}
