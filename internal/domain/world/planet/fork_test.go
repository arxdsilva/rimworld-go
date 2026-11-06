package planet

import (
	"sync"
	"testing"
)

// TestForksRunHopsConcurrently: two forks of one planet run Hops on separate
// goroutines (the race detector watches the scratch) and get the parent's
// answers, and the parent's own scratch is left untouched.
func TestForksRunHopsConcurrently(t *testing.T) {
	p := world(t, "fork", 30)
	a, b := landPair(t, p)
	want := p.Hops(a, b, 30)
	p.ext.hops = HopScratch{}
	var wg sync.WaitGroup
	got := make([][]int, 2)
	for i := range got {
		f := p.Fork()
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 20 {
				got[i] = append(got[i], f.Hops(a, b, 30), f.Hops(b, a, 30))
			}
		}()
	}
	wg.Wait()
	for i, g := range got {
		for _, h := range g {
			if h != want {
				t.Fatalf("fork %d: hops %d, the planet says %d", i, h, want)
			}
		}
	}
	if p.ext.hops.mark != nil || p.ext.hops.gen != 0 {
		t.Fatal("a fork wrote the parent's hop scratch")
	}
}
