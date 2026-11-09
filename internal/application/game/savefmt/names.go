package savefmt

import (
	"strings"
	"unicode/utf8"
)

// NameMax is the longest save name, in runes.
const NameMax = 60

// The reasons CheckName gives, in English. The client shows each through
// its catalog with a literal of its own (options/saves), so a new reason
// needs a line there too.
const (
	NameEmpty    = "Name the save"
	NameTooLong  = "Use at most 60 characters"
	NameChars    = "Use only letters, digits, spaces, - and _"
	NameAutosave = "Names ending in -autosave are kept for autosaves"
	NameDevice   = "Windows keeps this name for a device"
)

// CheckName is why name cannot name a save ("" when it can; SAV-004). The
// name is trimmed first, as the store trims it. The rules:
//   - not empty and at most NameMax runes;
//   - ASCII letters, digits, space, '-' and '_' only (macOS stores file
//     names decomposed, so anything else may not round-trip);
//   - not <x>-autosave or <x>-autosave-N, the names the game's autosaves use;
//   - not a Windows device name (CON, PRN, AUX, NUL, COM1-9, LPT1-9).
func CheckName(name string) (reason string) {
	name = strings.TrimSpace(name)
	switch {
	case name == "":
		return NameEmpty
	case utf8.RuneCountInString(name) > NameMax:
		return NameTooLong
	case !plainName(name):
		return NameChars
	case autosaveName(name):
		return NameAutosave
	case deviceName(name):
		return NameDevice
	}
	return ""
}

// plainName reports s holds only the characters a save name may use.
func plainName(s string) bool {
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == ' ', r == '-', r == '_':
		default:
			return false
		}
	}
	return true
}

// autosaveName reports s's file name is one an autosave uses (BaseSlot
// strips a suffix from it), in any case: the common file systems ignore it.
func autosaveName(s string) bool {
	k := strings.ToLower(SlotKey(s))
	return BaseSlot(k) != k
}

// deviceName reports s's file name is a Windows device's, which no file
// can take there.
func deviceName(s string) bool {
	u := strings.ToUpper(SlotKey(s))
	switch u {
	case "CON", "PRN", "AUX", "NUL":
		return true
	}
	return len(u) == 4 && (strings.HasPrefix(u, "COM") || strings.HasPrefix(u, "LPT")) && u[3] >= '1' && u[3] <= '9'
}
