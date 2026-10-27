// Command benchmark runs the engine-scale fixtures (NG-1 §6.2): for each
// fixture x map size x speed it loads a grown colony, steps it frame by
// frame and prints p50/p95/p99 per tick, per frame and per Step phase.
//
//	go run ./cmd/benchmark -sizes 120,250,500 -ticks 3000 -speeds 1,3 -cache /tmp/fx
//
// Wall-clock time is observation only (game.PhaseObserver); it never
// feeds the simulation.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"runtime/pprof"
	"slices"
	"strconv"
	"strings"
	"time"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/application/scenario"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "benchmark:", err)
		os.Exit(1)
	}
}

type options struct {
	seed       uint64
	sizes      []int
	speeds     []int
	fixtures   []string
	ticks      int
	cache      string
	cpuprofile string
	buildOnly  bool
	hash       bool
}

func parse(args []string) (options, error) {
	fs := flag.NewFlagSet("benchmark", flag.ContinueOnError)
	seed := fs.Uint64("seed", 1, "fixture seed")
	sizes := fs.String("sizes", "120,250,500", "map sides, comma-separated")
	speeds := fs.String("speeds", "1,3", "ticks per frame (1 normal, 2 fast, 3 faster), comma-separated")
	fixtures := fs.String("fixtures", "grown,raid", "fixtures: grown (a grown colony) | raid (the same under a 12-raider assault)")
	ticks := fs.Int("ticks", 3000, "ticks to time per run")
	cache := fs.String("cache", os.Getenv("RIMWORLD_FIXTURE_DIR"), "directory keeping built fixtures between runs (default $RIMWORLD_FIXTURE_DIR)")
	cpu := fs.String("cpuprofile", "", "write a CPU profile of the timed ticks to this file")
	buildOnly := fs.Bool("build-only", false, "only build (and cache) the fixtures")
	hash := fs.Bool("hash", false, "print each run's final state hash (compare builds for identical behaviour)")
	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	o := options{seed: *seed, ticks: *ticks, cache: *cache, cpuprofile: *cpu, buildOnly: *buildOnly, hash: *hash}
	var err error
	if o.sizes, err = ints(*sizes); err != nil {
		return o, fmt.Errorf("-sizes: %w", err)
	}
	if o.speeds, err = ints(*speeds); err != nil {
		return o, fmt.Errorf("-speeds: %w", err)
	}
	for _, f := range strings.Split(*fixtures, ",") {
		if f != "grown" && f != "raid" {
			return o, fmt.Errorf("-fixtures: unknown %q", f)
		}
		o.fixtures = append(o.fixtures, f)
	}
	if o.ticks < 1 {
		return o, fmt.Errorf("-ticks must be positive")
	}
	return o, nil
}

func ints(s string) ([]int, error) {
	var out []int
	for _, f := range strings.Split(s, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(f))
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("bad value %q", f)
		}
		out = append(out, n)
	}
	return out, nil
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

// pct is the p-th percentile (0..100) of ds.
func pct(ds []time.Duration, p int) time.Duration {
	if len(ds) == 0 {
		return 0
	}
	s := slices.Clone(ds)
	slices.Sort(s)
	return s[min(len(s)-1, len(s)*p/100)]
}

func mean(ds []time.Duration) time.Duration {
	if len(ds) == 0 {
		return 0
	}
	var sum time.Duration
	for _, d := range ds {
		sum += d
	}
	return sum / time.Duration(len(ds))
}

func ms(d time.Duration) string { return fmt.Sprintf("%.3f", float64(d)/float64(time.Millisecond)) }

func report(out io.Writer, name string, r result) {
	fmt.Fprintf(out, "%-18s %8s %8s %8s %8s %8s  (ms)\n", name, "mean", "p50", "p95", "p99", "max")
	line := func(label string, ds []time.Duration) {
		fmt.Fprintf(out, "  %-16s %8s %8s %8s %8s %8s\n", label, ms(mean(ds)), ms(pct(ds, 50)), ms(pct(ds, 95)), ms(pct(ds, 99)), ms(pct(ds, 100)))
	}
	line("tick", r.ticks)
	line("frame", r.frames)
	line("views", r.views)
	for _, p := range game.Phases() {
		line(p, r.phases[p])
	}
}
