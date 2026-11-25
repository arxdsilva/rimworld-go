// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package social

import (
	"math/rand/v2"
	"reflect"
	"testing"
)

// AppendKinBetween over a per-person Ancestry must equal the one-shot KinOf for
// every ordered pair, and the unrelated-pair shortcut must agree with the
// full blood and step walks.
func TestAppendKinBetweenEqualsKinOfEveryPair(t *testing.T) {
	graphs := []fam{sample()}
	r := rand.New(rand.NewPCG(7, 11))
	for range 20 {
		g := fam{par: map[ID][2]ID{}, sp: map[ID][]ID{}}
		for id := ID(1); id <= 24; id++ {
			if r.IntN(3) != 0 && id > 4 {
				g.par[id] = [2]ID{ID(1 + r.IntN(int(id-1))), ID(1 + r.IntN(int(id-1)))}
			}
			if r.IntN(4) == 0 {
				o := ID(1 + r.IntN(24))
				g.sp[id] = append(g.sp[id], o)
				g.sp[o] = append(g.sp[o], id)
			}
		}
		graphs = append(graphs, g)
	}
	for gi, g := range graphs {
		anc := make([]Ancestry, 26)
		for id := range anc {
			anc[id] = AncestryOf(ID(id), g)
		}
		for a := ID(0); a < 26; a++ {
			for b := ID(0); b < 26; b++ {
				want := KinOf(a, b, g)
				if got := AppendKinBetween(nil, &anc[a], &anc[b], g); !reflect.DeepEqual(got, want) {
					t.Fatalf("graph %d %d->%d: AppendKinBetween=%v KinOf=%v", gi, a, b, got, want)
				}
				if a == 0 || b == 0 || a == b || anc[a].Related() || anc[b].Related() {
					continue
				}
				if _, ok := bloodKin(a, b, anc[a].anc, anc[b].anc); ok || len(stepKin(nil, anc[a], anc[b], g)) != 0 {
					t.Fatalf("graph %d: unrelated pair %d,%d has kin", gi, a, b)
				}
			}
		}
	}
}

var kinLookupBenchmarkSink int

// One observation reads each person's family once, then all ordered pairs.
// The fresh baseline traverses both family trees for every pair.
func BenchmarkKinLookupRoster(b *testing.B) {
	graph := sample()
	ids := []ID{1, 2, 3, 4, 5, 6, 8, 9, 10, 11, 12, 13, 14, 59, 60, 61, 70, 71, 80, 81, 82, 90, 100, 101, 102}
	b.Run("fresh", func(b *testing.B) {
		var total int
		for range b.N {
			for _, from := range ids {
				for _, to := range ids {
					total += len(KinOf(from, to, graph))
				}
			}
		}
		kinLookupBenchmarkSink = total
	})
	b.Run("scoped", func(b *testing.B) {
		anc := make([]Ancestry, len(ids))
		var total int
		for range b.N {
			for i, id := range ids {
				anc[i] = AncestryOf(id, graph)
			}
			for i := range ids {
				for j := range ids {
					total += len(AppendKinBetween(nil, &anc[i], &anc[j], graph))
				}
			}
		}
		kinLookupBenchmarkSink = total
	})
}
