package game

import (
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/filth"
	"rimworld-go/internal/domain/world/room"
)

// Food poisoning (WS-M). Every cooked meal carries a poison chance
// (item.Stack.Poison, permille per meal), set by the cook in doBill: a
// base from its Cooking level, scaled by how clean the kitchen is, with a
// floor for meals cooked over a campfire. Merged stacks average it by
// quantity (item.MergePoison). Eating a meal rolls it once on the medical
// stream (afterEat); a hit gives food poisoning (body/cond_food.go), which
// runs a day by itself: initial, major, recovering. It vomits now and
// then, gives the "sick" thought (feelsSick), and the major stage sends a
// colonist to bed until it is recovering (bed rest: in at major, out at
// recovering, so it never flaps). It never kills. Original tuning
// throughout.

const (
	// poisonUnskilled and poisonMaster are a meal's base poison chance
	// (permille) from a cook at Cooking 0 and at 20; it falls on a curve
	// between them (a quarter of the way up the skill cuts it most).
	poisonUnskilled = 50
	poisonMaster    = 3
	// poisonCampfire is the least poison chance of a campfire meal.
	poisonCampfire = 15
	// Kitchen factors (permille) by the cleanliness of the room the cook
	// works in (filth.Label): clean halves it, filthy triples it, and no
	// room at all (outdoors) makes it half again.
	kitchenClean    = 500
	kitchenSlight   = 1000
	kitchenDirty    = 1800
	kitchenFilthy   = 3000
	kitchenOutdoors = 1500
)

// poisonBase is a meal's poison chance (permille) from a cook at Cooking
// level lvl, before the kitchen.
func poisonBase(lvl int) int {
	d := 20 - min(max(lvl, 0), 20)
	return poisonMaster + (poisonUnskilled-poisonMaster)*d*d/400
}

// kitchenFactor is the poison factor (permille) of cooking at (x, y): the
// cleanliness of the room there, or kitchenOutdoors with no room.
func (g *Game) kitchenFactor(x, y int) int {
	r := g.roomAt(x, y)
	if r == room.Outdoors || r < 0 || r >= len(g.rooms) {
		return kitchenOutdoors
	}
	sum, _ := g.roomFilth(r)
	switch filth.Label(sum / max(1, g.rooms[r].contents.Cells)) {
	case "clean":
		return kitchenClean
	case "slightly dirty":
		return kitchenSlight
	case "dirty":
		return kitchenDirty
	}
	return kitchenFilthy
}

// mealPoison is the poison chance (permille) of a meal cook makes at a
// bench of kind bench, working at (x, y).
func (g *Game) mealPoison(cook entity.EntityID, bench construction.Kind, x, y int) int16 {
	p := poisonBase(int(g.skillOf(cook, skill.Cooking).Level)) * g.kitchenFactor(x, y) / 1000
	if bench == construction.Campfire {
		p = max(p, poisonCampfire)
	}
	return int16(min(max(p, 1), 1000))
}

// dropMeal drops qty meals of kind k with poison chance poison at (x, y),
// merging into a stack there (the chance averaged by quantity).
func (g *Game) dropMeal(k item.Kind, qty, x, y int, poison int16) {
	sid, err := g.SpawnStack(k, qty, x, y)
	if err != nil {
		return
	}
	if st, ok := g.stacks.Get(sid); ok {
		st.Poison = poison
		g.mergeOrSet(sid, st)
	}
}

// rollFoodPoison rolls the poison chance of the meal id just ate from st
// (afterEat): a hit gives it food poisoning (once: a second bad meal
// changes nothing while it is still sick).
func (g *Game) rollFoodPoison(id entity.EntityID, st item.Stack) {
	if st.Poison <= 0 {
		return // raw food, older stacks: no roll, the stream untouched
	}
	if g.stream(streamMedical).IntN(1000) >= int(st.Poison) {
		return
	}
	v, ok := g.vital.Get(id)
	if !ok || v.Status == health.Dead {
		return
	}
	b := g.ensureBody(id, &v)
	if b == nil || !b.Poison(uint64(g.clock.Now())) {
		return
	}
	g.bodyChanged(id, b, &v) // body.go: stats follow, joins the patients
	g.updateDowned(id, b, &v)
	pos, _ := g.positions.Get(id)
	g.message(pos.X, pos.Y, "%s has food poisoning from a badly cooked meal.", g.name(id))
}

// stepFoodPoison advances id's food poisoning to now (hourly, from
// stepDiseases): its stage by time, a vomit now and then on the map, and
// a message once it is over.
func (g *Game) stepFoodPoison(id entity.EntityID, b *body.Body, v *health.Record, now uint64) {
	if b.StepFoodPoisoning(now) {
		if _, still := b.Poisoned(); !still {
			pos, _ := g.positions.Get(id)
			g.message(pos.X, pos.Y, "%s is over the food poisoning.", g.name(id))
		}
		g.bodyChanged(id, b, v) // body.go: stats follow the stage
		g.updateDowned(id, b, v)
	}
	g.foodVomit(id, b)
}

// foodVomit rolls a vomit for id's food poisoning stage: a splash of dirt
// where it stands (none off the map).
func (g *Game) foodVomit(id entity.EntityID, b *body.Body) {
	c, ok := b.Poisoned()
	if !ok {
		return
	}
	every := body.FoodVomitEvery(c.Stage)
	if every <= 0 || g.stream(streamMedical).IntN(int(every)) != 0 {
		return
	}
	if pos, on := g.positions.Get(id); on {
		g.addDirt(pos.Y*g.grid.Width()+pos.X, vomitDirt) // filth.go
	}
}

// foodPoisonRest reports id's food poisoning at its major stage: it goes
// to bed (needsBedRest).
func (g *Game) foodPoisonRest(id entity.EntityID) bool {
	c, ok := g.bodies[id].Poisoned()
	return ok && c.Stage == body.FoodPoisonRestStage
}

// foodPoisonSick reports id's food poisoning not yet recovering: a
// patient in bed stays there until it is (restOver; hysteresis: in at
// major, out at recovering).
func (g *Game) foodPoisonSick(id entity.EntityID) bool {
	c, ok := g.bodies[id].Poisoned()
	return ok && c.Stage >= body.FoodPoisonRestStage
}

// poisoned reports id with food poisoning (the "sick" thought).
func (g *Game) poisoned(id entity.EntityID) bool {
	_, ok := g.bodies[id].Poisoned()
	return ok
}
