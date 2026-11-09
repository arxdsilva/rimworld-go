package game

import (
	"fmt"
	"os"

	uiadapter "rimworld-go/internal/adapters/inbound/ebiten"
	"rimworld-go/internal/application/devscene"
	appgame "rimworld-go/internal/application/game"
	"rimworld-go/internal/platform/pool"
)

// newSimBuilder starts wizard colonies for the loading screen, their path
// stages on p (GAR-035).
func newSimBuilder(p *pool.Pool) func(uiadapter.NewGame) (uiadapter.SimBuilder, error) {
	return func(ng uiadapter.NewGame) (uiadapter.SimBuilder, error) {
		return &colonyBuilder{b: appgame.NewColonyBuilder(appgame.Config{Seed: ng.Seed, Biome: ng.Biome, Setup: ng.Setup, Pool: p})}, nil
	}
}

// simPool is the process's simulation lanes: one in a capture, so a
// capture never starts a worker (the answers are the same either way).
func simPool(capture bool) *pool.Pool {
	if capture {
		return pool.New(pool.Options{Lanes: 1})
	}
	return pool.New(pool.Options{})
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
