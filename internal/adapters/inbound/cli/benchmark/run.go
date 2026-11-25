// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package benchmark

import (
	"fmt"
	"io"
	"os"
	"runtime"
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
			for _, n := range o.colonists {
				if err := runFixture(out, o, fx, side, n); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// runFixture builds (or loads from the cache) one fixture of n colonists on
// a side x side map and, unless build-only, times it at every speed.
func runFixture(out io.Writer, o options, fx string, side, n int) error {
	t0 := time.Now()
	data, err := scenario.GrownColony(o.seed, side, side, n, fx == "raid")
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "fixture %s %dx%d c%d: %d KB, ready in %v\n", fx, side, side, n, len(data)/1024, time.Since(t0).Round(time.Millisecond))
	if o.buildOnly {
		return nil
	}
	for _, speed := range o.speeds {
		g, err := scenario.Load(data)
		if err != nil {
			return err
		}
		report(out, fmt.Sprintf("%s/%d/c%d/x%d", fx, side, n, speed), measure(g, o.ticks, speed))
		if o.hash {
			fmt.Fprintf(out, "  state hash after %d ticks: %#x\n", o.ticks, g.StateHash())
		}
	}
	return nil
}

// result is one run's samples: views is the read model (World) the
// renderer takes once a frame, after the frame's ticks. mallocs and
// allocBytes are the heap objects and bytes allocated over the timed loop.
type result struct {
	ticks, frames, views []time.Duration
	phases               map[string][]time.Duration
	mallocs, allocBytes  uint64
}

// allocDelta is the heap objects and bytes allocated between two MemStats
// reads (Mallocs and TotalAlloc only grow).
func allocDelta(before, after *runtime.MemStats) (mallocs, bytes uint64) {
	return after.Mallocs - before.Mallocs, after.TotalAlloc - before.TotalAlloc
}

// perTick is the mean heap objects and bytes allocated per tick.
func perTick(mallocs, bytes uint64, ticks int) (objects, perBytes float64) {
	if ticks <= 0 {
		return 0, 0
	}
	return float64(mallocs) / float64(ticks), float64(bytes) / float64(ticks)
}

// measure steps g for ticks ticks in frames of speed ticks, timing every
// tick, frame and phase, and one World() per frame with the view cache on
// (the renderer's setting; views never change the simulation). Sample
// slices are sized up front so the harness allocates nothing in the loop;
// MemStats is read once before the first frame and once after the last.
func measure(g *game.Game, ticks, speed int) result {
	frames := (ticks + speed - 1) / speed
	r := result{
		ticks:  make([]time.Duration, 0, ticks),
		frames: make([]time.Duration, 0, frames),
		views:  make([]time.Duration, 0, frames),
		phases: map[string][]time.Duration{},
	}
	for _, p := range game.Phases() {
		r.phases[p] = make([]time.Duration, 0, ticks)
	}
	tickPhase := make(map[string]time.Duration, len(r.phases))
	g.SetViewCache(true)
	g.SetPhaseObserver(func(p string, d time.Duration) { tickPhase[p] += d })
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
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
	runtime.ReadMemStats(&after)
	r.mallocs, r.allocBytes = allocDelta(&before, &after)
	g.SetPhaseObserver(nil)
	return r
}
