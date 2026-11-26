// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package dossierview

import (
	"rimworld-go/internal/domain/pawn/pawngen"
	"rimworld-go/internal/domain/sim/command"
)

// RenamePerson validates the three fields of a native pawn identity.
func RenamePerson(p pawngen.Pawn, raw string) (pawngen.Pawn, bool) {
	first, nick, last, ok := command.UnpackRename(raw)
	if !ok {
		return p, false
	}
	var names [3]string
	for i, s := range [3]string{first, nick, last} {
		names[i], ok = CleanName(s)
		if !ok {
			return p, false
		}
	}
	if names[0] == "" {
		return p, false
	}
	p.First, p.Nick, p.Last = names[0], names[1], names[2]
	return p, true
}
