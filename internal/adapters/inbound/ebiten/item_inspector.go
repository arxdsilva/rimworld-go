package ebiten

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// Item panel (UI-011, MAIN_UI §10): what the stack is and its condition,
// and per-colonist Equip / Wear / Haul / Eat orders.

// itemRect is the item panel's frame (the inspector's top-left slot).
func itemRect() button { return button{x: hudPad, y: hudTopH + hudPad, w: 300, h: 200} }

// itemButtons are the item panel's buttons (ids "item.*").
func (a *App) itemButtons() []button { return nil } // UI-011 W1

// drawItemInspector draws the item panel when an item is selected.
func (a *App) drawItemInspector(screen *ebiten.Image) {} // UI-011 W1

// applyItemClick handles an "item.*" button.
func (a *App) applyItemClick(id string) {} // UI-011 W1
