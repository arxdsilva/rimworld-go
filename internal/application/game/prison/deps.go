package prison

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/entity"
)

// Host is everything prison needs from the simulation, as consumer-owned
// roles (.cursor/SOLID_GO.md §7, §10). Every method takes and returns
// values, so a host call on the tick path moves nothing to the heap.
type Host interface {
	Bodies
	Roster
	Life
	Cells
	Wardens
	Talk
	Fate
	Escapes
}

// Bodies reads and writes a pawn's place, vital record and job.
type Bodies interface {
	Now() uint64
	// Pos is id's tile (ok false when it has none).
	Pos(id entity.EntityID) (x, y int, ok bool)
	Vital(id entity.EntityID) (health.Record, bool)
	SetVital(id entity.EntityID, v health.Record)
	Job(id entity.EntityID) (job.Record, bool)
	SetJob(id entity.EntityID, j job.Record)
}

// Held is the raid roster's record of a prisoner, a released pawn or a
// visitor: walking off the map (since Idle, no new try before RetryAt),
// the food band (Hungry, no food run before FoodRetry) and the warden
// who last came.
type Held struct {
	Leaving, Hungry          bool
	FoodRetry, Idle, RetryAt uint64
	Warden                   entity.EntityID
}

// Roster is the raid roster prisoners live on, and the prison's dice.
type Roster interface {
	// RaiderIDs is the host's own ascending id slice, read only.
	RaiderIDs() []entity.EntityID
	Held(id entity.EntityID) Held
	SetHeld(id entity.EntityID, r Held)
	// Captured: id is in a captor's arms.
	Captured(id entity.EntityID) bool
	// Owner is who holds a reservation on target.
	Owner(target entity.EntityID) (entity.EntityID, bool)
	ReleaseOwner(owner entity.EntityID)
	// Roll is a uniform draw in [0, n) from the host's prison stream.
	Roll(n int) int
}

// Life is a held pawn's body: needs, healing, going down and dying.
type Life interface {
	Needs(id entity.EntityID) needs.Pair
	SetNeeds(id entity.EntityID, n needs.Pair)
	// Heal is one tick of healing; it returns the updated record.
	Heal(id entity.EntityID, v health.Record, now uint64) health.Record
	// Decay is one awake tick of needs.
	Decay(id entity.EntityID, n needs.Pair, now uint64) needs.Pair
	// DownedHunger is one tick of a downed pawn's food; it returns the
	// stored needs and the updated record.
	DownedHunger(id entity.EntityID, v health.Record, now uint64) (needs.Pair, health.Record)
	Down(id entity.EntityID, c health.Cause)
	Die(id entity.EntityID, c health.Cause)
}

// Cells is where a held pawn goes: jobs, beds and its room.
type Cells interface {
	StartJob(id entity.EntityID, rec job.Record) bool
	Interrupt(id entity.EntityID, reason job.FailReason)
	BesideSpot(id entity.EntityID, x, y int) (int, int, bool)
	ReleaseBed(id entity.EntityID)
	// SleepSpot is the bed id holds, if it still stands, and the cell it
	// sleeps on there.
	SleepSpot(id entity.EntityID) (bed entity.EntityID, x, y int, ok bool)
	// BedAt is that bed's anchor and its room.
	BedAt(id entity.EntityID) (x, y, room int, ok bool)
	// FreeInRoom: (x, y) is a dry walkable tile of room no one reserved.
	FreeInRoom(x, y, room int) bool
}

// Wardens is the warden's side: beds and meals (the care package) and
// its own work.
type Wardens interface {
	// BedBack sends warden id to carry a downed prisoner back to bed.
	BedBack(id entity.EntityID) bool
	// FeedRun sends warden id for food to bring prisoner p.
	FeedRun(id, p entity.EntityID) bool
	// Chase follows prisoner p that moved, every so many ticks.
	Chase(id, p entity.EntityID, j job.Record, every int)
	// SocialWork is one tick of id's Social work (it trains).
	SocialWork(id entity.EntityID) int
	ConsciousEnough(id entity.EntityID) bool
}

// Talk is a warden's chat with a prisoner.
type Talk interface {
	// Meet registers both as known people.
	Meet(a, b entity.EntityID)
	Opinion(from, to entity.EntityID) int32
	// Mood is the mood a prisoner's resistance answers to.
	Mood(id entity.EntityID) int
	// Negotiation is warden id's Social impact, permille.
	Negotiation(id entity.EntityID) int32
	SocialLevel(id entity.EntityID) int
	// Chat counts as a chat between a and b (memories, log).
	Chat(a, b entity.EntityID, now uint64)
}

// Fate is what becomes of a prisoner, and the news of it.
type Fate interface {
	// Recruit: p, no longer a prisoner, joins the colony won over by w.
	Recruit(p, w entity.EntityID)
	// Execute: warden w kills prisoner p.
	Execute(p, w entity.EntityID)
	PersonName(id entity.EntityID) string
	Message(x, y int, text string)
}

// Escapes is a prison break (CBT-4, escape.go).
type Escapes interface {
	// RoomOf is the room id stands in (0: none, outdoors).
	RoomOf(id entity.EntityID) int
	// Escape turns id, no longer a prisoner, into a hostile breaking out:
	// it fights its way out and flees (the raid roster's Escaping).
	Escape(id entity.EntityID)
	// Breakout announces a prison break of n started by id.
	Breakout(id entity.EntityID, n int)
	// Exit is a walkable tile by the map edge nearest (x, y) (ok false:
	// none) and whether (x, y) is on the edge (CBT-4: a released pawn's
	// walk out, moved from the game).
	Exit(x, y int) (tx, ty int, edge, ok bool)
	// Depart takes a released pawn that reached the edge off the map (its
	// faction grateful).
	Depart(id entity.EntityID)
}
