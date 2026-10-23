package ebiten

import (
	"sync"

	"rimworld-go/internal/application/game"
)

// The NG-1 E5 read models for the simulation doubles: a game.ChunkIndex
// over whatever world the double serves, updated on each call (double
// worlds are small). A double that overrides World() needs its own three
// methods (below), or the stub's would index the stub's world instead.

var doubleChunks sync.Map // the double -> *game.ChunkIndex

func chunkIndexOf(key any, w *game.WorldView, mw, mh int) *game.ChunkIndex {
	v, _ := doubleChunks.LoadOrStore(key, &game.ChunkIndex{})
	x := v.(*game.ChunkIndex)
	x.Update(w, mw, mh)
	return x
}

// worldReader is a double's world and size.
type worldReader interface {
	World() game.WorldView
	Width() int
	Height() int
}

func doubleChunk(s worldReader, cx, cy int) game.ChunkView {
	w := s.World()
	return chunkIndexOf(s, &w, s.Width(), s.Height()).Layer(cx, cy)
}

func doublePlantsIn(s worldReader, x0, y0, x1, y1 int, buf []int32) []int32 {
	w := s.World()
	return chunkIndexOf(s, &w, s.Width(), s.Height()).PlantsIn(&w, x0, y0, x1, y1, buf)
}

func doubleMinimap(s worldReader, ver uint64) (game.MinimapView, bool) {
	w := s.World()
	return chunkIndexOf(s, &w, s.Width(), s.Height()).Minimap(&w, ver)
}

func (s *stubSim) ChunkLayer(cx, cy int) game.ChunkView { return doubleChunk(s, cx, cy) }
func (s *stubSim) PlantsIn(x0, y0, x1, y1 int, buf []int32) []int32 {
	return doublePlantsIn(s, x0, y0, x1, y1, buf)
}
func (s *stubSim) MinimapLayer(ver uint64) (game.MinimapView, bool) { return doubleMinimap(s, ver) }

func (s *motionSim) ChunkLayer(cx, cy int) game.ChunkView { return doubleChunk(s, cx, cy) }
func (s *motionSim) PlantsIn(x0, y0, x1, y1 int, buf []int32) []int32 {
	return doublePlantsIn(s, x0, y0, x1, y1, buf)
}
func (s *motionSim) MinimapLayer(ver uint64) (game.MinimapView, bool) { return doubleMinimap(s, ver) }

func (s *envSim) ChunkLayer(cx, cy int) game.ChunkView { return doubleChunk(s, cx, cy) }
func (s *envSim) PlantsIn(x0, y0, x1, y1 int, buf []int32) []int32 {
	return doublePlantsIn(s, x0, y0, x1, y1, buf)
}
func (s *envSim) MinimapLayer(ver uint64) (game.MinimapView, bool) { return doubleMinimap(s, ver) }

func (s *worldSim) ChunkLayer(cx, cy int) game.ChunkView { return doubleChunk(s, cx, cy) }
func (s *worldSim) PlantsIn(x0, y0, x1, y1 int, buf []int32) []int32 {
	return doublePlantsIn(s, x0, y0, x1, y1, buf)
}
func (s *worldSim) MinimapLayer(ver uint64) (game.MinimapView, bool) { return doubleMinimap(s, ver) }

func (s *stainSim) ChunkLayer(cx, cy int) game.ChunkView { return doubleChunk(s, cx, cy) }
func (s *stainSim) PlantsIn(x0, y0, x1, y1 int, buf []int32) []int32 {
	return doublePlantsIn(s, x0, y0, x1, y1, buf)
}
func (s *stainSim) MinimapLayer(ver uint64) (game.MinimapView, bool) { return doubleMinimap(s, ver) }

func (s *carpetSim) ChunkLayer(cx, cy int) game.ChunkView { return doubleChunk(s, cx, cy) }
func (s *carpetSim) PlantsIn(x0, y0, x1, y1 int, buf []int32) []int32 {
	return doublePlantsIn(s, x0, y0, x1, y1, buf)
}
func (s *carpetSim) MinimapLayer(ver uint64) (game.MinimapView, bool) { return doubleMinimap(s, ver) }

func (s *dirtSim) ChunkLayer(cx, cy int) game.ChunkView { return doubleChunk(s, cx, cy) }
func (s *dirtSim) PlantsIn(x0, y0, x1, y1 int, buf []int32) []int32 {
	return doublePlantsIn(s, x0, y0, x1, y1, buf)
}
func (s *dirtSim) MinimapLayer(ver uint64) (game.MinimapView, bool) { return doubleMinimap(s, ver) }

func (s *recSim) ChunkLayer(cx, cy int) game.ChunkView { return doubleChunk(s, cx, cy) }
func (s *recSim) PlantsIn(x0, y0, x1, y1 int, buf []int32) []int32 {
	return doublePlantsIn(s, x0, y0, x1, y1, buf)
}
func (s *recSim) MinimapLayer(ver uint64) (game.MinimapView, bool) { return doubleMinimap(s, ver) }
