// Package evlog is the per-creature event log's kinds and line text
// (LOG-001), moved out of the game package (AX-2 budget): the game keeps
// the entries, stamps and views (eventlog.go) and reads the words here.
// It is pure: no state, no host.
package evlog

import (
	"fmt"

	"rimworld-go/internal/domain/combat/weapon"
)

// Line is one line of a creature's log, newest first in the views
// (game.LogLineView).
type Line struct {
	Tick uint64
	Day  int    // in-game day (1-based)
	Hour int    // 0..23
	Text string // e.g. "Bitten by Ridgewolf (-6)"
}

// Event kinds (the game's logEntry.Kind). Values are saved; append only.
const (
	Hit uint8 = iota + 1
	Attacked
	Missed
	WasMissed
	Bit
	Bitten
	Trap
	Downed
	Died
	Killed
	Fled
	TendedBy
	Tended
	Ate
	SleptBed
	SleptGround
	TamedBy
	ResistedTaming
	Tamed
	FailedTame
	Manhunter
	Mental
	Recovered
	Infected
	InfectionCleared
	Hypothermia
	Heatstroke
	RescuedBy
	Rescued
	Collapsed
	Starving  // SIM-024: an animal's food ran out
	Leaving   // SIM-025: set off to leave the map (over capacity, crossing, done gnawing)
	Joined    // SIM-025: a stray or farm animal joined the colony
	Liking    // SIM-025: a wild animal tamed itself
	Mated     // SIM-026: mated with Name
	GaveBirth // SIM-026: Amount young (Name: "goat kids")
	Born      // SIM-026: born to Name
	OldAge    // SIM-026: died of old age
	Hatched   // SIM-026: hatched from an egg
	Charged   // PLAY-010: a predator charged Name
	ChargedBy // PLAY-010: charged by Name ("a cougar")
	Ingested  // ENG-003 M2: took a drug (Name: "beer")
	Surgery   // ENG-003 M1: Name is the operation's outcome line
	Scarred   // ENG-003 M1: a wound left a scar (Name: the part)
	PutOn     // PLAY-017: put on a garment (Name: "parka")
	TookOff   // PLAY-017: took a garment off (Name: "duster")
	Visited   // SIM-065: a friend visited the bed (Name: the visitor)
	FedBy     // SIM-064: fed by Name (a doctor or warden)
	Fed       // SIM-064: fed Name
)

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

// Text is the log line of an event of kind with weapon w (a
// weapon.Kind), the other party or thing name and amount.
func Text(kind, w uint8, name string, amount int) string {
	with := ""
	if n := weapon.DefOf(weapon.Kind(w)).Name; n != "fists" && n != "" {
		with = " with " + n
	}
	switch kind {
	case Ingested:
		return "Took " + name
	case PutOn:
		return "Put on a " + name
	case TookOff:
		return "Took off the " + name
	case Visited:
		return "Visited by " + name
	case FedBy:
		return "Fed by " + name
	case Fed:
		return "Fed " + name
	case Surgery, Scarred:
		return name
	case Hit:
		return fmt.Sprintf("Hit %s%s (-%d)", name, with, amount)
	case Attacked:
		return fmt.Sprintf("Attacked by %s%s (-%d)", name, with, amount)
	case Missed:
		return "Missed " + name
	case WasMissed:
		return name + " missed"
	case Bit:
		return fmt.Sprintf("%s %s (-%d)", strikeVerb(w, false), name, amount)
	case Bitten:
		return fmt.Sprintf("%s by %s (-%d)", strikeVerb(w, true), name, amount)
	case Trap:
		return fmt.Sprintf("Hurt by a trap (-%d)", amount)
	case Downed:
		return "Downed"
	case Died:
		return "Died"
	case Killed:
		return "Killed " + name
	case Fled:
		return "Fled from " + name
	case TendedBy:
		return "Tended by " + name
	case Tended:
		return "Tended " + name
	case Ate:
		return "Ate " + name
	case SleptBed:
		return "Slept in a bed"
	case SleptGround:
		return "Slept on the ground"
	case TamedBy:
		return "Tamed by " + name
	case ResistedTaming:
		return "Resisted taming by " + name
	case Tamed:
		return "Tamed " + name
	case FailedTame:
		return "Failed to tame " + name
	case Manhunter:
		return "Went manhunter"
	case Mental:
		return "Mental break: " + name
	case Recovered:
		return "Recovered from a mental break"
	case Infected:
		return "Wound infected"
	case InfectionCleared:
		return "Infection cleared"
	case Hypothermia:
		return "Suffering hypothermia"
	case Heatstroke:
		return "Suffering heatstroke"
	case RescuedBy:
		return "Rescued by " + name
	case Rescued:
		return "Rescued " + name
	case Collapsed:
		return "Collapsed from exhaustion"
	case Starving:
		return "Starving"
	case Leaving:
		return "Left for the edge of the map"
	case Joined:
		return "Joined the colony"
	case Liking:
		return "Took a liking to the colony"
	case Mated:
		return "Mated with " + name
	case GaveBirth:
		return fmt.Sprintf("Gave birth to %d %s", amount, name)
	case Born:
		return "Born to " + name
	case OldAge:
		return "Died of old age"
	case Hatched:
		return "Hatched"
	case Charged:
		return "Charged " + name
	case ChargedBy:
		return "Charged by " + name
	}
	return ""
}
