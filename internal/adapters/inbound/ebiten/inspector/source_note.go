// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package inspector

import (
	"rimworld-go/internal/adapters/inbound/ebiten/lang"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
)

// sources reads definitions' provenance from the run's content (PLG-072).
type sources struct {
	c  game.Content
	tr lang.Tr
}

// sources is the pane's provenance reader.
func (a *Pane) sources() sources { return sources{a.env().Content, a.shell} }

// sourceNote is the "Added by"/"Changed by" text of the definition keyed
// key in class, read from the run's content only; "" for the game's own,
// untouched ones.
func (s sources) sourceNote(class, key string) string {
	if key == "" {
		return ""
	}
	src := s.c.SourceOf(class, key)
	return uikit.SourceNote(s.tr, src.AddedBy, src.PatchedBy, nil)
}

// withSource is tip with the definition's provenance under it.
func (s sources) withSource(tip, class, key string) string {
	return uikit.WithSourceNote(tip, s.sourceNote(class, key))
}

// traitsWithSource is b with each trait's hover text carrying its
// provenance (b's own slice is left alone).
func (s sources) traitsWithSource(b game.BioView) game.BioView {
	var out []game.TraitView
	for i, t := range b.Traits {
		n := s.sourceNote("trait", t.Key)
		if n == "" {
			continue
		}
		if out == nil {
			out = append([]game.TraitView(nil), b.Traits...)
		}
		out[i].Desc = uikit.WithSourceNote(t.Desc, n)
	}
	if out != nil {
		b.Traits = out
	}
	return b
}

// recipeSource is the note of the recipe named name: a core recipe's key
// is "core."+name, a plugin's is its name.
func (s sources) recipeSource(name string) string {
	if n := s.sourceNote("recipe", name); n != "" {
		return n
	}
	return s.sourceNote("recipe", "core."+name)
}
