package wealth

import (
	"math"
	"testing"

	"rimworld-go/internal/domain/sim/clock"
)

// parts is a Source over a slice of values; reads counts the parts summed.
type parts struct {
	v     []int
	reads int
}

func (p *parts) WealthOf(from, to int) (int, int) {
	to = min(to, len(p.v))
	sum := 0
	for i := max(from, 0); i < to; i++ {
		sum += p.v[i]
		p.reads++
	}
	return sum, len(p.v)
}

func total(v []int) int {
	s := 0
	for _, x := range v {
		s += x
	}
	return s
}

func seq(n int) []int {
	v := make([]int, n)
	for i := range v {
		v[i] = i%7 + 1
	}
	return v
}

// run steps t on every stride from tick from (exclusive) to to (inclusive).
func run(t *Tracker, from, to uint64) {
	for now := (from/Stride + 1) * Stride; now <= to; now += Stride {
		t.Step(now)
	}
}

// TestSweepSlices: a pass is spread over the hour, no slice reads much
// more than its share, and the finished value is the full count.
func TestSweepSlices(t *testing.T) {
	src := &parts{v: seq(1000)}
	tr := New(src)
	if got := tr.Value(); got != total(src.v) {
		t.Fatalf("before any pass: %d, want the live count %d", got, total(src.v))
	}
	src.reads = 0
	h0 := uint64(3 * Pass)
	for _, now := range []uint64{h0 + 7, h0 + Stride, h0 + Pass/2} { // in the hour, not its first tick (a load's Step)
		tr.Step(now)
		if src.reads != 0 || tr.Export() != nil {
			t.Fatalf("a pass started at tick %d, not an hour's first", now)
		}
	}
	most := 0
	for now := h0; now < h0+Pass; now += Stride {
		before := src.reads
		tr.Step(now)
		tr.Step(now) // twice at one tick: the work once
		most = max(most, src.reads-before)
	}
	if src.reads != len(src.v) || most > len(src.v)/steps+1 {
		t.Fatalf("%d parts read, at most %d in a slice; want %d, about %d", src.reads, most, len(src.v), len(src.v)/steps)
	}
	if tr.Value() != total(src.v) || len(tr.History()) != 1 {
		t.Fatalf("value %d, history %v", tr.Value(), tr.History())
	}
	src.v[0] += 100 // the value holds until the next pass finishes
	if tr.Value() != total(src.v)-100 {
		t.Fatal("the value moved between passes")
	}
	run(tr, h0+Pass-Stride, h0+2*Pass)
	if tr.Value() != total(src.v) {
		t.Fatalf("next pass %d, want %d", tr.Value(), total(src.v))
	}
}

// TestHistoryDaily: one sample a day, at most HistoryDays of them.
func TestHistoryDaily(t *testing.T) {
	src := &parts{v: seq(50)}
	tr := New(src)
	days := HistoryDays + 5
	for d := 0; d < days; d++ {
		start := uint64(d) * clock.TicksPerDay
		run(tr, start, start+3*Pass) // three passes a day: one sample
		src.v[d%len(src.v)]++
	}
	h := tr.History()
	if len(h) != HistoryDays {
		t.Fatalf("%d samples, want %d", len(h), HistoryDays)
	}
	for i := 1; i < len(h); i++ {
		if h[i].Tick/clock.TicksPerDay != h[i-1].Tick/clock.TicksPerDay+1 {
			t.Fatalf("samples %d and %d not a day apart: %v, %v", i-1, i, h[i-1], h[i])
		}
	}
}

// TestMissedSliceCaughtUp: a pass whose steps stop finishes on the next
// Step, whenever it comes, with the full count.
func TestMissedSliceCaughtUp(t *testing.T) {
	src := &parts{v: seq(300)}
	tr := New(src)
	tr.Step(0)
	run(tr, 0, Pass/2)
	tr.Step(5*Pass + Stride + 3)
	if tr.Value() != total(src.v) || !tr.s.Counted || tr.s.Hour != 0 {
		t.Fatalf("value %d, state %+v", tr.Value(), tr.s)
	}
}

// TestSaveMidPass: a tracker saved mid-pass and restored ends the pass on
// the same value as the one that ran on, and the save copies its history.
func TestSaveMidPass(t *testing.T) {
	a, b := &parts{v: seq(777)}, &parts{v: seq(777)}
	ta := New(a)
	ta.Step(0)
	run(ta, 0, Pass+Pass/3)
	s := ta.Export()
	if s == nil || s.Hour == 0 || s.Cursor == 0 {
		t.Fatalf("mid-pass save %+v", s)
	}
	tb := New(b)
	tb.Import(s)
	for _, p := range []*parts{a, b} {
		p.v[700] += 50 // a change ahead of the cursor counts in both
	}
	run(ta, Pass+Pass/3, 3*Pass)
	run(tb, Pass+Pass/3, 3*Pass)
	if ta.Value() != tb.Value() || ta.Value() != total(a.v) {
		t.Fatalf("values %d and %d, want %d", ta.Value(), tb.Value(), total(a.v))
	}
	s.History[0].Value = -1
	if ta.History()[0].Value == -1 {
		t.Fatal("Export shares the history")
	}
	tb.Import(nil)
	if tb.Export() != nil || tb.Value() != total(b.v) {
		t.Fatal("Import(nil) empties the tracker")
	}
}

func TestNilTracker(t *testing.T) {
	var tr *Tracker
	if tr.History() != nil || tr.Export() != nil {
		t.Fatal("a nil tracker has nothing")
	}
	if _, n := (&parts{}).WealthOf(0, math.MaxInt); n != 0 {
		t.Fatal("empty source")
	}
}
