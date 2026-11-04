package sense

import (
	"cmp"
	"maps"
	"slices"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/gameplay"
	"rimworld-go/internal/domain/pawn/inventory"
	"rimworld-go/internal/domain/sim/entity"
)

// Save is the habits on disk: nil when nothing differs from a new colony, so
// an older save loads as the defaults. Every list is sorted by its id.
type Save struct {
	Rules    gameplay.Settings
	Resume   []ResumeEntry
	TidyAt   []TickEntry
	HaulAt   []TickEntry
	NoClean  []entity.EntityID
	TopOff   []entity.EntityID
	Packs    []PackEntry
	FromPack []CarryEntry
	SpareAt  []TickEntry
	UnloadAt []TickEntry
}

// ResumeEntry is the job a pawn goes back to after a tidy.
type ResumeEntry struct {
	Pawn entity.EntityID
	Job  job.Record
}

// TickEntry is a pawn's cooldown.
type TickEntry struct {
	Pawn  entity.EntityID
	Until uint64
}

// PackEntry is a pawn's pack.
type PackEntry struct {
	Pawn entity.EntityID
	Pack inventory.Pack
}

// CarryEntry is the carried stack a pawn took out of its pack.
type CarryEntry struct {
	Pawn entity.EntityID
	Item entity.EntityID
}

// Export is the habits to save, in id order; nil at the defaults.
func (s *Sense) Export() *Save {
	out := Save{
		Rules:    s.Rules,
		TidyAt:   ticksOf(s.TidyAt),
		HaulAt:   ticksOf(s.HaulAt),
		SpareAt:  ticksOf(s.SpareAt),
		UnloadAt: ticksOf(s.UnloadAt),
		NoClean:  idsOf(s.NoClean),
		TopOff:   idsOf(s.TopOffOn),
	}
	for _, id := range slices.Sorted(maps.Keys(s.Resume)) {
		out.Resume = append(out.Resume, ResumeEntry{id, s.Resume[id]})
	}
	for _, id := range slices.Sorted(maps.Keys(s.Packs)) {
		out.Packs = append(out.Packs, PackEntry{id, s.Packs[id]})
	}
	for _, id := range slices.Sorted(maps.Keys(s.FromPack)) {
		out.FromPack = append(out.FromPack, CarryEntry{id, s.FromPack[id]})
	}
	if out.isZero() {
		return nil
	}
	return &out
}

func (s Save) isZero() bool {
	return s.Rules == (gameplay.Settings{}) && len(s.Resume)+len(s.TidyAt)+len(s.HaulAt)+len(s.NoClean)+
		len(s.TopOff)+len(s.Packs)+len(s.FromPack)+len(s.SpareAt)+len(s.UnloadAt) == 0
}

// Import replaces the habits with saved ones; nil gives the defaults.
func (s *Sense) Import(in *Save) {
	if in == nil {
		in = &Save{}
	}
	s.Rules = in.Rules
	s.TidyAt = ticksFrom(in.TidyAt)
	s.HaulAt = ticksFrom(in.HaulAt)
	s.SpareAt = ticksFrom(in.SpareAt)
	s.UnloadAt = ticksFrom(in.UnloadAt)
	s.NoClean = idsFrom(in.NoClean)
	s.TopOffOn = idsFrom(in.TopOff)
	s.Resume = make(map[entity.EntityID]job.Record, len(in.Resume))
	for _, e := range in.Resume {
		s.Resume[e.Pawn] = e.Job
	}
	s.Packs = make(map[entity.EntityID]inventory.Pack, len(in.Packs))
	for _, e := range in.Packs {
		s.Packs[e.Pawn] = e.Pack
	}
	s.FromPack = make(map[entity.EntityID]entity.EntityID, len(in.FromPack))
	for _, e := range in.FromPack {
		s.FromPack[e.Pawn] = e.Item
	}
}

func ticksOf(m map[entity.EntityID]uint64) []TickEntry {
	var out []TickEntry
	for id, until := range m {
		out = append(out, TickEntry{id, until})
	}
	slices.SortFunc(out, func(a, b TickEntry) int { return cmp.Compare(a.Pawn, b.Pawn) })
	return out
}

func ticksFrom(in []TickEntry) map[entity.EntityID]uint64 {
	out := make(map[entity.EntityID]uint64, len(in))
	for _, e := range in {
		out[e.Pawn] = e.Until
	}
	return out
}

func idsOf(m map[entity.EntityID]bool) []entity.EntityID {
	var out []entity.EntityID
	for id, on := range m {
		if on {
			out = append(out, id)
		}
	}
	slices.Sort(out)
	return out
}

func idsFrom(in []entity.EntityID) map[entity.EntityID]bool {
	out := make(map[entity.EntityID]bool, len(in))
	for _, id := range in {
		out[id] = true
	}
	return out
}
