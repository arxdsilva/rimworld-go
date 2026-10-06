package main

import (
	"flag"
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
	assets := flag.String("assets", "", "sprite pack directory (default: locate from cwd)")
	capture := flag.String("capture", "", "dev: save one colony frame to this PNG and exit")
	captureTicks := flag.Int("capture-ticks", 5400, "dev: simulation ticks to run before -capture")
	captureZoom := flag.Float64("capture-zoom", 1, "dev: camera zoom for -capture")
	captureUI := flag.String("capture-ui", "", "dev: UI state for -capture: architect | orders")
	flag.Parse()

	var shot *uiadapter.Capture
	if *capture != "" {
		shot = &uiadapter.Capture{Path: *capture, Ticks: *captureTicks, Zoom: *captureZoom, UI: *captureUI}
	}

	app := uiadapter.NewApp(uiadapter.AppConfig{
		Version: "dev",
		Assets:  *assets,
		Capture: shot,
		NewSim: func() (uiadapter.Simulation, error) {
			g := game.NewColony(game.Config{Seed: *seed, Width: *width, Height: *height})
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
	if err := app.CaptureErr(); err != nil {
		log.Fatal(err)
	}
}
