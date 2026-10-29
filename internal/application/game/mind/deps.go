package mind

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/pawn/mentalbreak"
	"rimworld-go/internal/domain/sim/entity"
)

// Host is everything mind needs from the simulation, as consumer-owned
// roles (.cursor/SOLID_GO.md §7, §10). Every method takes and returns
// values, so a host call on the tick path moves nothing to the heap.
type Host interface {
	Pawns
	Breaks
	Acts
	Aggression
	Brawls
	Events
}

// Pawns answers who a pawn is, where and how it feels.
type Pawns interface {
	Now() uint64
	// Pos is id's tile (ok false when it has none).
	Pos(id entity.EntityID) (x, y int, ok bool)
	// Alive: not dead (downed counts); Standing: alive and on its feet.
	Alive(id entity.EntityID) bool
	Standing(id entity.EntityID) bool
	// Colonists and Prisoners are the host's own id slices, read only.
	Colonists() []entity.EntityID
	Prisoners() []entity.EntityID
	// Mood is id's mood, 0-100.
	Mood(id entity.EntityID) int
	// BreakShift is id's trait offset to the minor break threshold.
	BreakShift(id entity.EntityID) int
}

// Breaks reads and writes the game's break record and rolls the dice.
type Breaks interface {
	Break(id entity.EntityID) (k mentalbreak.Kind, until uint64, ok bool)
	SetBreak(id entity.EntityID, k mentalbreak.Kind, until uint64)
	ClearBreak(id entity.EntityID)
	// Roll is a uniform draw in [0, n) from the host's social stream.
	Roll(n int) int
}

// Acts is what the minor breaks and the catatonic collapse do.
type Acts interface {
	// Seize takes id out of the player's hands: its job, draft and queued
	// orders go.
	Seize(id entity.EntityID)
	// Wander strolls somewhere near and lingers there.
	Wander(id entity.EntityID, linger int) bool
	CanBinge(id entity.EntityID) bool
	// Binge starts eating far past full (false: no food).
	Binge(id entity.EntityID) bool
	CanHide(id entity.EntityID) bool
	Hide(id entity.EntityID, ticks int)
	// Down lays id down (catatonic); StandUp gets it up again unless
	// something else keeps it down.
	Down(id entity.EntityID)
	StandUp(id entity.EntityID)
}

// Aggression is what the breaks aimed at others do.
type Aggression interface {
	CanViolence(id entity.EntityID) bool
	// Attackable: a standing humanlike or a living animal.
	Attackable(id entity.EntityID) bool
	// NearestCreature is the nearest attackable creature within r of id
	// (0: none).
	NearestCreature(id entity.EntityID, r int) entity.EntityID
	// Approach walks id towards target (false: unreachable).
	Approach(id, target entity.EntityID) bool
	// Insult: a insults b through the social interaction path (b may
	// start a social fight).
	Insult(a, b entity.EntityID)
	// Assault keeps id attacking target: a standing one in a normal
	// attack, a downed one struck where it lies (false: unreachable).
	Assault(id, target entity.EntityID) bool
}

// Brawls is the social fight's brawl job and its blows.
type Brawls interface {
	Job(id entity.EntityID) job.Record
	SetJob(id entity.EntityID, j job.Record)
	// Brawl gives id a fresh brawl job against foe.
	Brawl(id, foe entity.EntityID)
	// StopBrawl cancels id's brawl job, if it has one.
	StopBrawl(id entity.EntityID)
	PathTo(id entity.EntityID, x, y int) ([]pathing.Point, bool)
	// Fighter: id is a standing colonist.
	Fighter(id entity.EntityID) bool
	// Punch throws id's fist at foe (a roll on the social stream);
	// PunchTicks is id's fist cooldown.
	Punch(id, foe entity.EntityID)
	PunchTicks(id entity.EntityID) int
}

// Events records what mind did, for logs, letters and mood.
type Events interface {
	// Started: id broke into k, aimed at target (0: nobody).
	Started(id entity.EntityID, k mentalbreak.Kind, target entity.EntityID)
	// Recovered: id's break k ended (it calms, feels catharsis).
	Recovered(id entity.EntityID, k mentalbreak.Kind)
	// FightStarted logs and announces starter's fight with other.
	FightStarted(starter, other entity.EntityID)
	// Outcome rolls f's memory of the fight with other (none for the
	// dead).
	Outcome(f, other entity.EntityID)
}
