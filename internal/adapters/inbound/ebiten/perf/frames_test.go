package perf

import (
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestProbeHistogramAndLows(t *testing.T) {
	t.Parallel()
	var h Hist
	ms := func(f float64) time.Duration { return time.Duration(f * float64(time.Millisecond)) }
	for range 990 {
		h.Add(ms(16.6))
	}
	for range 8 {
		h.Add(ms(17.5)) // over 16.7, on time within the 20% allowance
	}
	h.Add(ms(25)) // one missed vsync
	h.Add(ms(70)) // a hitch, in the overflow bucket
	if h.N != 1000 || h.Bucket[16] != 990 || h.Bucket[17] != 8 || h.Bucket[25] != 1 || h.Bucket[HistBuckets-1] != 1 {
		t.Fatalf("buckets %v n %d", h.Bucket, h.N)
	}
	if h.Over != 10 || h.Late != 2 || h.Hitch != 1 || h.Max != ms(70) {
		t.Fatalf("over %d late %d hitch %d max %v", h.Over, h.Late, h.Hitch, h.Max)
	}
	if got := h.Line(); got != "16:990 17:8 25:1 50+:1" {
		t.Fatalf("line %q", got)
	}

	frames := make([]time.Duration, 0, 1000)
	for range 990 {
		frames = append(frames, ms(16.6))
	}
	for range 10 {
		frames = append(frames, ms(40))
	}
	// The 99th percentile gap is the first slow one: the 1% low is 25 fps;
	// the 0.1% low reads the same slow tail.
	if l := low(frames, 0.99); l < 24.9 || l > 25.1 {
		t.Fatalf("1%% low %.2f", l)
	}
	if l := low(frames, 0.999); l < 24.9 || l > 25.1 {
		t.Fatalf("0.1%% low %.2f", l)
	}
	if low(nil, 0.99) != 0 {
		t.Fatal("no frames, no low")
	}
}

func TestProbeUpdatesPerDraw(t *testing.T) {
	t.Parallel()
	var u UpdatesPerDraw
	u.Draw() // a frame with no Update
	u.Update()
	u.Draw()
	u.Update()
	u.Update()
	u.Draw() // the clock caught up
	for range 5 {
		u.Update()
	}
	u.Draw()
	if u.Count != [4]int{1, 1, 1, 1} {
		t.Fatalf("counts %v", u.Count)
	}
}

// The per-frame bookkeeping allocates nothing.
func TestProbeAllocFree(t *testing.T) {
	var h Hist
	var u UpdatesPerDraw
	if n := testing.AllocsPerRun(1000, func() {
		u.Update()
		h.Add(17 * time.Millisecond)
		u.Draw()
	}); n != 0 {
		t.Fatalf("%v allocations per frame", n)
	}
}

func TestGCWindowSeesACollection(t *testing.T) {
	var w GCWindow
	if (w.End() != GC{}) {
		t.Fatal("End before Begin is zero")
	}
	w.Begin()
	sink := make([][]byte, 0, 64)
	for range 64 {
		sink = append(sink, make([]byte, 1<<10))
	}
	runtime.KeepAlive(sink)
	runtime.GC()
	g := w.End()
	if g.Cycles < 1 || g.Allocs < 64 || g.CPU <= 0 || g.MaxPause <= 0 {
		t.Fatalf("gc %+v", g)
	}
}

func TestDetailLineHasTheMatrixKeys(t *testing.T) {
	t.Parallel()
	st := &Probe{frames: []time.Duration{16 * time.Millisecond, 40 * time.Millisecond}, update: []time.Duration{time.Millisecond}, draw: []time.Duration{2 * time.Millisecond}}
	st.hist.Add(16 * time.Millisecond)
	st.hist.Add(40 * time.Millisecond)
	st.upd.Update()
	st.upd.Draw()
	line := Detail(st, GC{Cycles: 2, Allocs: 400})
	for _, k := range []string{"fps probe detail:", "frames=2", "hitch33=1", "late20=1", "upd_per_draw=0/1/0/0", "gc_cycles=2", "allocs_per_frame=200", "hist=[16:1 40:1]", "update_max=1ms", "draw_max=2ms"} {
		if !strings.Contains(line, k) {
			t.Errorf("detail %q lacks %q", line, k)
		}
	}
}
