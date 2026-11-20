// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmhost

import (
	"encoding/hex"
	"fmt"
	"hash/fnv"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"rimworld-go/internal/adapters/outbound/wasmhost/wasmtest"
	"rimworld-go/internal/application/scripting"
	"rimworld-go/internal/domain/sim/command"
)

// goldenHash is FNV-1a 64 of the golden transcript, the same on every
// engine and arch.
//
// Re-pin: GOFLAGS=-p=2 go test -race -count=1 -v -run TestGolden
// ./internal/adapters/outbound/wasmhost/ prints the hash both engines
// agree on. Pinned 2026-11-20 (PLG-063, first pin; darwin/arm64, checked
// under Rosetta on darwin/amd64). Re-pin in the same commit as, and name
// the trigger: a StackBudget, MaxDepth, DefaultFuel or import-cost change,
// a rewriter change that moves fuel, a fixture rebuild, or a change to
// the sequence or modules below.
const goldenHash = 0xa3bc661840bdc5fc

// goldenEngine is one engine the proofs run on.
type goldenEngine struct {
	name string
	e    Engine
}

// goldenEngines is the engine matrix: the interpreter always, the compiler
// on amd64 and arm64 (where it must run). Explicit, not Auto, so the
// compiler can never silently become the interpreter.
func goldenEngines() []goldenEngine {
	return []goldenEngine{{"interpreter", Interpreter}, {"compiler", Compiler}}
}

// goldenCompilerArch reports whether wazero's compiler runs on this arch.
func goldenCompilerArch() bool { return runtime.GOARCH == "amd64" || runtime.GOARCH == "arm64" }

// goldenEachEngine runs f per engine, serially. The compiler subtest skips
// only on an arch wazero has no compiler for; never under -race or -short.
func goldenEachEngine(t *testing.T, f func(t *testing.T, o Options)) {
	t.Helper()
	for _, ge := range goldenEngines() {
		t.Run(ge.name, func(t *testing.T) {
			if ge.e == Compiler && !goldenCompilerArch() {
				t.Skipf("wazero has no compiler for GOARCH=%s", runtime.GOARCH)
			}
			f(t, Options{Engine: ge.e})
		})
	}
}

// goldenPawns are the colony the pawn hooks see, like scriptbench's.
func goldenPawns() []scripting.Pawn {
	ps := make([]scripting.Pawn, 5)
	for i := range ps {
		id := uint64(i + 1)
		ps[i] = scripting.Pawn{
			ID: id, Name: "colonist" + strconv.Itoa(i), X: i % 3, Y: -i,
			Food: int(id * 37 % 101), Rest: int(id * 53 % 101), Mood: int(id * 17 % 101),
			Downed: id%4 == 0, Drafted: id%2 == 0,
		}
	}
	return ps
}

// goldenEvents is the fixed sequence: 30 on_hour, 2 on_day, 3 on_pawns,
// 3 on_pawn, 1 on_incident and 1 on_colonist_died, interleaved.
func goldenEvents() []scripting.Event {
	ps := goldenPawns()
	var evs []scripting.Event
	tick := uint64(0)
	add := func(ev scripting.Event) {
		tick += 250
		ev.Tick, ev.Seed = tick, 20261120
		evs = append(evs, ev)
	}
	for i := range 30 {
		add(scripting.Event{Hook: scripting.OnHour})
		switch i {
		case 4, 14, 24:
			add(scripting.Event{Hook: scripting.OnPawns, Pawns: ps[:3+i/10]})
		case 7, 17, 27:
			add(scripting.Event{Hook: scripting.OnPawn, Pawns: ps[i/10 : i/10+2]})
		case 11, 23:
			add(scripting.Event{Hook: scripting.OnDay})
		case 15:
			add(scripting.Event{Hook: scripting.OnIncident, Incident: scripting.Incident{Key: "core.herd_crossing", Category: "nature"}})
		case 20:
			add(scripting.Event{Hook: scripting.OnColonistDied, Name: "colonist3"})
		}
	}
	return evs
}

// goldenTrap is a module whose hook runs code and then traps or not.
func goldenTrap(hook string, code ...[]byte) []byte {
	g := newGuest()
	g.hook(hook, cat(append([][]byte{call("message", false, g.str(hook+" before trap"))}, code...)...))
	return g.bytes()
}

// goldenBulk copies tick&4095 bytes with memory.copy on every on_hour, so
// the fuel it reports moves with n.
func goldenBulk() []byte {
	g := newGuest()
	n := cat(i32c(0), []byte{wasmtest.OpI32Load, 2}, sleb(bufAt+8), i32c(4095), []byte{0x71}) // i32.and
	g.hook("on_hour",
		i32c(outAt), i32c(strAt), n, []byte{wasmtest.PrefixFC, wasmtest.FCMemoryCopy, 0, 0},
		call("message", false, g.str("copied")))
	return g.bytes()
}

// goldenPlugins are the golden's modules, in load order.
func goldenPlugins(t *testing.T) []Plugin {
	t.Helper()
	return []Plugin{
		fixture(t, "golden", "golden"),
		fixture(t, "state", "state"),
		plug("nan", "scripts/nan.wasm", nanModule()),
		plug("bulk", "scripts/bulk.wasm", goldenBulk()),
		plug("spin", "scripts/spin.wasm", fuelSpin("on_day")),
		plug("deep", "scripts/deep.wasm", depthSmall("on_incident")),
		plug("div0", "scripts/div0.wasm", goldenTrap("on_colonist_died", i32c(1), i32c(0), []byte{nanOpI32DivS, wasmtest.OpDrop})),
		plug("oob", "scripts/oob.wasm", goldenTrap("on_pawns", i32c(0x7FFFFFF0), []byte{wasmtest.OpI32Load, 2, 0, wasmtest.OpDrop})),
	}
}

// goldenWriter builds the transcript: one line per item, every string
// quoted, so no field can run into the next.
type goldenWriter struct{ b strings.Builder }

func (w *goldenWriter) line(format string, a ...any) { fmt.Fprintf(&w.b, format+"\n", a...) }

func (w *goldenWriter) commands(cmds []command.Command) {
	for _, c := range cmds {
		w.line("cmd kind=%d mode=%d count=%d name=%s", c.Kind, c.Mode, c.Count, strconv.Quote(c.Name))
	}
}

func (w *goldenWriter) problems(tag string, ps []scripting.Problem) {
	for _, p := range ps {
		w.line("%s plugin=%s file=%s line=%d msg=%s", tag, strconv.Quote(p.Plugin), strconv.Quote(p.File), p.Line, strconv.Quote(p.Msg))
	}
}

func (w *goldenWriter) stats(st []Stat) {
	for _, s := range st {
		w.line("stat plugin=%s file=%s fuel=%d calls=%d pages=%d off=%t", strconv.Quote(s.Plugin), strconv.Quote(s.File), s.Fuel, s.Calls, s.Pages, s.Off)
	}
}

// goldenRun runs the sequence on engine o and returns the transcript and
// the golden module's call count.
func goldenRun(t *testing.T, o Options) (string, uint64) {
	t.Helper()
	h := newHost(t, o, goldenPlugins(t)...)
	pawns := fakePawns(goldenPawns())
	var w goldenWriter
	for i, ev := range goldenEvents() {
		res := h.Fire(ev, fakeWorld{}, pawns)
		w.line("event %d %s tick=%d", i, ev.Hook, ev.Tick)
		w.commands(res.Commands)
		w.problems("disabled", res.Disabled)
		w.stats(h.Stats())
	}
	blob, err := h.Save("state", "scripts/state.wasm")
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	w.line("save %s", hex.EncodeToString(blob))
	w.problems("problem", h.Problems())
	return w.b.String(), h.Stats()[0].Calls
}

func goldenSum(s string) uint64 {
	f := fnv.New64a()
	_, _ = f.Write([]byte(s))
	return f.Sum64()
}

// goldenFirstDiff names the first line where a and b differ.
func goldenFirstDiff(a, b string) string {
	la, lb := strings.Split(a, "\n"), strings.Split(b, "\n")
	for i := 0; i < len(la) || i < len(lb); i++ {
		var x, y string
		if i < len(la) {
			x = la[i]
		}
		if i < len(lb) {
			y = lb[i]
		}
		if x != y {
			return fmt.Sprintf("line %d:\n  %s\n  %s", i+1, x, y)
		}
	}
	return "none"
}

// C1: the fixed sequence over golden.wasm, the state fixture and the
// builder modules (NaN, bulk, spin, deep recursion, div by zero, OOB load)
// gives the same transcript on both engines, and its hash is pinned. It
// never skips under -race or -short.
func TestGoldenHashIsPinnedOnBothEngines(t *testing.T) {
	// golden.wasm runs on every event: 30 + 2 + 3 + 1 + 1 calls, plus one
	// per due pawn of the three on_pawn events (2 each).
	const wantCalls = 30 + 2 + 3 + 1 + 1 + 3*2
	if n := len(goldenEvents()); n != 40 {
		t.Fatalf("%d events, want 40", n)
	}
	var first, firstName string
	goldenEachEngine(t, func(t *testing.T, o Options) {
		got, calls := goldenRun(t, o)
		if calls != wantCalls {
			t.Errorf("golden.wasm made %d calls, want %d", calls, wantCalls)
		}
		sum := goldenSum(got)
		t.Logf("golden hash %#016x on %s/%s (%d bytes of transcript)", sum, runtime.GOOS, runtime.GOARCH, len(got))
		if first == "" {
			first, firstName = got, t.Name()
		} else if got != first {
			t.Errorf("transcript differs from %s at %s", firstName, goldenFirstDiff(first, got))
		}
		if sum != goldenHash {
			t.Errorf("hash %#016x, pinned %#016x", sum, uint64(goldenHash))
		}
	})
}

// The transcript holds what the hash is meant to pin: each trapping
// module's problem, the fuel spent and the save blob.
func TestGoldenTranscriptCoversEveryModule(t *testing.T) {
	got, _ := goldenRun(t, Options{Engine: Interpreter})
	for _, want := range []string{
		`msg="out of fuel in on_day (budget 1000000)"`,
		`msg="call stack over budget in on_incident"`,
		`plugin="div0" file="scripts/div0.wasm" line=0 msg="trap in on_colonist_died: `,
		`plugin="oob" file="scripts/oob.wasm" line=0 msg="trap in on_pawns: `,
		`name="f32.div00=000000007fc00000"`,
		`name="copied"`,
		`name="count 30"`,
		"save 52574d53",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("transcript lacks %s", want)
		}
	}
	for _, unwanted := range []string{"spin before", "on_colonist_died before trap", "on_pawns before trap"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("transcript keeps %q from a call that failed", unwanted)
		}
	}
}

// The golden is not a constant transcript: one more tick moves the hash.
func TestGoldenHashMovesWithTheSequence(t *testing.T) {
	h := newHost(t, Options{Engine: Interpreter}, goldenPlugins(t)[:1]...)
	ev := goldenEvents()[0]
	a := fmt.Sprint(h.Fire(ev, fakeWorld{}, nil).Commands)
	ev.Tick++
	if b := fmt.Sprint(h.Fire(ev, fakeWorld{}, nil).Commands); a == b {
		t.Errorf("tick %d and %d give the same commands %s", ev.Tick-1, ev.Tick, a)
	}
}
