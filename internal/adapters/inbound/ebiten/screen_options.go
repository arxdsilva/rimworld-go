package ebiten

import ebiten "github.com/hajimehoshi/ebiten/v2"

func (a *App) optionsButtons(sw, sh int) []button {
	state := "On"
	if a.noThreatPause {
		state = "Off"
	}
	return []button{{
		id:    "options.pausethreat",
		label: "Pause on threat: " + state,
		x:     40,
		y:     120,
		w:     240,
		h:     36,
	}, {
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
		"Gameplay",
		"  Pause on threat: stop the clock when a raid or",
		"  predator pack arrives, so you can draft colonists.",
		"",
		"Not wired yet:",
		"  Graphics   — resolution, VSync, UI scale, effects",
		"  Audio      — master, music, effects, ambient, UI",
		"  Controls   — key bindings",
		"  Accessibility — text scale, color-blind, reduced motion",
	}, a.optionsButtons(sw, sh))
}
