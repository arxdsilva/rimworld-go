package game

import (
	"rimworld-go/internal/application/game/evlog"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// Per-creature event logs (LOG-001): what happened to each colonist,
// raider and animal (attacked, bitten, downed, ate, slept, tamed...),
// bounded, saved and shown in the inspectors. Entries are written only
// from inside Step phases; the text is built when the view is read.

// maxLogEntries bounds one creature's log; the oldest entry is dropped.
const maxLogEntries = 30

// LogLineView is one line of a creature's log (game/evlog).
type LogLineView = evlog.Line

// The event kinds the care hosts name (game/evlog holds them all: saved,
// append only); the rest of the game names evlog's directly.
const (
	logTendedBy  = evlog.TendedBy
	logTended    = evlog.Tended
	logAte       = evlog.Ate
	logRescuedBy = evlog.RescuedBy
	logRescued   = evlog.Rescued
	logFedBy     = evlog.FedBy
	logFed       = evlog.Fed
)

// logEntry is one stored event (plain data; text is built for the view).
// Name is the other party (or food / break kind) as it was when it
// happened, so the line survives that creature leaving the map.
type logEntry struct {
	Tick   uint64
	Kind   uint8
	Other  entity.EntityID
	Amount int
	Weapon uint8
	Name   string
}

// creatureName is how a log refers to id: animals by their pet name or
// species, pawns by name.
func (g *Game) creatureName(id entity.EntityID) string {
	if a, ok := g.animals.Get(id); ok {
		return animalLabel(a)
	}
	return g.name(id)
}

func animalLabel(a animal.State) string {
	if a.Tame && a.Name != "" {
		return a.Name
	}
	return a.Species.Label()
}

// logEvent appends e (stamped now) to subject's log, dropping the oldest
// entry past the cap.
func (g *Game) logEvent(subject entity.EntityID, e logEntry) {
	if subject == 0 {
		return
	}
	if g.logs == nil {
		g.logs = map[entity.EntityID][]logEntry{}
	}
	e.Tick = uint64(g.clock.Now())
	l := append(g.logs[subject], e)
	if len(l) > maxLogEntries {
		l = append(l[:0:0], l[len(l)-maxLogEntries:]...)
	}
	g.logs[subject] = l
}

// logPair records an event on both creatures: subject gets kindSelf,
// other gets kindOther (0 skips it).
func (g *Game) logPair(subject, other entity.EntityID, selfKind, otherKind uint8, amount int, wk weapon.Kind) {
	g.logEvent(subject, logEntry{Kind: selfKind, Other: other, Amount: amount, Weapon: uint8(wk), Name: g.creatureName(other)})
	if otherKind != 0 && other != 0 {
		g.logEvent(other, logEntry{Kind: otherKind, Other: subject, Amount: amount, Weapon: uint8(wk), Name: g.creatureName(subject)})
	}
}

// logStrike records one resolved attack on attacker and target.
func (g *Game) logStrike(attacker, target entity.EntityID, wk weapon.Kind, hit bool, dmg int) {
	_, bites := g.animals.Get(attacker)
	switch {
	case hit && bites:
		g.logPair(attacker, target, evlog.Bit, 0, dmg, wk)
		g.logEvent(target, logEntry{Kind: evlog.Bitten, Other: attacker, Amount: dmg, Weapon: uint8(wk), Name: g.creatureName(attacker)})
	case hit:
		g.logPair(attacker, target, evlog.Hit, evlog.Attacked, dmg, wk)
	default:
		g.logPair(attacker, target, evlog.Missed, evlog.WasMissed, 0, wk)
	}
}

// logLines is id's log for the read model, newest first.
func (g *Game) logLines(id entity.EntityID) []LogLineView {
	l := g.logs[id]
	if len(l) == 0 {
		return nil
	}
	// With the view cache on, an unchanged log keeps its lines: entries are
	// only ever appended past the end or copied out to a new array, so the
	// same first element and length mean the same entries (the cache holds
	// the slice, so its array cannot be reused meanwhile).
	vc := &g.views
	if vc.on {
		if c, ok := vc.logs[id]; ok && len(c.src) == len(l) && &c.src[0] == &l[0] {
			return c.out
		}
	}
	out := make([]LogLineView, 0, len(l))
	for i := len(l) - 1; i >= 0; i-- {
		d := clock.DateAt(clock.Tick(l[i].Tick))
		out = append(out, LogLineView{Tick: l[i].Tick, Day: d.Day + 1, Hour: d.Hour, Text: evlog.Text(l[i].Kind, l[i].Weapon, l[i].Name, l[i].Amount)})
	}
	if vc.on {
		if vc.logs == nil {
			vc.logs = map[entity.EntityID]logLinesMemo{}
		}
		if len(vc.logs) > 2*len(g.logs)+64 { // drop the dead creatures' lines
			for k := range vc.logs {
				if _, ok := g.logs[k]; !ok {
					delete(vc.logs, k)
				}
			}
		}
		vc.logs[id] = logLinesMemo{src: l, out: out}
	}
	return out
}

// logLinesMemo is one creature's cached logLines and the log it was built
// from (viewCache).
type logLinesMemo struct {
	src []logEntry
	out []LogLineView
}

// exportLogs returns the logs for saving (copied; empty logs omitted).
func (g *Game) exportLogs() map[entity.EntityID][]logEntry {
	if len(g.logs) == 0 {
		return nil
	}
	out := make(map[entity.EntityID][]logEntry, len(g.logs))
	for id, l := range g.logs {
		out[id] = append([]logEntry(nil), l...)
	}
	return out
}

func (g *Game) importLogs(m map[entity.EntityID][]logEntry) {
	g.logs = map[entity.EntityID][]logEntry{}
	for id, l := range m {
		g.logs[id] = append([]logEntry(nil), l...)
	}
}
