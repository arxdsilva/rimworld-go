// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"slices"
	"testing"

	"rimworld-go/internal/application/game/letters"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/incident"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/roof"
	"rimworld-go/internal/domain/world/terrain"
	"rimworld-go/pkg/mathx"
)

// windfallColony is a 48x40 colony with two colonists at home, the map
// centre (24, 20): no bed, so home stays put.
func windfallColony(t *testing.T, seed uint64) (*Game, []entity.EntityID) {
	t.Helper()
	g := New(Config{Seed: seed, Width: 48, Height: 40})
	return g, []entity.EntityID{worker(t, g, 24, 20), worker(t, g, 25, 20)}
}

// oreCells are the rough-rock cells bearing ore.
func oreCells(g *Game) [][2]int {
	var out [][2]int
	for y := range g.grid.Height() {
		for x := range g.grid.Width() {
			if g.terr.At(x, y) == terrain.Rock && g.geo.OreAt(x, y) != geology.NoOre {
				out = append(out, [2]int{x, y})
			}
		}
	}
	return out
}

// newOre is the ore cells in after missing from before.
func newOre(before, after [][2]int) [][2]int {
	var out [][2]int
	for _, c := range after {
		if !slices.Contains(before, c) {
			out = append(out, c)
		}
	}
	return out
}

func bldgSpots(g *Game) [][3]int {
	var out [][3]int
	for _, id := range g.bldgIDs {
		b, _ := g.bldgs.Get(id)
		out = append(out, [3]int{int(b.Kind), b.X, b.Y})
	}
	return out
}

func lastLetterOf(t *testing.T, g *Game) MessageView {
	t.Helper()
	for i := len(g.messages) - 1; i >= 0; i-- {
		if g.messages[i].Letter {
			return g.messages[i]
		}
	}
	t.Fatal("no letter")
	return MessageView{}
}

func zoned(g *Game, x, y int) bool { _, ok := g.zones.At(x, y); return ok }

// fireMeteor fires the ore meteor and returns its cells.
func fireMeteor(t *testing.T, g *Game) [][2]int {
	t.Helper()
	before := oreCells(g)
	if !g.FireIncident("ore_meteor") {
		t.Fatal("ore meteor did not fire")
	}
	return newOre(before, oreCells(g))
}

func TestOreMeteorLandsOnOpenGround(t *testing.T) {
	t.Parallel()
	// Control: on bare ground the meteor seeds at the band's first ring
	// cell, (19, 15).
	bare, _ := windfallColony(t, 5)
	if cs, l := fireMeteor(t, bare), lastLetterOf(t, bare); !slices.Contains(cs, [2]int{19, 15}) || l.X != 19 || l.Y != 15 {
		t.Fatalf("bare meteor cells %v, letter at (%d, %d), want a seed at (19, 15)", cs, l.X, l.Y)
	}
	// Crowd the band's top row: a roof, a zone, an item, a plant, a wall, a
	// frame and an animal each rule some of it out.
	g, _ := windfallColony(t, 5)
	g.ensureRoofGrid()
	g.setRoof(15*g.grid.Width()+19, roof.Natural)
	g.zones.Paint(zone.KindStockpile, zone.NewStockpile(), zone.RectOf(20, 15, 21, 15), nil)
	if _, err := g.SpawnStack(item.Steel, 5, 22, 15); err != nil {
		t.Fatal(err)
	}
	if _, err := g.SpawnPlant(19, 16); err != nil {
		t.Fatal(err)
	}
	placeBuilt(t, g, construction.Wall, construction.South, 24, 14)
	g.placeFrameStuff(construction.Wall, item.Wood, construction.South, 27, 14)
	if _, err := g.SpawnAnimal("deer", 29, 16); err != nil {
		t.Fatal(err)
	}
	bldgs, letterCount := bldgSpots(g), len(g.messages)
	cs := fireMeteor(t, g)
	if len(cs) < incident.MeteorCellsMin || len(cs) > incident.MeteorCellsMax {
		t.Fatalf("meteor cells %v, want %d-%d", cs, incident.MeteorCellsMin, incident.MeteorCellsMax)
	}
	hx, hy := g.home()
	ore := g.geo.OreAt(cs[0][0], cs[0][1])
	for _, c := range cs {
		x, y := c[0], c[1]
		if d := max(mathx.Abs(x-hx), mathx.Abs(y-hy)); d < incident.MeteorMinDist || d > incident.MeteorRadius {
			t.Fatalf("cell %v at distance %d", c, d)
		}
		if y == 15 && x >= 19 && x <= 29 {
			t.Fatalf("cell %v on the crowded row", c)
		}
		if g.geo.OreAt(x, y) != ore || g.roofAt(x, y).Roofed() || zoned(g, x, y) || g.plantAt(x, y) != 0 || g.itemAt(x, y) != 0 {
			t.Fatalf("cell %v: mixed ore, roofed, zoned, planted or holding an item", c)
		}
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				for l := range construction.Layer(numLayers) {
					if g.layerBldgAt(x+dx, y+dy, l) != 0 || g.layerFrameAt(x+dx, y+dy, l) != 0 {
						t.Fatalf("cell %v beside a building or frame", c)
					}
				}
			}
		}
		if _, _, near := g.nearestWhere(x, y, 1, ckAll, nil); near {
			t.Fatalf("cell %v beside a creature", c)
		}
	}
	if ore != geology.Steel && ore != geology.Silver && ore != geology.Gold {
		t.Fatalf("ore %v", ore)
	}
	if !slices.Equal(bldgSpots(g), bldgs) {
		t.Fatalf("buildings moved: %v, were %v", bldgSpots(g), bldgs)
	}
	if l := lastLetterOf(t, g); len(g.messages) != letterCount+1 || l.Title != "Ore meteor" || l.Kind != letters.Good {
		t.Fatalf("letter %+v (messages %d, were %d)", l, len(g.messages), letterCount)
	}
}

// No site: SetGround (dev_hooks.go) turns the whole map to rough granite
// but home and a 3-cell island east of it, one short of a meteor.
func TestOreMeteorNoSite(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 5, Width: 48, Height: 40})
	if !g.SetGround(0, 0, 47, 39, "rock", "granite", "") || !g.SetGround(23, 19, 25, 21, "soil", "", "") ||
		!g.SetGround(30, 20, 32, 20, "soil", "", "") {
		t.Fatal("ground")
	}
	worker(t, g, 24, 20)
	terr, msgs := slices.Clone(g.terr.Cells), len(g.messages)
	if g.incidentFits(incident.OreMeteor, uint64(g.clock.Now())) || g.FireIncident("ore_meteor") {
		t.Fatal("ore meteor fit with no site")
	}
	if !slices.Equal(g.terr.Cells, terr) || len(g.messages) != msgs {
		t.Fatal("a refused meteor changed the map or posted")
	}
	// One more open cell makes the island a site.
	g.SetGround(33, 20, 33, 20, "soil", "", "")
	if !g.incidentFits(incident.OreMeteor, uint64(g.clock.Now())) {
		t.Fatal("a 4-cell island does not fit")
	}
}

func TestSupplyDropForbiddenStacks(t *testing.T) {
	t.Parallel()
	g, _ := windfallColony(t, 7)
	g.zones.Paint(zone.KindStockpile, zone.NewStockpile(), zone.RectOf(21, 17, 27, 23), nil) // home's stockpile
	g.ensureRoofGrid()
	for x := 18; x <= 30; x++ {
		g.setRoof(16*g.grid.Width()+x, roof.Natural) // a roofed row north of it
	}
	before, msgs := slices.Clone(g.itemIDs), len(g.messages)
	if !g.FireIncident("supply_drop") {
		t.Fatal("supply drop did not fire")
	}
	hx, hy := g.home()
	var kinds []item.Kind
	for _, id := range g.itemIDs {
		if slices.Contains(before, id) {
			continue
		}
		s, _ := g.stacks.Get(id)
		if !s.Forbidden || slices.Contains(kinds, s.Kind) || s.Qty <= 0 || s.Qty > g.sk.content.Items.StackLimit(s.Kind) {
			t.Fatalf("stack %+v: allowed, repeated kind or over its limit", s)
		}
		if max(mathx.Abs(s.X-hx), mathx.Abs(s.Y-hy)) > incident.SupplyRadius || g.roofAt(s.X, s.Y).Roofed() || zoned(g, s.X, s.Y) {
			t.Fatalf("stack %+v: far, roofed or zoned", s)
		}
		kinds = append(kinds, s.Kind)
	}
	if len(kinds) < incident.SupplyStacksMin || len(kinds) > incident.SupplyStacksMax {
		t.Fatalf("supply kinds %v", kinds)
	}
	if l := lastLetterOf(t, g); len(g.messages) != msgs+1 || l.Title != "Supply drop" || l.Kind != letters.Good {
		t.Fatalf("letter %+v", l)
	}
}

// songThought reports id has the positive sky song thought.
func songThought(g *Game, id entity.EntityID) bool {
	return slices.ContainsFunc(g.thoughts(id), func(th mood.Thought) bool { return th.Kind == mood.SkySong && th.Value > 0 })
}

func TestSkySongMoodHUDAndSave(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 3, Width: 24, Height: 18})
	ids := []entity.EntityID{worker(t, g, 8, 8), worker(t, g, 9, 8)}
	if !g.FireIncident("transport_pod_crash") || len(g.visitorIDs()) != 1 {
		t.Fatal("fixture: a guest")
	}
	guest := g.visitorIDs()[0]
	now := uint64(g.clock.Now())
	if !g.FireIncident("sky_song") {
		t.Fatal("sky song did not fire")
	}
	until := g.incidents.SongUntil
	if d := until - now; d < incident.SongMin*clock.TicksPerDay/100 || d > incident.SongMax*clock.TicksPerDay/100 {
		t.Fatalf("song lasts %d ticks", d)
	}
	if l := lastLetterOf(t, g); l.Title != "Sky song" || l.Kind != letters.Good {
		t.Fatalf("letter %+v", l)
	}
	if g.FireIncident("sky_song") {
		t.Fatal("sky song refired while playing")
	}
	check := func(playing bool) { // reads g, so it follows the reload below

		t.Helper()
		if want := map[bool]uint64{true: until}[playing]; g.World().SkySongEnds != want {
			t.Fatalf("at %d: SkySongEnds %d, want %d", g.clock.Now(), g.World().SkySongEnds, want)
		}
		for _, id := range ids {
			if songThought(g, id) != playing {
				t.Fatalf("at %d: colonist %d sky song thought %v, want %v", g.clock.Now(), id, !playing, playing)
			}
		}
		if songThought(g, guest) {
			t.Fatal("the guest was cheered")
		}
	}
	check(true)
	g = saveLoad(t, g)
	check(true)
	// Song, thought and chip end together, at until (no long Step).
	g.clock.Advance(int(until - uint64(g.clock.Now()) - 1))
	check(true)
	g.clock.Advance(1)
	check(false)
}

// A save with no song (the gob shape of a save from before the song)
// loads silent, and the song fits there.
func TestSkySongOldSaveSilent(t *testing.T) {
	t.Parallel()
	g, ids := windfallColony(t, 11)
	g = saveLoad(t, g)
	if g.incidents.SongUntil != 0 || g.World().SkySongEnds != 0 || songThought(g, ids[0]) {
		t.Fatal("a songless save loaded with a song")
	}
	if !g.incidentFits(incident.SkySong, uint64(g.clock.Now())) {
		t.Fatal("the song does not fit on a loaded songless save")
	}
}

type windfallRun struct {
	meteor [][2]int
	ore    geology.Ore
	drops  []item.Stack
	song   uint64
	next   int
}

func runWindfalls(t *testing.T, fitsFirst bool) windfallRun {
	t.Helper()
	g, _ := windfallColony(t, 13)
	g.Step(30)
	if fitsFirst { // Fits never draws, so asking first changes nothing
		for range 3 {
			for _, id := range []incident.ID{incident.OreMeteor, incident.SupplyDrop, incident.SkySong} {
				g.incidentFits(id, uint64(g.clock.Now()))
			}
		}
	}
	var r windfallRun
	r.meteor = fireMeteor(t, g)
	r.ore = g.geo.OreAt(r.meteor[0][0], r.meteor[0][1])
	before := slices.Clone(g.itemIDs)
	if !g.FireIncident("supply_drop") || !g.FireIncident("sky_song") {
		t.Fatal("windfalls did not fire")
	}
	for _, id := range g.itemIDs {
		if !slices.Contains(before, id) {
			s, _ := g.stacks.Get(id)
			r.drops = append(r.drops, item.Stack{Kind: s.Kind, Qty: s.Qty, X: s.X, Y: s.Y})
		}
	}
	r.song, r.next = g.incidents.SongUntil, g.rng.IntN(1<<30)
	return r
}

func TestWindfallsDeterministic(t *testing.T) {
	t.Parallel()
	a, b := runWindfalls(t, false), runWindfalls(t, true)
	if !slices.Equal(a.meteor, b.meteor) || a.ore != b.ore || !slices.Equal(a.drops, b.drops) || a.song != b.song || a.next != b.next {
		t.Fatalf("same seed, different windfalls:\n%+v\n%+v", a, b)
	}
}
