package game

import (
	"testing"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/sim/entity"
)

// EAI-0, the shared hostility rule: an animal attacking the colony is a
// wild one, or a tame one gone manhunter, in Attack mode on a colonist or
// one of the colony's tame animals; wild prey is no one's concern.
func TestHostileCoversTameAnimalsAndTameManhunters(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 63, Width: 30, Height: 16})
	bareGround(g)
	bo := draftedPawn(t, g, 10, 8)
	wolf, _ := g.SpawnAnimal("timber_wolf", 14, 8)
	dog, _ := g.SpawnAnimal("labrador", 16, 8)
	deer, _ := g.SpawnAnimal("deer", 18, 8)
	d := animalOf(g, dog)
	d.Tame = true
	g.animals.Set(dog, d)

	w := animalOf(g, wolf)
	w.Mode = animal.Attack
	for _, c := range []struct {
		name   string
		target entity.EntityID
		want   bool
	}{{"a colonist", bo, true}, {"a tame dog", dog, true}, {"a wild deer", deer, false}} {
		w.Target = c.target
		if g.hostile(w) != c.want {
			t.Errorf("a wild wolf attacking %s: hostile %v, want %v", c.name, !c.want, c.want)
		}
	}

	d.Mode, d.Target = animal.Attack, bo
	if g.hostile(d) {
		t.Error("a tame dog attacking without rage is not hostile")
	}
	d.Manhunter = uint64(g.clock.Now()) + 100
	if !g.hostile(d) {
		t.Error("a tame dog gone manhunter on a colonist is hostile")
	}
}
