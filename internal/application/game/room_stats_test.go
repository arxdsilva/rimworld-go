package game

import (
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/filth"
	"rimworld-go/internal/domain/world/room"
)

// BED-2: vanilla room stats, roles and room thoughts, whole game.

// statBedroom is a 4x4 walled room (interior 3..6) with a bed at (3,3); it
// returns the bed.
func statBedroom(t *testing.T, g *Game) entity.EntityID {
	t.Helper()
	box(t, g, 2, 2, 7, 7, [2]int{4, 7})
	placeBuilt(t, g, construction.Bed, construction.South, 3, 3)
	return g.layerBldgAt(3, 3, construction.LayerObject)
}

// furnish floors the 4x4 interior and adds an end table and a dresser.
func furnish(t *testing.T, g *Game) {
	t.Helper()
	for y := 3; y <= 6; y++ {
		for x := 3; x <= 6; x++ {
			placeBuilt(t, g, construction.Floor, construction.South, x, y)
		}
	}
	placeBuilt(t, g, construction.EndTable, construction.South, 4, 3)
	placeBuilt(t, g, construction.Dresser, construction.South, 5, 6)
}

// gild lays qty gold in one stack at (6, 5), inside the 4x4 room.
func gild(t *testing.T, g *Game, qty int) {
	t.Helper()
	if _, err := g.SpawnStack(item.Gold, qty, 6, 5); err != nil {
		t.Fatal(err)
	}
}

func roomView(t *testing.T, g *Game, x, y int) RoomView {
	t.Helper()
	rv, ok := g.RoomInfo(x, y)
	if !ok {
		t.Fatalf("no room at %d,%d", x, y)
	}
	return rv
}

// memValue is id's remembered thought k and its mood value.
func memValue(g *Game, id entity.EntityID, k mood.Kind) (int, bool) {
	for _, th := range g.thoughts(id) {
		if th.Kind == k {
			return th.Value, true
		}
	}
	return 0, false
}

func wakeIn(g *Game, id, bed entity.EntityID) {
	g.sleptAt[id] = bed
	g.rest.Waking.After(id)
}

func TestFurnishedBedroomScoresHigher(t *testing.T) {
	t.Parallel()
	g := dirtGame(t)
	statBedroom(t, g)
	bare := roomView(t, g, 4, 4)
	if bare.Role != "bedroom" || bare.Score != bare.Impressiveness/10 || bare.Label != room.ImpressivenessLabel(bare.Impressiveness) ||
		bare.Label != "awful" || bare.Space != room.Space(16, 2) || bare.SpaceLabel != "rather tight" {
		t.Fatalf("bare bedroom %+v", bare)
	}
	if bare.Beauty >= 0 || bare.CleanStat != -100 || bare.Cleanliness != "dirty" || bare.BeautyLabel != room.BeautyLabel(bare.Beauty) {
		t.Fatalf("bare soil is ugly and dirty: %+v", bare)
	}
	furnish(t, g)
	fine := roomView(t, g, 4, 4)
	if fine.Impressiveness <= bare.Impressiveness || fine.Beauty <= bare.Beauty || fine.Wealth <= bare.Wealth ||
		fine.Space >= bare.Space || fine.CleanStat != 0 || fine.Role != "bedroom" {
		t.Fatalf("furnished %+v, bare %+v", fine, bare)
	}
	gild(t, g, 400) // items lying in the room count as its wealth
	rich := roomView(t, g, 4, 4)
	if rich.Wealth != fine.Wealth+400*item.Gold.Value() || rich.Impressiveness <= fine.Impressiveness || rich.WealthLabel != room.WealthLabel(rich.Wealth) {
		t.Fatalf("gilded %+v, furnished %+v", rich, fine)
	}
}

func TestFilthLowersBeautyAndCleanliness(t *testing.T) {
	t.Parallel()
	g := dirtGame(t)
	statBedroom(t, g)
	furnish(t, g)
	clean := roomView(t, g, 4, 4)
	for y := 3; y <= 6; y++ {
		for x := 3; x <= 6; x++ {
			g.addDirt(tileIdx(g, x, y), filth.Full/2)
		}
	}
	dirty := roomView(t, g, 4, 4)
	if dirty.Beauty >= clean.Beauty || dirty.CleanStat >= clean.CleanStat || dirty.Cleanliness == clean.Cleanliness ||
		dirty.FilthPenalty <= 0 || dirty.FilthPenalty != (clean.Impressiveness-dirty.Impressiveness)/10 {
		t.Fatalf("dirty %+v, clean %+v", dirty, clean)
	}
	if dirty.CleanStat != -500/2 {
		t.Fatalf("half a filth per cell = %d hundredths", dirty.CleanStat)
	}
}

func TestRoomRoles(t *testing.T) {
	t.Parallel()
	g := dirtGame(t)
	bed := statBedroom(t, g)
	setBed := func(f func(*construction.Building)) {
		b, _ := g.bldgs.Get(bed)
		f(&b)
		g.bldgs.Set(bed, b)
	}
	// flags flip with no rebuild: the role reads them live
	setBed(func(b *construction.Building) { b.Medical = true })
	if r := roomView(t, g, 4, 4); r.Role != "hospital" {
		t.Fatalf("a medical-bed room is %q", r.Role)
	}
	setBed(func(b *construction.Building) { b.ForPrisoners = true })
	if r := roomView(t, g, 4, 4); r.Role != "prison cell" {
		t.Fatalf("a prisoner-bed room is %q", r.Role)
	}
	// kitchen and recreation room in two more boxes
	box(t, g, 9, 2, 14, 7, [2]int{11, 7})
	placeBuilt(t, g, construction.Stove, construction.South, 10, 3)
	if r := roomView(t, g, 12, 5); r.Role != "kitchen" {
		t.Fatalf("a stove room is %q", r.Role)
	}
	box(t, g, 16, 2, 21, 7, [2]int{18, 7})
	placeBuilt(t, g, construction.HorseshoePin, construction.South, 18, 4)
	if r := roomView(t, g, 19, 5); r.Role != "recreation room" {
		t.Fatalf("a horseshoe room is %q", r.Role)
	}
}

func TestBedroomThoughtLadder(t *testing.T) {
	t.Parallel()
	g := dirtGame(t)
	bed := statBedroom(t, g)
	id, _ := g.SpawnNamedPawn(3, 3, "Ada")
	wakeIn(g, id, bed)
	if v, ok := memValue(g, id, mood.SleptInBedroom); !ok || v != -2 {
		t.Fatalf("awful bedroom thought %d %v", v, ok)
	}
	furnish(t, g)
	gild(t, g, 600)
	r := roomView(t, g, 4, 4)
	stage := room.ImpressivenessStage(r.Impressiveness)
	want := [room.Stages]int{-2, 0, 1, 2, 3, 4, 5, 6, 7, 8}[stage]
	wakeIn(g, id, bed)
	if v, ok := memValue(g, id, mood.SleptInBedroom); !ok || v != want || stage < 2 {
		t.Fatalf("%s bedroom (%d) thought %d, want %d", r.Label, r.Impressiveness, v, want)
	}
	if g.hasMemory(id, mood.SleptInNiceRoom) || g.hasMemory(id, mood.SleptInAwfulRoom) {
		t.Fatal("the old nice/awful room thoughts are gone")
	}
	// a filthy floor drags it back down
	for y := 3; y <= 6; y++ {
		for x := 3; x <= 6; x++ {
			g.addDirt(tileIdx(g, x, y), filth.Full)
		}
	}
	wakeIn(g, id, bed)
	if v, _ := memValue(g, id, mood.SleptInBedroom); v >= want {
		t.Fatalf("filthy bedroom thought %d (clean %d)", v, want)
	}
}

func TestSharedBedroomIsBarracks(t *testing.T) {
	t.Parallel()
	g := dirtGame(t)
	bed := statBedroom(t, g)
	placeBuilt(t, g, construction.Bed, construction.South, 6, 3)
	other := g.layerBldgAt(6, 3, construction.LayerObject)
	ada, _ := g.SpawnNamedPawn(3, 3, "Ada")
	bo, _ := g.SpawnNamedPawn(6, 3, "Bo")
	for _, c := range [][2]entity.EntityID{{bed, ada}, {other, bo}} {
		b, _ := g.bldgs.Get(c[0])
		b.Claimant = uint64(c[1])
		g.bldgs.Set(c[0], b)
	}
	if r := roomView(t, g, 4, 4); r.Role != "barracks" {
		t.Fatalf("two beds make %q", r.Role)
	}
	wakeIn(g, ada, bed)
	if v, ok := memValue(g, ada, mood.SleptInBarracks); !ok || v != -7 || g.hasMemory(ada, mood.SleptInBedroom) {
		t.Fatalf("awful barracks thought %d %v", v, ok)
	}
}

func TestSharedBedThoughtInGame(t *testing.T) {
	t.Parallel()
	g := dirtGame(t)
	box(t, g, 2, 2, 7, 7, [2]int{4, 7})
	placeBuilt(t, g, construction.DoubleBed, construction.South, 3, 3)
	bed := g.layerBldgAt(3, 3, construction.LayerObject)
	ada, _ := g.SpawnNamedPawn(3, 3, "Ada")
	bo, _ := g.SpawnNamedPawn(4, 3, "Bo")
	b, _ := g.bldgs.Get(bed)
	b.Claimant, b.Claimant2 = uint64(ada), uint64(bo)
	g.bldgs.Set(bed, b)
	wakeIn(g, ada, bed)
	op := g.opinion(ada, bo)
	want := max(-10, min(0, -5+int(op)/2000))
	if v, ok := memValue(g, ada, mood.SharedBed); ok != (want < 0) || v != want {
		t.Fatalf("sharing a bed at opinion %d: %d %v, want %d", op, v, ok, want)
	}
}

// TestRoomThoughtDoesNotFlap: a bedroom whose impressiveness crosses a
// stage boundary every few ticks (filth tracked in and cleaned) leaves
// the thought as it was at waking; only the next waking re-reads it.
func TestRoomThoughtDoesNotFlap(t *testing.T) {
	t.Parallel()
	g := dirtGame(t)
	bed := statBedroom(t, g)
	furnish(t, g)
	id, _ := g.SpawnNamedPawn(3, 3, "Ada")
	wakeIn(g, id, bed)
	first, _ := memValue(g, id, mood.SleptInBedroom)
	clean := roomView(t, g, 4, 4).Label
	crossed, changes := false, 0
	last := first
	for i := range 200 {
		amt := filth.Full
		if i%2 == 1 {
			amt = -filth.Full
		}
		for x := 3; x <= 6; x++ {
			g.addDirt(tileIdx(g, x, 5), amt)
		}
		g.SetNeeds(id, needs.Full, needs.Full)
		g.Step(5)
		crossed = crossed || roomView(t, g, 4, 4).Label != clean
		if v, _ := memValue(g, id, mood.SleptInBedroom); v != last {
			changes++
			last = v
		}
	}
	if !crossed {
		t.Fatal("fixture: the room never crossed a stage boundary")
	}
	if changes != 0 {
		t.Fatalf("the bedroom thought changed %d times without a waking", changes)
	}
}

func TestDiningRoomThought(t *testing.T) {
	t.Parallel()
	g := dirtGame(t)
	box(t, g, 2, 2, 7, 7, [2]int{4, 7})
	for y := 3; y <= 6; y++ {
		for x := 3; x <= 6; x++ {
			placeBuilt(t, g, construction.Floor, construction.South, x, y)
		}
	}
	placeBuilt(t, g, construction.Table, construction.South, 5, 4)
	gild(t, g, 600)
	r := roomView(t, g, 4, 4)
	if r.Role != "dining room" || room.ImpressivenessStage(r.Impressiveness) < 3 {
		t.Fatalf("fixture dining room %+v", r)
	}
	id, _ := g.SpawnNamedPawn(4, 4, "Ada")
	st := item.Stack{Kind: item.Meal, Qty: 1}
	g.afterEat(id, st)
	if v, ok := memValue(g, id, mood.AteInDiningRoom); !ok || v < 2 {
		t.Fatalf("ate in a %s dining room: %d %v", r.Label, v, ok)
	}
}

// TestRoomQualityAllocatesNothing: the stat read a waking or a meal makes
// reuses its scratch.
func TestRoomQualityAllocatesNothing(t *testing.T) {
	g := dirtGame(t)
	statBedroom(t, g)
	furnish(t, g)
	gild(t, g, 100)
	r := g.roomAt(4, 4)
	restRooms{g}.RoomQuality(r)
	if n := testing.AllocsPerRun(50, func() { restRooms{g}.RoomQuality(r) }); n != 0 {
		t.Fatalf("roomQuality allocates %v per read", n)
	}
}
