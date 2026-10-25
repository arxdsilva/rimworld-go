package features

import "rimworld-go/internal/domain/economy/item"

// lootEntry is one loot kind and its stack-size range (inclusive).
type lootEntry struct {
	kind   item.Kind
	lo, hi int
}

// lootTable is every stack a ruin may hold.
var lootTable = []lootEntry{
	{item.Steel, 10, 30},
	{item.Components, 1, 3},
	{item.Silver, 20, 60},
	{item.Cloth, 5, 15},
}
