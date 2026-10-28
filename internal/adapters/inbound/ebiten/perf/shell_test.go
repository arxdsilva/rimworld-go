package perf

import (
	"time"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/platform/config"
	"rimworld-go/internal/platform/input"
)

// fakeShell is a headless shell: a settable clock and Env, a recording
// Host.
type fakeShell struct {
	now    time.Time
	env    Env
	set    config.Settings
	rect   gfx.Rect
	toasts []string
	forced []int
	resets int
}

func (f *fakeShell) Settings() config.Settings   { return f.set }
func (f *fakeShell) KeyName(input.Action) string { return "K" }
func (f *fakeShell) KeyHint(input.Action) string { return "[K]" }
func (f *fakeShell) Now() time.Time              { return f.now }
func (f *fakeShell) Env() Env                    { return f.env }
func (f *fakeShell) ResetMaxOwed()               { f.resets++ }
func (f *fakeShell) PerfRect() gfx.Rect          { return f.rect }
func (f *fakeShell) Toast(s string)              { f.toasts = append(f.toasts, s) }
func (f *fakeShell) ArtMem() string              { return "art" }
func (f *fakeShell) ForceSpeed(t int)            { f.forced = append(f.forced, t) }
func (f *fakeShell) StartProbe(*FPSProbe) (int, uint64, string) {
	return 2, 10, ""
}
