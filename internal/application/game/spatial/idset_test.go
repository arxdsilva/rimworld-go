package spatial

import "testing"

// IDSet answers as a map would through adds and resets.
func TestIDSetMatchesAMap(t *testing.T) {
	var s IDSet[uint64]
	x := uint64(12345)
	next := func(n uint64) uint64 { x = x*6364136223846793005 + 1442695040888963407; return (x >> 33) % n }
	for round := range 50 {
		want := map[uint64]bool{}
		for range next(40) {
			id := next(3000)
			s.Add(id)
			want[id] = true
		}
		for id := uint64(0); id < 3100; id++ {
			if s.Has(id) != want[id] {
				t.Fatalf("round %d: Has(%d) = %v", round, id, s.Has(id))
			}
		}
		s.Reset()
	}
	if s.Has(0) || s.Has(1<<40) {
		t.Fatal("a reset set holds ids")
	}
}

func TestIDSetResetAllocatesNothing(t *testing.T) {
	var s IDSet[uint32]
	for i := range uint32(200) {
		s.Add(i * 7)
	}
	s.Reset()
	if a := testing.AllocsPerRun(50, func() {
		for i := range uint32(200) {
			s.Add(i * 7)
		}
		s.Reset()
	}); a != 0 {
		t.Fatalf("%v allocs", a)
	}
}
