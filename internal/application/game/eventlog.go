package game

import (
	"fmt"

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

// LogLineView is one line of a creature's log, newest first in the views.
type LogLineView struct {
	Tick uint64
	Day  int    // in-game day (1-based)
	Hour int    // 0..23
	Text string // e.g. "Bitten by Ridgewolf (-6)"
}

// Event kinds. Values are saved; append only.
const (
	logHit uint8 = iota + 1
	logAttacked
	logMissed
	logWasMissed
	logBit
	logBitten
	logTrap
	logDowned
	logDied
	logKilled
	logFled
	logTendedBy
	logTended
	logAte
	logSleptBed
	logSleptGround
	logTamedBy
	logResistedTaming
	logTamed
	logFailedTame
	logManhunter
	logMental
	logRecovered
	logInfected
	logInfectionCleared
	logHypothermia
	logHeatstroke
	logRescuedBy
	logRescued
	logCollapsed
	logStarving  // SIM-024: an animal's food ran out
	logLeaving   // SIM-025: set off to leave the map (over capacity, crossing, done gnawing)
	logJoined    // SIM-025: a stray or farm animal joined the colony
	logLiking    // SIM-025: a wild animal tamed itself
	logMated     // SIM-026: mated with Name
	logGaveBirth // SIM-026: Amount young (Name: "goat kids")
	logBorn      // SIM-026: born to Name
	logOldAge    // SIM-026: died of old age
	logHatched   // SIM-026: hatched from an egg
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
		g.logPair(attacker, target, logBit, 0, dmg, wk)
		g.logEvent(target, logEntry{Kind: logBitten, Other: attacker, Amount: dmg, Weapon: uint8(wk), Name: g.creatureName(attacker)})
	case hit:
		g.logPair(attacker, target, logHit, logAttacked, dmg, wk)
	default:
		g.logPair(attacker, target, logMissed, logWasMissed, 0, wk)
	}
}

// logLines is id's log for the read model, newest first.
func (g *Game) logLines(id entity.EntityID) []LogLineView {
	l := g.logs[id]
	if len(l) == 0 {
		return nil
	}
	out := make([]LogLineView, 0, len(l))
	for i := len(l) - 1; i >= 0; i-- {
		d := clock.DateAt(clock.Tick(l[i].Tick))
		out = append(out, LogLineView{Tick: l[i].Tick, Day: d.Day + 1, Hour: d.Hour, Text: logText(l[i])})
	}
	return out
}

// strikeVerb is an animal blow's verb ("Bit", "Clawed"), passive
// ("Bitten", "Clawed") when suffered.
func strikeVerb(wk uint8, passive bool) string {
	switch weapon.Kind(wk) {
	case weapon.Claw:
		return "Clawed"
	case weapon.Kick:
		return "Kicked"
	case weapon.Gore:
		return "Gored"
	case weapon.Headbutt:
		return "Headbutted"
	case weapon.Peck:
		return "Pecked"
	}
	if passive {
		return "Bitten"
	}
	return "Bit"
}

func logText(e logEntry) string {
	with := ""
	if n := weapon.DefOf(weapon.Kind(e.Weapon)).Name; n != "fists" && n != "" {
		with = " with " + n
	}
	switch e.Kind {
	case logHit:
		return fmt.Sprintf("Hit %s%s (-%d)", e.Name, with, e.Amount)
	case logAttacked:
		return fmt.Sprintf("Attacked by %s%s (-%d)", e.Name, with, e.Amount)
	case logMissed:
		return "Missed " + e.Name
	case logWasMissed:
		return e.Name + " missed"
	case logBit:
		return fmt.Sprintf("%s %s (-%d)", strikeVerb(e.Weapon, false), e.Name, e.Amount)
	case logBitten:
		return fmt.Sprintf("%s by %s (-%d)", strikeVerb(e.Weapon, true), e.Name, e.Amount)
	case logTrap:
		return fmt.Sprintf("Hurt by a trap (-%d)", e.Amount)
	case logDowned:
		return "Downed"
	case logDied:
		return "Died"
	case logKilled:
		return "Killed " + e.Name
	case logFled:
		return "Fled from " + e.Name
	case logTendedBy:
		return "Tended by " + e.Name
	case logTended:
		return "Tended " + e.Name
	case logAte:
		return "Ate " + e.Name
	case logSleptBed:
		return "Slept in a bed"
	case logSleptGround:
		return "Slept on the ground"
	case logTamedBy:
		return "Tamed by " + e.Name
	case logResistedTaming:
		return "Resisted taming by " + e.Name
	case logTamed:
		return "Tamed " + e.Name
	case logFailedTame:
		return "Failed to tame " + e.Name
	case logManhunter:
		return "Went manhunter"
	case logMental:
		return "Mental break: " + e.Name
	case logRecovered:
		return "Recovered from a mental break"
	case logInfected:
		return "Wound infected"
	case logInfectionCleared:
		return "Infection cleared"
	case logHypothermia:
		return "Suffering hypothermia"
	case logHeatstroke:
		return "Suffering heatstroke"
	case logRescuedBy:
		return "Rescued by " + e.Name
	case logRescued:
		return "Rescued " + e.Name
	case logCollapsed:
		return "Collapsed from exhaustion"
	case logStarving:
		return "Starving"
	case logLeaving:
		return "Left for the edge of the map"
	case logJoined:
		return "Joined the colony"
	case logLiking:
		return "Took a liking to the colony"
	case logMated:
		return "Mated with " + e.Name
	case logGaveBirth:
		return fmt.Sprintf("Gave birth to %d %s", e.Amount, e.Name)
	case logBorn:
		return "Born to " + e.Name
	case logOldAge:
		return "Died of old age"
	case logHatched:
		return "Hatched"
	}
	return ""
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
