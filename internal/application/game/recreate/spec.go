package recreate

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/recreation"
	"rimworld-go/internal/domain/pawn/skill"
)

// Mode is how a recreation piece is enjoyed.
type Mode uint8

const (
	// Toss: thrown at from a few tiles off (horseshoes, ring toss).
	Toss Mode = iota + 1
	// Play: played from a cell beside it, sitting on a chair there if
	// there is one (boards and tables).
	Play
	// Watch: watched from its watch area, a seat preferred (televisions).
	Watch
	// Gaze: looked through from its spot under the night sky (telescope).
	Gaze
	// Admire: art looked at from a cell beside it (display case and pedestal
	// while holding a stack, sculptures always).
	Admire
)

// Spec is one recreation piece's numbers.
type Spec struct {
	Mode Mode
	Joy  recreation.JoyKind
	// Power is its recreation power, permille (wiki: each item's page).
	Power int
	// Users is how many colonists use it at once.
	Users int
	// Quality: the piece's quality scales Power (wiki: Quality,
	// recreation power; game tables only, not screens or the telescope).
	Quality bool
	// Trains is the skill it trains (Learns false: none).
	Trains skill.ID
	Learns bool
	// Verb is what a colonist doing it reads as ("play chess").
	Verb string
}

// SpecOf is kind k's recreation numbers; ok false when k is no
// recreation piece. wiki: Horseshoes pin, Hoopstone ring, Game of Ur,
// Chess table, Billiards table, Poker table, Tube television, Flatscreen
// television, Megascreen television, Telescope (power, users, joy kind,
// skill; the horseshoe pin's eight throwing spots are this game's).
func SpecOf(k construction.Kind) (s Spec, ok bool) {
	switch k {
	case construction.HorseshoePin:
		return Spec{Toss, recreation.JoyDexterity, 1000, len(tossSpots), true, skill.Shooting, true, "play horseshoes"}, true
	case construction.RingToss:
		return Spec{Toss, recreation.JoyDexterity, 1000, 3, true, skill.Shooting, true, "toss rings"}, true
	case construction.UrBoard:
		return Spec{Play, recreation.JoyCerebral, 800, 2, true, skill.Intellectual, true, "play ur"}, true
	case construction.ChessTable:
		return Spec{Play, recreation.JoyCerebral, 1000, 2, true, skill.Intellectual, true, "play chess"}, true
	case construction.BilliardsTable:
		return Spec{Play, recreation.JoyDexterity, 1300, 2, true, skill.Shooting, true, "play billiards"}, true
	case construction.PokerTable:
		return Spec{Play, recreation.JoyCerebral, 1300, 4, true, skill.Intellectual, true, "play poker"}, true
	case construction.TubeTV:
		return Spec{Mode: Watch, Joy: recreation.JoyTelevision, Power: 1200, Users: 15, Verb: "watch television"}, true
	case construction.FlatscreenTV:
		return Spec{Mode: Watch, Joy: recreation.JoyTelevision, Power: 1400, Users: 30, Verb: "watch television"}, true
	case construction.BigScreenTV:
		return Spec{Mode: Watch, Joy: recreation.JoyTelevision, Power: 1600, Users: 42, Verb: "watch television"}, true
	case construction.Telescope:
		return Spec{Gaze, recreation.JoyTelescope, 1200, 1, false, skill.Intellectual, true, "stargaze"}, true
	case construction.DisplayCase, construction.RoundPedestal,
		construction.SmallSculpture, construction.LargeSculpture, construction.GrandSculpture:
		// NST-G, original tuning: art gives solitary joy at a fixed power.
		return Spec{Mode: Admire, Joy: recreation.JoySolitary, Power: 700, Users: 1, Verb: "admire"}, true
	}
	return Spec{}, false
}

// JobOf is the job a colonist enjoying a piece of mode m does.
func JobOf(k construction.Kind, m Mode) job.Kind {
	switch {
	case k == construction.HorseshoePin:
		return job.Horseshoes // the saved job of older games
	case m == Watch:
		return job.WatchTelevision
	case m == Gaze:
		return job.Stargaze
	}
	return job.Recreate
}

// Uses reports a job this package runs (Do).
func Uses(k job.Kind) bool {
	return k == job.Horseshoes || k == job.Recreate || k == job.WatchTelevision || k == job.Stargaze
}

// tossSpots are where a thrower stands, three tiles off the pin or ring.
var tossSpots = [...][2]int{{0, 3}, {3, 0}, {0, -3}, {-3, 0}, {2, 2}, {-2, 2}, {2, -2}, {-2, -2}}

// ActivityOf maps a free-time job without a recreation piece to its
// recreation activity (moved from game/idle.go).
func ActivityOf(k job.Kind) recreation.Activity {
	switch k {
	case job.Wander:
		return recreation.Wander
	case job.Chat, job.SocialRelax:
		return recreation.Chat
	case job.Relax:
		return recreation.Relax
	case job.FireSide:
		return recreation.FireSide
	}
	return recreation.None
}

// joyOfIdle is the joy kind of a free-time job without a recreation
// piece (wiki: Recreation: walking and sitting are solitary relaxation,
// chatting and gatherings social).
func joyOfIdle(k job.Kind) recreation.JoyKind {
	switch k {
	case job.Wander, job.Relax, job.FireSide, job.SitDown:
		return recreation.JoySolitary
	case job.Chat, job.SocialRelax, job.Gather:
		return recreation.JoySocial
	case job.Read:
		return recreation.JoyReading
	}
	return recreation.JoyNone
}

// Mood is the recreation thought of a colonist at recreation joy (moved
// from game/rooms.go): starved, bored or entertained; ok false between.
func Mood(joy int) (k mood.Kind, ok bool) {
	switch {
	case joy < recreation.Starved:
		return mood.RecreationStarved, true
	case joy < recreation.Bored:
		return mood.Bored, true
	case joy >= recreation.Entertained:
		return mood.Entertained, true
	}
	return 0, false
}

// PassionDrain is recreation drain rate r (permille) while doing work of
// passion p: 20 % less for a minor passion, 40 % for a major one (moved
// from game/skills.go).
func PassionDrain(r int32, p skill.Passion) int32 {
	switch p {
	case skill.Minor:
		return r * 800 / 1000
	case skill.Major:
		return r * 600 / 1000
	}
	return r
}
