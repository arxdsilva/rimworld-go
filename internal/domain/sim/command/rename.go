// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package command

import "strings"

// RenameName packs RenamePawn's first, nick and last names into Command.Name.
// Names never hold a line break (a name is letters, spaces, apostrophes and
// hyphens), so the line break is the separator.
func RenameName(first, nick, last string) string { return first + "\n" + nick + "\n" + last }

// UnpackRename splits RenamePawn's Name; ok is false when it is not three
// parts.
func UnpackRename(name string) (first, nick, last string, ok bool) {
	parts := strings.Split(name, "\n")
	if len(parts) != 3 {
		return "", "", "", false
	}
	return parts[0], parts[1], parts[2], true
}
