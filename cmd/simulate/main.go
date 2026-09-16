package main

import (
	"flag"
	"fmt"

	"rimworld-go/internal/application/game"
)

func main() {
	seed := flag.Uint64("seed", 1, "deterministic RNG seed")
	ticks := flag.Int("ticks", 100, "number of simulation ticks to run")
	width := flag.Int("width", 20, "map width in tiles")
	height := flag.Int("height", 20, "map height in tiles")
	flag.Parse()

	g := game.NewColony(game.Config{Seed: *seed, Width: *width, Height: *height})
	g.Step(*ticks)

	fmt.Printf("tick=%d pawns=%d food=%d wood=%d\n", g.Tick(), len(g.Snapshot()), g.World().Food, g.World().Wood)
	for _, p := range g.Snapshot() {
		fmt.Printf("pawn %s: (%d, %d) job=%s hunger=%d rest=%d %s\n", p.Name, p.X, p.Y, p.Job, p.Hunger, p.Rest, p.Status)
	}
}
