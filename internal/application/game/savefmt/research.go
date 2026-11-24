// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package savefmt

import (
	"rimworld-go/internal/domain/research"
	"rimworld-go/internal/domain/sim/command"
)

// ResearchIDs maps a saved research project onto the running content
// (PLG-026): the project of the same key now, ok false when the run lacks
// the key (its plugin is no longer loaded). Project 0 is a real project,
// so it is shifted by one through Remap, whose 0 is "gone".
type ResearchIDs func(research.Project) (research.Project, bool)

// Research is the research map onto book. A save without research keys
// (before PLG-026: core content only) keeps its projects.
func (k Keys) Research(book research.Book) ResearchIDs {
	to := k.Remap(research.Class, func(key string) (int, bool) {
		p, ok := book.IDOf(key)
		return int(p) + 1, ok
	})
	return func(p research.Project) (research.Project, bool) {
		j := to(int(p) + 1)
		if j == 0 {
			return 0, false
		}
		return research.Project(j - 1), true // #nosec G115 -- Remap yields 0, the input, or a current book ID (keys.go:23-40)
	}
}

// Progress is saved points (indexed by saved project, research.Progress.
// Slice) moved onto the running projects; a gone project's points are
// dropped. It is never nil, so a load still reads it over the old V1 field.
func (m ResearchIDs) Progress(pts []int) []int {
	out := make([]int, 0, len(pts))
	for i, v := range pts {
		if v == 0 || i >= research.MaxBook {
			continue
		}
		p, ok := m(research.Project(i))
		if !ok {
			continue
		}
		for len(out) <= int(p) {
			out = append(out, 0)
		}
		out[p] = v
	}
	return out
}

// Current is the saved current project on the running content; a gone
// one is let go (set false), never pointed at another project.
func (m ResearchIDs) Current(cur research.Project, set bool) (research.Project, bool) {
	p, ok := m(cur)
	if !ok {
		return 0, false
	}
	return p, set
}

// Commands remaps the project of every SetResearch command in cmds, in
// place, dropping one whose project is gone (cmds itself when none is).
func (m ResearchIDs) Commands(cmds []command.Command) []command.Command {
	var out []command.Command // nil until a command is dropped
	for i, c := range cmds {
		if c.Kind == command.SetResearch {
			p, ok := m(research.Project(c.Build))
			if !ok {
				if out == nil {
					out = append(make([]command.Command, 0, len(cmds)), cmds[:i]...)
				}
				continue
			}
			c.Build = uint8(p)
			cmds[i] = c
		}
		if out != nil {
			out = append(out, c)
		}
	}
	if out == nil {
		return cmds
	}
	return out
}
