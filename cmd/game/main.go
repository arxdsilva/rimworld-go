package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"rimworld-go/internal/domain/animal"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	uiadapter "rimworld-go/internal/adapters/inbound/ebiten"
	"rimworld-go/internal/adapters/outbound/ebitenaudio"
	"rimworld-go/internal/adapters/outbound/filesave"
	"rimworld-go/internal/adapters/outbound/settingsfile"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/mentalbreak"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/weather"
)

// version is the build's version, shown on the main menu. Release builds
// set it from the git tag: -ldflags "-X main.version=v1.2.3".
var version = "dev"

func main() {
	seed := flag.Uint64("seed", 1, "deterministic RNG seed")
	width := flag.Int("width", 48, "map width in tiles")
	height := flag.Int("height", 40, "map height in tiles")
	assets := flag.String("assets", "", "sprite pack directory (default: locate from cwd)")
	capture := flag.String("capture", "", "dev: save one colony frame to this PNG and exit")
	captureTicks := flag.Int("capture-ticks", 5400, "dev: simulation ticks to run before -capture")
	captureZoom := flag.Float64("capture-zoom", 1, "dev: camera zoom for -capture")
	scenario := flag.String("scenario", "", "dev: start scenario: pen (a pen with a feeding trough, tame animals and wild ones about) | rescue (a colonist starts badly hurt) | beds (sleeping spots in the other facings) | double (a double bed by the start) | corridor (two colonists meet head-on in a 1-wide corridor) | crowd (every colonist sent into a room with one door) | armory (armed, drafted colonists against a mixed raid) | chores (cutting, mining and tending by the start) | lategame (a grown colony in autumn rain, raid incoming) | rain (a fall storm) | snow (winter snowfall) | burial (a graveyard, a pyre and two bodies) | roofs (winter: a roofed room, an unroofed one, a roofed porch, a yard too big to roof) | sunny (a clear summer day from before dawn) | fence (a fenced field with a gate, deer outside) | forbid (loose items, some forbidden) | zoo (all 61 animal species, tamed and named, in a pen; use -width 80 -height 60)")
	biomeKey := flag.String("biome", "temperate_forest", "dev: biome of the map: "+biomeKeys()+" (also the landing picker's starting choice)")
	hour := flag.Int("hour", -1, "dev: jump to this hour (0-23) of the scenario's day, or the next day's if past")
	captureFrames := flag.Int("capture-frames", 0, "dev: record this many consecutive frames (PATH_NNN.png) instead of one still")
	captureSpeed := flag.Int("capture-speed", 1, "dev: game speed (1-3) while recording -capture-frames")
	captureUI := flag.String("capture-ui", "", "dev: UI state for -capture: picker | wild | trough | inspector | architect | orders | bills | pawn | combat | melee | follow | grow | corridor | crowd | lategame | weather | grass | burial | roofs | roofs-overlay | home | carpet | plant | item | pawnactions | paused-stockpile | move | copy | menu | credits | options | options-graphics | options-gameplay | options-access | zones | paths | log | frame | fence | forbid | zoo")
	captureTimelapse := flag.Int("capture-timelapse", 0, "dev: with -capture-frames, step this many sim ticks per recorded frame (a time-lapse)")
	noAudio := flag.Bool("noaudio", false, "dev: run without sound")
	flag.Parse()

	var shot *uiadapter.Capture
	if *capture != "" {
		shot = &uiadapter.Capture{Path: *capture, Ticks: *captureTicks, Zoom: *captureZoom, UI: *captureUI, Frames: *captureFrames, Speed: *captureSpeed, Timelapse: *captureTimelapse}
	}

	biomeID, ok := biome.Parse(*biomeKey)
	if !ok {
		log.Fatalf("unknown -biome %q (one of: %s)", *biomeKey, biomeKeys())
	}
	seedSet := false
	flag.Visit(func(f *flag.Flag) { seedSet = seedSet || f.Name == "seed" })
	defaultSeed := uint64(0) // the wizard rolls a random seed
	if seedSet || shot != nil {
		defaultSeed = *seed
	}

	cfg := uiadapter.AppConfig{
		DefaultSeed:  defaultSeed,
		DefaultBiome: biomeID,
		Version:      version,
		Assets:       *assets,
		Capture:      shot,
		Saves:        saveHooks(),
		Settings:     settingsHooks(shot != nil),
		Window: func(fullscreen, vsync bool) {
			ebiten.SetFullscreen(fullscreen)
			ebiten.SetVsyncEnabled(vsync)
		},
		NewSim: func(ng uiadapter.NewGame) (uiadapter.Simulation, error) {
			g := game.NewColony(game.Config{Seed: ng.Seed, Width: *width, Height: *height, Biome: ng.Biome})
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
				// a pen with a husky, a cow, a horse and a goat, a wild deer marked for
				// taming, and a manhunter on its way
				cx, cy := *width/2, *height/2
				g.ClearPlants(cx+4, cy-6, cx+12, cy+2)
				g.Enqueue(command.Command{Kind: command.DesignatePen, X: cx + 5, Y: cy - 5, X2: cx + 11, Y2: cy + 1})
				for _, t := range []struct {
					species, name string
					x, y          int
				}{{"husky", "Ash", cx + 7, cy - 3}, {"cow", "Clover", cx + 9, cy - 1}, {"horse", "Maple", cx + 6, cy - 1}, {"goat", "Pip", cx + 10, cy - 4}} {
					if id, err := g.SpawnAnimal(t.species, t.x, t.y); err == nil {
						g.TameAnimal(id, t.name)
					}
				}
				if _, err := g.SpawnAnimal("deer", cx-6, cy-5); err == nil {
					g.Enqueue(command.Command{Kind: command.DesignateTame, X: cx - 6, Y: cy - 5, X2: cx - 6, Y2: cy - 5})
				}
			}
			if *scenario == "pen" {
				// a pen with a feeding trough and tame goat, cow, hen and
				// horse, and wild animals about: the wildlife UI captures
				penScene(g, *width/2, *height/2)
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
			if *scenario == "fence" {
				// a fenced field east of the start with a gate on its south
				// side, a few crops inside and deer grazing outside
				fenceField(g, *width/2, *height/2)
			}
			if *scenario == "forbid" {
				// wood, food and a rifle on the ground, some of it forbidden
				forbidScene(g, *width/2, *height/2)
			}
			if *scenario == "double" {
				g.BuildNow("double bed", "wood", "south", *width/2-1, *height/2+1)
			}
			if *scenario == "lategame" {
				// a grown colony in autumn rain: a stone compound (bedroom,
				// kitchen, workshop), fields, a pen, a sandbag line facing
				// east, seven armed colonists, and a raid on its way
				lateGame(g)
			}
			if *scenario == "burial" {
				// a graveyard west of the start: three dug graves and a
				// funeral pyre, and two settlers who did not make it
				burialGround(g)
			}
			if *scenario == "rain" {
				g.SkipTo(30, 13) // just past a 6-hour weather block, so the storm holds
				g.SetWeather(weather.Storm, 1)
			}
			if *scenario == "snow" {
				g.SkipTo(45, 7) // just past a 6-hour weather block, so the snow holds
				g.SetWeather(weather.Snow, 0.7)
			}
			if *scenario == "armory" {
				// colonists armed with a knife, a spear and a rifle, drafted,
				// against a raid carrying every weapon kind
				kinds := []item.Kind{item.Knife, item.Spear, item.Rifle}
				for i, p := range g.Snapshot() {
					g.Arm(p.ID, kinds[i%len(kinds)])
					g.Enqueue(command.Command{Kind: command.DraftPawn, Pawn: uint64(p.ID), Enabled: true})
				}
				g.RaidArmed(item.Knife, item.Spear, item.Club, item.Rifle)
			}
			if *scenario == "chores" {
				// trees to cut, rock to mine and a hurt colonist to tend,
				// all close to the start
				cx, cy := *width/2, *height/2
				g.Enqueue(command.Command{Kind: command.DesignateCut, X: cx - 9, Y: cy - 6, X2: cx - 3, Y2: cy - 2})
				g.Enqueue(command.Command{Kind: command.DesignateMine, X: cx - 2, Y: cy + 7, X2: cx + 4, Y2: cy + 9})
				if snap := g.Snapshot(); len(snap) > 2 {
					g.Injure(snap[2].ID, 35)
				}
			}
			if *scenario == "corridor" {
				// a 1-wide corridor north of the start; two colonists cross it
				// from opposite ends
				cx, cy := *width/2, *height/2-11
				g.ClearPlants(cx-10, cy-3, cx+10, cy+3)
				for x := cx - 6; x <= cx+6; x++ {
					g.BuildNow("wall", "wood", "south", x, cy-1)
					g.BuildNow("wall", "wood", "south", x, cy+1)
				}
				if snap := g.Snapshot(); len(snap) > 1 {
					a, b := uint64(snap[0].ID), uint64(snap[1].ID)
					g.Enqueue(command.Command{Kind: command.MovePawn, Pawn: a, X: cx - 8, Y: cy})
					g.Enqueue(command.Command{Kind: command.MovePawn, Pawn: a, X: cx + 8, Y: cy, Queue: true})
					g.Enqueue(command.Command{Kind: command.MovePawn, Pawn: b, X: cx + 9, Y: cy})
					g.Enqueue(command.Command{Kind: command.MovePawn, Pawn: b, X: cx - 9, Y: cy, Queue: true})
				}
			}
			if *scenario == "crowd" {
				// a 5x5 room east of the start with one door facing the
				// colony; every colonist is sent inside, so they queue at it
				cx, cy := *width/2+13, *height/2
				g.ClearPlants(cx-5, cy-4, cx+4, cy+4)
				for x := cx - 3; x <= cx+3; x++ {
					g.BuildNow("wall", "wood", "south", x, cy-3)
					g.BuildNow("wall", "wood", "south", x, cy+3)
				}
				for y := cy - 2; y <= cy+2; y++ {
					kind := "wall"
					if y == cy {
						kind = "door"
					}
					g.BuildNow(kind, "wood", "east", cx-3, y)
					g.BuildNow("wall", "wood", "south", cx+3, y)
				}
				for i, p := range g.Snapshot() {
					g.Enqueue(command.Command{Kind: command.MovePawn, Pawn: uint64(p.ID), X: cx + 1, Y: cy - 1 + i%3})
				}
			}
			if *scenario == "rescue" {
				if snap := g.Snapshot(); len(snap) > 1 {
					g.Injure(snap[1].ID, 75)
				}
				cx, cy := *width/2, *height/2
				g.BuildNow("sleeping spot", "wood", "south", cx-4, cy+1)
				g.BuildNow("bed", "wood", "east", cx-4, cy+3)
			}
			if *scenario == "zoo" {
				zoo(g)
			}
			if *scenario == "roofs" {
				roofYard(g, *width/2, *height/2)
			}
			if *scenario == "sunny" {
				g.SkipTo(12, 5) // a summer day, before sunrise
				g.SetWeather(weather.Clear, 0)
				g.HoldWeather()
			}
			if *hour >= 0 {
				now := int(g.Tick())
				day := now / clock.TicksPerDay
				if now%clock.TicksPerDay > *hour*clock.TicksPerHour {
					day++
				}
				g.SkipTo(day, *hour)
			}
			g.RoofRooms() // scenario rooms start roofed (SIM-018)
			return g, nil
		},
	}
	// only a live engine: a nil *Engine in the interface would not be nil
	if snd := openSound(shot != nil || *noAudio || os.Getenv("RIMWORLD_GO_NOAUDIO") == "1"); snd != nil {
		cfg.Sound = snd
	}
	app := uiadapter.NewApp(cfg)

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

// biomeKeys lists the -biome keys, comma separated. The "biomes" capture
// is a script-level loop: for b in temperate_forest boreal_forest ...; do
// go run ./cmd/game -noaudio -biome $b -capture out_$b.png; done
func biomeKeys() string {
	keys := make([]string, 0, biome.NumBiomes)
	for _, d := range biome.All() {
		keys = append(keys, d.Key)
	}
	return strings.Join(keys, ", ")
}

// openSound opens the audio device and the sound list; nil when off
// (captures, -noaudio, RIMWORLD_GO_NOAUDIO=1) or when they can't be
// opened, and the game runs silent.
func openSound(off bool) *ebitenaudio.Engine {
	if off {
		return nil
	}
	root, err := ebitenaudio.LocateAudio()
	if err != nil {
		log.Printf("sound off: %v", err)
		return nil
	}
	snd, err := ebitenaudio.New(root)
	if err != nil {
		log.Printf("sound off: %v", err)
		return nil
	}
	return snd
}

// settingsHooks wires the Options to the per-user settings file. Captures
// get none, so screenshots always use the default settings.
func settingsHooks(capturing bool) uiadapter.SettingsHooks {
	if capturing {
		return uiadapter.SettingsHooks{}
	}
	store, err := settingsfile.Default()
	if err != nil {
		log.Printf("settings not saved: %v", err)
		return uiadapter.SettingsHooks{}
	}
	return uiadapter.SettingsHooks{Load: store.Load, Save: store.Save}
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

// lateGame builds the -scenario lategame colony (cmd only: dev hooks).
func lateGame(g *game.Game) {
	g.SkipTo(30, 7) // just past a 6-hour weather block, so the rain holds
	g.SetWeather(weather.Rain, 0.5)
	g.ClearPlants(12, 4, 47, 24)
	// the compound: stone walls x 26..40, y 5..15, split in three rooms
	for x := 26; x <= 40; x++ {
		for _, y := range []int{5, 15} {
			kind := "wall"
			if y == 15 && (x == 28 || x == 33 || x == 38) {
				kind = "door"
			}
			g.BuildNow(kind, "stone", "south", x, y)
		}
	}
	for y := 6; y <= 14; y++ {
		for _, x := range []int{26, 31, 36, 40} {
			kind := "wall"
			if y == 10 && (x == 31 || x == 36) {
				kind = "door"
			}
			g.BuildNow(kind, "stone", "south", x, y)
		}
	}
	for x := 27; x <= 30; x++ { // bedroom
		g.BuildNow("bed", "wood", "south", x, 6)
	}
	g.BuildNow("lamp", "wood", "south", 27, 13)
	g.BuildNow("stove", "wood", "south", 32, 6) // kitchen and dining
	g.BuildNow("table", "wood", "south", 33, 11)
	for _, c := range [][2]int{{32, 11}, {34, 11}, {33, 10}, {33, 12}} {
		g.BuildNow("chair", "wood", "south", c[0], c[1])
	}
	g.BuildNow("lamp", "wood", "south", 35, 14)
	g.BuildNow("workbench", "wood", "south", 37, 6) // workshop
	g.BuildNow("tailoring bench", "wood", "south", 37, 9)
	g.BuildNow("research bench", "wood", "south", 37, 12)
	// fields west of the compound, at three growth stages
	for i, crop := range []string{"corn", "wheat", "potato"} {
		x0 := 13 + i*4
		g.Enqueue(command.Command{Kind: command.DesignateGrowZone, X: x0, Y: 6, X2: x0 + 2, Y2: 13})
		for y := 6; y <= 13; y++ {
			for x := x0; x <= x0+2; x++ {
				_, _ = g.SpawnCrop(crop, x, y, 35+i*33)
			}
		}
	}
	// stockpile south of the compound
	g.Enqueue(command.Command{Kind: command.DesignateStockpile, X: 27, Y: 17, X2: 31, Y2: 19})
	for i, k := range []item.Kind{item.Wood, item.Stone, item.Meal, item.Berries, item.Leather} {
		_, _ = g.SpawnStack(k, 20+i*5, 27+i, 17)
	}
	// a pen with tame animals east of the compound
	g.Enqueue(command.Command{Kind: command.DesignatePen, X: 42, Y: 5, X2: 46, Y2: 11})
	for i, a := range [][2]string{{"dustbuck", "Clover"}, {"dustbuck", "Moss"}, {"ridgewolf", "Ash"}} {
		if id, err := g.SpawnAnimal(a[0], 43+i, 7+i); err == nil {
			g.TameAnimal(id, a[1])
		}
	}
	// defences facing east, a campfire by the fields
	for y := 15; y <= 22; y++ {
		g.BuildNow("sandbags", "wood", "south", 42, y)
	}
	for _, y := range []int{16, 19, 22} {
		g.BuildNow("spike trap", "wood", "south", 45, y)
	}
	g.BuildNow("campfire", "wood", "south", 20, 16)
	g.FuelCampfire(20, 16)
	g.CompleteResearch()
	// seven colonists, armed
	for i, n := range []string{"Dara", "Ivo", "Lune", "Wren"} {
		spot := [][2]int{{28, 12}, {29, 12}, {30, 12}, {34, 13}}[i]
		_, _ = g.SpawnNamedPawn(spot[0], spot[1], n)
	}
	// four armed colonists hold the line, drafted; the rest keep working
	kinds := []item.Kind{item.Rifle, item.Spear, item.Knife, item.Rifle, item.Club, item.Knife, item.Spear}
	for i, p := range g.Snapshot() {
		g.Arm(p.ID, kinds[i%len(kinds)])
		if i < 4 {
			g.Enqueue(command.Command{Kind: command.DraftPawn, Pawn: uint64(p.ID), Enabled: true})
		}
	}
	g.RaidArmed(item.Knife, item.Spear, item.Rifle)
	burialGround(g)
}

// burialGround builds the -scenario burial set piece (and the late
// game's graveyard): dug graves and a pyre at x 13..18, y 15..19.
func burialGround(g *game.Game) {
	g.ClearPlants(12, 14, 19, 20)
	for _, x := range []int{13, 15, 17} {
		g.BuildNow("grave", "wood", "south", x, 15)
	}
	g.BuildNow("funeral pyre", "wood", "south", 13, 19)
	_, _ = g.SpawnStack(item.Wood, 12, 16, 19)
	for i, n := range []string{"Old Tam", "Sefa"} {
		if id, err := g.SpawnNamedPawn(15+i*2, 18, n); err == nil {
			g.Injure(id, 500)
		}
	}
}

// roofYard builds the -scenario roofs set piece east and west of the
// start on a snowy winter day (cmd only: dev hooks): a roofed stone room
// with a fire, the same room with its roof taken off, an open porch
// roofed by designation, and a walled yard too big to roof itself.
func roofYard(g *game.Game, cx, cy int) {
	g.SkipTo(44, 10)
	g.SetWeather(weather.Snow, 0.6)
	g.HoldWeather()
	g.ClearPlants(cx-21, cy-9, cx+18, cy+7)
	box := func(x0, y0, x1, y1, doorX, doorY int) {
		for x := x0; x <= x1; x++ {
			for _, y := range []int{y0, y1} {
				kind, stuff := "wall", "stone"
				if x == doorX && y == doorY {
					kind, stuff = "door", "wood" // doors are wooden
				}
				g.BuildNow(kind, stuff, "south", x, y)
			}
		}
		for y := y0 + 1; y < y1; y++ {
			for _, x := range []int{x0, x1} {
				kind, stuff := "wall", "stone"
				if x == doorX && y == doorY {
					kind, stuff = "door", "wood" // doors are wooden
				}
				g.BuildNow(kind, stuff, "south", x, y)
			}
		}
	}
	// A: roofed, with a fire and a bed
	box(cx+3, cy-8, cx+9, cy-2, cx+6, cy-2)
	g.BuildNow("campfire", "wood", "south", cx+6, cy-5)
	g.FuelCampfire(cx+6, cy-5)
	g.BuildNow("bed", "wood", "south", cx+4, cy-7)
	// B: the same room, its roof removed
	box(cx+11, cy-8, cx+17, cy-2, cx+14, cy-2)
	g.BuildNow("campfire", "wood", "south", cx+14, cy-5)
	g.FuelCampfire(cx+14, cy-5)
	g.BuildNow("bed", "wood", "south", cx+12, cy-7)
	g.Enqueue(command.Command{Kind: command.DesignateRemoveRoof, X: cx + 12, Y: cy - 7, X2: cx + 16, Y2: cy - 3})
	// C: an open porch south of A, on two posts, roofed by designation
	g.BuildNow("wall", "wood", "south", cx+3, cy+1)
	g.BuildNow("wall", "wood", "south", cx+9, cy+1)
	g.BuildNow("table", "wood", "south", cx+6, cy)
	g.Enqueue(command.Command{Kind: command.DesignateRoof, X: cx + 3, Y: cy - 1, X2: cx + 9, Y2: cy})
	// D: a 13x13 walled yard: too big to roof on its own
	box(cx-21, cy-8, cx-7, cy+6, cx-7, cy-1)
	g.Step(1) // apply the designations before the roofs are completed
}

// zoo pens every animal species, tamed and named after its species, in
// rows of 12 three tiles apart from (4, 3) (cmd only: dev hooks).
func zoo(g *game.Game) {
	const cols, step, x0, y0 = 12, 3, 4, 3
	species := animal.All()
	rows := (len(species) + cols - 1) / cols
	x1, y1 := x0+cols*step, y0+rows*step
	g.ClearPlants(x0-1, y0-1, x1, y1)
	g.Enqueue(command.Command{Kind: command.DesignatePen, X: x0 - 1, Y: y0 - 1, X2: x1, Y2: y1})
	for i, s := range species {
		x, y := x0+(i%cols)*step, y0+(i/cols)*step
		for _, d := range [][2]int{{0, 0}, {1, 0}, {0, 1}, {-1, 0}, {0, -1}, {1, 1}, {-1, 1}, {1, -1}, {-1, -1}} {
			if id, err := g.SpawnAnimal(s.String(), x+d[0], y+d[1]); err == nil { // rock: the next tile
				g.TameAnimal(id, s.Label())
				break
			}
		}
	}
}

// penScene builds the -scenario pen set piece east of the start (cmd
// only: dev hooks): a fenced pen with a feeding trough and four tame
// animals, a marked wild deer, and a few wild animals about.
func penScene(g *game.Game, cx, cy int) {
	g.ClearPlants(cx+3, cy-8, cx+14, cy+3)
	g.Enqueue(command.Command{Kind: command.DesignatePen, X: cx + 4, Y: cy - 6, X2: cx + 11, Y2: cy + 1})
	g.BuildNow("feeding trough", "wood", "south", cx+7, cy-3)
	for _, t := range []struct {
		species, name string
		x, y          int
	}{{"goat", "Pip", cx + 5, cy - 4}, {"cow", "Clover", cx + 9, cy - 1}, {"chicken", "Hen", cx + 6, cy}, {"horse", "Maple", cx + 10, cy - 5}} {
		if id, err := g.SpawnAnimal(t.species, t.x, t.y); err == nil {
			g.TameAnimal(id, t.name)
		}
	}
	for i, sp := range []string{"deer", "deer", "deer", "boar", "timber_wolf", "hare", "hare"} {
		if _, err := g.SpawnAnimal(sp, cx-9+i*2, cy-9+i%3*2); err != nil {
			continue
		}
	}
	g.Step(1)
}

// fenceField builds a wooden fence rectangle with a gate in its south
// side east of the start, and spawns deer outside it.
func fenceField(g *game.Game, cx, cy int) {
	x0, y0, x1, y1 := cx+8, cy-6, cx+16, cy-1
	g.ClearPlants(x0-3, y0-3, x1+3, y1+4)
	for x := x0; x <= x1; x++ {
		g.BuildNow("fence", "wood", "south", x, y0)
		if x == x0+4 {
			g.BuildNow("fence gate", "wood", "south", x, y1)
			continue
		}
		g.BuildNow("fence", "wood", "south", x, y1)
	}
	for y := y0 + 1; y < y1; y++ {
		g.BuildNow("fence", "wood", "south", x0, y)
		g.BuildNow("fence", "wood", "south", x1, y)
	}
	g.Enqueue(command.Command{Kind: command.DesignateGrowZone, X: x0 + 1, Y: y0 + 1, X2: x1 - 1, Y2: y1 - 1})
	for i, p := range [][2]int{{x1 + 2, y0 + 1}, {x1 + 3, y1 - 1}, {x0 + 3, y1 + 2}} {
		_ = i
		_, _ = g.SpawnAnimal("deer", p[0], p[1])
	}
}

// forbidScene drops a few loose stacks near the start and forbids half.
func forbidScene(g *game.Game, cx, cy int) {
	spots := []struct {
		kind item.Kind
		qty  int
	}{{item.Wood, 30}, {item.Meat, 12}, {item.Rifle, 1}, {item.Berries, 15}, {item.Wood, 12}}
	for i, s := range spots {
		id, err := g.SpawnStack(s.kind, s.qty, cx+4+i, cy-5)
		if err == nil && i%2 == 0 {
			g.Enqueue(command.Command{Kind: command.SetForbidden, Target: uint64(id), Enabled: true})
		}
	}
}
