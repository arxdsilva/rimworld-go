package ebiten

import (
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/platform/input"
)

// TestJourneyCombatOverlay (DEF-1): Shift+O toggles the cover overlay, and
// a drafted, armed colonist selected with a raider under the cursor draws
// the range ring, the line of sight and the hit-chance card.
func TestJourneyCombatOverlay(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}))
	cover := &j.a.renderer.overlays().Combat.Cover
	j.key("O", input.ModShift)
	if !cover.On {
		t.Fatal("Shift+O did not show the cover overlay")
	}
	j.key("O", input.ModShift)
	if cover.On {
		t.Fatal("Shift+O again did not hide the cover overlay")
	}
	j.key("O", input.ModShift)

	shooter := ordColonists(j)[0]
	j.g.Arm(entity.EntityID(shooter), item.Rifle)
	ordPick(j, 0)
	j.key("R", 0)
	j.g.RaidArmed(item.Rifle, item.Rifle)
	j.update(2)
	var rx, ry int
	for _, p := range j.g.Snapshot() {
		if p.Hostile {
			rx, ry = p.X, p.Y
			break
		}
	}
	j.lookAt(rx, ry)
	j.f.MoveTo(j.a, rx, ry)
	j.update(1)
	j.draw(2) // range ring, sight line and card draw without fault
}
