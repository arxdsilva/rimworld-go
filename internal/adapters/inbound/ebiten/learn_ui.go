package ebiten

import ebiten "github.com/hajimehoshi/ebiten/v2"

// The learning helper (PLAY-018: LEARN owns it): a collapsible top-right
// panel of short original lessons, in the layout's Learn rect.

// learnButtons are the helper's buttons.
func (a *App) learnButtons() []button { return nil }

// drawLearn draws the helper.
func (a *App) drawLearn(screen *ebiten.Image) {}

// learnHeight is the helper's height the layout reserves (0: hidden).
func (a *App) learnHeight() float32 { return 0 }

// learnChrome is the helper's legacy area (none).
func (a *App) learnChrome() []rect { return nil }

// applyLearnClick handles a "learn." click.
func (a *App) applyLearnClick(id string) {}

// learnUI declares the helper's widget-kit parts (stub).
func (a *App) learnUI(sw, sh int) {}

// learnState is this region's presentation state (App.hst).
type learnState struct{}
