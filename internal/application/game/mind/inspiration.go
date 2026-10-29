package mind

import (
	"maps"

	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// Inspirations (PRD-2, wiki: Inspiration): a colonist in a good mood may
// be inspired. Only inspired creativity exists so far: the next thing
// the colonist crafts or builds with a quality rolls two levels up
// (craft/quality.go spends it), or it fades after InspireDuration. The
// other vanilla inspirations (frenzies, surgery, taming, recruitment,
// trade) are not modelled yet.
//
// No flapping: an inspiration starts only at InspireMinMood or above
// and never ends with mood, only by being used or running out; after it
// ends, none starts again for InspireCalm.
const (
	// InspireMinMood: no inspiration below this mood (wiki: 50 %).
	InspireMinMood = 50
	// InspireDuration: an unused inspiration fades after 8 days (wiki).
	InspireDuration = 8 * clock.TicksPerDay
	// InspireCalm: no new inspiration for a day after one ends (original
	// tuning: the wiki states no cooldown, only one at a time).
	InspireCalm = clock.TicksPerDay
	// CreativityMinSkill: inspired creativity needs Crafting or
	// Construction at this level (wiki: Artistic, Crafting or
	// Construction 3; there is no Artistic skill).
	CreativityMinSkill = 3
)

// InspirationKind is an inspiration type. Saved: append only.
type InspirationKind uint8

const (
	NoInspiration InspirationKind = iota
	Creativity
)

// String is the player-facing name ("" for none).
func (k InspirationKind) String() string {
	if k == Creativity {
		return "inspired creativity"
	}
	return ""
}

// Inspiration is one colonist's inspiration and the tick it fades.
type Inspiration struct {
	Kind  InspirationKind
	Until uint64
}

// MTBHours is the mean time between inspirations, in hours, at mood
// (0-100): (410 - 4 x mood) days (wiki); 0 below InspireMinMood (none).
func MTBHours(mood int) int {
	if mood < InspireMinMood {
		return 0
	}
	return (410 - 4*min(mood, 100)) * clock.HoursPerDay
}

// Inspirations is the inspiration component (Mind.Inspire).
type Inspirations struct {
	h    Host
	on   map[entity.EntityID]Inspiration
	calm map[entity.EntityID]uint64 // no new inspiration before this tick
}

func newInspirations(h Host) *Inspirations {
	return &Inspirations{h: h, on: map[entity.EntityID]Inspiration{}, calm: map[entity.EntityID]uint64{}}
}

// Of is colonist id's inspiration (ok false: none, or it has faded).
func (n *Inspirations) Of(id entity.EntityID) (Inspiration, bool) {
	in, ok := n.on[id]
	if !ok || n.h.Now() >= in.Until {
		return Inspiration{}, false
	}
	return in, true
}

// Check is colonist id's hourly inspiration check (Mind.Tick, outside a
// break): a faded inspiration ends; else, calm and in a good enough
// mood, it is inspired with the hour's share of the mean time between.
func (n *Inspirations) Check(id entity.EntityID) {
	h := n.h
	now := h.Now()
	if in, ok := n.on[id]; ok {
		if now >= in.Until {
			n.end(id, in.Kind)
			h.InspirationEnded(id, in.Kind.String())
		}
		return
	}
	if now < n.calm[id] {
		return
	}
	mtb := MTBHours(h.Mood(id))
	if mtb == 0 || !n.creative(id) || h.Roll(mtb) != 0 {
		return
	}
	n.Start(id, Creativity)
}

// creative: id is skilled enough for inspired creativity.
func (n *Inspirations) creative(id entity.EntityID) bool {
	return n.h.SkillLevel(id, skill.Crafting) >= CreativityMinSkill || n.h.SkillLevel(id, skill.Construction) >= CreativityMinSkill
}

// Start inspires id with k for InspireDuration (dev tools, fixtures, and
// the hourly check), replacing any it had.
func (n *Inspirations) Start(id entity.EntityID, k InspirationKind) {
	n.on[id] = Inspiration{Kind: k, Until: n.h.Now() + InspireDuration}
	n.h.Inspired(id, k.String())
}

// Use spends id's inspiration of kind k: true when it had one.
func (n *Inspirations) Use(id entity.EntityID, k InspirationKind) bool {
	in, ok := n.Of(id)
	if !ok || in.Kind != k {
		return false
	}
	n.end(id, k)
	return true
}

// end drops id's inspiration and starts its calm.
func (n *Inspirations) end(id entity.EntityID, _ InspirationKind) {
	delete(n.on, id)
	n.calm[id] = n.h.Now() + InspireCalm
}

// export and load are the save's inspirations and calms.
func (n *Inspirations) export() (map[entity.EntityID]Inspiration, map[entity.EntityID]uint64) {
	return maps.Clone(n.on), maps.Clone(n.calm)
}

func (n *Inspirations) load(on map[entity.EntityID]Inspiration, calm map[entity.EntityID]uint64) {
	n.on, n.calm = map[entity.EntityID]Inspiration{}, map[entity.EntityID]uint64{}
	maps.Copy(n.on, on)
	maps.Copy(n.calm, calm)
}
