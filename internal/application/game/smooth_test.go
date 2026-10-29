package game

import (
	"slices"
	"strings"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/geology"
)

// smoothGame is a 14x8 map with a granite rock column at x = 10 and a
// patch of rough granite ground at 4..5, 5; one builder stands at 2, 2.
func smoothGame(t *testing.T) (*Game, entity.EntityID) {
	t.Helper()
	g := New(Config{Seed: 1, Width: 14, Height: 8})
	if !g.SetGround(10, 0, 10, 7, "rock", "granite", "") || !g.SetGround(4, 5, 5, 5, "stone", "granite", "") {
		t.Fatal("set ground")
	}
	id := postWorker(t, g, 2, 2, "Ada", work.Construct)
	return g, id
}

func applyNow(g *Game, c command.Command) { g.Enqueue(c); g.ApplyPending() }

func smoothRect(g *Game, x0, y0, x1, y1 int) {
	applyNow(g, command.Command{Kind: command.DesignateSmooth, X: x0, Y: y0, X2: x1, Y2: y1})
}

func orderLabels(g *Game, id entity.EntityID, x, y int) map[string]OrderOption {
	out := map[string]OrderOption{}
	for _, o := range g.OrdersAt(id, x, y) {
		out[o.Label] = o
	}
	return out
}

func TestSmoothWallAndFloor(t *testing.T) {
	t.Parallel()
	g, id := smoothGame(t)
	smoothRect(g, 10, 2, 10, 2)
	smoothRect(g, 4, 5, 4, 5)
	if s := g.World().Smooths; !slices.Equal(s, [][2]int{{10, 2}, {4, 5}}) {
		t.Fatalf("smooth marks %v", s)
	}
	stepFed(g, TicksPerCell*40+SmoothWallWork+SmoothFloorWork+100, id)
	if g.TerrainAt(10, 2) != "rock_smooth" || g.grid.Walkable(10, 2) || g.stoneAt(10, 2) != geology.Granite {
		t.Fatalf("wall = %s walkable %v stone %v", g.TerrainAt(10, 2), g.grid.Walkable(10, 2), g.stoneAt(10, 2))
	}
	if g.TerrainAt(4, 5) != "stone_smooth" || !g.grid.Walkable(4, 5) || g.stoneAt(4, 5) != geology.Granite {
		t.Fatalf("floor = %s walkable %v stone %v", g.TerrainAt(4, 5), g.grid.Walkable(4, 5), g.stoneAt(4, 5))
	}
	if g.TerrainAt(10, 1) != "rock" || g.TerrainAt(5, 5) != "stone" {
		t.Fatal("only the marked tiles are smoothed")
	}
	if len(g.World().Smooths) != 0 || len(g.reserve.Export()) != 0 {
		t.Fatalf("marks %v claims %v left", g.World().Smooths, g.reserve.Export())
	}
}

func TestSmoothJobViewAndWork(t *testing.T) {
	t.Parallel()
	g, id := smoothGame(t)
	smoothRect(g, 10, 2, 10, 2)
	if firstActive(g, id, TicksPerCell*30, job.Smooth) < 0 {
		t.Fatal("no smoothing job")
	}
	j, _ := g.jobs.Get(id)
	if j.DestX != 10 || j.DestY != 2 || j.TX != 9 || g.workTotal(id, j) != SmoothWallWork {
		t.Fatalf("job %+v total %d", j, g.workTotal(id, j))
	}
	if owner, ok := g.reserve.TileOwner(10, 2); !ok || owner != id {
		t.Fatal("the smoothed tile is reserved")
	}
	for range TicksPerCell * 10 {
		stepFed(g, 1, id)
		if j, _ := g.jobs.Get(id); j.State == job.Do {
			break
		}
	}
	if a := g.actionOf(id, mustJob(g, id)); a != ActionSmooth {
		t.Fatalf("action %q", a)
	}
	if act, x, y := g.acting(id, mustJob(g, id)); act != "smooth" || x != 10 || y != 2 {
		t.Fatalf("acting %q %d,%d", act, x, y)
	}
}

func mustJob(g *Game, id entity.EntityID) job.Record {
	j, _ := g.jobs.Get(id)
	return j
}

func TestSmoothRejectsOreAndBuiltFloor(t *testing.T) {
	t.Parallel()
	g, id := smoothGame(t)
	if !g.SetGround(10, 4, 10, 4, "rock", "granite", "steel") || !g.SetGround(10, 6, 10, 6, "rock_smooth", "granite", "") ||
		!g.SetGround(6, 5, 7, 5, "stone", "granite", "") || !g.SetGround(8, 5, 8, 5, "stone_smooth", "granite", "") {
		t.Fatal("set ground")
	}
	if !g.BuildNow("floor", "wood", "south", 4, 5) || !g.BuildNow("wall", "wood", "south", 6, 5) {
		t.Fatal("build")
	}
	smoothRect(g, 0, 0, 13, 7)
	want := [][2]int{{10, 0}, {10, 1}, {10, 2}, {10, 3}, {5, 5}, {7, 5}, {10, 5}, {10, 7}}
	got := g.World().Smooths
	slices.SortFunc(got, func(a, b [2]int) int { return a[1]*100 + a[0] - b[1]*100 - b[0] })
	slices.SortFunc(want, func(a, b [2]int) int { return a[1]*100 + a[0] - b[1]*100 - b[0] })
	if !slices.Equal(got, want) {
		t.Fatalf("smoothable = %v, want %v", got, want)
	}
	if _, ok := orderLabels(g, id, 10, 4)["Smooth wall"]; ok {
		t.Fatal("ore offered for smoothing")
	}
	if _, ok := orderLabels(g, id, 4, 5)["Smooth floor"]; ok {
		t.Fatal("a built floor offered for smoothing")
	}
	if _, ok := orderLabels(g, id, 10, 6)["Smooth wall"]; ok {
		t.Fatal("a smoothed wall offered again")
	}
	if o, ok := orderLabels(g, id, 10, 3)["Smooth wall"]; !ok || o.Disabled {
		t.Fatalf("smooth wall order %+v %v", o, ok)
	}
	if o, ok := orderLabels(g, id, 7, 5)["Smooth floor"]; !ok || o.Disabled {
		t.Fatalf("smooth floor order %+v %v", o, ok)
	}
	// a builder with construction off is told why
	g.setPriority(id, work.Construct, 0)
	if o := orderLabels(g, id, 7, 5)["Smooth floor"]; !o.Disabled || o.Reason != ReasonNoConstruct {
		t.Fatalf("disabled builder: %+v", o)
	}
}

func TestSmoothForcedOrder(t *testing.T) {
	t.Parallel()
	g, id := smoothGame(t)
	o := orderLabels(g, id, 5, 5)["Smooth floor"]
	if o.Disabled {
		t.Fatalf("order %+v", o)
	}
	g.Enqueue(o.Cmd)
	stepFed(g, 1, id)
	if j, _ := g.jobs.Get(id); j.Kind != job.Smooth || !j.Forced || j.DestX != 5 || j.DestY != 5 || j.TX != 5 || j.TY != 5 {
		t.Fatalf("forced job %+v", j)
	}
	if !slices.Contains(g.World().Smooths, [2]int{5, 5}) {
		t.Fatal("a forced smooth marks the tile")
	}
	stepFed(g, TicksPerCell*20+SmoothFloorWork+20, id)
	if g.TerrainAt(5, 5) != "stone_smooth" {
		t.Fatalf("forced smoothing left %s", g.TerrainAt(5, 5))
	}
}

func TestCancelAndUndesignateSmooth(t *testing.T) {
	t.Parallel()
	g, _ := smoothGame(t)
	smoothRect(g, 10, 0, 10, 7)
	smoothRect(g, 4, 5, 5, 5)
	applyNow(g, command.Command{Kind: command.DesignateMine, X: 10, Y: 0, X2: 10, Y2: 7})
	applyNow(g, command.Command{Kind: command.Undesignate, Order: command.OrderMine, X: 10, Y: 0, X2: 10, Y2: 3})
	if n := len(g.World().Smooths); n != 10 {
		t.Fatalf("unmarking mines dropped smooth marks: %d", n)
	}
	applyNow(g, command.Command{Kind: command.Undesignate, Order: command.OrderSmooth, X: 10, Y: 0, X2: 10, Y2: 7})
	if s := g.World().Smooths; !slices.Equal(s, [][2]int{{4, 5}, {5, 5}}) {
		t.Fatalf("undesignate smooth left %v", s)
	}
	if m := g.World().Mines; len(m) != 4 {
		t.Fatalf("undesignate smooth touched mines: %v", m)
	}
	applyNow(g, command.Command{Kind: command.DesignateCancel, X: 5, Y: 5, X2: 5, Y2: 5})
	if s := g.World().Smooths; !slices.Equal(s, [][2]int{{4, 5}}) {
		t.Fatalf("cancel left %v", s)
	}
}

func TestSmoothPostponedWhenUnreachable(t *testing.T) {
	t.Parallel()
	g, id := smoothGame(t)
	// a rock mass whose centre has no open neighbour, and a stone floor
	// ringed by rock
	if !g.SetGround(5, 0, 7, 2, "rock", "granite", "") || !g.SetGround(1, 4, 3, 6, "rock", "slate", "") ||
		!g.SetGround(2, 5, 2, 5, "stone", "slate", "") {
		t.Fatal("set ground")
	}
	smoothRect(g, 6, 1, 6, 1)
	smoothRect(g, 2, 5, 2, 5)
	stepFed(g, 30, id)
	if !g.postponedFor(id, job.Smooth, 0, 6, 1) || !g.postponedFor(id, job.Smooth, 0, 2, 5) {
		t.Fatal("unreachable smoothing not put off")
	}
	if j, _ := g.jobs.Get(id); j.Kind == job.Smooth {
		t.Fatalf("smoothing an unreachable tile: %+v", j)
	}
	lines := strings.Join(g.postponedLines(id), "|")
	if !strings.Contains(lines, "smooth rock: can't reach") || !strings.Contains(lines, "smooth floor: can't reach") {
		t.Fatalf("postponed lines %q", lines)
	}
	if o := orderLabels(g, id, 6, 1)["Smooth wall"]; !o.Disabled || o.Reason != ReasonUnreachable {
		t.Fatalf("order on a walled-in wall: %+v", o)
	}
}

// TestSmoothNoFlap: one builder, two marked walls. The builder never
// switches target mid-job: it finishes one before it starts the other,
// and a job interrupted halfway keeps its mark and is resumed.
func TestSmoothNoFlap(t *testing.T) {
	t.Parallel()
	g, id := smoothGame(t)
	smoothRect(g, 10, 2, 10, 3)
	var order [][2]int
	for range TicksPerCell*40 + 2*SmoothWallWork + 100 {
		stepFed(g, 1, id)
		j, _ := g.jobs.Get(id)
		if j.Kind != job.Smooth || !j.Active() {
			continue
		}
		cur := [2]int{j.DestX, j.DestY}
		if len(order) > 0 && order[len(order)-1] == cur {
			continue
		}
		if len(order) > 0 {
			prev := order[len(order)-1]
			if g.TerrainAt(prev[0], prev[1]) != "rock_smooth" {
				t.Fatalf("switched from %v to %v before finishing it", prev, cur)
			}
		}
		order = append(order, cur)
	}
	if len(order) != 2 || g.TerrainAt(10, 2) != "rock_smooth" || g.TerrainAt(10, 3) != "rock_smooth" {
		t.Fatalf("targets %v, tiles %s %s", order, g.TerrainAt(10, 2), g.TerrainAt(10, 3))
	}
	// interrupted halfway: the mark stays and the work is done later
	smoothRect(g, 10, 5, 10, 5)
	for range TicksPerCell*40 + SmoothWallWork {
		stepFed(g, 1, id)
		if j, _ := g.jobs.Get(id); j.Kind == job.Smooth && j.State == job.Do && j.WorkLeft < SmoothWallWork/2 {
			break
		}
	}
	g.interrupt(id, job.Interrupted)
	g.setDraft(id, true)
	stepFed(g, 50, id)
	if g.TerrainAt(10, 5) != "rock" || !slices.Contains(g.World().Smooths, [2]int{10, 5}) {
		t.Fatalf("interrupted wall = %s, marks %v", g.TerrainAt(10, 5), g.World().Smooths)
	}
	if _, ok := g.reserve.TileOwner(10, 5); ok {
		t.Fatal("an interrupted smoothing keeps its claim")
	}
	g.setDraft(id, false)
	stepFed(g, TicksPerCell*40+SmoothWallWork+20, id)
	if g.TerrainAt(10, 5) != "rock_smooth" {
		t.Fatalf("interrupted wall not finished: %s", g.TerrainAt(10, 5))
	}
}

func TestSmoothedStoneMakesRoomsFinerAndCounts(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 12, Height: 10})
	// a 3x3 room carved in granite: rock all round, rough stone inside
	if !g.SetGround(2, 2, 6, 6, "rock", "granite", "") || !g.SetGround(3, 3, 5, 5, "stone", "granite", "") {
		t.Fatal("set ground")
	}
	r := func() roomStats {
		g.ensureRooms()
		return g.rooms[g.roomGrid[4*g.Width()+4]]
	}
	score := func() int { _, imp := restRooms{g}.RoomQuality(g.roomGrid[4*g.Width()+4]); return imp }
	before, beforeImp := r(), score()
	if before.contents.Floored != 0 {
		t.Fatalf("rough room %+v", before.contents)
	}
	if !g.SetGround(3, 3, 5, 5, "stone_smooth", "granite", "") || !g.SetGround(2, 2, 6, 2, "rock_smooth", "granite", "") {
		t.Fatal("set ground")
	}
	after, afterImp := r(), score() // BED-2: smooth floor beauty +1 and cleanliness 0 (rough stone -1, -1)
	if after.contents.Floored != 9 || afterImp <= beforeImp {
		t.Fatalf("smoothed room %+v impressiveness %d (was %d)", after.contents, afterImp, beforeImp)
	}
}

func TestSmoothStopsWhenTheWallIsDugOut(t *testing.T) {
	t.Parallel()
	g, id := smoothGame(t)
	smoothRect(g, 10, 2, 10, 2)
	for range TicksPerCell * 30 {
		stepFed(g, 1, id)
		if j, _ := g.jobs.Get(id); j.Kind == job.Smooth && j.State == job.Do {
			break
		}
	}
	g.digRock(10, 2, rockHP) // a sapper breaks through mid-job
	stepFed(g, 5, id)
	if j, _ := g.jobs.Get(id); j.Kind == job.Smooth && j.Active() {
		t.Fatalf("still smoothing a dug-out wall: %+v", j)
	}
	stepFed(g, SmoothFloorWork+50, id)
	if g.TerrainAt(10, 2) != "stone" || len(g.World().Smooths) != 0 {
		t.Fatalf("dug-out tile = %s, marks %v", g.TerrainAt(10, 2), g.World().Smooths)
	}
}
