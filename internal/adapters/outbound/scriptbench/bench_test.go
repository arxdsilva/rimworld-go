// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package scriptbench

import (
	"fmt"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

// TestLuaLinks opens and closes a bare Lua state: gopher-lua links into
// this test binary (and go mod tidy keeps its require).
func TestLuaLinks(t *testing.T) {
	L := lua.NewState(lua.Options{SkipOpenLibs: true})
	defer L.Close()
	if err := L.DoString("x = 1 + 1"); err != nil {
		t.Fatalf("lua: %v", err)
	}
	if got := L.GetGlobal("x"); got != lua.LNumber(2) {
		t.Fatalf("x = %v, want 2", got)
	}
}

// BenchmarkPawnReads measures one on_hour call that reads every colonist.
// Batched is one ctx.pawns(); percall is ctx.pawn(id) for id in 1..N.
// Records: starlark's pawnValue and lua-lazy (userdata, shared metatable)
// resolve fields on access; lua-eager builds a 9-field table per pawn. The
// noop cases are NOT symmetric: starlark noop is a full host Fire (fresh
// thread with a step limit, ctx, result and command slice); lua noop is a
// bare protected call of an empty function, with no step budget at all.
func BenchmarkPawnReads(b *testing.B) {
	type variant struct{ name, star, lua string }
	vs := []variant{{"batched", starBatched, luaBatched}, {"percall", starPerCall, luaPerCall}}
	for _, v := range vs {
		for _, n := range []int{50, 500} {
			c := newColony(n)
			b.Run(fmt.Sprintf("starlark/%s/n=%d", v.name, n), func(b *testing.B) {
				h := starHost(b, v.star)
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					starFire(b, h, c)
				}
			})
			for _, rt := range []struct {
				name string
				lazy bool
			}{{"lua-lazy", true}, {"lua-eager", false}} {
				b.Run(fmt.Sprintf("%s/%s/n=%d", rt.name, v.name, n), func(b *testing.B) {
					r := newLuaRun(b, v.lua, c, rt.lazy)
					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						r.fire(b)
					}
				})
			}
		}
	}
	c := newColony(50)
	b.Run("starlark/noop/n=50", func(b *testing.B) {
		h := starHost(b, starNoop)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			starFire(b, h, c)
		}
	})
	b.Run("lua/noop/n=50", func(b *testing.B) {
		r := newLuaRun(b, luaNoop, c, true)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			r.fire(b)
		}
	})
}
