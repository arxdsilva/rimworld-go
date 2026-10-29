package ebiten

import (
	"rimworld-go/internal/adapters/inbound/ebiten/architect"
	"testing"
)

// TestGlobalToggleIDsUnique: global toggle ids are unique, and the power
// overlay's id (PLAY-021) belongs to the power toggle only.
func TestGlobalToggleIDsUnique(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	for _, tg := range globalToggleList {
		if seen[tg.id] {
			t.Errorf("toggle id %s twice", tg.id)
		}
		seen[tg.id] = true
		if tg.id == architect.PowerButtonID && tg.icon != "overlay_power" {
			t.Errorf("%s is the power overlay's id, not %s's", architect.PowerButtonID, tg.icon)
		}
	}
}
