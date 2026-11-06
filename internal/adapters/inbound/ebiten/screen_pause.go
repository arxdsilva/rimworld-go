package ebiten

import (
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/mainmenu"
	"rimworld-go/internal/adapters/inbound/ebiten/options"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

func (a *App) pauseButtons(sw, sh int) []uikit.Button {
	cx := float32(sw)/2 - 140
	y := float32(sh)/2 - 140
	save := uikit.ButtonSpec{ID: "pause.save", Label: a.shell.T("Save"), Disabled: !a.canSave(), Reason: a.shell.T("saving unavailable")}
	load := uikit.ButtonSpec{ID: "pause.load", Label: a.shell.T("Load"), Disabled: !a.canLoad(), Reason: a.shell.T("loading unavailable")}
	if a.commitment() { // saves.go: one timeline, saved by the game itself
		save = uikit.ButtonSpec{ID: "pause.savequit", Label: a.shell.T("Save & Quit to Menu"), Disabled: !a.canSave(), Reason: a.shell.T("saving unavailable")}
		load = uikit.ButtonSpec{ID: "pause.load", Label: a.shell.T("Load"), Disabled: true, Reason: options.CommitLoadReason}
	}
	return uikit.StackButtons(cx, y, 280, 36, 8, []uikit.ButtonSpec{
		{ID: "pause.resume", Label: a.shell.T("Resume")},
		save,
		load,
		{ID: "pause.options", Label: a.shell.T("Options")},
		{ID: "pause.help", Label: a.shell.T("Help"), Disabled: true, Reason: a.shell.T("not implemented")},
		{ID: "pause.menu", Label: a.shell.T("Return to Main Menu")},
		{ID: "pause.quit", Label: a.shell.T("Quit")},
	})
}

func (a *App) drawPause(screen *ebiten.Image, sw, sh int) {
	drawDim(screen, sw, sh)
	pw, ph := float32(360), float32(360)
	px := float32(sw)/2 - pw/2
	py := float32(sh)/2 - ph/2
	uikit.DrawPanel(screen, px, py, pw, ph)
	gfx.Text(screen, a.shell.T("Paused"), float64(px)+24, float64(py)+16, gfx.WithColor(gfx.Large, uikit.Heading))
	newer, _ := a.cfg.Updates.UpdateAvailable()
	ver := gfx.WithColor(gfx.Body, uikit.TextDim)
	ver.Align = gfx.AlignRight
	gfx.Text(screen, mainmenu.VersionLine(a.cfg.Version, newer), float64(px+pw)-24, float64(py)+20, ver)
	mx, my := a.currentCursor()
	a.drawButtons(screen, a.pauseButtons(sw, sh), mx, my)
}
