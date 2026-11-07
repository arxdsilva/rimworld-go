package kinds

import (
	"testing"

	"rimworld-go/internal/application/game/quest"
	"rimworld-go/internal/domain/faction"
)

// TestSwarmsHoldNoCampStashOrPrisoner: a hostile faction whose raiders are
// not people (QST-060, QST-050, QST-065) never holds a camp, guards goods
// or keeps a prisoner, and a camp is built to its holder's tech.
func TestSwarmsHoldNoCampStashOrPrisoner(t *testing.T) {
	t.Parallel()
	e := newEnv()
	if got := campTech(rootCtx(e), 3); got != faction.Neolithic {
		t.Fatalf("faction 3's camp tech %v, want neolithic", got)
	}
	if got := quest.TechOf(e.world.factions, 9); got != faction.Industrial {
		t.Fatalf("no faction: tech %v, want industrial", got)
	}
	for i := range e.world.factions {
		e.world.factions[i].Human = e.world.factions[i].Relation != faction.Hostile
	}
	c := rootCtx(e)
	for name, k := range map[string]quest.Kind{"camp": Camp{}, "stash": Stash{}, "rescue": Rescue{}} {
		if k.CanOffer(c) {
			t.Errorf("%s offered with only a swarm hostile", name)
		}
	}
}
