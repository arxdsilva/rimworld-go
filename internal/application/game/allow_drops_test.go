package game

import (
	"strings"
	"testing"

	"rimworld-go/internal/application/game/craft"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// alGame is a grassless map, its left half (x < 20) the home area.
func alGame(w, h int) *Game {
	g := New(Config{Seed: 1, Width: w, Height: h})
	bareGround(g)
	g.paintHome(0, 0, 19, h-1, true)
	return g
}

// alWolf is a hungry wild wolf at (x, y).
func alWolf(t *testing.T, g *Game, x, y int) entity.EntityID {
	t.Helper()
	w, err := g.SpawnAnimal("timber_wolf", x, y)
	if err != nil {
		t.Fatal(err)
	}
	setAnimal(g, w, func(a *animal.State) { a.Food = 2000 })
	return w
}

// alCorpseStore paints a stockpile that takes animal corpses.
func alCorpseStore(t *testing.T, g *Game, x1, y1, x2, y2 int) {
	t.Helper()
	applyNow(g, command.Command{Kind: command.DesignateStockpile, X: x1, Y: y1, X2: x2, Y2: y2})
	applyNow(g, command.Command{Kind: command.SetStockpileFilter, Target: uint64(zoneViewAt(t, g, x1, y1).ID), Name: "Corpses/Animal", Enabled: true})
}

// alCarcass is the carcass stack of species s at (x, y).
func alCarcass(t *testing.T, g *Game, s animal.Species, x, y int) (entity.EntityID, item.Stack) {
	t.Helper()
	for _, id := range g.itemIDs {
		if st, ok := g.stacks.Get(id); ok && st.Kind == item.CorpseAnimal && st.Animal == uint8(s) && st.X == x && st.Y == y && st.Carrier == 0 {
			return id, st
		}
	}
	t.Fatalf("no carcass at %d,%d", x, y)
	return 0, item.Stack{}
}

// alHold sets wolf to eating carcass c, with a meal long enough to outlast
// the test, and takes its claim the way phaseAnimals does.
func alHold(g *Game, wolf, c entity.EntityID) {
	st, _ := g.stacks.Get(c)
	setAnimal(g, wolf, func(a *animal.State) {
		a.Mode, a.Feed, a.FeedLeft = animal.Feed, animal.FeedTarget{Kind: animal.FeedCarcass, ID: c, X: st.X, Y: st.Y}, 1<<20
		g.syncFeedClaim(wolf, a)
	})
}

// AL-1: a wild predator's kill follows the home-area rule; only a hunter's
// own kill, a colonist's or a tame animal's on its hunt, stays allowed.
func TestWildKillFollowsTheHomeRule(t *testing.T) {
	t.Parallel()
	g := alGame(60, 30)
	shown := func(x, y int) bool {
		for _, iv := range g.World().Items {
			if iv.Kind == "deer corpse" && iv.X == x && iv.Y == y {
				return iv.Forbidden
			}
		}
		t.Fatalf("no deer corpse in the view at %d,%d", x, y)
		return false
	}
	for _, c := range []struct {
		name string
		x, y int
		want bool
	}{{"inside Home", 5, 5, false}, {"outside Home", 40, 5, true}} {
		deer, _ := g.SpawnAnimal("deer", c.x, c.y)
		wolf := alWolf(t, g, c.x+1, c.y)
		killWith(g, deer, wolf)
		if _, st := alCarcass(t, g, animal.Deer, c.x, c.y); st.Forbidden != c.want {
			t.Errorf("a wild kill %s: forbidden %v, want %v", c.name, st.Forbidden, c.want)
		}
		if got := shown(c.x, c.y); got != c.want { // what the inspector reads
			t.Errorf("a wild kill %s: the view shows forbidden %v, want %v", c.name, got, c.want)
		}
	}
	// a colonist's hunt
	ada, _ := g.SpawnNamedPawn(2, 25, "Ada")
	prey, _ := g.SpawnAnimal("deer", 40, 10)
	g.jobs.Set(ada, job.Record{Kind: job.Hunt, Target: prey})
	if !killTaken(g, ada, prey) {
		t.Fatal("a colonist's hunt is taken")
	}
	killWith(g, prey, ada)
	if _, st := alCarcass(t, g, animal.Deer, 40, 10); st.Forbidden {
		t.Fatal("a colonist's hunted kill stays allowed")
	}
	// a tame animal on its hunt, and one that merely fought
	rex := alWolf(t, g, 41, 15)
	g.TameAnimal(rex, "Rex")
	hare, _ := g.SpawnAnimal("deer", 40, 15)
	setAnimal(g, rex, func(a *animal.State) { a.Target, a.HuntSince = hare, 1 })
	killWith(g, hare, rex)
	if _, st := alCarcass(t, g, animal.Deer, 40, 15); st.Forbidden {
		t.Fatal("a tame hunter's kill stays allowed")
	}
	foe, _ := g.SpawnAnimal("deer", 40, 20)
	setAnimal(g, rex, func(a *animal.State) { a.Target, a.HuntSince = foe, 0 })
	if killTaken(g, rex, foe) {
		t.Fatal("a tame animal not on a hunt takes nothing")
	}
	other, _ := g.SpawnAnimal("deer", 45, 20)
	if killTaken(g, ada, other) || killTaken(g, 0, other) {
		t.Fatal("only the hunter's own prey")
	}
}

// AL-2: a wild killer holds the carcass from the moment of the kill, in
// the home area as well.
func TestWildKillerHoldsItsCarcass(t *testing.T) {
	t.Parallel()
	g := alGame(60, 20)
	for _, x := range []int{5, 40} {
		deer, _ := g.SpawnAnimal("deer", x, 5)
		wolf := alWolf(t, g, x+1, 5)
		killWith(g, deer, wolf)
		c, _ := alCarcass(t, g, animal.Deer, x, 5)
		if a := animalOf(g, wolf); a.Mode != animal.Feed || a.Feed.ID != c || a.FeedClaim != c {
			t.Fatalf("x=%d: the wolf feeds on its kill and holds it: %+v", x, a)
		}
		if o, held := g.reserve.EntityOwner(c); !held || o != wolf {
			t.Fatalf("x=%d: the carcass is held by %d (%v), want the wolf %d", x, o, held, wolf)
		}
	}
	// a kill that spills off an occupied tile is held where it lies
	g.dropCorpse(animal.Deer, 10, 12)
	deer, _ := g.SpawnAnimal("deer", 10, 12)
	wolf := alWolf(t, g, 9, 12)
	killWith(g, deer, wolf)
	a := animalOf(g, wolf)
	st, ok := g.stacks.Get(a.Feed.ID)
	if !ok || st.X != a.Feed.X || st.Y != a.Feed.Y || a.FeedClaim != a.Feed.ID {
		t.Fatalf("the meal's spot is the carcass's: %+v, stack %+v", a.Feed, st)
	}
}

// AL-2: one that walks to a carcass already lying there holds it on the
// way and through the meal, without a gap, and lets go when it is eaten.
func TestWildFeederHoldsWithoutFlapping(t *testing.T) {
	t.Parallel()
	g := alGame(60, 20)
	c := g.dropCorpse(animal.Deer, 16, 10)
	wolf := alWolf(t, g, 8, 10)
	holding := false
	for i := 0; i < 2000; i++ {
		g.Step(1)
		if _, there := g.stacks.Get(c); !there {
			break
		}
		a := animalOf(g, wolf)
		o, held := g.reserve.EntityOwner(c)
		if a.Mode == animal.Feed && a.Feed.ID == c && (!held || o != wolf || a.FeedClaim != c) {
			t.Fatalf("tick %d: feeding without the claim: owner %d held %v, %+v", i, o, held, a)
		}
		if holding && !held {
			t.Fatalf("tick %d: the claim dropped mid-meal", i)
		}
		holding = holding || held
	}
	if _, there := g.stacks.Get(c); there || !holding {
		t.Fatalf("the wolf walks over, holds it and eats it (there %v, holding %v)", there, holding)
	}
	g.Step(1)
	if a := animalOf(g, wolf); a.FeedClaim != 0 {
		t.Fatalf("the claim is gone with the carcass: %+v", a)
	}
	if _, held := g.reserve.EntityOwner(c); held {
		t.Fatal("no reservation outlives the carcass")
	}
}

// AL-2: a pack still eats one kill together; when the holder goes, the
// next feeder takes the claim over.
func TestPackSharesAHeldCarcass(t *testing.T) {
	t.Parallel()
	g := alGame(60, 20)
	c := g.dropCorpse(animal.Deer, 30, 10)
	first, second := alWolf(t, g, 29, 10), alWolf(t, g, 31, 10)
	st, _ := g.stacks.Get(c)
	if tg, ok := g.findFeed(&animal.State{Species: animal.TimberWolf}, Position{X: 29, Y: 10}, animal.EcologyOf(animal.TimberWolf).Diet, nil); !ok || tg.ID != c {
		t.Fatalf("a free carcass is on the menu: %+v %v", tg, ok)
	}
	alHold(g, first, c)
	if tg, ok := g.findFeed(&animal.State{Species: animal.TimberWolf}, Position{X: 31, Y: 10}, animal.EcologyOf(animal.TimberWolf).Diet, nil); !ok || tg.ID != c {
		t.Fatalf("a pack mate's carcass is still on the menu: %+v %v", tg, ok)
	}
	setAnimal(g, second, func(a *animal.State) {
		a.Mode, a.Feed, a.FeedLeft = animal.Feed, animal.FeedTarget{Kind: animal.FeedCarcass, ID: c, X: st.X, Y: st.Y}, 1<<20
	})
	g.Step(1)
	if a := animalOf(g, second); a.Mode != animal.Feed || a.Feed.ID != c {
		t.Fatalf("the second wolf feeds on: %+v", a)
	}
	g.removeAnimal(first)
	g.Step(1)
	if o, held := g.reserve.EntityOwner(c); !held || o != second {
		t.Fatalf("the second wolf takes the claim over: owner %d held %v", o, held)
	}
	if a := animalOf(g, second); a.FeedClaim != c {
		t.Fatalf("its record says so: %+v", a)
	}
}

// AL-2: a colonist's claim comes first: a wild feeder neither eats nor
// picks a carcass someone in the colony holds, and doesn't keep trying.
func TestWildFeederLeavesAColonistsCarcass(t *testing.T) {
	t.Parallel()
	g := alGame(60, 20)
	c := g.dropCorpse(animal.Deer, 30, 10)
	wolf := alWolf(t, g, 29, 10)
	const colonist = entity.EntityID(1 << 40)
	alHold(g, wolf, c)
	g.reserve.ReleaseEntity(wolf, c)
	setAnimal(g, wolf, func(a *animal.State) { a.FeedClaim = 0 })
	if !g.reserve.TryEntity(colonist, c) {
		t.Fatal("claim")
	}
	for i := 0; i < 200; i++ {
		setAnimal(g, wolf, func(a *animal.State) { a.Food = 2000 })
		g.Step(1)
		if a := animalOf(g, wolf); a.Mode == animal.Feed && a.Feed.ID == c {
			t.Fatalf("tick %d: the wolf went for a held carcass: %+v", i, a)
		}
		if a := animalOf(g, wolf); a.FeedClaim != 0 {
			t.Fatalf("tick %d: it holds nothing: %+v", i, a)
		}
	}
	if _, there := g.stacks.Get(c); !there {
		t.Fatal("the carcass was eaten from under the colonist")
	}
}

// AL-2: a held carcass is off every work giver, each proved by the same
// giver taking it once the holder is gone.
func TestHeldCarcassIsOffEveryWorkGiver(t *testing.T) {
	t.Parallel()
	setup := func(t *testing.T) (*Game, entity.EntityID, entity.EntityID, entity.EntityID) {
		t.Helper()
		g := alGame(60, 20)
		c := g.dropCorpse(animal.Deer, 10, 10) // the colony's own: the kind a hauler takes
		wolf := alWolf(t, g, 11, 10)
		ada, err := g.SpawnNamedPawn(3, 5, "Ada")
		if err != nil {
			t.Fatal(err)
		}
		g.SetNeeds(ada, needs.Full, needs.Full)
		alHold(g, wolf, c)
		return g, c, wolf, ada
	}
	onJob := func(g *Game, id, c entity.EntityID) bool {
		j, _ := g.jobs.Get(id)
		return j.Target == c && j.Active()
	}
	t.Run("haul", func(t *testing.T) {
		t.Parallel()
		g, c, wolf, ada := setup(t)
		alCorpseStore(t, g, 2, 14, 6, 16)
		if g.assignHaul(ada); onJob(g, ada, c) {
			t.Fatal("a hauler was sent for a held carcass")
		}
		g.removeAnimal(wolf)
		if g.assignHaul(ada); !onJob(g, ada, c) {
			t.Fatal("control: free, it is hauled")
		}
	})
	t.Run("butcher bill", func(t *testing.T) {
		t.Parallel()
		g, c, wolf, ada := setup(t)
		placeBuilt(t, g, construction.ButcherTable, construction.South, 6, 3)
		m, _ := g.labor.Get(ada)
		if g.craft.Shop.Assign(ada, m); onJob(g, ada, c) {
			t.Fatal("a butcher was sent for a held carcass")
		}
		g.removeAnimal(wolf)
		// the failed search waits out the bill's retry cooldown (PRD-1)
		if !stepUntil(g, craft.IngredientRetry+50, func() bool { return onJob(g, ada, c) }) {
			t.Fatal("control: free, it is butchered")
		}
	})
	t.Run("burial", func(t *testing.T) {
		t.Parallel()
		g, c, wolf, ada := setup(t)
		buildNow(t, g, construction.Grave, item.Wood, construction.South, 4, 12)
		gz := zoneViewAt(t, g, 4, 12)
		applyNow(g, command.Command{Kind: command.SetStockpileFilter, Target: uint64(gz.ID), Name: "Corpses/Animal", Enabled: true})
		if g.assignBury(ada); onJob(g, ada, c) {
			t.Fatal("a digger was sent for a held carcass")
		}
		g.removeAnimal(wolf)
		if g.assignBury(ada); !onJob(g, ada, c) {
			t.Fatal("control: free, it is buried")
		}
	})
	t.Run("food", func(t *testing.T) {
		t.Parallel()
		g, c, wolf, ada := setup(t)
		malnourish(g, ada, 500) // only the malnourished eat carcasses (FOOD-1)
		if best, _, _, _ := g.bestFood(ada, ada); best == c {
			t.Fatal("a held carcass is no meal")
		}
		if g.hasEdible(true) {
			t.Fatal("a held carcass doesn't count as food about")
		}
		g.removeAnimal(wolf)
		if best, _, _, _ := g.bestFood(ada, ada); best != c {
			t.Fatalf("control: free, it is on the menu, got %d", best)
		}
	})
	t.Run("order menu", func(t *testing.T) {
		t.Parallel()
		g, c, wolf, ada := setup(t)
		alCorpseStore(t, g, 2, 14, 6, 16)
		option := func() OrderOption {
			for _, o := range g.OrdersAt(ada, 10, 10) {
				if strings.HasPrefix(o.Label, "Haul ") {
					return o
				}
			}
			t.Fatal("the menu offers no haul")
			return OrderOption{}
		}
		if o := option(); !o.Disabled || o.Reason != ReasonBusyTarget+"a timber wolf" {
			t.Fatalf("the menu says why: %+v", o)
		}
		g.Apply(option().Cmd)
		g.Step(1)
		if onJob(g, ada, c) {
			t.Fatal("the order started anyway")
		}
		g.removeAnimal(wolf)
		if o := option(); o.Disabled {
			t.Fatalf("control: free, the order is open: %+v", o)
		}
	})
}

// AL-2: when the predator breaks off or dies, what is left follows the
// home-area rule it landed under, and is free.
func TestCarcassAfterTheFeederGoes(t *testing.T) {
	t.Parallel()
	g := alGame(60, 20)
	for _, c := range []struct {
		x    int
		want bool
	}{{5, false}, {40, true}} {
		deer, _ := g.SpawnAnimal("deer", c.x, 5)
		wolf := alWolf(t, g, c.x+1, 5)
		killWith(g, deer, wolf)
		id, _ := alCarcass(t, g, animal.Deer, c.x, 5)
		setAnimal(g, wolf, func(a *animal.State) {
			a.Mode, a.Feed, a.FeedLeft = animal.Wander, animal.FeedTarget{}, 0
			a.Food = animal.Full
		})
		g.Step(1)
		st, there := g.stacks.Get(id)
		if !there || st.Forbidden != c.want {
			t.Fatalf("x=%d: the remainder %+v (there %v), forbidden want %v", c.x, st, there, c.want)
		}
		if _, held := g.reserve.EntityOwner(id); held {
			t.Fatalf("x=%d: it is free once the wolf broke off", c.x)
		}
	}
	// dead: same
	deer, _ := g.SpawnAnimal("deer", 7, 15)
	wolf := alWolf(t, g, 8, 15)
	killWith(g, deer, wolf)
	id, _ := alCarcass(t, g, animal.Deer, 7, 15)
	g.removeAnimal(wolf)
	if _, held := g.reserve.EntityOwner(id); held {
		t.Fatal("a dead feeder holds nothing")
	}
}

// AL-3, the reported glitch: a colonist on its way to a stack that is
// eaten away stops at once, on every path that fetches one.
func TestStackGoneEndsTheWalkAtOnce(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		rec  func(st entity.EntityID) job.Record
	}{
		{"haul", func(s entity.EntityID) job.Record {
			return job.Record{Kind: job.Haul, Target: s, TX: 35, TY: 5, DestX: 2, DestY: 8}
		}},
		{"hopper fill", func(s entity.EntityID) job.Record {
			return job.Record{Kind: job.FillHopper, Target: s, TX: 35, TY: 5, DestX: 2, DestY: 8}
		}},
		{"butcher bill", func(s entity.EntityID) job.Record {
			return job.Record{Kind: job.Bill, Target: s, TX: 35, TY: 5, Dest: 1 << 30, DestX: 2, DestY: 8, Count: 1}
		}},
		{"bill gather", func(s entity.EntityID) job.Record {
			return job.Record{Kind: job.HaulIngredients, Target: s, TX: 35, TY: 5, Dest: 1 << 30, DestX: 2, DestY: 8,
				Route: []entity.EntityID{s, 1}, Count: 2}
		}},
		{"burial", func(s entity.EntityID) job.Record {
			return job.Record{Kind: job.Bury, Target: s, TX: 35, TY: 5, Dest: 1 << 30, DestX: 2, DestY: 8, WorkLeft: buryWork}
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			g := alGame(40, 12)
			s := g.dropCorpse(animal.Deer, 35, 5)
			ada, _ := g.SpawnNamedPawn(3, 5, "Ada")
			g.SetNeeds(ada, needs.Full, needs.Full)
			if !g.startJob(ada, c.rec(s)) {
				j, _ := g.jobs.Get(ada)
				t.Fatalf("the job starts: %+v", j)
			}
			g.Step(10)
			if j, _ := g.jobs.Get(ada); j.Target != s || j.State != job.Goto {
				t.Fatalf("on its way: %+v", j)
			}
			st, _ := g.stacks.Get(s)
			g.takeItems(s, st, st.Qty) // eaten
			if j, _ := g.jobs.Get(ada); j.State != job.Fail || j.Fail != job.TargetGone {
				t.Fatalf("the job ends the moment the stack goes: %+v", j)
			}
			far := pos(g, ada).X
			for i := 0; i < 40; i++ {
				g.Step(1)
				if j, _ := g.jobs.Get(ada); j.Target == s {
					t.Fatalf("tick %d: still on the eaten stack: %+v", i, j)
				}
				if x := pos(g, ada).X; x >= 34 {
					t.Fatalf("tick %d: it walked to the empty spot (from x=%d)", i, far)
				}
			}
		})
	}
}

// AL-3: only someone still walking to the stack stops. A pawn working on
// it (State Do) finishes its own step, and one that carries its load goes
// on whatever became of the pile it came from.
func TestStackGoneStopsOnlyTheWalkers(t *testing.T) {
	t.Parallel()
	g := alGame(40, 12)
	s, _ := g.SpawnStack(item.Wood, 5, 20, 5)
	load, _ := g.SpawnStack(item.Wood, 5, 21, 5)
	walker, _ := g.SpawnNamedPawn(3, 3, "Walker")
	worker, _ := g.SpawnNamedPawn(20, 5, "Worker")
	carrier, _ := g.SpawnNamedPawn(21, 5, "Carrier")
	w := job.Record{Kind: job.Haul, State: job.Goto, Target: s, TX: 20, TY: 5, Path: nil}
	d := job.Record{Kind: job.Haul, State: job.Do, Target: s, TX: 20, TY: 5}
	k := job.Record{Kind: job.Haul, State: job.Goto, Target: s, Carry: load, TX: 2, TY: 8, DestX: 2, DestY: 8}
	g.jobs.Set(walker, w)
	g.jobs.Set(worker, d)
	g.jobs.Set(carrier, k)
	g.reserve.TryCount(walker, s, 1, 5)
	g.reserve.TryCount(worker, s, 1, 5)
	g.deleteItem(s)
	if j, _ := g.jobs.Get(walker); j.State != job.Fail || j.Fail != job.TargetGone {
		t.Fatalf("the walker stops: %+v", j)
	}
	if j, _ := g.jobs.Get(worker); j.State != job.Do || j.Fail != 0 {
		t.Fatalf("the worker's own step decides: %+v", j)
	}
	if j, _ := g.jobs.Get(carrier); j.State != job.Goto || j.Fail != 0 {
		t.Fatalf("the carrier goes on: %+v", j)
	}
}

// alNoFail runs g until done, failing on any job of id that ended with its
// target gone: consuming or merging a stack in the course of a job isn't
// losing it (the actor's own deleteItem must not fail the actor).
func alNoFail(t *testing.T, g *Game, id entity.EntityID, ticks int, done func() bool) {
	t.Helper()
	for i := 0; i < ticks; i++ {
		g.Step(1)
		if j, _ := g.jobs.Get(id); j.State == job.Fail && j.Fail == job.TargetGone {
			t.Fatalf("tick %d: the job failed on its own stack: %+v", i, j)
		}
		if done() {
			return
		}
	}
	t.Fatal("it never finished")
}

// AL-3: a job that uses its stack up finishes normally.
func TestActorUsingUpItsStackFinishes(t *testing.T) {
	t.Parallel()
	t.Run("eating the last one", func(t *testing.T) {
		t.Parallel()
		g := alGame(30, 12)
		ada, _ := g.SpawnNamedPawn(3, 5, "Ada")
		s, _ := g.SpawnStack(item.Berries, 1, 8, 5)
		g.SetNeeds(ada, 100, needs.Full)
		alNoFail(t, g, ada, 2000, func() bool {
			_, there := g.stacks.Get(s)
			return !there
		})
		g.Step(2)
		if n, _ := g.need.Get(ada); n.Hunger <= 100 {
			t.Fatalf("it ate: %+v", n)
		}
	})
	t.Run("a bill's ingredient", func(t *testing.T) {
		t.Parallel()
		g := alGame(30, 12)
		cook, _ := g.SpawnNamedPawn(2, 5, "Cook")
		g.SetNeeds(cook, needs.Full, needs.Full)
		onlyWork(g, cook, work.Cook)
		placeBuilt(t, g, construction.ButcherTable, construction.South, 6, 3)
		g.dropCorpse(animal.Deer, 3, 7)
		alNoFail(t, g, cook, 6000, func() bool {
			g.SetNeeds(cook, needs.Full, needs.Full)
			return itemsOnGround(g, "meat") > 0
		})
		if itemsOnGround(g, "deer corpse") != 0 {
			t.Fatal("the corpse went into the bill")
		}
	})
	t.Run("a haul merging into a stack", func(t *testing.T) {
		t.Parallel()
		g := alGame(30, 12)
		ada, _ := g.SpawnNamedPawn(3, 5, "Ada")
		g.SetNeeds(ada, needs.Full, needs.Full)
		onlyWork(g, ada, work.Haul)
		applyNow(g, command.Command{Kind: command.DesignateStockpile, X: 12, Y: 5, X2: 12, Y2: 5})
		stored, _ := g.SpawnStack(item.Wood, 5, 12, 5)
		s, _ := g.SpawnStack(item.Wood, 5, 6, 5)
		alNoFail(t, g, ada, 2000, func() bool {
			g.SetNeeds(ada, needs.Full, needs.Full)
			st, ok := g.stacks.Get(stored)
			return ok && st.Qty == 10
		})
		if _, there := g.stacks.Get(s); there {
			t.Fatal("the loose stack merged into the stored one")
		}
	})
}

// AL-4: a wild mother's egg follows the home-area rule; a tame one's is the
// colony's wherever it falls.
func TestWildEggFollowsTheHomeRule(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		x    int
		tame bool
		want bool
	}{{"wild in Home", 5, false, false}, {"wild outside", 40, false, true}, {"tame outside", 40, true, false}} {
		g := alGame(60, 30)
		warmDay(t, g)
		hen := spawnSexed(t, g, "turkey", c.x, 10, true, c.tame, "Hen")
		setAnimal(g, hen, func(a *animal.State) { a.Mated = animal.MatedEggs })
		day := int(uint64(g.clock.Now())/clock.TicksPerDay) + 1
		for !animal.LaysToday(animal.Turkey, uint64(hen), day) {
			day++
		}
		advance(g, uint64(day)*clock.TicksPerDay+animal.LayTick(animal.Turkey, uint64(hen)))
		keepFed(g, hen)
		g.Step(1)
		_, st, ok := eggStack(g, item.FertileEgg)
		if !ok {
			t.Fatalf("%s: no egg", c.name)
		}
		if st.Forbidden != c.want {
			t.Errorf("%s: egg forbidden %v, want %v", c.name, st.Forbidden, c.want)
		}
	}
}
