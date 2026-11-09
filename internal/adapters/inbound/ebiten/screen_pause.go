package ebiten

import (
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/mainmenu"
	"rimworld-go/internal/adapters/inbound/ebiten/options/saves"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// pauseEntries is how many buttons the in-game menu holds (the reference
// shows five; ours has nine and the panel grows by a pitch for each more).
const (
	pauseEntries = 9
	pauseBaseN   = 5
	pauseTitleH  = 40 // the "Paused" title row above the first button
)

// pauseRect is the in-game menu's panel (UIS-069): PauseW wide, bottom-right
// above the tab bar, taller by PauseBtnPitch for each entry past five.
func pauseRect(sw, sh int) gfx.Rect {
	w := float32(hudlayout.PauseW)
	h := float32(hudlayout.PauseH + (pauseEntries-pauseBaseN)*hudlayout.PauseBtnPitch)
	return gfx.Rect{X: max(0, float32(sw)-w), Y: max(0, float32(sh-hudlayout.TabsH)-h), W: w, H: h}
}

func (a *App) pauseButtons(sw, sh int) []uikit.Button {
	p := pauseRect(sw, sh)
	cx := p.X + hudlayout.PauseInset
	y := p.Y + hudlayout.PauseInset + pauseTitleH
	save := uikit.ButtonSpec{ID: "pause.save", Label: a.shell.T("Save"), Disabled: !a.canSave(), Reason: a.shell.T("saving unavailable")}
	load := uikit.ButtonSpec{ID: "pause.load", Label: a.shell.T("Load"), Disabled: !a.canLoad(), Reason: a.shell.T("loading unavailable")}
	if a.commitment() { // options/saves: one timeline, saved by the game itself
		save = uikit.ButtonSpec{ID: "pause.save", Label: a.shell.T("Save"), Disabled: true, Reason: a.shell.T("the game saves itself")}
		load = uikit.ButtonSpec{ID: "pause.load", Label: a.shell.T("Load"), Disabled: true, Reason: saves.CommitLoadReason}
	}
	return uikit.StackButtons(cx, y, hudlayout.PauseBtnW, hudlayout.PauseBtnH, hudlayout.PauseBtnPitch-hudlayout.PauseBtnH, []uikit.ButtonSpec{
		{ID: "pause.resume", Label: a.shell.T("Resume")},
		save,
		load,
		{ID: "pause.options", Label: a.shell.T("Options")},
		{ID: "pause.help", Label: a.shell.T("Help"), Disabled: true, Reason: a.shell.T("not implemented")},
		{ID: "pause.savequit", Label: a.shell.T("Save & Quit to Menu"), Disabled: !a.canSave(), Reason: a.shell.T("saving unavailable")},
		{ID: "pause.savequitos", Label: a.shell.T("Save & Quit to Desktop"), Disabled: !a.canSave(), Reason: a.shell.T("saving unavailable")},
		{ID: "pause.menu", Label: a.shell.T("Return to Main Menu")},
		{ID: "pause.quit", Label: a.shell.T("Quit")},
	})
}

// drawPause draws the in-game menu over the game, which keeps its look: no
// scrim (UIS-069).
func (a *App) drawPause(screen *ebiten.Image, sw, sh int) {
	p := pauseRect(sw, sh)
	uikit.DrawPanel(screen, p.X, p.Y, p.W, p.H)
	gfx.Text(screen, a.shell.T("Paused"), float64(p.X+hudlayout.PauseInset), float64(p.Y+hudlayout.PauseInset), gfx.WithColor(gfx.Title, uikit.Heading))
	newer, _ := a.cfg.Updates.UpdateAvailable()
	ver := gfx.WithColor(gfx.Body, uikit.TextDim)
	ver.Align = gfx.AlignRight
	gfx.Text(screen, mainmenu.VersionLineIn(&a.shell.shellTexts, a.cfg.Version, newer), float64(p.Right()-hudlayout.PauseInset), float64(p.Y+hudlayout.PauseInset)+4, ver)
	mx, my := a.currentCursor()
	a.drawButtons(screen, a.pauseButtons(sw, sh), mx, my)
	a.opt.Saves.DrawPauseStatus(screen, p.X+hudlayout.PauseInset, p.Bottom()-hudlayout.PauseInset-float32(gfx.LineH(gfx.Small))) // SAV-013
}
