// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package herd

import (
	"rimworld-go/internal/application/game/evlog"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/combat/weapon"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// Life owns animal reproduction, product cadence and birth/hatch
// transitions. It is stateless: the host keeps authoritative animals,
// physical egg stacks, calendar, RNG, positions and event log.
type Life struct{ H LifeHost }

type LifeHost interface {
	LifeRoster
	LifeWorld
	LifeBirth
	LifeEggs
}

type LifeRoster interface {
	Animal(entity.EntityID) (animal.State, bool)
	SetAnimal(entity.EntityID, animal.State)
	Position(entity.EntityID) (pathing.Point, bool)
	NearestAnimal(x, y, r int, keep func(entity.EntityID, animal.State) bool) (entity.EntityID, pathing.Point, bool)
	BeingHandled(entity.EntityID) bool
	GoTo(entity.EntityID, animal.State, int, int) animal.State
}

type LifeWorld interface {
	Calendar() clock.CalendarPolicy
	Now() uint64
	Roll(int) int
	Message(int, int, string, ...any)
	Event(entity.EntityID, uint8, entity.EntityID, int, string)
	Birthday(entity.EntityID, animal.State, uint64) animal.State
	OldAge(entity.EntityID, animal.State, uint64) bool
}

type LifeBirth interface {
	BirthSpot(int, int) (int, int, bool)
	SpawnYoung(animal.Species, int, int, bool, uint32, uint64) (entity.EntityID, bool)
	WildHerdNear(animal.Species, int, int, uint64) uint32
}

type LifeEggs interface {
	LayOne(entity.EntityID, animal.State) animal.State
	Delete(entity.EntityID)
	PutEgg(entity.EntityID, item.Stack)
}

const lifeEvery = 250

func (l Life) grown(a animal.State, now uint64) bool {
	return animal.Adult(a.Species, animal.AgeDays(a.Born, now, a.Species, l.H.Calendar()))
}

func lifeLabel(a animal.State) string {
	if a.Tame && a.Name != "" {
		return a.Name
	}
	return a.Species.Label()
}
func (l Life) Tick(id entity.EntityID, a *animal.State, now uint64, dawnHour uint64) bool {
	// wiki: Animals. Wild animals do not begin mating; a completed
	// fertilization or pregnancy survives a change of ownership.
	if a.Mate != 0 && (!a.Tame || a.Mode != animal.Wander) {
		l.EndMating(id, a) // she lay down, he went to eat, either fled
	}
	if a.Pregnant != 0 && now >= a.Pregnant {
		l.GiveBirth(id, a, now)
	}
	*a = l.H.Birthday(id, *a, now)
	tod := now % clock.TicksPerDay
	if tod == dawnHour*clock.TicksPerHour && l.H.Calendar().BiologicalDayDue(now) && animal.Breeds(a.Species) && l.H.OldAge(id, *a, now) {
		return false
	}
	fill := (now+uint64(id))%lifeEvery == 0
	var daily bool
	switch {
	case !a.Female:
		daily = tod == animal.MateTick(a.Species, uint64(id))
	case animal.Layer(a.Species):
		daily = tod == animal.LayTick(a.Species, uint64(id))
	}
	if !fill && !daily || !animal.Breeds(a.Species) || !l.grown(*a, now) {
		return true
	}
	if fill && a.Product < animal.ProductFull && animal.Produces(a.Species, a.Female, true) {
		per := animal.ProductPerDay(a.Species, a.Food)
		a.Product = min(animal.ProductFull, a.Product+animal.RateSpan(now, per, clock.TicksPerDay, lifeEvery))
	}
	switch {
	case daily && a.Female && (a.Mated > 0 || animal.EcologyOf(a.Species).Eggs == animal.PlainEggs):
		l.LayEgg(id, a, now)
	case daily && a.Tame && !a.Female:
		l.MateCheck(id, a, now)
	}
	return true
}

func (l Life) MateCheck(id entity.EntityID, a *animal.State, now uint64) {
	if a.Female || !a.Tame || a.Mode != animal.Wander || a.Leaving || a.Mate != 0 || a.Food < animal.MateFood || a.Enraged(now) || l.H.BeingHandled(id) {
		return
	}
	pos, _ := l.H.Position(id)
	male := *a
	fid, _, ok := l.H.NearestAnimal(pos.X, pos.Y, animal.MateRadius, func(o entity.EntityID, f animal.State) bool {
		return o != id && l.mateable(f, male, now) && !l.H.BeingHandled(o)
	})
	if !ok || l.H.Roll(100) >= animal.MateChance {
		return
	}
	f, _ := l.H.Animal(fid)
	until := now + animal.MateWalkLimit + animal.MateTicks
	a.Mate, a.MateLeft, a.MateUntil = fid, animal.MateTicks, until
	a.Path, a.PathI, a.NextThink = nil, 0, now
	f.Mate, f.MateLeft, f.MateUntil = id, animal.MateTicks, until
	f.Path, f.PathI = nil, 0
	l.H.SetAnimal(fid, f)
}

func (l Life) mateable(f, m animal.State, now uint64) bool {
	return m.Tame && f.Tame && f.Species == m.Species && f.HasSex && f.Female && l.grown(f, now) &&
		f.Food >= animal.MateFood && f.Mode == animal.Wander && !f.Leaving && f.Mate == 0 &&
		f.Pregnant == 0 && f.Mated == 0 && !f.Enraged(now)
}

func (l Life) Move(id entity.EntityID, a *animal.State, pos pathing.Point, now uint64) bool {
	if a.Mate != 0 {
		return l.MateStep(id, a, pos, now)
	}
	return l.followMother(id, a, pos, now)
}

func (l Life) MateStep(id entity.EntityID, a *animal.State, pos pathing.Point, now uint64) bool {
	p, ok := l.H.Animal(a.Mate)
	if !a.Tame || !ok || !p.Tame || p.Mate != id || p.Mode != animal.Wander || now > a.MateUntil {
		l.EndMating(id, a)
		return false
	}
	if a.Female {
		a.Path, a.PathI = nil, 0
		return true
	}
	pp, _ := l.H.Position(a.Mate)
	if weapon.Distance(pos.X, pos.Y, pp.X, pp.Y) > 1 {
		if a.PathI >= len(a.Path) && now >= a.RetryAt {
			*a = l.H.GoTo(id, *a, pp.X, pp.Y)
		}
		return true
	}
	a.Path, a.PathI = nil, 0
	if a.MateLeft--; a.MateLeft > 0 {
		return true
	}
	fid := a.Mate
	if animal.Layer(p.Species) {
		p.Mated = animal.MatedEggsOf(p.Species)
	} else {
		p.Pregnant = now + animal.GestTicks(p.Species)
	}
	a.Mate, a.MateLeft, a.MateUntil = 0, 0, 0
	p.Mate, p.MateLeft, p.MateUntil = 0, 0, 0
	l.H.SetAnimal(fid, p)
	l.H.Event(id, evlog.Mated, fid, 0, lifeLabel(p))
	l.H.Event(fid, evlog.Mated, id, 0, lifeLabel(*a))
	return true
}

func (l Life) EndMating(id entity.EntityID, a *animal.State) {
	if a.Mate == 0 {
		return
	}
	if p, ok := l.H.Animal(a.Mate); ok && p.Mate == id {
		p.Mate, p.MateLeft, p.MateUntil = 0, 0, 0
		l.H.SetAnimal(a.Mate, p)
	}
	a.Mate, a.MateLeft, a.MateUntil = 0, 0, 0
}

func (l Life) mother(id entity.EntityID, a animal.State, pos pathing.Point, now uint64) (pathing.Point, bool) {
	_, mp, ok := l.H.NearestAnimal(pos.X, pos.Y, animal.MotherRadius, func(o entity.EntityID, m animal.State) bool {
		return o != id && m.Species == a.Species && m.Female && m.Tame == a.Tame &&
			(a.Herd == 0 || m.Herd == a.Herd) && l.grown(m, now)
	})
	return mp, ok
}

func (l Life) followMother(id entity.EntityID, a *animal.State, pos pathing.Point, now uint64) bool {
	if a.PathI < len(a.Path) || now < a.NextThink || l.grown(*a, now) {
		return false
	}
	mp, ok := l.mother(id, *a, pos, now)
	if !ok {
		return false
	}
	a.NextThink = now + 120 + uint64(l.H.Roll(240))
	r := animal.FollowNear
	if weapon.Distance(pos.X, pos.Y, mp.X, mp.Y) > r {
		r = 1 // catch up
	}
	*a = l.H.GoTo(id, *a, mp.X+l.H.Roll(2*r+1)-r, mp.Y+l.H.Roll(2*r+1)-r)
	return true
}

func (l Life) GiveBirth(id entity.EntityID, a *animal.State, now uint64) {
	pos, ok := l.H.Position(id)
	if !ok {
		return
	}
	if _, _, available := l.H.BirthSpot(pos.X, pos.Y); !available {
		return
	}
	if a.Mated == 0 {
		life := animal.LifeOf(a.Species)
		a.Mated = life.LitterMin + l.H.Roll(max(life.LitterMax-life.LitterMin, 0)+1)
	}
	born := 0
	for a.Mated > 0 {
		x, y, ok := l.H.BirthSpot(pos.X, pos.Y)
		if !ok {
			break
		}
		cid, ok := l.H.SpawnYoung(a.Species, x, y, a.Tame, a.Herd, now)
		if !ok {
			break
		}
		if c, found := l.H.Animal(cid); found {
			c.Mother = uint64(id)
			l.H.SetAnimal(cid, c)
		}
		l.H.Event(cid, evlog.Born, id, 0, lifeLabel(*a))
		a.Mated--
		born++
	}
	if a.Mated == 0 {
		a.Pregnant = 0
	}
	if born == 0 {
		return
	}
	l.H.Event(id, evlog.GaveBirth, 0, born, animal.YoungName(a.Species, born))
	if a.Tame {
		l.H.Message(pos.X, pos.Y, "%s gave birth to %s.", a.Name, animal.YoungPhrase(a.Species, born))
	}
}

func (l Life) LayEgg(id entity.EntityID, a *animal.State, now uint64) {
	n := animal.EggsToday(a.Species, uint64(id), int(now/clock.TicksPerDay)) // FOOD-3: the species' wiki interval
	if a.Food < animal.LayFood || a.Leaving {
		return
	}
	for range n {
		*a = l.H.LayOne(id, *a)
	}
}

func (l Life) Hatch(id entity.EntityID, st item.Stack) {
	s := animal.Species(st.Animal)
	now := l.H.Now()
	herd := uint32(0)
	n := 0
	for n < st.Qty {
		x, y, ok := l.H.BirthSpot(st.X, st.Y)
		if !ok {
			break
		}
		if n == 0 && !st.Tame {
			herd = l.H.WildHerdNear(s, st.X, st.Y, now)
		}
		cid, ok := l.H.SpawnYoung(s, x, y, st.Tame, herd, now)
		if !ok {
			break
		}
		l.H.Event(cid, evlog.Hatched, 0, 0, "")
		n++
	}
	switch {
	case n == st.Qty:
		l.H.Delete(id)
	case n > 0:
		st.Qty -= n
		l.H.PutEgg(id, st)
	default:
		l.H.PutEgg(id, st)
	}
	if n == 0 || !st.Tame {
		return
	}
	if n == 1 {
		l.H.Message(st.X, st.Y, "%s hatched.", animal.YoungSentence(s, 1))
		return
	}
	l.H.Message(st.X, st.Y, "%s hatched.", animal.YoungPhrase(s, n))
}
