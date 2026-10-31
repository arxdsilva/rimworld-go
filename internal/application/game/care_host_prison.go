package game

import (
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// carePrison is care.Prison (care/deps.go): what capture and arrest need
// from the simulation (SIM-062). Prisoner state, the raid roster and the
// prison RNG stream stay here.
type carePrison struct{ g *Game }

func (h carePrison) IsRaider(id entity.EntityID) bool     { return h.g.isRaider(id) }
func (h carePrison) IsVisitor(id entity.EntityID) bool    { return h.g.visitor(id) }
func (h carePrison) PersonName(id entity.EntityID) string { return h.g.personName(id) }
func (h carePrison) PrisonBedFor(target entity.EntityID, x, y int) (entity.EntityID, bool) {
	return h.g.prisonBedFor(target, Position{X: x, Y: y})
}
func (h carePrison) DropGear(target entity.EntityID) { h.g.dropRaiderGear(target) }
func (h carePrison) MakePrisoner(target, captor entity.EntityID, arrested bool) {
	if arrested && h.g.visitor(target) {
		if f := h.g.factionOf(h.g.personOf(target).Faction); f != nil {
			faction.Adjust(f, -arrestGoodwill, "visitor arrested")
		}
	}
	h.g.makePrisoner(target, captor)
}
func (h carePrison) ArrestRoll(arrester, target entity.EntityID) bool {
	return h.g.stream(streamPrison).IntN(1000) < h.g.arrestChance(arrester, target)
}
func (h carePrison) StartFight(starter, other entity.EntityID) { h.g.mind.StartFight(starter, other) }

// Feed is care.Treatment's (SIM-064): patient eats the carried stack
// carry, the portions that fill it best (the excess wasted), leftovers
// dropped where it lies. A prisoner's food band follows.
func (h careTreat) Feed(patient, carry entity.EntityID) {
	g := h.g
	st, ok := g.stacks.Get(carry)
	if !ok {
		return
	}
	n, _ := g.need.Get(patient)
	eat := min(st.Qty, portionsFor(n.Hunger, st)) // dining.go
	st.Qty -= eat
	n.Hunger = min(n.Hunger+eat*animal.StackNutrition(st), needs.Full)
	g.need.Set(patient, n)
	if st.Qty <= 0 {
		g.deleteItem(carry)
	} else {
		pp, _ := g.positions.Get(patient)
		st.Carrier, st.X, st.Y = 0, pp.X, pp.Y
		g.mergeOrSet(carry, st)
	}
	g.lastMeal[patient] = st.Kind
	g.afterEat(patient, st, true) // rooms.go: the meal's thoughts and food poisoning (FOOD-4)
	g.ateAt[patient] = true
	g.logEvent(patient, logEntry{Kind: logAte, Name: item.Kind(st.Kind).String()})
	if g.prison.Is(patient) {
		g.prison.FoodBand(patient, n)
	}
}

// careMeals implements care.Meals (care/feed.go, SIM-064) over the game.
type careMeals struct{ g *Game }

func (h careMeals) Hunger(id entity.EntityID) int {
	if n, ok := h.g.need.Get(id); ok {
		return n.Hunger
	}
	return needs.Full
}

// BestFood: by the patient's policy and taste, a paste dispenser too (FOOD-4, food.go).
func (h careMeals) BestFood(id, patient entity.EntityID) (entity.EntityID, int, int) {
	food, x, y, _ := mealFor(h.g, id, patient)
	return food, x, y
}

// TakeFood: feeder id at stack food picks up portions for patient's
// hunger (dining.go: portionsFor), split off a bigger stack; at a paste
// dispenser it takes a fresh meal in hand (store/paste.go, FOOD-4).
func (h careMeals) TakeFood(id, food, patient entity.EntityID) (entity.EntityID, bool) {
	g := h.g
	if _, isB := g.bldgs.Get(food); isB {
		g.reserve.ReleaseEntity(id, food) // the next colonist may use the dispenser
		meal, ok := g.haul.store.Paste.Dispense(haulHost{g}, haulHost{g}, food)
		if !ok {
			return 0, false
		}
		pos, _ := g.positions.Get(id)
		meal.X, meal.Y, meal.Carrier, food = pos.X, pos.Y, id, g.entities.Next()
		g.itemIDs = append(g.itemIDs, food)
		g.putStack(food, meal)
		return food, true
	}
	st, ok := g.stacks.Get(food)
	if !ok || st.Carrier != 0 || st.Qty <= 0 {
		return 0, false
	}
	n, _ := g.need.Get(patient)
	take := min(st.Qty, portionsFor(n.Hunger, st))
	if st.Qty <= take {
		st.Carrier = id
		g.putStack(food, st)
		return food, true
	}
	st.Qty -= take
	g.putStack(food, st)
	carry := g.entities.Next()
	g.itemIDs = append(g.itemIDs, carry)
	st.Qty, st.Carrier = take, id
	g.putStack(carry, st)
	return carry, true
}

func (h careMeals) FedBy(patient, feeder entity.EntityID) {
	h.g.logPair(patient, feeder, logFedBy, logFed, 0, 0)
}

// downedHunger is one tick of a downed pawn's food (SIM-064): it grows
// hungry at the awake rate (the wiki publishes no downed rate) unless
// asleep, whose SleepTick already counts it; rest stays as it is. It
// returns the needs, stored.
func (g *Game) downedHunger(id entity.EntityID, now uint64, asleep bool) needs.Pair {
	n, _ := g.need.Get(id)
	if !asleep {
		rest := n.Rest
		n = g.decayNeeds(id, n, now) // skills.go: HungerRate
		n.Rest = rest
		g.need.Set(id, n)
	}
	return n
}

// migrateStarvation (save v7, FOOD-1) gets pawn id up if an older save has
// it down from hunger, unless its body still downs it (bleed.go).
func migrateStarvation(g *Game, id entity.EntityID) {
	if v, _ := g.vital.Get(id); v.Status == health.Downed && v.Cause == health.CauseNoFood {
		v.Status, v.Cause = health.Alive, health.CauseNone
		g.vital.Set(id, v)
		g.updateDowned(id, g.bodies[id], &v)
	}
}

// stepMalnutrition is an hour of malnutrition (FOOD-1,
// body/cond_malnutrition.go) on every pawn with needs that is at 0 % food
// or still malnourished, in store order: it sets in, rises, recedes once
// fed and kills at full severity (the old down-and-die rule is gone).
func stepMalnutrition(g *Game, now uint64) {
	var ids []entity.EntityID
	g.need.All(func(id entity.EntityID, n needs.Pair) {
		if n.Starving() || body.Malnutrition(g.bodies[id]) > 0 {
			ids = append(ids, id)
		}
	})
	for _, id := range ids {
		v, ok := g.vital.Get(id)
		b := g.ensureBody(id, &v)
		if !ok || v.Status == health.Dead || b == nil {
			continue
		}
		changed, dead := body.StepMalnutrition(b, g.need.Ref(id).Starving(), now/clock.TicksPerHour, g.stream(streamMedical).IntN)
		if dead {
			pos, _ := g.positions.Get(id)
			g.message(pos.X, pos.Y, "%s starved to death.", g.name(id))
			g.dieMedical(id, health.CauseNoFood) // caravan_medical.go
			g.clearPatient(id)
		} else if changed {
			g.bodyChanged(id, b, &v) // body.go: stats follow, joins the patients
			g.updateDowned(id, b, &v)
		}
	}
}
