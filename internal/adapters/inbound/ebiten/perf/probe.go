package perf

import (
	"fmt"
	"log"
	"runtime"
	"time"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
)

// FPSProbe is a dev measurement (NG-1 §6.9): start the colony, run it at
// Speed for Seconds of real time at Zoom, then log frame statistics and
// quit. Wall-clock time is used for observation only; it never reaches
// the simulation.
type FPSProbe struct {
	Seconds int
	Zoom    float64
	Speed   int // 1, 2 (3 ticks/frame) or 3 (6 ticks/frame); 0 means 2
	// Measuring, when set, runs once as the warm-up ends (dev: start a
	// CPU profile of the measured span only).
	Measuring func()
}

// Probe is the frame-rate probe's state.
type Probe struct {
	started  bool
	begin    time.Time
	last     time.Time
	frames   []time.Duration // gaps between Draw calls
	update   []time.Duration // Update CPU time
	draw     []time.Duration // Draw CPU time
	upStart  time.Time
	warmupTo time.Time
	report   string
	measured bool
	drawn    bool // the first colony frame is done (warm-up runs from it)
	// speed is the probe's speed tier, put back every frame (a threat or
	// the window losing focus would pause it and make the numbers
	// meaningless); tick0 the game tick at the start, held how often it was
	// put back (speed resets, not the Update budget: BudgetHeld0 is the
	// budget's held count as the measured span starts, so the report counts
	// that span's holds).
	speed       int
	tick0       uint64
	held        int
	BudgetHeld0 int
}

// Report is the probe's final report ("" until it ran).
func (p *Probe) Report() string { return p.report }

// probeWarmup is the unmeasured time after the first colony frame
// (atlas pages, first bakes).
const probeWarmup = 3 * time.Second

// ProbeUpdate starts the probe's colony and reports when time is up
// (true: quit). It times each Update.
func (m *Meter) ProbeUpdate() bool {
	env := m.sh.Env()
	p := env.Probe
	if p == nil {
		return false
	}
	st := m.Probe
	now := time.Now()
	if !st.started {
		speed, tick0, err := m.sh.StartProbe(p)
		if err != "" {
			log.Printf("fps probe: %s", err)
			return true
		}
		st.speed, st.tick0 = speed, tick0
		st.started, st.begin = true, now
		st.warmupTo = now.Add(probeWarmup) // atlas pages, first bakes
		return false
	}
	if st.drawn && now.Sub(st.begin) > time.Duration(p.Seconds)*time.Second+probeWarmup {
		st.report = Report(st, env.Scale) + fmt.Sprintf(", %dx%d physical; %d ticks, speed held %d times; %v",
			env.PW, env.PH, env.Tick-st.tick0, st.held, m.sh.ArtMem()) + BudgetLine(st, env.Held, env.MaxOwed)
		log.Print(st.report)
		return true
	}
	if env.Speed != st.speed { // a threat pause or the window losing focus
		m.sh.ForceSpeed(st.speed)
		st.held++
	}
	if !st.measured && st.drawn && now.After(st.warmupTo) {
		st.measured = true
		st.BudgetHeld0 = env.Held // the budget line covers the measured span
		m.sh.ResetMaxOwed()
		if p.Measuring != nil {
			p.Measuring()
		}
	}
	st.upStart = now
	return false
}

// BudgetLine is the Update budget's part of the report (PERF-002 HS7):
// Updates in the measured span that stopped short (held now), and the
// largest debt.
func BudgetLine(st *Probe, held, maxOwed int) string {
	return fmt.Sprintf("; budget held %d updates, max %d ticks owed", held-st.BudgetHeld0, maxOwed)
}

// ProbeAfterUpdate records the Update's CPU time.
func (m *Meter) ProbeAfterUpdate() {
	if st := m.Probe; st.started && st.drawn && st.upStart.After(st.warmupTo) {
		st.update = append(st.update, time.Since(st.upStart))
	}
}

// ProbeDraw times a Draw (start: before, now: after).
func (m *Meter) ProbeDraw(start time.Time) {
	st := m.Probe
	if !st.started {
		return
	}
	now := time.Now()
	if !st.drawn {
		// The first colony frame loads art and bakes for seconds: the
		// warm-up (and the run) counts from its end.
		st.drawn = true
		st.begin, st.warmupTo = now, now.Add(probeWarmup)
	}
	if start.After(st.warmupTo) {
		if !st.last.IsZero() {
			st.frames = append(st.frames, now.Sub(st.last))
		}
		st.draw = append(st.draw, now.Sub(start))
	}
	st.last = now
}

// Report summarises the measured frames at UI scale sc (s and dev,
// PLAY-012: HiDPI renders the map at dev x the pixels).
func Report(st *Probe, sc gfx.Scale) string {
	f := FrameStats(st.frames, st.update, st.draw)
	fps, low := f.FPS, f.Low1
	// work is the CPU time a frame costs (one Update plus one Draw): the
	// display-independent number (PERF-002), since a 30 Hz or occluded
	// display caps the frame rate whatever the renderer does.
	upAvg, upP99, drAvg, drP99 := f.UpdateAvg, f.UpdateP99, f.DrawAvg, f.DrawP99
	return fmt.Sprintf("fps probe: %d frames, avg %.1f fps, 1%% low %.1f fps (frame p99 %v); update avg %v p99 %v; draw avg %v p99 %v; work avg %v p99 %v; scale s %g dev %g world %g; %s",
		len(st.frames), fps, low, f.FrameP99, upAvg, upP99, drAvg, drP99, upAvg+drAvg, upP99+drP99, sc.S, sc.Dev, sc.WorldK(), memLine())
}

// memLine is the Go runtime's memory at the end of a probe (PLAY-019): the
// heap in use and what the runtime holds from the OS. Process RSS, which
// also counts GPU-shared buffers, comes from /usr/bin/time -l.
func memLine() string {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return fmt.Sprintf("go heap %d MiB, go sys %d MiB", m.HeapInuse>>20, m.Sys>>20)
}
