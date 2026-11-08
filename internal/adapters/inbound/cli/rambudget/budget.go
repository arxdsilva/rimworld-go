package rambudget

// reserveMiB is kept for the OS, the editor and the lead's own checks.
const reserveMiB = 4 * 1024

// Measured max RSS (/usr/bin/time -l) on an 18 GiB, 11-core M3 Pro,
// 2026-10-25 (PLAY-019):
//   - go test -race ./internal/adapters/inbound/ebiten/: 4573 MiB (it
//     loads the art); ./internal/application/game/: 325 MiB; the two
//     trees together with -p 2: 4770 MiB;
//   - the game itself (captures, fps probes): 2.4-2.7 GiB; after the
//     PLAY-019 art diet about 1.1 GiB RSS on every map, with a peak
//     footprint (which counts the GPU's share of unified memory) of
//     2.2 GiB on a small or 250 map and 3.4 GiB on a grown 500;
//   - a targeted go test -race -run in the ebiten package: about 355 MiB
//     (plus about 1 GiB while the race build compiles);
//   - go vet ./...: 60 MiB.
//
// A Blender family render is not measured yet: about 3 GiB, an estimate.
const (
	heavyPeakMiB   = 4800 // a -race run of the ebiten package (or any test that loads the art)
	artPeakMiB     = 3072 // Blender, or a game run on a big map or an fps probe
	capturePeakMiB = 2304 // a game run on a small map for captures (footprint)
	lightPeakMiB   = 1024 // targeted tests of packages without art, vet, docs
)

// maxAgents is the parallel-work ceiling (PARALLEL_WORK §5).
const maxAgents = 30

// agentKinds are the measured peaks (max RSS) of what agents run. Re-measure
// with /usr/bin/time -l when the packages grow; the figures and the
// machine they came from are in the PLAY-019 handoff.
var agentKinds = []struct {
	Name    string
	PeakMiB int
	What    string
}{
	{"heavy", heavyPeakMiB, "a full go test -race of the ebiten package: the lead's gate, run alone"},
	{"art", artPeakMiB, "a Blender family render, a game run on a 250+ map, or an fps probe"},
	{"capture", capturePeakMiB, "a game run on a small map for screenshots (-capture), one at a time"},
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
