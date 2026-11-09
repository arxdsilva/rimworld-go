package pool

import (
	"errors"
	"runtime"
	"sync/atomic"
	"testing"
	"unsafe"
)

// TestWorkersStartLazilyAndStop: no goroutine before the first parallel
// Run, seven parked after it, none after Close. It runs first: no other
// test's workers are still stopping.
func TestWorkersStartLazilyAndStop(t *testing.T) {
	before := runtime.NumGoroutine()
	p := New(Options{})
	if runtime.NumGoroutine() != before {
		t.Fatal("New started workers")
	}
	p.Run(64, func(int, int) {})
	if got := runtime.NumGoroutine() - before; got != MaxLanes-1 {
		t.Fatalf("%d workers after a parallel run, want %d", got, MaxLanes-1)
	}
	p.Close()
	for range 1000 {
		if runtime.NumGoroutine() == before {
			break
		}
		runtime.Gosched()
	}
	if runtime.NumGoroutine() != before {
		t.Fatal("workers still running after Close")
	}
	var n atomic.Int32
	p.Run(64, func(lane, _ int) {
		if lane == 0 {
			n.Add(1)
		}
	})
	if n.Load() != 64 {
		t.Fatal("a closed pool's run was not inline")
	}
}

func TestRunCallsEveryTaskOnce(t *testing.T) {
	for _, o := range []Options{{Lanes: 1}, {Lanes: 2}, {}, {MinTasks: 1}, {Shuffle: 9, MinTasks: 1}} {
		p := New(o)
		for _, n := range []int{0, 1, 7, 8, 9, 31, 32, 33, 100, 1000} {
			hits := make([]atomic.Int32, n)
			var badLane atomic.Bool
			p.Run(n, func(lane, i int) {
				if lane < 0 || lane >= p.Lanes() {
					badLane.Store(true)
				}
				hits[i].Add(1)
			})
			for i := range hits {
				if c := hits[i].Load(); c != 1 {
					t.Fatalf("%+v n=%d: task %d ran %d times", o, n, i, c)
				}
			}
			if badLane.Load() {
				t.Fatalf("%+v n=%d: a lane outside [0, %d)", o, n, p.Lanes())
			}
		}
		p.Close()
	}
}

func TestRunInlineBelowMinTasks(t *testing.T) {
	p := New(Options{})
	defer p.Close()
	var off atomic.Int32
	p.Run(minParallelTasks-1, func(lane, _ int) {
		if lane != 0 {
			off.Add(1)
		}
	})
	if off.Load() != 0 || p.started {
		t.Fatal("a run below minParallelTasks left the owner's lane or started workers")
	}
	one := New(Options{Lanes: 1, MinTasks: 1})
	one.Run(100, func(lane, _ int) {
		if lane != 0 {
			off.Add(1)
		}
	})
	if off.Load() != 0 || one.started {
		t.Fatal("a one-lane pool used another lane")
	}
}

// TestPanicReRaisedAtLowestTask: whichever lane meets them, the owner sees
// the panic of the lowest task index, and the pool runs on afterwards.
func TestPanicReRaisedAtLowestTask(t *testing.T) {
	for _, o := range []Options{{}, {Shuffle: 3, MinTasks: 1}, {Lanes: 2}} {
		p := New(o)
		for range 20 {
			got := func() (p0 *Panic) {
				defer func() {
					e, _ := recover().(error)
					errors.As(e, &p0)
				}()
				p.Run(200, func(_, i int) {
					if i == 150 || i == 13 || i == 77 {
						panic(i)
					}
				})
				return nil
			}()
			if got == nil || got.Task != 13 || got.Value != 13 || len(got.Stack) == 0 {
				t.Fatalf("%+v: re-raised %+v, want task 13", o, got)
			}
		}
		var n atomic.Int32
		p.Run(50, func(int, int) { n.Add(1) })
		if n.Load() != 50 {
			t.Fatalf("%+v: %d tasks after a panic, want 50", o, n.Load())
		}
		p.Close()
	}
}

func TestLaneStatePadded(t *testing.T) {
	if s := unsafe.Sizeof(lane{}); s%laneBytes != 0 {
		t.Fatalf("lane is %d bytes, want a multiple of %d", s, laneBytes)
	}
	var p Pool
	if off := unsafe.Offsetof(p.lanes); off < laneBytes {
		t.Fatalf("the claim counter shares a line with lane 0 (lanes at %d)", off)
	}
}

func BenchmarkRunEmpty64(b *testing.B) {
	p := New(Options{})
	defer p.Close()
	b.ReportAllocs()
	for b.Loop() {
		p.Run(64, func(int, int) {})
	}
}
