package game

import (
	"testing"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

func meatOnGround(g *Game) int {
	n := 0
	for _, it := range g.World().Items {
		if it.Kind == "meat" && !it.Carried {
			n += it.Qty
		}
	}
	return n
}

func animalAlive(g *Game, id entity.EntityID) bool {
	_, ok := g.animals.Get(id)
	return ok
}

func TestHunterFetchesRifleAndBringsDownDustbuck(t *testing.T) {
	g := New(Config{Seed: 3, Width: 40, Height: 16})
	p, _ := g.SpawnNamedPawn(2, 8, "Hunter")
	if _, err := g.SpawnStack(item.Rifle, 1, 4, 8); err != nil {
		t.Fatal(err)
	}
	buck, err := g.SpawnAnimal("dustbuck", 30, 8)
	if err != nil {
		t.Fatal(err)
	}
	g.Apply(command.Command{Kind: command.DesignateHunt, X: 25, Y: 0, X2: 39, Y2: 15})
	for i := 0; i < 60 && animalAlive(g, buck); i++ {
		g.Step(100)
	}
	if animalAlive(g, buck) {
		a, _ := g.animals.Get(buck)
		t.Fatalf("dustbuck survived: %+v, hunter job %q, weapon %q", a, g.Snapshot()[0].Job, g.Equipped(p))
	}
	if g.Equipped(p) != "rifle" {
		t.Fatalf("hunter should have picked up the rifle, has %q", g.Equipped(p))
	}
	if got := meatOnGround(g); got != animal.DefOf(animal.Dustbuck).Meat {
		t.Fatalf("meat = %d, want %d", got, animal.DefOf(animal.Dustbuck).Meat)
	}
}

func TestHungryWolfAttacksAndDraftedRiflemanKillsIt(t *testing.T) {
	g := New(Config{Seed: 5, Width: 30, Height: 12})
	victim, _ := g.SpawnNamedPawn(10, 6, "Bait")
	shooter, _ := g.SpawnNamedPawn(4, 6, "Shooter")
	g.equip[shooter] = item.Rifle
	wolf, _ := g.SpawnAnimal("ridgewolf", 14, 6)
	a, _ := g.animals.Get(wolf)
	a.Hunger = animal.DefOf(animal.Ridgewolf).HungerTicks
	g.animals.Set(wolf, a)

	g.Step(200)
	if v := g.VitalOf(victim); v.Injury == 0 {
		a, _ := g.animals.Get(wolf)
		t.Fatalf("hungry wolf should have bitten the nearby colonist: %+v", a)
	}
	if w := g.World(); len(w.Animals) != 1 || !w.Animals[0].Hostile {
		t.Fatalf("wolf should read as hostile: %+v", w.Animals)
	}
	g.Apply(command.Command{Kind: command.DraftPawn, Pawn: uint64(shooter), Enabled: true})
	for i := 0; i < 40 && animalAlive(g, wolf); i++ {
		g.Step(50)
	}
	if animalAlive(g, wolf) {
		a, _ := g.animals.Get(wolf)
		t.Fatalf("drafted rifleman should auto-engage the wolf: wolf %+v", a)
	}
	if got := meatOnGround(g); got != animal.DefOf(animal.Ridgewolf).Meat {
		t.Fatalf("wolf meat = %d", got)
	}
	if g.VitalOf(victim).Status == health.Dead {
		t.Fatal("victim should survive a short fight")
	}
}

func TestInjuryDownsThenHeals(t *testing.T) {
	g := New(Config{Seed: 1, Width: 10, Height: 10})
	p, _ := g.SpawnNamedPawn(5, 5, "Ada")
	g.hurt(p, health.DownedInjury+5, 0)
	v := g.VitalOf(p)
	if v.Status != health.Downed || v.Cause != health.CauseInjury {
		t.Fatalf("should be downed by injury: %+v", v)
	}
	if g.Snapshot()[0].Reason != "injured" || g.Snapshot()[0].Health != 25 {
		t.Fatalf("view: %+v", g.Snapshot()[0])
	}
	g.SetNeeds(p, 1<<20, 1<<20) // keep needs out of the way
	for i := 0; i < 100 && g.VitalOf(p).Status != health.Alive; i++ {
		g.Step(500)
		g.SetNeeds(p, 1<<20, 1<<20)
	}
	if v := g.VitalOf(p); v.Status != health.Alive || v.Injury > health.RecoverInjury {
		t.Fatalf("should stand up after healing: %+v", v)
	}
	g.hurt(p, health.MaxHP, 0)
	if g.VitalOf(p).Status != health.Dead {
		t.Fatal("lethal damage kills")
	}
}

func TestWallsBlockShots(t *testing.T) {
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	from, to := Position{X: 2, Y: 4}, Position{X: 8, Y: 4}
	if !g.lineOfSight(from.X, from.Y, to.X, to.Y) {
		t.Fatal("open field has sight")
	}
	placeBuilt(t, g, construction.Wall, construction.South, 5, 4)
	if g.lineOfSight(from.X, from.Y, to.X, to.Y) {
		t.Fatal("a wall blocks sight")
	}
}

func TestCombatOrders(t *testing.T) {
	g := New(Config{Seed: 1, Width: 14, Height: 8})
	p, _ := g.SpawnNamedPawn(2, 4, "Ada")
	if _, err := g.SpawnAnimal("dustbuck", 8, 4); err != nil {
		t.Fatal(err)
	}
	if _, err := g.SpawnStack(item.Club, 1, 3, 4); err != nil {
		t.Fatal(err)
	}
	find := func(opts []OrderOption, label string) (OrderOption, bool) {
		for _, o := range opts {
			if o.Label == label {
				return o, true
			}
		}
		return OrderOption{}, false
	}
	hunt, ok := find(g.OrdersAt(p, 8, 4), "Hunt dustbuck")
	if !ok || !hunt.Disabled || hunt.Reason != ReasonNoRanged {
		t.Fatalf("unarmed hunt option = %+v %v", hunt, ok)
	}
	equip, ok := find(g.OrdersAt(p, 3, 4), "Equip club")
	if !ok || equip.Disabled {
		t.Fatalf("equip option = %+v %v", equip, ok)
	}
	g.Apply(equip.Cmd)
	g.Step(60)
	if g.Equipped(p) != "club" {
		t.Fatalf("equipped = %q", g.Equipped(p))
	}
	g.Apply(command.Command{Kind: command.DraftPawn, Pawn: uint64(p), Enabled: true})
	g.Step(1)
	buck := g.World().Animals[0]
	atk, ok := find(g.OrdersAt(p, buck.X, buck.Y), "Melee attack dustbuck")
	if !ok || atk.Disabled {
		t.Fatalf("drafted attack option = %+v %v (orders %+v)", atk, ok, g.OrdersAt(p, buck.X, buck.Y))
	}
}

func TestColonyStartsWithWildlifeAndARifle(t *testing.T) {
	g := NewColony(Config{Seed: 7, Width: 48, Height: 40})
	bucks, wolves := 0, 0
	for _, a := range g.World().Animals {
		switch a.Species {
		case "dustbuck":
			bucks++
		case "ridgewolf":
			wolves++
		}
	}
	if bucks < herdSize || wolves != startWolves {
		t.Fatalf("wildlife: %d dustbucks, %d wolves", bucks, wolves)
	}
	rifles := 0
	for _, it := range g.World().Items {
		if it.Kind == "rifle" {
			rifles += it.Qty
		}
	}
	if rifles != 1 {
		t.Fatalf("start kit rifles = %d", rifles)
	}
}
