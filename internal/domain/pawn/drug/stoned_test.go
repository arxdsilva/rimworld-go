package drug

import (
	"testing"

	"rimworld-go/internal/domain/economy/item"
)

// TestStoned (FOOD-6): only the ashleaf tea high (the smokeleaf
// stand-in) counts, and only while it lasts.
func TestStoned(t *testing.T) {
	t.Parallel()
	var none *State
	if none.Stoned(0) {
		t.Fatal("no state is not stoned")
	}
	var s State
	s.Ingest(item.SparkTablet, nil, 100)
	if s.Stoned(100) {
		t.Fatal("a spark tablet is no smokeleaf high")
	}
	s.Ingest(item.AshleafTea, nil, 100)
	until := 100 + uint64(defs[index(item.AshleafTea)].HighHours)*hourTicks
	if !s.Stoned(100) || !s.Stoned(until-1) || s.Stoned(until) {
		t.Fatal("stoned while the ashleaf high lasts")
	}
}
