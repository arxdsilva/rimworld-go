package game

import (
	"slices"

	"rimworld-go/internal/domain/world/terrain"
)

// Fog of war (MAPGEN-2 §5). A Setup colony starts with every tile hidden
// but what its landing spot sees: a flood over the ground the colonists
// could walk (doors pass) and open water, plus the 8 neighbours of every
// flooded tile, so cliff faces and walls show. Sealed caves, walled-in
// ruins and rock interiors stay hidden until a tile next to explored
// ground opens into them (mining, a wall taken down): fogOpen then floods
// the newly connected hidden area only, so a reveal costs the area it
// shows, never a map scan. Tiles are never hidden again.
//
// Fog is presentation-facing state: the simulation and the AI see the
// truth; World()/Snapshot() leave hidden things out (fog_view.go) and
// orders and placement refuse hidden tiles, except mine designations so
// the player can dig in.

// fogState is the fog of war: which tiles are unexplored. The zero value
// is no fog (legacy colonies, older saves).
type fogState struct {
	hidden []uint64 // bitset, 1 = unexplored; nil = no fog
	ver    uint64   // bumps on every change (WorldView.FogVer)
	// never saved:
	queue   []int32    // flood scratch
	visited int        // tiles the last flood expanded (reveal cost, tests)
	view    layerCache // WorldView.Fog, rebuilt when ver moves
}

const (
	// caveMessage is posted when an opening reveals at least
	// caveMessageMin tiles.
	caveMessage    = "A cave opened up."
	caveMessageMin = 20
)

// fogged reports an unexplored tile.
func (g *Game) fogged(x, y int) bool {
	if g.fog.hidden == nil || !g.grid.InBounds(x, y) {
		return false
	}
	i := y*g.grid.Width() + x
	return g.fog.hidden[i>>6]&(1<<(uint(i)&63)) != 0
}

// Fogged reports whether tile (x, y) is unexplored (fog of war).
func (g *Game) Fogged(x, y int) bool { return g.fogged(x, y) }

// initFog fogs a Setup colony's map from its landing spot (after the
// arrival). A legacy colony keeps no fog.
func (g *Game) initFog() {
	if g.ng.setup == nil {
		return
	}
	x, y := g.landingSpot()
	g.fogFromStart(x, y)
}

// fogFromStart hides the whole map, then reveals what (x, y) sees.
func (g *Game) fogFromStart(x, y int) {
	w, h := g.grid.Width(), g.grid.Height()
	n := w * h
	if n == 0 {
		return
	}
	words := (n + 63) / 64
	g.fog.hidden = make([]uint64, words)
	for i := range g.fog.hidden {
		g.fog.hidden[i] = ^uint64(0)
	}
	if r := n & 63; r != 0 {
		g.fog.hidden[words-1] = 1<<uint(r) - 1 // no bits past the map
	}
	if !g.grid.InBounds(x, y) {
		x, y = w/2, h/2
	}
	g.fogFlood(x, y)
	g.fogChanged()
}

// fogPass reports a tile the reveal flood crosses: walkable ground (doors
// included) or open water, which hides nothing behind it.
func (g *Game) fogPass(x, y int) bool {
	return g.grid.Walkable(x, y) || g.terr.At(x, y) == terrain.DeepWater
}

// fogOpen is the reveal hook for a tile that may just have become
// walkable (terrain changed, a blocking building removed or moved): when
// it touches explored open ground and hidden tiles, the hidden area it
// now connects to is revealed. A no-op without fog.
func (g *Game) fogOpen(x, y int) {
	if g.fog.hidden == nil || !g.grid.InBounds(x, y) || !g.fogPass(x, y) {
		return
	}
	hidden, seen := false, false
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			nx, ny := x+dx, y+dy
			switch {
			case !g.grid.InBounds(nx, ny):
			case g.fogged(nx, ny):
				hidden = true
			case (dx != 0 || dy != 0) && g.fogPass(nx, ny):
				seen = true
			}
		}
	}
	if !hidden || !seen {
		return
	}
	if n := g.fogFlood(x, y); n >= caveMessageMin {
		g.message(x, y, caveMessage)
	}
	g.fogChanged()
}

// fogFlood reveals (sx, sy), then floods from it over hidden passable
// tiles, revealing each one and its 8 neighbours. Explored passable tiles
// are not expanded: their neighbours already show. It returns how many
// tiles it revealed.
func (g *Game) fogFlood(sx, sy int) int {
	w, h := g.grid.Width(), g.grid.Height()
	revealed := 0
	if g.fogReveal(sx, sy, sy*w+sx) {
		revealed++
	}
	q := append(g.fog.queue[:0], int32(sy*w+sx))
	for head := 0; head < len(q); head++ {
		i := int(q[head])
		x, y := i%w, i/w
		for dy := -1; dy <= 1; dy++ {
			ny := y + dy
			if ny < 0 || ny >= h {
				continue
			}
			for dx := -1; dx <= 1; dx++ {
				nx := x + dx
				if nx < 0 || nx >= w {
					continue
				}
				j := ny*w + nx
				if !g.fogReveal(nx, ny, j) {
					continue
				}
				revealed++
				if g.fogPass(nx, ny) {
					q = append(q, int32(j))
				}
			}
		}
	}
	g.fog.visited = len(q)
	g.fog.queue = q[:0]
	return revealed
}

// fogReveal clears tile i = (x, y) if hidden, reporting whether it was. A
// mine mark on a revealed tile that isn't rock (marked in the dark) goes.
func (g *Game) fogReveal(x, y, i int) bool {
	word, bit := &g.fog.hidden[i>>6], uint64(1)<<(uint(i)&63)
	if *word&bit == 0 {
		return false
	}
	*word &^= bit
	if len(g.mines) > 0 && !terrain.IsRockWall(g.terr.At(x, y)) {
		if k := [2]int{x, y}; g.mines[k] {
			delete(g.mines, k)
			g.minesVer++
		}
	}
	return true
}

// fogChanged invalidates the fog's read model and the view cache.
func (g *Game) fogChanged() {
	g.fog.ver++
	g.viewGen++ // view cache (indexes.go): a reveal may come between ticks
}

// exportFog is the fog for a save (nil: no fog).
func (g *Game) exportFog() []uint64 { return slices.Clone(g.fog.hidden) }

// importFog restores a saved fog; an absent (older save) or mis-sized one
// is no fog.
func (g *Game) importFog(bits []uint64) {
	g.fog = fogState{}
	n := g.grid.Width() * g.grid.Height()
	if len(bits) == 0 || len(bits) != (n+63)/64 {
		return
	}
	g.fog.hidden = slices.Clone(bits)
	if r := n & 63; r != 0 {
		g.fog.hidden[len(bits)-1] &= 1<<uint(r) - 1
	}
	g.fog.ver = 1
}
