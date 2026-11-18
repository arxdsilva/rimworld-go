// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package pool

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"runtime/pprof"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"
)

// labeled counts the live goroutines carrying the pprof label key=val. A
// goroutine inherits its creator's labels, so this sees only this test's
// pool workers, whatever other tests' pools are doing.
func labeled(key, val string) int {
	var buf bytes.Buffer
	if err := pprof.Lookup("goroutine").WriteTo(&buf, 1); err != nil {
		panic(err)
	}
	want := fmt.Sprintf("%q:%q", key, val)
	total, count := 0, 0
	for _, line := range strings.Split(buf.String(), "\n") {
		if n, _, ok := strings.Cut(line, " @ "); ok {
			count, _ = strconv.Atoi(n)
		} else if strings.HasPrefix(line, "# labels:") && strings.Contains(line, want) {
			total += count
		}
	}
	return total
}

// settle polls (bounded) until the labelled goroutine count is want.
func settle(key, val string, want int) int {
	deadline := time.Now().Add(5 * time.Second)
	for {
		got := labeled(key, val)
		if got == want || time.Now().After(deadline) {
			return got
		}
		time.Sleep(time.Millisecond)
	}
}

// TestWorkersStartLazilyAndStop: no worker before the first parallel Run,
// seven parked after it, none after Close. Only this test's workers are
// counted (a unique pprof label), so test order and other pools' goroutines
// do not matter.
func TestWorkersStartLazilyAndStop(t *testing.T) {
	key, val := "pooltest", t.Name()+"/"+strconv.FormatInt(time.Now().UnixNano(), 36)
	pprof.Do(context.Background(), pprof.Labels(key, val), func(context.Context) {
		self := labeled(key, val) // the test goroutine carries the label too
		p := New(Options{})
		if got := labeled(key, val) - self; got != 0 {
			t.Fatalf("New started %d workers", got)
		}
		p.Run(64, func(int, int) {})
		if got := settle(key, val, self+MaxLanes-1) - self; got != MaxLanes-1 {
			t.Fatalf("%d workers after a parallel run, want %d", got, MaxLanes-1)
		}
		p.Close()
		if got := settle(key, val, self) - self; got != 0 {
			t.Fatalf("%d workers still running after Close", got)
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
	})
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
