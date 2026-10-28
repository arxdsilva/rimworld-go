package ebiten

import (
	"runtime/metrics"
	"slices"
	"time"
)

// perfClock is the always-on frame timing behind the performance overlay
// (PERF-002, perf_overlay.go): a rolling window of frame gaps, Update and
// Draw times, summarised a couple of times a second. Wall-clock time is
// presentation-only and never reaches the simulation.
type perfClock struct {
	ring      perfRing
	upStart   time.Time
	lastDraw  time.Time
	frozen    bool // a capture: no new samples, fixed heap and sim rate
	computed  time.Time
	heapAt    time.Time
	heapMiB   int
	gcCycles  uint64
	gcAt      time.Time
	gcPerMin  float64
	samples   [2]metrics.Sample
	samplesOK bool
	stats     perfStats
	lines     perfLines
	spark     []int // the newest frame times in microseconds, oldest first
	sparkBuf  []time.Duration
	boxScale  float64 // the UI scale boxW/boxH were measured at
	boxW      float32
	boxH      float32
}

// perfRingLen is the window: about five seconds at 120 fps.
const perfRingLen = 600

// perfSample is one frame's timings.
type perfSample struct{ frame, update, draw time.Duration }

// perfRing keeps the last perfRingLen frame gaps, Updates and Draws. An
// Update and a Draw are not always paired (a held-back frame), so the
// three series run independently.
type perfRing struct {
	frame, update, draw perfSeries
}

type perfSeries struct {
	v    [perfRingLen]time.Duration
	n    int // samples held (up to perfRingLen)
	head int // next slot to write
}

func (s *perfSeries) push(d time.Duration) {
	s.v[s.head] = d
	s.head = (s.head + 1) % perfRingLen
	s.n = min(s.n+1, perfRingLen)
}

// last copies the newest k samples into dst, oldest first.
func (s *perfSeries) last(k int, dst []time.Duration) []time.Duration {
	k = min(k, s.n)
	dst = dst[:0]
	for i := k; i > 0; i-- {
		dst = append(dst, s.v[(s.head-i+perfRingLen)%perfRingLen])
	}
	return dst
}

// push adds one sample's non-zero parts.
func (r *perfRing) push(p perfSample) {
	if p.frame > 0 {
		r.frame.push(p.frame)
	}
	if p.update > 0 {
		r.update.push(p.update)
	}
	if p.draw > 0 {
		r.draw.push(p.draw)
	}
}

// perfStats is the summary the overlay shows (and the fps probe prints).
type perfStats struct {
	FPS, Low1            float64
	FrameAvg, FrameP99   time.Duration
	UpdateAvg, UpdateP99 time.Duration
	DrawAvg, DrawP99     time.Duration
	TicksPerUpdate       float64
	Held                 int
	HeapMiB              int
	GCPerMin             float64
}

// percentile is the q quantile of d (0 for none), q in [0,1).
func percentile(d []time.Duration, q float64) time.Duration {
	if len(d) == 0 {
		return 0
	}
	s := slices.Clone(d)
	slices.Sort(s)
	return s[min(len(s)-1, int(float64(len(s))*q))]
}

func avgDur(d []time.Duration) time.Duration {
	if len(d) == 0 {
		return 0
	}
	var t time.Duration
	for _, x := range d {
		t += x
	}
	return t / time.Duration(len(d))
}

// frameStats summarises frame gaps and Update and Draw times: fps from the
// mean gap, the 1 % low from the 99th percentile gap.
func frameStats(frames, update, draw []time.Duration) perfStats {
	st := perfStats{
		FrameAvg: avgDur(frames), FrameP99: percentile(frames, 0.99),
		UpdateAvg: avgDur(update), UpdateP99: percentile(update, 0.99),
		DrawAvg: avgDur(draw), DrawP99: percentile(draw, 0.99),
	}
	if st.FrameAvg > 0 {
		st.FPS = float64(time.Second) / float64(st.FrameAvg)
	}
	if st.FrameP99 > 0 {
		st.Low1 = float64(time.Second) / float64(st.FrameP99)
	}
	return st
}

// perfBeginUpdate marks the start of an Update.
func (a *App) perfBeginUpdate() {
	if !a.perf.frozen {
		a.perf.upStart = a.now()
	}
}

// perfEndUpdate records the Update's time.
func (a *App) perfEndUpdate() {
	p := &a.perf
	if p.frozen || p.upStart.IsZero() {
		return
	}
	p.ring.update.push(max(1, a.now().Sub(p.upStart)))
}

// perfEndDraw records a Draw that began at start and the gap since the
// last one. It runs before the background throttle sleeps.
func (a *App) perfEndDraw(start time.Time) {
	p := &a.perf
	if p.frozen {
		return
	}
	now := a.now()
	p.ring.draw.push(max(1, now.Sub(start)))
	if !p.lastDraw.IsZero() {
		if gap := now.Sub(p.lastDraw); gap > 0 {
			p.ring.frame.push(gap)
		}
	}
	p.lastDraw = now
}

// perfRefresh recomputes the stats and the overlay's strings at most twice
// a second (and the heap at most once a second).
func (a *App) perfRefresh(now time.Time) {
	p := &a.perf
	if !p.computed.IsZero() && now.Sub(p.computed) < 500*time.Millisecond && now.After(p.computed) {
		return
	}
	p.computed = now
	r := &p.ring
	var fr, up, dr []time.Duration
	fr = r.frame.last(perfRingLen, nil)
	up = r.update.last(perfRingLen, nil)
	dr = r.draw.last(perfRingLen, nil)
	st := frameStats(fr, up, dr)
	if !p.frozen {
		if a.renderer != nil {
			st.TicksPerUpdate = float64(a.renderer.mo.Stepped)
		}
		st.Held = a.debt.held
		a.perfHeap(now)
		st.HeapMiB, st.GCPerMin = p.heapMiB, p.gcPerMin
	} else {
		st.TicksPerUpdate, st.Held, st.HeapMiB, st.GCPerMin = 3, a.debt.held, 412, 12
	}
	p.stats = st
	p.sparkBuf = r.frame.last(perfSparkLen, p.sparkBuf)
	p.spark = p.spark[:0]
	for _, d := range p.sparkBuf {
		p.spark = append(p.spark, int(d/time.Microsecond))
	}
	p.lines = st.format()
}

// perfHeap reads the heap and the GC rate from runtime/metrics at most once
// a second (never runtime.ReadMemStats, which stops the world).
func (a *App) perfHeap(now time.Time) {
	p := &a.perf
	if !p.heapAt.IsZero() && now.Sub(p.heapAt) < time.Second && now.After(p.heapAt) {
		return
	}
	if !p.samplesOK {
		p.samples[0].Name = "/memory/classes/heap/objects:bytes"
		p.samples[1].Name = "/gc/cycles/total:gc-cycles"
		p.samplesOK = true
	}
	metrics.Read(p.samples[:])
	if p.samples[0].Value.Kind() == metrics.KindUint64 {
		p.heapMiB = int(p.samples[0].Value.Uint64() >> 20)
	}
	if p.samples[1].Value.Kind() == metrics.KindUint64 {
		c := p.samples[1].Value.Uint64()
		if !p.gcAt.IsZero() && now.After(p.gcAt) && c >= p.gcCycles {
			// a smoothed per-minute rate over the span since the last read
			rate := float64(c-p.gcCycles) / now.Sub(p.gcAt).Minutes()
			p.gcPerMin = 0.5*p.gcPerMin + 0.5*rate
		}
		p.gcCycles, p.gcAt = c, now
	}
	p.heapAt = now
}
