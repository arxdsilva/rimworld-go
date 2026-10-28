package perf

import (
	"runtime/metrics"
	"slices"
	"time"
)

// Clock is the always-on frame timing behind the performance overlay
// (PERF-002, overlay.go): a rolling window of frame gaps, Update and Draw
// times, summarised a couple of times a second. Wall-clock time is
// presentation-only and never reaches the simulation.
type Clock struct {
	ring      Ring
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
	stats     Stats
	Lines     Lines
	Spark     []int // the newest frame times in microseconds, oldest first
	sparkBuf  []time.Duration
	boxScale  float64 // the UI scale boxW/boxH were measured at
	boxW      float32
	boxH      float32
}

// RingLen is the window: about five seconds at 120 fps.
const RingLen = 600

// Sample is one frame's timings.
type Sample struct{ Frame, Update, Draw time.Duration }

// Ring keeps the last RingLen frame gaps, Updates and Draws. An Update and
// a Draw are not always paired (a held-back frame), so the three series
// run independently.
type Ring struct {
	Frame, Update, Draw Series
}

// Series is one ring of durations.
type Series struct {
	v    [RingLen]time.Duration
	N    int // samples held (up to RingLen)
	head int // next slot to write
}

// Push adds one duration.
func (s *Series) Push(d time.Duration) {
	s.v[s.head] = d
	s.head = (s.head + 1) % RingLen
	s.N = min(s.N+1, RingLen)
}

// Last copies the newest k samples into dst, oldest first.
func (s *Series) Last(k int, dst []time.Duration) []time.Duration {
	k = min(k, s.N)
	dst = dst[:0]
	for i := k; i > 0; i-- {
		dst = append(dst, s.v[(s.head-i+RingLen)%RingLen])
	}
	return dst
}

// Push adds one sample's non-zero parts.
func (r *Ring) Push(p Sample) {
	if p.Frame > 0 {
		r.Frame.Push(p.Frame)
	}
	if p.Update > 0 {
		r.Update.Push(p.Update)
	}
	if p.Draw > 0 {
		r.Draw.Push(p.Draw)
	}
}

// Stats is the summary the overlay shows (and the fps probe prints).
type Stats struct {
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

// FrameStats summarises frame gaps and Update and Draw times: fps from the
// mean gap, the 1 % low from the 99th percentile gap.
func FrameStats(frames, update, draw []time.Duration) Stats {
	st := Stats{
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

// BeginUpdate marks the start of an Update at now.
func (m *Meter) BeginUpdate(now time.Time) {
	if !m.Clock.frozen {
		m.Clock.upStart = now
	}
}

// EndUpdate records the Update's time (it ends at now).
func (m *Meter) EndUpdate(now time.Time) {
	c := m.Clock
	if c.frozen || c.upStart.IsZero() {
		return
	}
	c.ring.Update.Push(max(1, now.Sub(c.upStart)))
}

// EndDraw records a Draw that began at start and ends at now, and the gap
// since the last one. It runs before the background throttle sleeps.
func (m *Meter) EndDraw(start, now time.Time) {
	c := m.Clock
	if c.frozen {
		return
	}
	c.ring.Draw.Push(max(1, now.Sub(start)))
	if !c.lastDraw.IsZero() {
		if gap := now.Sub(c.lastDraw); gap > 0 {
			c.ring.Frame.Push(gap)
		}
	}
	c.lastDraw = now
}

// refresh recomputes the stats and the overlay's strings at most twice a
// second (and the heap at most once a second).
func (m *Meter) refresh(now time.Time) {
	c := m.Clock
	if !c.computed.IsZero() && now.Sub(c.computed) < 500*time.Millisecond && now.After(c.computed) {
		return
	}
	c.computed = now
	r := &c.ring
	var fr, up, dr []time.Duration
	fr = r.Frame.Last(RingLen, nil)
	up = r.Update.Last(RingLen, nil)
	dr = r.Draw.Last(RingLen, nil)
	st := FrameStats(fr, up, dr)
	e := m.sh.Env()
	if !c.frozen {
		if e.InGame {
			st.TicksPerUpdate = float64(e.Stepped)
		}
		st.Held = e.Held
		c.heap(now)
		st.HeapMiB, st.GCPerMin = c.heapMiB, c.gcPerMin
	} else {
		st.TicksPerUpdate, st.Held, st.HeapMiB, st.GCPerMin = 3, e.Held, 412, 12
	}
	c.stats = st
	c.sparkBuf = r.Frame.Last(sparkLen, c.sparkBuf)
	c.Spark = c.Spark[:0]
	for _, d := range c.sparkBuf {
		c.Spark = append(c.Spark, int(d/time.Microsecond))
	}
	c.Lines = st.format()
}

// heap reads the heap and the GC rate from runtime/metrics at most once a
// second (never runtime.ReadMemStats, which stops the world).
func (c *Clock) heap(now time.Time) {
	if !c.heapAt.IsZero() && now.Sub(c.heapAt) < time.Second && now.After(c.heapAt) {
		return
	}
	if !c.samplesOK {
		c.samples[0].Name = "/memory/classes/heap/objects:bytes"
		c.samples[1].Name = "/gc/cycles/total:gc-cycles"
		c.samplesOK = true
	}
	metrics.Read(c.samples[:])
	if c.samples[0].Value.Kind() == metrics.KindUint64 {
		c.heapMiB = int(c.samples[0].Value.Uint64() >> 20)
	}
	if c.samples[1].Value.Kind() == metrics.KindUint64 {
		n := c.samples[1].Value.Uint64()
		if !c.gcAt.IsZero() && now.After(c.gcAt) && n >= c.gcCycles {
			// a smoothed per-minute rate over the span since the last read
			rate := float64(n-c.gcCycles) / now.Sub(c.gcAt).Minutes()
			c.gcPerMin = 0.5*c.gcPerMin + 0.5*rate
		}
		c.gcCycles, c.gcAt = n, now
	}
	c.heapAt = now
}
