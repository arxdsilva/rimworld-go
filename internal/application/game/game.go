// Package game orchestrates the minimal headless simulation loop: a grid,
// entity allocation, pawn positions, and a deterministic movement step.
package game

import (
	"fmt"

	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/component"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/grid"
	"rimworld-go/pkg/randx"
)

// Position is a pawn's location on the grid.
type Position struct {
	X, Y int
}

// Config configures a new Game.
type Config struct {
	Seed          uint64
	Width, Height int
}

// PawnView is a read-only snapshot of a single pawn.
type PawnView struct {
	ID   entity.EntityID
	X, Y int
}

// Game holds all authoritative simulation state for the minimal slice.
type Game struct {
	clock     *clock.Clock
	grid      *grid.Grid
	entities  *entity.Allocator
	positions *component.Store[Position]
	rng       *randx.Rand
}

// New builds a Game from cfg. Width/Height must be positive.
func New(cfg Config) *Game {
	return &Game{
		clock:     clock.New(),
		grid:      grid.New(cfg.Width, cfg.Height),
		entities:  entity.NewAllocator(),
		positions: component.NewStore[Position](),
		rng:       randx.New(cfg.Seed),
	}
}

// SpawnPawn creates a pawn at (x, y). It returns an error if the tile is
// not walkable.
func (g *Game) SpawnPawn(x, y int) (entity.EntityID, error) {
	if !g.grid.Walkable(x, y) {
		return 0, fmt.Errorf("spawn pawn at (%d, %d): tile not walkable", x, y)
	}
	id := g.entities.Next()
	g.positions.Set(id, Position{X: x, Y: y})
	return id, nil
}

// Tick returns the current simulation tick.
func (g *Game) Tick() clock.Tick {
	return g.clock.Now()
}

// Date returns the calendar date for the current simulation tick.
func (g *Game) Date() clock.Date {
	return g.clock.Date()
}

// Width returns the map width in tiles.
func (g *Game) Width() int { return g.grid.Width() }

// Height returns the map height in tiles.
func (g *Game) Height() int { return g.grid.Height() }

// Step advances the simulation by n ticks. Each tick, every pawn attempts
// one random single-step move; blocked or out-of-bounds moves are skipped.
// Pawns are processed in a fixed (insertion) order for determinism.
func (g *Game) Step(n int) {
	for i := 0; i < n; i++ {
		g.stepOnce()
		g.clock.Advance(1)
	}
}

func (g *Game) stepOnce() {
	deltas := [3]int{-1, 0, 1}

	type move struct {
		id  entity.EntityID
		pos Position
	}
	var moves []move

	g.positions.All(func(id entity.EntityID, pos Position) {
		dx := deltas[g.rng.IntN(len(deltas))]
		dy := deltas[g.rng.IntN(len(deltas))]
		next := Position{X: pos.X + dx, Y: pos.Y + dy}
		if g.grid.Walkable(next.X, next.Y) {
			moves = append(moves, move{id: id, pos: next})
		}
	})

	for _, m := range moves {
		g.positions.Set(m.id, m.pos)
	}
}

// Snapshot returns a read-only view of every pawn, in insertion order.
func (g *Game) Snapshot() []PawnView {
	views := make([]PawnView, 0, g.positions.Len())
	g.positions.All(func(id entity.EntityID, pos Position) {
		views = append(views, PawnView{ID: id, X: pos.X, Y: pos.Y})
	})
	return views
}
