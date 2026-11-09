package sched

import "time"

// Schedule is the systems in run order and, while an observer is set,
// their timings.
type Schedule struct {
	sys   []System
	end   []bool // the last system of its phase's run
	stats []stat
	obs   Observer
}

// Observer receives each phase's wall-clock time once a pass, after its
// last system (systems skipped on the pass count as none). Observation
// only: a game steps identically with or without one.
type Observer func(phase string, elapsed time.Duration)

type stat struct {
	runs       uint64
	total, max time.Duration
}

// Observe installs o (nil removes it). Timing, and the Snapshot's counts,
// are taken only while one is set.
func (s *Schedule) Observe(o Observer) { s.obs = o }

// Pass runs the order once for one map at tick: primary is the first
// map's pass, the only one that runs the Colony systems.
func (s *Schedule) Pass(primary bool, tick uint64) { s.pass(primary, tick, "") }

// PassPhase is Pass for the systems of one phase only (tests that drive a
// phase on its own).
func (s *Schedule) PassPhase(phase string, primary bool, tick uint64) { s.pass(primary, tick, phase) }

// pass runs the order, or only phase's systems when phase is set.
func (s *Schedule) pass(primary bool, tick uint64, phase string) {
	var acc time.Duration
	for i := range s.sys {
		y := &s.sys[i]
		if phase != "" && y.Phase != phase {
			continue
		}
		run := (primary || y.Scope == PerMap) && (y.Every == 0 || (tick+y.Offset)%y.Every == 0) && (y.RunIf == nil || y.RunIf())
		if s.obs == nil {
			if run {
				y.Run()
			}
			continue
		}
		if run {
			t0 := time.Now()
			y.Run()
			d := time.Since(t0)
			st := &s.stats[i]
			st.runs, st.total, st.max, acc = st.runs+1, st.total+d, max(st.max, d), acc+d
		}
		if s.end[i] {
			s.obs(y.Phase, acc)
			acc = 0
		}
	}
}

// Stat is one system's place and, from the observed passes, how often it
// ran and how long it took.
type Stat struct {
	Name, Phase   string
	Scope         Scope
	Every, Offset uint64
	Runs          uint64
	Mean, Max     time.Duration
}

// Snapshot is every system in run order with its counts (a read model:
// GAR-012's debug HUD).
func (s *Schedule) Snapshot() []Stat {
	out := make([]Stat, len(s.sys))
	for i, y := range s.sys {
		st := s.stats[i]
		out[i] = Stat{Name: y.Name, Phase: y.Phase, Scope: y.Scope, Every: y.Every, Offset: y.Offset, Runs: st.runs, Max: st.max}
		if st.runs > 0 {
			out[i].Mean = st.total / time.Duration(st.runs)
		}
	}
	return out
}
