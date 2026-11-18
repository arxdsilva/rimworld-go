// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package windows

import (
	"slices"
	"strconv"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/research"
	"rimworld-go/internal/domain/sim/command"
)

// TestResearchTabShowsPluginProject (PLG-026): the tab draws a plugin
// project from the game's own read model in its tech tier with its
// prerequisite's line, and lets the player select, start and finish it.
func TestResearchTabShowsPluginProject(t *testing.T) {
	t.Parallel()
	core := research.Core()
	list := append(slices.Clone(core.All()), research.Def{Name: "frontier.weave", Label: "Slat weaving", Cost: 400,
		TechLevel: research.Spacer, Requires: []research.Project{research.Traps}})
	book := research.NewBook(list, append(core.Keys(), "frontier.weave"))
	weave := research.NumProjects
	g := game.New(game.Config{Width: 8, Height: 8, Content: game.Content{Research: book}})
	g.ResetResearch()
	g.AddResearch("traps", research.DefOf(research.Traps).Cost)

	sh, w := treeWindow(t, g.World().Research)
	i, traps := nodeOf(t, w, weave), nodeOf(t, w, research.Traps)
	st := w.rs()
	if n := st.nodes[i]; n.v.Label != "Slat weaving" || n.level != int(research.Spacer) || !slices.Contains(n.pre, traps) {
		t.Fatalf("plugin node %+v (traps node %d)", n, traps)
	}
	col := st.g.Col[i]
	if !slices.ContainsFunc(st.g.Bands, func(b treeBand) bool {
		return b.Level == int(research.Spacer) && col >= b.Col0 && col < b.Col1
	}) {
		t.Fatalf("column %d is in no Spacer band: %+v", col, st.g.Bands)
	}
	w.Pick("research.pick." + strconv.Itoa(int(weave)))
	st.buildPanel(200)
	if st.sel != i || st.panel.startOff {
		t.Fatalf("selected %d (want %d), start off %v (%q)", st.sel, i, st.panel.startOff, st.panel.startReason)
	}
	w.start()
	if len(sh.cmds) != 1 || sh.cmds[0] != (command.Command{Kind: command.SetResearch, Build: uint8(weave)}) {
		t.Fatalf("start sent %+v", sh.cmds)
	}
	sh.cmds = nil
	w.finish()
	if len(sh.cmds) != 2 || sh.cmds[1].Kind != command.DevFinishResearch || sh.cmds[1].Build != uint8(weave) {
		t.Fatalf("finish sent %+v", sh.cmds)
	}
}
