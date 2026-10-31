package game

import (
	"rimworld-go/internal/application/game/diet"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/sim/entity"
)

// Food poisoning (WS-M; FOOD-4: the wiki's chances, game/diet poison.go):
// a cooked meal carries its chance (item.Stack.Poison, merged stacks
// average it); eating rolls it (raw food and carcasses their flat ones,
// paste none) and a hit gives a day of it (body/cond_food.go): vomiting,
// "sick", bed rest from the major stage until recovering (no flapping).

// mealPoison is the poison chance (permille) of a meal cook makes
// working at (x, y): the cook roll and the room's (or outdoor) roll.
func (g *Game) mealPoison(cook entity.EntityID, _ construction.Kind, x, y int) int16 {
	clean, indoors := craftShop{g}.Cleanliness(x, y) // craft_host.go: the room's wiki cleanliness
	return diet.Permille(diet.MealChance(diet.CookChance(int(g.skillOf(cook, skill.Cooking).Level)), diet.KitchenChance(clean, indoors)))
}

// rollFoodPoison rolls the poison chance (basis points) of what id just
// ate from st (afterEat): a hit gives it food poisoning (once: a second
// bad meal changes nothing while it is still sick).
func (g *Game) rollFoodPoison(id entity.EntityID, st item.Stack) {
	bp := int(st.Poison) * 10
	switch {
	case st.Kind == item.CorpseAnimal:
		bp = diet.CorpseChance
	case st.Kind.Raw():
		bp = diet.RawChance
	}
	if bp <= 0 || g.stream(streamMedical).IntN(10000) >= bp {
		return // a clean meal (or paste): the stream untouched
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
	g.message(pos.X, pos.Y, "%s has food poisoning from something eaten.", g.name(id))
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

// poisoned reports id with food poisoning (the "sick" thought).
func (g *Game) poisoned(id entity.EntityID) bool {
	_, ok := g.bodies[id].Poisoned()
	return ok
}
