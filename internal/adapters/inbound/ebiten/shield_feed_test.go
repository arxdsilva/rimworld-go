package ebiten

import (
	"testing"

	"rimworld-go/internal/application/game"
)

// TestShieldStatesNoAllocs: the cue feed reuses its scratch; with no
// ShieldPct field yet (or with it) a steady frame allocates nothing.
func TestShieldStatesNoAllocs(t *testing.T) {
	pawns := make([]game.PawnView, 8)
	buf := shieldStates(nil, pawns)
	if n := testing.AllocsPerRun(20, func() { buf = shieldStates(buf[:0], pawns) }); n > 1 {
		t.Fatalf("allocs %v", n)
	}
}
