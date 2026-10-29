package rest

import (
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/world/room"
)

// Room impressiveness thoughts (BED-2). Each ladder is the mood of one
// impressiveness stage, awful (0) to wondrously impressive (9), read once
// when the occasion happens (waking, a meal, the end of a pastime) and
// remembered for a day: a room hovering on a stage boundary changes the
// thought only at the next occasion, never while it is held (no flap).
// wiki: Thoughts (bedroom, barracks, hospital, dining room, recreation
// room). The Thoughts page names the bedroom's sixth rung "impressive"
// where the Room page says "somewhat impressive"; ladders follow stage
// order, so that rung is the somewhat-impressive stage.

// Occasion is when a room's impressiveness is felt.
type Occasion uint8

const (
	// Slept: waking in a bedroom, barracks (prison cell and prison
	// barracks alike) or hospital.
	Slept Occasion = iota
	// Ate: a meal at a table in a dining room.
	Ate
	// Enjoyed: a pastime finished in a recreation room.
	Enjoyed
)

// none marks a stage with no thought.
const none = -100

var (
	bedroomLadder  = [room.Stages]int{-2, 0, 1, 2, 3, 4, 5, 6, 7, 8}
	barracksLadder = [room.Stages]int{-7, -5, -4, -3, -2, -1, 1, 2, 3, 4}
	hospitalLadder = [room.Stages]int{-4, -2, none, none, 1, 2, 3, 4, 5, 6}
	// fineRoomLadder is the dining and recreation rooms' (none below decent).
	fineRoomLadder = [room.Stages]int{none, none, none, 2, 3, 4, 5, 6, 7, 8}
)

// RoomMood is the thought a room of role with impressiveness tenths gives
// on occasion o: its kind, value and label; ok false when it gives none
// (the role has no ladder for the occasion, or the stage has no rung).
func RoomMood(role room.Role, tenths int, o Occasion) (k mood.Kind, value int, label string, ok bool) {
	var ladder *[room.Stages]int
	var noun string
	switch {
	case o == Slept && (role == room.Bedroom || role == room.PrisonCell):
		k, ladder, noun = mood.SleptInBedroom, &bedroomLadder, "slept in %s bedroom"
	case o == Slept && (role == room.Barracks || role == room.PrisonBarracks):
		k, ladder, noun = mood.SleptInBarracks, &barracksLadder, "slept in %s barracks"
	case o == Slept && role == room.Hospital:
		k, ladder, noun = mood.SleptInHospital, &hospitalLadder, "lay in %s hospital"
	case o == Ate && role == room.Dining:
		k, ladder, noun = mood.AteInDiningRoom, &fineRoomLadder, "ate in %s dining room"
	case o == Enjoyed && role == room.RecRoom:
		k, ladder, noun = mood.EnjoyedRecRoom, &fineRoomLadder, "relaxed in %s recreation room"
	default:
		return 0, 0, "", false
	}
	stage := room.ImpressivenessStage(tenths)
	if ladder[stage] == none {
		return k, 0, "", false
	}
	return k, ladder[stage], stageLabel(noun, stage), true
}

// stageLabel fills noun's %s with the stage's name and its article ("an
// awful", "a dull"). It allocates one short string per occasion.
func stageLabel(noun string, stage int) string {
	name := room.StageName(stage)
	article := "a "
	if name != "" && (name[0] == 'a' || name[0] == 'e' || name[0] == 'u') {
		article = "an "
	}
	for i := 0; i+1 < len(noun); i++ {
		if noun[i] == '%' && noun[i+1] == 's' {
			return noun[:i] + article + name + noun[i+2:]
		}
	}
	return noun
}

// sharedBedValue is the mood of sharing a bed with someone who is not a
// partner: -5 at opinion 0, one more per 20 opinion, -10..0 (wiki:
// Thoughts). The wiki flags this thought as possibly from another game
// version; kept as published. Opinion is in hundredths (social.OpinionMax
// 10000 is 100).
func sharedBedValue(opinion int32) int {
	return max(-10, min(0, -5+int(opinion)/2000))
}
