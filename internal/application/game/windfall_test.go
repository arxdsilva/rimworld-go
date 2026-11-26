// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

// windfall_test.go: the 25.2 helpful incidents (#13) through the whole
// game: an escape pod's stranger, an ore meteor, a supply drop and the sky
// song, fired by FireIncident and by the storyteller's own roll.

import (
	"slices"
	"testing"

	"rimworld-go/internal/application/game/care"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/incident"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/internal/domain/world/roof"
	"rimworld-go/internal/domain/world/temperature"
	"rimworld-go/internal/domain/world/terrain"
	"rimworld-go/internal/domain/world/weather"
	"rimworld-go/pkg/mathx"
)

// windfallGame is a cleared 50×50 colony at noon on day 10 (past every
// windfall's MinDay) with at least one colonist.
func windfallGame(t *testing.T, seed uint64) *Game {
	t.Helper()
	g := NewColony(Config{Seed: seed, Width: 50, Height: 50})
	jumpTo(g, 10, 12)
	g.SetWeather(weather.Clear, 0)
	g.ClearPlants(0, 0, 49, 49)
	if len(g.pawnIDs) == 0 {
		t.Fatal("fixture: no colonists")
	}
	return g
}

// titledLetters is the letters titled title.
func titledLetters(g *Game, title string) []LetterView {
	var out []LetterView
	for _, l := range g.letterViews() {
		if l.Title == title {
			out = append(out, l)
		}
	}
	return out
}

// newVisitors is the visitors that were not in before.
func newVisitors(g *Game, before []visitorRec) []visitorRec {
	var out []visitorRec
	for _, v := range g.worldS().visitors {
		if !slices.ContainsFunc(before, func(b visitorRec) bool { return b.ID == v.ID }) {
			out = append(out, v)
		}
	}
	return out
}

// crashStranger fires an escape pod crash and returns its stranger.
func crashStranger(t *testing.T, g *Game) entity.EntityID {
	t.Helper()
	before := slices.Clone(g.worldS().visitors)
	if !g.FireIncident("escape_pod_crash") {
		t.Fatal("the escape pod crash did not fire")
	}
	vs := newVisitors(g, before)
	if len(vs) != 1 {
		t.Fatalf("one stranger lands, got %d", len(vs))
	}
	return vs[0].ID
}

// TestEscapePodCrashStranger: the pod's stranger lands downed but alive
// near home, a faction-less visitor, with one letter focused on them; left
// alone a day, they are still here.
func TestEscapePodCrashStranger(t *testing.T) {
	t.Parallel()
	g := windfallGame(t, 2)
	hx, hy := g.home()
	s := crashStranger(t, g)
	g.Step(1)
	if v, _ := g.vital.Get(s); v.Status != health.Downed {
		t.Fatalf("the stranger is downed and alive: %+v", v)
	}
	if !g.visitor(s) || (careGuests{g}).FactionOf(s) != 0 {
		t.Fatalf("a faction-less visitor: visitor %v faction %d", g.visitor(s), (careGuests{g}).FactionOf(s))
	}
	p, _ := g.positions.Get(s)
	if dx, dy := p.X-hx, p.Y-hy; dx*dx+dy*dy > podRadius*podRadius {
		t.Fatalf("landed at (%d,%d), more than %d from home (%d,%d)", p.X, p.Y, podRadius, hx, hy)
	}
	if l := titledLetters(g, "Escape pod crash"); len(l) != 1 || l[0].Focus != s || l[0].Kind != "good" {
		t.Fatalf("one good letter focused on the stranger: %+v", l)
	}
	jumpTo(g, 11, 12)
	g.Step(1)
	if !g.visitor(s) {
		t.Fatal("left alone a day, the downed stranger is still a visitor")
	}
}

// TestEscapePodStrangerJoins: the crashed stranger, put in the colony's
// bed, is tended, recovers and asks to join; accepting makes them a
// colonist.
func TestEscapePodStrangerJoins(t *testing.T) {
	t.Parallel()
	g, _, _, _ := guestScene(t)
	g.climate = temperature.Climate{YearMean: 200} // mild while it heals (guest_rescue_test.go:73)
	s := crashStranger(t, g)
	g.Step(1)
	if v, _ := g.vital.Get(s); v.Status != health.Downed {
		t.Fatalf("fixture: the stranger is down: %+v", v)
	}
	bid := g.bldgIDs[len(g.bldgIDs)-1] // as strangerInBed (guest_rescue_test.go:141): rescued to the bed
	b, _ := g.bldgs.Get(bid)
	if !b.Kind.Sleeps() {
		t.Fatalf("fixture: building %d is no bed", bid)
	}
	b = b.Claim(uint64(s))
	g.bldgs.Set(bid, b)
	x, y := g.sleepCellFor(s, b)
	g.positions.Set(s, Position{X: x, Y: y})
	if g.bedUnder(s) != bid {
		t.Fatal("fixture: the stranger in bed")
	}
	var l *LetterView
	tended := false
	for i := 0; i < 600 && l == nil; i++ {
		if v, _ := g.vital.Get(s); v.Status == health.Alive && healingInjury(g.bodies[s], v) <= care.RestWellInjury+1 && g.climate.YearMean != -500 {
			// about a game hour from well (it heals about 1 an hour): deadly cold out
			// makes the ask certain (guest_rescue_test.go:172), too briefly for hypothermia
			g.climate = temperature.Climate{YearMean: -500}
		}
		if g.bedUnder(s) == 0 {
			t.Fatalf("step %d: out of bed before it recovered", i)
		}
		g.Step(250)
		tended = tended || g.tended(s)
		for _, id := range g.pawnIDs {
			g.SetNeeds(id, needs.Full, needs.Full)
		}
		g.SetNeeds(s, needs.Full, needs.Full)
		if v, _ := g.vital.Get(s); v.Status == health.Dead {
			t.Fatalf("step %d: the stranger died in care: %+v, climate %d, body %+v", i, v, g.climate.YearMean, g.bodies[s])
		}
		l = joinLetter(g, s)
	}
	if !tended || l == nil || g.bedUnder(s) != 0 {
		t.Fatalf("tended (%v), the recovered stranger asks to join and gets up (bed %d): letters %+v", tended, g.bedUnder(s), g.letterViews())
	}
	g.Enqueue(command.Command{Kind: command.AcceptLetter, Target: l.ID})
	g.Step(1)
	if !g.colonist(s) || g.personOf(s).Where != whereColony || g.visitor(s) {
		t.Fatalf("accepted, the stranger joins: colonist %v where %v", g.colonist(s), g.personOf(s).Where)
	}
}

// meteorScene zones the whole map but a 24×24 window round home, and puts
// a wall, a wall frame and a colonist inside it; it returns the window's
// corner and the three tiles.
func meteorScene(t *testing.T) (g *Game, x0, y0 int, keep [3][2]int) {
	t.Helper()
	g = windfallGame(t, 2)
	hx, hy := g.home()
	x0, y0 = max(hx-12, 0), max(hy-12, 0)
	w, h := g.grid.Width(), g.grid.Height()
	in := func(x, y int) bool { return x >= x0 && x < x0+24 && y >= y0 && y < y0+24 }
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if !in(x, y) {
				g.paintZone(zone.KindStockpile, zone.Zone{}, x, y, x, y)
			}
		}
	}
	keep[0] = [2]int{x0 + 6, y0 + 6}
	buildNow(t, g, construction.Wall, item.Wood, construction.South, keep[0][0], keep[0][1])
	keep[1] = [2]int{x0 + 16, y0 + 8}
	g.placeFrame(construction.Wall, keep[1][0], keep[1][1])
	if g.layerFrameAt(keep[1][0], keep[1][1], construction.LayerObject) == 0 {
		t.Fatal("fixture: the frame")
	}
	keep[2] = [2]int{x0 + 10, y0 + 17}
	if _, err := g.SpawnNamedPawn(keep[2][0], keep[2][1], "Rocky"); err != nil {
		t.Fatal(err)
	}
	return g, x0, y0, keep
}

// TestOreMeteorLandsOnOpenGround: 3-6 rough rock cells of one ore land on
// open ground near home, never on a building, frame, zone or pawn tile; a
// map with no room gets no meteor and no letter.
func TestOreMeteorLandsOnOpenGround(t *testing.T) {
	t.Parallel()
	g, x0, y0, keep := meteorScene(t)
	w, h := g.grid.Width(), g.grid.Height()
	before := make([]terrain.Type, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			before[y*w+x] = g.terr.At(x, y)
		}
	}
	if !g.FireIncident("ore_meteor") {
		t.Fatal("the meteor did not fire")
	}
	l := titledLetters(g, "Ore meteor")
	hx, hy := g.home()
	if len(l) != 1 || mathx.Abs(l[0].X-hx) > incident.MeteorRadius || mathx.Abs(l[0].Y-hy) > incident.MeteorRadius {
		t.Fatalf("one letter at the meteor within %d of home (%d,%d): %+v", incident.MeteorRadius, hx, hy, l)
	}
	var ores []geology.Ore
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if g.terr.At(x, y) == before[y*w+x] {
				continue
			}
			if g.terr.At(x, y) != terrain.Rock || terrain.IsRockWall(before[y*w+x]) {
				t.Fatalf("(%d,%d): %v -> %v, want open ground -> rock", x, y, before[y*w+x], g.terr.At(x, y))
			}
			if x < x0 || x >= x0+24 || y < y0 || y >= y0+24 {
				t.Fatalf("(%d,%d): rock on a zoned tile", x, y)
			}
			if mathx.Abs(x-l[0].X) > incident.MeteorCellsMax || mathx.Abs(y-l[0].Y) > incident.MeteorCellsMax {
				t.Fatalf("(%d,%d): rock far from the meteor's tile (%d,%d)", x, y, l[0].X, l[0].Y)
			}
			ores = append(ores, g.geo.OreAt(x, y))
		}
	}
	if n := len(ores); n < incident.MeteorCellsMin || n > incident.MeteorCellsMax {
		t.Fatalf("%d rock cells, want %d-%d", n, incident.MeteorCellsMin, incident.MeteorCellsMax)
	}
	if ores[0] == geology.NoOre || len(slices.Compact(slices.Clone(ores))) != 1 {
		t.Fatalf("one ore in every cell: %v", ores)
	}
	for _, k := range keep {
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				if x, y := k[0]+dx, k[1]+dy; g.terr.At(x, y) != before[y*w+x] {
					t.Fatalf("(%d,%d) beside a kept tile changed", x, y)
				}
			}
		}
	}

	// no room: every tile zoned
	g2 := windfallGame(t, 2)
	g2.paintZone(zone.KindStockpile, zone.Zone{}, 0, 0, 49, 49)
	if g2.FireIncident("ore_meteor") || len(titledLetters(g2, "Ore meteor")) != 0 {
		t.Fatal("a meteor with no room fired")
	}
}

// TestMeteorFreeRefusesOccupiedGround: the host's meteor check refuses
// every kind of occupied, covered or edge tile and takes open ground.
func TestMeteorFreeRefusesOccupiedGround(t *testing.T) {
	t.Parallel()
	g, x0, y0, keep := meteorScene(t)
	open := [2]int{x0 + 12, y0 + 3}
	plantAt := [2]int{x0 + 3, y0 + 12}
	stackAt := [2]int{x0 + 20, y0 + 12}
	roofAt := [2]int{x0 + 3, y0 + 20}
	waterAt := [2]int{x0 + 20, y0 + 20}
	for _, c := range [][2]int{open, plantAt, stackAt, roofAt} {
		g.setTerrain(c[0], c[1], terrain.Soil) // the map's own rock is no ground
	}
	if _, err := g.spawnPlant(plant.Rice, plantAt[0], plantAt[1], 0); err != nil {
		t.Fatal(err)
	}
	stack(t, g, item.Steel, 5, stackAt[0], stackAt[1])
	g.ensureRoofGrid()
	g.setRoof(roofAt[1]*g.grid.Width()+roofAt[0], roof.Built)
	g.setTerrain(waterAt[0], waterAt[1], terrain.ShallowWater)
	for _, c := range []struct {
		name string
		x, y int
		want bool
	}{
		{"open ground", open[0], open[1], true},
		{"wall", keep[0][0], keep[0][1], false},
		{"beside the wall", keep[0][0] + 1, keep[0][1], false},
		{"frame", keep[1][0], keep[1][1], false},
		{"pawn", keep[2][0], keep[2][1], false},
		{"beside the pawn", keep[2][0] - 1, keep[2][1] + 1, false},
		{"zoned", x0 - 1, y0 + 12, false},
		{"plant", plantAt[0], plantAt[1], false},
		{"stack", stackAt[0], stackAt[1], false},
		{"roofed", roofAt[0], roofAt[1], false},
		{"water", waterAt[0], waterAt[1], false},
	} {
		if got := (windfallHost{g}).MeteorFree(c.x, c.y); got != c.want {
			t.Errorf("%s (%d,%d): MeteorFree %v, want %v", c.name, c.x, c.y, got, c.want)
		}
	}
	// the inset, on an unzoned map: 2 in is ground, 1 in is not
	e := windfallHost{windfallGame(t, 2)}
	w, h := e.Size()
	for _, c := range [][3]int{{2, 2, 1}, {1, 25, 0}, {25, 1, 0}, {w - 2, 25, 0}, {25, h - 2, 0}, {w - 3, h - 3, 1}} {
		e.g.setTerrain(c[0], c[1], terrain.Soil)
		if got := e.MeteorFree(c[0], c[1]); got != (c[2] == 1) {
			t.Errorf("inset (%d,%d): MeteorFree %v", c[0], c[1], got)
		}
	}
}

// TestMeteorFreeRefusesBesideABuildingOrCreature: a non-blocking building
// (a door, which the rock would turn), an animal or a visitor (listed with
// the raiders) within 1 of a tile keeps the meteor off it.
func TestMeteorFreeRefusesBesideABuildingOrCreature(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name  string
		place func(t *testing.T, g *Game, x, y int)
	}{
		{"door", func(t *testing.T, g *Game, x, y int) {
			buildNow(t, g, construction.Door, item.Wood, construction.South, x, y-1)
		}},
		{"animal", func(t *testing.T, g *Game, x, y int) {
			if _, err := g.spawnAnimal(animal.Chicken, x+1, y+1); err != nil {
				t.Fatal(err)
			}
		}},
		{"visitor", func(t *testing.T, g *Game, x, y int) {
			if g.spawnVisitor(0, nil, x-1, y, uint64(g.clock.Now())) == 0 {
				t.Fatal("fixture: no visitor")
			}
		}},
	} {
		g, x0, y0, _ := meteorScene(t)
		x, y := x0+12, y0+3
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				g.setTerrain(x+dx, y+dy, terrain.Soil)
			}
		}
		if !(windfallHost{g}).MeteorFree(x, y) {
			t.Fatalf("%s: fixture: (%d,%d) is open ground", c.name, x, y)
		}
		c.place(t, g, x, y)
		if (windfallHost{g}).MeteorFree(x, y) {
			t.Errorf("%s within 1: the meteor may land on (%d,%d)", c.name, x, y)
		}
	}
}

// TestSupplyDropForbiddenStacks: 2-3 forbidden stacks of distinct loot
// kinds, each within its range, land near home with one letter.
func TestSupplyDropForbiddenStacks(t *testing.T) {
	t.Parallel()
	for _, seed := range []uint64{2, 7, 11} {
		g := windfallGame(t, seed)
		old := slices.Clone(g.itemIDs)
		if !g.FireIncident("supply_drop") {
			t.Fatalf("seed %d: the supply drop did not fire", seed)
		}
		hx, hy := g.home()
		qty := map[item.Kind]int{}
		for _, id := range g.itemIDs {
			s, ok := g.stacks.Get(id)
			if !ok || slices.Contains(old, id) {
				continue
			}
			if !s.Forbidden {
				t.Fatalf("seed %d: %v stack not forbidden", seed, s.Kind)
			}
			if dx, dy := s.X-hx, s.Y-hy; dx*dx+dy*dy > podRadius*podRadius {
				t.Fatalf("seed %d: stack at (%d,%d), more than %d from home (%d,%d)", seed, s.X, s.Y, podRadius, hx, hy)
			}
			qty[s.Kind] += s.Qty
		}
		if n := len(qty); n < incident.SupplyStacksMin || n > incident.SupplyStacksMax {
			t.Fatalf("seed %d: %d kinds, want %d-%d: %v", seed, n, incident.SupplyStacksMin, incident.SupplyStacksMax, qty)
		}
		for k, q := range qty {
			i := slices.IndexFunc(incident.SupplyLoot[:], func(s incident.Supply) bool { return s.Kind == k })
			if i < 0 || q < incident.SupplyLoot[i].Min || q > incident.SupplyLoot[i].Max {
				t.Fatalf("seed %d: %d of %v is not loot in range", seed, q, k)
			}
		}
		if l := titledLetters(g, "Supply drop"); len(l) != 1 {
			t.Fatalf("seed %d: one supply letter, got %d", seed, len(l))
		}
	}
}

// thoughtShown reports whether colonist id has thought k, in the game's
// thoughts and in the read model.
func thoughtShown(g *Game, id entity.EntityID, k mood.Kind) (inGame, inView bool) {
	inGame = slices.ContainsFunc(g.thoughts(id), func(t mood.Thought) bool { return t.Kind == k })
	for _, pv := range g.Snapshot() {
		if pv.ID == id {
			inView = slices.ContainsFunc(pv.Thoughts, func(t ThoughtView) bool { return t.Label == mood.DefOf(k).Label })
		}
	}
	return inGame, inView
}

// songEverywhere reports whether every colonist has the sky song's
// thought in the game and the read model.
func songEverywhere(t *testing.T, g *Game, want bool) {
	t.Helper()
	for _, id := range g.pawnIDs {
		if inGame, inView := thoughtShown(g, id, mood.SkySong); inGame != want || inView != want {
			t.Fatalf("tick %d, colonist %d: sky song thought %v (view %v), want %v", g.clock.Now(), id, inGame, inView, want)
		}
	}
}

// TestSkySongMoodAndView: the song lasts 1.75-2.25 days, lifts every
// colonist and shows its end in the read model until the end tick.
func TestSkySongMoodAndView(t *testing.T) {
	t.Parallel()
	g := windfallGame(t, 2)
	now := uint64(g.clock.Now())
	if !g.FireIncident("sky_song") {
		t.Fatal("the sky song did not fire")
	}
	until := g.incidents.SongUntil
	if d := until - now; d < incident.SongTicks(0) || d > incident.SongTicks(incident.SongSpan()) {
		t.Fatalf("the song lasts %d ticks, want %d-%d", d, incident.SongTicks(0), incident.SongTicks(incident.SongSpan()))
	}
	songEverywhere(t, g, true)
	if w := g.World(); w.SkySongEnds != until {
		t.Fatalf("view: song ends %d, want %d", w.SkySongEnds, until)
	}
	if l := titledLetters(g, "Sky song"); len(l) != 1 || l[0].Kind != "good" {
		t.Fatalf("one good letter: %+v", l)
	}
	g.clock.Advance(int(until - 1 - now))
	songEverywhere(t, g, true)
	if w := g.World(); w.SkySongEnds != until {
		t.Fatalf("view at the last tick: %d", w.SkySongEnds)
	}
	g.clock.Advance(1)
	songEverywhere(t, g, false)
	if w := g.World(); w.SkySongEnds != 0 {
		t.Fatalf("view after the song: %d", w.SkySongEnds)
	}
}

// TestSkySongNoFlap: a short song's thought turns on once and off once,
// sampled every 60 ticks to 600 past its end; a second song while it
// plays is refused and the end stays.
func TestSkySongNoFlap(t *testing.T) {
	t.Parallel()
	g := windfallGame(t, 2)
	id := g.pawnIDs[0]
	on := func() bool { in, _ := thoughtShown(g, id, mood.SkySong); return in }
	last, changes := on(), 0
	now := uint64(g.clock.Now())
	if last || !g.incidents.StartSong(now, 2*clock.TicksPerHour) {
		t.Fatal("fixture: a quiet sky, then a two-hour song")
	}
	until := g.incidents.SongUntil
	if g.FireIncident("sky_song") || g.incidents.SongUntil != until {
		t.Fatal("a second song while one plays")
	}
	for at := now; at <= until+600; {
		if v := on(); v != last {
			last, changes = v, changes+1
		}
		if at == until-1 && !last || at == until && last {
			t.Fatalf("tick %d (end %d): song thought %v", at, until, last)
		}
		next := at + 60
		if at < until-1 && next > until-1 {
			next = until - 1
		} else if at == until-1 {
			next = until
		}
		g.Step(int(next - at))
		at = next
	}
	if changes != 2 || last {
		t.Fatalf("the song thought changed %d times (ends on %v), want twice: on, then off", changes, last)
	}
}

// TestSkySongSurvivesSave: a save mid-song keeps its end and the thought.
func TestSkySongSurvivesSave(t *testing.T) {
	t.Parallel()
	g := windfallGame(t, 2)
	if !g.FireIncident("sky_song") {
		t.Fatal("the sky song did not fire")
	}
	g.Step(clock.TicksPerHour)
	h := saveLoad(t, g)
	if h.incidents.SongUntil != g.incidents.SongUntil || h.incidents.SongUntil == 0 {
		t.Fatalf("song end %d after load, want %d", h.incidents.SongUntil, g.incidents.SongUntil)
	}
	songEverywhere(t, h, true)
	if h.World().SkySongEnds != g.incidents.SongUntil {
		t.Fatal("the loaded view shows the song's end")
	}
}

// windfallKeys is the four windfalls' catalogue keys.
var windfallKeys = [...]string{"escape_pod_crash", "ore_meteor", "supply_drop", "sky_song"}

// TestWindfallDeterministic: two games of one seed firing the four
// windfalls hash the same after each.
func TestWindfallDeterministic(t *testing.T) {
	t.Parallel()
	a, b := windfallGame(t, 9), windfallGame(t, 9)
	for _, k := range windfallKeys {
		if fa, fb := a.FireIncident(k), b.FireIncident(k); !fa || !fb {
			t.Fatalf("%s did not fire (%v, %v)", k, fa, fb)
		}
		a.Step(60)
		b.Step(60)
		if ha, hb := a.StateHash(), b.StateHash(); ha != hb {
			t.Fatalf("after %s: hashes %x and %x", k, ha, hb)
		}
	}
}

// TestWindfallFromStoryteller: with every other nature event on cooldown,
// the storyteller's own roll picks and fires each windfall.
func TestWindfallFromStoryteller(t *testing.T) {
	t.Parallel()
	for _, key := range windfallKeys {
		g := windfallGame(t, 2)
		book := g.sk.content.Incidents
		want, ok := book.ByName(key)
		if !ok {
			t.Fatalf("%s: not in the catalogue", key)
		}
		now := uint64(g.clock.Now())
		rn := g.rel(now)
		if rn/clock.TicksPerDay < uint64(want.MinDay) {
			t.Fatalf("fixture: day %d is before %s's day %d", rn/clock.TicksPerDay, key, want.MinDay)
		}
		for id := incident.ID(0); int(id) < book.Len(); id++ {
			if d := book.DefOf(id); d.Category == incident.Nature && id != want.ID {
				g.incidents.Fired(id, rn) // on cooldown
			}
		}
		odds := natureOdds(g)
		if pool := book.Total(incident.Nature, g.incidents, rn, g.biome, odds, func(id incident.ID) bool { return g.incidentFits(id, now) }); pool != want.WeightOdds(odds) || pool == 0 {
			t.Fatalf("%s: the nature pool weighs %d, want only %s's %d", key, pool, key, want.WeightOdds(odds))
		}
		g.rollNature(now)
		if at, fired := g.incidents.Last[want.ID]; !fired || at != rn {
			t.Fatalf("%s: the roll did not fire it (last %v)", key, g.incidents.Last[want.ID])
		}
		if len(titledLetters(g, want.Name)) != 1 {
			t.Fatalf("%s: no %q letter", key, want.Name)
		}
	}
}
