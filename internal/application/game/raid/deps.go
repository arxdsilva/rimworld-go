package raid

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/grid"
)

// Host is everything the raid cluster needs from the simulation, as
// consumer-owned roles (.cursor/SOLID_GO.md §7, §10). Every method takes
// and returns values, so a host call on the tick path moves no caller
// local to the heap. Roles beyond these eight come in Roles.
type Host interface {
	Roster
	Captives
	Bodies
	Jobs
	Fight
	Ground
	Events
	Dice
}

// Roles are the host roles beyond Host.
type Roles struct {
	Wild  Wild
	Story Story
	// DEF-4 (roles.go): drop pods, thieves and burners, siege mortars,
	// faction rivalry and the combat log.
	Arrive    Arrive
	Mischief  Mischief
	Siege     SiegeWorks
	Chronicle Chronicle
	// Mech is the mechanoids' and ship parts' own role (mech.go).
	Mech MechHost
	// CBT-4: Tactics (tactics.go: cover, grenades, targets, crops,
	// mechanoids' door bashing) and Nest (nest.go: hive mounds).
	Tactics Tactics
	Nest    Nest
	// Kidnap (kidnap.go, moved from the game by CBT-4).
	Kidnap Kidnap
}

// Roster is the raider records and raid groups, which stay on the game.
type Roster interface {
	// Raider is id's record (zero when it has none).
	Raider(id entity.EntityID) Raider
	SetRaider(id entity.EntityID, r Raider)
	// RaiderIDs is every raider, prisoner and visitor on the map, in
	// spawn order (the game's slice: read only).
	RaiderIDs() []entity.EntityID
	// Groups is the game's raid groups: raid writes through the elements.
	Groups() []Group
	// StandingRaider: a hostile raider still on its feet.
	StandingRaider(id entity.EntityID) bool
	// RemoveRaider takes id off the map (gone home).
	RemoveRaider(id entity.EntityID)
	// DropGear leaves a dead raider's weapon where it fell.
	DropGear(id entity.EntityID)
}

// Captives is prisoners, captures and kidnaps.
type Captives interface {
	Prisoner(id entity.EntityID) bool
	// LivePrisoner runs a prisoner or a released one walking out.
	LivePrisoner(id entity.EntityID, now uint64)
	// DropPatient: a downed kidnapper lets go.
	DropPatient(id entity.EntityID)
	// BeingCaptured: a colonist is carrying id off to a prison bed.
	BeingCaptured(id entity.EntityID) bool
}

// Bodies is where creatures are and how they fare.
type Bodies interface {
	Pos(id entity.EntityID) (x, y int, ok bool)
	Vital(id entity.EntityID) health.Record
	// Heal closes a downed raider's wounds on their cadence.
	Heal(id entity.EntityID, v health.Record, now uint64)
	Name(id entity.EntityID) string
	// NearestColonist is the closest standing colonist to (x, y).
	NearestColonist(x, y int) (id entity.EntityID, cx, cy int, ok bool)
	// RaidTarget is what a raider at (x, y) goes for: a colonist or an ally.
	RaidTarget(x, y int) (id entity.EntityID, tx, ty int, ok bool)
	// Home is the colony's centre.
	Home() (x, y int)
}

// Jobs reads and drives creatures' jobs.
type Jobs interface {
	Job(id entity.EntityID) (job.Record, bool)
	SetJob(id entity.EntityID, j job.Record)
	StartJob(id entity.EntityID, j job.Record) bool
	Interrupt(id entity.EntityID, r job.FailReason)
	ReleaseOwner(id entity.EntityID)
}

// Fight is the combat cluster as raiders use it.
type Fight interface {
	WalkableNear(x, y, r int) (int, int, bool)
	// Strike runs an attack job.
	Strike(id entity.EntityID, j job.Record, now uint64)
	Weapon(id entity.EntityID) weapon.Def
	CanHit(x, y int, d weapon.Def, tx, ty int) bool
	AimTicks(d weapon.Def) int
	// Breach runs a breach job; StartBreach plans one through the walls.
	Breach(id entity.EntityID, j job.Record)
	StartBreach(id entity.EntityID, tx, ty int) bool
	// TurretFor is a powered turret in sight and well closer than the
	// colonist cd tiles away, which draws a raider's fire instead.
	TurretFor(x, y, cd, sight int) (id entity.EntityID, bx, by int, ok bool)
}

// Ground is the map.
type Ground interface {
	Size() (w, h int)
	InBounds(x, y int) bool
	// Rock: natural rock wall.
	Rock(x, y int) bool
	DryWalkable(x, y int) bool
	// Shape is the grid with its version and the structure version (any
	// terrain or building change), the hive exit's cache key.
	Shape() (g *grid.Grid, gridVer, structVer uint64)
	// Mine opens a dug-out rock tile.
	Mine(x, y int)
	NearestEdge(x, y int) (int, int)
	// Build places a finished building (a siege's sandbags).
	Build(kind, stuff, facing string, x, y int)
}

// Events posts messages and threat letters.
type Events interface {
	Message(x, y int, text string)
	Threat(title string, x, y int, text string)
}

// Dice rolls on the game's main stream.
type Dice interface {
	Roll(n int) int
}

// Wild spawns animals at the map edge.
type Wild interface {
	// PickSpecies is a season-fitting species that keep accepts.
	PickSpecies(keep func(animal.Species) bool) (animal.Species, bool)
	SpawnAnimal(s animal.Species, x, y int) (entity.EntityID, error)
	Animal(id entity.EntityID) animal.State
	SetAnimal(id entity.EntityID, a animal.State)
	Plural(s animal.Species) string
	// EdgeTile is a walkable map-edge tile the colony can be reached from.
	EdgeTile() (int, int, bool)
}

// Story is the threats the storyteller sends through the game.
type Story interface {
	ManhunterPack(n int)
	// MadAnimal: one wild animal near home goes manhunter (false: none could).
	MadAnimal(now uint64) bool
	ScaledRaid(day int)
	// Enclosed: no colonist can be reached from the map edge without
	// breaking through something.
	Enclosed() bool
	// RaidPoints is what a raid may spend on colony day day; ShipPart
	// fires the crashed ship part incident (false: it could not happen).
	RaidPoints(day int) int
	ShipPart() bool
}
