package game

import (
	"fmt"
	"math/rand/v2"
	"reflect"
	"sort"
	"testing"

	"rimworld-go/internal/application/game/postpone"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/sim/entity"
)

// postponedLinesRef is the pre-ENG-F postponedLines (a full scan and two
// sorts per colonist): postponedTops must give exactly its lines.
func (g *Game) postponedLinesRef(id entity.EntityID) []string {
	p := &g.postponed
	if p.Len() == 0 {
		return nil
	}
	now := uint64(g.clock.Now())
	stale := p.Rev != g.grid.Version()
	var es []postpone.Entry
	p.All(func(k postpone.Key, v postpone.Val) {
		if k.Pawn != id || now >= v.Until || v.Unreach && stale || g.postTargetGone(k) {
			return
		}
		es = append(es, postpone.Entry{Key: k, Until: v.Until, Unreach: v.Unreach})
	})
	postpone.SortEntries(es)
	sort.SliceStable(es, func(i, j int) bool { return es[i].Until < es[j].Until })
	var out []string
	for _, e := range es {
		if len(out) == postpone.ViewLines {
			break
		}
		if e.Unreach {
			out = append(out, g.postLabel(e.Key)+": can't reach")
			continue
		}
		secs := (e.Until - now + 59) / 60
		out = append(out, fmt.Sprintf("%s: blocked, retry in %d s", g.postLabel(e.Key), secs))
	}
	return out
}

func TestPostponedTopsMatchFullSort(t *testing.T) {
	t.Parallel()
	g := NewColony(Config{Seed: 4, Width: 48, Height: 40})
	g.Step(10)
	now := uint64(g.clock.Now())
	rng := rand.New(rand.NewPCG(1, 2))
	kinds := []job.Kind{job.Mine, job.Haul, job.Construct, job.Clean, job.Cut}
	for round := 0; round < 50; round++ {
		g.postponed.Load(nil)
		n := rng.IntN(60)
		for i := 0; i < n; i++ {
			k := postpone.Key{
				Pawn: entity.EntityID(1 + rng.IntN(5)),
				Kind: kinds[rng.IntN(len(kinds))],
				X:    rng.IntN(4), Y: rng.IntN(4),
			}
			// few distinct retry times, so ties are common; some expired
			g.postponed.Put(k, postpone.Val{Until: now - 2 + uint64(rng.IntN(6))*30, Unreach: rng.IntN(3) == 0})
		}
		tops := g.postponedTops(nil)
		for id := entity.EntityID(0); id <= 6; id++ {
			got, want := g.postponedLinesIn(tops, id), g.postponedLinesRef(id)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("round %d pawn %d: got %q, want %q", round, id, got, want)
			}
		}
	}
}
