package ebiten

// The Architect's tool buttons and description box (PLAY-018: ARCHITECT
// owns them): two category columns at the lower-left, the tools as command
// buttons to their right, the description box above the categories.

// archToolCommands are the open category's tools as command buttons.
func (a *App) archToolCommands() []cmdButton { return nil }

// archDescHeight is the description box's height (0: none, or still
// drawn the legacy way).
func (a *App) archDescHeight() float32 { return 0 }

// archChrome is the Architect's legacy area.
func (a *App) archChrome() []rect {
	if !a.archOpen {
		return nil
	}
	r := a.architectRect()
	return []rect{{r.x, r.y, r.w, r.h}}
}
