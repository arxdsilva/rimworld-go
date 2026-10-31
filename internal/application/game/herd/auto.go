package herd

import (
	"fmt"
	"slices"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// Auto-slaughter (FOOD-3, wiki: Animal husbandry): per species the player
// caps the tame adult males, adult females and young; once a day
// (CheckHour) each class over its cap has its surplus marked for
// slaughter, oldest first, never a bonded or pregnant animal (the wiki's
// two toggles, off). Marked animals no longer count, and a mark is never
// lifted by the check, so a herd at its cap is left alone: the daily
// cadence and the one-way mark are the hysteresis (no mark and unmark
// flapping: TestAutoSlaughterNoFlap).
type Auto struct {
	h Host
	s *Settings
}

// Settings is the herd's saved state, kept by the game (gob): the
// auto-slaughter caps by species key (animal.Species.String()).
type Settings struct {
	Caps map[string]Caps
}

// Caps are one species' herd caps (-1: no cap).
type Caps struct {
	Males, Females, Young int
}

// CheckHour is the hour of the day the auto-slaughter check runs.
const CheckHour = 7

// CheckTick reports tick now as the day's check tick.
func CheckTick(now uint64) bool { return now%clock.TicksPerDay == CheckHour*clock.TicksPerHour }

// Apply sets a species' caps from command.SetAutoSlaughter (Name the
// species key, X males, Y females, X2 young; -1 none, and all -1 drops
// the row); an unknown species is ignored.
func (au *Auto) Apply(c command.Command) {
	if _, ok := animal.ParseSpecies(c.Name); !ok || au.s == nil {
		return
	}
	cp := Caps{Males: max(c.X, -1), Females: max(c.Y, -1), Young: max(c.X2, -1)}
	if cp == (Caps{-1, -1, -1}) {
		delete(au.s.Caps, c.Name)
		return
	}
	if au.s.Caps == nil {
		au.s.Caps = make(map[string]Caps)
	}
	au.s.Caps[c.Name] = cp
}

// Each calls fn for every capped species, in key order (the read model).
func (au *Auto) Each(fn func(key string, c Caps)) {
	if au.s == nil {
		return
	}
	keys := make([]string, 0, len(au.s.Caps))
	for k := range au.s.Caps {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		fn(k, au.s.Caps[k])
	}
}

// class is an animal's auto-slaughter class.
type class uint8

const (
	classMale class = iota
	classFemale
	classYoung
)

// Check marks every capped species' surplus for slaughter (the daily
// check; the caller runs it at CheckTick). It returns how many it marked.
func (au *Auto) Check() int {
	if au.s == nil || len(au.s.Caps) == 0 {
		return 0
	}
	marked := 0
	au.Each(func(key string, c Caps) {
		s, _ := animal.ParseSpecies(key)
		marked += au.cull(s, c)
	})
	return marked
}

// cull marks the surplus of species s over caps c and returns how many.
func (au *Auto) cull(s animal.Species, c Caps) int {
	h := au.h
	now := h.Now()
	var counts [3]int
	var cands [3][]entity.EntityID
	for _, id := range h.AnimalIDs() {
		a, ok := h.Animal(id)
		if !ok || a.Species != s || !a.Tame || a.Designated || a.Dead() {
			continue
		}
		k := classMale
		switch {
		case !grown(a, now):
			k = classYoung
		case a.Female:
			k = classFemale
		}
		counts[k]++
		if a.Bond == 0 && a.Pregnant == 0 {
			cands[k] = append(cands[k], id)
		}
	}
	marked, x, y := 0, 0, 0
	for k, cp := range [3]int{c.Males, c.Females, c.Young} {
		over := counts[k] - cp
		if cp < 0 || over <= 0 {
			continue
		}
		ids := cands[k]
		slices.SortStableFunc(ids, func(x, y entity.EntityID) int { return au.older(x, y) })
		for _, id := range ids[:min(over, len(ids))] {
			a, _ := h.Animal(id)
			a.Designated = true
			h.SetAnimal(id, a)
			if marked++; marked == 1 {
				x, y, _ = h.Pos(id)
			}
		}
	}
	if marked > 0 {
		h.Message(x, y, fmt.Sprintf("Auto-slaughter (%s): %d marked for slaughter.", s.Label(), marked))
	}
	return marked
}

// older orders x before y when x is older (born earlier; Born 0, an adult
// from an older save, is oldest), ties by id. Born ticks may wrap below
// zero (animal.BornAt): ages are compared, not ticks.
func (au *Auto) older(x, y entity.EntityID) int {
	now := au.h.Now()
	ax, _ := au.h.Animal(x)
	ay, _ := au.h.Animal(y)
	dx, dy := age(ax, now), age(ay, now)
	switch {
	case dx > dy:
		return -1
	case dx < dy:
		return 1
	case x < y:
		return -1
	case x > y:
		return 1
	}
	return 0
}

// age is how long animal a has lived at now, in ticks (an adult from an
// older save, Born 0, the most).
func age(a animal.State, now uint64) uint64 {
	if a.Born == 0 {
		return 1 << 63
	}
	return now - a.Born
}
