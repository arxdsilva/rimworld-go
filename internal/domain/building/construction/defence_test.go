package construction

import (
	"testing"

	"rimworld-go/internal/domain/economy/item"
)

// TestDefenceHP (DEF-3): the wiki's hit points, steel-built kinds through
// steel's 140 %. wiki: Barricade, Autodoor, Autocannon turret, Uranium
// slug turret, Mortar
func TestDefenceHP(t *testing.T) {
	t.Parallel()
	if MaxHP(Barricade, item.Wood) != MaxHP(Sandbags, item.Wood) || MaxHP(Autodoor, item.Steel) != MaxHP(Door, item.Steel) {
		t.Fatal("barricade as sandbags, autodoor as door")
	}
	if got := MaxHP(Autocannon, item.Steel); got != 380 {
		t.Fatalf("autocannon %d", got)
	}
	if got := MaxHP(UraniumSlugTurret, item.Steel); got != 380 {
		t.Fatalf("slug turret %d", got)
	}
	if got := MaxHP(Mortar, item.Steel); got != 180 {
		t.Fatalf("mortar %d", got)
	}
	if !Door.IsDoor() || !Autodoor.IsDoor() || Wall.IsDoor() || FenceGate.IsDoor() {
		t.Fatal("IsDoor")
	}
}
