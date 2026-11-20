// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmhost

import (
	"encoding/binary"
	"fmt"
	"strings"
	"testing"

	"rimworld-go/internal/adapters/outbound/wasmhost/wasmtest"
	"rimworld-go/internal/application/scripting"
)

// Opcodes the NaN module needs beyond wasmtest's.
const (
	nanOpI32Load8U  byte = 0x2D
	nanOpI32Store8  byte = 0x3A
	nanOpI32DivS    byte = 0x6D
	nanOpI64And     byte = 0x83
	nanOpI64ShrU    byte = 0x88
	nanOpI32WrapI64 byte = 0xA7
	nanHexTable          = 1024 // "0123456789abcdef"
	nanTextAt            = outAt
	nanTextStride        = 64
	nanCanonF32          = 0x7FC00000
	nanCanonF64          = 0x7FF8000000000000
	nanPayloadF32        = 0xFFC00123         // negative, quiet, payload 0x123
	nanSignalF32         = 0x7FA00001         // signalling, payload 1
	nanPayloadF64        = 0xFFF8000000000123 // negative, quiet, payload 0x123
	nanSignalF64         = 0x7FF4000000000001 // signalling, payload 1
	nanInfF32            = 0x7F800000
	nanNegInfF32         = 0xFF800000
	nanInfF64            = 0x7FF0000000000000
	nanNegInfF64         = 0xFFF0000000000000
)

// nanCase is one float op whose i64 result bits the module sends as
// "<label>=<16 hex digits>"; want is what every engine must print.
type nanCase struct {
	hook  scripting.Hook
	label string
	code  []byte // leaves one i64 on the stack
	want  uint64
}

func nanF32(bits uint32) []byte {
	return binary.LittleEndian.AppendUint32([]byte{wasmtest.OpF32Const}, bits)
}

func nanF64(bits uint64) []byte {
	return binary.LittleEndian.AppendUint64([]byte{wasmtest.OpF64Const}, bits)
}

// nanBits32 turns the f32 on the stack into its bits as an i64.
var nanBits32 = []byte{wasmtest.OpI32ReinterpretF32, wasmtest.OpI64ExtendI32U}

// nanBits64 turns the f64 on the stack into its bits.
var nanBits64 = []byte{wasmtest.OpI64ReinterpretF64}

func nanSat(sub byte, in []byte, i32 bool) []byte {
	out := cat(in, []byte{wasmtest.PrefixFC, sub})
	if i32 {
		out = append(out, wasmtest.OpI64ExtendI32U)
	}
	return out
}

// nanCases are the plan's NaN producers (div 0/0, sqrt(-1), min/max,
// ceil/floor/trunc/nearest and add of a payload NaN, promote/demote of a
// payload NaN), the sat-trunc edges, and two controls: neg is not
// canonicalised, so it keeps the payload with the sign flipped.
func nanCases() []nanCase {
	h, d, inc := scripting.OnHour, scripting.OnDay, scripting.OnIncident
	f32un := func(op byte) []byte { return cat(nanF32(nanPayloadF32), []byte{op}, nanBits32) }
	f64un := func(op byte) []byte { return cat(nanF64(nanPayloadF64), []byte{op}, nanBits64) }
	one32, one64 := nanF32(0x3F800000), nanF64(0x3FF0000000000000)
	return []nanCase{
		{h, "f32.div00", cat(nanF32(0), nanF32(0), []byte{wasmtest.OpF32Div}, nanBits32), nanCanonF32},
		{h, "f32.sqrtm1", cat(nanF32(0xBF800000), []byte{wasmtest.OpF32Sqrt}, nanBits32), nanCanonF32},
		{h, "f32.min", cat(nanF32(nanPayloadF32), one32, []byte{wasmtest.OpF32Min}, nanBits32), nanCanonF32},
		{h, "f32.max", cat(one32, nanF32(nanSignalF32), []byte{wasmtest.OpF32Max}, nanBits32), nanCanonF32},
		{h, "f32.ceil", f32un(wasmtest.OpF32Ceil), nanCanonF32},
		{h, "f32.floor", f32un(wasmtest.OpF32Floor), nanCanonF32},
		{h, "f32.trunc", f32un(wasmtest.OpF32Trunc), nanCanonF32},
		{h, "f32.nearest", f32un(wasmtest.OpF32Nearest), nanCanonF32},
		{h, "f32.add", cat(nanF32(nanSignalF32), one32, []byte{wasmtest.OpF32Add}, nanBits32), nanCanonF32},
		{h, "f32.demote", cat(nanF64(nanPayloadF64), []byte{wasmtest.OpF32DemoteF64}, nanBits32), nanCanonF32},
		{h, "f32.neg", f32un(wasmtest.OpF32Neg), 0x7FC00123},
		{d, "f64.div00", cat(nanF64(0), nanF64(0), []byte{wasmtest.OpF64Div}, nanBits64), nanCanonF64},
		{d, "f64.sqrtm1", cat(nanF64(0xBFF0000000000000), []byte{wasmtest.OpF64Sqrt}, nanBits64), nanCanonF64},
		{d, "f64.min", cat(nanF64(nanPayloadF64), one64, []byte{wasmtest.OpF64Min}, nanBits64), nanCanonF64},
		{d, "f64.max", cat(one64, nanF64(nanSignalF64), []byte{wasmtest.OpF64Max}, nanBits64), nanCanonF64},
		{d, "f64.ceil", f64un(wasmtest.OpF64Ceil), nanCanonF64},
		{d, "f64.floor", f64un(wasmtest.OpF64Floor), nanCanonF64},
		{d, "f64.trunc", f64un(wasmtest.OpF64Trunc), nanCanonF64},
		{d, "f64.nearest", f64un(wasmtest.OpF64Nearest), nanCanonF64},
		{d, "f64.sub", cat(nanF64(nanSignalF64), one64, []byte{wasmtest.OpF64Sub}, nanBits64), nanCanonF64},
		{d, "f64.promote", cat(nanF32(nanPayloadF32), []byte{wasmtest.OpF64PromoteF32}, nanBits64), nanCanonF64},
		{d, "f64.neg", f64un(wasmtest.OpF64Neg), 0x7FF8000000000123},
		{inc, "i32.sat.f32s.nan", nanSat(0, nanF32(nanPayloadF32), true), 0},
		{inc, "i32.sat.f32s.inf", nanSat(0, nanF32(nanInfF32), true), 0x7FFFFFFF},
		{inc, "i32.sat.f32s.-inf", nanSat(0, nanF32(nanNegInfF32), true), 0x80000000},
		{inc, "i32.sat.f64u.nan", nanSat(3, nanF64(nanSignalF64), true), 0},
		{inc, "i32.sat.f64u.inf", nanSat(3, nanF64(nanInfF64), true), 0xFFFFFFFF},
		{inc, "i64.sat.f64s.nan", nanSat(6, nanF64(nanPayloadF64), false), 0},
		{inc, "i64.sat.f64s.-inf", nanSat(6, nanF64(nanNegInfF64), false), 0x8000000000000000},
		{inc, "i64.sat.f32u.inf", nanSat(5, nanF32(nanInfF32), false), 0xFFFFFFFFFFFFFFFF},
	}
}

// nanHexFunc adds hex(v i64, p i32): writes v as 16 lowercase hex digits
// at p. It returns the function index.
func nanHexFunc(g *guest) uint32 {
	g.b.Data(nanHexTable, []byte("0123456789abcdef"))
	code := cat(
		i32c(16), []byte{wasmtest.OpLocalSet, 2},
		[]byte{wasmtest.OpLoop, wasmtest.Empty},
		// addr = p + i - 1
		[]byte{wasmtest.OpLocalGet, 1, wasmtest.OpLocalGet, 2, wasmtest.OpI32Add}, i32c(1), []byte{wasmtest.OpI32Sub},
		// table[v & 15]
		[]byte{wasmtest.OpLocalGet, 0}, i64c(15), []byte{nanOpI64And, nanOpI32WrapI64}, i32c(nanHexTable),
		[]byte{wasmtest.OpI32Add, nanOpI32Load8U, 0, 0, nanOpI32Store8, 0, 0},
		// v >>= 4
		[]byte{wasmtest.OpLocalGet, 0}, i64c(4), []byte{nanOpI64ShrU, wasmtest.OpLocalSet, 0},
		// i -= 1; br_if i != 0
		[]byte{wasmtest.OpLocalGet, 2}, i32c(1), []byte{wasmtest.OpI32Sub, wasmtest.OpLocalTee, 2, wasmtest.OpBrIf, 0},
		[]byte{wasmtest.OpEnd, wasmtest.OpEnd},
	)
	return g.b.Func([]byte{wasmtest.I64, wasmtest.I32}, nil, []byte{wasmtest.I32}, code...)
}

// nanModule is the NaN builder module: each case is one message, the
// hooks on_hour (f32), on_day (f64) and on_incident (sat-trunc).
func nanModule() []byte {
	g := newGuest()
	hex := nanHexFunc(g)
	bodies := map[scripting.Hook][]byte{}
	for i, c := range nanCases() {
		at := int64(nanTextAt + i*nanTextStride)
		g.b.Data(uint32(at), []byte(c.label+"="))
		n := int64(len(c.label) + 1)
		bodies[c.hook] = cat(bodies[c.hook], c.code, i32c(at+n), []byte{wasmtest.OpCall, byte(hex)},
			call("message", false, i32c(at), i32c(n+16)))
	}
	for hk, body := range bodies {
		g.hook(hk.String(), body)
	}
	return g.bytes()
}

// nanWant is the message every engine must send for case c.
func nanWant(c nanCase) string { return fmt.Sprintf("%s=%016x", c.label, c.want) }

// C6: every NaN producer's result reaches the message canonical, the
// sat-trunc edges are exact, and the untouched neg keeps its payload,
// through the host on both engines.
func TestNaNBuilderModuleMessagesAreCanonical(t *testing.T) {
	cases := nanCases()
	if len(cases) > 3*scripting.MaxCommands {
		t.Fatalf("%d cases do not fit three hooks", len(cases))
	}
	goldenEachEngine(t, func(t *testing.T, o Options) {
		h := newHost(t, o, plug("nan", "scripts/nan.wasm", nanModule()))
		var got []string
		for _, hk := range []scripting.Hook{scripting.OnHour, scripting.OnDay, scripting.OnIncident} {
			ev := event(hk)
			ev.Incident = scripting.Incident{Key: "core.raid", Category: "threat"}
			res := fire(h, ev)
			if len(res.Disabled) != 0 {
				t.Fatalf("%s: disabled %+v", hk, res.Disabled)
			}
			got = append(got, messages(res.Commands)...)
		}
		if len(got) != len(cases) {
			t.Fatalf("%d messages, want %d: %q", len(got), len(cases), got)
		}
		for i, c := range cases {
			if want := nanWant(c); got[i] != want {
				t.Errorf("%s: got %q, want %q", c.label, got[i], want)
			}
		}
		if t.Failed() {
			t.Logf("messages:\n%s", strings.Join(got, "\n"))
		}
	})
}

// The hex helper itself is exact, so a canonical message is not an
// artefact of the formatting: a known non-NaN value prints as itself.
func TestNaNHexHelperPrintsTheBits(t *testing.T) {
	g := newGuest()
	hex := nanHexFunc(g)
	g.b.Data(nanTextAt, []byte("v="))
	g.hook("on_hour", i64c(0x0123456789abcdef), i32c(nanTextAt+2), []byte{wasmtest.OpCall, byte(hex)},
		call("message", false, i32c(nanTextAt), i32c(18)))
	h := newHost(t, Options{Engine: Interpreter}, plug("hex", "scripts/hex.wasm", g.bytes()))
	res := fire(h, event(scripting.OnHour))
	if got := messages(res.Commands); len(got) != 1 || got[0] != "v=0123456789abcdef" {
		t.Errorf("got %q (disabled %+v)", got, res.Disabled)
	}
}
