package devscene

import (
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/mentalbreak"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/world/weather"
)

// scene places a set piece on a fresh colony of w x h tiles.
type scene func(g *game.Game, w, h int)

// centred adapts a set piece built around the map's centre.
func centred(f func(g *game.Game, cx, cy int)) scene {
	return func(g *game.Game, w, h int) { f(g, w/2, h/2) }
}

// whole adapts a set piece at fixed tiles.
func whole(f func(g *game.Game)) scene {
	return func(g *game.Game, _, _ int) { f(g) }
}

// scenes is the -scenario table (cmd/game's flag help lists the ones meant
// for media). An immutable lookup table: never assigned after declaration.
var scenes = map[string]scene{
	"beds":    beds,
	"winter":  centred(winterHuts),
	"raid":    whole(func(g *game.Game) { g.Raid(3) }),
	"siege":   whole(func(g *game.Game) { g.Siege(3) }),
	"sappers": whole(func(g *game.Game) { g.Sappers(2) }),
	// a pen with a husky, a cow, a horse and a goat, a wild deer marked
	// for taming, and a manhunter on its way
	"animals": centred(animalPen),
	// a pen with a feeding trough and tame goat, cow, hen and horse, and
	// wild animals about: the wildlife UI captures
	"pen":      centred(penScene),
	"research": centred(researchBench),
	// a healroot patch at every stage, medicine in store and Bo down with
	// an infection in a bed nearby
	"medic": centred(medic),
	// a sandbag ring with spike traps outside it, a battered wall, then a
	// raid
	"defense": centred(defense),
	// a horseshoe pin by the start, and Bo in a sad wander
	"fun":    centred(fun),
	"drying": centred(drying),
	// a fenced field east of the start with a gate on its south side, a
	// few crops inside and deer grazing outside
	"fence": centred(fenceField),
	// wood, food and a rifle on the ground, some of it forbidden
	"forbid": centred(forbidScene),
	// the forbidden stacks, a downed colonist and a bed: -capture-ui marks
	// drafts, strip-marks and deconstruct-marks on it (GAR-161)
	"marks": centred(func(g *game.Game, cx, cy int) {
		forbidScene(g, cx, cy)
		rescue(g, cx, cy)
	}),
	"double": func(g *game.Game, w, h int) {
		g.BuildNow("double bed", "wood", "south", w/2-1, h/2+1)
	},
	// a grown colony in autumn rain: a stone compound (bedroom, kitchen,
	// workshop), fields, a pen, a sandbag line facing east, seven armed
	// colonists, and a raid on its way
	"lategame": whole(lateGame),
	// a small powered base east of the start (PLAY-021): generators, a
	// battery, a switch, lamps, a heater and an electric stove, a hidden
	// run, a lamp off the grid and a torch lamp
	"power": centred(powerBase),
	// a graveyard west of the start: three dug graves and a funeral pyre,
	// and two settlers who did not make it
	"burial": whole(burialGround),
	"rain": whole(func(g *game.Game) {
		g.SkipTo(30, 13) // just past a 6-hour weather block, so the storm holds
		g.SetWeather(weather.Storm, 1)
	}),
	"snow": whole(func(g *game.Game) {
		g.SkipTo(45, 7) // just past a 6-hour weather block, so the snow holds
		g.SetWeather(weather.Snow, 0.7)
	}),
	// colonists armed with a knife, a spear and a rifle, drafted, against
	// a raid carrying every weapon kind
	"armory": whole(armory),
	// trees to cut, rock to mine and a hurt colonist to tend, all close to
	// the start
	"chores": centred(chores),
	// a 1-wide corridor north of the start; two colonists cross it from
	// opposite ends
	"corridor": func(g *game.Game, w, h int) { corridor(g, w/2, h/2-11) },
	// a 5x5 room east of the start with one door facing the colony; every
	// colonist is sent inside, so they queue at it
	"crowd":  func(g *game.Game, w, h int) { crowd(g, w/2+13, h/2) },
	"rescue": centred(rescue),
	// README clip (MED-01..03): Bo bleeding from deep cuts in the yard,
	// medicine and a hospital bed by the house
	"bleeding": centred(bleedingScene),
	// README clip (MED-06): a peg leg to fit, a hospital bed with a powered
	// vitals monitor, medicine at hand
	"surgery": centred(surgeryScene),
	// README clip (PLAY-024): a powered radio console and a trade beacon by
	// the stockpile, goods and silver to sell, a trader caravan arriving
	// and a ship in orbit
	"trade": centred(tradeScene),
	// README clip (PLAY-024): two powered sentry turrets by the house, then
	// a raid
	"turret": centred(turretScene),
	// README clip (MED-04): flu sweeps the colony
	"outbreak": whole(func(g *game.Game) { g.FireIncident("flu") }),
	"zoo":      whole(zoo),
	"roofs":    centred(roofYard),
	// linked walls and rock (ART-015): every connection case, five stones,
	// smoothed rock, ore and a mined-out hole
	"walls": centred(wallScene),
	"sunny": whole(func(g *game.Game) {
		g.SkipTo(12, 5) // a summer day, before sunrise
		g.SetWeather(weather.Clear, 0)
		g.HoldWeather()
	}),
}

// beds puts sleeping spots in the other facings and a double bed east of
// the start.
func beds(g *game.Game, w, h int) {
	cx, cy := w/2, h/2
	for i, f := range []string{"east", "west", "north"} {
		for dy := 0; dy < 8 && !g.BuildNow("sleeping spot", "wood", f, cx+6+dy, cy-6+3*i); dy++ {
		}
	}
	for dy := 0; dy < 6 && !g.BuildNow("double bed", "wood", "south", w/2+6, h/2+1+dy); dy++ {
	}
}

// winterHuts: a heated hut and an unheated one on a winter evening.
func winterHuts(g *game.Game, cx, cy int) {
	g.SkipTo(44, 17)
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
	g.BuildNow("tailoring bench", "wood", "south", cx+8, cy+2) // 3x1, inside the hut
	_, _ = g.SpawnStack(item.Parka, 2, cx+3, cy+2)
	_, _ = g.SpawnStack(item.Tuque, 3, cx+4, cy+2)
	_, _ = g.SpawnStack(item.Leather, 20, cx+5, cy+2)
}

func researchBench(g *game.Game, cx, cy int) {
	g.ClearPlants(cx+4, cy-3, cx+8, cy)
	g.BuildNow("research bench", "wood", "south", cx+5, cy-2)
	g.AddResearch("food_preservation", 2000)
	g.AddResearch("traps", 1600)
	g.PickResearch("traps")
}

func medic(g *game.Game, cx, cy int) {
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

func defense(g *game.Game, cx, cy int) {
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

func fun(g *game.Game, cx, cy int) {
	g.ClearPlants(cx+4, cy-2, cx+9, cy+4)
	g.BuildNow("horseshoe pin", "wood", "south", cx+6, cy+1)
	if snap := g.Snapshot(); len(snap) > 1 {
		g.ForceBreak(snap[1].ID, mentalbreak.SadWander)
	}
}

func drying(g *game.Game, cx, cy int) {
	for dx := 0; dx < 6 && !g.BuildNow("drying rack", "wood", "south", cx+6+dx, cy+2); dx++ {
	}
	for dx := 0; dx < 6 && !g.BuildNow("drying rack", "wood", "south", cx+6+dx, cy+4); dx++ {
	}
	_, _ = g.SpawnStack(item.Meat, 40, cx+5, cy+6)
	_, _ = g.SpawnStack(item.Berries, 20, cx+6, cy+6)
}

func armory(g *game.Game) {
	kinds := []item.Kind{item.Knife, item.Spear, item.Rifle}
	for i, p := range g.Snapshot() {
		g.Arm(p.ID, kinds[i%len(kinds)])
		g.Enqueue(command.Command{Kind: command.DraftPawn, Pawn: uint64(p.ID), Enabled: true})
	}
	g.RaidArmed(item.Knife, item.Spear, item.Club, item.Rifle)
}

func chores(g *game.Game, cx, cy int) {
	g.Enqueue(command.Command{Kind: command.DesignateCut, X: cx - 9, Y: cy - 6, X2: cx - 3, Y2: cy - 2})
	g.Enqueue(command.Command{Kind: command.DesignateMine, X: cx - 2, Y: cy + 7, X2: cx + 4, Y2: cy + 9})
	if snap := g.Snapshot(); len(snap) > 2 {
		g.Injure(snap[2].ID, 35)
	}
}

func corridor(g *game.Game, cx, cy int) {
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

func crowd(g *game.Game, cx, cy int) {
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

func rescue(g *game.Game, cx, cy int) {
	if snap := g.Snapshot(); len(snap) > 1 {
		g.Injure(snap[1].ID, 75)
	}
	g.BuildNow("sleeping spot", "wood", "south", cx-4, cy+1)
	g.BuildNow("bed", "wood", "east", cx-4, cy+3)
}
