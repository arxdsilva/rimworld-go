package mapgen

import (
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/pkg/randx"
)

// Faction settlements (MAP-05, wiki: Faction base). The layout is our
// own: a walled compound near the map's centre, a row of rooms along its
// north side (beds, and a workbench in the first), each opening onto a
// yard through a door; two gates in the outer wall (south, and east or
// west); a store in the yard holding the settlement's goods (silver, food,
// the faction's weapons, components for an industrial faction); sentry
// turrets in the yard's corners for an industrial faction. Tribes build
// in wood; industrial factions raise stone-block outer walls with steel
// rooms and doors. Every size, count and amount is original tuning.

// Piece is one building of a compound, laid as finished.
type Piece struct {
	Kind   construction.Kind
	Stuff  item.Kind
	Facing construction.Facing
	X, Y   int
}

// Compound is a faction settlement's layout on a map.
type Compound struct {
	// Bounds (inclusive) of the outer wall; Centre is the yard's middle.
	X0, Y0, X1, Y1 int
	Centre         Point
	// Pieces are its buildings: floors first, then walls, doors and
	// furniture, then turrets.
	Pieces []Piece
	// Loot is the goods in its store.
	Loot []Loot
	// Posts are where its defenders stand: inside each room door, inside
	// each gate and in the store (at least one).
	Posts []Point
}

// CompoundConfig is one settlement to lay out.
type CompoundConfig struct {
	// W, H is the map size in cells (the compound keeps a margin from the
	// edges).
	W, H int
	Tech faction.Tech
	Seed uint64
}

// Layout numbers (original tuning).
const (
	roomStep   = 6  // a room's width with one wall: 5 cells inside
	roomDepth  = 6  // the room row's depth with its south wall
	yardDepth  = 8  // the yard's depth inside the walls (tribal)
	storeW     = 5  // the store's footprint in the yard
	storeH     = 3  // ...
	jitter     = 8  // the compound's centre wanders this far from the map's
	edgeMargin = 12 // cells kept clear between the compound and the edge
	// settleSalt keeps the layout's stream apart from the map generator's.
	settleSalt uint64 = 0x5e771e3e47
)

// roomsFor is how many rooms a faction of tech builds.
func roomsFor(tech faction.Tech) int {
	if tech >= faction.Industrial {
		return 4
	}
	return 3
}

// Settlement lays out a faction settlement for cfg: the same compound for
// the same seed, tech and size.
func Settlement(cfg CompoundConfig) Compound {
	rng := randx.New(cfg.Seed ^ settleSalt)
	ind := cfg.Tech >= faction.Industrial
	n := roomsFor(cfg.Tech)
	w := n*roomStep + 1
	h := roomDepth + yardDepth + 1
	if ind {
		h += 2
	}
	cx := min(max(cfg.W/2+rng.IntN(2*jitter+1)-jitter, edgeMargin+w/2), cfg.W-edgeMargin-w/2-1)
	cy := min(max(cfg.H/2+rng.IntN(2*jitter+1)-jitter, edgeMargin+h/2), cfg.H-edgeMargin-h/2-1)
	c := Compound{X0: cx - w/2, Y0: cy - h/2}
	c.X1, c.Y1 = c.X0+w-1, c.Y0+h-1
	outer, inner, door := item.Wood, item.Wood, item.Wood
	if ind {
		outer = item.SandstoneBlocks + item.Kind(rng.IntN(int(item.MarbleBlocks-item.SandstoneBlocks)+1))
		inner, door = item.Steel, item.Steel
	}
	ry := c.Y0 + roomDepth // the rooms' south wall
	c.Centre = Point{(c.X0 + c.X1) / 2, (ry + c.Y1) / 2}
	gateS := Point{c.Centre.X, c.Y1}
	gateSide := Point{c.X0, c.Centre.Y}
	storeX := c.X1 - storeW - 1 // the store lies away from the side gate
	if rng.IntN(2) == 1 {
		gateSide.X, storeX = c.X1, c.X0+2
	}
	c.rooms(n, ry, inner, door, ind)
	for x := c.X0; x <= c.X1; x++ {
		for _, y := range [2]int{c.Y0, c.Y1} {
			c.wallOrGate(x, y, outer, door, gateS)
		}
	}
	for y := c.Y0 + 1; y < c.Y1; y++ {
		for _, x := range [2]int{c.X0, c.X1} {
			c.wallOrGate(x, y, outer, door, gateSide)
		}
	}
	storeY := c.Y1 - storeH - 1
	c.store(storeX, storeY, ind, rng)
	if ind { // sentry turrets in the yard's corners
		for _, x := range [2]int{c.X0 + 1, c.X1 - 1} {
			c.Pieces = append(c.Pieces, Piece{Kind: construction.SentryTurret, Stuff: item.Steel, X: x, Y: ry + 1})
		}
	}
	gx := gateSide.X + 1
	if gateSide.X == c.X1 {
		gx = gateSide.X - 1
	}
	c.Posts = append(c.Posts, Point{gateS.X, gateS.Y - 1}, Point{gx, gateSide.Y}, Point{storeX + storeW/2, storeY - 1}, c.Centre)
	return c
}

// rooms lays the n rooms of the north row (south wall at ry): floors,
// walls, a door each, two beds each and the workbench in the first.
func (c *Compound) rooms(n, ry int, wall, door item.Kind, ind bool) {
	bench, benchStuff := construction.CraftingSpot, item.Wood
	if ind {
		bench, benchStuff = construction.MachiningTable, item.Steel
	}
	for i := 0; i < n; i++ {
		rx := c.X0 + i*roomStep
		for y := c.Y0 + 1; y < ry; y++ {
			for x := rx + 1; x < rx+roomStep; x++ {
				c.Pieces = append(c.Pieces, Piece{Kind: construction.FloorFor(item.Wood), Stuff: item.Wood, X: x, Y: y})
			}
		}
	}
	for i := 0; i < n; i++ {
		rx := c.X0 + i*roomStep
		if i > 0 { // the wall between this room and the last
			for y := c.Y0 + 1; y < ry; y++ {
				c.Pieces = append(c.Pieces, Piece{Kind: construction.Wall, Stuff: wall, X: rx, Y: y})
			}
		}
		for x := max(rx, c.X0+1); x < rx+roomStep; x++ { // the outer wall closes its ends
			k, st := construction.Wall, wall
			if x == rx+roomStep/2 {
				k, st = construction.Door, door
			}
			c.Pieces = append(c.Pieces, Piece{Kind: k, Stuff: st, X: x, Y: ry})
		}
		if i == 0 {
			c.Pieces = append(c.Pieces, Piece{Kind: bench, Stuff: benchStuff, X: rx + 2, Y: c.Y0 + 1})
		}
		for _, x := range [2]int{rx + 1, rx + roomStep - 1} {
			c.Pieces = append(c.Pieces, Piece{Kind: construction.Bed, Stuff: item.Wood, X: x, Y: ry - 2})
		}
		c.Posts = append(c.Posts, Point{rx + roomStep/2, ry + 1})
	}
}

// wallOrGate lays the outer wall piece at (x, y): a door on the gate.
func (c *Compound) wallOrGate(x, y int, wall, door item.Kind, gate Point) {
	if x == gate.X && y == gate.Y {
		c.Pieces = append(c.Pieces, Piece{Kind: construction.Door, Stuff: door, X: x, Y: y})
		return
	}
	c.Pieces = append(c.Pieces, Piece{Kind: construction.Wall, Stuff: wall, X: x, Y: y})
}

// Loot amounts (original tuning): silver, food and components ranges; how
// many weapons lie in the store.
const (
	silverTribal, silverTribalRoll = 80, 81   // 80..160
	silverIndust, silverIndustRoll = 200, 201 // 200..400
	foodQty, foodRoll              = 30, 31   // 30..60 raw or preserved
	mealQty, mealRoll              = 10, 11   // 10..20 meals
	componentQty, componentRoll    = 6, 7     // 6..12
	storeWeapons                   = 2
)

// store fills the store at (x, y) (storeW x storeH, one stack a cell).
func (c *Compound) store(x, y int, ind bool, rng *randx.Rand) {
	weapons := []item.Kind{item.Spear, item.ShortBow, item.Club, item.RecurveBow}
	goods := []Loot{{Kind: item.Silver, Qty: silverTribal + rng.IntN(silverTribalRoll)},
		{Kind: item.Pemmican, Qty: foodQty + rng.IntN(foodRoll)}, {Kind: item.Corn, Qty: foodQty + rng.IntN(foodRoll)}}
	if ind {
		weapons = []item.Kind{item.Rifle, item.Autopistol, item.PumpShotgun, item.Revolver}
		goods = []Loot{{Kind: item.Silver, Qty: silverIndust + rng.IntN(silverIndustRoll)},
			{Kind: item.Meal, Qty: mealQty + rng.IntN(mealRoll)}, {Kind: item.Components, Qty: componentQty + rng.IntN(componentRoll)},
			{Kind: item.Rice, Qty: foodQty + rng.IntN(foodRoll)}}
	}
	for i := 0; i < storeWeapons; i++ {
		goods = append(goods, Loot{Kind: weapons[rng.IntN(len(weapons))], Qty: 1})
	}
	for i, l := range goods {
		l.X, l.Y = x+i%storeW, y+i/storeW
		c.Loot = append(c.Loot, l)
	}
}
