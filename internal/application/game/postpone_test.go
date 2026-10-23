package game

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/research"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/terrain"
)

// Postponed work (SIM-027).

// postWorker is a rested colonist doing only the given work, in order of
// priority.
func postWorker(t *testing.T, g *Game, x, y int, name string, types ...work.Type) entity.EntityID {
	t.Helper()
	id, err := g.SpawnNamedPawn(x, y, name)
	if err != nil {
		t.Fatal(err)
	}
	g.SetNeeds(id, needs.Full, needs.Full)
	onlyWork(g, id)
	for i, wt := range types {
		g.setPriority(id, wt, i+1)
	}
	return id
}

// readyWall places a wooden wall frame with its wood delivered.
func readyWall(t *testing.T, g *Game, x, y int) entity.EntityID {
	t.Helper()
	g.placeFrameStuff(construction.Wall, item.Wood, construction.South, x, y)
	fid := g.layerFrameAt(x, y, construction.LayerObject)
	fr, ok := g.frames.Get(fid)
	if !ok {
		t.Fatalf("no frame at %d,%d", x, y)
	}
	fr.Wood = fr.Need
	g.frames.Set(fid, fr)
	return fid
}

// rockRing walls (x, y) in with rock on its eight neighbours.
func rockRing(g *Game, x, y int) {
	for _, d := range stepDirs {
		g.setTerrain(x+d[0], y+d[1], terrain.Rock)
	}
}

// doorwayMap is a map with a rock wall down x=8 and a one-wide doorway
// at (8, 5).
func doorwayMap() *Game {
	g := New(Config{Seed: 1, Width: 20, Height: 12})
	for y := range 12 {
		if y != 5 {
			g.setTerrain(8, y, terrain.Rock)
		}
	}
	return g
}

func frameGone(g *Game, fid entity.EntityID) bool {
	_, ok := g.frames.Get(fid)
	return !ok
}

// stepFed steps n ticks keeping the pawns fed and rested (and any animal
// among them held in place: holdWild).
func stepFed(g *Game, n int, pawns ...entity.EntityID) {
	for range n {
		for _, p := range pawns {
			if g.isAnimal(p) {
				holdWild(g, p)
				continue
			}
			g.SetNeeds(p, needs.Full, needs.Full)
		}
		g.Step(1)
	}
}

// doorBlocker spawns a wild animal standing in a doorway. Colonists walk
// through each other (PLAY-010), so the creature that stands still, won't
// make way and leaves no way round is a wild one held for a handler (an
// off-map entity with a taming job); stepFed keeps it held.
func doorBlocker(t *testing.T, g *Game, x, y int) entity.EntityID {
	t.Helper()
	id := wildAt(t, g, x, y)
	handler := g.entities.Next()
	g.jobs.Set(handler, job.Record{Kind: job.Tame, State: job.Do, Target: id})
	if !g.reserve.TryEntity(handler, id) {
		t.Fatal("handler reservation")
	}
	holdWild(g, id)
	return id
}

// holdWild keeps wild animal id fed and rested; held for its handler, it
// stands still and never makes way.
func holdWild(g *Game, id entity.EntityID) {
	a, _ := g.animals.Get(id)
	a.Food, a.Rest = animal.Full, animal.Full
	g.animals.Set(id, a)
}

// firstActive steps up to n ticks until pawn id has an active job of kind
// k; it returns the tick (-1: never).
func firstActive(g *Game, id entity.EntityID, n int, k job.Kind) int {
	for i := 1; i <= n; i++ {
		stepFed(g, 1, id)
		if j, _ := g.jobs.Get(id); j.Kind == k && j.Active() {
			return i
		}
	}
	return -1
}

func messaged(g *Game, part string) bool {
	for _, m := range g.messages {
		if strings.Contains(m.Text, part) {
			return true
		}
	}
	return false
}

func TestBlockedJobIsPostponed(t *testing.T) {
	g := doorwayMap()
	fid := readyWall(t, g, 12, 5)
	bob := postWorker(t, g, 3, 5, "Bob", work.Construct, work.Haul)
	d := doorBlocker(t, g, 8, 5) // stands in the doorway
	// other work on Bob's side: two loads for a stockpile
	g.SpawnStack(item.GraniteBlocks, 20, 2, 10)
	g.SpawnStack(item.Wood, 20, 6, 10)
	g.paintZone(zone.KindStockpile, zone.Zone{Food: true, Wood: true, Stone: true}, 1, 1, 2, 2)

	failedAt, still, longest := -1, 0, 0
	last := pos(g, bob)
	for tick := 1; tick <= 1000; tick++ {
		stepFed(g, 1, bob, d)
		j, _ := g.jobs.Get(bob)
		if p := pos(g, bob); p == last && j.State == job.Goto {
			still++
			longest = max(longest, still)
		} else {
			still, last = 0, p
		}
		if failedAt < 0 && g.postponedFor(bob, job.Construct, fid, 0, 0) {
			failedAt = tick
		}
		if failedAt > 0 && tick > failedAt && j.Kind == job.Construct {
			t.Fatalf("tick %d: Bob picked the blocked frame again %d ticks after giving up", tick, tick-failedAt)
		}
	}
	if failedAt < 0 || failedAt > 400 {
		t.Fatalf("Bob gave up on the blocked frame at tick %d, want within ~2 s of reaching the doorway", failedAt)
	}
	if longest > 200 {
		t.Fatalf("Bob stood still %d ticks on a walk, want at most 200", longest)
	}
	if lines := pawnView(g, bob).Postponed; len(lines) == 0 || !strings.Contains(lines[0], "build wall: blocked, retry in") {
		t.Fatalf("view lines %q, want the postponed wall", lines)
	}
	// the blocker leaves; once the retry passes Bob builds the wall
	g.positions.Set(d, Position{X: 16, Y: 10})
	for i := 0; i < 3000 && !frameGone(g, fid); i++ {
		stepFed(g, 1, bob, d)
	}
	if !frameGone(g, fid) {
		t.Fatal("the wall was never built after the doorway cleared")
	}
}

func TestUnreachableFrameDoesNotHideReachableOnes(t *testing.T) {
	t.Run("frames", func(t *testing.T) {
		g := New(Config{Seed: 1, Width: 24, Height: 12})
		near := readyWall(t, g, 5, 5)
		rockRing(g, 5, 5)
		far := readyWall(t, g, 20, 5)
		bob := postWorker(t, g, 3, 8, "Bob", work.Construct)
		// without postponing, Bob only got to it once idle wandering
		// happened to bring him nearer to it (~1100 ticks)
		if at := firstActive(g, bob, 300, job.Construct); at < 0 {
			t.Fatal("Bob didn't start on the reachable frame: the walled-in one hid it")
		}
		for i := 0; i < 1500 && !frameGone(g, far); i++ {
			stepFed(g, 1, bob)
		}
		if !frameGone(g, far) {
			t.Fatal("the reachable frame was never built: the walled-in one hid it")
		}
		if frameGone(g, near) {
			t.Fatal("the walled-in frame got built")
		}
	})
	t.Run("mining", func(t *testing.T) {
		g := New(Config{Seed: 1, Width: 24, Height: 12})
		g.setTerrain(5, 5, terrain.Rock)
		rockRing(g, 5, 5)
		g.setTerrain(20, 5, terrain.Rock)
		g.markMine(5, 5, 5, 5, true)   // walled in by undesignated rock
		g.markMine(20, 5, 20, 5, true) // farther, reachable
		bob := postWorker(t, g, 3, 8, "Bob", work.Mine)
		if at := firstActive(g, bob, 300, job.Mine); at < 0 {
			t.Fatal("Bob didn't start on the reachable rock: the walled-in one hid it")
		}
		for i := 0; i < 2000 && g.terr.At(20, 5) == terrain.Rock; i++ {
			stepFed(g, 1, bob)
		}
		if g.terr.At(20, 5) == terrain.Rock {
			t.Fatal("the reachable rock was never mined: the walled-in one hid it")
		}
	})
	t.Run("research", func(t *testing.T) {
		g := New(Config{Seed: 1, Width: 30, Height: 14})
		g.research = research.Progress{}
		if !g.BuildNow("research bench", "wood", "south", 4, 4) || !g.BuildNow("research bench", "wood", "south", 25, 4) {
			t.Fatal("build research benches")
		}
		near := g.layerBldgAt(4, 4, construction.LayerObject)
		b, _ := g.bldgs.Get(near)
		cells, spot, _ := construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
		cells = append(cells, spot)
		for _, c := range cells { // wall the near bench and its spot in
			for _, d := range stepDirs {
				x, y := c[0]+d[0], c[1]+d[1]
				in := false
				for _, o := range cells {
					in = in || o[0] == x && o[1] == y
				}
				if !in {
					g.setTerrain(x, y, terrain.Rock)
				}
			}
		}
		g.setResearch(research.Traps)
		bob := postWorker(t, g, 4, 12, "Bob", work.Research)
		far := g.layerBldgAt(25, 4, construction.LayerObject)
		studied := false
		for i := 0; i < 300 && !studied; i++ {
			stepFed(g, 1, bob)
			j, _ := g.jobs.Get(bob)
			studied = j.Kind == job.Research && j.Target == far && j.Active()
		}
		if !studied {
			t.Fatal("Bob never went to the reachable research bench")
		}
	})
}

func TestUnreachableRetriedAfterWallRemoved(t *testing.T) {
	g := New(Config{Seed: 1, Width: 20, Height: 12})
	fid := readyWall(t, g, 5, 5)
	rockRing(g, 5, 5)
	bob := postWorker(t, g, 12, 8, "Bob", work.Construct)
	stepFed(g, 5, bob)
	if !g.postponedFor(bob, job.Construct, fid, 0, 0) {
		t.Fatal("the walled-in frame should be put off as out of reach")
	}
	if lines := pawnView(g, bob).Postponed; len(lines) == 0 || lines[0] != "build wall: can't reach" {
		t.Fatalf("view lines %q, want build wall: can't reach first", lines)
	}
	g.setTerrain(6, 5, terrain.Stone) // a way in
	if g.postponedFor(bob, job.Construct, fid, 0, 0) {
		t.Fatal("a walkability change should clear out-of-reach postponements")
	}
	for i := 0; i < 1000 && !frameGone(g, fid); i++ { // well under the hour
		stepFed(g, 1, bob)
	}
	if !frameGone(g, fid) {
		t.Fatal("the frame was not built once a way in opened")
	}
}

func TestWorkingPawnShufflesAside(t *testing.T) {
	// a 1-wide corridor along y=5 with a bay at (3,4)
	corridor := func() *Game {
		g := occTestGame()
		for x := 1; x < 12; x++ {
			g.grid.SetWalkable(x, 4, x == 3)
			g.grid.SetWalkable(x, 6, false)
		}
		return g
	}
	for _, k := range []job.Kind{job.Construct, job.Clean} {
		rec := job.Record{Kind: k, State: job.Do, Target: 999, TX: 3, TY: 5, WorkLeft: 5000}
		if k == job.Clean {
			rec.Target, rec.DestX, rec.DestY = 0, 3, 5
		}
		// PLAY-010: a colonist walks straight through a worker, who keeps
		// its job and its cell
		g := corridor()
		a := mustPawn(t, g, 1, 5)
		b := mustPawn(t, g, 3, 5)
		walkTo(g, a, job.Move, straight(1, 10, 5)...)
		g.jobs.Set(b, rec)
		if blocked := runMoves(g, 300, a); blocked != 0 {
			t.Fatalf("%v: walker held up %d ticks by a worker", k, blocked)
		}
		if pos(g, a) != (Position{10, 5}) || pos(g, b) != (Position{3, 5}) {
			t.Fatalf("%v: walker at %v, worker at %v", k, pos(g, a), pos(g, b))
		}
		if j, _ := g.jobs.Get(b); j.Kind != k || j.State != job.Do {
			t.Fatalf("%v: the worker lost its job: %s", k, j.Line())
		}

		// a worker in combat (here a live calm window) keeps its cell to
		// itself: a colonist heading for that exact cell waits, and the
		// worker shuffles to a free cell still next to its work
		g = corridor()
		a = mustPawn(t, g, 1, 5)
		b = mustPawn(t, g, 3, 5)
		walkTo(g, a, job.Sleep, straight(1, 3, 5)...)
		g.jobs.Set(b, rec)
		g.calm[b] = uint64(g.clock.Now()) + 5000
		for range 300 {
			g.phaseMovement()
			g.clock.Advance(1)
			if pos(g, a) == pos(g, b) {
				t.Fatalf("%v: walker and worker in combat share a cell", k)
			}
		}
		if pos(g, a) != (Position{3, 5}) {
			t.Fatalf("%v: walker at %v, want (3,5): the worker should shuffle aside", k, pos(g, a))
		}
		j, _ := g.jobs.Get(b)
		if j.Kind != k || j.State != job.Do {
			t.Fatalf("%v: the worker lost its job: %s", k, j.Line())
		}
		if p := pos(g, b); pathing.Dist(p.X, p.Y, 3, 5) > 1 {
			t.Fatalf("%v: worker at %v, no longer next to its work at (3,5)", k, p)
		}
	}
}

func TestBlockedForcedOrderIsPostponedNotLost(t *testing.T) {
	setup := func(t *testing.T) (*Game, entity.EntityID, entity.EntityID, entity.EntityID) {
		g := doorwayMap()
		fid := readyWall(t, g, 12, 5)
		bob := postWorker(t, g, 3, 5, "Bob", work.Construct, work.Haul)
		d := doorBlocker(t, g, 8, 5)
		g.Enqueue(command.Command{Kind: command.Prioritize, Pawn: uint64(bob), X: 12, Y: 5,
			Order: command.OrderBuild, Layer: uint8(construction.LayerObject)})
		return g, fid, bob, d
	}
	t.Run("retried", func(t *testing.T) {
		g, fid, bob, d := setup(t)
		stepFed(g, 1, bob, d)
		if j, _ := g.jobs.Get(bob); !j.Forced || j.Kind != job.Construct {
			t.Fatalf("the order did not start: %s", j.Line())
		}
		requeued := false
		for i := 0; i < 600 && !requeued; i++ {
			stepFed(g, 1, bob, d)
			q := g.orderQueue[bob]
			requeued = len(q) == 1 && q[0].retryAt > uint64(g.clock.Now()) && q[0].tries == 1
		}
		if !requeued {
			t.Fatalf("the blocked order should go back on the queue with a retry tick: %+v", g.orderQueue[bob])
		}
		g.positions.Set(d, Position{X: 16, Y: 10})
		for i := 0; i < 2500 && !frameGone(g, fid); i++ {
			stepFed(g, 1, bob, d)
		}
		if !frameGone(g, fid) {
			t.Fatal("the postponed order was lost: the wall was never built")
		}
	})
	t.Run("three strikes", func(t *testing.T) {
		g, fid, bob, d := setup(t)
		for i := 0; i < 4000 && !messaged(g, "couldn't get through"); i++ {
			stepFed(g, 1, bob, d)
		}
		if !messaged(g, "Bob couldn't get through to prioritize building wall.") {
			t.Fatalf("no give-up message after %d blocked tries", orderBlockedTries)
		}
		if len(g.orderQueue[bob]) != 0 || frameGone(g, fid) {
			t.Fatal("the dropped order should leave the queue (and the wall unbuilt)")
		}
	})
}

func TestPostponeSaveRoundTrip(t *testing.T) {
	g := New(Config{Seed: 1, Width: 20, Height: 12})
	fid := readyWall(t, g, 5, 5)
	bob := postWorker(t, g, 12, 8, "Bob", work.Construct)
	g.postpone(postKey{Pawn: bob, Kind: job.Construct, Target: fid}, job.Blocked)
	g.postponeTile(bob, job.Mine, 3, 4)
	c := command.Command{Kind: command.MovePawn, Pawn: uint64(bob), X: 9, Y: 9}
	g.orderQueue[bob] = []queuedOrder{{cmd: c, label: "Go to (9, 9)", retryAt: 1234, tries: 2}}
	g.postponed.orders[bob] = queuedOrder{cmd: c, label: "Go to (9, 9)", tries: 1}
	var buf bytes.Buffer
	if err := g.Save(&buf, "postpone"); err != nil {
		t.Fatal(err)
	}
	h, _, err := Load(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := h.exportPostponed(), g.exportPostponed(); !reflect.DeepEqual(got, want) || len(got) != 2 {
		t.Fatalf("postponed after load %+v, want %+v", got, want)
	}
	if !reflect.DeepEqual(h.orderQueue[bob], g.orderQueue[bob]) {
		t.Fatalf("queue after load %+v, want %+v", h.orderQueue[bob], g.orderQueue[bob])
	}
	if !reflect.DeepEqual(h.postponed.orders, g.postponed.orders) {
		t.Fatalf("followed orders after load %+v, want %+v", h.postponed.orders, g.postponed.orders)
	}
	if !h.postponedFor(bob, job.Construct, fid, 0, 0) || !h.postponedFor(bob, job.Mine, 0, 3, 4) {
		t.Fatal("the loaded game forgot what was put off")
	}
}

func TestPostponedLinesInView(t *testing.T) {
	g := New(Config{Seed: 1, Width: 20, Height: 12})
	fid := readyWall(t, g, 5, 5)
	bob := postWorker(t, g, 12, 8, "Bob")
	g.postpone(postKey{Pawn: bob, Kind: job.Construct, Target: fid}, job.Blocked)
	g.postponeTile(bob, job.Mine, 3, 4)
	sid, _ := g.SpawnStack(item.Wood, 5, 1, 1)
	g.postpone(postKey{Pawn: bob, Kind: job.Haul, Target: sid}, job.Unreachable)
	g.postponeTile(bob, job.Clean, 2, 2)
	got := pawnView(g, bob).Postponed
	// soonest retry first, then by job kind; at most three
	want := []string{"build wall: blocked, retry in 15 s", "haul wood: can't reach", "mine rock: can't reach"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("lines %q, want %q", got, want)
	}
	g.Step(300)
	if got := pawnView(g, bob).Postponed; got[0] != "build wall: blocked, retry in 10 s" {
		t.Fatalf("after 5 s: %q", got)
	}
	g.Step(600)
	if got := pawnView(g, bob).Postponed; len(got) != 3 || strings.Contains(got[0], "build wall") {
		t.Fatalf("an expired postponement still shows: %q", got)
	}
}
