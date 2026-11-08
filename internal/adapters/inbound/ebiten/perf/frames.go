package perf

import (
	"fmt"
	"math"
	"runtime/metrics"
	"strings"
	"time"
)

// The "always 60" numbers (GAR, PERF-004). At 60 Hz with VSync on a frame
// is 16.67 ms; a gap over Late missed one vsync, a gap over Hitch missed
// two or more. Gaps are measured at the CPU (Draw to Draw), so a frame on
// time can read a little over 16.7 ms: Late allows 20%.
const (
	Vsync60 = 16_667 * time.Microsecond
	Late    = 20 * time.Millisecond
	Hitch   = 33_333 * time.Microsecond
)

// HistBuckets is the frame histogram's size: 1 ms buckets from 0 to 49 ms
// and one for 50 ms and over.
const HistBuckets = 51

// Hist counts frame gaps without allocating.
type Hist struct {
	Bucket [HistBuckets]int
	N      int
	Over   int // gaps over Vsync60
	Late   int // gaps over Late
	Hitch  int // gaps over Hitch
	Max    time.Duration
}

// Add records one gap.
func (h *Hist) Add(d time.Duration) {
	h.N++
	b := int(d / time.Millisecond)
	h.Bucket[min(max(b, 0), HistBuckets-1)]++
	if d > Vsync60 {
		h.Over++
	}
	if d > Late {
		h.Late++
	}
	if d > Hitch {
		h.Hitch++
	}
	h.Max = max(h.Max, d)
}

// Line is the histogram as "ms:count" pairs for the non-empty buckets
// ("50+" for the last).
func (h *Hist) Line() string {
	var b strings.Builder
	for i, n := range h.Bucket {
		if n == 0 {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		if i == HistBuckets-1 {
			fmt.Fprintf(&b, "50+:%d", n)
		} else {
			fmt.Fprintf(&b, "%d:%d", i, n)
		}
	}
	return b.String()
}

// UpdatesPerDraw counts how many Updates ran before each Draw: [0], [1],
// [2] and [3] for three or more (Ebitengine runs 0, 1 or 2 at 60 TPS on a
// 60 Hz display; 2 means the clock caught up, 0 a frame with no tick).
type UpdatesPerDraw struct {
	Count   [4]int
	pending int
}

// Update notes one Update.
func (u *UpdatesPerDraw) Update() { u.pending++ }

// Draw closes the frame.
func (u *UpdatesPerDraw) Draw() {
	u.Count[min(u.pending, 3)]++
	u.pending = 0
}

// gcNames are the runtime/metrics the probe reads (Go 1.27).
var gcNames = [...]string{
	"/gc/cycles/total:gc-cycles",
	"/cpu/classes/gc/total:cpu-seconds",
	"/gc/heap/allocs:objects",
	"/gc/heap/live:bytes",
	"/sched/pauses/total/gc:seconds",
}

// GC is the runtime's garbage collector over the measured span: cycles,
// GC CPU, allocations, the live heap at the end and the longest
// stop-the-world pause (a histogram bucket's upper bound).
type GC struct {
	Cycles   uint64
	CPU      time.Duration
	Allocs   uint64
	LiveMiB  uint64
	MaxPause time.Duration
}

// GCWindow samples the runtime at the start and the end of a span.
type GCWindow struct {
	start [len(gcNames)]metrics.Sample
	end   [len(gcNames)]metrics.Sample
	begun bool
}

func readGC(s *[len(gcNames)]metrics.Sample) {
	for i, n := range gcNames {
		s[i].Name = n
	}
	metrics.Read(s[:])
}

// Begin samples the span's start.
func (w *GCWindow) Begin() {
	readGC(&w.start)
	w.begun = true
}

// End samples the span's end and returns the difference (zero if Begin
// never ran).
func (w *GCWindow) End() GC {
	if !w.begun {
		return GC{}
	}
	readGC(&w.end)
	u := func(i int) uint64 {
		if w.end[i].Value.Kind() != metrics.KindUint64 || w.start[i].Value.Kind() != metrics.KindUint64 {
			return 0
		}
		return w.end[i].Value.Uint64() - w.start[i].Value.Uint64()
	}
	g := GC{Cycles: u(0), Allocs: u(2)}
	if w.end[1].Value.Kind() == metrics.KindFloat64 && w.start[1].Value.Kind() == metrics.KindFloat64 {
		g.CPU = time.Duration((w.end[1].Value.Float64() - w.start[1].Value.Float64()) * float64(time.Second))
	}
	if w.end[3].Value.Kind() == metrics.KindUint64 {
		g.LiveMiB = w.end[3].Value.Uint64() >> 20
	}
	if w.end[4].Value.Kind() == metrics.KindFloat64Histogram && w.start[4].Value.Kind() == metrics.KindFloat64Histogram {
		g.MaxPause = maxNewPause(w.start[4].Value.Float64Histogram(), w.end[4].Value.Float64Histogram())
	}
	return g
}

// maxNewPause is the upper bound of the highest bucket that gained a count
// between two snapshots of one histogram.
func maxNewPause(a, b *metrics.Float64Histogram) time.Duration {
	if len(a.Counts) != len(b.Counts) {
		return 0
	}
	for i := len(b.Counts) - 1; i >= 0; i-- {
		if b.Counts[i] > a.Counts[i] {
			hi := b.Buckets[i+1]
			if math.IsInf(hi, 1) {
				hi = b.Buckets[i]
			}
			return time.Duration(hi * float64(time.Second))
		}
	}
	return 0
}

// low is the frame rate at the q quantile of gaps (the 1% low is q 0.99,
// the 0.1% low q 0.999).
func low(frames []time.Duration, q float64) float64 {
	if p := percentile(frames, q); p > 0 {
		return float64(time.Second) / float64(p)
	}
	return 0
}

// Detail is the probe's second report line, key=value pairs the probe
// matrix (scripts/fpsmatrix.sh) parses.
func Detail(st *Probe, gc GC) string {
	h := &st.hist
	frames := max(h.N, 1)
	return fmt.Sprintf("fps probe detail: frames=%d over16_7=%d late20=%d hitch33=%d frame_max=%v low1=%.1f low01=%.1f update_max=%v draw_max=%v upd_per_draw=%d/%d/%d/%d gc_cycles=%d gc_cpu=%v gc_max_pause=%v allocs_per_frame=%d live_heap_mib=%d hist=[%s]",
		h.N, h.Over, h.Late, h.Hitch, h.Max, low(st.frames, 0.99), low(st.frames, 0.999),
		maxDur(st.update), maxDur(st.draw),
		st.upd.Count[0], st.upd.Count[1], st.upd.Count[2], st.upd.Count[3],
		gc.Cycles, gc.CPU, gc.MaxPause, gc.Allocs/uint64(frames), gc.LiveMiB, h.Line())
}

func maxDur(d []time.Duration) time.Duration {
	var m time.Duration
	for _, x := range d {
		m = max(m, x)
	}
	return m
}
