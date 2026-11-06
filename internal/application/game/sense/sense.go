package sense

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/gameplay"
	"rimworld-go/internal/domain/pawn/inventory"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// Sense is the colony's habit rules, the state they keep and the components
// that act on them (Common Sense's behaviour, COMMON_SENSE_GAP_ANALYSIS.md).
// One lives on the world state, built once; everything in it is saved
// (save.go) and the zero rules are the defaults.
//
// The fields are the contract between the components: each file owns its
// component, none adds a field here.
type Sense struct {
	Rules gameplay.Settings

	Resume   map[entity.EntityID]job.Record // the job a tidy interrupted
	TidyAt   map[entity.EntityID]uint64     // no tidy before this tick
	HaulAt   map[entity.EntityID]uint64     // CSE-035's cooldown
	SpareAt  map[entity.EntityID]uint64     // CSE-005's retry
	UnloadAt map[entity.EntityID]uint64     // CSE-038's retry
	NoClean  map[entity.EntityID]bool       // benches that opted out of tidying
	TopOffOn map[entity.EntityID]bool       // CSE-002: not wanting recreation, from the high mark down to the low one
	Packs    map[entity.EntityID]inventory.Pack
	FromPack map[entity.EntityID]entity.EntityID // the carried stack came from the pack

	Tidy   Tidy
	Gear   Gear
	TopOff TopOff
	Relax  Relax
}

// Roles are the game's sides of the components that act on the tick.
type Roles struct {
	Tidy TidyRoles
	Gear GearRoles
	Joy  JoyRoles
}

// New builds the habits on their roles, with the default rules.
func New(r Roles) *Sense {
	s := &Sense{
		Resume:   make(map[entity.EntityID]job.Record),
		TidyAt:   make(map[entity.EntityID]uint64),
		HaulAt:   make(map[entity.EntityID]uint64),
		SpareAt:  make(map[entity.EntityID]uint64),
		UnloadAt: make(map[entity.EntityID]uint64),
		NoClean:  make(map[entity.EntityID]bool),
		TopOffOn: make(map[entity.EntityID]bool),
		Packs:    make(map[entity.EntityID]inventory.Pack),
		FromPack: make(map[entity.EntityID]entity.EntityID),
	}
	s.Tidy = newTidy(s, r.Tidy)
	s.Gear = newGear(s, r.Gear)
	s.TopOff = newTopOff(s, r.Joy)
	s.Relax = newRelax(s, r.Joy)
	return s
}

// Apply runs one of the habit commands.
func (s *Sense) Apply(c command.Command) {
	switch c.Kind {
	case command.SetGameplayToggle:
		if c.Index >= 0 && c.Index < int(gameplay.NumToggles) {
			s.Rules = s.Rules.With(gameplay.Toggle(c.Index), c.Enabled)
		}
	case command.SetGameplayLimit:
		if c.Index >= 0 && c.Index < int(gameplay.NumLimits) {
			s.Rules = s.Rules.WithLimit(gameplay.Limit(c.Index), c.Count)
		}
	case command.SetBenchClean:
		s.Tidy.SetBench(entity.EntityID(c.Target), c.Enabled)
	case command.MarkUnload:
		s.Gear.Mark(entity.EntityID(c.Pawn), int(c.Mode), c.Index, c.Enabled)
	case command.DropGear, command.ConsumeFromPack, command.SetApparelForced:
		s.Gear.applyGear(c.Kind, entity.EntityID(c.Pawn), int(c.Mode), c.Index, c.Enabled) // drop.go (GEAR-1)
	}
}

// Do runs one tick of a habit's job; PackIngredients belongs to game/craft.
func (s *Sense) Do(id entity.EntityID, j job.Record) {
	switch j.Kind {
	case job.CleanRoom:
		s.Tidy.Do(id, j)
	case job.UnloadPack:
		s.Gear.DoUnload(id, j)
	case job.TakeSpare:
		s.Gear.DoSpare(id, j)
	}
}
