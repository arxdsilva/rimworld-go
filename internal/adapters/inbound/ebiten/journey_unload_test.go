package ebiten

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/inspector"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// TestJourneyGearUnload (CSE-038): the Gear tab's Unload button next to the
// wielded weapon sends MarkUnload, and the colonist puts the weapon down.
func TestJourneyGearUnload(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}))
	pawn := ordColonists(j)[0]
	j.g.Arm(entity.EntityID(pawn), item.Knife)
	j.update(1)
	if w := j.pawn(pawn).Weapon; w == "fists" || w == "" {
		t.Fatalf("fixture: the colonist holds %q", w)
	}
	ordPick(j, 0)
	j.clickID("inspector.gear")
	j.update(2)
	j.draw(2)
	j.reset()
	if !j.clickID(inspector.UnloadID("weapon", 0)) {
		t.Fatal("the Unload button is disabled")
	}
	c, ok := j.lastCmd(command.MarkUnload)
	if !ok || c.Pawn != pawn || c.Mode != 2 || !c.Enabled {
		t.Fatalf("command %+v (sent %v), want MarkUnload mode 2 (weapon) on %d", c, ok, pawn)
	}
	j.tick(120)
	if w := j.pawn(pawn).Weapon; w != "fists" {
		t.Fatalf("the colonist still holds %q after 120 ticks", w)
	}
}
