// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmhost

import (
	"bytes"
	"errors"
	"slices"
	"strings"
	"testing"

	"rimworld-go/internal/adapters/outbound/wasmhost/wasmrewrite"
	"rimworld-go/internal/adapters/outbound/wasmhost/wasmtest"
	"rimworld-go/internal/application/scripting"
)

// The depth modules are the W2-fuel call-stack modules rebuilt as ABI
// modules: the on_hour hook (or the hook named) loads the event's tick
// into the i64 global n (index 0), sends "start", calls the recursive
// function rec, and sends "done". rec recurses while n > 1, decrementing
// n per frame, so tick k makes exactly k live frames of rec under the
// hook's own frame.

const (
	depthOpI64LeS   byte = 0x57
	depthOpI64Load  byte = 0x29
	depthOpWrapI64  byte = 0xA7
	depthFirstFunc       = 13 // newGuest declares 13 imports
	depthOverBudget      = "call stack over budget in on_hour"
)

// depthGuard is: if n <= 1 { base; return }; n -= 1.
func depthGuard(base ...byte) []byte {
	g := []byte{wasmtest.OpGlobalGet, 0, wasmtest.OpI64Const, 1, depthOpI64LeS, wasmtest.OpIf, wasmtest.Empty}
	g = append(g, base...)
	return append(g, wasmtest.OpReturn, wasmtest.OpEnd,
		wasmtest.OpGlobalGet, 0, wasmtest.OpI64Const, 1, wasmtest.OpI64Sub, wasmtest.OpGlobalSet, 0)
}

func depthI64s(n int) []byte { return bytes.Repeat([]byte{wasmtest.I64}, n) }

func depthULEB(b []byte, v uint64) []byte {
	for v >= 0x80 {
		b = append(b, byte(v)|0x80)
		v >>= 7
	}
	return append(b, byte(v))
}

// depthGuest is a guest with the counter global n.
func depthGuest() *guest {
	g := newGuest()
	g.b.Global(wasmtest.I64, true, wasmtest.OpI64Const, 0, wasmtest.OpEnd)
	return g
}

// depthHook sets n from the tick, sends start, runs call (which calls rec
// and leaves nothing), and sends done.
func depthHook(g *guest, hook string, callRec ...byte) []byte {
	g.hook(hook,
		i32c(0), []byte{depthOpI64Load, 3}, depthULEB(nil, bufAt+8), []byte{wasmtest.OpGlobalSet, 0},
		call("message", false, g.str("start")),
		callRec,
		call("message", false, g.str("done")))
	return g.bytes()
}

// depthSmall: rec is [] -> [], body guard; call rec. The floor weight.
func depthSmall(hook string) []byte {
	g := depthGuest()
	rec := g.b.Func(nil, nil, nil, cat(depthGuard(), []byte{wasmtest.OpCall, depthFirstFunc, wasmtest.OpEnd})...)
	return depthHook(g, hook, wasmtest.OpCall, byte(rec))
}

// depthManyResults: rec is [] -> [i64 x 64], body guard(64 zeros); call rec.
func depthManyResults() []byte {
	g := depthGuest()
	zeros := bytes.Repeat([]byte{wasmtest.OpI64Const, 0}, 64)
	rec := g.b.Func(nil, depthI64s(64), nil, cat(depthGuard(zeros...), []byte{wasmtest.OpCall, depthFirstFunc, wasmtest.OpEnd})...)
	return depthHook(g, "on_hour", append([]byte{wasmtest.OpCall, byte(rec)}, bytes.Repeat([]byte{wasmtest.OpDrop}, 64)...)...)
}

// depthWorstCase is at the Check limits: rec has 64 i64 params and 4032
// i64 locals (MaxLocals) and a MaxBody body. Behind an untaken branch it
// pushes every param and local and as many distinct call results as fit,
// all live until a chain of i64.add consumes them; then it recurses,
// passing its params on. p (function 13) returns one i64.
func depthWorstCase() []byte {
	const params = 64
	p, rec := byte(depthFirstFunc), byte(depthFirstFunc+1)
	build := func(calls int) []byte {
		code := depthGuard()
		code = append(code, wasmtest.OpLocalGet, 0, depthOpWrapI64, wasmtest.OpIf, wasmtest.Empty, wasmtest.OpLoop, wasmtest.Empty)
		for i := range MaxLocals {
			code = depthULEB(append(code, wasmtest.OpLocalGet), uint64(i))
		}
		code = append(code, bytes.Repeat([]byte{wasmtest.OpCall, p}, calls)...)
		code = append(code, bytes.Repeat([]byte{0x7C}, MaxLocals+calls-1)...) // i64.add
		code = append(code, wasmtest.OpDrop, wasmtest.OpEnd, wasmtest.OpEnd)
		for i := range params {
			code = append(code, wasmtest.OpLocalGet, byte(i))
		}
		return append(code, wasmtest.OpCall, rec, wasmtest.OpEnd)
	}
	calls := (MaxBody - len(build(0))) / 3
	g := depthGuest()
	g.b.Func(nil, []byte{wasmtest.I64}, nil, wasmtest.OpI64Const, 1, wasmtest.OpEnd)
	g.b.Func(depthI64s(params), nil, depthI64s(MaxLocals-params), build(calls)...)
	return depthHook(g, "on_hour", append(bytes.Repeat([]byte{wasmtest.OpI64Const, 0}, params), wasmtest.OpCall, rec)...)
}

// depthProduceConsume is the plan review's module: behind an untaken
// local.get 0; if; loop … end; end, k calls of f ([] -> [i64 x 64], 13)
// then k calls of g ([i64 x 64] -> [], 14), then rec (15) recurses.
// k <= 0 fills MaxBody (about 16k each).
func depthProduceConsume(k int) []byte {
	f, c, rec := byte(depthFirstFunc), byte(depthFirstFunc+1), byte(depthFirstFunc+2)
	build := func(k int) []byte {
		code := depthGuard()
		code = append(code, wasmtest.OpLocalGet, 0, wasmtest.OpIf, wasmtest.Empty, wasmtest.OpLoop, wasmtest.Empty)
		code = append(code, bytes.Repeat([]byte{wasmtest.OpCall, f}, k)...)
		code = append(code, bytes.Repeat([]byte{wasmtest.OpCall, c}, k)...)
		return append(code, wasmtest.OpEnd, wasmtest.OpEnd, wasmtest.OpLocalGet, 0, wasmtest.OpCall, rec, wasmtest.OpEnd)
	}
	if k <= 0 {
		k = (MaxBody - len(build(0))) / 4
	}
	g := depthGuest()
	g.b.Func(nil, depthI64s(64), nil, append(bytes.Repeat([]byte{wasmtest.OpI64Const, 0}, 64), wasmtest.OpEnd)...)
	g.b.Func(depthI64s(64), nil, nil, wasmtest.OpEnd)
	g.b.Func([]byte{wasmtest.I32}, nil, nil, build(k)...)
	return depthHook(g, "on_hour", cat(i32c(0), []byte{wasmtest.OpCall, rec})...)
}

var errDepthStop = errors.New("stop")

// depthWeight reads w(f) of function fn (in the function index space)
// from Prepare's output: a metered body starts global.get depth;
// i64.const w.
func depthWeight(t *testing.T, m *wasmrewrite.Module, fn uint32) int64 {
	t.Helper()
	var ins []wasmrewrite.Instr
	_ = wasmrewrite.Walk(m.Bodies[fn-depthFirstFunc].Code, func(in wasmrewrite.Instr) error {
		ins = append(ins, in)
		if len(ins) == 2 {
			return errDepthStop
		}
		return nil
	})
	if len(ins) < 2 || ins[0].Op != uint32(wasmtest.OpGlobalGet) || ins[1].Op != uint32(wasmtest.OpI64Const) {
		t.Fatalf("function %d does not start with the depth charge: %+v", fn, ins)
	}
	return int64(ins[1].A)
}

// depthWeights are the hook's and rec's weights in the host's pipeline.
func depthWeights(t *testing.T, wasm []byte, rec uint32) (hook, recW int64) {
	t.Helper()
	out, err := wasmrewrite.Prepare(wasm, rewriteConfig())
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	m, err := wasmrewrite.Decode(out)
	if err != nil {
		t.Fatalf("decode the output: %v", err)
	}
	for _, e := range m.Exports {
		if e.Name == "on_hour" && e.Kind == wasmtest.KindFunc {
			return depthWeight(t, m, e.Index), depthWeight(t, m, rec)
		}
	}
	t.Fatal("no on_hour export")
	return 0, 0
}

// depthOutcome is what one engine did at the cap and one frame past it.
type depthOutcome struct {
	frames         int64
	fuelAt, fuelPa uint64
}

// depthAtCap fires on_hour with tick = the most frames of rec that fit
// under the hook (floor((StackBudget - w_hook) / w_rec)): it completes,
// both messages kept and depth back at 0. A fresh host with one frame more
// traps on the depth check: the problem names the budget, both messages
// are dropped and the depth global holds w_hook + (frames+1) * w_rec.
func depthAtCap(t *testing.T, o Options, wasm []byte, rec uint32) depthOutcome {
	t.Helper()
	wh, wr := depthWeights(t, wasm, rec)
	frames := (StackBudget - wh) / wr
	var out depthOutcome
	out.frames = frames

	h := newHost(t, o, plug("d", "scripts/d.wasm", wasm))
	res := h.Fire(scripting.Event{Hook: scripting.OnHour, Tick: uint64(frames)}, fakeWorld{}, nil)
	if len(res.Disabled) != 0 || !slices.Equal(messages(res.Commands), []string{"start", "done"}) {
		t.Fatalf("%d frames (w_hook %d, w_rec %d): commands %q disabled %+v, want both messages",
			frames, wh, wr, messages(res.Commands), res.Disabled)
	}
	if d := h.mods[0].depth.Get(); d != 0 {
		t.Errorf("%d frames: depth %d after the call, want 0", frames, d)
	}
	out.fuelAt = h.Stats()[0].Fuel

	h = newHost(t, o, plug("d", "scripts/d.wasm", wasm))
	res = h.Fire(scripting.Event{Hook: scripting.OnHour, Tick: uint64(frames + 1)}, fakeWorld{}, nil)
	if len(res.Disabled) != 1 || res.Disabled[0].Msg != depthOverBudget || len(res.Commands) != 0 {
		t.Fatalf("%d frames: commands %q disabled %+v, want none and %q", frames+1, messages(res.Commands), res.Disabled, depthOverBudget)
	}
	if d, want := int64(h.mods[0].depth.Get()), wh+(frames+1)*wr; d != want {
		t.Errorf("%d frames: depth %d at the trap, want %d", frames+1, d, want)
	}
	if fuel := int64(h.mods[0].fuel.Get()); fuel < 0 {
		t.Errorf("%d frames: fuel %d, want the depth trap with fuel left", frames+1, fuel)
	}
	out.fuelPa = h.Stats()[0].Fuel
	t.Logf("w_hook %d, w_rec %d: %d frames complete, %d trap", wh, wr, frames, frames+1)
	return out
}

// depthParity runs depthAtCap on every engine and checks they agree.
func depthParity(t *testing.T, wasm []byte, rec uint32) depthOutcome {
	t.Helper()
	var got []depthOutcome
	goldenEachEngine(t, func(t *testing.T, o Options) {
		got = append(got, depthAtCap(t, o, wasm, rec))
	})
	if len(got) == 0 {
		t.Fatal("no engine completed")
	}
	for _, o := range got[1:] {
		if o != got[0] {
			t.Errorf("engines disagree: %+v vs %+v", got[0], o)
		}
	}
	return got[0]
}

// C5: the floor-weight frame: w 4194, so 999 frames fit under the hook's
// own floor-weight frame (1000 in all) and the next one traps.
func TestDepthSmallFrameParityThroughTheHost(t *testing.T) {
	wh, wr := depthWeights(t, depthSmall("on_hour"), depthFirstFunc)
	if wh != 4194 || wr != 4194 {
		t.Fatalf("w_hook %d w_rec %d, want the floor 4194", wh, wr)
	}
	if o := depthParity(t, depthSmall("on_hour"), depthFirstFunc); o.frames != 999 {
		t.Errorf("%d frames, want 999", o.frames)
	}
}

// C5: 64 i64 results per frame stay at the floor weight.
func TestDepthManyResultsParityThroughTheHost(t *testing.T) {
	if o := depthParity(t, depthManyResults(), depthFirstFunc); o.frames != 999 {
		t.Errorf("%d frames, want 999", o.frames)
	}
}

// C5: the worst-case frame at the Check limits.
func TestDepthWorstCaseFrameParityThroughTheHost(t *testing.T) {
	wasm := depthWorstCase()
	_, wr := depthWeights(t, wasm, depthFirstFunc+1)
	if wr <= 4*MaxLocals {
		t.Errorf("w_rec %d does not count the live values", wr)
	}
	if o := depthParity(t, wasm, depthFirstFunc+1); o.frames < 2 {
		t.Errorf("%d frames", o.frames)
	}
}

// C5: the produce/consume module just under MaxWeight (k = 1984) loads and
// agrees on both engines at the cap and one frame past it.
func TestDepthProduceConsumeUnderTheCapParityThroughTheHost(t *testing.T) {
	wasm := depthProduceConsume(1984)
	_, wr := depthWeights(t, wasm, depthFirstFunc+2)
	if wr > MaxWeight || wr < 66*1984 {
		t.Errorf("w_rec %d: want the 66 values per k counted and at most %d", wr, MaxWeight)
	}
	if o := depthParity(t, wasm, depthFirstFunc+2); o.frames < 2 {
		t.Errorf("%d frames", o.frames)
	}
}

// C5: one more k (1985), and the full-size module (about 16k each, 1M
// values), are over MaxWeight: both engines reject them at load with the
// same problem, before any compile, and no Stat exists.
func TestDepthProduceConsumeOverTheCapIsRejectedOnBothEngines(t *testing.T) {
	for _, k := range []int{1985, 0} {
		wasm := depthProduceConsume(k)
		var msgs []string
		goldenEachEngine(t, func(t *testing.T, o Options) {
			h := newHost(t, o, plug("d", "scripts/d.wasm", wasm))
			res := fire(h, event(scripting.OnHour))
			if len(res.Disabled) != 1 || !strings.HasPrefix(res.Disabled[0].Msg, "rejected: ") ||
				!strings.Contains(res.Disabled[0].Msg, wasmrewrite.RuleWeight) {
				t.Fatalf("k=%d: disabled %+v, want a %q rejection", k, res.Disabled, wasmrewrite.RuleWeight)
			}
			if len(h.Stats()) != 0 || len(res.Commands) != 0 {
				t.Errorf("k=%d: stats %+v commands %v", k, h.Stats(), res.Commands)
			}
			msgs = append(msgs, res.Disabled[0].Msg)
		})
		if len(msgs) > 1 && msgs[0] != msgs[1] {
			t.Errorf("k=%d: engines disagree: %q vs %q", k, msgs[0], msgs[1])
		}
	}
}
