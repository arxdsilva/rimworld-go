package features

import (
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/pkg/randx"
)

// ruinBookPct is the chance a shed holds a book, a novel twice as often
// as a textbook or a schematic (A7; original tuning: the wiki names
// traders and quest rewards, and this game's ruins stand in for the
// quests it does not have).
const ruinBookPct = 35

var ruinBooks = []item.Kind{item.Novel, item.Novel, item.Textbook, item.Schematic}

// placeBooks leaves a book on a floored cell of the sheds placed from
// index from on, a cell no loot, urn, screen or telescope takes, drawn
// from its own stream so everything else in the ruins is unchanged. The
// game mints its quality and title when it spawns the stack.
func placeBooks(c *mapgen.Ctx, rn *randx.Rand, from int) {
	for i := from; i < len(c.R.Ruins); i++ {
		ru := &c.R.Ruins[i]
		if ru.Kind != mapgen.Shed || len(ru.Floors) == 0 || rn.IntN(100) >= ruinBookPct {
			continue
		}
		kind := ruinBooks[rn.IntN(len(ruinBooks))]
		var free [][2]int
		for _, f := range ru.Floors {
			if !hasLoot(ru, f.X, f.Y) && !hasUrn(ru, f.X, f.Y) && !underAntique(ru, f.X, f.Y) {
				free = append(free, [2]int{f.X, f.Y})
			}
		}
		if len(free) == 0 {
			continue
		}
		p := free[rn.IntN(len(free))]
		ru.Books = append(ru.Books, mapgen.Loot{X: p[0], Y: p[1], Kind: kind, Qty: 1})
	}
}

// underAntique reports (x, y) under a big screen (three cells east of
// its anchor) or a telescope (two cells south).
func underAntique(ru *mapgen.Ruin, x, y int) bool {
	for _, s := range ru.Screens {
		if y == s.Y && x >= s.X && x <= s.X+2 {
			return true
		}
	}
	for _, s := range ru.Scopes {
		if x == s.X && y >= s.Y && y <= s.Y+1 {
			return true
		}
	}
	return false
}
