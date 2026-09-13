package main

import (
	"flag"
	"fmt"
	"log"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	uiadapter "rimworld-go/internal/adapters/inbound/ebiten"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
)

func main() {
	seed := flag.Uint64("seed", 1, "deterministic RNG seed")
	width := flag.Int("width", 20, "map width in tiles")
	height := flag.Int("height", 20, "map height in tiles")
	pawns := flag.Int("pawns", 5, "number of pawns to spawn")
	assets := flag.String("assets", "", "sprite pack directory (default: locate from cwd)")
	flag.Parse()

	app := uiadapter.NewApp(uiadapter.AppConfig{
		Version: "dev",
		Assets:  *assets,
		NewSim: func() (uiadapter.Simulation, error) {
			g := game.New(game.Config{Seed: *seed, Width: *width, Height: *height})
			for i := 0; i < *pawns; i++ {
				x, y := i%*width, (i*7)%*height
				if _, err := g.SpawnPawn(x, y); err != nil {
					return nil, fmt.Errorf("spawn pawn %d: %w", i, err)
				}
			}
			return g, nil
		},
	})

	ebiten.SetTPS(clock.TicksPerSecond)
	ebiten.SetWindowSize(uiadapter.DefaultWindowWidth, uiadapter.DefaultWindowHeight)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetWindowTitle("rimworld-go")
	if err := ebiten.RunGame(app); err != nil {
		log.Fatal(err)
	}
}
