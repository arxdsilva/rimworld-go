package ebiten

import (
	"fmt"
	"log"
	"slices"
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
}

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
		if p.Zoom > 0 {
			a.renderer.cam.zoom = clampZoom(p.Zoom)
		}
		tile := a.renderer.tilePx()
		a.renderer.cam.offsetX = (float64(a.renderer.game.Width())/2+0.5)*tile - float64(a.sw)/2
		a.renderer.cam.offsetY = (float64(a.renderer.game.Height())/2)*tile - float64(a.sh)/2
		st.started, st.begin = true, now
		st.warmupTo = now.Add(3 * time.Second) // atlas pages, first bakes
		return false
	}
	if now.Sub(st.begin) > time.Duration(p.Seconds)*time.Second+3*time.Second {
		st.report = fpsReport(st)
		log.Print(st.report)
		return true
	}
	st.upStart = now
	return false
}

// fpsProbeAfterUpdate records the Update's CPU time.
func (a *App) fpsProbeAfterUpdate() {
	if st := &a.fps; a.cfg.FPSProbe != nil && st.started && !st.upStart.IsZero() && time.Now().After(st.warmupTo) {
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
	if now.After(st.warmupTo) {
		if !st.last.IsZero() {
			st.frames = append(st.frames, now.Sub(st.last))
		}
		st.draw = append(st.draw, now.Sub(start))
	}
	st.last = now
}

func fpsReport(st *fpsProbeState) string {
	pct := func(d []time.Duration, q float64) time.Duration {
		if len(d) == 0 {
			return 0
		}
		s := slices.Clone(d)
		slices.Sort(s)
		return s[min(len(s)-1, int(float64(len(s))*q))]
	}
	avg := func(d []time.Duration) time.Duration {
		if len(d) == 0 {
			return 0
		}
		var t time.Duration
		for _, x := range d {
			t += x
		}
		return t / time.Duration(len(d))
	}
	fps := 0.0
	if a := avg(st.frames); a > 0 {
		fps = float64(time.Second) / float64(a)
	}
	low := 0.0
	if p := pct(st.frames, 0.99); p > 0 {
		low = float64(time.Second) / float64(p)
	}
	return fmt.Sprintf("fps probe: %d frames, avg %.1f fps, 1%% low %.1f fps (frame p99 %v); update avg %v p99 %v; draw avg %v p99 %v",
		len(st.frames), fps, low, pct(st.frames, 0.99), avg(st.update), pct(st.update, 0.99), avg(st.draw), pct(st.draw, 0.99))
}
