package rage

import (
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/pkg/msgfmt"
)

// Pos is a cell (the game's Position: the same fields, so either converts
// to the other).
type Pos struct {
	X, Y int
}

// Kind picks what a sight query sees, a bit set in the game's creature
// index order (the game's tests pin the bits to its own).
type Kind uint8

// The kinds of creature a sight query sees.
const (
	Pawns   Kind = 1 << iota // colonists (and anyone else the game counts a pawn)
	Raiders                  // raiders
	Animals                  // animals
)

// Host is everything rage needs from the simulation: value-only roles the
// game implements with small adapters (game/handling.go).
type Host interface {
	World
	Beasts
	People
	Ground
	Story
	Fight
}

// World is the clock, the dice, names, the log and the messages.
type World interface {
	Now() uint64
	// Roll is a uniform draw in [0, n) from the simulation's main stream.
	Roll(n int) int
	// Name is a colonist's name; CreatureName how a log names anyone;
	// KillerName how a log names a killer.
	Name(id entity.EntityID) string
	CreatureName(id entity.EntityID) string
	KillerName(id entity.EntityID) string
	// Log adds a line (an evlog kind) to subject's log.
	Log(subject entity.EntityID, kind uint8, other entity.EntityID, name string)
	// Alarm posts a danger message at tick, focused on focus: msg, or text
	// when msg is zero.
	Alarm(tick uint64, x, y int, focus entity.EntityID, text string, msg msgfmt.Text)
	// Threat posts a threat letter.
	Threat(title string, x, y int, format string, args ...any)
}

// Beasts reads and writes the animals.
type Beasts interface {
	// Pos is a creature's cell (ok false: none).
	Pos(id entity.EntityID) (Pos, bool)
	Animal(id entity.EntityID) (animal.State, bool)
	// Ref is the stored animal, read only (nil: none).
	Ref(id entity.EntityID) *animal.State
	SetAnimal(id entity.EntityID, a animal.State)
	// Grown: a is no longer young at now.
	Grown(a animal.State, now uint64) bool
	// Asleep: a lies resting where it is.
	Asleep(a animal.State) bool
	// Hostile: a is of a kind that attacks the colony on sight.
	Hostile(a animal.State) bool
	// Spawn places a fresh animal of species s at (x, y).
	Spawn(s animal.Species, x, y int) (entity.EntityID, error)
}

// People reads the colonists and raiders.
type People interface {
	IsPawn(id entity.EntityID) bool
	IsRaider(id entity.EntityID) bool
	StandingPawn(id entity.EntityID) bool
	StandingRaider(id entity.EntityID) bool
	// Colonist: id is one of the colony's own (never a raider, visitor or
	// prisoner).
	Colonist(id entity.EntityID) bool
	// NearestColonist is the standing colonist nearest p (0: none).
	NearestColonist(p Pos) entity.EntityID
}

// Sight is a sight query's filter, passed by value so a query allocates
// nothing: Keep is a plain function (never a closure) that sees the
// query's rage, its seer and the tick.
type Sight struct {
	R    *Rage
	Self entity.EntityID
	Now  uint64
	Keep func(s Sight, o entity.EntityID) bool
}

// Ground is sight and the walk: the creature index and the paths. The
// animal under thought goes in and out by value, never by pointer: a
// pointer handed to an interface escapes, and every animal would move to
// the heap every tick.
type Ground interface {
	// Nearest is the creature of a kind in k nearest (x, y) within
	// Chebyshev radius r that s keeps (ties: the lower id).
	Nearest(x, y, r int, k Kind, s Sight) (entity.EntityID, Pos, bool)
	// Within is every creature of a kind in k within Chebyshev radius r of
	// (x, y), in ascending id order.
	Within(x, y, r int, k Kind) []entity.EntityID
	// GoTo paths animal id (state a) towards (x, y) (clamped, nudged to a
	// walkable cell) and returns its new state.
	GoTo(id entity.EntityID, a animal.State, x, y int) animal.State
	// PathQuery is an animal's walk query on its layer (tame: fences shut,
	// gates open; wild: both shut).
	PathQuery(tame bool) pathing.Query
	// LayerReach: a walk over q from p may reach (x, y).
	LayerReach(q pathing.Query, p Pos, x, y int) bool
	// WalkableNear is a walkable cell within r of (x, y).
	WalkableNear(x, y, r int) (int, int, bool)
	// FireThreat is the nearest fire a at p should run from.
	FireThreat(a animal.State, p Pos) (Pos, bool)
	InBounds(x, y int) bool
}

// Story is the storyteller's side: difficulty and incidents.
type Story interface {
	// RevengePct is a revenge chance base scaled by the difficulty.
	RevengePct(base int) int
	// PredatorsHuntColonists: the difficulty lets predators hunt people.
	PredatorsHuntColonists() bool
	// PickSpecies is a season-fitting species keep accepts.
	PickSpecies(keep func(animal.Species) bool) (animal.Species, bool)
	// EdgeTile is a walkable map-edge cell to enter from.
	EdgeTile() (int, int, bool)
}

// Fight is the game's side of a fight.
type Fight interface {
	// Strike is a melee blow by attacker on target with weapon kind wk.
	Strike(attacker, target entity.EntityID, wk weapon.Kind)
	// TameHuntOn: tame hunter id may still hunt target at tp (PLAY-017).
	TameHuntOn(id, target entity.EntityID, tp Pos) bool
}
