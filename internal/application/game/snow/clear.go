package snow

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
// ClearStart deep, down to bare ground (job ClearSnow), so a cleared
// cell is not taken up again until that much has fallen anew
// (hysteresis). Candidates come from per-chunk counts kept by the snow
// passes, never from a map scan.

const (
	// ClearStart is 15%: shallower snow is left alone.
	ClearStart = 38
	// ClearMinWork is the work ticks for the shallowest snow ...
	ClearMinWork = 20
	// ClearMaxWork ... and for the deepest.
	ClearMaxWork = 80
	clearProbes  = 16 // reachability probes per assignment, nearest first
)

// InClear reports cell i is in the snow clearing area.
func (l Layer) InClear(i int) bool {
	c := l.s.clear
	return i >= 0 && i>>6 < len(c) && c[i>>6]&(1<<(uint(i)&63)) != 0
}

// PaintClear adds (on) or clears the snow clearing area over the
// rectangle.
func (l Layer) PaintClear(x, y, x2, y2 int, on bool) {
	minX, minY, maxX, maxY := zone.Normalize(x, y, x2, y2)
	w, h := l.h.Size()
	minX, minY = max(0, minX), max(0, minY)
	maxX, maxY = min(w-1, maxX), min(h-1, maxY)
	if minX > maxX || minY > maxY {
		return
	}
	s := l.s
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
		cg := l.chunkGrid()
		for cy := minY >> chunk.Shift; cy <= maxY>>chunk.Shift; cy++ {
			for cx := minX >> chunk.Shift; cx <= maxX>>chunk.Shift; cx++ {
				l.recountClearChunk(cy*cg.CW + cx)
			}
		}
	}
}

// recountClearChunk recounts chunk c's area cells deep enough to clear.
func (l Layer) recountClearChunk(c int) {
	s := l.s
	if c < 0 || c >= len(s.clearN) {
		return
	}
	s.clearN[c] = 0
	if s.clear == nil || s.depth == nil || s.chunkSum[c] == 0 {
		return
	}
	x0, y0, x1, y1 := l.chunkGrid().Bounds(c)
	w, _ := l.h.Size()
	n := uint16(0)
	for y := y0; y < y1; y++ {
		for i := y*w + x0; i < y*w+x1; i++ {
			if s.depth[i] >= ClearStart && l.InClear(i) {
				n++
			}
		}
	}
	s.clearN[c] = n
}

// ClearWork is the labour to shovel snow of depth d: ClearMinWork
// for a dusting up to ClearMaxWork for the deepest.
func ClearWork(d uint8) int {
	return ClearMinWork + int(d)*(ClearMaxWork-ClearMinWork)/255
}

// AssignClear gives colonist id the nearest unreserved, reachable area
// cell with snow to clear inside its allowed area (ties by chunk, then
// index); chunks are visited nearest first and only while they can still
// beat the candidates found.
func (l Layer) AssignClear(id entity.EntityID) bool {
	s := l.s
	if s.depth == nil || s.clear == nil {
		return false
	}
	cg := l.chunkGrid()
	px, py := l.h.Pos(id)
	type near struct{ d, c int }
	var chunks []near
	for c, n := range s.clearN {
		if n == 0 {
			continue
		}
		x0, y0, x1, y1 := cg.Bounds(c)
		dx := max(x0-px, 0, px-(x1-1))
		dy := max(y0-py, 0, py-(y1-1))
		chunks = append(chunks, near{max(dx, dy), c})
	}
	if len(chunks) == 0 {
		return false
	}
	sort.Slice(chunks, func(i, j int) bool {
		return chunks[i].d < chunks[j].d || chunks[i].d == chunks[j].d && chunks[i].c < chunks[j].c
	})
	type cand struct{ d, i int }
	var best [clearProbes]cand
	n := 0
	w, _ := l.h.Size()
	for _, ch := range chunks {
		if n == len(best) && ch.d >= best[n-1].d {
			break // no cell of this chunk or a farther one can win
		}
		x0, y0, x1, y1 := cg.Bounds(ch.c)
		for y := y0; y < y1; y++ {
			for x := x0; x < x1; x++ {
				i := y*w + x
				if s.depth[i] < ClearStart || !l.InClear(i) {
					continue
				}
				d := pathing.Dist(px, py, x, y)
				if n == len(best) && d >= best[n-1].d {
					continue
				}
				if l.h.TileTaken(x, y) {
					continue
				}
				if !l.h.Allowed(id, x, y) || l.h.PostponedFor(id, job.ClearSnow, x, y) {
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
		tx, ty, ok := l.h.ReachableSpot(id, x, y)
		if !ok {
			l.h.PostponeTile(id, job.ClearSnow, x, y)
			continue
		}
		work := ClearWork(s.depth[c.i])
		if !l.h.StartJob(id, job.Record{Kind: job.ClearSnow, TX: tx, TY: ty, DestX: x, DestY: y, WorkLeft: work, Count: work}) {
			return false
		}
		l.h.ClaimTile(id, x, y)
		return true
	}
	return false
}

// DoClear shovels the job's cell: once the work is done it is bare.
func (l Layer) DoClear(id entity.EntityID, j job.Record) {
	mw, mh := l.h.Size()
	i := j.DestY*mw + j.DestX
	switch {
	case j.DestX < 0 || j.DestX >= mw || j.DestY < 0 || j.DestY >= mh || !l.InClear(i):
		j.State, j.Fail = job.Fail, job.TargetGone
	case l.At(j.DestX, j.DestY) == 0 && j.WorkLeft >= j.Count:
		j.State, j.Fail = job.Fail, job.TargetGone // melted before it was started
	case l.At(j.DestX, j.DestY) == 0:
		j.State = job.Complete
	}
	if j.State != job.Do {
		l.h.SetJob(id, j)
		return
	}
	u := l.h.Work(id)
	if u == 0 {
		l.h.SetJob(id, j)
		return
	}
	j.WorkLeft -= u
	if j.WorkLeft <= 0 {
		l.SetCell(i, 0) // snow.go
		j.State = job.Complete
	}
	l.h.SetJob(id, j)
}

// View is the layer and the clearing area for the read model: slices once
// handed out are never written again. Nil: no layer, no area. The versions
// are content hashes (0: no layer), so a loaded game reads the same
// versions as the game it was saved from.
type View struct {
	Snow     []uint8
	SnowVer  uint64
	Clear    []uint8
	ClearVer uint64
}

// View copies the snow layer and the clearing area into a view: new
// slices are built when they changed. While snow falls or melts the layer
// changes every tick (a few chunks each), and so is copied once per
// World().
func (l Layer) View() View {
	var v View
	s := l.s
	if s.depth != nil {
		if !s.viewOK || s.viewVer != s.ver {
			s.view, s.viewVer, s.viewOK = slices.Clone(s.depth), s.ver, true
			s.viewFP = layerHash(s.view)
		}
		v.Snow, v.SnowVer = s.view, s.viewFP
	}
	if s.clear != nil {
		if !s.clearViewOK || s.clearViewVer != s.clearVer {
			w, h := l.h.Size()
			n := w * h
			c := make([]uint8, n)
			for i := range n {
				if l.InClear(i) {
					c[i] = 1
				}
			}
			s.clearView, s.clearViewVer, s.clearViewOK = c, s.clearVer, true
			s.clearFP = layerHash(c)
		}
		v.Clear, v.ClearVer = s.clearView, s.clearFP
	}
	return v
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

// Export is the saved layer (nil: none lying) and clearing area (nil:
// none painted).
func (l Layer) Export() ([]byte, []uint64) {
	return slices.Clone(l.s.depth), slices.Clone(l.s.clear)
}

// Import restores them after the weather. A missing or malformed
// layer (an older save) leaves it nil: the saved global depth reads on
// every open cell and seeds the layer at the next climate step.
func (l Layer) Import(depth []byte, clear []uint64) {
	old := *l.s
	*l.s = State{ver: old.ver + 1, clearVer: old.clearVer + 1} // view caches rebuild
	w, h := l.h.Size()
	n := w * h
	if len(clear) == (n+63)/64 {
		l.s.clear = slices.Clone(clear)
	}
	if len(depth) == n {
		l.s.depth = slices.Clone(depth)
		l.recount()
	}
}
