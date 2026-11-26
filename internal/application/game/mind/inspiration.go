// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package mind

import (
	"maps"

	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/pawn/stat"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// Core mental inspirations. Saved Kind values are append-only.
const (
	InspireMinMood     = 50
	InspireDuration    = 8 * clock.TicksPerDay
	CreativityMinSkill = 3
	FocusMinSkill      = 3
)

type InspirationKind uint8

const (
	NoInspiration InspirationKind = iota
	Creativity                    // historical saved kind, must remain 1
	GoFrenzy
	Recruitment
	Surgery
	Taming
	Trade
	ShootFrenzy
	WorkFrenzy
	numInspirations
)

var inspirationLabels = [numInspirations]string{
	NoInspiration: "",
	Creativity:    "inspired creativity",
	GoFrenzy:      "quick steps",
	Recruitment:   "persuasive insight",
	Surgery:       "surgical insight",
	Taming:        "animal insight",
	Trade:         "bargaining insight",
	ShootFrenzy:   "steady aim",
	WorkFrenzy:    "productive focus",
}

func (k InspirationKind) String() string {
	if k >= numInspirations {
		return ""
	}
	return inspirationLabels[k]
}

type Inspiration struct {
	Kind  InspirationKind
	Until uint64
}

// MTBHours converts current mood to mean hours between inspirations.
func MTBHours(mood int) int {
	if mood < InspireMinMood {
		return 0
	}
	return (410 - 4*min(mood, 100)) * clock.HoursPerDay
}

type Inspirations struct {
	h  Host
	on map[entity.EntityID]Inspiration
}

func newInspirations(h Host) *Inspirations {
	return &Inspirations{h: h, on: map[entity.EntityID]Inspiration{}}
}

func (n *Inspirations) Of(id entity.EntityID) (Inspiration, bool) {
	in, ok := n.on[id]
	if !ok || n.h.Now() >= in.Until {
		return Inspiration{}, false
	}
	return in, true
}

func (n *Inspirations) Has(id entity.EntityID, k InspirationKind) bool {
	in, ok := n.Of(id)
	return ok && in.Kind == k
}

// Check is called once per waking pawn's hourly mental check. Every
// candidate comes from the pawn's skills and eligibility, and is
// weighted by its associated passion (wiki: Mental inspiration).
func (n *Inspirations) Check(id entity.EntityID) {
	h := n.h
	now := h.Now()
	if in, ok := n.on[id]; ok {
		if now >= in.Until {
			n.end(id)
			h.InspirationEnded(id, in.Kind.String())
		}
		return
	}
	mtb := MTBHours(h.Mood(id))
	if mtb == 0 || h.Roll(mtb) != 0 {
		return
	}
	if k := n.choose(id); k != NoInspiration {
		n.Start(id, k)
	}
}

// eligible enforces the Core's skill and trait prerequisites.
func (n *Inspirations) eligible(id entity.EntityID, k InspirationKind) bool {
	h := n.h
	switch k {
	case Creativity:
		return h.SkillLevel(id, skill.Artistic) >= CreativityMinSkill ||
			h.SkillLevel(id, skill.Crafting) >= CreativityMinSkill ||
			h.SkillLevel(id, skill.Construction) >= CreativityMinSkill
	case Recruitment, Trade:
		return h.SkillLevel(id, skill.Social) >= FocusMinSkill
	case Surgery:
		return h.SkillLevel(id, skill.Medicine) >= FocusMinSkill
	case Taming:
		return h.SkillLevel(id, skill.Animals) >= FocusMinSkill
	case GoFrenzy:
		return h.CanRun(id)
	case ShootFrenzy:
		return h.CanShoot(id)
	case WorkFrenzy:
		return true
	}
	return false
}

// Only skill-related inspirations gain additional lottery weight.
// Creativity uses the Artistic passion.
func (n *Inspirations) weight(id entity.EntityID, k InspirationKind) int {
	var sk skill.ID
	switch k {
	case Creativity:
		sk = skill.Artistic
	case Recruitment, Trade:
		sk = skill.Social
	case Surgery:
		sk = skill.Medicine
	case Taming:
		sk = skill.Animals
	default:
		return 10
	}
	switch n.h.Passion(id, sk) {
	case skill.Minor:
		return 25
	case skill.Major:
		return 50
	default:
		return 10
	}
}

func (n *Inspirations) choose(id entity.EntityID) InspirationKind {
	var kinds [numInspirations - 1]InspirationKind
	var weights [numInspirations - 1]int
	count, total := 0, 0
	for k := Creativity; k < numInspirations; k++ {
		if !n.eligible(id, k) {
			continue
		}
		w := n.weight(id, k)
		kinds[count], weights[count] = k, w
		count++
		total += w
	}
	if total == 0 {
		return NoInspiration
	}
	roll := n.h.Roll(total)
	for i := 0; i < count; i++ {
		if roll < weights[i] {
			return kinds[i]
		}
		roll -= weights[i]
	}
	return NoInspiration
}

func (n *Inspirations) Start(id entity.EntityID, k InspirationKind) {
	if k <= NoInspiration || k >= numInspirations {
		return
	}
	n.on[id] = Inspiration{Kind: k, Until: n.h.Now() + InspireDuration}
	n.h.Inspired(id, k.String())
}

func (n *Inspirations) Use(id entity.EntityID, k InspirationKind) bool {
	if !n.Has(id, k) {
		return false
	}
	n.end(id)
	return true
}

func (n *Inspirations) end(id entity.EntityID) {
	delete(n.on, id)
}

// Keep the legacy cooldown field in the serialized schema for old saves.
func (n *Inspirations) export() (map[entity.EntityID]Inspiration, map[entity.EntityID]uint64) {
	return maps.Clone(n.on), nil
}

func (n *Inspirations) load(on map[entity.EntityID]Inspiration, _ map[entity.EntityID]uint64) {
	n.on = map[entity.EntityID]Inspiration{}
	maps.Copy(n.on, on)
}

// AdjustStat applies the Core duration bonuses after the pawn's normal
// stat layers, without rebuilding its saved skill/stat state.
func AdjustStat(s stat.ID, base int32, k InspirationKind) int32 {
	switch {
	case k == ShootFrenzy && s == stat.ShootAcc:
		return base + 8 // wiki: Mental inspiration (Inspired shooting)
	case k == GoFrenzy && s == stat.MoveSpeed:
		return base * 140 / 100 // wiki: Mental inspiration (Go frenzy)
	case k == WorkFrenzy && s == stat.WorkSpeed:
		return base * 180 / 100 // wiki: Mental inspiration (Work frenzy)
	}
	return base
}
