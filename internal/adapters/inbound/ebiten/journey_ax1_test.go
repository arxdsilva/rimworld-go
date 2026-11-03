package ebiten

// journey_ax1_test.go (AX-1): the Architect's remaining orders and zone
// clears through the real UI loop: uninstall from the Architect and the
// building pane, the crate's Reinstall placement, chop wood, slaughter,
// remove floor, delete zone, the two roof-area clears, strip from the
// Architect and the pawn pane, opening a grave from its pane and from the
// Architect, and claiming a ruin on a generated map.

import (
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/dialogs"
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/adapters/inbound/ebiten/inspector"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/difficulty"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/scenario"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/site"
	"rimworld-go/pkg/mathx"
	"rimworld-go/pkg/randx"
)

// ax1Drag arms tool in cat, drags it over the rectangle, leaves the
// Architect, and returns the command of kind the drag sent.
func ax1Drag(t *testing.T, j *journey, cat, tool string, kind command.Kind, x0, y0, x1, y1 int) command.Command {
	t.Helper()
	j.lookAt((x0+x1)/2, (y0+y1)/2)
	j.update(1)
	j.openArch(cat, tool)
	j.dragTiles(inputframe.Left, x0, y0, x1, y1, 0)
	c, ok := j.lastCmd(kind)
	if !ok {
		t.Fatalf("%s: the drag sent no %v", tool, kind)
	}
	j.leaveArch()
	j.update(1)
	return c
}

// ax1Building is the world's building of kind at (x, y).
func ax1Building(j *journey, kind string, x, y int) (game.BuildingView, bool) {
	for _, b := range j.world().Buildings {
		if b.Kind == kind && b.X == x && b.Y == y {
			return b, true
		}
	}
	return game.BuildingView{}, false
}

// ax1Crate is the crated building on the map.
func ax1Crate(j *journey) (game.ItemView, bool) {
	for _, it := range j.world().Items {
		if it.AX.Building != "" {
			return it, true
		}
	}
	return game.ItemView{}, false
}

// ax1Run runs the colony at speed 3 until cond holds, keeping colonists fed.
func ax1Run(j *journey, max int, cond func() bool) bool {
	j.key("Digit3", 0)
	defer j.key("Space", 0)
	for i := 0; i < max && !cond(); i++ {
		careFeed(j)
		j.update(1)
	}
	return cond()
}

// TestJourneyAX1UninstallReinstall: the bed pane's Uninstall marks it (the
// pane says so and the map draws the mark), a builder packs it into a
// crate that keeps its wear, the crate's pane names it, and its Reinstall
// places a blueprint the builder carries the crate to and installs.
func TestJourneyAX1UninstallReinstall(t *testing.T) {
	t.Parallel()
	j := archJourney(t)
	if !j.g.BuildNow("bed", "wood", "south", 9, 5) {
		t.Fatal("no bed")
	}
	j.clickTile(9, 5, inputframe.Left, 0)
	if !j.clickID(inspector.UninstallBtnID) {
		t.Fatal("Uninstall is disabled on a bed")
	}
	if c, ok := j.lastCmd(command.DesignateUninstall); !ok || !c.Enabled || c.X != 9 || c.Y != 5 {
		t.Fatalf("Uninstall sent %+v (%v)", c, ok)
	}
	j.update(1)
	b, _ := ax1Building(j, "bed", 9, 5)
	if !b.AX.Uninstall || !strings.Contains(strings.Join(j.a.insp.Building().BuildingPane(b.ID).Lines, "\n"), "Marked to uninstall") {
		t.Fatalf("the bed shows no uninstall mark: %+v", b.AX)
	}
	j.draw(1) // the map's uninstall mark
	if !ax1Run(j, 6000, func() bool { _, ok := ax1Crate(j); return ok }) {
		t.Fatal("no builder packed the bed")
	}
	crate, _ := ax1Crate(j)
	if crate.AX.Building != "bed" || crate.Stuff != "wood" {
		t.Fatalf("crate %+v, want a wooden bed", crate)
	}
	// the builder's figure covers the crate while it stands by: wait for it
	// to walk off
	ax1Run(j, 4000, func() bool {
		for _, p := range j.g.Snapshot() {
			if mathx.Abs(p.X-crate.X) <= 1 && mathx.Abs(p.Y-crate.Y) <= 2 {
				return false
			}
		}
		return true
	})
	j.lookAt(crate.X, crate.Y)
	j.update(1)
	j.settle()
	j.clickTile(crate.X, crate.Y, inputframe.Left, 0)
	if lines := strings.Join(j.a.insp.Thing().ItemPane(crate.ID).Lines, "\n"); !strings.Contains(lines, "packed bed") {
		t.Fatalf("crate pane %q, want what it holds", lines)
	}
	if !j.clickID(inspector.ReinstallBtnID) {
		t.Fatal("Reinstall is disabled")
	}
	if st := j.a.arch.Mover.State(); !st.On || st.Crate != crate.ID {
		t.Fatalf("Reinstall mode %+v, want the crate", *st)
	}
	j.draw(1) // the ghost
	j.key("E", 0)
	j.settle()
	j.clickTile(14, 6, inputframe.Left, 0)
	c, ok := j.lastCmd(command.Reinstall)
	if !ok || c.Target != uint64(crate.ID) || c.X != 14 || c.Y != 6 || c.Facing != 1 {
		t.Fatalf("Reinstall sent %+v (%v), want crate %d at (14, 6) facing east", c, ok, crate.ID)
	}
	j.update(1)
	if !ax1Run(j, 8000, func() bool { _, ok := ax1Building(j, "bed", 14, 6); return ok }) {
		t.Fatalf("the bed was never reinstalled: frames %+v", j.world().Frames)
	}
	if _, ok := ax1Crate(j); ok {
		t.Fatal("the crate is still on the map")
	}
}

// TestJourneyAX1ArchitectUninstallAndChop: the Uninstall card marks a
// dragged bed; Chop wood marks the grown tree, not the sapling or the
// bush.
func TestJourneyAX1ArchitectUninstallAndChop(t *testing.T) {
	t.Parallel()
	j := archJourney(t)
	if !j.g.BuildNow("bed", "wood", "south", 9, 5) {
		t.Fatal("no bed")
	}
	ax1Drag(t, j, "orders", "uninstall", command.DesignateUninstall, 8, 4, 10, 6)
	if b, _ := ax1Building(j, "bed", 9, 5); !b.AX.Uninstall {
		t.Fatal("the Uninstall drag did not mark the bed")
	}
	grown, _ := j.g.SpawnPlantSpecies("tree", 12, 3, 1<<30)
	young, _ := j.g.SpawnPlantSpecies("tree", 13, 3, 1)
	bush, _ := j.g.SpawnPlantSpecies("berry_bush", 14, 3, 1<<30)
	ax1Drag(t, j, "orders", "chop", command.DesignateChopWood, 12, 3, 14, 3)
	cut := map[entity.EntityID]bool{}
	for _, p := range j.world().Plants {
		cut[p.ID] = p.Cut
	}
	if !cut[grown] || cut[young] || cut[bush] {
		t.Fatalf("chop wood marks: grown %v young %v bush %v", cut[grown], cut[young], cut[bush])
	}
}

// TestJourneyAX1SlaughterAndRemoveFloor: Slaughter marks the tame
// dustbuck only; Remove floor marks the floor under the bed, not the bed.
func TestJourneyAX1SlaughterAndRemoveFloor(t *testing.T) {
	t.Parallel()
	j := archJourney(t)
	wild, _ := j.g.SpawnAnimal("dustbuck", 10, 6)
	tame, _ := j.g.SpawnAnimal("dustbuck", 12, 6)
	j.g.TameAnimal(tame, "Pip")
	ax1Drag(t, j, "orders", "slaughter", command.DesignateSlaughter, 9, 5, 13, 7)
	for _, an := range j.world().Animals {
		if an.ID == wild && an.Designated || an.ID == tame && !an.Designated {
			t.Fatalf("slaughter marks %+v", an)
		}
	}
	if !j.g.BuildNow("floor", "wood", "south", 15, 3) || !j.g.BuildNow("bed", "wood", "south", 15, 3) {
		t.Fatal("no floor and bed")
	}
	ax1Drag(t, j, "floors", "remove_floor", command.DesignateRemoveFloor, 15, 3, 15, 3)
	f, _ := ax1Building(j, "floor", 15, 3)
	b, _ := ax1Building(j, "bed", 15, 3)
	if !f.Deconstruct || b.Deconstruct {
		t.Fatalf("remove floor marked floor %v bed %v", f.Deconstruct, b.Deconstruct)
	}
}

// TestJourneyAX1DeleteZoneAndRoofClears: Delete zone erases the dragged
// part of a stockpile; the clear cards take cells out of the build roof
// and remove roof areas.
func TestJourneyAX1DeleteZoneAndRoofClears(t *testing.T) {
	t.Parallel()
	j := zoneJourney(t)
	j.layZone("zones", "stockpile", 8, 2, 12, 4)
	j.settle()
	ax1Drag(t, j, "zones", "delete_zone", command.DesignateDeleteZone, 8, 2, 9, 4)
	if _, ok := j.zoneAt(8, 3); ok {
		t.Fatal("the deleted cells still hold the stockpile")
	}
	if _, ok := j.zoneAt(12, 3); !ok {
		t.Fatal("the rest of the stockpile went too")
	}
	ax1Drag(t, j, "zones", "roof", command.DesignateRoof, 14, 2, 16, 4)
	if c := ax1Drag(t, j, "zones", "clear_roof_area", command.DesignateClearRoofArea, 14, 2, 14, 4); c.X != 14 {
		t.Fatalf("clear build roof area sent %+v", c)
	}
	ax1Drag(t, j, "zones", "noroof", command.DesignateRemoveRoof, 17, 2, 18, 4)
	if c := ax1Drag(t, j, "zones", "clear_noroof_area", command.DesignateClearNoRoofArea, 17, 2, 17, 4); c.X != 17 {
		t.Fatalf("clear remove roof area sent %+v", c)
	}
	j.draw(1)
}

// TestJourneyAX1StripDowned: the downed colonist's pane offers Strip; the
// mark shows on its pane, and a hauler strips it. The Architect's Strip
// card marks the same way.
func TestJourneyAX1StripDowned(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}, [2]int{12, 6}))
	_, patient, px, py := careDoerAndPatient(j)
	c := ax1Drag(t, j, "orders", "strip", command.DesignateStrip, px, py, px, py)
	if !c.Enabled || !j.pawn(patient).AX.StripMarked {
		t.Fatalf("the Strip card sent %+v; marked %v", c, j.pawn(patient).AX.StripMarked)
	}
	j.update(1)
	j.lookAt(px, py)
	j.update(1)
	j.settle()
	j.clickTile(px, py, inputframe.Left, 0)
	if uint64(j.a.renderer.Selected()) != patient {
		t.Fatalf("selected %d, want the patient %d", j.a.renderer.Selected(), patient)
	}
	if !j.clickID(inspector.StripBtnID) { // marked: the toggle cancels
		t.Fatal("Strip is disabled on a downed colonist")
	}
	if u, ok := j.lastCmd(command.Undesignate); !ok || u.Order != command.OrderStrip {
		t.Fatalf("Cancel strip sent %+v (%v)", u, ok)
	}
	j.update(1)
	if !j.clickID(inspector.StripBtnID) {
		t.Fatal("Strip is disabled")
	}
	j.update(1)
	if !strings.Contains(strings.Join(j.a.insp.Pawn().PawnLines(j.pawn(patient)), "\n"), "Marked for stripping") {
		t.Fatal("the pawn pane shows no strip mark")
	}
	j.draw(1) // the map's strip mark
	if !ax1Run(j, 8000, func() bool { return !j.pawn(patient).AX.StripMarked }) {
		t.Fatal("nobody stripped the downed colonist")
	}
}

// TestJourneyAX1OpenGrave: a buried colonist's grave pane offers Open (the
// Architect's Open card marks it too); a hauler lifts the body out.
func TestJourneyAX1OpenGrave(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}, [2]int{12, 6}))
	ids := ordColonists(j)
	dead := ids[1]
	for dmg := 50; dmg <= 2000 && j.pawn(dead).Status != "dead"; dmg += 50 {
		j.g.Injure(entity.EntityID(dead), dmg)
	}
	if j.pawn(dead).Status != "dead" {
		t.Fatal("fixture: could not kill the colonist")
	}
	if !j.g.BuildNow("grave", "wood", "south", 15, 2) {
		t.Fatal("no grave")
	}
	// the body fell outside the Home area, so it lies forbidden: Allow it
	body := j.pawn(dead)
	ax1Drag(t, j, "orders", "allow", command.DesignateForbid, body.X, body.Y, body.X, body.Y)
	if !ax1Run(j, 12000, func() bool { g, _ := ax1Building(j, "grave", 15, 2); return g.Occupant != "" }) {
		t.Fatal("the dead colonist was never buried")
	}
	ax1Drag(t, j, "orders", "open", command.DesignateOpen, 15, 2, 15, 2)
	if g, _ := ax1Building(j, "grave", 15, 2); !g.AX.OpenMarked {
		t.Fatal("the Open card did not mark the grave")
	}
	j.lookAt(15, 2)
	j.update(1)
	j.settle()
	j.clickTile(15, 2, inputframe.Left, 0)
	if !j.clickID(inspector.OpenGraveBtnID) { // marked: the toggle cancels
		t.Fatal("Open is disabled on a filled grave")
	}
	if u, ok := j.lastCmd(command.Undesignate); !ok || u.Order != command.OrderOpen {
		t.Fatalf("Cancel open sent %+v (%v)", u, ok)
	}
	j.update(1)
	if !j.clickID(inspector.OpenGraveBtnID) {
		t.Fatal("Open is disabled")
	}
	j.update(1)
	g, _ := ax1Building(j, "grave", 15, 2)
	if !strings.Contains(strings.Join(j.a.insp.Building().BuildingPane(g.ID).Lines, "\n"), "Marked to open") {
		t.Fatal("the grave pane shows no open mark")
	}
	j.draw(1) // the map's open mark
	if !ax1Run(j, 8000, func() bool { g, _ := ax1Building(j, "grave", 15, 2); return g.Occupant == "" }) {
		t.Fatal("nobody opened the grave")
	}
}

// ax1RuinColony is a generated colony on a settled tile (at least one
// ruin, wiki-less MAPGEN-2 tuning) with a ruin in plain view, and that
// ruin.
func ax1RuinColony(t *testing.T) (*game.Game, game.BuildingView) {
	t.Helper()
	d, ok := scenario.Builtin("broken_hull")
	if !ok {
		t.Fatal("no broken_hull scenario")
	}
	for seed := uint64(4243); seed < 4263; seed++ {
		s := &game.Setup{
			Scenario: d, Storyteller: "steady", Preset: difficulty.Seasoned, Difficulty: difficulty.Of(difficulty.Seasoned),
			Site: site.Site{Biome: biome.TemperateForest, Climate: biome.DefOf(biome.TemperateForest).Climate, Hills: site.Flat,
				Stones: []geology.Stone{geology.Granite}, Settled: true},
			Factions: faction.Generate(faction.DefaultSlots(), randx.New(7)),
			MapSize:  80, Season: game.SeasonSpring, Seed: seed,
		}
		s.Colonists = game.NewCandidatePool(s.Seed, s.Scenario.Apply()).Selected()
		b := game.NewColonyBuilder(game.Config{Setup: s})
		for i := 0; i < 400; i++ {
			if _, _, done := b.Step(1); done {
				break
			}
		}
		g := b.Game()
		for _, bv := range g.World().Buildings {
			if bv.Ruin && bv.Kind == "wall" {
				return g, bv
			}
		}
	}
	t.Fatal("no settled-tile colony showed a ruin")
	return nil, game.BuildingView{}
}

// TestJourneyAX1ClaimRuin: on a generated colony the Claim card makes a
// ruined wall the colony's own.
func TestJourneyAX1ClaimRuin(t *testing.T) {
	t.Parallel()
	g, ruin := ax1RuinColony(t)
	j := newJourney(t, jGame(g))
	j.lookAt(ruin.X, ruin.Y)
	j.update(1)
	j.openArch("orders", "claim")
	j.dragTiles(inputframe.Left, ruin.X, ruin.Y, ruin.X, ruin.Y, 0)
	if _, ok := j.lastCmd(command.DesignateClaim); !ok {
		t.Fatal("the Claim drag sent nothing")
	}
	j.leaveArch()
	j.update(1)
	for _, b := range j.world().Buildings {
		if b.ID == ruin.ID && b.Ruin {
			t.Fatalf("the ruin %s at (%d, %d) is still a ruin", b.Kind, b.X, b.Y)
		}
	}
}

// TestJourneyAX1BuyCrateFromTradeShip: the orbital trade ship's crated
// furniture shows as its own row in the trade dialog; taking one and
// accepting sends the crate in the offer, and the drop pod brings a
// crated building of that kind.
func TestJourneyAX1BuyCrateFromTradeShip(t *testing.T) {
	t.Parallel()
	j := radioColony(t)
	if !j.g.FireIncident("orbital_trader") {
		t.Fatal("the orbital trader incident refused this colony")
	}
	if _, err := j.g.SpawnStack(item.Silver, 3000, 13, 6); err != nil {
		t.Fatal(err)
	}
	useRadio(t, j)
	j.clickID("comms.call")
	j.update(2)
	if j.a.uiModalTop() != dialogs.TradeModalID {
		t.Fatalf("top modal %q after the call, want the trade dialog", j.a.uiModalTop())
	}
	v, _ := j.g.TradeView()
	var row game.TradeRowView
	for _, r := range v.Rows {
		if strings.Contains(r.Kind, ":") && r.TraderQty > 0 {
			row = r
			break
		}
	}
	if row.Kind == "" {
		t.Fatalf("the trade ship stocks no crated furniture: %+v", v.Rows)
	}
	if !strings.HasSuffix(row.Label, "(crated)") || row.WillSell {
		t.Fatalf("crate row %+v", row)
	}
	if !j.clickID("trade." + row.Kind + ".take.1") {
		t.Fatalf("take 1 %s is disabled", row.Label)
	}
	if !j.clickID("modal." + dialogs.TradeModalID + ".accept") {
		t.Fatalf("Accept is disabled with a %s on the table", row.Label)
	}
	if c, ok := j.lastCmd(command.ProposeTrade); !ok || !strings.Contains(c.Name, "/") {
		t.Fatalf("ProposeTrade %+v (sent %v), want the crate's building in the offer", c, ok)
	}
	j.clickID("modal." + dialogs.TradeModalID + ".cancel")
	want := strings.SplitN(row.Kind, ":", 2)[1]
	if !ax1Run(j, 3000, func() bool { c, ok := ax1Crate(j); return ok && c.AX.Building == want }) {
		t.Fatalf("no crated %s came down", want)
	}
}
