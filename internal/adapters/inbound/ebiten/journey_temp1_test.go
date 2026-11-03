package ebiten

// journey_temp1_test.go: the TEMP-1..4 journeys: passive cooler and vent
// through the Architect, a heater and the open door, unroofed rooms,
// clothing layers, heat waves, colonist and animal exposure.

import (
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/hud"
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/adapters/inbound/ebiten/inspector"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/pkg/mathx"
)

// t1Room walls an interior (x0..x1, y0..y1) with a ring of walls, leaving
// the tile door (or none when door is [2]int{-1, -1}).
func t1Room(j *journey, x0, y0, x1, y1 int, door [2]int) {
	j.t.Helper()
	for x := x0 - 1; x <= x1+1; x++ {
		for y := y0 - 1; y <= y1+1; y++ {
			if x >= x0 && x <= x1 && y >= y0 && y <= y1 {
				continue
			}
			if [2]int{x, y} == door {
				wdBuild(j, "door", x, y)
				continue
			}
			wdBuild(j, "wall", x, y)
		}
	}
}

// t1Room returns the room view at (x, y).
func t1RoomAt(j *journey, x, y int) game.RoomView {
	j.t.Helper()
	j.update(1)
	r, ok := j.g.RoomInfo(x, y)
	if !ok {
		j.t.Fatalf("no room at (%d, %d)", x, y)
	}
	return r
}

// t1Roofed waits (up to two days) for the colonists to roof the room
// holding (x, y).
func t1Roofed(j *journey, x, y int) {
	j.t.Helper()
	if !fdSteps(j, 2*foodDay, func() bool { careFeed(j); return t1RoomAt(j, x, y).Roofed >= 0.9 }) {
		j.t.Fatalf("the room at (%d, %d) is only %.0f%% roofed after two days (needs 90 percent)", x, y, t1RoomAt(j, x, y).Roofed*100)
	}
}

// t1Hours runs the colony for h in-game hours, keeping everyone fed.
func t1Hours(j *journey, h int, each func()) {
	for i := 0; i < h; i++ {
		fdSteps(j, 2500, func() bool {
			careFeed(j)
			if each != nil {
				each()
			}
			return false
		})
	}
}

// t1Place arms an Architect tool from the Temperature category and clicks a tile.
func t1Place(j *journey, tool string, x, y int) {
	j.t.Helper()
	j.openArch("temperature", tool)
	j.lookAt(x, y)
	j.update(1)
	j.settle()
	j.clickTile(x, y, inputframe.Left, 0)
	if c, ok := j.lastCmd(command.DesignateBuild); !ok || c.X != x || c.Y != y {
		j.t.Fatalf("clicking (%d, %d) with %s sent %+v (%v), want a DesignateBuild there", x, y, tool, c, ok)
	}
	j.leaveArch()
}

// t1Building is the building view of the kind at (x, y).
func t1Building(j *journey, kind string, x, y int) (game.BuildingView, bool) {
	for _, b := range j.world().Buildings {
		if b.Kind == kind && b.X == x && b.Y == y {
			return b, true
		}
	}
	return game.BuildingView{}, false
}

// TestJourneyTempPassiveCoolerAndVent (TEMP-1): the Architect builds a
// passive cooler and a vent; the fuelled cooler cools a hot sealed room
// toward its floor.
func TestJourneyTempPassiveCoolerAndVent(t *testing.T) {
	t.Parallel()
	g := game.New(game.Config{Seed: 3, Width: 30, Height: 30, Biome: biome.Desert})
	if _, err := g.SpawnPawn(12, 8); err != nil {
		t.Fatal(err)
	}
	j := newJourney(t, jGame(g))
	j.g.ClearPlants(0, 0, 29, 29)
	t1Room(j, 11, 12, 13, 14, [2]int{12, 11})
	g.SkipTo(8, 14) // a hot afternoon
	t1Place(j, "passive_cooler", 12, 13)
	t1Place(j, "vent", 14, 13) // the east wall
	if _, err := j.g.SpawnStack(item.Wood, 60, 12, 9); err != nil {
		t.Fatal(err)
	}
	if _, err := j.g.SpawnStack(item.Steel, 40, 13, 9); err != nil {
		t.Fatal(err)
	}
	built := false
	j.key("Digit3", 0)
	for i := 0; i < 8000 && !built; i++ {
		careFeed(j)
		j.update(1)
		_, a := t1Building(j, "passive cooler", 12, 13)
		_, b := t1Building(j, "vent", 14, 13)
		built = a && b
	}
	j.key("Space", 0)
	if !built {
		t.Fatalf("the passive cooler and the vent were not both built: %+v", j.world().Frames)
	}
	t1Roofed(j, 12, 13)
	hot := t1RoomAt(j, 12, 13).Temp
	t1Hours(j, 12, nil)
	cooler, _ := t1Building(j, "passive cooler", 12, 13)
	if cooler.HeatRate >= 0 {
		t.Fatalf("the passive cooler's HeatRate is %d (fuel %d%%), want it cooling", cooler.HeatRate, cooler.FuelPct)
	}
	room := t1RoomAt(j, 12, 13)
	if room.Temp >= hot {
		t.Fatalf("room temperature %d -> %d tenths of a degree, want it falling with the cooler running", hot, room.Temp)
	}
	if room.HeatRate >= 0 {
		t.Fatalf("RoomView.HeatRate is %d with the cooler running", room.HeatRate)
	}
}

// t1Winter is a colony in the depth of winter with a walled room
// (interior 11..15 x 12..14, a door at (13, 11)) and a heater on its own
// generator inside it.
func t1Winter(t *testing.T, hold bool) *journey {
	t.Helper()
	g := game.New(game.Config{Seed: 4, Width: 30, Height: 30})
	if _, err := g.SpawnPawn(13, 8); err != nil {
		t.Fatal(err)
	}
	g.SkipTo(38, 7) // a cold late-autumn morning, about -8 C: bearable for the builder
	j := newJourney(t, jGame(g))
	j.g.ClearPlants(0, 0, 29, 29)
	t1Room(j, 11, 12, 15, 14, [2]int{13, 11})
	wdBuild(j, "wood-fired generator", 11, 12)
	j.g.BuildNow("power conduit", "steel", "south", 13, 12)
	wdBuild(j, "heater", 14, 12)
	j.g.FuelCampfire(11, 12)
	if hold {
		j.g.Enqueue(command.Command{Kind: command.SetDoorHoldOpen, X: 13, Y: 11, Enabled: true})
		j.g.ApplyPending()
	}
	return j
}

// TestJourneyTempHeaterAndOpenDoor (TEMP-1): a heater warms a closed room
// in winter (RoomView.HeatRate above zero) and a held-open door lets the
// cold in faster than a shut one.
func TestJourneyTempHeaterAndOpenDoor(t *testing.T) {
	t.Parallel()
	run := func(hold bool) (game.RoomView, int) {
		j := t1Winter(t, hold)
		t1Roofed(j, 13, 13)
		t1Hours(j, 10, func() { j.g.FuelCampfire(11, 12) })
		return t1RoomAt(j, 13, 13), j.g.TemperatureAt(13, 8)
	}
	closed, out := run(false)
	open, _ := run(true)
	if closed.HeatRate <= 0 {
		t.Fatalf("RoomView.HeatRate is %d with a powered heater", closed.HeatRate)
	}
	if closed.Temp <= out+100 {
		t.Fatalf("the heated room is %d tenths against %d outside, want it clearly warmer", closed.Temp, out)
	}
	if open.Temp >= closed.Temp-10 {
		t.Fatalf("held-open door room %d, shut door room %d: the open door should cool it faster", open.Temp, closed.Temp)
	}
}

// TestJourneyTempUnroofedRoomIsOutdoors (TEMP-1): a walled room whose roof
// is not built reads the outdoor temperature.
func TestJourneyTempUnroofedRoomIsOutdoors(t *testing.T) {
	t.Parallel()
	g := game.New(game.Config{Seed: 4, Width: 30, Height: 30})
	if _, err := g.SpawnPawn(13, 8); err != nil {
		t.Fatal(err)
	}
	g.SkipTo(38, 7)
	j := newJourney(t, jGame(g))
	j.g.ClearPlants(0, 0, 29, 29)
	id := ordColonists(j)[0]
	j.g.Enqueue(command.Command{Kind: command.SetWorkPriority, Pawn: id, Work: command.WorkConstruct, Count: 0}) // nobody roofs it
	j.g.ApplyPending()
	t1Room(j, 11, 12, 15, 14, [2]int{13, 11})
	t1Hours(j, 4, nil)
	r := t1RoomAt(j, 13, 13)
	if r.Roofed >= 0.75 {
		t.Fatalf("the room is %.0f%% roofed, want it open to the sky", r.Roofed*100)
	}
	out := j.g.TemperatureAt(13, 8)
	if mathx.Abs(r.Temp-out) > 30 {
		t.Fatalf("the unroofed room reads %d tenths, outside is %d", r.Temp, out)
	}
}

// TestJourneyTempClothingLayers (TEMP-4): a colonist starts dressed in a
// shirt and pants, a tailoring bill makes a jacket that is worn in the
// cold, and a stripped colonist gets the Naked thought.
func TestJourneyTempClothingLayers(t *testing.T) {
	t.Parallel()
	j := t1Winter(t, false)
	id := ordColonists(j)[0]
	p := j.pawn(id)
	wornNames := strings.ToLower(strings.Join(p.Apparel, ","))
	for _, w := range p.Worn {
		wornNames += "," + strings.ToLower(w.Kind)
	}
	if !strings.Contains(wornNames, "shirt") || !strings.Contains(wornNames, "pants") {
		t.Fatalf("the colonist starts in %q, want a T-shirt and pants", wornNames)
	}
	bench := "tailoring bench"
	if !j.g.BuildNow(bench, "wood", "south", 16, 8) {
		if !j.g.BuildNow("loom", "wood", "south", 16, 8) {
			t.Fatal("no tailoring bench could be built")
		}
	}
	j.g.Enqueue(command.Command{Kind: command.AddBill, X: 16, Y: 8, Recipe: uint8(recipe.MakeJacket), Mode: 0, Count: 1})
	j.g.ApplyPending()
	if _, err := j.g.SpawnStack(item.Cloth, 60, 17, 9); err != nil {
		t.Fatal(err)
	}
	worn := func() bool {
		q := j.pawn(id)
		return strings.Contains(strings.ToLower(strings.Join(q.Apparel, ",")), "jacket")
	}
	j.key("Digit3", 0)
	for i := 0; i < 20000 && !worn(); i++ {
		careFeed(j)
		j.update(1)
	}
	j.key("Space", 0)
	if !worn() {
		t.Fatalf("no jacket on the colonist after the bill: %q", j.pawn(id).Apparel)
	}
	// stripped by a downed-colonist strip order: the Naked thought
	j2 := newJourney(t, jPawns([2]int{8, 4}, [2]int{9, 4}))
	cs := ordColonists(j2)
	j2.g.Injure(entity.EntityID(cs[1]), 90)
	j2.update(2)
	j2.g.Enqueue(command.Command{Kind: command.Prioritize, Pawn: cs[0], Order: command.OrderStrip, Target: cs[1]})
	j2.g.ApplyPending()
	j2.key("Digit3", 0)
	naked := false
	for i := 0; i < 3000 && !naked; i++ {
		j2.update(1)
		for _, th := range j2.pawn(cs[1]).Thoughts {
			naked = naked || strings.Contains(strings.ToLower(th.Label), "naked")
		}
	}
	j2.key("Space", 0)
	if !naked {
		t.Fatalf("no Naked thought on the stripped colonist: %+v", j2.pawn(cs[1]).Thoughts)
	}
}

// TestJourneyTempHeatWave (TEMP-2): a heat wave sets WorldView.ClimateEvent
// and shows the HUD chip with its +17 C card.
func TestJourneyTempHeatWave(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	j.g.SkipTo(20, 8)
	if !j.g.FireIncident("heat_wave") {
		t.Fatal(`FireIncident("heat_wave") did not fire`)
	}
	j.update(1)
	w := j.world()
	if w.ClimateEvent == "" || w.ClimateEventEnds == 0 {
		t.Fatalf("WorldView.ClimateEvent %q ends %d after a heat wave", w.ClimateEvent, w.ClimateEventEnds)
	}
	var chip hud.StatusChip
	for _, c := range hud.StatusChips(w) {
		if strings.Contains(strings.ToLower(c.Label), "heat wave") {
			chip = c
		}
	}
	if chip.Label == "" || !strings.Contains(chip.Tip, "17") {
		t.Fatalf("chips %+v, want a Heat wave chip whose card says about 17 C", hud.StatusChips(w))
	}
	j.draw(2)
}

// TestJourneyTempColonistFreezes (TEMP-3): a colonist left in the open at
// about -30 C gains hypothermia, the pane shows it, the colonist collapses
// at the extreme stage and frostbite appears.
func TestJourneyTempColonistFreezes(t *testing.T) {
	t.Parallel()
	g := game.New(game.Config{Seed: 5, Width: 24, Height: 24, Biome: biome.IceSheet})
	if _, err := g.SpawnPawn(8, 8); err != nil {
		t.Fatal(err)
	}
	g.SkipTo(45, 4)
	j := newJourney(t, jGame(g))
	j.g.FireIncident("cold_snap")
	id := ordColonists(j)[0]
	j.lookAt(8, 8)
	j.update(1)
	j.settle()
	weDraft(j, 0) // draft: it stays where it stands
	if out := j.g.TemperatureAt(8, 8); out > -250 {
		t.Fatalf("the ice sheet in winter is only %d tenths of a degree", out)
	}
	shown, down, bite := false, false, false
	lastStatus := ""
	for i := 0; i < 3*foodDay/60 && !(shown && down && bite); i++ {
		j.g.Step(60)
		careFeed(j)
		p := j.pawn(id)
		if p.Status == "dead" { // downed is the extreme stage, not the end
			t.Logf("the colonist died (%s) after %d half-minutes; shown %v down %v bite %v", p.Reason, i, shown, down, bite)
			break
		}
		if p.Hypothermia > 0 && strings.Contains(strings.Join(inspector.PawnStatusLines(p, func(entity.EntityID) string { return "" }), "\n"), "Hypothermia") {
			shown = true
		}
		down = down || p.Lying || p.Status == "downed"
		if hv, ok := j.g.Health(entity.EntityID(id)); ok && p.Hypothermia >= 600 {
			for _, c := range hv.Capacities {
				if c.Label == "Consciousness" {
					t.Logf("hypothermia %d: consciousness %d%%, state %q", p.Hypothermia, c.Pct, hv.State)
				}
			}
		}
		if p.Status != lastStatus {
			t.Logf("status %q at %d half-minutes (hypothermia %d, lying %v)", p.Status, i, p.Hypothermia, p.Lying)
			lastStatus = p.Status
		}
		bite = bite || p.Frostbite > 0
	}
	switch {
	case !shown:
		t.Fatalf("no Hypothermia line on the pane: %+v", j.pawn(id).Hypothermia)
	case !down:
		t.Fatal("the colonist never collapsed at the extreme stage")
	case !bite:
		t.Fatal("no frostbite appeared")
	}
}

// TestJourneyTempAnimalsInTheCold (TEMP-3): an animal outside its comfort
// range gains hypothermia, an insect slows down instead of dying.
func TestJourneyTempAnimalsInTheCold(t *testing.T) {
	t.Parallel()
	g := game.New(game.Config{Seed: 5, Width: 24, Height: 24, Biome: biome.IceSheet})
	if _, err := g.SpawnPawn(8, 8); err != nil {
		t.Fatal(err)
	}
	g.SkipTo(45, 4)
	insect := ""
	for i := 0; i < 200 && insect == ""; i++ {
		if s := animal.Species(i); animal.Slows(s) && s.String() != "" {
			insect = s.String()
		}
	}
	j := newJourney(t, jGame(g))
	j.g.FireIncident("cold_snap")
	iguana, err := j.g.SpawnAnimal("iguana", 14, 14)
	if err != nil {
		t.Fatal(err)
	}
	bug, err := j.g.SpawnAnimal(insect, 16, 14)
	if err != nil {
		t.Fatalf("cannot spawn the insect %q: %v", insect, err)
	}
	view := func(id entity.EntityID) (game.AnimalView, bool) { return fdAnimal(j, id) }
	hypo, bugCold := false, false
	for i := 0; i < 2*foodDay/250 && !(hypo && bugCold); i++ {
		j.g.Step(250)
		careFeed(j)
		if a, ok := view(iguana); ok && a.Hypothermia > 0 {
			hypo = true
		}
		if a, ok := view(bug); ok && a.Hypothermia > 0 {
			bugCold = true
		}
	}
	if !hypo {
		a, ok := view(iguana)
		t.Fatalf("the iguana (%v) has hypothermia %d, comfort %d..%d", ok, a.Hypothermia, a.ComfortMin, a.ComfortMax)
	}
	// keep it out until the slowdown is at full severity, where hypothermia
	// would kill anything else: the insect only slows (wiki: Ailments)
	for i := 0; i < 2*foodDay/250; i++ {
		if a, ok := view(bug); !ok || a.Hypothermia >= 100 { // percent
			break
		}
		j.g.Step(250)
		careFeed(j)
	}
	a, ok := view(bug)
	if !ok {
		t.Fatalf("the %s died of the cold; an insect only slows", insect)
	}
	if a.Hypothermia < 100 {
		t.Fatalf("the %s's slowdown only reached %d after two days at the ice sheet", insect, a.Hypothermia)
	}
	if !bugCold {
		t.Fatalf("the %s shows no cold at all (hypothermia %d, comfort %d..%d)", insect, a.Hypothermia, a.ComfortMin, a.ComfortMax)
	}
}
