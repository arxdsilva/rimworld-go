package care

import (
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/entity"
)

// fakeMeals is care.Meals for the feeding tests: one food stack (0:
// none) on a tile, hunger by pawn (needs.Full when unset), and a record
// of what feeding asked it to do.
type fakeMeals struct {
	h        *fakeHost
	hunger   map[entity.EntityID]int
	food     entity.EntityID
	fx, fy   int
	takeFail bool
	looks    int // BestFood calls
	log      []string
}

func newFakeMeals(h *fakeHost) *fakeMeals {
	return &fakeMeals{h: h, hunger: map[entity.EntityID]int{}}
}

func (m *fakeMeals) Hunger(id entity.EntityID) int {
	if v, ok := m.hunger[id]; ok {
		return v
	}
	return needs.Full
}
func (m *fakeMeals) BestFood(id, _ entity.EntityID) (entity.EntityID, int, int) {
	m.looks++
	return m.food, m.fx, m.fy
}
func (m *fakeMeals) TakeFood(id, food, patient entity.EntityID) (entity.EntityID, bool) {
	if m.takeFail || food != m.food {
		return 0, false
	}
	m.log = append(m.log, "take")
	return food, true
}
func (m *fakeMeals) FedBy(patient, feeder entity.EntityID) { m.log = append(m.log, "fed by") }

// feedScene: doctor doc on (2, 5), downed colonist patient on (5, 5) at
// Hungry, food 50 on (1, 5).
func feedScene() (*Care, *fakeHost, *fakeMeals) {
	h := newFakeHost()
	h.pawn(doc, 2, 5)
	h.pawn(patient, 5, 5)
	h.vital[patient] = health.Record{Status: health.Downed, Cause: health.CauseInjury, Injury: 50}
	h.doctors[doc] = true
	m := newFakeMeals(h)
	m.hunger[patient] = FeedHungry
	m.food, m.fx, m.fy = 50, 1, 5
	return New(h, Roles{Prison: newFakePrison(h), Meals: m}), h, m
}
