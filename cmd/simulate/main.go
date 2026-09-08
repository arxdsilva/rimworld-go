package main

import (
	"flag"
	"fmt"
	"os"

	"rimworld-go/internal/application/game"
)

func main() {
	seed := flag.Uint64("seed", 1, "deterministic RNG seed")
	ticks := flag.Int("ticks", 100, "number of simulation ticks to run")
	width := flag.Int("width", 20, "map width in tiles")
	height := flag.Int("height", 20, "map height in tiles")
	pawns := flag.Int("pawns", 3, "number of pawns to spawn")
	flag.Parse()

	g := game.New(game.Config{Seed: *seed, Width: *width, Height: *height})

	for i := 0; i < *pawns; i++ {
		x, y := i%*width, (i*7)%*height
		if _, err := g.SpawnPawn(x, y); err != nil {
			fmt.Fprintf(os.Stderr, "spawn pawn %d: %v\n", i, err)
			os.Exit(1)
		}
	}

	g.Step(*ticks)

	fmt.Printf("tick=%d pawns=%d\n", g.Tick(), *pawns)
	for _, p := range g.Snapshot() {
		fmt.Printf("pawn %d: (%d, %d)\n", p.ID, p.X, p.Y)
	}
}
