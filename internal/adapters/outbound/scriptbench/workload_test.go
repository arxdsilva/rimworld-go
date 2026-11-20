// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package scriptbench

import (
	"fmt"
	"reflect"
	"testing"
)

// want computes the workload's message from the colony directly.
func want(c *colony) string {
	low, mood := 0, 0
	for _, p := range c.Pawns() {
		if p.Food < 30 {
			low++
		}
		mood += p.Mood
	}
	return fmt.Sprintf("%d %d", low, mood)
}

// TestWorkloadsAgree: the four variants queue the same one message.
func TestWorkloadsAgree(t *testing.T) {
	t.Parallel()
	for _, n := range []int{50, 500} {
		c := newColony(n)
		exp := []string{want(c)}
		got := map[string][]string{
			"starlark/batched":  starFire(t, starHost(t, starBatched), c),
			"starlark/percall":  starFire(t, starHost(t, starPerCall), c),
			"lua-lazy/batched":  append([]string(nil), newLuaRun(t, luaBatched, c, true).fire(t)...),
			"lua-lazy/percall":  append([]string(nil), newLuaRun(t, luaPerCall, c, true).fire(t)...),
			"lua-eager/batched": append([]string(nil), newLuaRun(t, luaBatched, c, false).fire(t)...),
			"lua-eager/percall": append([]string(nil), newLuaRun(t, luaPerCall, c, false).fire(t)...),
		}
		for name, g := range got {
			if !reflect.DeepEqual(g, exp) {
				t.Errorf("n=%d %s = %v, want %v", n, name, g, exp)
			}
		}
	}
}
