package recipe

import (
	"testing"

	"rimworld-go/internal/domain/building/construction"
)

// What All and For return is clipped (cap == len), so a caller's append
// copies instead of writing into the frozen book.
func TestBookSlicesDoNotAlias(t *testing.T) {
	b := Core()
	if all := b.All(); cap(all) != len(all) {
		t.Fatalf("All: cap %d, len %d", cap(all), len(all))
	}
	for k := construction.Kind(0); k.Valid(); k++ {
		if got := b.For(k); cap(got) != len(got) {
			t.Fatalf("For(%v): cap %d, len %d", k, cap(got), len(got))
		}
	}
}
