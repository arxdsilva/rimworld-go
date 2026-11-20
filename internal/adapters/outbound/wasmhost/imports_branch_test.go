// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmhost

import (
	"math"
	"testing"

	"rimworld-go/internal/application/scripting"
	"rimworld-go/internal/testkit/allocs"
)

// stops runs f and reports whether it panicked with the host's stop.
func stops(f func()) (stopped bool) {
	defer func() {
		if r := recover(); r != nil {
			_, stopped = r.(stopSignal)
			if !stopped {
				panic(r)
			}
		}
	}()
	f()
	return false
}

func branchCall(t *testing.T) (*Host, *callState) {
	h := newHost(t, Options{Engine: Interpreter, Fuel: 100}, fixture(t, "p", "dice"))
	cs := &callState{m: h.mods[0], hook: "on_hour"}
	h.begin(cs)
	return h, cs
}

// An import whose cost is over the fuel left stops the call, and the fuel
// global is left below 0 for the host to read.
func TestImportsChargeBelowZeroStops(t *testing.T) {
	_, cs := branchCall(t)
	if stops(func() { cs.charge(100) }) { // exactly the budget: fuel 0 is fine
		t.Fatal("charging exactly the fuel left must not stop")
	}
	if !stops(func() { cs.charge(1) }) {
		t.Fatal("charging past 0 must stop the call")
	}
	if int64(cs.m.fuel.Get()) != -1 {
		t.Errorf("fuel = %d, want -1", int64(cs.m.fuel.Get()))
	}
}

// A write past the end of memory fails the call with the import's message.
func TestImportsPutOutOfRangeFailsTheCall(t *testing.T) {
	_, cs := branchCall(t)
	size := cs.m.mem.Size()
	if !stops(func() { cs.put("pawns", size-1, []byte{1, 2}) }) || cs.fail != "pawns: pointer out of range" {
		t.Fatalf("fail = %q", cs.fail)
	}
}

func TestImportsClampI32(t *testing.T) {
	for _, c := range []struct {
		in   int
		want int32
	}{
		{0, 0}, {-7, -7}, {math.MaxInt32, math.MaxInt32}, {math.MaxInt32 + 1, math.MaxInt32},
		{math.MinInt32, math.MinInt32}, {math.MinInt32 - 1, math.MinInt32}, {1 << 40, math.MaxInt32}, {-1 << 40, math.MinInt32},
	} {
		if got := clampI32(c.in); got != c.want {
			t.Errorf("clampI32(%d) = %d, want %d", c.in, got, c.want)
		}
	}
}

// The event buffer was range-checked at load, so the write failing means
// memory is not what load saw: the call fails, it does not panic.
func TestHostWriteEventOutsideMemoryFails(t *testing.T) {
	h, cs := branchCall(t)
	cs.m.buf = cs.m.mem.Size() // past the end, with a size the host believes
	cs.m.bufSize = 4096
	if msg := h.writeEvent(cs.m, event(scripting.OnHour), nil); msg != "event does not fit in memory" {
		t.Errorf("msg = %q", msg)
	}
}

// Not parallel (allocation counts). A text length over MaxText fails before the bytes are copied: the
// allocations of the failing call do not grow with the length.
func TestImportsOversizeLengthIsNotCopied(t *testing.T) {
	_, cs := branchCall(t)
	run := func(n uint64) func() {
		return func() {
			cs.fail = ""
			stops(func() { cs.message([]uint64{0, n}) })
		}
	}
	small := allocs.Fewest(20, run(uint64(scripting.MaxText+1)))
	huge := allocs.Fewest(20, run(1<<31)) // 2 GiB: a copy could not even succeed
	if huge > small || cs.fail != "message: longer than 500 bytes" {
		t.Errorf("allocs small %v huge %v, fail %q", small, huge, cs.fail)
	}
}
