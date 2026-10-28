package ebiten

import (
	"fmt"

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

type linkFlag uint8

const (
	linkMapEdge linkFlag = 1 << iota
	linkRock
	linkWall
	linkFence
	linkConduit // the power layer: a transmitter (a conduit, shown or hidden, or a power switch)
	linkDevice  // the power layer: any cell of a power device
)

// linkCell is what a cell is (flags) and what it joins (accepts).
type linkCell struct{ Flags, Accepts linkFlag }

var (
	rockLink  = linkCell{Flags: linkRock | linkMapEdge, Accepts: linkRock}
	wallLink  = linkCell{Flags: linkWall | linkRock, Accepts: linkWall | linkRock}
	fenceLink = linkCell{Flags: linkFence, Accepts: linkFence | linkWall}
	// conduitLink is a conduit on the power layer.
	conduitLink = linkCell{Flags: linkConduit, Accepts: linkConduit | linkDevice}
)

// Corner bits: clockwise from north-east, like the cardinal mask bits.
const (
	cornerNE = 1
	cornerSE = 2
	cornerSW = 4
	cornerNW = 8
)

// terrainLinkOf maps WorldView.Terrain codes to link cells (rock and
// smoothed rock; ore lies in rock and links like it).
var terrainLinkOf = func() [256]linkCell {
	var out [256]linkCell
	for i, n := range game.TerrainNames() {
		if isRockWallName(n) {
			out[i] = rockLink
		}
	}
	return out
}()

// isRockWallName reports a natural rock wall terrain (rough or smoothed).
func isRockWallName(n string) bool { return n == "rock" || n == "rock_smooth" }

// isRockWallCode is isRockWallName for a WorldView.Terrain code.
func isRockWallCode(c uint8) bool { return terrainLinkOf[c].Flags&linkRock != 0 }

// buildingLink is the link cell of a building or frame kind.
func buildingLink(kind string) linkCell {
	switch kind {
	case "wall", "door", coolerKind: // a cooler fills a wall gap like a door
		return wallLink
	case fenceKind, gateKind:
		return fenceLink
	}
	return linkCell{}
}

// powerLink is the power-layer link cell of a building or frame kind:
// conduits link, a switch is a transmitter they join, a device is joined.
func powerLink(kind string) linkCell {
	switch {
	case isConduitKind(kind):
		return conduitLink
	case kind == powerSwitchKind:
		return linkCell{Flags: linkConduit}
	case powerDeviceKinds[kind]:
		return linkCell{Flags: linkDevice}
	}
	return linkCell{}
}

// buildingPowerFlags are a built building's power-layer flags: by kind,
// or a device by its view's PowerRole (the simulation's word).
func buildingPowerFlags(b *game.BuildingView) linkFlag {
	if c := powerLink(b.Kind); c.Flags != 0 {
		return c.Flags
	}
	switch b.PowerRole {
	case "producer", "consumer", "storage":
		return linkDevice
	}
	return 0
}

// linkGrid is the per-cell link data of the map, cached on StructVer.
// StructVer is saved, so it can repeat across games: the grid lives on the
// Renderer, and every session (new or loaded) gets a new Renderer
// (App.startSession). Frames don't bump StructVer, so they stay in the
// per-frame planned set.
type linkGrid struct {
	w, h  int
	ver   uint64
	built bool
	cells []linkCell
	// power is the power layer's flags per cell (conduits, switches and
	// every footprint cell of a device).
	power []linkFlag
	// planned is the reusable per-frame set of frame and ghost cells.
	planned map[[2]int]linkFlag
}

// Sync rebuilds the grid when it was never built, the map size changed or
// w.StructVer moved. A zero StructVer (a sim that publishes no version)
// rebuilds every call: correct, just not cached.
func (g *linkGrid) Sync(w game.WorldView, width, height int) {
	if g.built && w.StructVer != 0 && w.StructVer == g.ver && g.w == width && g.h == height {
		return
	}
	g.built, g.ver, g.w, g.h = true, w.StructVer, width, height
	n := width * height
	if cap(g.cells) < n {
		g.cells = make([]linkCell, n)
	}
	g.cells = g.cells[:n]
	clear(g.cells)
	if cap(g.power) < n {
		g.power = make([]linkFlag, n)
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
	for i := range w.Buildings {
		b := &w.Buildings[i]
		if c := buildingLink(b.Kind); c.Flags != 0 && g.in(b.X, b.Y) {
			g.cells[b.Y*width+b.X] = c
		}
		if f := buildingPowerFlags(b); f != 0 {
			g.stampPower(g.power, nil, f, b.X, b.Y, b.W, b.H)
		}
	}
}

// stampPower ors f into the power layer over the footprint w×h at (x, y)
// (at least one cell): into cells when it is not nil, else into the
// planned set p.
func (g *linkGrid) stampPower(cells []linkFlag, p map[[2]int]linkFlag, f linkFlag, x, y, w, h int) {
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
func (g *linkGrid) ConduitMask(x, y int) int { return g.PowerMask(conduitLink.Accepts, x, y, nil) }

// PowerMask is the power-layer mask at (x, y) of a cell accepting
// accepts, against the built layer plus the planned set p (nil: built
// only). Nothing links off the map.
func (g *linkGrid) PowerMask(accepts linkFlag, x, y int, p map[[2]int]linkFlag) int {
	m := 0
	for _, d := range [4][3]int{{0, -1, wallN}, {1, 0, wallE}, {0, 1, wallS}, {-1, 0, wallW}} {
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

func (g *linkGrid) in(x, y int) bool { return x >= 0 && y >= 0 && x < g.w && y < g.h }

func (g *linkGrid) at(x, y int) linkCell {
	if !g.in(x, y) {
		return linkCell{}
	}
	return g.cells[y*g.w+x]
}

// Linked reports whether the cell at (x, y) joins its neighbour (nx, ny).
func (g *linkGrid) Linked(x, y, nx, ny int) bool { return g.linkOf(g.at(x, y), nx, ny, nil) }

// linkOf is linked for a cell c (built or planned) against the grid plus
// the planned set p (nil: built only). Off the map only map-edge cells
// link.
func (g *linkGrid) linkOf(c linkCell, nx, ny int, p map[[2]int]linkFlag) bool {
	if c.Accepts == 0 {
		return false
	}
	if !g.in(nx, ny) {
		return c.Flags&linkMapEdge != 0
	}
	f := g.cells[ny*g.w+nx].Flags
	if p != nil {
		f |= p[[2]int{nx, ny}]
	}
	return f&c.Accepts != 0
}

// Mask is the cardinal link Mask of (x, y): N=1 E=2 S=4 W=8.
func (g *linkGrid) Mask(x, y int) int { return g.maskOf(g.at(x, y), x, y, nil) }

// Corners is the filled-corner set of (x, y): NE=1 SE=2 SW=4 NW=8, each
// set when both its cardinals and its diagonal link.
func (g *linkGrid) Corners(x, y int) int { return g.cornersOf(g.at(x, y), x, y, nil) }

func (g *linkGrid) maskOf(c linkCell, x, y int, p map[[2]int]linkFlag) int {
	m := 0
	if g.linkOf(c, x, y-1, p) {
		m |= wallN
	}
	if g.linkOf(c, x+1, y, p) {
		m |= wallE
	}
	if g.linkOf(c, x, y+1, p) {
		m |= wallS
	}
	if g.linkOf(c, x-1, y, p) {
		m |= wallW
	}
	return m
}

func (g *linkGrid) cornersOf(c linkCell, x, y int, p map[[2]int]linkFlag) int {
	m := g.maskOf(c, x, y, p)
	k := 0
	if m&(wallN|wallE) == wallN|wallE && g.linkOf(c, x+1, y-1, p) {
		k |= cornerNE
	}
	if m&(wallS|wallE) == wallS|wallE && g.linkOf(c, x+1, y+1, p) {
		k |= cornerSE
	}
	if m&(wallS|wallW) == wallS|wallW && g.linkOf(c, x-1, y+1, p) {
		k |= cornerSW
	}
	if m&(wallN|wallW) == wallN|wallW && g.linkOf(c, x-1, y-1, p) {
		k |= cornerNW
	}
	return k
}

// plannedCell is a ghost (placement preview) cell for the planned set.
type plannedCell struct {
	X, Y  int
	Flags linkFlag
}

// Plan fills the reusable planned set with the frames' and ghosts' link
// flags (a device frame's power flags over its whole footprint): O(frames
// + ghosts), never O(map).
func (g *linkGrid) Plan(frames []game.FrameView, ghosts []plannedCell) map[[2]int]linkFlag {
	if g.planned == nil {
		g.planned = map[[2]int]linkFlag{}
	}
	clear(g.planned)
	for i := range frames {
		f := &frames[i]
		if c := buildingLink(f.Kind); c.Flags != 0 {
			g.planned[[2]int{f.X, f.Y}] |= c.Flags
		}
		if c := powerLink(f.Kind); c.Flags != 0 {
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
func (g *linkGrid) PlannedMask(p map[[2]int]linkFlag, accepts linkFlag, x, y int) int {
	return g.maskOf(linkCell{Accepts: accepts}, x, y, p)
}

// PlannedCorners is cornersOf for a planned cell.
func (g *linkGrid) PlannedCorners(p map[[2]int]linkFlag, accepts linkFlag, x, y int) int {
	return g.cornersOf(linkCell{Accepts: accepts}, x, y, p)
}

// relevantCorners are the corners a mask can fill: NE with N and E, SE
// with S and E, SW with S and W, NW with N and W.
func relevantCorners(mask int) int {
	k := 0
	if mask&(wallN|wallE) == wallN|wallE {
		k |= cornerNE
	}
	if mask&(wallS|wallE) == wallS|wallE {
		k |= cornerSE
	}
	if mask&(wallS|wallW) == wallS|wallW {
		k |= cornerSW
	}
	if mask&(wallN|wallW) == wallN|wallW {
		k |= cornerNW
	}
	return k
}

// linkedKey is the baked variant of a linked family: <prefix>_<NN> when
// no relevant corner is filled, else <prefix>_<NN>_c<h> (lowercase hex
// of the filled corners). 47 keys per family.
func linkedKey(prefix string, mask, corners int) string {
	mask &= 15
	c := corners & relevantCorners(mask)
	if c == 0 {
		return prefix + "_" + twoDigits(mask)
	}
	return fmt.Sprintf("%s_%s_c%x", prefix, twoDigits(mask), c)
}

// linkedKeys lists every key linkedKey produces for prefix.
func linkedKeys(prefix string) []string {
	seen := map[string]bool{}
	var out []string
	for m := 0; m < 16; m++ {
		for k := 0; k < 16; k++ {
			if s := linkedKey(prefix, m, k); !seen[s] {
				seen[s] = true
				out = append(out, s)
			}
		}
	}
	return out
}
