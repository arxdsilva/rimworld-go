package ebiten

import (
	"testing"

	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/world/planet"
)

// tinySeed is a seed whose 5% world has valid landing sites, for fast
// tests of the world, landing and character screens.
const tinySeed = "alpha"

// runUntil runs frames until cond holds (failing after a generous cap), so
// a progress modal can step its builder to done.
func runUntil(t *testing.T, a *App, cond func() bool) {
	t.Helper()
	for i := 0; i < 4000; i++ {
		if cond() {
			return
		}
		if a.src == nil {
			fakeOn(a)
		}
		if err := a.Update(); err != nil {
			t.Fatal(err)
		}
	}
	t.Fatalf("condition never held; screen=%s", a.nav.screen())
}

// toWorld opens Create World from the main menu with a tiny world.
func toWorld(t *testing.T, a *App) {
	t.Helper()
	click(t, a, "menu.newgame")
	click(t, a, "wizard.next")
	click(t, a, "wizard.next")
	if a.nav.screen() != screenWorldGen {
		t.Fatalf("screen = %s, want world_gen", a.nav.screen())
	}
	ng := a.draft()
	ng.world.SeedText, ng.world.Coverage = tinySeed, 5
}

// toLanding generates the tiny world and arrives at Select Landing Site.
func toLanding(t *testing.T, a *App) {
	t.Helper()
	toWorld(t, a)
	click(t, a, "world.generate")
	runUntil(t, a, func() bool { return a.nav.screen() == screenLanding })
}

// toPawns picks a random site and arrives at Create Characters.
func toPawns(t *testing.T, a *App) {
	t.Helper()
	toLanding(t, a)
	click(t, a, "land.random")
	click(t, a, "wizard.next")
	if a.nav.screen() != screenPawns {
		t.Fatalf("screen = %s, want pawns (tile %d)", a.nav.screen(), a.draft().tile)
	}
	settleUI(t, a)
}

// tinyPlanet builds the tiny world directly.
func tinyPlanet() *planet.Planet {
	p := defaultTestParams()
	return planet.Build(p)
}

func defaultTestParams() planet.Params {
	return planet.Params{SeedText: tinySeed, Coverage: 5, Rain: 2, Temp: 2, Pop: 1, Factions: faction.DefaultSlots()}
}
