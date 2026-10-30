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
}

// Add appends text at the current tick, dropping the oldest entry past
// LogSize.
func (l *Log) Add(text string) {
	if text == "" {
		return
	}
	if len(l.entries) >= LogSize {
		l.entries = slices.Delete(l.entries, 0, len(l.entries)-LogSize+1)
	}
	l.entries = append(l.entries, LogEntry{Tick: l.h.Now(), Text: text})
	l.viewsOK = false
}

// Down logs colonist target struck down by a blow (up: it stood before).
func (l *Log) Down(target entity.EntityID, up bool) {
	if up && !l.h.StandingPawn(target) {
		l.Add(l.h.Name(target) + " was struck down.")
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
