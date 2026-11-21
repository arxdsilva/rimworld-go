// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package modstate

import "unicode/utf8"

// Limits on the mod state a save holds.
const (
	MaxData  = 64<<10 + 16 // bytes of one blob's data: a module's 64 KiB state plus the host's header
	MaxBlobs = 256         // blobs in one save
	MaxName  = 256         // bytes of a blob's plugin id or file path
	MaxNotes = 16          // notes kept until taken, the last one saying more were dropped
)

// Blob is one Wasm file's versioned save data, opaque outside wasmhost.
type Blob struct {
	Plugin, File string
	Data         []byte
}

// Note is one thing loading the mod state reported about a file (a blob
// dropped, a file with no blob), shown to the player as a script message.
type Note struct {
	Plugin, File, Msg string
}

// Source is the run's Wasm mod state for a real save. Only the save path
// calls it; StateHash never does. The composition root's mux implements
// it over wasmhost (W2-compose).
type Source interface {
	SaveBlobs() []Blob
}

// Check keeps the blobs a save may hold, in order, and notes each one it
// drops: an empty or over-MaxName plugin or file, data over MaxData, a
// second blob for the same (plugin, file) (the first is kept), and every
// blob once MaxBlobs are kept. A dropped blob claims no key. Notes stay
// within MaxNotes (AppendNotes). It reads bs only; kept shares bs's data.
func Check(bs []Blob) (kept []Blob, notes []Note) {
	seen := make(map[[2]string]bool, min(len(bs), MaxBlobs))
	for _, b := range bs {
		why := dropReason(b, seen, len(kept))
		if why != "" {
			notes = AppendNotes(notes, []Note{{Plugin: b.Plugin, File: b.File, Msg: why}})
			continue
		}
		seen[[2]string{b.Plugin, b.File}] = true
		kept = append(kept, b)
	}
	return kept, notes
}

// dropReason says why Check drops b ("" keeps it), given the keys kept
// so far and how many blobs are kept.
func dropReason(b Blob, seen map[[2]string]bool, n int) string {
	switch {
	case b.Plugin == "" || b.File == "":
		return "saved state dropped: no plugin or file name"
	case len(b.Plugin) > MaxName || len(b.File) > MaxName:
		return "saved state dropped: plugin or file name too long"
	case len(b.Data) > MaxData:
		return "saved state dropped: over the size limit"
	case seen[[2]string{b.Plugin, b.File}]:
		return "saved state dropped: a second copy for the same file"
	case n >= MaxBlobs:
		return "saved state dropped: too many files with state"
	}
	return ""
}

// AppendNotes adds ns to dst within MaxNotes, names clipped to MaxName
// bytes: past MaxNotes-1 notes, one unnamed note says more were dropped,
// and later ones are left out. Check and the save path both use it.
func AppendNotes(dst, ns []Note) []Note {
	for _, n := range ns {
		switch {
		case len(dst) < MaxNotes-1:
			dst = append(dst, Note{Plugin: clip(n.Plugin), File: clip(n.File), Msg: n.Msg})
		case len(dst) == MaxNotes-1:
			dst = append(dst, Note{Msg: "saved state dropped for more files (too many to list)"})
		}
	}
	return dst
}

// clip cuts s to at most MaxName bytes, on a rune boundary.
func clip(s string) string {
	if len(s) <= MaxName {
		return s
	}
	n := MaxName
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}
