package game

import (
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/needs"
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
// carry, portions while they still fit under full, leftovers dropped
// where it lies. A prisoner's food band follows.
func (h careTreat) Feed(patient, carry entity.EntityID) {
	g := h.g
	st, ok := g.stacks.Get(carry)
	if !ok {
		return
	}
	n, _ := g.need.Get(patient)
	nut := st.Kind.Nutrition()
	for st.Qty > 0 && n.Hunger+nut/2 <= needs.Full {
		st.Qty--
		n.Hunger += nut
	}
	n.Hunger = min(n.Hunger, needs.Full)
	g.need.Set(patient, n)
	if st.Qty <= 0 {
		g.deleteItem(carry)
	} else {
		pp, _ := g.positions.Get(patient)
		st.Carrier, st.X, st.Y = 0, pp.X, pp.Y
		g.mergeOrSet(carry, st)
	}
	g.lastMeal[patient] = st.Kind
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

func (h careMeals) BestFood(id entity.EntityID) (entity.EntityID, int, int) { return h.g.bestFood(id) }

// TakeFood: feeder id at stack food picks up portions for patient's
// hunger (dining.go: portionsFor), split off a bigger stack.
func (h careMeals) TakeFood(id, food, patient entity.EntityID) (entity.EntityID, bool) {
	g := h.g
	st, ok := g.stacks.Get(food)
	if !ok || st.Carrier != 0 || st.Qty <= 0 {
		return 0, false
	}
	n, _ := g.need.Get(patient)
	take := min(st.Qty, portionsFor(n.Hunger, st.Kind))
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
// asleep, whose SleepTick already counts it; rest stays as it is. A pawn
// down from starvation that was fed gets up, or, still incapacitated by
// its body, lies on from its wounds. It returns the needs, stored.
func (g *Game) downedHunger(id entity.EntityID, v *health.Record, now uint64, asleep bool) needs.Pair {
	n, _ := g.need.Get(id)
	if !asleep {
		rest := n.Rest
		n = g.decayNeeds(id, n, now) // skills.go: HungerRate
		n.Rest = rest
		g.need.Set(id, n)
	}
	if v.Cause == health.CauseNoFood && !n.Starving() {
		v.Status, v.Cause = health.Alive, health.CauseNone
		if g.downsNow(id, *v, g.bodies[id]) { // bleed.go
			v.Status, v.Cause = health.Downed, health.CauseInjury
		}
		g.vital.Set(id, *v)
	}
	return n
}
