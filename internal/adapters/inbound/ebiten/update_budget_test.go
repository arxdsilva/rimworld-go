package ebiten

import (
	"strings"
	"testing"
	"time"
)

// budgetColony is a running colony at speed s whose ticks each take *d of
// fake wall time.
func budgetColony(t *testing.T, s speed) (*App, *stubSim, *fakeInput, *time.Duration) {
	t.Helper()
	a, sim, f := inputColony(t)
	clk := time.Unix(1000, 0)
	d := new(time.Duration)
	a.now = func() time.Time { return clk }
	sim.onStep = func() { clk = clk.Add(*d) }
	a.renderer.speed = s
	sim.steps = 0
	return a, sim, f, d
}

// stepFrame runs one Update and returns the ticks it stepped.
func stepFrame(t *testing.T, a *App, sim *stubSim, f *fakeInput) int {
	t.Helper()
	before := sim.steps
	f.Frame(t, a)
	return sim.steps - before
}

func TestUpdateBudgetCarriesTicks(t *testing.T) {
	t.Parallel()
	a, sim, f, d := budgetColony(t, speedFast)
	*d = 3 * time.Millisecond
	if got := stepFrame(t, a, sim, f); got != 2 {
		t.Fatalf("3 ms ticks under a 5 ms budget: stepped %d, want 2", got)
	}
	if a.debt.owed != 1 || a.debt.held != 1 || a.renderer.mo.Stepped != 2 {
		t.Fatalf("debt %+v stepped %d, want owed 1 held 1 stepped 2", a.debt, a.renderer.mo.Stepped)
	}
	*d = 0
	if got := stepFrame(t, a, sim, f); got != 4 {
		t.Fatalf("an idle Update pays the debt: stepped %d, want 4", got)
	}
	if sim.steps != 6 || a.debt.owed != 0 {
		t.Fatalf("steps %d owed %d, want 6 and 0", sim.steps, a.debt.owed)
	}
}

func TestUpdateBudgetDebtCapped(t *testing.T) {
	t.Parallel()
	a, sim, f, d := budgetColony(t, speedVeryFast)
	*d = 6 * time.Millisecond
	for i := range 20 {
		if got := stepFrame(t, a, sim, f); got != 1 {
			t.Fatalf("frame %d: a tick over the budget steps once, got %d", i, got)
		}
		if a.debt.owed > speedVeryFast.ticksPerFrame() {
			t.Fatalf("frame %d: owed %d exceeds one Update's nominal", i, a.debt.owed)
		}
	}
	if sim.steps != 20 {
		t.Fatalf("steps %d, want 20", sim.steps)
	}
	*d = 0
	if got := stepFrame(t, a, sim, f); got != 12 || a.debt.owed != 0 {
		t.Fatalf("recovery: stepped %d owed %d, want 12 and 0", got, a.debt.owed)
	}
	if a.debt.maxOwed != 6 {
		t.Fatalf("maxOwed %d, want 6", a.debt.maxOwed)
	}
}

func TestUpdateBudgetNormalNeverSplits(t *testing.T) {
	t.Parallel()
	a, sim, f, d := budgetColony(t, speedNormal)
	*d = 10 * time.Millisecond
	for i := range 5 {
		if got := stepFrame(t, a, sim, f); got != 1 {
			t.Fatalf("frame %d: normal speed stepped %d", i, got)
		}
	}
	if a.debt.held != 0 || a.debt.owed != 0 {
		t.Fatalf("normal speed never holds: %+v", a.debt)
	}
}

func TestUpdateBudgetClearsOnSpeedChangeAndPause(t *testing.T) {
	t.Parallel()
	a, sim, f, d := budgetColony(t, speedFast)
	*d = 3 * time.Millisecond
	stepFrame(t, a, sim, f)
	if a.debt.owed == 0 {
		t.Fatal("no debt to clear")
	}
	a.renderer.setSpeed(speedNormal)
	*d = 0
	if got := stepFrame(t, a, sim, f); got != 1 || a.debt.owed != 0 {
		t.Fatalf("6x->1x must not burst: stepped %d owed %d", got, a.debt.owed)
	}

	a.renderer.setSpeed(speedFast)
	*d = 3 * time.Millisecond
	stepFrame(t, a, sim, f)
	if a.debt.owed == 0 {
		t.Fatal("no debt to clear")
	}
	a.renderer.togglePause()
	*d = 0
	if got := stepFrame(t, a, sim, f); got != 0 {
		t.Fatalf("paused stepped %d", got)
	}
	a.renderer.togglePause() // resumes at normal
	a.renderer.setSpeed(speedFast)
	if a.debt.owed != 0 {
		t.Fatalf("pause kept the debt: %+v", a.debt)
	}
	if got := stepFrame(t, a, sim, f); got != 3 {
		t.Fatalf("resumed Fast stepped %d, want 3", got)
	}

	// the pause overlay clears the debt too
	*d = 3 * time.Millisecond
	stepFrame(t, a, sim, f)
	if a.debt.owed == 0 {
		t.Fatal("no debt to clear")
	}
	a.nav.setOverlay(overlayPause)
	a.tick()
	if a.debt.owed != 0 {
		t.Fatalf("pause overlay kept the debt: %+v", a.debt)
	}
}

func TestCaptureIgnoresBudget(t *testing.T) {
	t.Parallel()
	a, sim, _, d := budgetColony(t, speedFast)
	*d = 10 * time.Millisecond
	a.cfg.Capture = &Capture{}
	a.advanceColony()
	if sim.steps != 3 || a.debt.held != 0 || a.debt.owed != 0 {
		t.Fatalf("capture: steps %d debt %+v, want 3 steps and no debt", sim.steps, a.debt)
	}

	// the renderer's plain advance (tickCapture) always steps the full tier
	s := &stubSim{w: 8, h: 8}
	r := New(s)
	r.speed = speedVeryFast
	s.onStep = func() { time.Sleep(time.Millisecond) }
	r.advance()
	if s.steps != r.ticksWanted() || r.mo.Stepped != 6 {
		t.Fatalf("advance stepped %d (stepped %d), want %d", s.steps, r.mo.Stepped, r.ticksWanted())
	}
}

func TestDevSingleStepConsumedOnce(t *testing.T) {
	t.Parallel()
	s := &stubSim{w: 8, h: 8}
	r := New(s)
	r.speed = speedPaused
	r.stepTicks = 1
	if r.ticksWanted() != 1 {
		t.Fatalf("pending single step wants %d", r.ticksWanted())
	}
	r.advance()
	r.advance()
	if s.steps != 1 || r.stepTicks != 0 || r.mo.Stepped != 0 {
		t.Fatalf("single step: steps %d pending %d stepped %d", s.steps, r.stepTicks, r.mo.Stepped)
	}
}

func TestFPSReportShowsBudget(t *testing.T) {
	t.Parallel()
	st := &fpsProbeState{budgetHeld0: 2}
	got := budgetLine(st, tickDebt{held: 7, maxOwed: 3})
	if !strings.Contains(got, "budget held 5 updates") || !strings.Contains(got, "max 3 ticks owed") {
		t.Fatalf("budget line %q", got)
	}
}
