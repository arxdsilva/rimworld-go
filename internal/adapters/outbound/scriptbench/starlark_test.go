// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package scriptbench

import (
	"testing"

	"rimworld-go/internal/adapters/outbound/starlarkhost"
	"rimworld-go/internal/application/scripting"
	"rimworld-go/internal/domain/sim/command"
)

const starBatched = `
def on_hour(ctx):
    low = 0
    mood = 0
    for p in ctx.pawns():
        if p.food < 30:
            low += 1
        mood += p.mood
    ctx.message("%d %d" % (low, mood))
`

const starPerCall = `
def on_hour(ctx):
    low = 0
    mood = 0
    for id in range(1, ctx.colonists + 1):
        p = ctx.pawn(id)
        if p.food < 30:
            low += 1
        mood += p.mood
    ctx.message("%d %d" % (low, mood))
`

const starNoop = "def on_hour(ctx):\n    pass\n"

// starHost builds the real adapter around one script.
func starHost(tb testing.TB, src string) *starlarkhost.Host {
	tb.Helper()
	h := starlarkhost.New([]starlarkhost.Plugin{{ID: "bench", Files: []starlarkhost.File{{Path: "scripts/bench.star", Src: []byte(src)}}}})
	if p := h.Problems(); len(p) != 0 {
		tb.Fatalf("problems: %v", p)
	}
	return h
}

// starFire fires on_hour once and returns the queued message texts.
func starFire(tb testing.TB, h *starlarkhost.Host, c *colony) []string {
	tb.Helper()
	r := h.Fire(scripting.Event{Hook: scripting.OnHour, Tick: 2500, Seed: 7}, c, c)
	if len(r.Disabled) != 0 {
		tb.Fatalf("disabled: %v", r.Disabled)
	}
	var out []string
	for _, cmd := range r.Commands {
		if cmd.Kind == command.ScriptMessage {
			out = append(out, cmd.Name)
		}
	}
	return out
}
