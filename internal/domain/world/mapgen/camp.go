package mapgen

import (
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/pkg/randx"
)

// Hostile camps (QST-060..064, QST-105). The layout is our own: one walled
// yard, smaller than a settlement, near the map's centre; a row of beds
// against its north wall; a campfire (a table for an industrial faction)
// before them; the settlement's store of goods in the south-east corner;
// one gate in the south wall, on the centre column (the path the builder
// clears runs south from it). Posts stand inside the gate, in the yard, at
// the store and the beds, and just outside the walls (south, east and
// west), all within the defenders' leash of the centre. Tribes build in
// wood, industrial factions in steel. Every size and count is original
// tuning.

const (
	campW = 13 // the outer wall's width and height, walls included
	campH = 11
	// campSalt keeps the camp's stream apart from the map generator's and
	// a settlement's.
	campSalt uint64 = 0xca3b0f1e
)

// bedsFor is how many beds a camp of tech holds: one per settlement room.
func bedsFor(tech faction.Tech) int { return roomsFor(tech) }

// Camp lays out a hostile camp for cfg: the same compound for the same
// seed, tech and size.
func Camp(cfg CompoundConfig) Compound {
	rng := randx.New(cfg.Seed ^ campSalt)
	ind := cfg.Tech >= faction.Industrial
	cx := min(max(cfg.W/2+rng.IntN(2*jitter+1)-jitter, edgeMargin+campW/2), cfg.W-edgeMargin-campW/2-1)
	cy := min(max(cfg.H/2+rng.IntN(2*jitter+1)-jitter, edgeMargin+campH/2), cfg.H-edgeMargin-campH/2-1)
	c := Compound{X0: cx - campW/2, Y0: cy - campH/2, Centre: Point{cx, cy}}
	c.X1, c.Y1 = c.X0+campW-1, c.Y0+campH-1
	wall := item.Wood
	if ind {
		wall = item.Steel
	}
	gate := Point{cx, c.Y1}
	for x := c.X0; x <= c.X1; x++ {
		for _, y := range [2]int{c.Y0, c.Y1} {
			c.wallOrGate(x, y, wall, wall, gate)
		}
	}
	for y := c.Y0 + 1; y < c.Y1; y++ {
		for _, x := range [2]int{c.X0, c.X1} {
			c.wallOrGate(x, y, wall, wall, gate)
		}
	}
	beds := [...]int{c.X0 + 2, c.X1 - 2, c.X0 + 4, c.X1 - 4}
	for _, x := range beds[:bedsFor(cfg.Tech)] {
		c.Pieces = append(c.Pieces, Piece{Kind: construction.Bed, Stuff: item.Wood, X: x, Y: c.Y0 + 1})
	}
	if ind {
		c.Pieces = append(c.Pieces, Piece{Kind: construction.Table, Stuff: item.Steel, X: cx, Y: c.Y0 + 2})
	} else {
		c.Pieces = append(c.Pieces, Piece{Kind: construction.Campfire, Stuff: item.Wood, X: cx, Y: c.Y0 + 3})
	}
	storeX, storeY := c.X1-storeW, c.Y1-storeH
	c.store(storeX, storeY, ind, rng)
	c.Posts = append(c.Posts,
		Point{gate.X, gate.Y - 1}, c.Centre, Point{storeX + storeW/2, storeY - 1}, Point{c.X0 + 3, c.Y0 + 4},
		Point{gate.X, gate.Y + 1}, Point{c.X0 - 1, cy}, Point{c.X1 + 1, cy})
	return c
}
