// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmhost

import (
	"testing"

	"rimworld-go/internal/adapters/outbound/wasmhost/wasmtest"
	"rimworld-go/internal/application/scripting"
	"rimworld-go/internal/domain/sim/command"
)

type fakeWorld struct{}

func (fakeWorld) Day() int       { return 4 }
func (fakeWorld) Hour() int      { return 6 }
func (fakeWorld) Season() string { return "summer" }
func (fakeWorld) Colonists() int { return 3 }
func (fakeWorld) Wealth() int    { return 1234 }
func (fakeWorld) ItemCount(key string) (int, bool) {
	if key == "core.wood" {
		return 75, true
	}
	return 0, false
}

type fakePawns []scripting.Pawn

func (f fakePawns) Pawns() []scripting.Pawn { return f }
func (f fakePawns) Pawn(id uint64) (scripting.Pawn, bool) {
	for _, p := range f {
		if p.ID == id {
			return p, true
		}
	}
	return scripting.Pawn{}, false
}

func colonists() []scripting.Pawn {
	return []scripting.Pawn{
		{ID: 7, Name: "Ann", X: 1, Y: -2, Food: 30, Rest: 40, Mood: 50, Downed: true},
		{ID: 9, Name: "Bo", X: 5, Y: 6, Food: 1, Rest: 2, Mood: 3, Drafted: true},
		{ID: 12, Name: "Cyd"},
	}
}

// engines are the two engines every host test runs on; Auto is the
// compiler on amd64 and arm64.
var engines = map[string]Engine{"interpreter": Interpreter, "auto": Auto}

// eachEngine runs f once per engine, serially.
func eachEngine(t *testing.T, f func(t *testing.T, o Options)) {
	t.Helper()
	for name, e := range engines {
		t.Run(name, func(t *testing.T) { f(t, Options{Engine: e}) })
	}
}

func newHost(t *testing.T, o Options, ps ...Plugin) *Host {
	t.Helper()
	h := New(ps, o)
	t.Cleanup(func() { _ = h.Close() })
	return h
}

func plug(id, path string, wasm []byte) Plugin {
	return Plugin{ID: id, Files: []File{{Path: path, Wasm: wasm}}}
}

func fixture(t *testing.T, id, name string) Plugin {
	t.Helper()
	return plug(id, "scripts/"+name+".wasm", readFixture(t, name))
}

func event(hk scripting.Hook) scripting.Event {
	return scripting.Event{Hook: hk, Tick: 2500, Seed: 7}
}

func fire(h *Host, ev scripting.Event) scripting.Result {
	return h.Fire(ev, fakeWorld{}, fakePawns(colonists()))
}

func messages(cmds []command.Command) []string {
	var out []string
	for _, c := range cmds {
		out = append(out, c.Name)
	}
	return out
}

// guest builds small modules that call the imports with constant
// arguments. Memory: the event buffer at bufAt (bufLen bytes), strings from
// strAt up.
type guest struct {
	b     wasmtest.Builder
	next  uint32
	hooks map[string][]byte
	extra map[string][]byte // other exports: name -> body
	sigs  map[string][2][]byte
}

const (
	bufAt  = 4096
	bufLen = 4096
	strAt  = 16384
	outAt  = 8192
)

// import indices, in declaration order
var impIdx = map[string]byte{
	"message": 0, "letter": 1, "spawn": 2, "rand": 3, "count": 4, "day": 5,
	"hour": 6, "colonists": 7, "wealth": 8, "season": 9, "pawns": 10, "pawn": 11, "pawn_name": 12,
}

func newGuest() *guest {
	g := &guest{next: strAt, hooks: map[string][]byte{}, extra: map[string][]byte{}, sigs: map[string][2][]byte{}}
	i, l := wasmtest.I32, wasmtest.I64
	g.b.Memory(1, 2)
	g.b.Import("rimworld", "message", []byte{i, i}, nil)
	g.b.Import("rimworld", "letter", []byte{i, i, i, i}, nil)
	g.b.Import("rimworld", "spawn", []byte{i, i, i}, nil)
	g.b.Import("rimworld", "rand", []byte{i}, []byte{i})
	g.b.Import("rimworld", "count", []byte{i, i}, []byte{i})
	g.b.Import("rimworld", "day", nil, []byte{i})
	g.b.Import("rimworld", "hour", nil, []byte{i})
	g.b.Import("rimworld", "colonists", nil, []byte{i})
	g.b.Import("rimworld", "wealth", nil, []byte{l})
	g.b.Import("rimworld", "season", []byte{i, i}, []byte{i})
	g.b.Import("rimworld", "pawns", []byte{i, i}, []byte{i})
	g.b.Import("rimworld", "pawn", []byte{l, i}, []byte{i})
	g.b.Import("rimworld", "pawn_name", []byte{l, i, i}, []byte{i})
	return g
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

func i32c(v int64) []byte { return append([]byte{wasmtest.OpI32Const}, sleb(v)...) }
func i64c(v int64) []byte { return append([]byte{wasmtest.OpI64Const}, sleb(v)...) }

// str places s in memory and returns its (ptr, len) as two constants.
func (g *guest) str(s string) []byte {
	p := g.next
	g.b.Data(p, []byte(s))
	g.next += uint32(len(s))
	return append(i32c(int64(p)), i32c(int64(len(s)))...)
}

// call is a call of imp with args; a result is dropped.
func call(imp string, drop bool, args ...[]byte) []byte {
	var out []byte
	for _, a := range args {
		out = append(out, a...)
	}
	out = append(out, wasmtest.OpCall, impIdx[imp])
	if drop {
		out = append(out, wasmtest.OpDrop)
	}
	return out
}

func cat(parts ...[]byte) []byte {
	var out []byte
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

func (g *guest) hook(name string, body ...[]byte) *guest {
	g.hooks[name] = cat(body...)
	return g
}

// bytes emits the module: rw_buf and rw_buf_size, the hooks, extra exports.
func (g *guest) bytes() []byte {
	b := &g.b
	b.Export("memory", wasmtest.KindMemory, 0)
	b.Export("rw_buf", wasmtest.KindFunc, b.Func(nil, []byte{wasmtest.I32}, nil, append(i32c(bufAt), wasmtest.OpEnd)...))
	b.Export("rw_buf_size", wasmtest.KindFunc, b.Func(nil, []byte{wasmtest.I32}, nil, append(i32c(bufLen), wasmtest.OpEnd)...))
	for _, name := range scripting.Hooks() {
		if body, ok := g.hooks[name.String()]; ok {
			b.Export(name.String(), wasmtest.KindFunc, b.Func(nil, nil, nil, append(append([]byte(nil), body...), wasmtest.OpEnd)...))
		}
	}
	for name, body := range g.extra {
		s := g.sigs[name]
		b.Export(name, wasmtest.KindFunc, b.Func(s[0], s[1], nil, append(append([]byte(nil), body...), wasmtest.OpEnd)...))
	}
	return b.Bytes()
}

// export adds a non-hook export with signature params -> results.
func (g *guest) export(name string, params, results []byte, body ...[]byte) *guest {
	g.extra[name] = cat(body...)
	g.sigs[name] = [2][]byte{params, results}
	return g
}
