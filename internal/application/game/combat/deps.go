package combat

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/combat/armor"
	"rimworld-go/internal/domain/combat/projectile"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/entity"
)

// Host is everything combat needs from the simulation, as consumer-owned
// roles (.cursor/SOLID_GO.md §7, §10). Every method takes and returns
// values (or a slice combat owns), so a host call on the tick path moves
// no caller local to the heap. Roles beyond these eight come in Roles.
type Host interface {
	Creatures
	Rosters
	Ground
	Skills
	Jobs
	Harm
	Events
	Dice
}

// Roles are the host roles beyond Host, each its own consumer-owned
// interface so Host stays at eight roles.
type Roles struct {
	Power  Power
	Flight Flight
	Hunts  Hunts
	Breach Breaches
}

// Creatures answers who a creature is and where.
type Creatures interface {
	Now() uint64
	// Pos is id's tile (ok false when it has none).
	Pos(id entity.EntityID) (x, y int, ok bool)
	// Animal is id's animal state (ok false: not an animal).
	Animal(id entity.EntityID) (animal.State, bool)
	// Weapon is what id wields (weapon.Fists when unarmed).
	Weapon(id entity.EntityID) weapon.Kind
	// StandingPawn: a colonist alive and not downed.
	StandingPawn(id entity.EntityID) bool
	// StandingRaider: a hostile raider on its feet (never a prisoner,
	// visitor or trader).
	StandingRaider(id entity.EntityID) bool
	// Hostile: animal a is attacking a colonist.
	Hostile(a animal.State) bool
	// Armor appends the garments id wears, outermost first, to buf (a
	// slice combat owns) and returns it.
	Armor(id entity.EntityID, buf []armor.Layer) []armor.Layer
}

// Who picks the creatures Near collects.
type Who uint8

// The creature kinds Near can collect (the same bits as the game's
// creature index kinds).
const (
	Pawns Who = 1 << iota
	Raiders
	Animals
	Everyone = Animals | Raiders | Pawns
)

// Seen is a creature Near found, with its tile.
type Seen struct {
	ID   entity.EntityID
	X, Y int
}

// Rosters lists creatures.
type Rosters interface {
	// AnimalIDs and RaiderIDs are the host's own id slices, read only.
	AnimalIDs() []entity.EntityID
	RaiderIDs() []entity.EntityID
	// Near appends to buf every creature of a kind in who within
	// Chebyshev radius r of (x, y), in no particular order, and returns
	// it (the creature index: no scan of every creature).
	Near(x, y, r int, who Who, buf []Seen) []Seen
	Drafted(id entity.EntityID) bool
	Name(id entity.EntityID) string
	// RaiderOn is raider t's tile when it is standing and attacking id
	// (ok false otherwise): one call for the threat scan's raider loop.
	RaiderOn(t, id entity.EntityID) (x, y int, ok bool)
}

// Ground is the map: rock, buildings and their changes.
type Ground interface {
	InBounds(x, y int) bool
	Rock(x, y int) bool
	// Object is the object-layer building on (x, y) and its kind (id 0:
	// none).
	Object(x, y int) (entity.EntityID, construction.Kind)
	// Building is building id (ok false: gone).
	Building(id entity.EntityID) (construction.Building, bool)
	// AnyBuilt: a building of kind k stands somewhere.
	AnyBuilt(k construction.Kind) bool
	RemoveBuilding(id entity.EntityID)
	// PlaceFrame lays a blueprint of kind k in stuff.
	PlaceFrame(k construction.Kind, stuff item.Kind, f construction.Facing, x, y int)
	// DigRock hacks dmg off the rock tile (x, y).
	DigRock(x, y, dmg int)
}

// Skills is a humanlike's fighting: skill, stats and capability.
type Skills interface {
	// HitChance is base adjusted for id's Shooting or Melee and accuracy.
	HitChance(id entity.EntityID, ranged bool, base int) int
	// ShotAim is id's shooting accuracy per tile (ppm, from its Shooting
	// skill) and its Shooting accuracy stat offset (permille).
	ShotAim(id entity.EntityID) (perTilePPM, offset int)
	// Learn gives id fighting experience for an attack.
	Learn(id entity.EntityID, ranged bool)
	// AimTicks is cooldown adjusted for id's aiming.
	AimTicks(id entity.EntityID, cooldown int) int
	CanViolence(id entity.EntityID) bool
	// FightsBack: armed and capable of violence.
	FightsBack(id entity.EntityID) bool
}

// Jobs reads and starts creatures' jobs.
type Jobs interface {
	Job(id entity.EntityID) (job.Record, bool)
	SetJob(id entity.EntityID, j job.Record)
	StartJob(id entity.EntityID, j job.Record) bool
	Interrupt(id entity.EntityID, reason job.FailReason)
	PathTo(id entity.EntityID, x, y int) ([]pathing.Point, bool)
}

// Harm applies what an attack does.
type Harm interface {
	// Hurt wounds target by dmg from by's weapon; HurtAs with a wound
	// kind and a cause of death.
	Hurt(target entity.EntityID, dmg int, by entity.EntityID)
	HurtAs(target entity.EntityID, dmg int, by entity.EntityID, kind body.WoundKind, cause health.Cause)
	// DamageBuilding harms building id (construction: it breaks and
	// leaves a blueprint; a sentry turret then calls Turrets.Blast).
	DamageBuilding(id entity.EntityID, dmg int)
	// Stain bloodies the ground for a hit from (ax, ay) on target at
	// (tx, ty), before the hurt.
	Stain(target entity.EntityID, ax, ay, tx, ty, dmg int)
	// Trace records an attack for presentation.
	Trace(t projectile.Trace)
	// Struck and Hit are the presentation stamps of the last blow given
	// and taken.
	Struck(attacker, target entity.EntityID, tick uint64)
	Hit(target entity.EntityID, tick uint64, fromX, fromY, dmg int)
}

// Events records what combat did, for logs, messages and effects.
type Events interface {
	LogStrike(attacker, target entity.EntityID, wk weapon.Kind, hit bool, dmg int)
	// Fled: id ran from threat.
	Fled(id, threat entity.EntityID)
	// Shot: a sentry turret (named by) shot at target.
	Shot(target entity.EntityID, hit bool, dmg int, by string)
	// Trapped: a spike trap dealt dmg to victim.
	Trapped(victim entity.EntityID, dmg int)
	Message(x, y int, text string)
	// Burst is a destroyed turret's blast for the renderer; Ignite may set
	// (x, y) alight.
	Burst(x, y int, tick uint64)
	Ignite(x, y int)
}

// Dice rolls on the game's streams: uniform draws in [0, n).
type Dice interface {
	// Roll draws from the main stream (strikes, traps).
	Roll(n int) int
	// DefenseRoll draws from the defence stream (turrets).
	DefenseRoll(n int) int
}

// Power is the colony's electricity as turrets see it.
type Power interface {
	// Wired: the colony has a power device at all.
	Wired() bool
	Powered(id entity.EntityID) bool
	// PowerGen is the power list's generation; it moves whenever the
	// powered buildings change.
	PowerGen() uint64
	// AppendDefense appends the defence consumers (the turrets), ascending.
	AppendDefense(buf []entity.EntityID) []entity.EntityID
}

// Flight is an unarmed colonist's flight and the calm window after it.
// The window (the tick a colonist that fled holds back from work until)
// stays on the game, which lying down and stashing also clear.
type Flight interface {
	// FleeGoal is a tile for id at (x, y) to run to from (tx, ty).
	FleeGoal(id entity.EntityID, x, y, tx, ty int) (int, int, bool)
	// DropForFlight puts j down (an order goes back on the queue).
	DropForFlight(id entity.EntityID, j job.Record)
	// CheckThreats cross-checks a threat scan in self-check builds.
	CheckThreats(id entity.EntityID, x, y, animalR, raiderR int, got entity.EntityID, gotD int)
	// Calm is id's calm window end (ok false: none); SetCalm and
	// ForgetCalm write it.
	Calm(id entity.EntityID) (until uint64, ok bool)
	SetCalm(id entity.EntityID, until uint64)
	ForgetCalm(id entity.EntityID)
}

// Hunts are the hunting area's rules: a hunt is given up when the prey
// stays out of the hunter's area.
type Hunts interface {
	ClearHuntOut(id entity.EntityID)
	HuntGivenUp(id entity.EntityID, j job.Record, tx, ty int, now uint64) bool
	AbandonHunt(id entity.EntityID, j job.Record)
}

// Breaches plans a raider's way in.
type Breaches interface {
	// BreachRoute is id's path to (tx, ty) with every wall and door (and,
	// for a sapper, rock) knocked down.
	BreachRoute(id entity.EntityID, tx, ty int) ([]pathing.Point, bool)
	// Query is the walkability id plans with.
	Query(id entity.EntityID) pathing.Query
}
