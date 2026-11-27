// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"testing"

	"rimworld-go/internal/application/scripting"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/testkit/allocs"
)

// TestExtSeamsFreeWithoutPlugins: with no extensions (and with ones that
// take nothing) every seam on the tick path returns before any work: the
// command veto, the incident replacement, the events, the panels, the
// craft and research words and the order gate allocate nothing.
func TestExtSeamsFreeWithoutPlugins(t *testing.T) { // not parallel: AllocsPerRun
	book := incidentBookWith(false, "frontier.dark_noon")
	g := New(Config{Seed: 3, Width: 16, Height: 16, Content: Content{Incidents: book}})
	ada := spawnColonists(t, g, 1, 2)[0]
	plugin, _ := book.IDOf("frontier.dark_noon")
	draft := command.Command{Kind: command.DraftPawn, Pawn: uint64(ada), Enabled: true}
	order := command.Command{Kind: command.ScriptOrder, Pawn: uint64(ada), Order: command.OrderMine, Target: 1}
	club := recipe.Core().DefOf(recipe.Club)
	seams := func() {
		scriptVetoed(g, draft)
		scriptReplaced(g, plugin, 1)
		emitEvent(g, scripting.Event{Kind: scripting.Quadrum})
		scriptPanels(g, 1)
		newCraftHost(g).Crafted(ada, club, 1, 2, 2)
		researchHost{g}.Finished("core.traps")
		scriptOrderDue(g, order)
	}
	if n := allocs.Fewest(5, seams); n != 0 {
		t.Fatalf("no extensions: %v allocations", n)
	}
	UseExtensions(g, &fakeExt{})
	if n := allocs.Fewest(5, seams); n != 0 {
		t.Fatalf("extensions that take nothing: %v allocations", n)
	}
	if g.scripts.panels != nil || g.cmds.Len() != 0 || len(g.messageViews()) != 0 {
		t.Fatal("an idle seam changed the game")
	}
}
