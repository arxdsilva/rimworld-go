package ebiten

import ebiten "github.com/hajimehoshi/ebiten/v2"

func (a *App) mainMenuButtons(sw, sh int) []button {
	cx := float32(sw)/2 - 150
	y := float32(140)
	return stackButtons(cx, y, 300, 36, 10, []buttonSpec{
		{id: "menu.newgame", label: "New Game"},
		{id: "menu.continue", label: "Continue", disabled: true, reason: "no saves yet"},
		{id: "menu.load", label: "Load Game"},
		{id: "menu.plugins", label: "Plugins", disabled: true, reason: "not implemented"},
		{id: "menu.options", label: "Options"},
		{id: "menu.tutorial", label: "Tutorial / Help", disabled: true, reason: "not implemented"},
		{id: "menu.credits", label: "Credits"},
		{id: "menu.quit", label: "Quit"},
	})
}

func (a *App) drawMainMenu(screen *ebiten.Image, sw, sh int) {
	drawMenuBackdrop(screen, sw, sh)
	drawPanel(screen, 40, 24, float32(sw-80), 72)
	drawLabel(screen, 56, 40, "rimworld-go")
	drawLabel(screen, 56, 60, formatVersion(a.cfg.Version))
	mx, my := a.currentCursor()
	a.drawButtons(screen, a.mainMenuButtons(sw, sh), mx, my)
}

func (a *App) loadButtons(sw, sh int) []button {
	return []button{{
		id:    "load.back",
		label: "Back",
		x:     40,
		y:     float32(sh - 64),
		w:     140,
		h:     36,
	}}
}

func (a *App) drawLoad(screen *ebiten.Image, sw, sh int) {
	a.paintMenu(screen, sw, sh, "Load Game", []string{
		"No saves yet.",
		"",
		"Compatible, migration-required, missing-plugin, and corrupt",
		"states will appear here once persistence exists.",
	}, a.loadButtons(sw, sh))
}

func (a *App) creditsButtons(sw, sh int) []button {
	return []button{{
		id:    "credits.back",
		label: "Back",
		x:     40,
		y:     float32(sh - 64),
		w:     140,
		h:     36,
	}}
}

func (a *App) drawCredits(screen *ebiten.Image, sw, sh int) {
	a.paintMenu(screen, sw, sh, "Credits", []string{
		"rimworld-go is original work. It is not affiliated with Ludeon",
		"Studios or RimWorld.",
		"",
		"Vendor sprites (first playable stand-in): Raphael Goncalves",
		"(Rgsdev), CC0.",
	}, a.creditsButtons(sw, sh))
}
