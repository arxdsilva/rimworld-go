// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package uikit

import (
	"strings"

	"rimworld-go/internal/adapters/inbound/ebiten/lang"
)

// SourceNote is the provenance text of one definition (PLG-072): "Added by
// <plugin>" when addedBy names a plugin and "Changed by <plugins>" when
// patchedBy does, one per line, in the player's language. names maps a
// plugin id to its display name (nil, or "" for an id: the id is shown).
// A core definition no plugin touched gives "", so a caller appends it
// only when set and nothing shifts for the game's own.
func SourceNote(tr lang.Tr, addedBy string, patchedBy []string, names func(id string) string) string {
	name := func(id string) string {
		if names != nil {
			if n := names(id); n != "" {
				return n
			}
		}
		return id
	}
	var lines []string
	if addedBy != "" {
		lines = append(lines, lang.Tf(tr, "Added by %s", name(addedBy)))
	}
	if len(patchedBy) > 0 {
		ns := make([]string, len(patchedBy))
		for i, id := range patchedBy {
			ns[i] = name(id)
		}
		lines = append(lines, lang.Tf(tr, "Changed by %s", strings.Join(ns, ", ")))
	}
	return strings.Join(lines, "\n")
}

// WithSourceNote is text with the note on a new line under it ("" note:
// text unchanged; "" text: the note alone).
func WithSourceNote(text, note string) string {
	switch {
	case note == "":
		return text
	case text == "":
		return note
	}
	return text + "\n" + note
}
