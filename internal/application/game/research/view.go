package research

import (
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/recipe"
	proj "rimworld-go/internal/domain/research"
	"rimworld-go/internal/domain/sim/clock"
)

// View is one project for the Research tab (game.ResearchView).
type View struct {
	ID        uint8
	Name      string
	Label     string
	Desc      string
	Cost      int // base cost; Progress is on the same scale
	Progress  int
	Done      bool
	Available bool
	Current   bool
	Requires  []string // labels of unfinished prerequisites
	Unlocks   []string // player-facing: "drying rack", "healroot crop", ...
	// TechLevel and Bench in words; CostNow is the cost at the colony's
	// tech level (the "X / Y" Y; X is Progress*CostNow/Cost); Hidden
	// prerequisites' labels (all of them, never drawn); BenchMissing why
	// no bench can work it ("" when one can); ETAHours at today's rate
	// (0 unknown).
	TechLevel, Bench, BenchMissing string
	CostNow, ETAHours              int
	Hidden                         []string
	// Prereqs is the ids of every drawn prerequisite, done or not (the
	// tree's lines); shared by every list, never written.
	Prereqs []int
}

// Why no bench can work a project (View.BenchMissing).
const (
	MissingBench    = "needs a research bench"
	MissingHiTech   = "needs a hi-tech research bench"
	MissingAnalyzer = "needs a powered multi-analyzer linked to a hi-tech research bench"
)

// viewKey is everything the views read; memo is the last lists.
type viewKey struct {
	prog    proj.Progress
	cur     proj.Project
	set     bool
	tiers   [proj.NumBenchTiers]bool
	rate    int64
	tech    proj.TechLevel
	costPct int
	ok      bool
}

type memo struct {
	key    viewKey
	views  []View
	locked map[string]string
}

// Lists are the Research tab's projects and every still-locked unlock key
// with the project that opens it; with cache on, the last lists while
// nothing they read changed (shared: callers never write them).
func (l *Lab) Lists(cache bool) ([]View, map[string]string) {
	k := viewKey{prog: l.Prog, cur: l.Cur, set: l.Set, tiers: l.tiers(), rate: l.rate(), tech: l.h.Tech(),
		costPct: l.h.CostPct(), ok: true}
	if cache && l.memo.key == k {
		return l.memo.views, l.memo.locked
	}
	l.memo = memo{key: k, views: l.views(k), locked: l.lockedKeys()}
	return l.memo.views, l.memo.locked
}

func (l *Lab) views(k viewKey) []View {
	if l.prereqs == nil { // static: built once, shared by every list
		l.prereqs = make([][]int, proj.NumProjects)
		for _, d := range proj.All() {
			for _, r := range d.Requires {
				l.prereqs[d.Project] = append(l.prereqs[d.Project], int(r))
			}
		}
	}
	out := make([]View, 0, proj.NumProjects)
	for _, d := range proj.All() {
		p := d.Project
		v := View{ID: uint8(p), Name: d.Name, Label: d.Label, Desc: d.Desc, Cost: d.Cost, Progress: l.Prog[p],
			Done: l.Prog.Done(p), Unlocks: UnlockLabels(d), Available: l.Prog.Available(p), Current: l.Set && l.Cur == p,
			TechLevel: d.TechLevel.String(), Bench: d.Bench.String(), CostNow: d.Cost * l.costPct(p) / 100, Prereqs: l.prereqs[p]}
		for _, r := range d.Requires {
			if !l.Prog.Done(r) {
				v.Requires = append(v.Requires, proj.DefOf(r).Label)
			}
		}
		for _, h := range d.Hidden {
			v.Hidden = append(v.Hidden, proj.DefOf(h).Label)
		}
		if !v.Done {
			v.BenchMissing = missing(d.Bench, k.tiers)
			if v.BenchMissing == "" && k.rate > 0 {
				left := int64(d.Cost-l.Prog[p]) * int64(l.costPct(p)*l.h.CostPct()) / 100 * nano / 100
				v.ETAHours = int((left/k.rate + clock.TicksPerHour - 1) / clock.TicksPerHour)
			}
		}
		out = append(out, v)
	}
	return out
}

// missing is why no bench of the colony can work a project of tier t.
func missing(t proj.BenchTier, have [proj.NumBenchTiers]bool) string {
	switch {
	case have[t]:
		return ""
	case t == proj.HiTechAnalyzer && have[proj.HiTechBench]:
		return MissingAnalyzer
	case t >= proj.HiTechBench:
		return MissingHiTech
	}
	return MissingBench
}

// tiers is which project tiers some research bench can work now.
func (l *Lab) tiers() (have [proj.NumBenchTiers]bool) {
	for _, id := range l.cache().benches {
		b, ok := l.h.Building(id)
		if !ok {
			continue
		}
		have[proj.SimpleBench] = true
		if hiTech(b.Kind) {
			have[proj.HiTechBench] = true
			have[proj.HiTechAnalyzer] = have[proj.HiTechAnalyzer] || l.analyzed(id)
		}
	}
	return have
}

// rate is the colony's raw research points a tick (billionths) on the
// last full tick, 0 when nobody researched then.
func (l *Lab) rate() int64 {
	switch l.h.Now() {
	case l.rateTick + 1:
		return l.rateNow
	case l.rateTick:
		return l.rateWas
	}
	return 0
}

// lockedKeys maps every still-locked unlock key to the project that
// opens it (the UI greys these out).
func (l *Lab) lockedKeys() map[string]string {
	out := map[string]string{}
	for _, d := range proj.All() {
		if l.Prog.Done(d.Project) {
			continue
		}
		for _, u := range d.Unlocks {
			out[u] = d.Label
		}
	}
	return out
}

// Alert is the "need a research project" alert: no project picked while
// one is open, the colony has a research bench and a colonist who can
// research. Its inputs change only on discrete events (a pick, a finish,
// a bench built or lost, work toggled), so it needs no hysteresis.
func (l *Lab) Alert() bool {
	if l.Working() || len(l.cache().benches) == 0 {
		return false
	}
	for p := proj.Project(0); p < proj.NumProjects; p++ {
		if l.Prog.Available(p) {
			return l.h.Researcher()
		}
	}
	return false
}

// UnlockLabels is d's unlocks in words (nil: none).
func UnlockLabels(d proj.Def) (out []string) {
	for _, u := range d.Unlocks {
		out = append(out, unlockLabel(u))
	}
	return out
}

// unlockLabel turns an unlock key into words.
func unlockLabel(key string) string {
	for _, p := range []struct{ prefix, suffix string }{{"build:", ""}, {"crop:", " crop"}, {"recipe:", ""}} {
		if len(key) > len(p.prefix) && key[:len(p.prefix)] == p.prefix {
			name := key[len(p.prefix):]
			if p.prefix == "recipe:" {
				for _, r := range recipe.All() {
					if r.Name == name {
						return r.Label
					}
				}
			}
			if name == construction.ElectricResearchBench.String() {
				return HiTechLabel
			}
			return name + p.suffix
		}
	}
	return key
}

// HiTechLabel is what the player calls the electric research bench kind.
const HiTechLabel = "hi-tech research bench"
