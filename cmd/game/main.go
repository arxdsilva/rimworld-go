package main

import (
	"flag"
	"log"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	uiadapter "rimworld-go/internal/adapters/inbound/ebiten"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/clock"
)

func main() {
	seed := flag.Uint64("seed", 1, "deterministic RNG seed")
	width := flag.Int("width", 48, "map width in tiles")
	height := flag.Int("height", 40, "map height in tiles")
	assets := flag.String("assets", "", "sprite pack directory (default: locate from cwd)")
	capture := flag.String("capture", "", "dev: save one colony frame to this PNG and exit")
	captureTicks := flag.Int("capture-ticks", 5400, "dev: simulation ticks to run before -capture")
	captureZoom := flag.Float64("capture-zoom", 1, "dev: camera zoom for -capture")
	scenario := flag.String("scenario", "", "dev: start scenario: rescue (a colonist starts badly hurt) | beds (sleeping spots in the other facings) | double (a double bed by the start)")
	captureFrames := flag.Int("capture-frames", 0, "dev: record this many consecutive frames (PATH_NNN.png) instead of one still")
	captureUI := flag.String("capture-ui", "", "dev: UI state for -capture: architect | orders | bills | pawn | combat | melee | follow | grow")
	flag.Parse()

	var shot *uiadapter.Capture
	if *capture != "" {
		shot = &uiadapter.Capture{Path: *capture, Ticks: *captureTicks, Zoom: *captureZoom, UI: *captureUI, Frames: *captureFrames}
	}

	app := uiadapter.NewApp(uiadapter.AppConfig{
		Version: "dev",
		Assets:  *assets,
		Capture: shot,
		NewSim: func() (uiadapter.Simulation, error) {
			g := game.NewColony(game.Config{Seed: *seed, Width: *width, Height: *height})
			if *scenario == "beds" {
				cx, cy := *width/2, *height/2
				for i, f := range []string{"east", "west", "north"} {
					for dy := 0; dy < 8 && !g.BuildNow("sleeping spot", "wood", f, cx+6+dy, cy-6+3*i); dy++ {
					}
				}
			}
			if *scenario == "beds" {
				for dy := 0; dy < 6 && !g.BuildNow("double bed", "wood", "south", *width/2+6, *height/2+1+dy); dy++ {
				}
			}
			if *scenario == "winter" {
				// a heated hut and an unheated one on a winter evening
				g.SkipTo(44, 17)
				cx, cy := *width/2, *height/2
				g.ClearPlants(cx+5, cy, cx+18, cy+7)
				for _, h := range [][4]int{{cx + 6, cy + 1, cx + 11, cy + 6}, {cx + 13, cy + 1, cx + 17, cy + 6}} {
					for x := h[0]; x <= h[2]; x++ {
						for _, y := range []int{h[1], h[3]} {
							kind := "wall"
							if y == h[3] && x == (h[0]+h[2])/2 {
								kind = "door"
							}
							g.BuildNow(kind, "wood", "south", x, y)
						}
					}
					for y := h[1] + 1; y < h[3]; y++ {
						g.BuildNow("wall", "wood", "south", h[0], y)
						g.BuildNow("wall", "wood", "south", h[2], y)
					}
				}
				g.BuildNow("campfire", "wood", "south", cx+8, cy+3)
				g.FuelCampfire(cx+8, cy+3)
				g.BuildNow("campfire", "wood", "south", cx+15, cy+3)
				g.BuildNow("tailoring bench", "wood", "south", cx+9, cy+1+1)
				_, _ = g.SpawnStack(item.Parka, 2, cx+3, cy+2)
				_, _ = g.SpawnStack(item.Tuque, 3, cx+4, cy+2)
				_, _ = g.SpawnStack(item.Leather, 20, cx+5, cy+2)
			}
			if *scenario == "drying" {
				cx, cy := *width/2, *height/2
				for dx := 0; dx < 6 && !g.BuildNow("drying rack", "wood", "south", cx+6+dx, cy+2); dx++ {
				}
				for dx := 0; dx < 6 && !g.BuildNow("drying rack", "wood", "south", cx+6+dx, cy+4); dx++ {
				}
				_, _ = g.SpawnStack(item.Meat, 40, cx+5, cy+6)
				_, _ = g.SpawnStack(item.Berries, 20, cx+6, cy+6)
			}
			if *scenario == "double" {
				g.BuildNow("double bed", "wood", "south", *width/2-1, *height/2+1)
			}
			if *scenario == "rescue" {
				if snap := g.Snapshot(); len(snap) > 1 {
					g.Injure(snap[1].ID, 75)
				}
				cx, cy := *width/2, *height/2
				g.BuildNow("sleeping spot", "wood", "south", cx-4, cy+1)
				g.BuildNow("bed", "wood", "east", cx-4, cy+3)
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
	if err := app.CaptureErr(); err != nil {
		log.Fatal(err)
	}
}
