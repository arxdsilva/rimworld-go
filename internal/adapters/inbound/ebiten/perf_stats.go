package ebiten

import "time"

// perfClock is the always-on frame timing behind the performance overlay
// (PERF-002, perf_overlay.go): a rolling window of frame gaps, Update and
// Draw times, summarised a couple of times a second. Wall-clock time is
// presentation-only and never reaches the simulation.
//
// PERF-002 wave 0 contract: the hooks below are wired into App.Update and
// App.Draw; the HS8 agent fills the ring, the stats and the overlay.
type perfClock struct{}

// perfBeginUpdate marks the start of an Update.
func (a *App) perfBeginUpdate() {}

// perfEndUpdate records the Update's time.
func (a *App) perfEndUpdate() {}

// perfEndDraw records a Draw that began at start and the gap since the
// last one. It runs before the background throttle sleeps.
func (a *App) perfEndDraw(start time.Time) {}
