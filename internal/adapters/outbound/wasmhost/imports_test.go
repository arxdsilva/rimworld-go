// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmhost

import (
	"bytes"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"rimworld-go/internal/application/scripting"
	"rimworld-go/internal/domain/sim/command"
)

// C4: each limit fails the whole call with the Starlark message.
func TestImportsFailWithTheStarlarkMessages(t *testing.T) {
	long := strings.Repeat("x", scripting.MaxText+1)
	cases := []struct {
		name string
		body func(g *guest) []byte
		want string
	}{
		{"empty message", func(g *guest) []byte { return call("message", false, g.str("  ")) }, "message: empty text"},
		{"long message", func(g *guest) []byte { return call("message", false, g.str(long)) }, "message: longer than 500 bytes"},
		{"title with newline", func(g *guest) []byte {
			return call("letter", false, g.str("a\nb"), g.str("text"))
		}, "letter: the title is one line"},
		{"empty title", func(g *guest) []byte { return call("letter", false, g.str(" "), g.str("text")) }, "letter title: empty text"},
		{"long text", func(g *guest) []byte { return call("letter", false, g.str("T"), g.str(long)) }, "letter: longer than 500 bytes"},
		{"spawn count 0", func(g *guest) []byte {
			return call("spawn", false, g.str("core.wood"), i32c(0))
		}, "spawn: count must be 1 to 500, got 0"},
		{"spawn count 501", func(g *guest) []byte {
			return call("spawn", false, g.str("core.wood"), i32c(501))
		}, "spawn: count must be 1 to 500, got 501"},
		{"spawn negative", func(g *guest) []byte {
			return call("spawn", false, g.str("core.wood"), i32c(-3))
		}, "spawn: count must be 1 to 500, got -3"},
		{"spawn unknown item", func(g *guest) []byte {
			return call("spawn", false, g.str("core.nope"), i32c(1))
		}, `spawn: unknown item "core.nope"`},
		{"count unknown item", func(g *guest) []byte {
			return call("count", true, g.str("core.nope"))
		}, `count: unknown item "core.nope"`},
		{"rand 0", func(g *guest) []byte { return call("rand", true, i32c(0)) }, "rand: n must be 1 to 1073741824, got 0"},
		{"rand too big", func(g *guest) []byte {
			return call("rand", true, i32c(1<<30+1))
		}, "rand: n must be 1 to 1073741824, got 1073741825"},
		{"rand negative", func(g *guest) []byte { return call("rand", true, i32c(-5)) }, "rand: n must be 1 to 1073741824, got -5"},
		{"seventeenth command", func(g *guest) []byte {
			var out []byte
			for i := 0; i < scripting.MaxCommands+1; i++ {
				out = append(out, call("message", false, g.str("m"))...)
			}
			return out
		}, "more than 16 commands in one call"},
		{"pointer out of range", func(g *guest) []byte {
			return call("message", false, i32c(-256), i32c(8))
		}, "message: pointer out of range"},
		{"length out of range", func(g *guest) []byte {
			return call("message", false, i32c(0), i32c(-1))
		}, "message: pointer out of range"},
		{"season pointer", func(g *guest) []byte {
			return call("season", true, i32c(65533), i32c(64))
		}, "season: pointer out of range"},
		{"pawns pointer", func(g *guest) []byte {
			return call("pawns", true, i32c(65530), i32c(3))
		}, "pawns: pointer out of range"},
		{"pawn pointer", func(g *guest) []byte {
			return call("pawn", true, i64c(7), i32c(65530))
		}, "pawn: pointer out of range"},
		{"pawn_name pointer", func(g *guest) []byte {
			return call("pawn_name", true, i64c(7), i32c(65535), i32c(8))
		}, "pawn_name: pointer out of range"},
	}
	eachEngine(t, func(t *testing.T, o Options) {
		for _, c := range cases {
			g := newGuest()
			// the first command is dropped with the failing call
			g.hook("on_hour", call("message", false, g.str("kept?")), c.body(g))
			h := newHost(t, o, plug("p", "scripts/x.wasm", g.bytes()))
			res := fire(h, event(scripting.OnHour))
			if len(res.Disabled) != 1 || res.Disabled[0].Msg != c.want {
				t.Errorf("%s: disabled = %+v, want %q", c.name, res.Disabled, c.want)
			}
			if len(res.Commands) != 0 {
				t.Errorf("%s: commands survive a failed call: %+v", c.name, res.Commands)
			}
		}
	})
}

// on_pawn: one command budget per file per tick, shared by its pawns.
func TestImportsOnPawnCommandBudgetIsPerTick(t *testing.T) {
	eachEngine(t, func(t *testing.T, o Options) {
		h := newHost(t, o, fixture(t, "p", "hooks")) // 3 commands per pawn
		ev := event(scripting.OnPawn)
		for i := 0; i < 5; i++ {
			ev.Pawns = append(ev.Pawns, scripting.Pawn{ID: uint64(i + 1), Name: "n"})
		}
		if res := fire(h, ev); len(res.Disabled) != 0 || len(res.Commands) != 15 {
			t.Fatalf("5 pawns: %d commands, %+v", len(res.Commands), res.Disabled)
		}
		ev.Pawns = append(ev.Pawns, scripting.Pawn{ID: 6, Name: "n"})
		res := fire(h, ev)
		if len(res.Disabled) != 1 || res.Disabled[0].Msg != "more than 16 commands in on_pawn's per-tick budget" || len(res.Commands) != 0 {
			t.Fatalf("6 pawns: %+v", res)
		}
	})
}

// C5: rand is keyed as the contract says; compared with randx directly.
func TestImportsRandIsKeyedPerPluginPathHookAndSub(t *testing.T) {
	eachEngine(t, func(t *testing.T, o Options) {
		h := newHost(t, o, fixture(t, "plug.a", "dice"), plug("plug.b", "scripts/other.wasm", readFixture(t, "dice")))
		ev := event(scripting.OnHour)
		res := fire(h, ev)
		var want []command.Command
		for _, f := range [][2]string{{"plug.a", "scripts/dice.wasm"}, {"plug.b", "scripts/other.wasm"}} {
			n := 1 + draw(f[0], f[1], ev, fnv64a("\x00"), 9)
			want = append(want,
				command.Command{Kind: command.ScriptMessage, Mode: command.ScriptNote, Name: "dropped " + strconv.Itoa(n) + " wood at 6:00"},
				command.Command{Kind: command.ScriptSpawn, Name: "core.wood", Count: n})
		}
		got := res.Commands
		if len(got) != 4 {
			t.Fatalf("commands = %+v", got)
		}
		// message then spawn per file; the fixture spawns first
		for i := 0; i < 2; i++ {
			if got[2*i].Kind != command.ScriptSpawn || got[2*i+1].Kind != command.ScriptMessage {
				t.Fatalf("order: %+v", got)
			}
			if got[2*i].Count != want[2*i+1].Count || got[2*i+1].Name != want[2*i].Name {
				t.Errorf("file %d: got %+v %+v, want %+v %+v", i, got[2*i], got[2*i+1], want[2*i+1], want[2*i])
			}
		}
		// a later tick, and the same call twice, draw the same numbers
		ev.Tick++
		a, b := fire(h, ev), fire(h, ev)
		if !reflect.DeepEqual(a.Commands, b.Commands) {
			t.Errorf("the same call draws different numbers: %+v vs %+v", a.Commands, b.Commands)
		}
	})
}

func TestImportsOnPawnRandIsKeyedPerPawnID(t *testing.T) {
	eachEngine(t, func(t *testing.T, o Options) {
		h := newHost(t, o, fixture(t, "p", "dice"))
		ev := event(scripting.OnPawn)
		var want []command.Command
		for id := uint64(1); id <= 12; id++ {
			ev.Pawns = append(ev.Pawns, scripting.Pawn{ID: id, Name: "n"})
			if draw("p", "scripts/dice.wasm", ev, fnv64a(strconv.FormatUint(id, 10)), 4) == 0 {
				want = append(want, command.Command{Kind: command.ScriptSpawn, Name: "core.wood", Count: 1 + int(id%3)})
			}
		}
		if len(want) == 0 {
			t.Fatal("no pawn rolls 0: pick other ids")
		}
		if res := fire(h, ev); !reflect.DeepEqual(res.Commands, want) {
			t.Errorf("got %+v\nwant %+v", res.Commands, want)
		}
	})
}

// C6: batched pawns() and per-call pawn(id) write identical records; a
// small cap writes only cap records.
func TestImportsBatchedPawnsEqualPerCallPawn(t *testing.T) {
	eachEngine(t, func(t *testing.T, o Options) {
		g := newGuest()
		g.hook("on_hour",
			call("pawns", true, i32c(0x2000), i32c(8)),
			call("pawn", true, i64c(7), i32c(0x3000)),
			call("pawn", true, i64c(9), i32c(0x3020)),
			call("pawn", true, i64c(12), i32c(0x3040)),
			call("pawns", true, i32c(0x4000), i32c(1)),
			call("pawns", true, i32c(0x5000), i32c(0)),
			call("pawn", true, i64c(0), i32c(0x6000)),
			call("pawn", true, i64c(99), i32c(0x6000)),
		)
		h := newHost(t, o, plug("p", "scripts/x.wasm", g.bytes()))
		if res := fire(h, event(scripting.OnHour)); len(res.Disabled) != 0 {
			t.Fatalf("disabled %+v", res.Disabled)
		}
		mem := h.mods[0].mem
		rd := func(at uint32, n uint32) []byte {
			b, ok := mem.Read(at, n)
			if !ok {
				t.Fatalf("read %d", at)
			}
			return b
		}
		if !bytes.Equal(rd(0x2000, 96), rd(0x3000, 96)) {
			t.Errorf("batched != per-call:\n%x\n%x", rd(0x2000, 96), rd(0x3000, 96))
		}
		if !bytes.Equal(rd(0x4000, 32), rd(0x2000, 32)) || !bytes.Equal(rd(0x4020, 64), make([]byte, 64)) {
			t.Error("cap 1 must write exactly one record")
		}
		if !bytes.Equal(rd(0x5000, 32), make([]byte, 32)) || !bytes.Equal(rd(0x6000, 32), make([]byte, 32)) {
			t.Error("cap 0 and a missing pawn write nothing")
		}
		// record layout: id, x, y, food, rest, mood, flags
		want := []byte{7, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0xFE, 0xFF, 0xFF, 0xFF, 30, 0, 0, 0, 40, 0, 0, 0, 50, 0, 0, 0, 1, 0, 0, 0}
		if !bytes.Equal(rd(0x2000, 32), want) {
			t.Errorf("record = %x, want %x", rd(0x2000, 32), want)
		}
		if rd(0x201C, 4)[0] != 1 || rd(0x203C, 4)[0] != 2 {
			t.Error("flags: downed is bit 0, drafted bit 1")
		}
	})
}

// A reader of nil answers an empty colony, as the Starlark ctx does.
func TestImportsNilPawnReader(t *testing.T) {
	g := newGuest()
	g.hook("on_hour", call("pawns", true, i32c(0x2000), i32c(8)), call("pawn", true, i64c(7), i32c(0x3000)),
		call("pawn_name", true, i64c(7), i32c(0x3000), i32c(8)))
	h := newHost(t, Options{Engine: Interpreter}, plug("p", "scripts/x.wasm", g.bytes()))
	if res := h.Fire(event(scripting.OnHour), fakeWorld{}, nil); len(res.Disabled) != 0 {
		t.Fatalf("disabled %+v", res.Disabled)
	}
}

// An import that drives fuel below 0 stops the call at once, doing nothing.
func TestImportsChargeFuelAndStopAtOnce(t *testing.T) {
	eachEngine(t, func(t *testing.T, o Options) {
		g := newGuest()
		g.hook("on_hour", call("spawn", false, g.str("core.wood"), i32c(1)))
		o.Fuel = 3 // any import costs 4
		h := newHost(t, o, plug("p", "scripts/x.wasm", g.bytes()))
		res := fire(h, event(scripting.OnHour))
		if len(res.Disabled) != 1 || res.Disabled[0].Msg != "out of fuel in on_hour (budget 3)" || len(res.Commands) != 0 {
			t.Fatalf("res = %+v", res)
		}
		if st := h.Stats()[0]; st.Fuel != 3 || !st.Off {
			t.Errorf("stats = %+v, want the whole budget spent", st)
		}
	})
}

func TestImportsChargeCostsPerWordAndRecord(t *testing.T) {
	g := newGuest()
	g.hook("on_hour",
		call("message", false, g.str(strings.Repeat("a", 64))), // 4 + 2
		call("pawns", true, i32c(0x2000), i32c(8)),             // 4 + 3
		call("day", true), // 4
	)
	h := newHost(t, Options{Engine: Interpreter, Fuel: 1000}, plug("p", "scripts/x.wasm", g.bytes()))
	fire(h, event(scripting.OnHour))
	left := int64(h.mods[0].fuel.Get())
	// The imports cost 6+7+4; Meter charges the body once at entry.
	if used := 1000 - left; used != 17+11 {
		t.Errorf("used %d fuel, want 17 for the imports + 11 for the hook's entry charge (1 + its 10 instructions, end included)", used)
	}
}

func TestImportsOutsideAHookHaveNoWorld(t *testing.T) {
	g := newGuest()
	g.b.Export("memory", 0x02, 0)
	g.b.Export("rw_buf", 0, g.b.Func(nil, []byte{0x7F}, nil, append(call("day", true), append(i32c(4096), 0x0B)...)...))
	g.b.Export("rw_buf_size", 0, g.b.Func(nil, []byte{0x7F}, nil, append(i32c(4096), 0x0B)...))
	h := newHost(t, Options{Engine: Interpreter}, plug("p", "scripts/x.wasm", g.b.Bytes()))
	if p := h.Problems(); len(p) != 1 || p[0].Msg != "day: not available outside a hook" {
		t.Fatalf("problems = %+v", p)
	}
}

// message, letter and spawn have no call to queue into during save and
// load: they fail the call (a Problem), as the world reads do.
func TestImportsCommandsOutsideAHookFail(t *testing.T) {
	for _, imp := range []string{"message", "letter", "spawn"} {
		g := newGuest()
		var body []byte
		switch imp {
		case "message":
			body = call("message", false, g.str("hi"))
		case "letter":
			body = call("letter", false, g.str("T"), g.str("x"))
		default:
			body = call("spawn", false, g.str("core.wood"), i32c(1))
		}
		g.export("state_version", nil, []byte{0x7F}, i32c(1)).export("save", nil, []byte{0x7F}, body, i32c(0)).
			export("load", []byte{0x7F}, []byte{0x7F}, i32c(0))
		h := newHost(t, Options{Engine: Interpreter}, plug("p", "scripts/x.wasm", g.bytes()))
		_, err := h.Save("p", "scripts/x.wasm")
		if err == nil || !strings.Contains(err.Error(), imp+": not available outside a hook") {
			t.Errorf("%s: err = %v", imp, err)
		}
	}
}

// Every write import checks its pointer before it charges, so a bad
// pointer is its own message even when the charge would not fit (the
// entry charge is 6, the import's 4 + 3 records would not).
func TestImportsBoundsComeBeforeTheCharge(t *testing.T) {
	g := newGuest()
	g.hook("on_hour", call("pawns", true, i32c(65530), i32c(3)))
	h := newHost(t, Options{Engine: Interpreter, Fuel: 12}, plug("p", "scripts/x.wasm", g.bytes()))
	res := fire(h, event(scripting.OnHour))
	if len(res.Disabled) != 1 || res.Disabled[0].Msg != "pawns: pointer out of range" {
		t.Fatalf("disabled = %+v", res.Disabled)
	}
}
