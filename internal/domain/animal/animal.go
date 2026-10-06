package animal

import (
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

// Species is a kind of animal.
type Species uint8

const (
	// Dustbuck is a skittish grazer: it wanders in small herds and flees
	// from people and predators. Hunted for meat.
	Dustbuck Species = iota
	// Ridgewolf is a predator: it hunts dustbucks when hungry, attacks
	// people who come too close while hungry, and fights back when hurt.
	Ridgewolf
	numSpecies
)

var names = [numSpecies]string{"dustbuck", "ridgewolf"}

func (s Species) String() string {
	if s >= numSpecies {
		return "animal"
	}
	return names[s]
}

// ParseSpecies maps a name to a species.
func ParseSpecies(name string) (Species, bool) {
	for i, n := range names {
		if n == name {
			return Species(i), true
		}
	}
	return 0, false
}

// Def is a species definition. Step ticks are per tile moved.
type Def struct {
	MaxHP int
	// Corpse is the item left on death (meat comes from butchering it).
	Corpse    item.Kind
	StepTicks int
	FleeStep  int
	Meat      int
	Attack    weapon.Kind
	Predator  bool
	Skittish  bool
	// FleeRadius: skittish animals flee threats this close.
	FleeRadius int
	// AggroRadius: hungry predators attack people this close.
	AggroRadius int
	// HungerTicks until a predator starts hunting.
	HungerTicks uint64
	// HuntRadius: how far a hungry predator looks for prey.
	HuntRadius   int
	WanderRadius int
}

var defs = [numSpecies]Def{
	Dustbuck: {MaxHP: 40, Corpse: item.CorpseDustbuck, StepTicks: 18, FleeStep: 10, Meat: 12, Attack: weapon.Kick,
		Skittish: true, FleeRadius: 5, WanderRadius: 4},
	Ridgewolf: {MaxHP: 60, Corpse: item.CorpseRidgewolf, StepTicks: 14, FleeStep: 14, Meat: 6, Attack: weapon.Bite,
		Predator: true, AggroRadius: 5, HungerTicks: 90000, HuntRadius: 22, WanderRadius: 6},
}

// DefOf returns the definition for s.
func DefOf(s Species) Def {
	if s >= numSpecies {
		return defs[Dustbuck]
	}
	return defs[s]
}

// Mode is what an animal is doing.
type Mode uint8

const (
	Wander Mode = iota
	Flee
	Attack
)

func (m Mode) String() string {
	switch m {
	case Flee:
		return "fleeing"
	case Attack:
		return "attacking"
	default:
		return "wandering"
	}
}

// State is one living animal.
type State struct {
	Species  Species
	Injury   int
	Mode     Mode
	Target   entity.EntityID
	Attacker entity.EntityID
	// FleeUntil keeps a hurt grazer running for a while.
	FleeUntil uint64
	// Hunger counts ticks since the last meal (predators).
	Hunger     uint64
	Designated bool // marked for hunting
	NextThink  uint64
	Cooldown   int
	Path       []pathing.Point
	PathI      int
	GoalX      int
	GoalY      int
}

// HP is remaining hit points.
func (s State) HP() int {
	hp := DefOf(s.Species).MaxHP - s.Injury
	if hp < 0 {
		return 0
	}
	return hp
}

// Dead reports whether the animal has no hit points left.
func (s State) Dead() bool { return s.HP() <= 0 }

// Hungry reports whether a predator wants to hunt.
func (s State) Hungry() bool {
	d := DefOf(s.Species)
	return d.Predator && s.Hunger >= d.HungerTicks
}

// Step is the ticks per tile for the current mode.
func (s State) Step() int {
	d := DefOf(s.Species)
	if s.Mode == Flee && d.FleeStep > 0 {
		return d.FleeStep
	}
	return d.StepTicks
}

// FleeGoal is a point dist tiles from (x, y) directly away from the
// threat at (tx, ty). The caller clamps it to the map and finds a
// walkable tile near it.
func FleeGoal(x, y, tx, ty, dist int) (int, int) {
	dx, dy := x-tx, y-ty
	if dx == 0 && dy == 0 {
		dx = 1
	}
	ax, ay := dx, dy
	if ax < 0 {
		ax = -ax
	}
	if ay < 0 {
		ay = -ay
	}
	m := ax
	if ay > m {
		m = ay
	}
	return x + dx*dist/m, y + dy*dist/m
}
