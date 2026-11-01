package research

import (
	"slices"

	"rimworld-go/internal/domain/difficulty"
	proj "rimworld-go/internal/domain/research"
	"rimworld-go/internal/domain/sim/entity"
)

// legacyCost is every project's cost before RSCH-1 (saves up to v8), in
// project order; the projects RSCH-1 added had none.
var legacyCost = [...]int{2000, 2500, 3000, 2500, 6000, 5000, 4000, 2500, 3500, 4000, 1500, 3000, 2500, 4000, 3500, 5000,
	6000, 4500, 3000, 3000, 3500, 2500, 4000, 4000, 5000, 6000, 7000, 6000, 3500, 2000, 3000, 5000, 6000, 7000, 2500, 1500,
	3000, 3000, 4500, 7000, 4000, 3500, 9000, 2500, 3000}

// Migrate brings research from a save of before RSCH-1 (v8 or older) to
// the wiki costs: each project keeps the same fraction of its cost (a
// finished one stays finished), the carried fraction of a point (then in
// hundredths) is dropped, and a project picked that is no longer open is
// let go. A project already at exactly its wiki cost counts as finished,
// so a save that is already current comes through unchanged.
func (l *Lab) Migrate() {
	for p := range legacyCost {
		old, now := legacyCost[p], proj.DefOf(proj.Project(p)).Cost
		switch v := l.Prog[p]; {
		case v >= old || v == now: // v == now: already at the wiki cost (a current save relabelled, a sandbox)
			l.Prog[p] = now
		case v > 0:
			l.Prog[p] = min(now-1, max(1, int(int64(v)*int64(now)/int64(old))))
		}
	}
	l.Carry = 0
	if l.Set && !l.Prog.Available(l.Cur) {
		l.Set = false
	}
}

// legacySpeed is the research speed of the presets RSCH-1 moved to the
// wiki's difficulty factors (Wanderer 130 -> 120, Gentle 115 -> 120,
// Settled 105 -> 100).
var legacySpeed = [...]struct {
	p   difficulty.Preset
	old int
}{{difficulty.Wanderer, 130}, {difficulty.Gentle, 115}, {difficulty.Settled, 105}}

// MigrateDifficulty is s from a save of before RSCH-1: a preset whose
// research speed was moved reads the preset again; custom settings stay.
func MigrateDifficulty(s difficulty.Settings) difficulty.Settings {
	for _, l := range legacySpeed {
		was := difficulty.Of(l.p)
		was.ResearchSpeed = l.old
		if s == was {
			return difficulty.Of(l.p)
		}
	}
	return s
}

// ColdList is the benches in the bad temperature band, sorted (saved).
func (l *Lab) ColdList() []entity.EntityID {
	var out []entity.EntityID
	for id := range l.Cold {
		out = append(out, id)
	}
	slices.Sort(out)
	return out
}

// SetCold restores ColdList's output.
func (l *Lab) SetCold(ids []entity.EntityID) {
	l.Cold = nil
	for _, id := range ids {
		if l.Cold == nil {
			l.Cold = map[entity.EntityID]bool{}
		}
		l.Cold[id] = true
	}
}
