package ebiten

import (
	"fmt"
	"log"
	"runtime"
	"time"
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

type fpsProbeState struct {
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
	// speed is the probe's speed, put back every frame (a threat or the
	// window losing focus would pause it and make the numbers meaningless);
	// tick0 the game tick at the start, held how often it was put back
	// (speed resets, not the Update budget: budgetHeld0 is a.debt.held as
	// the measured span starts, so the report counts that span's holds).
	speed       speed
	tick0       uint64
	held        int
	budgetHeld0 int
}

// probeWarmup is the unmeasured time after the first colony frame
// (atlas pages, first bakes).
const probeWarmup = 3 * time.Second

// fpsProbeUpdate starts the probe's colony and reports when time is up
// (true: quit). It times each Update.
func (a *App) fpsProbeUpdate() bool {
	p := a.cfg.FPSProbe
	if p == nil {
		return false
	}
	st := &a.fps
	now := time.Now()
	if !st.started {
		if a.nav.screen() == screenBoot {
			a.finishBoot()
		}
		a.startGame()
		if a.renderer == nil {
			log.Printf("fps probe: %s", a.sessionErr)
			return true
		}
		switch p.Speed {
		case 1:
			a.renderer.speed = speedNormal
		case 3:
			a.renderer.speed = speedVeryFast
		default:
			a.renderer.speed = speedFast
		}
		st.speed, st.tick0 = a.renderer.speed, uint64(a.renderer.game.Tick())
		if p.Zoom > 0 {
			a.renderer.cam.zoom = clampZoom(p.Zoom)
		}
		a.renderer.syncDev() // the probe frames the screen, in map pixels
		tile, k := a.renderer.tilePx(), a.scale.worldK()
		a.renderer.cam.offsetX = (float64(a.renderer.game.Width())/2+0.5)*tile - float64(a.pw)*k/2
		a.renderer.cam.offsetY = (float64(a.renderer.game.Height())/2)*tile - float64(a.ph)*k/2
		st.started, st.begin = true, now
		st.warmupTo = now.Add(probeWarmup) // atlas pages, first bakes
		return false
	}
	if st.drawn && now.Sub(st.begin) > time.Duration(p.Seconds)*time.Second+probeWarmup {
		st.report = fpsReport(st, a.scale) + fmt.Sprintf(", %dx%d physical; %d ticks, speed held %d times; %v",
			a.pw, a.ph, uint64(a.renderer.game.Tick())-st.tick0, st.held, a.art.memStats()) + budgetLine(st, a.debt)
		log.Print(st.report)
		return true
	}
	if a.renderer.speed != st.speed { // a threat pause or the window losing focus
		a.renderer.speed = st.speed
		st.held++
	}
	if !st.measured && st.drawn && now.After(st.warmupTo) {
		st.measured = true
		st.budgetHeld0, a.debt.maxOwed = a.debt.held, 0 // the budget line covers the measured span
		if p.Measuring != nil {
			p.Measuring()
		}
	}
	st.upStart = now
	return false
}

// budgetLine is the Update budget's part of the report (PERF-002 HS7):
// Updates in the measured span that stopped short, and the largest debt.
func budgetLine(st *fpsProbeState, d tickDebt) string {
	return fmt.Sprintf("; budget held %d updates, max %d ticks owed", d.held-st.budgetHeld0, d.maxOwed)
}

// fpsProbeAfterUpdate records the Update's CPU time.
func (a *App) fpsProbeAfterUpdate() {
	if st := &a.fps; a.cfg.FPSProbe != nil && st.started && st.drawn && st.upStart.After(st.warmupTo) {
		st.update = append(st.update, time.Since(st.upStart))
	}
}

// fpsProbeDraw times a Draw (start: before, done: after).
func (a *App) fpsProbeDraw(start time.Time) {
	st := &a.fps
	if a.cfg.FPSProbe == nil || !st.started {
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

// fpsReport summarises the measured frames at UI scale sc (s and dev,
// PLAY-012: HiDPI renders the map at dev x the pixels).
func fpsReport(st *fpsProbeState, sc uiScaleState) string {
	f := frameStats(st.frames, st.update, st.draw)
	fps, low := f.FPS, f.Low1
	// work is the CPU time a frame costs (one Update plus one Draw): the
	// display-independent number (PERF-002), since a 30 Hz or occluded
	// display caps the frame rate whatever the renderer does.
	upAvg, upP99, drAvg, drP99 := f.UpdateAvg, f.UpdateP99, f.DrawAvg, f.DrawP99
	return fmt.Sprintf("fps probe: %d frames, avg %.1f fps, 1%% low %.1f fps (frame p99 %v); update avg %v p99 %v; draw avg %v p99 %v; work avg %v p99 %v; scale s %g dev %g world %g; %s",
		len(st.frames), fps, low, f.FrameP99, upAvg, upP99, drAvg, drP99, upAvg+drAvg, upP99+drP99, sc.s, sc.dev, sc.worldK(), memLine())
}

// memLine is the Go runtime's memory at the end of a probe (PLAY-019): the
// heap in use and what the runtime holds from the OS. Process RSS, which
// also counts GPU-shared buffers, comes from /usr/bin/time -l.
func memLine() string {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return fmt.Sprintf("go heap %d MiB, go sys %d MiB", m.HeapInuse>>20, m.Sys>>20)
}
