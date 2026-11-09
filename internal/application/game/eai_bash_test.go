package game

import (
	"testing"

	"rimworld-go/internal/application/game/rage"
	"rimworld-go/internal/application/game/raid"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// EAI-001 and EAI-002: shut-out manhunters and insects break through,
// and a manhunter's choice of foe.

// eaiEnrage sends animal id manhunter (for good, for a hive insect).
func eaiEnrage(g *Game, id entity.EntityID) {
	a := animalOf(g, id)
	if animal.DefOf(a.Species).Hive {
		a.Manhunter, a.Mode = raid.HiveForever, animal.Attack
	} else {
		rageOf(g).Enrage(id, &a, 0, uint64(g.clock.Now()))
	}
	g.animals.Set(id, a)
}

// eaiView is animal id's read model.
func eaiView(g *Game, id entity.EntityID) AnimalView {
	for _, v := range g.World().Animals {
		if v.ID == id {
			return v
		}
	}
	return AnimalView{}
}

// eaiBox walls in x0..x1, y0..y1 (no roof), with a shut door on each
// of doors.
func eaiBox(t *testing.T, g *Game, x0, y0, x1, y1 int, doors ...[2]int) {
	t.Helper()
	for x := x0; x <= x1; x++ {
		for y := y0; y <= y1; y++ {
			if x != x0 && x != x1 && y != y0 && y != y1 {
				continue
			}
			kind := "wall"
			for _, d := range doors {
				if d == [2]int{x, y} {
					kind = "door"
				}
			}
			if !g.BuildNow(kind, "wood", "south", x, y) {
				t.Fatalf("%s at %d,%d", kind, x, y)
			}
		}
	}
}

// eaiHurt reports building b damaged or gone.
func eaiHurt(g *Game, b entity.EntityID) bool {
	o, ok := g.bldgs.Get(b)
	return !ok || o.HP() < construction.MaxHP(o.Kind, o.Stuff)
}

// eaiShutIn builds a hut whose shut door is at (13, 11) and puts a
// drafted colonist inside: it returns the door and the colonist.
func eaiShutIn(t *testing.T, g *Game) (entity.EntityID, entity.EntityID) {
	t.Helper()
	return eaiShutInBy(t, g, [2]int{13, 11})
}

// eaiShutInBy is eaiShutIn with the shut doors at doors (the first
// returned).
func eaiShutInBy(t *testing.T, g *Game, doors ...[2]int) (entity.EntityID, entity.EntityID) {
	t.Helper()
	bareGround(g)
	eaiBox(t, g, 10, 5, 16, 11, doors...) // the drafted colonist stays inside
	return g.layerBldgAt(doors[0][0], doors[0][1], construction.LayerObject), draftedPawn(t, g, 13, 8)
}

// TestManhunterBashesShutDoor: a manhunter whose foe is behind a shut
// door batters that door (and shows it) until it is down.
func TestManhunterBashesShutDoor(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 3, Width: 30, Height: 22})
	door, ada := eaiShutIn(t, g)
	wolf, _ := g.SpawnAnimal("timber_wolf", 13, 18)
	eaiEnrage(g, wolf)
	shown := false
	for i := 0; i < 6000 && !eaiHurt(g, door); i++ {
		g.Step(1)
		if i%20 == 0 && !shown {
			shown = eaiView(g, wolf).Bashing
		}
	}
	if !eaiHurt(g, door) {
		t.Fatalf("the manhunter never battered the shut door (target %d, Ada %d)", animalOf(g, wolf).Target, ada)
	}
	if !shown {
		t.Fatal("the inspector never showed the manhunter breaking through the door")
	}
	for _, id := range g.bldgIDs {
		if b, _ := g.bldgs.Get(id); b.Kind == construction.Wall && eaiHurt(g, id) {
			t.Fatal("a manhunter breaks doors, never walls")
		}
	}
}

// TestInsectBashesWallWithoutDoorRoute: walled in without a door, the
// colonist is reached by a hive insect battering a wall; a manhunter wolf
// in its place breaks no wall.
func TestInsectBashesWallWithoutDoorRoute(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		species string
		walls   bool
	}{{"hive_crawler", true}, {"timber_wolf", false}} {
		g := New(Config{Seed: 4, Width: 30, Height: 22})
		bareGround(g)
		eaiBox(t, g, 10, 5, 16, 11)
		draftedPawn(t, g, 13, 8)
		bug, err := g.SpawnAnimal(c.species, 13, 18)
		if err != nil {
			t.Fatal(err)
		}
		eaiEnrage(g, bug)
		hit := false
		for i := 0; i < 4000 && !hit; i++ {
			g.Step(1)
			for _, id := range g.bldgIDs {
				hit = hit || eaiHurt(g, id)
			}
		}
		if hit != c.walls {
			t.Fatalf("%s: walls battered %v, want %v", c.species, hit, c.walls)
		}
	}
}

// TestAnimalBashNoFlapOnHeldDoor: a shut-out manhunter holds the one
// block it chose, whatever its foe does; once that door is held open it
// stops at once and walks in, never going back to battering; a bash given
// up waits out the path retry before another starts.
func TestAnimalBashNoFlapOnHeldDoor(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 5, Width: 30, Height: 22})
	door, _ := eaiShutInBy(t, g, [2]int{13, 11}, [2]int{10, 8}) // a second way in, also shut
	wolf, _ := g.SpawnAnimal("timber_wolf", 13, 18)
	eaiEnrage(g, wolf)
	var held rage.Bash
	for i := 0; i < 3000; i++ {
		g.Step(1)
		if b, ok := g.wl.Rage.Bash[wolf]; ok {
			if held == (rage.Bash{}) {
				held = b
			} else if b != held {
				t.Fatalf("tick %d: the bash moved from %+v to %+v", i, held, b)
			}
		}
		if held != (rage.Bash{}) && i%40 == 0 {
			ada := g.pawnIDs[0]
			g.positions.Set(ada, Position{X: 11 + i/40%5, Y: 6 + i/40%4}) // the foe moves about inside
		}
		if eaiHurt(g, door) && held != (rage.Bash{}) {
			break
		}
	}
	if held != (rage.Bash{X: 13, Y: 11, Since: held.Since}) {
		t.Fatalf("the wolf should batter the south door it faces: %+v", held)
	}
	if !eaiHurt(g, door) {
		t.Fatal("the wolf never battered the door")
	}
	// a bash given up: dropped, and no new one before the path retry
	now := uint64(g.clock.Now())
	g.wl.Rage.Bash[wolf] = rage.Bash{X: 13, Y: 11, Since: now - rage.BashGiveUp - 1}
	g.Step(1)
	a := animalOf(g, wolf)
	if _, ok := g.wl.Rage.Bash[wolf]; ok || a.RetryAt <= now {
		t.Fatalf("a bash past its give-up is dropped and waits for the retry: %+v retry %d", g.wl.Rage.Bash[wolf], a.RetryAt)
	}
	for uint64(g.clock.Now()) < a.RetryAt-1 {
		g.Step(1)
		if _, ok := g.wl.Rage.Bash[wolf]; ok {
			t.Fatal("a new bash started before the path retry")
		}
	}
	for i := 0; i < 300; i++ {
		if _, ok := g.wl.Rage.Bash[wolf]; ok {
			break
		}
		g.Step(1)
	}
	if _, ok := g.wl.Rage.Bash[wolf]; !ok {
		t.Fatal("after the retry the wolf goes back to the door")
	}
	// the door held open: the bash is dropped at once and stays dropped
	g.Apply(command.Command{Kind: command.SetDoorHoldOpen, X: 13, Y: 11, Enabled: true})
	for i := 0; i < 400; i++ {
		g.Step(1)
		if b, ok := g.wl.Rage.Bash[wolf]; ok && i > 1 {
			t.Fatalf("tick %d after the door was held open: still bashing %+v", i, b)
		}
	}
}

// eaiLook is the first tick from now on which animal id makes a full
// decision (a manhunter's look).
func eaiLook(g *Game, id entity.EntityID) uint64 {
	t := uint64(g.clock.Now())
	for (t+uint64(id))%animal.DecideEvery != 0 {
		t++
	}
	return t
}

// eaiThink runs manhunter id's thought at now from (x, y), standing
// still, and returns its foe.
func eaiThink(g *Game, id entity.EntityID, a *animal.State, now uint64, x, y int) entity.EntityID {
	g.positions.Set(id, Position{X: x, Y: y})
	g.invalidateCreatures()
	rageOf(g).ThinkManhunter(id, a, now)
	a.Path, a.PathI = nil, 0
	return a.Target
}

// TestManhunterRetargetNoFlap: a manhunter keeps its foe until another is
// retargetMargin (6) tiles closer, and even then only once its lock has
// run 180 ticks; two foes trading places by less never swap it.
func TestManhunterRetargetNoFlap(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 7, Width: 60, Height: 20})
	bareGround(g)
	ada := draftedPawn(t, g, 15, 10)
	bo := draftedPawn(t, g, 45, 10)
	wolf, _ := g.SpawnAnimal("timber_wolf", 5, 10)
	eaiEnrage(g, wolf)
	a := animalOf(g, wolf)
	a.Target = 0
	t0 := eaiLook(g, wolf)
	move := func(id entity.EntityID, x int) { g.positions.Set(id, Position{X: x, Y: 10}) }
	if f := eaiThink(g, wolf, &a, t0, 5, 10); f != ada {
		t.Fatalf("the nearest foe first: %d, want Ada %d", f, ada)
	}
	move(bo, 8) // 7 tiles closer than Ada (10 away), inside the lock
	for _, dt := range []uint64{60, 120} {
		if f := eaiThink(g, wolf, &a, t0+dt, 5, 10); f != ada {
			t.Fatalf("+%d: switched inside the lock to %d", dt, f)
		}
	}
	if f := eaiThink(g, wolf, &a, t0+180, 5, 10); f != bo {
		t.Fatalf("after the lock a foe 7 tiles closer wins: %d, want Bo %d", f, bo)
	}
	// they trade places, by less than the margin each time: no swapping
	flips := 0
	for i := 0; i < 20; i++ {
		if i%2 == 0 {
			move(ada, 11)
			move(bo, 15)
		} else {
			move(ada, 15)
			move(bo, 11)
		}
		prev := a.Target
		if eaiThink(g, wolf, &a, t0+360+uint64(i)*animal.DecideEvery, 5, 10) != prev {
			flips++
		}
	}
	if flips != 0 {
		t.Fatalf("the foe flapped %d times between two near foes", flips)
	}
}

// TestManhunterTargetsGuestsRaidersLivestock: anyone of the colony's
// world is a manhunter's foe: a guest, a raider, a tame cow.
func TestManhunterTargetsGuestsRaidersLivestock(t *testing.T) {
	t.Parallel()
	for _, who := range []string{"visitor", "raider", "cow"} {
		g := New(Config{Seed: 8, Width: 40, Height: 20})
		bareGround(g)
		var foe entity.EntityID
		switch who {
		case "visitor":
			foe = g.spawnVisitor(0, nil, 20, 10, uint64(g.clock.Now()))
		case "raider":
			g.Raid(1)
			foe = g.raiderIDs[len(g.raiderIDs)-1]
			g.positions.Set(foe, Position{X: 20, Y: 10})
		case "cow":
			foe, _ = g.SpawnAnimal("cow", 20, 10)
			g.TameAnimal(foe, "Bessie")
		}
		wolf, _ := g.SpawnAnimal("timber_wolf", 5, 10)
		eaiEnrage(g, wolf)
		a := animalOf(g, wolf)
		if f := eaiThink(g, wolf, &a, eaiLook(g, wolf), 5, 10); f != foe {
			t.Fatalf("a manhunter goes for the %s %d: got %d", who, foe, f)
		}
	}
}

// TestManhunterPrefersReachable: a colonist walled in nearby loses to one
// in the open farther off; with nobody reachable, the walled-in one is
// the foe (and gets broken through to).
func TestManhunterPrefersReachable(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 9, Width: 50, Height: 20})
	bareGround(g)
	eaiBox(t, g, 10, 7, 14, 13)
	ada := draftedPawn(t, g, 12, 10)
	bo := draftedPawn(t, g, 40, 10)
	wolf, _ := g.SpawnAnimal("timber_wolf", 20, 10)
	eaiEnrage(g, wolf)
	a := animalOf(g, wolf)
	t0 := eaiLook(g, wolf)
	if f := eaiThink(g, wolf, &a, t0, 20, 10); f != bo {
		t.Fatalf("the reachable colonist first: %d, want Bo %d (Ada %d)", f, bo, ada)
	}
	g.positions.Set(bo, Position{X: 12, Y: 9}) // Bo walls himself in too
	a.Target = 0
	if f := eaiThink(g, wolf, &a, t0+60, 20, 10); f != ada && f != bo {
		t.Fatalf("with no one reachable the nearest walled-in colonist: %d", f)
	}
}
