// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmrewrite_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"rimworld-go/internal/adapters/outbound/wasmhost/wasmrewrite"
	. "rimworld-go/internal/adapters/outbound/wasmhost/wasmtest"
)

// The call-stack modules recurse while the exported i64 global "n" (index
// 0) is above 1, decrementing it per frame, so n = k makes exactly k live
// frames of the recursive function.

// fuelGuard is: if n <= 1 { base; return }; n -= 1.
func fuelGuard(base ...byte) []byte {
	g := []byte{OpGlobalGet, 0, OpI64Const, 1, fuelOpI64LeS, OpIf, Empty}
	g = append(g, base...)
	return append(g, OpReturn, OpEnd, OpGlobalGet, 0, OpI64Const, 1, OpI64Sub, OpGlobalSet, 0)
}

func fuelCounter(b *Builder) {
	n := b.Global(I64, true, OpI64Const, 0, OpEnd)
	b.Export("n", KindGlobal, n)
}

// fuelSmallRecursion is [] -> [], body guard; call self: the floor weight.
func fuelSmallRecursion() []byte {
	var b Builder
	fuelCounter(&b)
	self := b.Func(nil, nil, nil, fuelCat(fuelGuard(), []byte{OpCall, 0, OpEnd})...)
	b.Export("rec", KindFunc, self)
	return b.Bytes()
}

func fuelI64s(n int) []byte { return bytes.Repeat([]byte{I64}, n) }

// fuelManyResults is [] -> [i64 x 64], body guard(64 zeros); call self.
func fuelManyResults() []byte {
	var b Builder
	fuelCounter(&b)
	zeros := bytes.Repeat([]byte{OpI64Const, 0}, 64)
	self := b.Func(nil, fuelI64s(64), nil, fuelCat(fuelGuard(zeros...), []byte{OpCall, 0, OpEnd})...)
	b.Export("rec", KindFunc, self)
	return b.Bytes()
}

// fuelULEB appends v as an unsigned LEB128.
func fuelULEB(b []byte, v uint64) []byte {
	for v >= 0x80 {
		b = append(b, byte(v)|0x80)
		v >>= 7
	}
	return append(b, byte(v))
}

// fuelWorstCase is at the Check limits: 64 i64 params, 4032 i64 locals
// (MaxLocals in all) and a body of MaxBody bytes. Behind an untaken
// branch, it pushes every param and local and then as many distinct call
// results as fit, all live until a chain of i64.add consumes them at the
// end. The recursion passes the 64 params on.
func fuelWorstCase() (wasm []byte, values int) {
	const params, maxLocals, maxBody = 64, 4096, 64 << 10
	build := func(calls int) []byte {
		code := fuelGuard()
		code = append(code, OpLocalGet, 0, fuelOpI32WrapI64, OpIf, Empty, OpLoop, Empty)
		for i := range maxLocals {
			code = append(code, OpLocalGet)
			code = fuelULEB(code, uint64(i))
		}
		code = append(code, bytes.Repeat([]byte{OpCall, 0}, calls)...)
		code = append(code, bytes.Repeat([]byte{0x7C}, maxLocals+calls-1)...) // i64.add
		code = append(code, OpDrop, OpEnd, OpEnd)
		for i := range params {
			code = append(code, OpLocalGet, byte(i))
		}
		return append(code, OpCall, 1, OpEnd)
	}
	calls := (maxBody - len(build(0))) / 3
	var b Builder
	fuelCounter(&b)
	b.Func(nil, []byte{I64}, nil, OpI64Const, 1, OpEnd) // p: one distinct value per call
	self := b.Func(fuelI64s(params), nil, fuelI64s(maxLocals-params), build(calls)...)
	b.Export("rec", KindFunc, self)
	return b.Bytes(), maxLocals + calls
}

// fuelProduceConsume is the plan review's module: behind an untaken
// local.get 0; if; loop … end; end, about 16k call f (f: [] -> [i64 x 64])
// then as many call g (g: [i64 x 64] -> []), about 1M values live, within
// MaxBody; then it recurses into itself.
func fuelProduceConsume() (wasm []byte, k int) { return fuelProduceConsumeK(0) }

// fuelProduceConsumeK builds it with k calls of each; k <= 0 fills MaxBody.
func fuelProduceConsumeK(k int) ([]byte, int) {
	const maxBody = 64 << 10
	build := func(k int) []byte {
		code := fuelGuard()
		code = append(code, OpLocalGet, 0, OpIf, Empty, OpLoop, Empty)
		code = append(code, bytes.Repeat([]byte{OpCall, 0}, k)...)
		code = append(code, bytes.Repeat([]byte{OpCall, 1}, k)...)
		return append(code, OpEnd, OpEnd, OpLocalGet, 0, OpCall, 2, OpEnd)
	}
	if k <= 0 {
		k = (maxBody - len(build(0))) / 4
	}
	var b Builder
	fuelCounter(&b)
	b.Func(nil, fuelI64s(64), nil, append(bytes.Repeat([]byte{OpI64Const, 0}, 64), OpEnd)...)
	b.Func(fuelI64s(64), nil, nil, OpEnd)
	self := b.Func([]byte{I32}, nil, nil, build(k)...)
	b.Export("rec", KindFunc, self)
	return b.Bytes(), k
}

// fuelDepthAtBudget checks defined function fn (exported as "rec", taking
// args): w equals the contract formula, floor(StackBudget/w) frames
// complete with depth back at 0, and the next frame traps on the depth
// check (depth = (frames+1)*w > StackBudget, fuel left) on every engine.
func fuelDepthAtBudget(t *testing.T, wasm []byte, fn int, args ...uint64) (w, frames int64) {
	t.Helper()
	return fuelDepthAtBudgetOn(t, engines(t), wasm, fn, args...)
}

func fuelDepthAtBudgetOn(t *testing.T, es []testEngine, wasm []byte, fn int, args ...uint64) (w, frames int64) {
	t.Helper()
	m := fuelMetered(t, wasm)
	w = fuelWeight(t, m, fn)
	if want := fuelOracleWeight(t, m, fn); w != want {
		t.Fatalf("w = %d, the formula gives %d", w, want)
	}
	budget := int64(testConfig().StackBudget)
	frames = budget / w
	out := m.Encode()
	for _, e := range es {
		fi := fuelInstantiate(t, e, out)
		fi.set(t, "n", frames)
		if res := fi.call(t, fuelPlenty, "rec", args...); res.err != nil || res.depth != 0 {
			t.Errorf("%s: %d frames: err %v depth %d, want success at depth 0", e.name, frames, res.err, res.depth)
		}
		fi.set(t, "n", frames+1)
		res := fi.call(t, fuelPlenty, "rec", args...)
		if res.err == nil || !strings.Contains(res.err.Error(), "unreachable") {
			t.Fatalf("%s: %d frames: err %v, want the depth check's unreachable trap", e.name, frames+1, res.err)
		}
		if res.depth != (frames+1)*w || res.depth <= budget || res.fuel < 0 {
			t.Errorf("%s: %d frames: depth %d fuel %d, want depth %d > %d with fuel left",
				e.name, frames+1, res.depth, res.fuel, (frames+1)*w, budget)
		}
	}
	return w, frames
}

func TestDepthFloorWeightAllowsExactly1000Frames(t *testing.T) {
	w, frames := fuelDepthAtBudget(t, fuelSmallRecursion(), 0)
	if w != 4194 || frames != 1000 {
		t.Errorf("w %d frames %d, want the floor 4194 and 1000", w, frames)
	}
}

func TestDepthManyResultsAllowsExactly1000Frames(t *testing.T) {
	w, frames := fuelDepthAtBudget(t, fuelManyResults(), 0)
	if w != 4194 || frames != 1000 {
		t.Errorf("w %d frames %d, want the floor 4194 and 1000", w, frames)
	}
}

func TestDepthWorstCaseFrameAtCheckLimits(t *testing.T) {
	wasm, values := fuelWorstCase()
	w, frames := fuelDepthAtBudget(t, wasm, 1, make([]uint64, 64)...)
	// Every value is pushed (1 + 1) and consumed by an add (1 + 1), on top
	// of the 4096 params and locals.
	if w < int64(4096+4*values-2) {
		t.Errorf("w %d does not count the %d live values", w, values)
	}
	t.Logf("worst case at the Check limits: %d values, w = %d, %d frames", values, w, frames)
}

// fuelPrepareRule runs Prepare and returns the RejectError rule, or "".
func fuelPrepareRule(t *testing.T, wasm []byte, c wasmrewrite.Config) string {
	t.Helper()
	_, err := wasmrewrite.Prepare(wasm, c)
	if err == nil {
		return ""
	}
	var re *wasmrewrite.RejectError
	if !errors.As(err, &re) {
		t.Fatalf("Prepare: %v, want nil or a RejectError", err)
	}
	return re.Rule
}

// fuelProduceConsumeUnderCap is the largest produce/consume module whose
// w fits MaxWeight: each k adds 66 (call f 1+64, call g 1).
func fuelProduceConsumeUnderCap(t *testing.T) (wasm []byte, k int) {
	t.Helper()
	// k = 100 is above the floor weight, so w is linear in k from there.
	hundred, _ := fuelProduceConsumeK(100)
	base := fuelWeight(t, fuelMetered(t, hundred), 2) - 66*100
	k = int((int64(testConfig().MaxWeight) - base) / 66)
	wasm, _ = fuelProduceConsumeK(k)
	return wasm, k
}

// The full-size produce/consume module (about 1M values, w about 1.08M)
// is over MaxWeight: Prepare rejects it in pure Go, so every engine and
// arch refuses it the same way. Without the cap the wazevo arm64 compiler
// alone refused to compile it (16 MiB machine-code cap per function).
func TestDepthProduceConsumeAtMaxBodyIsRejected(t *testing.T) {
	wasm, k := fuelProduceConsume()
	if k < 16000 {
		t.Fatalf("k = %d, want about 16k", k)
	}
	if rule := fuelPrepareRule(t, wasm, testConfig()); rule != wasmrewrite.RuleWeight {
		t.Errorf("Prepare at MaxBody: rule %q, want %q", rule, wasmrewrite.RuleWeight)
	}
}

func TestDepthProduceConsumeJustUnderTheWeightCap(t *testing.T) {
	wasm, k := fuelProduceConsumeUnderCap(t)
	if rule := fuelPrepareRule(t, wasm, testConfig()); rule != "" {
		t.Fatalf("k = %d: rejected (%s), want accepted", k, rule)
	}
	over, _ := fuelProduceConsumeK(k + 1)
	if rule := fuelPrepareRule(t, over, testConfig()); rule != wasmrewrite.RuleWeight {
		t.Errorf("k = %d: rule %q, want %q", k+1, rule, wasmrewrite.RuleWeight)
	}
	zero := testConfig()
	zero.MaxWeight = 0 // DefaultMaxWeight
	if fuelPrepareRule(t, wasm, zero) != "" || fuelPrepareRule(t, over, zero) != wasmrewrite.RuleWeight {
		t.Errorf("MaxWeight 0 does not default to %d", wasmrewrite.DefaultMaxWeight)
	}
	w, frames := fuelDepthAtBudget(t, wasm, 2, 0)
	if w > int64(testConfig().MaxWeight) || w < int64(66*k) {
		t.Errorf("w %d: want the %d produced values counted and at most %d", w, 64*k, testConfig().MaxWeight)
	}
	t.Logf("produce/consume under the cap: k = %d, w = %d, %d frames", k, w, frames)
}

// fuelFloorOne is testConfig with a floor weight of 1, so w shows V.
func fuelFloorOne(budget uint64) wasmrewrite.Config {
	c := testConfig()
	c.StackBudget = budget
	c.MaxDepth = uint32(budget)
	return c
}

// V is pinned by hand on the final body. Each of the depth charge and the
// fuel charge is 10 instructions pushing 6 values (16); the epilogue is
// global.get, i64.const, i64.sub, global.set, end (5 + 3 = 8).
func TestDepthWeightCountsValuesExactly(t *testing.T) {
	// The recursion body: wrapper block [] (1); guard, 10 instructions
	// pushing 6 (16); call self with no results (1); end (1).
	// w = 16 + 16 + 1 + 16 + 1 + 1 + 8 = 59, no params or locals.
	m := fuelMeteredWith(t, fuelSmallRecursion(), fuelFloorOne(1<<22))
	if w := fuelWeight(t, m, 0); w != 59 {
		t.Errorf("recursion: w = %d, want 59", w)
	}
	var b Builder
	b.Memory(1, 1)
	pair := b.Import("rimworld", "pair", nil, []byte{I32, I32})
	b.Func([]byte{I32}, []byte{I32}, nil,
		OpCall, byte(pair), OpDrop, OpDrop, // 1+2, 1, 1
		OpLocalGet, 0, OpI32Load, 2, 0, OpLocalGet, 0, OpLocalGet, 0, OpSelect, // 5 x (1+1)
		OpBlock, I32, OpI32Const, 5, OpEnd, OpI32Add, // 1+1, 1+1, 1, 1+1
		OpLocalGet, 0, OpI32Const, 9, OpI32Store, 2, 0, OpEnd) // 1+1, 1+1, 1, 1
	// wrapper block (result i32) 2; body 5 + 10 + 7 + 6 = 28; 1 param.
	// w = 16 + 16 + 2 + 28 + 8 + 1 = 71.
	m = fuelMeteredWith(t, b.Bytes(), fuelFloorOne(1<<22))
	if w := fuelWeight(t, m, 0); w != 71 {
		t.Errorf("mixed body: w = %d, want 71", w)
	}
}

// With StackBudget = N*w exactly, N frames fit (depth == budget is allowed)
// and frame N+1 traps.
func TestDepthBudgetIsInclusive(t *testing.T) {
	const frames = 500
	w := fuelWeight(t, fuelMeteredWith(t, fuelSmallRecursion(), fuelFloorOne(1<<22)), 0)
	budget := frames * w
	out := fuelMeteredWith(t, fuelSmallRecursion(), fuelFloorOne(uint64(budget))).Encode()
	for _, e := range engines(t) {
		fi := fuelInstantiate(t, e, out)
		fi.set(t, "n", frames)
		if res := fi.call(t, fuelPlenty, "rec"); res.err != nil || res.depth != 0 {
			t.Errorf("%s: %d frames at depth == budget: err %v depth %d", e.name, frames, res.err, res.depth)
		}
		fi.set(t, "n", frames+1)
		if res := fi.call(t, fuelPlenty, "rec"); res.err == nil || res.depth != budget+w {
			t.Errorf("%s: %d frames: err %v depth %d, want a trap at %d", e.name, frames+1, res.err, res.depth, budget+w)
		}
	}
}
