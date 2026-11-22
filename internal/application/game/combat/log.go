// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package combat

import (
	"slices"

	"rimworld-go/internal/domain/sim/entity"
)

// Log is the combat log (DEF-4): a bounded record of the colony's fights
// and threats, newest last, for the UI's combat records (a raid arriving,
// a colonist downed, a raider killed, a retreat). Data only: the raid
// package and the game's adapters write it, the read model copies it. It
// keeps the last LogSize entries.
type Log struct {
	h       Host
	entries []LogEntry // saved, oldest first
	views   []LogEntry // the read model's copy, rebuilt after a change
	viewsOK bool
}

// LogSize bounds the log.
const LogSize = 64

// LogEntry is one line of the log. Fields are exported for the save.
type LogEntry struct {
	Tick uint64
	Text string
	Kind LogKind // DEF 3.1 (save v19)
}

// LogKind sorts a log entry for the History tabs (DEF 3.1).
type LogKind uint8

// The log kinds: LogUnsorted is every entry of a v18 save (the Combat
// tab only); LogFight a blow's outcome; LogThreat a threat to the colony.
const (
	LogUnsorted LogKind = iota
	LogFight
	LogThreat
)

// Add appends text of kind k at the current tick, dropping the oldest
// entry past LogSize.
func (l *Log) Add(k LogKind, text string) {
	if text == "" {
		return
	}
	if len(l.entries) >= LogSize {
		l.entries = slices.Delete(l.entries, 0, len(l.entries)-LogSize+1)
	}
	l.entries = append(l.entries, LogEntry{Tick: l.h.Now(), Text: text, Kind: k})
	l.viewsOK = false
}

// Fight logs a blow's outcome (raid.Chronicle).
func (l *Log) Fight(text string) { l.Add(LogFight, text) }

// Alarm logs a threat to the colony (raid.Chronicle).
func (l *Log) Alarm(text string) { l.Add(LogThreat, text) }

// Down logs colonist target struck down by a blow (up: it stood before).
func (l *Log) Down(target entity.EntityID, up bool) {
	if up && !l.h.StandingPawn(target) {
		l.Fight(l.h.Name(target) + " was struck down.")
	}
}

// Entries is the log, oldest first: a shared slice replaced (never
// rewritten) when the log changes, so treat it as read only.
func (l *Log) Entries() []LogEntry {
	if !l.viewsOK {
		l.views, l.viewsOK = slices.Clone(l.entries), true
	}
	return l.views
}

// export is the log for the save (nil when empty).
func (l *Log) export() []LogEntry {
	if len(l.entries) == 0 {
		return nil
	}
	return slices.Clone(l.entries)
}

// importLog restores a saved log (the newest LogSize entries).
func (l *Log) importLog(s []LogEntry) {
	if len(s) > LogSize {
		s = s[len(s)-LogSize:]
	}
	l.entries, l.viewsOK = append(l.entries[:0], s...), false
}
