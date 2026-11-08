package benchmark

import (
	"fmt"
	"io"
	"slices"
	"time"

	"rimworld-go/internal/application/game"
)

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
	over := overBudget(r.frames)
	fmt.Fprintf(out, "  frames > %v: %d (%.1f%%)\n", frameBudget, over, 100*float64(over)/float64(max(len(r.frames), 1)))
}

// frameBudget mirrors the ebiten adapter's Update budget (update_budget.go):
// at 3x and faster a frame whose ticks run longer carries the rest over.
const frameBudget = 5 * time.Millisecond

// overBudget counts the frames whose ticks took longer than frameBudget:
// the Updates the budget would hold.
func overBudget(frames []time.Duration) int {
	n := 0
	for _, f := range frames {
		if f > frameBudget {
			n++
		}
	}
	return n
}
