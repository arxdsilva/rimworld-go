package game

import (
	"runtime"
	"sync"
	"weak"

	"rimworld-go/internal/application/game/mapview"
	"rimworld-go/internal/domain/world/chunk"
)

// Per-chunk read models for the renderer (NG-1 §6.7 E5). The renderer
// draws only the chunks in view, so it needs each chunk's share of the
// World() lists, and a version per chunk to know when a baked image of
// the chunk's ground is stale. Both come from the same World() the
// renderer draws (they are rebuilt once per read-model key, the tick and
// command generation of the view cache), so the indexes stay valid for it.

// ChunkSize is a chunk's side in tiles.
const ChunkSize = chunk.Size

// chunkReach is how far around a chunk its ground version looks: one
// column each side, one row north and three south.
const (
	chunkReachX, chunkReachN, chunkReachS = 1, 1, 3
)

// ChunkIndex is the chunk read model over one WorldView at a time: Game
// keeps one per game (ChunkLayer, PlantsIn, MinimapLayer), and a
// simulation double can keep its own over the world it serves. Update it
// with each new view; its versions count changes between updates.
type ChunkIndex struct {
	key  [7]uint64 // Game: the key it was updated at (chunkKey)
	ok   bool
	grid chunk.Grid

	plants, items, bldgs, frames mapview.Buckets

	// structVer is the WorldView.StructVer terrHash was hashed at.
	structVer uint64
	structOK  bool
	terrHash  []uint64 // per chunk: terrain, stone and ore over the reach
	ground    []uint64 // per chunk: last ground hash (terrain + buildings)
	plantHash []uint64 // per chunk: last plant hash
	plantAt   []uint64 // per chunk: last hash of the plants' tiles only
	ver       []uint64
	plantVer  []uint64
	scratch   []uint64
	// gen counts changes anywhere to the ground or to where plants stand
	// (MinimapLayer's version; growth alone does not count).
	gen uint64

	mini minimapState
}

// The chunk read model lives beside the Game, keyed weakly so a Game that
// is dropped takes its state with it.
var (
	chunkMu     sync.Mutex
	chunkStates = map[weak.Pointer[Game]]*ChunkIndex{}
)

// chunkIndex is g's ChunkIndex (a side table until Game holds it as a
// field; the cleanup drops it with the game).
func (g *Game) chunkIndex() *ChunkIndex {
	k := weak.Make(g)
	chunkMu.Lock()
	defer chunkMu.Unlock()
	s := chunkStates[k]
	if s == nil {
		s = &ChunkIndex{}
		chunkStates[k] = s
		runtime.AddCleanup(g, func(k weak.Pointer[Game]) {
			chunkMu.Lock()
			delete(chunkStates, k)
			chunkMu.Unlock()
		}, k)
	}
	return s
}

// chunks is the chunk read model for the current read-model key: the
// view cache's (tick, command generation) plus what a dev hook moves
// between ticks (structure version, list lengths), so a caller without
// the view cache sees those edits as World() does.
func (g *Game) chunks() *ChunkIndex {
	s := g.chunkIndex()
	k := g.chunkKey()
	if s.ok && s.key == k {
		return s
	}
	w := g.World()
	s.Update(&w, g.Width(), g.Height())
	s.key, s.ok = k, true
	return s
}

func (g *Game) chunkKey() [7]uint64 {
	v := g.viewKey()
	return [7]uint64{v[0], v[1], g.structVer, uint64(len(g.itemIDs)), uint64(len(g.plantIDs)), uint64(len(g.bldgIDs)), uint64(len(g.frameIDs))}
}

// ChunkLayer is chunk (cx, cy)'s share of the current World() (a zero
// view off the map).
func (g *Game) ChunkLayer(cx, cy int) ChunkView { return g.chunks().Layer(cx, cy) }

// PlantsIn appends to buf the indexes into World().Plants of the plants
// on tiles x0..x1 × y0..y1 (inclusive), chunk by chunk, and returns it.
func (g *Game) PlantsIn(x0, y0, x1, y1 int, buf []int32) []int32 {
	s := g.chunks()
	w := g.World()
	return s.PlantsIn(&w, x0, y0, x1, y1, buf)
}

// Layer is chunk (cx, cy)'s view (zero off the map).
func (s *ChunkIndex) Layer(cx, cy int) ChunkView {
	if cx < 0 || cy < 0 || cx >= s.grid.CW || cy >= s.grid.CH {
		return ChunkView{}
	}
	c := cy*s.grid.CW + cx
	return ChunkView{
		Ver: s.ver[c], PlantVer: s.plantVer[c],
		Plants: s.plants.Of(c), Items: s.items.Of(c), Buildings: s.bldgs.Of(c), Frames: s.frames.Of(c),
	}
}

// PlantsIn is Game.PlantsIn over w, the view s was last updated with.
func (s *ChunkIndex) PlantsIn(w *WorldView, x0, y0, x1, y1 int, buf []int32) []int32 {
	x0, y0 = max(x0, 0), max(y0, 0)
	x1, y1 = min(x1, s.grid.W-1), min(y1, s.grid.H-1)
	if x0 > x1 || y0 > y1 {
		return buf
	}
	for cy := y0 >> chunk.Shift; cy <= y1>>chunk.Shift; cy++ {
		for cx := x0 >> chunk.Shift; cx <= x1>>chunk.Shift; cx++ {
			for _, i := range s.plants.Of(cy*s.grid.CW + cx) {
				if int(i) >= len(w.Plants) {
					continue
				}
				p := &w.Plants[i]
				if p.X >= x0 && p.X <= x1 && p.Y >= y0 && p.Y <= y1 {
					buf = append(buf, i)
				}
			}
		}
	}
	return buf
}

// Update buckets w's lists (an mw × mh map) by chunk and bumps the
// versions of the chunks whose ground or plants changed since the last
// update.
func (s *ChunkIndex) Update(w *WorldView, mw, mh int) {
	if s.grid.W != mw || s.grid.H != mh || len(s.ver) != chunk.New(mw, mh).Count() {
		*s = ChunkIndex{grid: chunk.New(mw, mh), gen: s.gen + 1}
		n := s.grid.Count()
		s.terrHash, s.ground, s.plantHash, s.plantAt = make([]uint64, n), make([]uint64, n), make([]uint64, n), make([]uint64, n)
		s.ver, s.plantVer = make([]uint64, n), make([]uint64, n)
	}
	cg, n := s.grid, s.grid.Count()
	of := func(x, y int) int {
		if x < 0 || y < 0 || x >= cg.W || y >= cg.H {
			return -1
		}
		return cg.Of(x, y)
	}
	s.plants.Fill(n, len(w.Plants), func(i int) int { return of(w.Plants[i].X, w.Plants[i].Y) })
	s.items.Fill(n, len(w.Items), func(i int) int { return of(w.Items[i].X, w.Items[i].Y) })
	s.bldgs.Fill(n, len(w.Buildings), func(i int) int { return of(w.Buildings[i].X, w.Buildings[i].Y) })
	s.frames.Fill(n, len(w.Frames), func(i int) int { return of(w.Frames[i].X, w.Frames[i].Y) })

	if !s.structOK || s.structVer != w.StructVer {
		s.hashTerrain(w)
		s.structVer, s.structOK = w.StructVer, true
	}
	// ground: terrain plus every building touching the chunk's reach
	gh := mapview.Grow64(s.scratch, n)
	s.scratch = gh
	copy(gh, s.terrHash)
	for i := range w.Buildings {
		b := &w.Buildings[i]
		h := mapview.MixString(mapview.MixString(mapview.MixString(mapview.Mix(mapview.Mix(0, uint64(b.X)), uint64(b.Y)), b.Kind), b.Stuff), b.Facing)
		h = mapview.Mix(mapview.Mix(h, uint64(b.W)), uint64(b.H))
		bw, bh := max(b.W, 1), max(b.H, 1)
		// chunks whose reach overlaps the footprint
		cx0, cx1 := max(b.X-chunkReachX, 0)>>chunk.Shift, min(b.X+bw-1+chunkReachX, cg.W-1)>>chunk.Shift
		cy0, cy1 := max(b.Y-chunkReachS, 0)>>chunk.Shift, min(b.Y+bh-1+chunkReachN, cg.H-1)>>chunk.Shift
		for cy := cy0; cy <= cy1; cy++ {
			for cx := cx0; cx <= cx1; cx++ {
				c := cy*cg.CW + cx
				gh[c] += h // order-free: a sum of per-building hashes
			}
		}
	}
	for c := 0; c < n; c++ {
		if gh[c] != s.ground[c] || s.ver[c] == 0 {
			s.ground[c] = gh[c]
			s.ver[c]++
			s.gen++
		}
		var ph, at uint64
		for _, i := range s.plants.Of(c) {
			p := &w.Plants[i]
			h := mapview.Mix(mapview.Mix(0, uint64(p.X)), uint64(p.Y))
			at += h
			h = mapview.Mix(mapview.MixString(h, p.Species), uint64(p.Stage)<<1|uint64(mapview.Bit(p.Harvestable)))
			ph += h
		}
		if ph != s.plantHash[c] || s.plantVer[c] == 0 {
			s.plantHash[c] = ph
			s.plantVer[c]++
		}
		if at != s.plantAt[c] {
			s.plantAt[c] = at
			s.gen++
		}
	}
}

// hashTerrain hashes terrain, stone and ore over each chunk's reach.
func (s *ChunkIndex) hashTerrain(w *WorldView) {
	cg := s.grid
	tw := w.TerrainW
	at := func(l []uint8, i int) uint64 {
		if i < len(l) {
			return uint64(l[i])
		}
		return 0
	}
	for c := range s.terrHash {
		x0, y0, x1, y1 := cg.Bounds(c)
		x0, x1 = max(x0-chunkReachX, 0), min(x1+chunkReachX, cg.W)
		y0, y1 = max(y0-chunkReachN, 0), min(y1+chunkReachS, cg.H)
		h := uint64(c)
		for y := y0; y < y1 && tw > 0; y++ {
			for x := x0; x < x1; x++ {
				i := y*tw + x
				h = mapview.Mix(h, at(w.Terrain, i)|at(w.StoneType, i)<<8|at(w.Ore, i)<<16)
			}
		}
		s.terrHash[c] = h
	}
}
