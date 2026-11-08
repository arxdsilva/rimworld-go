package benchmark

import (
	"fmt"
	"io"
	"os"
	"runtime/pprof"
	"time"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/application/scenario"
)

// Main runs the tool with args (without the program name), writes its
// report to out and its error to errOut, and returns the exit code.
func Main(args []string, out, errOut io.Writer) int {
	if err := run(args, out); err != nil {
		fmt.Fprintln(errOut, "benchmark:", err)
		return 1
	}
	return 0
}

func run(args []string, out io.Writer) error {
	o, err := parse(args)
	if err != nil {
		return err
	}
	scenario.CacheDir = o.cache
	if o.cpuprofile != "" {
		f, err := os.Create(o.cpuprofile)
		if err != nil {
			return err
		}
		defer f.Close()
		if err := pprof.StartCPUProfile(f); err != nil {
			return err
		}
		defer pprof.StopCPUProfile()
	}
	for _, fx := range o.fixtures {
		for _, side := range o.sizes {
			t0 := time.Now()
			data := scenario.GrownColony(o.seed, side, side, fx == "raid")
			fmt.Fprintf(out, "fixture %s %dx%d: %d KB, ready in %v\n", fx, side, side, len(data)/1024, time.Since(t0).Round(time.Millisecond))
			if o.buildOnly {
				continue
			}
			for _, speed := range o.speeds {
				g, err := scenario.Load(data)
				if err != nil {
					return err
				}
				report(out, fmt.Sprintf("%s/%d/x%d", fx, side, speed), measure(g, o.ticks, speed))
				if o.hash {
					fmt.Fprintf(out, "  state hash after %d ticks: %#x\n", o.ticks, g.StateHash())
				}
			}
		}
	}
	return nil
}

// result is one run's samples: views is the read model (World) the
// renderer takes once a frame, after the frame's ticks.
type result struct {
	ticks, frames, views []time.Duration
	phases               map[string][]time.Duration
}

// measure steps g for ticks ticks in frames of speed ticks, timing every
// tick, frame and phase, and one World() per frame with the view cache on
// (the renderer's setting; views never change the simulation).
func measure(g *game.Game, ticks, speed int) result {
	r := result{phases: map[string][]time.Duration{}}
	tickPhase := map[string]time.Duration{}
	g.SetViewCache(true)
	g.SetPhaseObserver(func(p string, d time.Duration) { tickPhase[p] += d })
	for done := 0; done < ticks; {
		f0 := time.Now()
		for i := 0; i < speed && done < ticks; i++ {
			clear(tickPhase)
			t0 := time.Now()
			g.Step(1)
			r.ticks = append(r.ticks, time.Since(t0))
			for p, d := range tickPhase {
				r.phases[p] = append(r.phases[p], d)
			}
			done++
		}
		r.frames = append(r.frames, time.Since(f0))
		v0 := time.Now()
		_ = g.World()
		r.views = append(r.views, time.Since(v0))
	}
	g.SetPhaseObserver(nil)
	return r
}
