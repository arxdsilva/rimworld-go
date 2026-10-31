package diet

import (
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// Food choice by optimality (wiki: Food: the tastiest allowed food,
// weighed against the walk). Each candidate scores optimalBase less its
// distance in tiles, plus what its eat thoughts are worth (moodWorth),
// less desperatePenalty for food a person eats only in need (kibble, a
// carcass), plus rotSoonBonus for food spoiling within rotSoon, less
// travelFood for the food kept for the road (survival meals, pemmican).
// The shape follows vanilla's; the numbers are original tuning (the
// wiki publishes none).
const (
	optimalBase      = 300
	desperatePenalty = 150
	rotSoonBonus     = 12
	rotSoon          = clock.TicksPerDay / 2
	travelFood       = 10
	// A mood point is worth goodPoint tiles of walking when positive; a
	// bad thought costs badBase plus badPoint tiles a point.
	goodPoint = 8
	badBase   = 40
	badPoint  = 6
)

// moodWorth is what a meal's summed thought effect m is worth in tiles.
func moodWorth(m int) int {
	if m >= 0 {
		return m * goodPoint
	}
	return -badBase + m*badPoint
}

// Picker chooses a person's food: its policy menu, whether it is
// desperate (malnourished: anything edible, carcasses too, whatever the
// policy; wiki: Malnutrition) and whether it is a cannibal.
type Picker struct {
	menu      Menu
	desperate bool
	cannibal  bool
}

// Picker is person id's food chooser.
func (b *Book) Picker(id entity.EntityID, desperate, cannibal bool) Picker {
	return Picker{menu: b.Menu(id, false), desperate: desperate, cannibal: cannibal}
}

// Takes reports st is food the person may eat: edible by people (never
// hay) and allowed by its policy, or a fresh carcass while desperate.
func (p Picker) Takes(st *item.Stack) bool {
	if p.desperate && animal.EdibleCorpse(*st) {
		return true
	}
	k := st.Kind
	return k.IsFood() && k != item.Hay && (p.desperate || p.menu.Has(k))
}

// Score is the optimality of eating from st d tiles away (higher first).
func (p Picker) Score(st *item.Stack, d int) int {
	s := optimalBase - d + moodWorth(AteMood(*st, p.cannibal))
	switch st.Kind {
	case item.Kibble, item.CorpseAnimal:
		s -= desperatePenalty
	case item.SurvivalMeal, item.Pemmican:
		s -= travelFood
	}
	if f := st.FreshTicks(); f > 0 && f < rotSoon {
		s += rotSoonBonus
	}
	return s
}

// Allows reports the person's policy allows kind k (a desperate person
// eats anything).
func (p Picker) Allows(k item.Kind) bool { return p.desperate || p.menu.Has(k) }
