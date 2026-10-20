package game

import (
	"testing"

	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/economy/item"
)

// The dead are carried to graves and pyres, never shelved with the food.
func TestHumanCorpseAndBonesNeverStockpiled(t *testing.T) {
	all := zone.Stockpile{AllowFood: true, AllowWood: true, AllowStone: true}
	for _, k := range []item.Kind{item.CorpseHuman, item.Bones} {
		if allows(all, k) {
			t.Fatalf("%s allowed in a stockpile", k)
		}
	}
	if !allows(all, item.CorpseDustbuck) {
		t.Fatal("animal corpses still go to food stockpiles for butchering")
	}
}
