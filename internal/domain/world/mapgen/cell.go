package mapgen

import (
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
)

// The prisoner rescue's site (QST-066, QST-067, decision 13): a faction
// settlement with a small walled cell in its yard. The cell is a closed
// room of cellInner x cellInner cells whose only way in is one door on its
// north side; a bed and a meal lie inside for the prisoner. The layout is
// our own; every size is original tuning.
const (
	// cellInner is the cell's inside width and depth; cellOuter with walls.
	cellInner = 3
	cellOuter = cellInner + 2
	// cellRow is the cell's north wall below the rooms' south wall: one
	// clear yard row between them keeps the room doors and their posts free.
	cellRow = 2
	// cellMeals is the meal stack in the cell.
	cellMeals = 2
)

// Cell is the prisoner's cell inside a compound.
type Cell struct {
	// X0, Y0, X1, Y1 bound the cell's walls (inclusive).
	X0, Y0, X1, Y1 int
	// Door is the cell's one door, in its north wall.
	Door Point
	// Inside is where the prisoner stands: inside the cell, clear of the
	// bed and the meal.
	Inside Point
}

// CellCamp lays the settlement for cfg (Settlement) with a prisoner's cell
// in its yard, and says where the cell is. The same compound and cell for
// the same seed, tech and size. The yard's centre post moves to the cell's
// door, where its guard stands; the other posts are the settlement's own.
func CellCamp(cfg CompoundConfig) (Compound, Cell) {
	c := Settlement(cfg)
	ry := c.Y0 + roomDepth // the rooms' south wall
	stuff := item.Wood
	if cfg.Tech >= faction.Industrial {
		stuff = item.Steel
	}
	cx := c.Centre.X
	cell := Cell{X0: cx - cellOuter/2, Y0: ry + cellRow}
	cell.X1, cell.Y1 = cell.X0+cellOuter-1, cell.Y0+cellOuter-1
	cell.Door = Point{cx + 1, cell.Y0}
	cell.Inside = Point{cx + 1, cell.Y1 - 1}
	for x := cell.X0; x <= cell.X1; x++ {
		for _, y := range [2]int{cell.Y0, cell.Y1} {
			k := construction.Wall
			if x == cell.Door.X && y == cell.Door.Y {
				k = construction.Door
			}
			c.Pieces = append(c.Pieces, Piece{Kind: k, Stuff: stuff, X: x, Y: y})
		}
	}
	for y := cell.Y0 + 1; y < cell.Y1; y++ {
		for _, x := range [2]int{cell.X0, cell.X1} {
			c.Pieces = append(c.Pieces, Piece{Kind: construction.Wall, Stuff: stuff, X: x, Y: y})
		}
	}
	c.Pieces = append(c.Pieces, Piece{Kind: construction.Bed, Stuff: item.Wood, X: cell.X0 + 1, Y: cell.Y0 + 1})
	c.Loot = append(c.Loot, Loot{Kind: item.Meal, Qty: cellMeals, X: cell.X1 - 1, Y: cell.Y0 + 1})
	// The centre post would stand in the cell: its guard stands at the door.
	c.Posts[len(c.Posts)-1] = Point{cell.Door.X, cell.Door.Y - 1}
	return c, cell
}
