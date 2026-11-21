// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"

	"rimworld-go/internal/adapters/inbound/ebiten/plugins"
	"rimworld-go/internal/adapters/outbound/starlarkhost"
	"rimworld-go/internal/adapters/outbound/wasmhost"
	"rimworld-go/internal/adapters/outbound/wasmhost/wasmtest"
	appgame "rimworld-go/internal/application/game"
	"rimworld-go/internal/application/modding"
	"rimworld-go/internal/application/scripting"
	"rimworld-go/internal/domain/sim/clock"
)

// starPlugin is a loaded plugin with one Starlark file.
func starPlugin(id, path, src string) modding.Plugin {
	return modding.Plugin{Manifest: modding.Manifest{ID: id}, Scripts: []modding.Blob{{Path: path, Data: []byte(src)}}}
}

// wasmPlugin is a loaded plugin with one Wasm module.
func wasmPlugin(id, path string, wasm []byte) modding.Plugin {
	return modding.Plugin{Manifest: modding.Manifest{ID: id}, Wasm: []modding.Blob{{Path: path, Data: wasm}}}
}

// testMux is newScriptMux on the interpreter, closed with the test.
func testMux(t *testing.T, ps ...modding.Plugin) *scriptMux {
	t.Helper()
	m := newScriptMux(ps, wasmhost.Options{Engine: wasmhost.Interpreter})
	t.Cleanup(func() {
		if err := m.Close(); err != nil {
			t.Error(err)
		}
	})
	return m
}

func tallyWasm(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "adapters", "outbound", "wasmhost", "testdata", "tally.wasm"))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// guest is a small module built in Go: on_hour posts hour (traps when
// trapHour), and with state it exports state_version 1, a save that writes
// nothing (traps when trapSave) and a load that accepts anything.
type guest struct {
	hour            string
	trapHour, state bool
	trapSave        bool
}

func sleb(v int64) []byte {
	var out []byte
	for {
		b := byte(v & 0x7F)
		v >>= 7
		if (v == 0 && b&0x40 == 0) || (v == -1 && b&0x40 != 0) {
			return append(out, b)
		}
		out = append(out, b|0x80)
	}
}

func i32(v int64) []byte { return append([]byte{wasmtest.OpI32Const}, sleb(v)...) }

func body(parts ...[]byte) []byte {
	var out []byte
	for _, p := range parts {
		out = append(out, p...)
	}
	return append(out, wasmtest.OpEnd)
}

func (g guest) bytes() []byte {
	const bufAt, bufLen, strAt = 4096, 4096, 16384
	var b wasmtest.Builder
	i := wasmtest.I32
	msg := b.Import("rimworld", "message", []byte{i, i}, nil)
	b.Memory(1, 2)
	b.Data(strAt, []byte(g.hour))
	b.Export("memory", wasmtest.KindMemory, 0)
	b.Export("rw_buf", wasmtest.KindFunc, b.Func(nil, []byte{i}, nil, body(i32(bufAt))...))
	b.Export("rw_buf_size", wasmtest.KindFunc, b.Func(nil, []byte{i}, nil, body(i32(bufLen))...))
	hour := body(i32(strAt), i32(int64(len(g.hour))), []byte{wasmtest.OpCall, byte(msg)})
	if g.trapHour {
		hour = body([]byte{wasmtest.OpUnreachable})
	}
	b.Export("on_hour", wasmtest.KindFunc, b.Func(nil, nil, nil, hour...))
	if g.state {
		save := body(i32(0))
		if g.trapSave {
			save = body([]byte{wasmtest.OpUnreachable})
		}
		b.Export("state_version", wasmtest.KindFunc, b.Func(nil, []byte{i}, nil, body(i32(1))...))
		b.Export("save", wasmtest.KindFunc, b.Func(nil, []byte{i}, nil, save...))
		b.Export("load", wasmtest.KindFunc, b.Func([]byte{i}, []byte{i}, nil, body(i32(0))...))
	}
	return b.Bytes()
}

func hourEvent(hour int) scripting.Event {
	return scripting.Event{Hook: scripting.OnHour, Tick: uint64(hour * clock.TicksPerHour), Seed: 1}
}

func names(r scripting.Result) []string {
	var out []string
	for _, c := range r.Commands {
		out = append(out, c.Name)
	}
	return out
}

// C1 (AC-1): plugins run in load order across runtimes, Starlark and Wasm
// interleaved per plugin, through Fire and through a colony.
func TestMuxInterleavesPluginsInLoadOrder(t *testing.T) {
	t.Parallel()
	say := func(s string) string { return "def on_hour(ctx):\n    ctx.message(\"" + s + "\")\n" }
	b := wasmPlugin("B", "scripts/b.wasm", guest{hour: "B"}.bytes())
	b.Scripts = []modding.Blob{{Path: "scripts/b.star", Data: []byte(say("B star"))}}
	m := testMux(t, starPlugin("A", "scripts/a.star", say("A")), b, starPlugin("C", "scripts/c.star", say("C")))
	if p := m.Late(); len(p) != 0 {
		t.Fatalf("problems: %+v", p)
	}
	if got := strings.Join(names(m.Fire(hourEvent(1), fixedWorld{}, nil)), ","); got != "A,B star,B,C" {
		t.Fatalf("Fire order = %s, want A,B star,B,C", got)
	}
	g := appgame.New(appgame.Config{Seed: 1, Width: 8, Height: 8})
	m.Reset()
	m.attach(g)
	g.Step(2)
	var got []string
	for _, msg := range g.World().Messages {
		got = append(got, msg.Text)
	}
	if s := strings.Join(got, ","); !strings.Contains(s, "A,B star,B,C") {
		t.Fatalf("colony messages = %s, want A,B star,B,C in order", s)
	}
}

// C2 (AC-1): a Starlark-only set through the mux is the single host's
// colony, and an overrun turns off that file only.
func TestMuxStarlarkOnlyMatchesSingleHost(t *testing.T) {
	t.Parallel()
	ps := []modding.Plugin{starPlugin("dice", "scripts/dice.star", randSpawner), starPlugin("pawns", "scripts/pawns.star", pawnSpawner)}
	one := starlarkhost.New([]starlarkhost.Plugin{
		{ID: "dice", Files: []starlarkhost.File{{Path: "scripts/dice.star", Src: []byte(randSpawner)}}},
		{ID: "pawns", Files: []starlarkhost.File{{Path: "scripts/pawns.star", Src: []byte(pawnSpawner)}}}})
	h1, w1 := runColonyHost(t, 11, 3, one)
	h2, w2 := runColonyHost(t, 11, 3, testMux(t, ps...))
	if h1 != h2 || w1 != w2 {
		t.Fatalf("mux differs from one host: hash %x/%x, wood %d/%d", h1, h2, w1, w2)
	}
	flood := "def on_hour(ctx):\n    for i in range(17):\n        ctx.message(\"x\")\n"
	m := testMux(t, starPlugin("flood", "scripts/flood.star", flood), starPlugin("ok", "scripts/ok.star",
		"def on_hour(ctx):\n    ctx.message(\"ok\")\n"))
	r := m.Fire(hourEvent(1), fixedWorld{}, nil)
	if len(r.Disabled) != 1 || r.Disabled[0].Plugin != "flood" || strings.Join(names(r), ",") != "ok" {
		t.Fatalf("overrun: %+v", r)
	}
	late := m.Late()
	if len(late) != 1 || late[0].Plugin != "flood" || !strings.HasPrefix(late[0].Msg, "script turned off for this run: ") {
		t.Fatalf("Late = %+v", late)
	}
}

// C3 (AC-1): Uses allocates nothing.
func TestMuxUsesDoesNotAllocate(t *testing.T) {
	m := testMux(t, starPlugin("A", "scripts/a.star", randSpawner), wasmPlugin("B", "scripts/b.wasm", guest{hour: "B"}.bytes()))
	if !m.Uses(scripting.OnHour) || m.Uses(scripting.OnPawn) {
		t.Fatal("Uses: on_hour is used, on_pawn is not")
	}
	if n := testing.AllocsPerRun(100, func() { _ = m.Uses(scripting.OnPawn) }); n != 0 {
		t.Fatalf("Uses: %v allocs", n)
	}
}

// C3 (AC-1): a corrupt module and a trapping hook are Late problems, the
// rest run, the trap posts the in-game message and a load failure is
// listed once across games.
func TestMuxWasmProblems(t *testing.T) {
	t.Parallel()
	m := testMux(t,
		wasmPlugin("bad", "scripts/bad.wasm", []byte("not wasm")),
		wasmPlugin("trap", "scripts/trap.wasm", guest{trapHour: true}.bytes()),
		starPlugin("ok", "scripts/ok.star", "def on_hour(ctx):\n    ctx.message(\"ok\")\n"))
	late := m.Late()
	if len(late) != 1 || late[0].Plugin != "bad" || !strings.HasPrefix(late[0].Msg, "module turned off: ") {
		t.Fatalf("Late after load = %+v", late)
	}
	g := appgame.New(appgame.Config{Seed: 1, Width: 8, Height: 8})
	m.Reset()
	m.attach(g)
	g.Step(2)
	var ok, trapMsg bool
	for _, msg := range g.World().Messages {
		ok = ok || msg.Text == "ok"
		trapMsg = trapMsg || strings.Contains(msg.Text, "Plugin trap: script scripts/trap.wasm stopped")
	}
	if !ok || !trapMsg {
		t.Fatalf("messages: %+v", g.World().Messages)
	}
	trapLate := false
	for _, p := range m.Late() {
		trapLate = trapLate || p.Plugin == "trap" && strings.HasPrefix(p.Msg, "module turned off: ")
	}
	if !trapLate || !statOf(m.Stats(), "trap").Off {
		t.Fatalf("the trap is not a Late problem or not off: %+v, %+v", m.Late(), m.Stats())
	}
	m.Reset() // a second game keeps the load failure, once
	m.attach(appgame.New(appgame.Config{Seed: 1, Width: 8, Height: 8}))
	late = m.Late()
	var bad int
	for _, p := range late {
		if p.Plugin == "bad" {
			bad++
		}
	}
	if bad != 1 {
		t.Fatalf("the load failure is listed %d times: %+v", bad, late)
	}
	if st := statOf(m.Stats(), "trap"); st.Off {
		t.Fatalf("a new game turns the trapped file on again: %+v", st)
	}
}

func statOf(ss []plugins.ModStat, plugin string) plugins.ModStat {
	for _, s := range ss {
		if s.Plugin == plugin {
			return s
		}
	}
	return plugins.ModStat{}
}

// C9 (AC-3): Stats and Late read from another goroutine while the owner
// fires: no race, and fuel never goes down within a game.
func TestMuxStatsRace(t *testing.T) {
	t.Parallel()
	m := testMux(t, wasmPlugin("tally", "scripts/tally.wasm", tallyWasm(t)))
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(done)
		for h := 1; h <= 200; h++ {
			m.Fire(hourEvent(h), fixedWorld{}, nil)
		}
	}()
	var last uint64
	for reading := true; reading; {
		select {
		case <-done:
			reading = false
		default:
		}
		fuel := statOf(m.Stats(), "tally").Fuel
		if fuel < last {
			t.Fatalf("fuel went down: %d after %d", fuel, last)
		}
		last = fuel
		_ = m.Late()
	}
	wg.Wait()
	if st := statOf(m.Stats(), "tally"); st.Calls != 200 || st.Fuel == 0 {
		t.Fatalf("stats after 200 fires: %+v", st)
	}
}

// The one in-game notice stays within scripting.MaxText however many
// files it names.
func TestMuxNoticeIsCapped(t *testing.T) {
	t.Parallel()
	var notes []plugins.Problem
	for i := range 40 {
		notes = append(notes, plugins.Problem{Plugin: "plugin" + strings.Repeat("é", i), File: "scripts/a.wasm"})
	}
	s := notice("Plugin mod state on load: ", notes)
	if len(s) > scripting.MaxText || !strings.HasSuffix(s, " and more. See Plugins > Problems.") || !utf8.ValidString(s) {
		t.Fatalf("notice (%d bytes): %q", len(s), s)
	}
}
