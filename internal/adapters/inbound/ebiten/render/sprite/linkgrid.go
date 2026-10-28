package sprite

import (
	"fmt"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/application/game"
)

// Linked rendering (ART-015, HEXAGONAL §21: presentation-only). Walls,
// doors, fences and natural rock autotile through link flags: a cell
// links to a neighbour when the neighbour's flags meet the cell's
// accepts. Walls and rock join each other; fences join walls one way;
// rock also links off the map edge; a cooler stands in a wall gap and
// walls join it like a door. The grid is rebuilt only when the read
// model's StructVer changes, so a frame never scans the map.
//
// Power conduits (PLAY-021) link on a layer of their own (power), so a
// conduit laid under a wall never joins the wall, nor the wall the
// conduit. A conduit, shown or hidden, joins the transmitters beside it
// (conduits and power switches) and the cells of any power device (a
// producer, consumer or battery): the art draws a pipe with stubs, and a
// stub reaching into the device beside it reads as the connection.

// LinkFlag is what a cell is, or joins, on the link layers.
type LinkFlag uint8

const (
	LinkMapEdge LinkFlag = 1 << iota
	LinkRock
	LinkWall
	LinkFence
	LinkConduit // the power layer: a transmitter (a conduit, shown or hidden, or a power switch)
	LinkDevice  // the power layer: any cell of a power device
)

// LinkCell is what a cell is (flags) and what it joins (accepts).
type LinkCell struct{ Flags, Accepts LinkFlag }

var (
	RockLink  = LinkCell{Flags: LinkRock | LinkMapEdge, Accepts: LinkRock}
	WallLink  = LinkCell{Flags: LinkWall | LinkRock, Accepts: LinkWall | LinkRock}
	FenceLink = LinkCell{Flags: LinkFence, Accepts: LinkFence | LinkWall}
	// ConduitLink is a conduit on the power layer.
	ConduitLink = LinkCell{Flags: LinkConduit, Accepts: LinkConduit | LinkDevice}
)

// Corner bits: clockwise from north-east, like the cardinal mask bits.
const (
	CornerNE = 1
	CornerSE = 2
	CornerSW = 4
	CornerNW = 8
)

// terrainLinkOf maps WorldView.Terrain codes to link cells (rock and
// smoothed rock; ore lies in rock and links like it).
var terrainLinkOf = func() [256]LinkCell {
	var out [256]LinkCell
	for i, n := range game.TerrainNames() {
		if IsRockWallName(n) {
			out[i] = RockLink
		}
	}
	return out
}()

// IsRockWallName reports a natural rock wall terrain (rough or smoothed).
func IsRockWallName(n string) bool { return n == "rock" || n == "rock_smooth" }

// IsRockWallCode is IsRockWallName for a WorldView.Terrain code.
func IsRockWallCode(c uint8) bool { return terrainLinkOf[c].Flags&LinkRock != 0 }

// Kinds names the building kinds that link, for the grid's owner to say
// (the kind names stay with it): the structure-layer cell of a building
// or frame kind (walls, doors, coolers, fences) and its power-layer cell
// (conduits, switches, devices). The grid calls a zero K.
type Kinds interface {
	Building(kind string) LinkCell
	Power(kind string) LinkCell
}

// powerFlags are a built building's power-layer flags: by kind, or a
// device by its view's PowerRole (the simulation's word).
func (g *LinkGrid[K]) powerFlags(b *game.BuildingView) LinkFlag {
	var k K
	if c := k.Power(b.Kind); c.Flags != 0 {
		return c.Flags
	}
	switch b.PowerRole {
	case "producer", "consumer", "storage":
		return LinkDevice
	}
	return 0
}

// LinkGrid is the per-cell link data of the map, cached on StructVer.
// StructVer is saved, so it can repeat across games: the grid lives on the
// Renderer, and every session (new or loaded) gets a new Renderer
// (App.startSession). Frames don't bump StructVer, so they stay in the
// per-frame planned set. K classifies the building kinds.
type LinkGrid[K Kinds] struct {
	w, h  int
	ver   uint64
	built bool
	cells []LinkCell
	// power is the power layer's flags per cell (conduits, switches and
	// every footprint cell of a device).
	power []LinkFlag
	// planned is the reusable per-frame set of frame and ghost cells.
	planned map[[2]int]LinkFlag
}

// Sync rebuilds the grid when it was never built, the map size changed or
// w.StructVer moved. A zero StructVer (a sim that publishes no version)
// rebuilds every call: correct, just not cached.
func (g *LinkGrid[K]) Sync(w game.WorldView, width, height int) {
	if g.built && w.StructVer != 0 && w.StructVer == g.ver && g.w == width && g.h == height {
		return
	}
	g.built, g.ver, g.w, g.h = true, w.StructVer, width, height
	n := width * height
	if cap(g.cells) < n {
		g.cells = make([]LinkCell, n)
	}
	g.cells = g.cells[:n]
	clear(g.cells)
	if cap(g.power) < n {
		g.power = make([]LinkFlag, n)
	}
	g.power = g.power[:n]
	clear(g.power)
	if tw := w.TerrainW; tw > 0 {
		for i, c := range w.Terrain {
			x, y := i%tw, i/tw
			if x < width && y < height {
				g.cells[y*width+x] = terrainLinkOf[c]
			}
		}
	}
	var k K
	for i := range w.Buildings {
		b := &w.Buildings[i]
		if c := k.Building(b.Kind); c.Flags != 0 && g.in(b.X, b.Y) {
			g.cells[b.Y*width+b.X] = c
		}
		if f := g.powerFlags(b); f != 0 {
			g.stampPower(g.power, nil, f, b.X, b.Y, b.W, b.H)
		}
	}
}

// stampPower ors f into the power layer over the footprint w×h at (x, y)
// (at least one cell): into cells when it is not nil, else into the
// planned set p.
func (g *LinkGrid[K]) stampPower(cells []LinkFlag, p map[[2]int]LinkFlag, f LinkFlag, x, y, w, h int) {
	for dy := 0; dy < max(h, 1); dy++ {
		for dx := 0; dx < max(w, 1); dx++ {
			cx, cy := x+dx, y+dy
			if !g.in(cx, cy) {
				continue
			}
			if cells != nil {
				cells[cy*g.w+cx] |= f
			} else {
				p[[2]int{cx, cy}] |= f
			}
		}
	}
}

// ConduitMask is the power-layer mask of a built conduit at (x, y):
// N=1 E=2 S=4 W=8 for each neighbour it joins.
func (g *LinkGrid[K]) ConduitMask(x, y int) int { return g.PowerMask(ConduitLink.Accepts, x, y, nil) }

// PowerMask is the power-layer mask at (x, y) of a cell accepting
// accepts, against the built layer plus the planned set p (nil: built
// only). Nothing links off the map.
func (g *LinkGrid[K]) PowerMask(accepts LinkFlag, x, y int, p map[[2]int]LinkFlag) int {
	m := 0
	for _, d := range [4][3]int{{0, -1, art.WallN}, {1, 0, art.WallE}, {0, 1, art.WallS}, {-1, 0, art.WallW}} {
		nx, ny := x+d[0], y+d[1]
		if !g.in(nx, ny) || len(g.power) != g.w*g.h {
			continue
		}
		f := g.power[ny*g.w+nx]
		if p != nil {
			f |= p[[2]int{nx, ny}]
		}
		if f&accepts != 0 {
			m |= d[2]
		}
	}
	return m
}

func (g *LinkGrid[K]) in(x, y int) bool { return x >= 0 && y >= 0 && x < g.w && y < g.h }

func (g *LinkGrid[K]) at(x, y int) LinkCell {
	if !g.in(x, y) {
		return LinkCell{}
	}
	return g.cells[y*g.w+x]
}

// Linked reports whether the cell at (x, y) joins its neighbour (nx, ny).
func (g *LinkGrid[K]) Linked(x, y, nx, ny int) bool { return g.linkOf(g.at(x, y), nx, ny, nil) }

// linkOf is linked for a cell c (built or planned) against the grid plus
// the planned set p (nil: built only). Off the map only map-edge cells
// link.
func (g *LinkGrid[K]) linkOf(c LinkCell, nx, ny int, p map[[2]int]LinkFlag) bool {
	if c.Accepts == 0 {
		return false
	}
	if !g.in(nx, ny) {
		return c.Flags&LinkMapEdge != 0
	}
	f := g.cells[ny*g.w+nx].Flags
	if p != nil {
		f |= p[[2]int{nx, ny}]
	}
	return f&c.Accepts != 0
}

// Mask is the cardinal link Mask of (x, y): N=1 E=2 S=4 W=8.
func (g *LinkGrid[K]) Mask(x, y int) int { return g.maskOf(g.at(x, y), x, y, nil) }

// Corners is the filled-corner set of (x, y): NE=1 SE=2 SW=4 NW=8, each
// set when both its cardinals and its diagonal link.
func (g *LinkGrid[K]) Corners(x, y int) int { return g.cornersOf(g.at(x, y), x, y, nil) }

func (g *LinkGrid[K]) maskOf(c LinkCell, x, y int, p map[[2]int]LinkFlag) int {
	m := 0
	if g.linkOf(c, x, y-1, p) {
		m |= art.WallN
	}
	if g.linkOf(c, x+1, y, p) {
		m |= art.WallE
	}
	if g.linkOf(c, x, y+1, p) {
		m |= art.WallS
	}
	if g.linkOf(c, x-1, y, p) {
		m |= art.WallW
	}
	return m
}

func (g *LinkGrid[K]) cornersOf(c LinkCell, x, y int, p map[[2]int]LinkFlag) int {
	m := g.maskOf(c, x, y, p)
	k := 0
	if m&(art.WallN|art.WallE) == art.WallN|art.WallE && g.linkOf(c, x+1, y-1, p) {
		k |= CornerNE
	}
	if m&(art.WallS|art.WallE) == art.WallS|art.WallE && g.linkOf(c, x+1, y+1, p) {
		k |= CornerSE
	}
	if m&(art.WallS|art.WallW) == art.WallS|art.WallW && g.linkOf(c, x-1, y+1, p) {
		k |= CornerSW
	}
	if m&(art.WallN|art.WallW) == art.WallN|art.WallW && g.linkOf(c, x-1, y-1, p) {
		k |= CornerNW
	}
	return k
}

// PlannedCell is a ghost (placement preview) cell for the planned set.
type PlannedCell struct {
	X, Y  int
	Flags LinkFlag
}

// Plan fills the reusable planned set with the frames' and ghosts' link
// flags (a device frame's power flags over its whole footprint): O(frames
// + ghosts), never O(map).
func (g *LinkGrid[K]) Plan(frames []game.FrameView, ghosts []PlannedCell) map[[2]int]LinkFlag {
	if g.planned == nil {
		g.planned = map[[2]int]LinkFlag{}
	}
	clear(g.planned)
	var k K
	for i := range frames {
		f := &frames[i]
		if c := k.Building(f.Kind); c.Flags != 0 {
			g.planned[[2]int{f.X, f.Y}] |= c.Flags
		}
		if c := k.Power(f.Kind); c.Flags != 0 {
			g.stampPower(nil, g.planned, c.Flags, f.X, f.Y, f.W, f.H)
		}
	}
	for _, c := range ghosts {
		g.planned[[2]int{c.X, c.Y}] |= c.Flags
	}
	return g.planned
}

// PlannedMask is the mask of a planned cell that accepts accepts, against
// built cells and the planned set p.
func (g *LinkGrid[K]) PlannedMask(p map[[2]int]LinkFlag, accepts LinkFlag, x, y int) int {
	return g.maskOf(LinkCell{Accepts: accepts}, x, y, p)
}

// PlannedCorners is cornersOf for a planned cell.
func (g *LinkGrid[K]) PlannedCorners(p map[[2]int]LinkFlag, accepts LinkFlag, x, y int) int {
	return g.cornersOf(LinkCell{Accepts: accepts}, x, y, p)
}

// relevantCorners are the corners a mask can fill: NE with N and E, SE
// with S and E, SW with S and W, NW with N and W.
func relevantCorners(mask int) int {
	k := 0
	if mask&(art.WallN|art.WallE) == art.WallN|art.WallE {
		k |= CornerNE
	}
	if mask&(art.WallS|art.WallE) == art.WallS|art.WallE {
		k |= CornerSE
	}
	if mask&(art.WallS|art.WallW) == art.WallS|art.WallW {
		k |= CornerSW
	}
	if mask&(art.WallN|art.WallW) == art.WallN|art.WallW {
		k |= CornerNW
	}
	return k
}

// LinkedKey is the baked variant of a linked family: <prefix>_<NN> when
// no relevant corner is filled, else <prefix>_<NN>_c<h> (lowercase hex
// of the filled corners). 47 keys per family.
func LinkedKey(prefix string, mask, corners int) string {
	mask &= 15
	c := corners & relevantCorners(mask)
	if c == 0 {
		return prefix + "_" + TwoDigits(mask)
	}
	return fmt.Sprintf("%s_%s_c%x", prefix, TwoDigits(mask), c)
}

// LinkedKeys lists every key LinkedKey produces for prefix.
func LinkedKeys(prefix string) []string {
	seen := map[string]bool{}
	var out []string
	for m := 0; m < 16; m++ {
		for k := 0; k < 16; k++ {
			if s := LinkedKey(prefix, m, k); !seen[s] {
				seen[s] = true
				out = append(out, s)
			}
		}
	}
	return out
}
