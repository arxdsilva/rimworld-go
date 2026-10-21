package ebiten

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// Plant panel (UI-011, MAIN_UI §10): what the plant is, how far it has
// grown, and Harvest / Cut (or Chop) with "now" orders per colonist.

// plantRect is the plant panel's frame (the inspector's top-left slot).
func plantRect() button { return button{x: hudPad, y: hudTopH + hudPad, w: 300, h: 190} }

// plantButtons are the plant panel's buttons (ids "plant.*").
func (a *App) plantButtons() []button { return nil } // UI-011 W1

// drawPlantInspector draws the plant panel when a plant is selected.
func (a *App) drawPlantInspector(screen *ebiten.Image) {} // UI-011 W1

// applyPlantClick handles a "plant.*" button.
func (a *App) applyPlantClick(id string) {} // UI-011 W1
