package game

import (
	"rimworld-go/internal/application/game/dose"
	"rimworld-go/internal/application/game/rest"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/gameplay"
	"rimworld-go/internal/domain/pawn/recreation"
	"rimworld-go/internal/domain/sim/entity"
)

// senseJoy is game/sense's JoyRoles (CSE-002, 008).
type senseJoy struct{ g *Game }

func (h senseJoy) Colonist(id entity.EntityID) bool { return careRoster(h).IsColonist(id) }
func (h senseJoy) SeatWanted(id entity.EntityID) bool {
	g := h.g
	if j, _ := g.jobs.Get(id); j.Kind == job.SitDown && j.Active() {
		return true
	}
	r := g.rest.Needs.Of(id)
	return r.Comfort < rest.SeekComfort && uint64(g.clock.Now()) >= r.SeatAt
}
func (h senseJoy) Joy(id entity.EntityID) int                     { return h.g.joy[id] }
func (h senseJoy) SetJob(id entity.EntityID, j job.Record)        { h.g.jobs.Set(id, j) }
func (h senseJoy) StartJob(id entity.EntityID, r job.Record) bool { return h.g.startJob(id, r) }
func (h senseJoy) Seat(id entity.EntityID, x, y int) (int, int, bool) {
	return h.g.besideSpot(id, x, y)
}
func (h senseJoy) Drink(id entity.EntityID) (item.Kind, entity.EntityID, int, int, bool) {
	return doseOf(h.g).DrinkFor(id)
}
func (h senseJoy) DoseJoy(k item.Kind) int { return dose.JoyOf(k) }
func (h senseJoy) Unpack(id entity.EntityID, k item.Kind) (entity.EntityID, bool) {
	return doseHabits(h).Unpack(id, k)
}
func (h senseJoy) TakeOne(id, stack entity.EntityID) (entity.EntityID, bool) {
	return doseOf(h.g).TakeOne(id, stack)
}
func (h senseJoy) HeldJoy(id, stack entity.EntityID) (int, bool) {
	k, ok := doseOf(h.g).Held(id, stack)
	return dose.JoyOf(k), ok
}
func (h senseJoy) Swallow(id, stack entity.EntityID) bool {
	return doseOf(h.g).Swallow(id, id, stack)
}

// doseHabits is game/dose's Habits.
type doseHabits struct{ g *Game }

func (h doseHabits) Mood(id entity.EntityID) (int, int) {
	t := h.g.thoughtLevel(id)
	return h.g.smoothedMood(id, t), t
}
func (h doseHabits) PotentialMood() bool { return senseOf(h.g).Rules.On(gameplay.DrugPotentialMood) }
func (h doseHabits) PackDoses() bool     { return senseOf(h.g).Rules.On(gameplay.AnyDrugFromPack) }
func (h doseHabits) PackHas(id entity.EntityID, k item.Kind) bool {
	_, ok := senseOf(h.g).Gear.Pack(id).Find(k)
	return ok
}
func (h doseHabits) Unpack(id entity.EntityID, k item.Kind) (entity.EntityID, bool) {
	gear := senseOf(h.g).Gear
	entry, ok := gear.Pack(id).Find(k)
	if !ok {
		return 0, false
	}
	return gear.TakeOut(id, entry, 1)
}
func (h doseHabits) Used(id entity.EntityID) { delete(senseOf(h.g).FromPack, id) }
func (h doseHabits) SpawnHeld(id entity.EntityID, st item.Stack) (entity.EntityID, bool) {
	return senseGear(h).SpawnCarried(id, st)
}

// careGifts is game/care's Gifts (CSE-007).
type careGifts struct{ g *Game }

func (h careGifts) GiftFor(visitor, patient entity.EntityID, turn uint64) (item.Kind, entity.EntityID, int, int, bool) {
	if !senseOf(h.g).Rules.On(gameplay.SickGifts) || h.g.joy[patient] >= recreation.Entertained {
		return 0, 0, 0, 0, false
	}
	return doseOf(h.g).GiftFor(visitor, patient, turn)
}
func (h careGifts) Unpack(id entity.EntityID, k item.Kind) (entity.EntityID, bool) {
	return doseHabits(h).Unpack(id, k)
}
func (h careGifts) TakeOne(id, stack entity.EntityID) (entity.EntityID, bool) {
	return doseOf(h.g).TakeOne(id, stack)
}
func (h careGifts) Tolerance(p entity.EntityID, k item.Kind) int { return doseOf(h.g).Tolerance(p, k) }
func (h careGifts) IsDose(stack entity.EntityID) bool {
	st, ok := h.g.stacks.Get(stack)
	return ok && st.Kind.IsDrug()
}
func (h careGifts) GiveDose(feeder, patient, stack entity.EntityID) {
	if doseOf(h.g).Swallow(feeder, patient, stack); h.g.isPawn(patient) {
		careMeals(h).FedBy(patient, feeder)
	}
}
