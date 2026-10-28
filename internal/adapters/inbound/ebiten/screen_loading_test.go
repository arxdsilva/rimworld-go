package ebiten

import (
	"errors"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/difficulty"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/world/biome"
)

func TestStartBuildsSetupFromDraft(t *testing.T) {
	t.Parallel()
	var got []NewGame
	a := appWith(&stubSim{w: 4, h: 4}, func(c *AppConfig) { c.Scenarios = newMemStore() }, &got)
	a.finishBoot()
	f := fakeOn(a)
	f.Cur.Mx, f.Cur.My = -50, -50
	click(t, a, "menu.newgame")
	click(t, a, "scen.row.ember_clan")
	click(t, a, "wizard.next")
	click(t, a, "diff.seasoned")
	click(t, a, "save.commit")
	click(t, a, "wizard.next")
	a.draft().world.SeedText, a.draft().world.Coverage = tinySeed, 5
	click(t, a, "world.generate")
	runUntil(t, a, func() bool { return a.nav.screen() == screenLanding })
	click(t, a, "land.random")
	click(t, a, "land.advanced")
	f.Frame(t, a)
	press(t, a, f, "land.season.4") // Winter
	f.Frame(t, a)
	press(t, a, f, "modal.land.advanced.close")
	f.Frame(t, a)
	ng := a.ng
	tile, plt := ng.tile, ng.planet
	click(t, a, "wizard.next")
	f.Frame(t, a)
	chosen := ng.pool.Selected()
	click(t, a, "wizard.start")
	if len(got) != 1 || got[0].Setup == nil {
		t.Fatalf("NewSim calls: %+v", got)
	}
	s := got[0].Setup
	if s.Scenario.Key != "ember_clan" || len(s.Colonists) != s.Scenario.People {
		t.Errorf("scenario %q with %d colonists", s.Scenario.Key, len(s.Colonists))
	}
	if s.Storyteller != defaultStoryteller || s.Preset != difficulty.Seasoned || s.Difficulty != difficulty.Of(difficulty.Seasoned) {
		t.Errorf("story %q preset %v", s.Storyteller, s.Preset)
	}
	if s.SaveMode != difficulty.Commitment {
		t.Errorf("save mode %v", s.SaveMode)
	}
	if s.World.SeedText != tinySeed || s.World.Coverage != 5 || s.Tile != tile {
		t.Errorf("world %+v tile %d", s.World, s.Tile)
	}
	if s.Site.Biome != plt.Biome[tile] || s.Site.Latitude != plt.Lat[tile] || got[0].Biome != s.Site.Biome {
		t.Errorf("site %+v, NewGame biome %v", s.Site, got[0].Biome)
	}
	if len(s.Factions) != len(plt.Factions) || len(s.Factions) == 0 {
		t.Errorf("%d factions, planet has %d", len(s.Factions), len(plt.Factions))
	}
	if s.MapSize != defaultMapSideEnabled() || s.Season != game.SeasonWinter {
		t.Errorf("map %d season %v", s.MapSize, s.Season)
	}
	if len(s.Colonists) != len(chosen) || s.Colonists[0].Nick != chosen[0].Nick {
		t.Errorf("colonists %v, selected %v", s.Colonists, chosen)
	}
	if want := fnv64a(tinySeed) ^ uint64(int64(tile))*0x9E3779B97F4A7C15 ^ s.Scenario.Seed; s.Seed != want || got[0].Seed != want {
		t.Errorf("seed %x, want %x", s.Seed, want)
	}
	if a.ng != nil || a.nav.screen() != screenColony || a.renderer == nil {
		t.Fatal("starting ends the wizard in the colony")
	}
	// The setup owns its factions: editing them leaves the planet alone.
	s.Factions[0].Name = "Changed"
	if plt.Factions[0].Name == "Changed" {
		t.Error("the setup's factions are a copy")
	}
	_ = faction.Player
	_ = biome.NumBiomes
}

func TestStartNeedsATileAndAPool(t *testing.T) {
	t.Parallel()
	ng := &newGameState{tile: -1}
	if _, ok := ng.buildSetup(); ok {
		t.Fatal("no planet, no setup")
	}
	if r := ng.startReason(); r == "" {
		t.Fatal("no pool: Start is disabled")
	}
}

// stepBuilder is a colony builder that needs several frames.
type stepBuilder struct {
	steps, need int
	err         error
	sim         Simulation
}

func (b *stepBuilder) Step(units int) (string, float32, bool) {
	b.steps++
	return "", float32(b.steps) / float32(b.need), b.steps >= b.need
}
func (b *stepBuilder) Sim() (Simulation, error) { return b.sim, b.err }

func TestColonyBuilderStepsToDone(t *testing.T) {
	t.Parallel()
	a, f, _ := ngTest(t, nil)
	toPawns(t, a)
	sb := &stepBuilder{need: 6, sim: &stubSim{w: 4, h: 4}}
	a.ng.buildHook = func(NewGame) (SimBuilder, error) { return sb, nil }
	f.Frame(t, a)
	click(t, a, "wizard.start")
	if a.nav.screen() != screenPawns || a.ng == nil || a.ng.load == nil {
		t.Fatalf("a builder that is not done keeps the loading screen: screen=%s", a.nav.screen())
	}
	img := ebiten.NewImage(a.sw, a.sh)
	var phases []string
	for i := 0; i < 20 && a.nav.screen() != screenColony; i++ {
		f.Frame(t, a)
		a.Draw(img)
		if a.ng != nil && a.ng.load != nil {
			phases = append(phases, a.ng.load.phase)
		}
	}
	if a.nav.screen() != screenColony || a.renderer == nil || a.ng != nil {
		t.Fatal("the colony starts when the builder is done")
	}
	if sb.steps != 6 {
		t.Fatalf("stepped %d times, want 6", sb.steps)
	}
	if len(phases) == 0 || phases[0] == "" {
		t.Fatalf("phases %v: an unnamed phase is named from the progress", phases)
	}
	// Esc or Back does nothing while the colony is built.
	a2, f2, _ := ngTest(t, nil)
	toPawns(t, a2)
	a2.ng.buildHook = func(NewGame) (SimBuilder, error) { return &stepBuilder{need: 50, sim: &stubSim{w: 4, h: 4}}, nil }
	click(t, a2, "wizard.start")
	f2.Key(t, a2, "Escape", 0)
	if a2.nav.screen() != screenPawns || a2.ng.load == nil {
		t.Fatalf("Esc while loading: screen=%s", a2.nav.screen())
	}
}

func TestStartFailureReturnsToCharacters(t *testing.T) {
	t.Parallel()
	a, f, _ := ngTest(t, nil)
	toPawns(t, a)
	a.ng.buildHook = func(NewGame) (SimBuilder, error) { return &stepBuilder{need: 1, err: errors.New("disk full")}, nil }
	click(t, a, "wizard.start")
	f.Frame(t, a)
	if a.nav.screen() != screenPawns || a.ng == nil || a.ng.load != nil || a.sessionErr != "disk full" {
		t.Fatalf("screen=%s err=%q", a.nav.screen(), a.sessionErr)
	}
	a.Draw(ebiten.NewImage(a.sw, a.sh)) // shows the failure
	a2 := appWith(&stubSim{}, func(c *AppConfig) { c.NewSim = nil }, nil)
	a2.finishBoot()
	a2.startGame()
	if a2.sessionErr != "no simulation factory" {
		t.Fatalf("err %q", a2.sessionErr)
	}
}

func TestLegacyStartGameKeepsSeedAndBiome(t *testing.T) {
	t.Parallel()
	var got []NewGame
	d, _ := biome.Parse("desert")
	a := appWith(&stubSim{w: 4, h: 4}, func(c *AppConfig) { c.DefaultSeed, c.DefaultBiome = 7, d }, &got)
	a.finishBoot()
	a.startGame()
	if len(got) != 1 || got[0].Seed != 7 || got[0].Biome != d || got[0].Setup != nil {
		t.Fatalf("got %+v", got)
	}
}
