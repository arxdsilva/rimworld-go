package main

import (
	"flag"
	"fmt"
	"log"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	uiadapter "rimworld-go/internal/adapters/inbound/ebiten"
	"rimworld-go/internal/adapters/outbound/filesave"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/mentalbreak"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
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
		Saves:   saveHooks(),
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
			if *scenario == "raid" {
				g.Raid(3)
			}
			if *scenario == "siege" {
				g.Siege(3)
			}
			if *scenario == "sappers" {
				g.Sappers(2)
			}
			if *scenario == "animals" {
				// a pen with two tame animals, a wild dustbuck marked for
				// taming, and a manhunter on its way
				cx, cy := *width/2, *height/2
				g.ClearPlants(cx+4, cy-6, cx+12, cy+2)
				g.Enqueue(command.Command{Kind: command.DesignatePen, X: cx + 5, Y: cy - 5, X2: cx + 11, Y2: cy + 1})
				if id, err := g.SpawnAnimal("ridgewolf", cx+7, cy-3); err == nil {
					g.TameAnimal(id, "Ash")
				}
				if id, err := g.SpawnAnimal("dustbuck", cx+9, cy-1); err == nil {
					g.TameAnimal(id, "Clover")
				}
				if _, err := g.SpawnAnimal("dustbuck", cx-6, cy-5); err == nil {
					g.Enqueue(command.Command{Kind: command.DesignateTame, X: cx - 6, Y: cy - 5, X2: cx - 6, Y2: cy - 5})
				}
			}
			if *scenario == "research" {
				cx, cy := *width/2, *height/2
				g.ClearPlants(cx+4, cy-3, cx+8, cy)
				g.BuildNow("research bench", "wood", "south", cx+5, cy-2)
				g.AddResearch("food_preservation", 2000)
				g.AddResearch("traps", 1600)
				g.PickResearch("traps")
			}
			if *scenario == "medic" {
				// a healroot patch at every stage, medicine in store and
				// Bo down with an infection in a bed nearby
				cx, cy := *width/2, *height/2
				g.ClearPlants(cx-1, cy-11, cx+7, cy-5)
				for i, growth := range []int{50000, 110000, 170000, 240000, 240000, 240000} {
					_, _ = g.SpawnPlantSpecies("healroot", cx+i%3, cy-10+i/3, growth)
				}
				_, _ = g.SpawnStack(item.HerbalMedicine, 6, cx+1, cy-7)
				g.BuildNow("bed", "wood", "south", cx+5, cy-10)
				if snap := g.Snapshot(); len(snap) > 1 {
					g.Infect(snap[1].ID, 45)
				}
			}
			if *scenario == "defense" {
				// a sandbag ring with spike traps outside it, a battered
				// wall, then a raid
				cx, cy := *width/2, *height/2
				g.ClearPlants(cx-8, cy-7, cx+8, cy+7)
				for d := -5; d <= 5; d++ {
					g.BuildNow("sandbags", "wood", "south", cx-6, cy+d)
					g.BuildNow("sandbags", "wood", "south", cx+6, cy+d)
					g.BuildNow("sandbags", "wood", "south", cx+d, cy-6)
				}
				for _, d := range []int{-3, 0, 3} {
					g.BuildNow("spike trap", "wood", "south", cx+8, cy+d)
					g.BuildNow("spike trap", "wood", "south", cx-8, cy+d)
				}
				for d := 2; d <= 4; d++ {
					g.BuildNow("wall", "wood", "south", cx+d, cy+4)
				}
				g.DamageAt(cx+3, cy+4, 90)
				g.Raid(3)
			}
			if *scenario == "fun" {
				// a horseshoe pin by the start, and Bo in a sad wander
				cx, cy := *width/2, *height/2
				g.ClearPlants(cx+4, cy-2, cx+9, cy+4)
				g.BuildNow("horseshoe pin", "wood", "south", cx+6, cy+1)
				if snap := g.Snapshot(); len(snap) > 1 {
					g.ForceBreak(snap[1].ID, mentalbreak.SadWander)
				}
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

// saveHooks wires the UI's save/load to the per-user saves folder.
func saveHooks() uiadapter.SaveHooks {
	store, err := filesave.Default()
	if err != nil {
		log.Printf("saving disabled: %v", err)
		return uiadapter.SaveHooks{}
	}
	return uiadapter.SaveHooks{
		Save: func(sim uiadapter.Simulation, slot string) error {
			g, ok := sim.(*game.Game)
			if !ok {
				return fmt.Errorf("this simulation can't be saved")
			}
			_, err := store.Save(g, slot)
			return err
		},
		List: func() ([]uiadapter.SaveSlot, error) {
			slots, err := store.List()
			out := make([]uiadapter.SaveSlot, 0, len(slots))
			for _, sl := range slots {
				s := uiadapter.SaveSlot{Name: sl.Name, Path: sl.Path, Date: sl.Info.Date,
					Colonists: sl.Info.Colonists, Modified: sl.Modified}
				if sl.Err != nil {
					s.Err = sl.Err.Error()
				}
				out = append(out, s)
			}
			return out, err
		},
		Load: func(path string) (uiadapter.Simulation, string, error) {
			g, info, err := store.Load(path)
			if err != nil {
				return nil, "", err
			}
			return g, info.Name, nil
		},
	}
}
