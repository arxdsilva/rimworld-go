package world

import (
	"cmp"
	"math/rand/v2"
	"slices"
	"testing"
)

// randomQueue is a queue like a frame's: rows and columns on a few cells,
// with the forbidden (+0.001), creature (+0.01) and bed (+0.02) nudges, so
// many keys tie.
func randomQueue(r *rand.Rand, n int) []ArtDraw {
	nudges := [...]float64{0, 0, 0, 0.001, 0.01, 0.02}
	q := make([]ArtDraw, n)
	for i := range q {
		q[i] = ArtDraw{
			Row:  float64(r.IntN(12)) + nudges[r.IntN(len(nudges))],
			Col:  float64(r.IntN(6)),
			Kind: ObjKind(r.IntN(9)),
			I:    int32(i),
		}
	}
	return q
}

// stableOrder is the order before GAR-161: a stable sort on (Row, Col).
func stableOrder(q []ArtDraw) []ArtDraw {
	want := slices.Clone(q)
	slices.SortStableFunc(want, func(a, b ArtDraw) int {
		if c := cmp.Compare(a.Row, b.Row); c != 0 {
			return c
		}
		return cmp.Compare(a.Col, b.Col)
	})
	return want
}

func TestSortDrawsMatchesStable(t *testing.T) {
	r := rand.New(rand.NewPCG(7, 161))
	var s drawSorter
	for _, n := range []int{0, 1, 2, 3, 17, 200, 3000} {
		for round := range 20 {
			q := randomQueue(r, n)
			if round%5 == 0 { // an already sorted queue takes the fast path
				q = stableOrder(q)
			}
			want := stableOrder(q)
			s.sort(q)
			for i := range q {
				if q[i].I != want[i].I || q[i].Row != want[i].Row || q[i].Col != want[i].Col {
					t.Fatalf("n=%d round %d: draw %d is record %d at (%v,%v), want record %d at (%v,%v)",
						n, round, i, q[i].I, q[i].Row, q[i].Col, want[i].I, want[i].Row, want[i].Col)
				}
			}
		}
	}
}

func TestSortDrawsReusesItsBuffers(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	base := randomQueue(r, 2000)
	q := slices.Clone(base)
	var s drawSorter
	s.sort(q) // grows the buffers once
	allocs := testing.AllocsPerRun(20, func() {
		copy(q, base)
		s.sort(q)
	})
	if allocs != 0 {
		t.Fatalf("sort allocates %.1f times a frame, want 0", allocs)
	}
	for i := range s.tmp[:cap(s.tmp)] {
		if s.tmp[:cap(s.tmp)][i].Fn != nil {
			t.Fatal("the sorter keeps a frame's closures alive")
		}
	}
}

// BenchmarkSortDraws compares the key sort with the stable sort it
// replaced, on a queue the size of a busy frame's.
func BenchmarkSortDraws(b *testing.B) {
	r := rand.New(rand.NewPCG(3, 4))
	base := randomQueue(r, 4000)
	q := slices.Clone(base)
	b.Run("keys", func(b *testing.B) {
		var s drawSorter
		b.ReportAllocs()
		for b.Loop() {
			copy(q, base)
			s.sort(q)
		}
	})
	b.Run("stable", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			copy(q, base)
			slices.SortStableFunc(q, func(a, b ArtDraw) int {
				if c := cmp.Compare(a.Row, b.Row); c != 0 {
					return c
				}
				return cmp.Compare(a.Col, b.Col)
			})
		}
	})
}
