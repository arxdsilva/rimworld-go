// Command rambudget prints how many parallel agents this machine can run
// without swapping, from its free memory and measured per-agent peaks
// (PLAY-019; .cursor/PARALLEL_WORK.md §1). The lead runs it, through
// `make agent-budget`, before launching agents and before each phase.
package main

import (
	"fmt"
	"os"

	"rimworld-go/internal/platform/sysinfo"
)

// reserveMiB is kept for the OS, the editor and the lead's own checks.
const reserveMiB = 4 * 1024

// Measured max RSS (/usr/bin/time -l) on an 18 GiB, 11-core M3 Pro,
// 2026-10-25 (PLAY-019):
//   - go test -race ./internal/adapters/inbound/ebiten/: 4573 MiB (it
//     loads the art); ./internal/application/game/: 325 MiB; the two
//     trees together with -p 2: 4770 MiB;
//   - the game itself (captures, fps probes): 2.4-2.7 GiB;
//   - a targeted go test -race -run in the ebiten package: about 355 MiB
//     (plus about 1 GiB while the race build compiles);
//   - go vet ./...: 60 MiB.
//
// A Blender family render is not measured yet: about 3 GiB, an estimate.
const (
	heavyPeakMiB = 4800 // a -race run of the ebiten package (or any test that loads the art)
	artPeakMiB   = 3072 // Blender, or a game run for captures
	lightPeakMiB = 1024 // targeted tests of packages without art, vet, docs
)

// maxAgents is the parallel-work ceiling (PARALLEL_WORK §5).
const maxAgents = 15

// agentKinds are the measured peaks (max RSS) of what agents run. Re-measure
// with /usr/bin/time -l when the packages grow; the figures and the
// machine they came from are in the PLAY-019 handoff.
var agentKinds = []struct {
	Name    string
	PeakMiB int
	What    string
}{
	{"heavy", heavyPeakMiB, "a full go test -race of the ebiten package: the lead's gate, run alone"},
	{"art", artPeakMiB, "a Blender family render, or game runs for captures and fps probes"},
	{"light", lightPeakMiB, "an agent's targeted go test -race -run, go vet, docs and reviews"},
}

// Kind is one agent kind's budget.
type Kind struct {
	Name    string
	PeakMiB int
	Max     int
	What    string
}

// Budget is what fits in the free memory.
type Budget struct {
	AvailableMiB, BudgetMiB int
	Kinds                   []Kind
}

// budget sizes agent work to availMiB of free memory.
func budget(availMiB, reserve int) Budget {
	b := Budget{AvailableMiB: availMiB, BudgetMiB: max(0, availMiB-reserve)}
	for _, k := range agentKinds {
		b.Kinds = append(b.Kinds, Kind{Name: k.Name, PeakMiB: k.PeakMiB, Max: min(b.BudgetMiB/k.PeakMiB, maxAgents), What: k.What})
	}
	return b
}

func main() {
	i := sysinfo.Read()
	if i.Total == 0 {
		fmt.Fprintln(os.Stderr, "rambudget: this platform does not report its memory; size agent batches by hand")
		os.Exit(1)
	}
	avail := int(i.Available >> 20)
	b := budget(avail, reserveMiB)
	fmt.Printf("memory: %.1f GiB total, %.1f GiB free, %d cores\n", i.TotalGiB(), i.AvailableGiB(), i.Cores)
	fmt.Printf("budget: %.1f GiB after a %.0f GiB reserve for the OS, editor and lead\n", float64(b.BudgetMiB)/1024, float64(reserveMiB)/1024)
	for _, k := range b.Kinds {
		fmt.Printf("  %-5s up to %2d at once (peak %.1f GiB each: %s)\n", k.Name, k.Max, float64(k.PeakMiB)/1024, k.What)
	}
	fmt.Println("mixing kinds: keep the sum of their peaks under the budget; start heavy agents a few minutes apart; agents test with GOFLAGS=-p=2")
	if b.Kinds[0].Max == 0 {
		fmt.Println("too little free memory for even one heavy agent: work alone or close programs first")
	}
}
