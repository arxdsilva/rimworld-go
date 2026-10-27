package ebiten

import ebiten "github.com/hajimehoshi/ebiten/v2"

// drawPerfOverlay draws the performance box (PERF-002: fps, 1% low, frame,
// update and draw time, sim rate and memory) when Graphics.PerfOverlay is
// on. It draws after the HUD and also while the UI is hidden, so the
// overlay can measure a bare map. sw, sh are logical.
//
// PERF-002 wave 0 contract: a no-op until the HS8 agent fills it.
func (a *App) drawPerfOverlay(screen *ebiten.Image, sw, sh int) {}
