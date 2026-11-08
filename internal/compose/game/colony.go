package game

import (
	"fmt"
	"os"

	uiadapter "rimworld-go/internal/adapters/inbound/ebiten"
	"rimworld-go/internal/application/devscene"
	appgame "rimworld-go/internal/application/game"
)

// newSimBuilder starts a wizard colony for the loading screen.
func newSimBuilder(ng uiadapter.NewGame) (uiadapter.SimBuilder, error) {
	return &colonyBuilder{b: appgame.NewColonyBuilder(appgame.Config{Seed: ng.Seed, Biome: ng.Biome, Setup: ng.Setup})}, nil
}

// colonyBuilder steps a wizard colony for the loading screen (NG-1 §2.10).
type colonyBuilder struct{ b *appgame.ColonyBuilder }

func (c *colonyBuilder) Step(units int) (string, float32, bool) { return c.b.Step(units) }

func (c *colonyBuilder) Sim() (uiadapter.Simulation, error) {
	g := c.b.Game()
	if g == nil {
		return nil, fmt.Errorf("colony not built")
	}
	g.RoofRooms()
	return g, nil
}

// devScenes builds the legacy start's colony: the -scenario set piece on
// a -width x -height map (internal/application/devscene).
type devScenes struct{ cfg devscene.Config }

func (s devScenes) newSim(ng uiadapter.NewGame) (uiadapter.Simulation, error) {
	c := s.cfg
	c.FixtureDir = os.Getenv("RIMWORLD_FIXTURE_DIR") // the grown colony's cache
	g, err := devscene.Build(c, ng.Seed, ng.Biome)
	return g, err
}
