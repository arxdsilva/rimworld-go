// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package raid

// FactionWarning returns the untranslated title and formatting template
// for an arriving hostile faction, preserving the original message args.
func FactionWarning(plan Plan, n int) (title, format, noun, verb string) {
	noun, verb = "raiders", "are"
	if n == 1 {
		noun, verb = "raider", "is"
	}
	switch plan {
	case Siege:
		title, format = "Siege", "Raid! %d %s of %s %s coming from the %s. They look ready to dig in for a siege."
	case Sappers:
		title, format = "Sappers", "Sappers! %d %s of %s %s tunnelling in through the walls from the %s."
	default:
		title, format = "Raid", "Raid! %d %s of %s %s coming from the %s."
	}
	return
}
