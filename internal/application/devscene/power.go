package devscene

import (
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/sim/command"
)

// powerBaseRow is how far north of the start the power base's conduit
// line runs (the capture state "power" centres there too).
const powerBaseRow = 9

// powerBase builds a small powered base around (cx, cy) for the -scenario
// power captures: a fuelled wood-fired generator,
// a solar panel and a wind turbine feed a battery through a conduit line
// with a switch and a hidden stretch; lamps, a heater and an electric
// stove draw on it; a lamp east of a gap is off the grid, and a torch lamp
// burns west of it all. West of the line stands a walled room (PWR-1): a
// conduit under its south wall, a cooler set in its north wall, a wall lamp
// and a heater inside, every wire over the walls.
func powerBase(g *game.Game, cx, cy int) {
	y := cy - powerBaseRow                // north of the start's house and fields
	g.ClearPlants(cx-20, y-5, cx+17, y+9) // trees would block the pieces
	for x := cx - 9; x <= cx+8; x++ {
		kind := "power conduit"
		switch {
		case x == cx+2:
			kind = "power switch"
		case x >= cx-2 && x <= cx:
			kind = "hidden conduit"
		}
		g.BuildNow(kind, "steel", "south", x, y)
	}
	g.BuildNow("wood-fired generator", "steel", "south", cx-9, y-3)
	g.FuelCampfire(cx-9, y-3)
	g.BuildNow("solar panel", "steel", "south", cx-6, y+1)
	g.BuildNow("wind turbine", "steel", "south", cx-12, y-1)
	g.BuildNow("battery", "steel", "south", cx-4, y-3)
	g.BuildNow("lamp", "wood", "south", cx+4, y-1)
	g.BuildNow("heater", "steel", "south", cx+6, y-1)
	g.BuildNow("electric stove", "steel", "south", cx+4, y+1)
	g.BuildNow("lamp", "wood", "south", cx+16, y+8) // too far from any conduit: not connected
	g.BuildNow("torch lamp", "wood", "south", cx-14, y+3)
	g.FuelCampfire(cx-14, y+3)
	for x := cx - 14; x <= cx-9; x++ { // the feed from the line to the room's wall
		g.BuildNow("power conduit", "steel", "south", x, y+1)
	}
	g.BuildNow("power conduit", "steel", "south", cx-14, y)
	for x := cx - 19; x <= cx-14; x++ {
		if x >= cx-16 {
			g.BuildNow("power conduit", "steel", "south", x, y-1) // under the south wall
		}
		g.BuildNow("wall", "steel", "south", x, y-1)
		if x != cx-16 {
			g.BuildNow("wall", "steel", "south", x, y-5)
		}
	}
	g.BuildNow("cooler", "steel", "north", cx-16, y-5) // into the gap the wall left
	for ry := y - 4; ry <= y-2; ry++ {
		g.BuildNow("wall", "steel", "south", cx-19, ry)
		g.BuildNow("wall", "steel", "south", cx-14, ry)
	}
	g.BuildNow("wall lamp", "steel", "east", cx-18, y-3)
	g.BuildNow("heater", "steel", "south", cx-15, y-3)
	for ry := y - 4; ry <= y-2; ry++ { // a dark floor, to read the wires on
		for x := cx - 18; x <= cx-15; x++ {
			g.BuildNow("flagstone", "slate blocks", "south", x, ry)
		}
	}
}

// bleedingScene: a colonist bleeding from deep cuts a few tiles from a
// hospital bed, with medicine in store, so the doctor rescues and tends
// (the bleed rate and death clock show in the Health tab).
func bleedingScene(g *game.Game, cx, cy int) {
	g.ClearPlants(cx-3, cy-4, cx+6, cy+3)
	g.BuildNow("hospital bed", "steel", "south", cx+4, cy-3)
	_, _ = g.SpawnStack(item.HerbalMedicine, 4, cx+2, cy-3)
	_, _ = g.SpawnStack(item.IndustrialMedicine, 2, cx+2, cy-2)
	if snap := g.Snapshot(); len(snap) > 1 {
		g.InjureCut(snap[1].ID, 14)
		g.InjureCut(snap[1].ID, 12)
	}
}

// surgeryScene: Bo has lost a leg; a peg leg and medicine lie by a
// hospital bed beside a vitals monitor powered by a fuelled generator, and
// the install is queued, so the doctor carries Bo in and operates.
func surgeryScene(g *game.Game, cx, cy int) {
	g.ClearPlants(cx-6, cy-6, cx+6, cy+2)
	g.BuildNow("hospital bed", "steel", "south", cx+2, cy-4)
	g.BuildNow("vitals monitor", "steel", "south", cx+3, cy-4)
	for x := cx - 3; x <= cx+3; x++ {
		g.BuildNow("power conduit", "steel", "south", x, cy-2)
	}
	g.BuildNow("wood-fired generator", "steel", "south", cx-5, cy-3)
	g.FuelCampfire(cx-5, cy-3)
	_, _ = g.SpawnStack(item.HerbalMedicine, 6, cx, cy-4)
	_, _ = g.SpawnStack(item.PegLeg, 1, cx, cy-3)
	if snap := g.Snapshot(); len(snap) > 1 {
		bo := snap[1].ID
		g.DevLosePart(bo, "left leg")
		g.Enqueue(command.Command{Kind: command.AddOperation, Pawn: uint64(bo),
			Index: int(body.InstallImplant), Build: uint8(body.LegL), Stuff: uint8(item.PegLeg)})
	}
}

// turretScene: east of the fields, a fuelled generator powering two
// sentry turrets on a conduit, then a raid of four.
func turretScene(g *game.Game, cx, cy int) {
	g.ClearPlants(cx+1, cy-2, cx+9, cy+3)
	g.BuildNow("wood-fired generator", "steel", "south", cx+7, cy-1)
	g.FuelCampfire(cx+7, cy-1)
	for x := cx + 2; x <= cx+7; x++ {
		g.BuildNow("power conduit", "steel", "south", x, cy+1)
	}
	g.BuildNow("sentry turret", "steel", "south", cx+3, cy+2)
	g.BuildNow("sentry turret", "steel", "south", cx+6, cy+2)
	g.Raid(4)
}

// tradeScene: east of the fields, a fuelled generator powering a radio
// console, a trade beacon with silver and goods in reach; the colony
// gets the default factions, then a trader caravan comes and, once the
// console has power, a ship enters orbit.
func tradeScene(g *game.Game, cx, cy int) {
	g.DevAddFactions()
	g.ClearPlants(cx+1, cy-2, cx+9, cy+3)
	g.BuildNow("wood-fired generator", "steel", "south", cx+7, cy-1)
	g.FuelCampfire(cx+7, cy-1)
	for x := cx + 2; x <= cx+7; x++ {
		g.BuildNow("power conduit", "steel", "south", x, cy+1)
	}
	g.BuildNow("radio console", "steel", "south", cx+2, cy+2)
	g.BuildNow("trade beacon", "steel", "south", cx+6, cy+2)
	_, _ = g.SpawnStack(item.Silver, 400, cx+7, cy+2)
	_, _ = g.SpawnStack(item.Leather, 30, cx+5, cy+2)
	_, _ = g.SpawnStack(item.Cloth, 40, cx+7, cy+3)
	g.FireIncident("trader_caravan")
	g.Step(240) // the generator lights and the console comes online
	g.FireIncident("orbital_trader")
}
