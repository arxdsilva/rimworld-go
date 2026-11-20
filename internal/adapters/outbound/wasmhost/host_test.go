// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmhost

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"

	"rimworld-go/internal/application/scripting"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/testkit/allocs"
	"rimworld-go/pkg/randx"
)

// draw is the n-th range draw of the call keyed as the contract says,
// computed without the host.
func draw(plugin, path string, ev scripting.Event, sub uint64, n int) int {
	k := randx.NewKeyed(ev.Seed, ev.Tick, fnv64a(plugin+"\x00"+path), uint64(ev.Hook), sub)
	return k.IntN(n)
}

// echo is what the hooks fixture sends for one call.
func echo(name string, ev scripting.Event, nrec, l1, l2, rnd int) string {
	return fmt.Sprintf("%s tick=%d nrec=%d l1=%d l2=%d day=4 hour=6 col=3 wealth=1234 wood=75 rand=%d "+
		"season=6:summer pawns=3 first=7 pawn=1 food=30 nl=3:Ann miss=0,-1", name, ev.Tick, nrec, l1, l2, rnd)
}

func wantCmds(name, echoMsg, t1, t2 string) []command.Command {
	return []command.Command{
		{Kind: command.ScriptMessage, Mode: command.ScriptNote, Name: echoMsg},
		{Kind: command.ScriptMessage, Mode: command.ScriptLetter, Name: name + "\n" + "t1=" + t1 + " t2=" + t2},
		{Kind: command.ScriptSpawn, Name: "core.wood", Count: 2},
	}
}

// C1: the hooks fixture gets the right header, records and texts on every
// hook, and plugins queue in load order.
func TestHostHooksQueueInLoadOrderWithRightEvent(t *testing.T) {
	eachEngine(t, func(t *testing.T, o Options) {
		h := newHost(t, o, fixture(t, "b_first", "hooks"), fixture(t, "a_second", "hooks"))
		if p := h.Problems(); len(p) != 0 {
			t.Fatalf("problems: %+v", p)
		}
		cases := []struct {
			name                string
			ev                  scripting.Event
			nrec, l1, l2        int
			t1, t2              string
			sub                 uint64
			hookSub, hookSubTxt string
		}{
			{name: "on_hour", ev: event(scripting.OnHour), sub: fnv64a("\x00")},
			{name: "on_day", ev: event(scripting.OnDay), sub: fnv64a("\x00")},
			{name: "on_incident", ev: func() scripting.Event {
				e := event(scripting.OnIncident)
				e.Incident = scripting.Incident{Key: "core.herd_crossing", Category: "nature"}
				return e
			}(), l1: 18, l2: 6, t1: "core.herd_crossing", t2: "nature", sub: fnv64a("core.herd_crossing\x00")},
			{name: "on_colonist_died", ev: func() scripting.Event {
				e := event(scripting.OnColonistDied)
				e.Name = "Ann Smith"
				return e
			}(), l1: 9, t1: "Ann Smith", sub: fnv64a("\x00Ann Smith")},
			{name: "on_pawns", ev: func() scripting.Event {
				e := event(scripting.OnPawns)
				e.Pawns = colonists()[:2]
				return e
			}(), nrec: 2, sub: fnv64a("\x00")},
		}
		for _, c := range cases {
			res := fire(h, c.ev)
			if len(res.Disabled) != 0 {
				t.Fatalf("%s: disabled %+v", c.name, res.Disabled)
			}
			var want []command.Command
			for _, p := range []string{"b_first", "a_second"} {
				rnd := draw(p, "scripts/hooks.wasm", c.ev, c.sub, 1000)
				want = append(want, wantCmds(c.name, echo(c.name, c.ev, c.nrec, c.l1, c.l2, rnd), c.t1, c.t2)...)
			}
			if !reflect.DeepEqual(res.Commands, want) {
				t.Errorf("%s:\n got %+v\nwant %+v", c.name, messages(res.Commands), messages(want))
			}
		}
	})
}

// on_pawn: header rewritten per pawn, rand rekeyed per pawn id.
func TestHostOnPawnRewritesTheHeaderPerPawn(t *testing.T) {
	eachEngine(t, func(t *testing.T, o Options) {
		h := newHost(t, o, fixture(t, "p", "hooks"))
		ev := event(scripting.OnPawn)
		ev.Pawns = colonists()[:2]
		res := fire(h, ev)
		if len(res.Disabled) != 0 || len(res.Commands) != 6 {
			t.Fatalf("got %d commands, disabled %+v", len(res.Commands), res.Disabled)
		}
		for i, p := range ev.Pawns {
			rnd := draw("p", "scripts/hooks.wasm", ev, fnv64a(strconv.FormatUint(p.ID, 10)), 1000)
			if got, want := res.Commands[3*i].Name, echo("on_pawn", ev, 1, 0, 0, rnd); got != want {
				t.Errorf("pawn %d:\n got %q\nwant %q", p.ID, got, want)
			}
		}
	})
}

func TestHostNoPawnsNoCall(t *testing.T) {
	h := newHost(t, Options{Engine: Interpreter}, fixture(t, "p", "hooks"))
	for _, hk := range []scripting.Hook{scripting.OnPawns, scripting.OnPawn} {
		if res := fire(h, event(hk)); len(res.Commands) != 0 || len(res.Disabled) != 0 {
			t.Errorf("%v with nobody due: %+v", hk, res)
		}
	}
	if st := h.Stats(); st[0].Calls != 0 {
		t.Errorf("calls = %d, want 0", st[0].Calls)
	}
}

// C2: a trap disables only the failing file and drops its commands.
func TestHostTrapDisablesOnlyThatFileAndDropsItsCommands(t *testing.T) {
	eachEngine(t, func(t *testing.T, o Options) {
		g := newGuest()
		bad := g.hook("on_hour", call("message", false, g.str("before the trap")), []byte{0x00}).bytes() // unreachable
		h := newHost(t, o,
			plug("bad", "scripts/bad.wasm", bad),
			fixture(t, "good", "dice"))
		res := fire(h, event(scripting.OnHour))
		if len(res.Disabled) != 1 {
			t.Fatalf("disabled = %+v", res.Disabled)
		}
		d := res.Disabled[0]
		if d.Plugin != "bad" || d.File != "scripts/bad.wasm" || d.Line != 0 || !strings.HasPrefix(d.Msg, "trap in on_hour: ") || strings.Contains(d.Msg, "\n") {
			t.Errorf("problem = %+v", d)
		}
		for _, c := range res.Commands {
			if strings.Contains(c.Name, "before the trap") {
				t.Errorf("a trapped file's command survived: %+v", c)
			}
		}
		if len(res.Commands) != 2 { // dice: message and spawn
			t.Errorf("good file's commands = %+v", res.Commands)
		}
		if h.Uses(scripting.OnHour) != true { // dice is still live
			t.Error("Uses(OnHour) must stay true through the live file")
		}
		again := fire(h, event(scripting.OnHour))
		if len(again.Disabled) != 0 || len(again.Commands) != 2 {
			t.Errorf("second fire: %+v", again)
		}
		if p := h.Problems(); len(p) != 1 || p[0] != d {
			t.Errorf("Problems() = %+v", p)
		}
		if st := h.Stats(); !st[0].Off || st[1].Off {
			t.Errorf("stats = %+v", st)
		}
	})
}

// C3: load failures are reported on the first Fire only.
func TestHostLoadFailuresReportedOnFirstFireOnly(t *testing.T) {
	eachEngine(t, func(t *testing.T, o Options) {
		h := newHost(t, o,
			plug("junk", "scripts/junk.wasm", []byte("not wasm")),
			fixture(t, "good", "dice"))
		if p := h.Problems(); len(p) != 1 || p[0].Plugin != "junk" || p[0].File != "scripts/junk.wasm" || p[0].Line != 0 {
			t.Fatalf("Problems() = %+v", p)
		}
		first := fire(h, event(scripting.OnHour))
		if len(first.Disabled) != 1 || first.Disabled[0].Plugin != "junk" {
			t.Fatalf("first Fire: %+v", first.Disabled)
		}
		if len(first.Commands) != 2 {
			t.Errorf("the good file still runs: %+v", first.Commands)
		}
		if again := fire(h, event(scripting.OnHour)); len(again.Disabled) != 0 {
			t.Errorf("second Fire repeats the load failure: %+v", again.Disabled)
		}
		if len(h.Problems()) != 1 {
			t.Errorf("Problems() = %+v", h.Problems())
		}
	})
}

// Every way a file may fail to load is one Problem, and the file never runs.
func TestHostLoadRejects(t *testing.T) {
	hook := func(g *guest) *guest { return g.hook("on_hour", call("day", true)) }
	cases := []struct {
		name string
		wasm func() []byte
		want string
	}{
		{"index out of range (C9: a future fuel global)", func() []byte {
			return newGuest().hook("on_hour", []byte{0x23, 0x00, 0x1A}).bytes() // global.get 0; drop, no globals
		}, "rejected: "},
		{"unknown import module", func() []byte {
			g := newGuest()
			g.b.Import("env", "x", nil, nil)
			return hook(g).bytes()
		}, "rejected: "},
		{"reserved export name", func() []byte {
			return hook(newGuest()).export("__rimworld_fuel", nil, nil).bytes()
		}, "rejected: "},
		{"wrong hook signature", func() []byte {
			return newGuest().export("on_hour", []byte{0x7F}, nil).bytes()
		}, "export on_hour is not () -> ()"},
		{"save without load", func() []byte {
			return hook(newGuest()).export("save", nil, []byte{0x7F}, i32c(0)).bytes()
		}, "must be exported together"},
		{"wrong state signature", func() []byte {
			g := hook(newGuest())
			g.export("state_version", nil, nil).export("save", nil, []byte{0x7F}, i32c(0)).export("load", nil, nil)
			return g.bytes()
		}, "wrong signature"},
		{"buffer past memory", func() []byte {
			g := newGuest()
			g.b.Export("memory", 0x02, 0)
			g.b.Export("rw_buf", 0, g.b.Func(nil, []byte{0x7F}, nil, append(i32c(65000), 0x0B)...))
			g.b.Export("rw_buf_size", 0, g.b.Func(nil, []byte{0x7F}, nil, append(i32c(4096), 0x0B)...))
			return g.b.Bytes()
		}, "rw_buf (65000, 4096 bytes)"},
		{"buffer under 64 bytes", func() []byte {
			g := newGuest()
			g.b.Export("memory", 0x02, 0)
			g.b.Export("rw_buf", 0, g.b.Func(nil, []byte{0x7F}, nil, append(i32c(64), 0x0B)...))
			g.b.Export("rw_buf_size", 0, g.b.Func(nil, []byte{0x7F}, nil, append(i32c(63), 0x0B)...))
			return g.b.Bytes()
		}, "rw_buf (64, 63 bytes)"},
		{"no rw_buf", func() []byte {
			g := newGuest()
			g.b.Export("memory", 0x02, 0)
			return g.b.Bytes()
		}, "export rw_buf is missing"},
		{"no memory", func() []byte {
			g := newGuest()
			g.b.Export("rw_buf", 0, g.b.Func(nil, []byte{0x7F}, nil, append(i32c(64), 0x0B)...))
			g.b.Export("rw_buf_size", 0, g.b.Func(nil, []byte{0x7F}, nil, append(i32c(64), 0x0B)...))
			return g.b.Bytes()
		}, "missing export memory"},
		{"rw_buf traps", func() []byte {
			g := newGuest()
			g.b.Export("memory", 0x02, 0)
			g.b.Export("rw_buf", 0, g.b.Func(nil, []byte{0x7F}, nil, 0x00, 0x0B))
			g.b.Export("rw_buf_size", 0, g.b.Func(nil, []byte{0x7F}, nil, append(i32c(64), 0x0B)...))
			return g.b.Bytes()
		}, "trap in rw_buf: "},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := newHost(t, Options{Engine: Interpreter}, plug("p", "scripts/x.wasm", c.wasm()))
			res := fire(h, event(scripting.OnHour))
			if len(res.Disabled) != 1 || !strings.Contains(res.Disabled[0].Msg, c.want) {
				t.Fatalf("disabled = %+v, want %q", res.Disabled, c.want)
			}
			if c.name[0] == 'i' && !strings.Contains(res.Disabled[0].Msg, "index out of range") {
				t.Errorf("C9 msg = %q", res.Disabled[0].Msg)
			}
			if h.Uses(scripting.OnHour) || len(res.Commands) != 0 || len(h.Stats()) != 0 {
				t.Error("a rejected file must not enter the module list")
			}
		})
	}
}

func TestHostEventTooLargeFailsTheCall(t *testing.T) {
	h := newHost(t, Options{Engine: Interpreter}, fixture(t, "p", "hooks"))
	ev := event(scripting.OnPawns)
	for i := 0; i < 200; i++ {
		ev.Pawns = append(ev.Pawns, scripting.Pawn{ID: uint64(i + 1)})
	}
	res := fire(h, ev)
	if len(res.Disabled) != 1 || !strings.Contains(res.Disabled[0].Msg, "does not fit the 4096 byte buffer") {
		t.Fatalf("disabled = %+v", res.Disabled)
	}
}

func TestHostErrMsgReadsTheGlobalsNotTheText(t *testing.T) {
	h := newHost(t, Options{Engine: Interpreter, Fuel: 500}, fixture(t, "p", "dice"))
	m := h.mods[0]
	cs := &callState{m: m, hook: "on_hour"}
	boom := errors.New("wasm error: unreachable\nwasm stack trace:\n\t.f()")
	if got := h.errMsg(cs, boom); got != "trap in on_hour: wasm error: unreachable" {
		t.Errorf("trap: %q", got)
	}
	m.depth.Set(StackBudget + 1)
	if got := h.errMsg(cs, boom); got != "call stack over budget in on_hour" {
		t.Errorf("depth: %q", got)
	}
	m.fuel.Set(^uint64(0)) // -1
	if got := h.errMsg(cs, boom); got != "out of fuel in on_hour (budget 500)" {
		t.Errorf("fuel: %q", got)
	}
	cs.fail = "message: empty text"
	if got := h.errMsg(cs, boom); got != "message: empty text" {
		t.Errorf("import: %q", got)
	}
}

// Uses follows the live modules, and does not allocate. Not parallel.
func TestUsesTracksLiveModules(t *testing.T) {
	g := newGuest()
	trapper := g.hook("on_pawn", []byte{0x00}).bytes()
	h := newHost(t, Options{Engine: Interpreter}, fixture(t, "p", "dice"), plug("q", "scripts/q.wasm", trapper))
	if h.Uses(scripting.OnPawns) || !h.Uses(scripting.OnPawn) || !h.Uses(scripting.OnHour) || h.Uses(scripting.Hook(0)) || h.Uses(scripting.Hook(99)) {
		t.Fatal("Uses must follow the exported hooks")
	}
	if n := allocs.Fewest(20, func() { h.Uses(scripting.OnPawn) }); n != 0 {
		t.Errorf("Uses allocates %v", n)
	}
	ev := event(scripting.OnPawn)
	ev.Pawns = colonists()
	if r := fire(h, ev); len(r.Disabled) != 1 || r.Disabled[0].Plugin != "q" {
		t.Fatalf("q should be disabled: %+v", r)
	}
	if !h.Uses(scripting.OnPawn) {
		t.Error("p still exports on_pawn")
	}
	h.mods[0].off = true
	if h.Uses(scripting.OnPawn) || h.Uses(scripting.OnHour) {
		t.Error("an off module is not used")
	}
}

// Several hosts built at once are race-free (wazero v1.9.0 races on
// runtime creation without the host's mutex).
func TestHostConcurrentNew(t *testing.T) {
	dice := readFixture(t, "dice")
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			h := New([]Plugin{plug("p", "scripts/dice.wasm", dice)}, Options{Engine: Interpreter})
			defer h.Close()
			if res := fire(h, event(scripting.OnHour)); len(res.Commands) != 2 {
				t.Errorf("commands = %+v", res.Commands)
			}
		}()
	}
	wg.Wait()
}

func TestHostCloseReleasesTheRuntime(t *testing.T) {
	h := New([]Plugin{fixture(t, "p", "dice")}, Options{Engine: Interpreter})
	if err := h.Close(); err != nil {
		t.Fatal(err)
	}
}
