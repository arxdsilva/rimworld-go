package game

import (
	"encoding/binary"
	"slices"
	"sort"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/chunk"
)

// The snow clearing area (PLAY-015 §4): a painted mask (Architect → Zone
// "Snow clearing area", command DesignateSnowClear). Colonists with
// Cleaning work shovel its cells once the snow there is at least
// snowClearStart deep, down to bare ground (job ClearSnow), so a cleared
// cell is not taken up again until that much has fallen anew
// (hysteresis). Candidates come from per-chunk counts kept by the snow
// passes, never from a map scan.

const (
	snowClearStart   = 38 // 15%: shallower snow is left alone
	snowClearMinWork = 20 // work ticks for the shallowest snow ...
	snowClearMaxWork = 80 // ... and for the deepest
	snowClearProbes  = 16 // reachability probes per assignment, nearest first
)

// inSnowClear reports cell i is in the snow clearing area.
func (g *Game) inSnowClear(i int) bool {
	c := g.snow.clear
	return i >= 0 && i>>6 < len(c) && c[i>>6]&(1<<(uint(i)&63)) != 0
}

// paintSnowClear adds (on) or clears the snow clearing area over the
// rectangle.
func (g *Game) paintSnowClear(x, y, x2, y2 int, on bool) {
	minX, minY, maxX, maxY := zone.Normalize(x, y, x2, y2)
	w, h := g.grid.Width(), g.grid.Height()
	minX, minY = max(0, minX), max(0, minY)
	maxX, maxY = min(w-1, maxX), min(h-1, maxY)
	if minX > maxX || minY > maxY {
		return
	}
	s := &g.snow
	if n := (w*h + 63) / 64; len(s.clear) != n {
		if !on {
			return
		}
		s.clear = make([]uint64, n)
	}
	changed := false
	for ty := minY; ty <= maxY; ty++ {
		for tx := minX; tx <= maxX; tx++ {
			i := ty*w + tx
			bit := uint64(1) << (uint(i) & 63)
			if (s.clear[i>>6]&bit != 0) == on {
				continue
			}
			s.clear[i>>6] ^= bit
			changed = true
		}
	}
	if !changed {
		return
	}
	s.clearVer++
	if s.depth != nil {
		cg := g.snowChunkGrid()
		for cy := minY >> chunk.Shift; cy <= maxY>>chunk.Shift; cy++ {
			for cx := minX >> chunk.Shift; cx <= maxX>>chunk.Shift; cx++ {
				g.recountClearChunk(cy*cg.CW + cx)
			}
		}
	}
}

// recountClearChunk recounts chunk c's area cells deep enough to clear.
func (g *Game) recountClearChunk(c int) {
	s := &g.snow
	if c < 0 || c >= len(s.clearN) {
		return
	}
	s.clearN[c] = 0
	if s.clear == nil || s.depth == nil || s.chunkSum[c] == 0 {
		return
	}
	x0, y0, x1, y1 := g.snowChunkGrid().Bounds(c)
	w := g.grid.Width()
	n := uint16(0)
	for y := y0; y < y1; y++ {
		for i := y*w + x0; i < y*w+x1; i++ {
			if s.depth[i] >= snowClearStart && g.inSnowClear(i) {
				n++
			}
		}
	}
	s.clearN[c] = n
}

// snowClearWork is the labour to shovel snow of depth d: snowClearMinWork
// for a dusting up to snowClearMaxWork for the deepest.
func snowClearWork(d uint8) int {
	return snowClearMinWork + int(d)*(snowClearMaxWork-snowClearMinWork)/255
}

// assignClearSnow gives colonist id the nearest unreserved, reachable area
// cell with snow to clear inside its allowed area (ties by chunk, then
// index); chunks are visited nearest first and only while they can still
// beat the candidates found.
func (g *Game) assignClearSnow(id entity.EntityID) bool {
	s := &g.snow
	if s.depth == nil || s.clear == nil {
		return false
	}
	cg := g.snowChunkGrid()
	pos, _ := g.positions.Get(id)
	type near struct{ d, c int }
	var chunks []near
	for c, n := range s.clearN {
		if n == 0 {
			continue
		}
		x0, y0, x1, y1 := cg.Bounds(c)
		dx := max(x0-pos.X, 0, pos.X-(x1-1))
		dy := max(y0-pos.Y, 0, pos.Y-(y1-1))
		chunks = append(chunks, near{max(dx, dy), c})
	}
	if len(chunks) == 0 {
		return false
	}
	sort.Slice(chunks, func(i, j int) bool {
		return chunks[i].d < chunks[j].d || chunks[i].d == chunks[j].d && chunks[i].c < chunks[j].c
	})
	type cand struct{ d, i int }
	var best [snowClearProbes]cand
	n := 0
	w := g.grid.Width()
	for _, ch := range chunks {
		if n == len(best) && ch.d >= best[n-1].d {
			break // no cell of this chunk or a farther one can win
		}
		x0, y0, x1, y1 := cg.Bounds(ch.c)
		for y := y0; y < y1; y++ {
			for x := x0; x < x1; x++ {
				i := y*w + x
				if s.depth[i] < snowClearStart || !g.inSnowClear(i) {
					continue
				}
				d := pathing.Dist(pos.X, pos.Y, x, y)
				if n == len(best) && d >= best[n-1].d {
					continue
				}
				if _, taken := g.reserve.TileOwner(x, y); taken {
					continue
				}
				if !g.inArea(id, x, y) || g.postponedFor(id, job.ClearSnow, 0, x, y) {
					continue
				}
				if n < len(best) {
					n++
				}
				k := n - 1
				for k > 0 && best[k-1].d > d {
					best[k] = best[k-1]
					k--
				}
				best[k] = cand{d, i}
			}
		}
	}
	for _, c := range best[:n] {
		x, y := c.i%w, c.i/w
		tx, ty, ok := g.reachableSpot(id, x, y)
		if !ok {
			g.postponeTile(id, job.ClearSnow, x, y)
			continue
		}
		work := snowClearWork(s.depth[c.i])
		if !g.startJob(id, job.Record{Kind: job.ClearSnow, TX: tx, TY: ty, DestX: x, DestY: y, WorkLeft: work, Count: work}) {
			return false
		}
		g.reserve.TryTile(id, x, y)
		return true
	}
	return false
}

// doClearSnow shovels the job's cell: once the work is done it is bare.
func (g *Game) doClearSnow(id entity.EntityID, j job.Record) {
	i := j.DestY*g.grid.Width() + j.DestX
	switch {
	case !g.grid.InBounds(j.DestX, j.DestY) || !g.inSnowClear(i):
		j.State, j.Fail = job.Fail, job.TargetGone
	case g.snowAt(j.DestX, j.DestY) == 0 && j.WorkLeft >= j.Count:
		j.State, j.Fail = job.Fail, job.TargetGone // melted before it was started
	case g.snowAt(j.DestX, j.DestY) == 0:
		j.State = job.Complete
	}
	if j.State != job.Do {
		g.jobs.Set(id, j)
		return
	}
	u := g.workProgress(id, skNone, false) // skills.go
	if u == 0 {
		g.jobs.Set(id, j)
		return
	}
	j.WorkLeft -= u
	if j.WorkLeft <= 0 {
		g.setSnowCell(i, 0) // snow.go
		j.State = job.Complete
	}
	g.jobs.Set(id, j)
}

// fillSnowView copies the snow layer and the clearing area into the view:
// slices once handed out are never written again; new ones are built when
// they changed. The versions are content hashes (0: no layer), so a loaded
// game reads the same versions as the game it was saved from. While snow
// falls or melts the layer changes every tick (a few chunks each), and so
// is copied once per World().
func (g *Game) fillSnowView(w *WorldView) {
	s := &g.snow
	if s.depth != nil {
		if !s.viewOK || s.viewVer != s.ver {
			s.view, s.viewVer, s.viewOK = slices.Clone(s.depth), s.ver, true
			s.viewFP = layerHash(s.view)
		}
		w.Snow, w.SnowVer = s.view, s.viewFP
	}
	if s.clear != nil {
		if !s.clearViewOK || s.clearViewVer != s.clearVer {
			n := g.grid.Width() * g.grid.Height()
			v := make([]uint8, n)
			for i := range n {
				if g.inSnowClear(i) {
					v[i] = 1
				}
			}
			s.clearView, s.clearViewVer, s.clearViewOK = v, s.clearVer, true
			s.clearFP = layerHash(v)
		}
		w.SnowClear, w.SnowClearVer = s.clearView, s.clearFP
	}
}

// layerHash is a content hash of a view layer, never 0.
func layerHash(b []uint8) uint64 {
	h := uint64(0xcbf29ce484222325)
	for len(b) >= 8 {
		h = (h ^ binary.LittleEndian.Uint64(b)) * 0x100000001b3
		h ^= h >> 29
		b = b[8:]
	}
	for _, v := range b {
		h = (h ^ uint64(v)) * 0x100000001b3
	}
	return h | 1
}

// exportSnow is the saved layer (nil: none lying) and clearing area (nil:
// none painted).
func (g *Game) exportSnow() ([]byte, []uint64) {
	return slices.Clone(g.snow.depth), slices.Clone(g.snow.clear)
}

// importSnow restores them after the weather. A missing or malformed
// layer (an older save) leaves it nil: the saved global depth reads on
// every open cell and seeds the layer at the next climate step.
func (g *Game) importSnow(depth []byte, clear []uint64) {
	old := g.snow
	g.snow = snowState{ver: old.ver + 1, clearVer: old.clearVer + 1} // view caches rebuild
	n := g.grid.Width() * g.grid.Height()
	if len(clear) == (n+63)/64 {
		g.snow.clear = slices.Clone(clear)
	}
	if len(depth) == n {
		g.snow.depth = slices.Clone(depth)
		g.snowRecount()
	}
}
