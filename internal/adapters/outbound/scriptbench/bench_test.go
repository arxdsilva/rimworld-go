// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package scriptbench

import (
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
