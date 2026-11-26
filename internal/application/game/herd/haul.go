// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package herd

import (
	"rimworld-go/internal/application/game/store"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/ai/reservation"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/animal/training"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// AnimalHauler shares the physical stack and storage owner. Only ordinary
// ground stockpiles are admitted, so livestock never load machinery or shelves.
type AnimalHauler interface {
	store.Cells
	HaulMotion
	HaulItems
	HaulOffers(entity.EntityID, animal.State) []HaulOffer
}
type HaulItems interface {
	ItemIDs() []entity.EntityID
	Stack(entity.EntityID) (item.Stack, bool)
	Visible(int, int) bool
	Take(entity.EntityID, entity.EntityID, int) (entity.EntityID, bool)
	Deposit(entity.EntityID, item.Stack)
}

type HaulMotion interface {
	Position(entity.EntityID) (int, int)
	Allowed(entity.EntityID, int, int) bool
	GoTo(entity.EntityID, animal.State, int, int) animal.State
}

func stockZone(h AnimalHauler, x, y int) (zone.Zone, bool) {
	zid, ok := h.Zones().At(x, y)
	if !ok {
		return zone.Zone{}, false
	}
	z, ok := h.Zones().Zone(zid)
	return z, ok && z.Kind.Storage()
}
func animalDestination(h AnimalHauler, id entity.EntityID, st item.Stack, x, y int) bool {
	z, ok := stockZone(h, x, y)
	if !ok || z.Kind != zone.KindStockpile && z.Kind != zone.KindDump || !z.AllowsStack(st) || !h.Allowed(id, x, y) || h.Blocked(x, y, false) {
		return false
	}
	top, has := h.First(x, y)
	return !has || item.SameLoad(top, st) && top.Qty+st.Qty <= h.Items().StackLimit(st.Kind)
}
func walkHaul(h AnimalHauler, id entity.EntityID, a *animal.State, x, y int) bool {
	px, py := h.Position(id)
	if px == x && py == y {
		return true
	}
	if a.PathI < len(a.Path) && a.GoalX == x && a.GoalY == y {
		return true
	}
	next := h.GoTo(id, *a, x, y)
	*a = next
	return len(a.Path) > a.PathI && a.GoalX == x && a.GoalY == y
}

// HaulStep moves a trained animal through source reservation, physical
// pickup and destination delivery. Needs, handlers and combat preempt it.
func HaulStep(h AnimalHauler, claims *reservation.Table, id entity.EntityID, a *animal.State, m *training.Maintenance, now uint64) bool {
	if !a.Tame || a.Downed || !TrainLearned(*a, training.Haul, now, *m) || !CanLearn(a.Species, training.Haul) {
		return false
	}
	if m.Task.Source == 0 {
		if (now+uint64(id))%animal.DecideEvery != 0 || now < a.RetryAt {
			return false
		}
		if !startHaul(h, claims, id, a, m, now) {
			return false
		}
	}
	task := m.Task
	px, py := h.Position(id)
	if owner, ok := claims.TileOwner(task.X, task.Y); !ok || owner != id || now >= task.Until {
		return failHaul(h, claims, id, a, m, now)
	}
	if task.Carry == 0 {
		st, ok := h.Stack(task.Source)
		owner, held := claims.EntityOwner(task.Source)
		if !ok || st.Carrier != 0 || st.Forbidden || st.Rotten() || !h.Visible(st.X, st.Y) || st.Qty < task.Qty || !held || owner != id || !h.Allowed(id, st.X, st.Y) {
			return failHaul(h, claims, id, a, m, now)
		}
		if px != st.X || py != st.Y {
			if !walkHaul(h, id, a, st.X, st.Y) {
				return failHaul(h, claims, id, a, m, now)
			}
			return true
		}
		load := st
		load.Qty = task.Qty
		if !animalDestination(h, id, load, task.X, task.Y) {
			return failHaul(h, claims, id, a, m, now)
		}
		carry, ok := h.Take(id, task.Source, task.Qty)
		if !ok {
			return failHaul(h, claims, id, a, m, now)
		}
		m.Task.Carry = carry
		claims.ReleaseEntity(id, task.Source)
		a.Path, a.PathI = nil, 0
		return true
	}
	st, ok := h.Stack(task.Carry)
	if !ok || st.Carrier != id || st.Forbidden || st.Rotten() || !animalDestination(h, id, st, task.X, task.Y) {
		return failHaul(h, claims, id, a, m, now)
	}
	if px != task.X || py != task.Y {
		if !walkHaul(h, id, a, task.X, task.Y) {
			return failHaul(h, claims, id, a, m, now)
		}
		return true
	}
	CancelHaul(h, claims, id, a, m) // physically deposit on the destination tile
	return true
}

func startHaul(h AnimalHauler, claims *reservation.Table, id entity.EntityID, a *animal.State, m *training.Maintenance, now uint64) bool {
	px, py := h.Position(id)
	offers := h.HaulOffers(id, *a)
	// The shared index bounds finalists. Rank by distance with stable
	// source order for ties, then try each at most once.
	var ranked [haulFinalists]HaulOffer
	n := copy(ranked[:], offers)
	for i := range n {
		best := i
		for j := i + 1; j < n; j++ {
			if ranked[j].distance(px, py) < ranked[best].distance(px, py) || ranked[j].distance(px, py) == ranked[best].distance(px, py) && ranked[j].Source < ranked[best].Source {
				best = j
			}
		}
		ranked[i], ranked[best] = ranked[best], ranked[i]
		o := ranked[i]
		st, ok := h.Stack(o.Source)
		if !ok || st.Carrier != 0 || st.Forbidden || st.Rotten() || st.Qty < o.Qty || !h.Visible(st.X, st.Y) || !h.Allowed(id, st.X, st.Y) || claims.Free(o.Source, st.Qty) != st.Qty {
			continue
		}
		load := st
		load.Qty = o.Qty
		if !animalDestination(h, id, load, o.X, o.Y) || !claims.TryEntity(id, o.Source) {
			continue
		}
		if !claims.TryTile(id, o.X, o.Y) {
			claims.ReleaseEntity(id, o.Source)
			continue
		}
		a.Path, a.PathI = nil, 0
		if !walkHaul(h, id, a, st.X, st.Y) {
			claims.ReleaseEntity(id, o.Source)
			claims.ReleaseTile(o.X, o.Y)
			continue
		}
		m.Task = training.HaulTask{Source: o.Source, X: o.X, Y: o.Y, Qty: o.Qty, Until: now + 2*clock.TicksPerHour}
		return true
	}
	return false
}

func failHaul(h AnimalHauler, claims *reservation.Table, id entity.EntityID, a *animal.State, m *training.Maintenance, now uint64) bool {
	CancelHaul(h, claims, id, a, m)
	a.Path, a.PathI = nil, 0
	a.RetryAt = now + animal.DecideEvery*5
	return false
}

// CancelHaul is idempotent, releases only owned claims, and drops an exact
// carried stack at the animal's real position even when interrupted.
func CancelHaul(h AnimalHauler, claims *reservation.Table, id entity.EntityID, a *animal.State, m *training.Maintenance) {
	task := m.Task
	if task.Source == 0 {
		return
	}
	if st, ok := h.Stack(task.Carry); task.Carry != 0 && ok && st.Carrier == id {
		st.X, st.Y = h.Position(id)
		st.Carrier = 0
		h.Deposit(task.Carry, st)
	}
	claims.ReleaseEntity(id, task.Source)
	if owner, ok := claims.TileOwner(task.X, task.Y); ok && owner == id {
		claims.ReleaseTile(task.X, task.Y)
	}
	m.Task = training.HaulTask{}
}

// WalkTarget is the existing ring search for an animal's walkable goal.
func WalkTarget(x, y int, walkable func(int, int) bool) (int, int, bool) {
	if walkable(x, y) {
		return x, y, true
	}
	for r := 1; r <= 3; r++ {
		for dy := -r; dy <= r; dy++ {
			for dx := -r; dx <= r; dx++ {
				if walkable(x+dx, y+dy) {
					return x + dx, y + dy, true
				}
			}
		}
	}
	return x, y, false
}

// AssignPath applies an exact path result to the animal's movement state.
// A failed search clears the obsolete path and uses the existing retry.
func AssignPath(a *animal.State, search *pathing.Searcher, q pathing.Query, px, py, gx, gy int, retry uint64) {
	if px == gx && py == gy {
		a.Path, a.PathI = nil, 0
		return
	}
	path, ok := search.Find(q, px, py, gx, gy)
	if !ok {
		a.Path, a.PathI, a.RetryAt = nil, 0, retry
		return
	}
	a.Path, a.PathI, a.GoalX, a.GoalY = path, 0, gx, gy
}
