// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package scriptbench

import (
	"testing"

	"rimworld-go/internal/application/scripting"

	lua "github.com/yuin/gopher-lua"
)

const luaBatched = `
function on_hour(ctx)
  local low, mood = 0, 0
  local ps = ctx.pawns()
  for i = 1, #ps do
    local p = ps[i]
    if p.food < 30 then low = low + 1 end
    mood = mood + p.mood
  end
  ctx.message(string.format("%d %d", low, mood))
end`

const luaPerCall = `
function on_hour(ctx)
  local low, mood = 0, 0
  for id = 1, ctx.colonists() do
    local p = ctx.pawn(id)
    if p.food < 30 then low = low + 1 end
    mood = mood + p.mood
  end
  ctx.message(string.format("%d %d", low, mood))
end`

const luaNoop = "function on_hour(ctx) end"

// luaRun is one sandboxed state with the script loaded and a ctx over c.
type luaRun struct {
	L    *lua.LState
	fn   lua.LValue
	ctx  *lua.LTable
	msgs []string
}

// newLuaState opens only base, table and string and removes the base
// functions that load code, reach files or collect garbage.
func newLuaState(tb testing.TB) *lua.LState {
	tb.Helper()
	L := lua.NewState(lua.Options{SkipOpenLibs: true})
	for _, lib := range []struct {
		name string
		open lua.LGFunction
	}{{lua.BaseLibName, lua.OpenBase}, {lua.TabLibName, lua.OpenTable}, {lua.StringLibName, lua.OpenString}} {
		L.Push(L.NewFunction(lib.open))
		L.Push(lua.LString(lib.name))
		L.Call(1, 0)
	}
	for _, name := range []string{"dofile", "loadfile", "load", "loadstring", "require", "collectgarbage", "print"} {
		L.SetGlobal(name, lua.LNil)
	}
	return L
}

// newLuaRun loads src into a fresh state. lazy picks the pawn record: a
// userdata over the Go pawn with one shared metatable whose __index resolves
// a field by name (like Starlark's lazy pawnValue); otherwise an eager
// 9-field table.
func newLuaRun(tb testing.TB, src string, c *colony, lazy bool) *luaRun {
	tb.Helper()
	r := &luaRun{L: newLuaState(tb)}
	var meta *lua.LTable
	if lazy {
		meta = r.L.NewTable()
		r.L.SetField(meta, "__index", r.L.NewFunction(luaPawnIndex))
	}
	record := func(L *lua.LState, p scripting.Pawn) lua.LValue {
		if lazy {
			ud := L.NewUserData()
			pp := p
			ud.Value = &pp
			ud.Metatable = meta
			return ud
		}
		rec := L.CreateTable(0, 9)
		fillLuaPawn(rec, p.ID, p.Name, p.X, p.Y, p.Food, p.Rest, p.Mood, p.Downed, p.Drafted)
		return rec
	}
	tb.Cleanup(r.L.Close)
	if err := r.L.DoString(src); err != nil {
		tb.Fatalf("lua: %v", err)
	}
	r.fn = r.L.GetGlobal("on_hour")
	ctx := r.L.NewTable()
	r.L.SetField(ctx, "colonists", r.L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(c.Colonists()))
		return 1
	}))
	r.L.SetField(ctx, "message", r.L.NewFunction(func(L *lua.LState) int {
		r.msgs = append(r.msgs, L.CheckString(1))
		return 0
	}))
	r.L.SetField(ctx, "pawns", r.L.NewFunction(func(L *lua.LState) int {
		ps := c.Pawns()
		arr := L.CreateTable(len(ps), 0)
		for _, p := range ps {
			arr.Append(record(L, p))
		}
		L.Push(arr)
		return 1
	}))
	r.L.SetField(ctx, "pawn", r.L.NewFunction(func(L *lua.LState) int {
		p, ok := c.Pawn(uint64(L.CheckInt(1)))
		if !ok {
			L.Push(lua.LNil)
			return 1
		}
		L.Push(record(L, p))
		return 1
	}))
	r.ctx = ctx
	return r
}

// luaPawnIndex is the shared __index: it resolves a field of the Go pawn
// behind the userdata by name, so only the fields a script reads are built.
func luaPawnIndex(L *lua.LState) int {
	p := L.CheckUserData(1).Value.(*scripting.Pawn)
	switch L.CheckString(2) {
	case "id":
		L.Push(lua.LNumber(p.ID))
	case "name":
		L.Push(lua.LString(p.Name))
	case "x":
		L.Push(lua.LNumber(p.X))
	case "y":
		L.Push(lua.LNumber(p.Y))
	case "food":
		L.Push(lua.LNumber(p.Food))
	case "rest":
		L.Push(lua.LNumber(p.Rest))
	case "mood":
		L.Push(lua.LNumber(p.Mood))
	case "downed":
		L.Push(lua.LBool(p.Downed))
	case "drafted":
		L.Push(lua.LBool(p.Drafted))
	default:
		L.Push(lua.LNil)
	}
	return 1
}

func fillLuaPawn(t *lua.LTable, id uint64, name string, x, y, food, rest, mood int, downed, drafted bool) {
	t.RawSetString("id", lua.LNumber(id))
	t.RawSetString("name", lua.LString(name))
	t.RawSetString("x", lua.LNumber(x))
	t.RawSetString("y", lua.LNumber(y))
	t.RawSetString("food", lua.LNumber(food))
	t.RawSetString("rest", lua.LNumber(rest))
	t.RawSetString("mood", lua.LNumber(mood))
	t.RawSetString("downed", lua.LBool(downed))
	t.RawSetString("drafted", lua.LBool(drafted))
}

// fire calls on_hour once and returns the messages it queued.
func (r *luaRun) fire(tb testing.TB) []string {
	tb.Helper()
	r.msgs = r.msgs[:0]
	if err := r.L.CallByParam(lua.P{Fn: r.fn, NRet: 0, Protect: true}, r.ctx); err != nil {
		tb.Fatalf("lua call: %v", err)
	}
	return r.msgs
}

// TestLuaSandbox: the state loads no code from outside, reaches no files
// and has no math or os (PLG-060 research: gopher-lua has no step budget,
// so this runtime is benchmark only; the sandbox keeps the comparison fair).
func TestLuaSandbox(t *testing.T) {
	t.Parallel()
	L := newLuaState(t)
	defer L.Close()
	for _, g := range []string{"dofile", "loadfile", "load", "loadstring", "require", "collectgarbage", "print", "os", "io", "math", "package", "debug"} {
		if v := L.GetGlobal(g); v != lua.LNil {
			t.Errorf("global %s = %v, want nil", g, v)
		}
	}
	for _, g := range []string{"table", "string", "pairs", "type"} {
		if L.GetGlobal(g) == lua.LNil {
			t.Errorf("global %s missing", g)
		}
	}
	if err := L.DoString("dofile('x')"); err == nil {
		t.Error("dofile call succeeded")
	}
}
