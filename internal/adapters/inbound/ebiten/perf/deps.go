package perf

import (
	"time"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/domain/sim/clock"
)

// Env is the program as the instruments read it, as of the call.
type Env struct {
	InGame  bool // a colony runs (false on a menu screen)
	Stepped int  // simulation ticks the last Update ran
	Held    int  // Updates the budget held short so far
	MaxOwed int  // the largest tick debt seen
	PW, PH  int  // the physical window
	Tick    uint64
	Date    clock.Date
	Speed   int // the game speed tier
	Scale   gfx.Scale
	Probe   *FPSProbe // the dev frame-rate probe (nil: none)
}

// Host is what the feature needs of the program that no ports role
// carries.
type Host interface {
	// Now is the presentation clock (injectable in tests).
	Now() time.Time
	// Env is the program now.
	Env() Env
	// ResetMaxOwed forgets the largest tick debt (the probe's measured
	// span starts).
	ResetMaxOwed()
	// PerfRect is where the HUD reserves the performance box.
	PerfRect() gfx.Rect
	// StartProbe starts the probe's colony, frames the camera and returns
	// the speed tier and tick it starts at, or why it could not start.
	StartProbe(p *FPSProbe) (speed int, tick0 uint64, err string)
	// Toast shows a transient message.
	Toast(msg string)
	// ArtMem describes the art catalogue's memory.
	ArtMem() string
	// ForceSpeed puts the game back on a speed tier without a cue.
	ForceSpeed(tier int)
}

// Shell composes the roles the instruments call with their Host.
type Shell interface {
	ports.Settings
	Host
}

// Meter is the frame-timing instruments: the always-on clock behind the
// performance overlay, the dev frame-rate probe and the screenshot. The
// root feeds it each Update and Draw.
type Meter struct {
	sh Shell
	// Clock, Probe and Shot are the instruments' state. The root points
	// them at its own copies until the sweep deletes those.
	Clock *Clock
	Probe *Probe
	Shot  *Shot
}

// New builds the feature over its shell.
func New(sh Shell) *Meter {
	return &Meter{sh: sh, Clock: &Clock{}, Probe: &Probe{}, Shot: &Shot{}}
}
