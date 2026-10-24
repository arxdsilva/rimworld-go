package game

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
)

// A doctor tends a hurt tame animal with medicine, once per TendTicks.
func TestVetTendsHurtTameAnimal(t *testing.T) {
	t.Parallel()
	g, ada, dog := bondYard(t, 30)
	g.positions.Set(dog, Position{X: 12, Y: 6})
	g.SpawnStack(item.HerbalMedicine, 3, 8, 3)
	a, _ := g.animals.Get(dog)
	a.Injury = 20
	g.animals.Set(dog, a)
	tends := 0
	last := job.None
	for i := 0; i < 3000; i++ {
		g.Step(1)
		if i%50 == 0 {
			g.SetNeeds(ada, 1<<20, 1<<20)
		}
		j, _ := g.jobs.Get(ada)
		if j.Kind == job.TendAnimal && last != job.TendAnimal {
			tends++
		}
		last = j.Kind
	}
	a, _ = g.animals.Get(dog)
	if !g.tended(dog) || a.Injury > 20-TendHeal {
		t.Fatalf("tended: %v, injury %d", g.tended(dog), a.Injury)
	}
	if tends != 1 {
		t.Fatalf("one tend per TendTicks, got %d", tends)
	}
	left := 0
	for _, id := range g.itemIDs {
		if st, ok := g.stacks.Get(id); ok && st.Kind == item.HerbalMedicine {
			left += st.Qty
		}
	}
	if left != 2 {
		t.Fatalf("the tend used one dose of medicine (%d left)", left)
	}
}

// Fed tame animals heal on their own on the decision cycle; hungry and
// wild ones do not.
func TestTameAnimalsHealNaturally(t *testing.T) {
	t.Parallel()
	g, ada, dog := bondYard(t, 31)
	g.setPriority(ada, work.Doctor, 0) // no tending: healing on its own
	wild, _ := g.SpawnAnimal("labrador", 18, 10)
	for _, id := range []entity.EntityID{dog, wild} {
		a, _ := g.animals.Get(id)
		a.Injury = 10
		g.animals.Set(id, a)
	}
	for i := 0; i < 6; i++ { // one heal period
		g.Step(animalHealEvery / 6)
		g.SetNeeds(ada, 1<<20, 1<<20)
	}
	d, _ := g.animals.Get(dog)
	w, _ := g.animals.Get(wild)
	if d.Injury != 9 || w.Injury != 10 {
		t.Fatalf("after one period: tame %d (want 9), wild %d (want 10)", d.Injury, w.Injury)
	}
	d.Food = 0
	g.animals.Set(dog, d)
	g.Step(animalHealEvery)
	if d2, _ := g.animals.Get(dog); d2.Injury < 9 {
		t.Fatalf("a starving animal does not heal: %d", d2.Injury)
	}
}
