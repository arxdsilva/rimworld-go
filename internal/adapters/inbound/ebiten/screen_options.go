package ebiten

import ebiten "github.com/hajimehoshi/ebiten/v2"

func (a *App) optionsButtons(sw, sh int) []button {
	return []button{{
		id:    "options.back",
		label: "Back",
		x:     40,
		y:     footerY(sh),
		w:     140,
		h:     36,
	}}
}

func (a *App) drawOptions(screen *ebiten.Image, sw, sh int) {
	a.paintMenu(screen, sw, sh, "Options", []string{
		"Tabs (not wired):",
		"  Graphics   — resolution, VSync, UI scale, effects",
		"  Audio      — master, music, effects, ambient, UI",
		"  Gameplay   — pause on threat, autosave, edge scroll",
		"  Controls   — key bindings",
		"  Accessibility — text scale, color-blind, reduced motion",
		"",
		"Changes on this screen do not affect simulation.",
	}, a.optionsButtons(sw, sh))
}
