package light

import "strconv"

// FlushReason is why the pending lit quads went out as one call (GAR-161).
type FlushReason uint8

const (
	FlushDst      FlushReason = iota // the next quad draws to another destination
	FlushPage                        // the next quad samples another sheet page, or the sheet starts again
	FlushShader                      // the next quad uses another shader
	FlushUniforms                    // the frame's light changed
	FlushFull                        // the batch reached its quad cap
	FlushRaw                         // a draw outside the batch: a raw sink's Flush, or a lit draw that cannot join
	FlushBlood                       // a bloodied figure, drawn alone (its noise reads absolute source positions)
	FlushBake                        // a bake's plain albedo draw
	FlushOff                         // lighting off or no shader: the plain draw
	FlushEnd                         // the batch was turned off (the pass ended, or a caller draws past the light)
	FlushNew                         // a lit draw whose image has no copy on the sheet yet draws alone (copied at the next StartFrame)
	NumFlushReasons
)

var flushNames = [NumFlushReasons]string{"dst", "page", "shader", "uniforms", "full", "raw", "blood", "bake", "off", "end", "new"}

// String is the reason's short name ("raw").
func (r FlushReason) String() string {
	if r < NumFlushReasons {
		return flushNames[r]
	}
	return "reason(" + strconv.Itoa(int(r)) + ")"
}

// Stats are one frame's lit-draw counts: the calls Lit issued itself (one
// per unbatched lit sprite, plain draw or batch), the quads that went out in
// a batch, the batch flushes by reason, and the images copied onto the
// batch sheet and the times it started again. StartFrame zeroes them
// (and counts the copies it makes into the new frame's); the
// PERF probe and the draw benchmarks read them (Lit.Stats).
type Stats struct {
	Calls   int
	Batched int
	Flushes [NumFlushReasons]int
	Copies  int
	Resets  int
}

// String is the probe line's form: "calls 120 batched 900 copies 3 resets 0
// flush dst=2 page=1 …" (reasons with no flush are left out).
func (s Stats) String() string {
	b := make([]byte, 0, 128)
	b = append(b, "calls "...)
	b = strconv.AppendInt(b, int64(s.Calls), 10)
	b = append(b, " batched "...)
	b = strconv.AppendInt(b, int64(s.Batched), 10)
	b = append(b, " copies "...)
	b = strconv.AppendInt(b, int64(s.Copies), 10)
	b = append(b, " resets "...)
	b = strconv.AppendInt(b, int64(s.Resets), 10)
	b = append(b, " flush"...)
	for r, n := range s.Flushes {
		if n > 0 {
			b = append(b, ' ')
			b = append(b, flushNames[r]...)
			b = append(b, '=')
			b = strconv.AppendInt(b, int64(n), 10)
		}
	}
	return string(b)
}

// Stats is this frame's lit-draw counts so far.
func (l *Lit) Stats() Stats { return l.stats }

// StartFrame begins a frame's lit drawing: the pipeline calls it once at
// the top of the world pass. It zeroes Stats, then makes the sheet copies
// the last frame asked for (counted in this frame's Stats), so no copy
// lands between two draws of the frame (lit_batch.go).
func (l *Lit) StartFrame() {
	prev := l.stats
	l.stats = Stats{}
	l.copyAsked(prev)
}

// count records one draw call Lit issued (Calls, the benchmarks' total, and
// this frame's Stats).
func (l *Lit) count() {
	l.Calls++
	l.stats.Calls++
}
