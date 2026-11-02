package books

import (
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/research"
)

// TestShareIsExactOverTheHour: an hour's ticks add up to the hourly rate
// with no fraction lost, at any rate.
func TestShareIsExactOverTheHour(t *testing.T) {
	t.Parallel()
	for _, rate := range []int{1, 12, 250, 312, 3600, 9000} {
		sum := 0
		for e := 1; e <= ReadTicks; e++ {
			sum += share(rate, e)
		}
		if sum != rate {
			t.Errorf("rate %d: an hour gives %d", rate, sum)
		}
	}
}

// TestMintAndRaidLoot: only books are minted, once; raid loot is a book
// about RaidBookPct of the time.
func TestMintAndRaidLoot(t *testing.T) {
	t.Parallel()
	done := func(research.Project) bool { return false }
	st := item.Stack{Kind: item.Wood, Qty: 1}
	Mint(&st, 1, 2, done)
	if st.Book != 0 || st.Quality != 0 {
		t.Fatalf("wood minted %+v", st)
	}
	st = item.Stack{Kind: item.Schematic, Qty: 1}
	Mint(&st, 1, 2, done)
	first := st
	Mint(&st, 9, 9, done)
	if st.Book == 0 || st != first || Title(st) == "" {
		t.Fatalf("schematic %+v (first %+v)", st, first)
	}
	n := 0
	for id := uint64(0); id < 5000; id++ {
		if k, ok := RaidLoot(3, id); ok {
			if !k.IsBook() {
				t.Fatalf("raid loot %s", k)
			}
			n++
		}
	}
	if n < 5000*RaidBookPct/200 || n > 5000*RaidBookPct*2/100 {
		t.Fatalf("raid books %d of 5000", n)
	}
	if !IsBookcase(construction.SmallBookcase) || IsBookcase(construction.Shelf) {
		t.Fatal("bookcase kinds")
	}
}
