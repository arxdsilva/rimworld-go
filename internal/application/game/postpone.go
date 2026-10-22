package game

import (
	"sort"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/sim/entity"
)

// Postponed work (SIM-027): a job that failed because the way was blocked
// or the target was out of reach is put off for a while instead of being
// picked again at once, so the colonist gets on with other work.

// postKey names one piece of postponed work: a pawn, a job kind and its
// target (an entity, or a tile when Target is 0).
type postKey struct {
	Pawn   entity.EntityID
	Kind   job.Kind
	Target entity.EntityID
	X, Y   int
}

// postEntry is one saved postponement.
type postEntry struct {
	Key   postKey
	Until uint64
}

// postponedFor reports whether pawn id has put this work off.
func (g *Game) postponedFor(id entity.EntityID, kind job.Kind, target entity.EntityID, x, y int) bool {
	until, ok := g.postponed[postKey{id, kind, target, x, y}]
	return ok && uint64(g.clock.Now()) < until
}

func (g *Game) exportPostponed() []postEntry {
	out := make([]postEntry, 0, len(g.postponed))
	for k, v := range g.postponed {
		out = append(out, postEntry{k, v})
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i].Key, out[j].Key
		if a.Pawn != b.Pawn {
			return a.Pawn < b.Pawn
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.Target != b.Target {
			return a.Target < b.Target
		}
		if a.X != b.X {
			return a.X < b.X
		}
		return a.Y < b.Y
	})
	return out
}

func (g *Game) importPostponed(in []postEntry) {
	g.postponed = make(map[postKey]uint64, len(in))
	for _, e := range in {
		g.postponed[e.Key] = e.Until
	}
}
