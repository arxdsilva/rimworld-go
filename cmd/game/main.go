package main

import (
	"flag"
	"log"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	uiadapter "rimworld-go/internal/adapters/inbound/ebiten"
	"rimworld-go/internal/application/game"
)

func main() {
	seed := flag.Uint64("seed", 1, "deterministic RNG seed")
	width := flag.Int("width", 20, "map width in tiles")
	height := flag.Int("height", 20, "map height in tiles")
	pawns := flag.Int("pawns", 5, "number of pawns to spawn")
	assets := flag.String("assets", "", "sprite pack directory (default: locate from cwd)")
	flag.Parse()

	g := game.New(game.Config{Seed: *seed, Width: *width, Height: *height})
	for i := 0; i < *pawns; i++ {
		x, y := i%*width, (i*7)%*height
		if _, err := g.SpawnPawn(x, y); err != nil {
			log.Fatalf("spawn pawn %d: %v", i, err)
		}
	}

	renderer := uiadapter.New(g)
	var loadErr error
	if *assets != "" {
		loadErr = renderer.LoadPack(*assets)
	} else {
		loadErr = renderer.LoadDefaultPack()
	}
	if loadErr != nil {
		log.Printf("sprite pack: %v (drawing placeholders)", loadErr)
	}

	ebiten.SetWindowSize(*width*uiadapter.TileSize, *height*uiadapter.TileSize)
	ebiten.SetWindowTitle("rimworld-go")
	if err := ebiten.RunGame(renderer); err != nil {
		log.Fatal(err)
	}
}
