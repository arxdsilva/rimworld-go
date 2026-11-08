package devscene

import (
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/world/weather"
)

// lateGame builds the -scenario lategame colony.
func lateGame(g *game.Game) {
	g.SkipTo(30, 7) // just past a 6-hour weather block, so the rain holds
	g.SetWeather(weather.Rain, 0.5)
	g.ClearPlants(12, 4, 47, 24)
	// the compound: stone walls x 26..40, y 5..15, split in three rooms
	for x := 26; x <= 40; x++ {
		for _, y := range []int{5, 15} {
			kind, stuff := "wall", "granite blocks"
			if y == 15 && (x == 28 || x == 33 || x == 38) {
				kind, stuff = "door", "wood" // doors are wood or steel, never stone
			}
			g.BuildNow(kind, stuff, "south", x, y)
		}
	}
	for y := 6; y <= 14; y++ {
		for _, x := range []int{26, 31, 36, 40} {
			kind, stuff := "wall", "granite blocks"
			if y == 10 && (x == 31 || x == 36) {
				kind, stuff = "door", "wood"
			}
			g.BuildNow(kind, stuff, "south", x, y)
		}
	}
	for x := 27; x <= 30; x++ { // bedroom
		g.BuildNow("bed", "wood", "south", x, 6)
	}
	// torch lamps: the compound has no power grid (PLAY-021 lamps need power)
	g.BuildNow("torch lamp", "wood", "south", 27, 13)
	g.FuelCampfire(27, 13)
	g.BuildNow("stove", "wood", "south", 32, 6) // kitchen and dining
	g.BuildNow("table", "wood", "south", 33, 11)
	for _, c := range [][2]int{{32, 11}, {34, 11}, {33, 10}, {33, 12}} {
		g.BuildNow("chair", "wood", "south", c[0], c[1])
	}
	g.BuildNow("torch lamp", "wood", "south", 35, 14)
	g.FuelCampfire(35, 14)
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
	markerPen(g, 41, 4, 47, 12, true)
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
// start on a snowy winter day: a roofed stone room
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
				kind, stuff := "wall", "granite blocks"
				if x == doorX && y == doorY {
					kind, stuff = "door", "wood" // doors are wooden
				}
				g.BuildNow(kind, stuff, "south", x, y)
			}
		}
		for y := y0 + 1; y < y1; y++ {
			for _, x := range []int{x0, x1} {
				kind, stuff := "wall", "granite blocks"
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
