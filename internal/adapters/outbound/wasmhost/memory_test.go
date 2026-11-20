// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmhost

import (
	"encoding/binary"
	"strings"
	"testing"

	"rimworld-go/internal/adapters/outbound/wasmhost/wasmtest"
	"rimworld-go/internal/application/scripting"
)

// memAt is where the grow module stores each memory.grow result.
const memAt = outAt

// memGrower's on_hour grows memory by each delta in turn, storing the
// i32 results at memAt, memAt+4, …, then sends "grew".
func memGrower(minPages, maxPages uint32, deltas ...int64) []byte {
	g := newGuest()
	g.b.Memory(minPages, maxPages)
	var body []byte
	for i, d := range deltas {
		body = cat(body, i32c(int64(memAt+4*i)), i32c(d), []byte{wasmtest.OpMemoryGrow, 0, wasmtest.OpI32Store, 2, 0})
	}
	g.hook("on_hour", body, call("message", false, g.str("grew")))
	return g.bytes()
}

// memResults reads the n grow results the last on_hour stored.
func memResults(t *testing.T, h *Host, n int) []int32 {
	t.Helper()
	b, ok := h.mods[0].mem.Read(memAt, uint32(4*n))
	if !ok {
		t.Fatal("results outside memory")
	}
	out := make([]int32, n)
	for i := range out {
		out[i] = int32(binary.LittleEndian.Uint32(b[4*i:]))
	}
	return out
}

// memFire fires on_hour and checks it ran with no problem.
func memFire(t *testing.T, h *Host) {
	t.Helper()
	res := fire(h, event(scripting.OnHour))
	if len(res.Disabled) != 0 || len(res.Commands) != 1 || res.Commands[0].Name != "grew" {
		t.Fatalf("commands %q disabled %+v, want grew and no problem", messages(res.Commands), res.Disabled)
	}
}

func memEqual(a []int32, b ...int32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// C3: a module with max <= the limit grows to its max; one page more
// returns -1 with no trap, the call's commands are kept, the file stays
// on, and Stats().Pages follows each step.
func TestMemoryGrowsToItsMaxThenGrowReturnsMinusOne(t *testing.T) {
	goldenEachEngine(t, func(t *testing.T, o Options) {
		o.MaxPages = 8
		h := newHost(t, o, plug("m", "scripts/m.wasm", memGrower(1, 4, 2, 1, 1, 0)))
		if p := h.Stats()[0].Pages; p != 1 {
			t.Fatalf("pages %d before any grow, want 1", p)
		}
		memFire(t, h)
		// 1 -> 3 -> 4, then the max stops it; size(0) is still 4
		if got := memResults(t, h, 4); !memEqual(got, 1, 3, -1, 4) {
			t.Errorf("grow results %v, want [1 3 -1 4]", got)
		}
		if st := h.Stats()[0]; st.Pages != 4 || st.Off {
			t.Errorf("stats %+v, want 4 pages and on", st)
		}
		// next call: every grow but the zero one fails; still no trap
		memFire(t, h)
		if got := memResults(t, h, 4); !memEqual(got, -1, -1, -1, 4) {
			t.Errorf("second call: grow results %v, want [-1 -1 -1 4]", got)
		}
		if st := h.Stats()[0]; st.Pages != 4 || st.Off || st.Calls != 2 {
			t.Errorf("stats %+v, want 4 pages, on, 2 calls", st)
		}
	})
}

// C3: a module with no max is capped at Options.MaxPages, and at
// DefaultMaxPages when that is zero.
func TestMemoryWithNoMaxIsCappedAtTheLimit(t *testing.T) {
	for _, tc := range []struct {
		name  string
		limit uint32 // Options.MaxPages
		cap   int64
	}{{"limit 8", 8, 8}, {"default", 0, DefaultMaxPages}} {
		t.Run(tc.name, func(t *testing.T) {
			goldenEachEngine(t, func(t *testing.T, o Options) {
				o.MaxPages = tc.limit
				h := newHost(t, o, plug("m", "scripts/m.wasm", memGrower(1, 0, tc.cap-1, 1)))
				memFire(t, h)
				if got := memResults(t, h, 2); !memEqual(got, 1, -1) {
					t.Errorf("grow results %v, want [1 -1]", got)
				}
				if st := h.Stats()[0]; st.Pages != uint32(tc.cap) || st.Off {
					t.Errorf("stats %+v, want %d pages", st, tc.cap)
				}
			})
		})
	}
}

// C3: a module whose min or max is over Options.MaxPages is rejected at
// load with the same problem on both engines and never runs; at the limit
// it loads.
func TestMemoryOverTheLimitIsRejectedAtLoad(t *testing.T) {
	const limit = 8
	for _, tc := range []struct {
		name     string
		min, max uint32
		reject   bool
	}{
		{"min over", limit + 1, 0, true},
		{"max over", 1, limit + 1, true},
		{"min and max over", limit + 1, limit + 2, true},
		{"max at the limit", 1, limit, false},
		{"min at the limit", limit, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var msgs []string
			goldenEachEngine(t, func(t *testing.T, o Options) {
				o.MaxPages = limit
				h := newHost(t, o, plug("m", "scripts/m.wasm", memGrower(tc.min, tc.max, 0)))
				res := fire(h, event(scripting.OnHour))
				if !tc.reject {
					if len(res.Disabled) != 0 || len(h.Stats()) != 1 {
						t.Fatalf("disabled %+v, want it loaded", res.Disabled)
					}
					return
				}
				if len(res.Disabled) != 1 || len(h.Stats()) != 0 || len(res.Commands) != 0 || h.Uses(scripting.OnHour) {
					t.Fatalf("disabled %+v stats %+v commands %v, want one load problem and no run", res.Disabled, h.Stats(), res.Commands)
				}
				msg := res.Disabled[0].Msg
				if !strings.HasPrefix(msg, "invalid module: ") && !strings.HasPrefix(msg, "rejected: ") {
					t.Errorf("problem %q is not a load rejection", msg)
				}
				msgs = append(msgs, msg)
			})
			if len(msgs) == 2 && msgs[0] != msgs[1] {
				t.Errorf("engines disagree: %q vs %q", msgs[0], msgs[1])
			}
			if len(msgs) > 0 {
				t.Logf("problem: %s", msgs[0])
			}
		})
	}
}
