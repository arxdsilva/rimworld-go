// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"strings"
	"testing"

	"rimworld-go/internal/application/game/combat"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

func itemsOnGround(g *Game, kind string) int {
	n := 0
	for _, it := range g.World().Items {
		if it.Kind == kind && !it.Carried {
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
	t.Parallel()
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
	if got := itemsOnGround(g, "deer corpse"); got != 1 {
		t.Fatalf("dustbuck corpses = %d, want 1", got)
	}
}

func TestHungryWolfAttacksAndDraftedRiflemanKillsIt(t *testing.T) {
	t.Parallel()
	// Upstream CBT 3.4b changed seed 5 to 6 on its own rule set.
	// The combined Core source retains the original seed 5 kill, while
	// seed 6 is downed with the rifleman still attacking at 2,000 ticks.
	// Independently observed both seeds before choosing these expectations.
	for _, tc := range []struct {
		name string
		seed uint64
		dead bool
	}{{"killed", 5, true}, {"downed still engaged", 6, false}} {
		t.Run(tc.name, func(t *testing.T) {
			seed := tc.seed
			g := New(Config{Seed: seed, Width: 30, Height: 12})
			victim, _ := g.SpawnNamedPawn(10, 6, "Bait")
			shooter, _ := g.SpawnNamedPawn(4, 6, "Shooter")
			g.equip[shooter] = item.Rifle
			// CBT-4: set to ignore threats, the undrafted rifleman leaves the
			// wolf be until drafted (by default it would shoot it on sight)
			g.combat.Drafts.SetResponse(shooter, combat.RespondIgnore)
			wolf, _ := g.SpawnAnimal("ridgewolf", 14, 6)
			a, _ := g.animals.Get(wolf)
			a.Hunger = animal.DefOf(animal.Ridgewolf).HungerTicks
			g.animals.Set(wolf, a)

			for i := 0; i < 30 && g.VitalOf(victim).Injury == 0; i++ {
				g.Step(100) // the bait runs; the wolf runs it down (CBT-2: it hits 62 %, the bait may dodge)
			}
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
			if tc.dead {
				if animalAlive(g, wolf) {
					a, _ := g.animals.Get(wolf)
					t.Fatalf("drafted rifleman should auto-engage the wolf: wolf %+v", a)
				}
				if got := itemsOnGround(g, "timber wolf corpse"); got != 1 {
					t.Fatalf("wolf corpses = %d", got)
				}
			} else {
				a, ok := g.animals.Get(wolf)
				j, _ := g.jobs.Get(shooter)
				if !ok || !a.Downed || a.Attacker != shooter || a.Injury <= 0 ||
					j.Kind != job.Attack || j.Target != wolf || !j.Active() {
					t.Fatalf("downed wolf must still be engaged: wolf %+v job %+v", a, j)
				}
				if got := itemsOnGround(g, "timber wolf corpse"); got != 0 {
					t.Fatalf("living downed wolf created %d corpses", got)
				}
			}
			if g.VitalOf(victim).Status == health.Dead {
				t.Fatal("victim should survive a short fight")
			}
		})
	}
}

func TestInjuryDownsThenHeals(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 10, Height: 10})
	p, _ := g.SpawnNamedPawn(5, 5, "Ada")
	// one torso bruise (MED-01): a hit that tore a part off would leave a
	// fresh stump bleeding, and no one is here to tend it
	addWound(t, g, p, body.Torso, health.DownedInjury+5, body.Bruise)
	v := g.VitalOf(p)
	if v.Status != health.Downed || v.Cause != health.CauseInjury {
		t.Fatalf("should be downed by injury: %+v", v)
	}
	if g.Snapshot()[0].Reason != "injured" || g.Snapshot()[0].Health != 25 {
		t.Fatalf("view: %+v", g.Snapshot()[0])
	}
	g.SetNeeds(p, 1<<20, 1<<20)                                       // keep needs out of the way
	for i := 0; i < 220 && g.VitalOf(p).Status != health.Alive; i++ { // MED-02: 15 HP at 12 a day on the ground
		g.Step(500)
		g.SetNeeds(p, 1<<20, 1<<20)
	}
	// it stands once pain is under the shock band (750: 60 HP of wounds)
	if v := g.VitalOf(p); v.Status != health.Alive || v.Injury > 60 {
		t.Fatalf("should stand up after healing: %+v", v)
	}
	g.hurt(p, health.MaxHP, 0)
	if g.VitalOf(p).Status != health.Dead {
		t.Fatal("lethal damage kills")
	}
}

func TestWallsBlockShots(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 12, Height: 8})
	from, to := Position{X: 2, Y: 4}, Position{X: 8, Y: 4}
	if !g.combat.Sight.Line(from.X, from.Y, to.X, to.Y) {
		t.Fatal("open field has sight")
	}
	placeBuilt(t, g, construction.Wall, construction.South, 5, 4)
	if g.combat.Sight.Line(from.X, from.Y, to.X, to.Y) {
		t.Fatal("a wall blocks sight")
	}
}

func TestCombatOrders(t *testing.T) {
	t.Parallel()
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
	hunt, ok := find(g.OrdersAt(p, 8, 4), "Hunt deer")
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
	atk, ok := find(g.OrdersAt(p, buck.X, buck.Y), "Melee attack deer")
	if !ok || atk.Disabled {
		t.Fatalf("drafted attack option = %+v %v (orders %+v)", atk, ok, g.OrdersAt(p, buck.X, buck.Y))
	}
}

func TestColonyStartsWithWildlifeAndARifle(t *testing.T) {
	t.Parallel()
	g := NewColony(Config{Seed: 7, Width: 48, Height: 40})
	// The map starts with weighted wildlife without a guaranteed predator.
	grazers, predators := 0, 0
	species := map[string]bool{}
	for _, a := range g.World().Animals {
		s, ok := animal.ParseSpecies(a.Species)
		if !ok {
			t.Fatalf("unknown species %q", a.Species)
		}
		d := animal.DefOf(s)
		if d.Hive || d.Legendary {
			t.Fatalf("%s at colony start", a.Species)
		}
		species[a.Species] = true
		if d.Predator {
			predators++
		} else {
			grazers++
		}
	}
	if grazers == 0 || predators > startPredatorMax*startHerdsMax || len(species) > startHerdsMax ||
		g.wildCount() > g.wildlifeCap()+largestWildHerdCost(g) {
		t.Fatalf("wildlife: %d grazers, %d predators, weight %d/%d, species %v",
			grazers, predators, g.wildCount(), g.wildlifeCap(), species)
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

func TestBigStacksHaulInLoadsAndTopUp(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 16, Height: 8})
	if _, err := g.SpawnNamedPawn(2, 4, "Hauler"); err != nil {
		t.Fatal(err)
	}
	// STO-2: wood stacks to 75 (wiki), so 140 loose and 10 stored fill
	// exactly two stockpile cells when every load tops up first
	if _, err := g.SpawnStack(item.Wood, 140, 3, 4); err != nil {
		t.Fatal(err)
	}
	if _, err := g.SpawnStack(item.Wood, 10, 12, 2); err != nil { // already stored
		t.Fatal(err)
	}
	g.Apply(command.Command{Kind: command.DesignateStockpile, X: 12, Y: 2, X2: 13, Y2: 3})
	g.Step(1)
	maxCarried := 0
	for i := 0; i < 400 && itemsOnGround(g, "wood") > 0; i++ {
		g.Step(20)
		for _, it := range g.World().Items {
			if it.Carried && it.Qty > maxCarried {
				maxCarried = it.Qty
			}
		}
		stored := 0
		for _, it := range g.World().Items {
			if it.Kind == "wood" && !it.Carried && it.X >= 12 && it.Y >= 2 && it.X <= 13 && it.Y <= 3 {
				stored += it.Qty
			}
		}
		if stored == 150 {
			break
		}
	}
	if maxCarried == 0 || maxCarried > min(item.CarryLimit, item.Wood.StackLimit()) {
		t.Fatalf("hauls go in loads of at most %d, saw %d", item.Wood.StackLimit(), maxCarried)
	}
	total, stacks := 0, 0
	for _, it := range g.World().Items {
		if it.Kind == "wood" {
			total += it.Qty
			stacks++
			if it.Qty > item.Wood.StackLimit() {
				t.Fatalf("stack over the limit: %d", it.Qty)
			}
		}
	}
	if total != 150 {
		t.Fatalf("no wood lost or created: %d", total)
	}
	if stacks > 2 {
		t.Fatalf("loads top up the stored stack instead of spreading: %d stacks", stacks)
	}
}

func TestMergeOverflowSpills(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 8, Height: 8})
	g.SpawnStack(item.Wood, 990, 4, 4)
	g.dropStack(item.Wood, 30, 4, 4)
	total, stacks := 0, 0
	for _, it := range g.World().Items {
		total += it.Qty
		stacks++
	}
	if total != 1020 || stacks != 2 {
		t.Fatalf("overflow spills to a neighbour: total %d stacks %d", total, stacks)
	}
}

func TestUnarmedColonistCalmsDown(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 2, Width: 40, Height: 16})
	runner, _ := g.SpawnNamedPawn(12, 8, "Runner")
	g.SetNeeds(runner, needs.Full, needs.Full)
	wolf, _ := g.SpawnAnimal("ridgewolf", 15, 8)
	a, _ := g.animals.Get(wolf)
	a.Target, a.Mode = runner, animal.Attack
	g.animals.Set(wolf, a)
	g.Step(2)
	if j, _ := g.jobs.Get(runner); j.Kind != job.Move {
		t.Fatalf("unarmed colonist should run, job %+v", j)
	}
	start := uint64(g.clock.Now())
	// the wolf loses interest and just stands about
	a, _ = g.animals.Get(wolf)
	a.Target, a.Mode, a.NextThink, a.Path, a.PathI = 0, animal.Wander, 1<<40, nil, 0
	g.animals.Set(wolf, a)
	for uint64(g.clock.Now()) < start+combat.FleeCalm-1 {
		g.Step(1)
		g.SetNeeds(runner, needs.Full, needs.Full)
		if j, _ := g.jobs.Get(runner); j.Kind != job.None && j.Kind != job.Move {
			t.Fatalf("tick %d: still shaken, the colonist should hold back from work, job %+v",
				uint64(g.clock.Now())-start, j)
		}
	}
	resumed := false
	for i := 0; i < 2000 && !resumed; i++ {
		g.Step(1)
		g.SetNeeds(runner, needs.Full, needs.Full)
		j, _ := g.jobs.Get(runner)
		resumed = j.Kind != job.None && j.Kind != job.Move
	}
	if !resumed {
		t.Fatal("once calm, the colonist goes back to its day")
	}
	if _, ok := g.calm[runner]; ok {
		t.Fatal("the calm window is cleared once it has passed")
	}
	n := 0
	for _, s := range logTexts(g, runner) {
		if strings.HasPrefix(s, "Fled from") {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("one flight is one log line, got %d: %q", n, logTexts(g, runner))
	}
}

func TestArmedColonistKeepsTarget(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 2, Width: 30, Height: 12})
	shooter, _ := g.SpawnNamedPawn(10, 6, "Shooter")
	g.Arm(shooter, item.Rifle)
	w1, _ := g.SpawnAnimal("ridgewolf", 13, 3)
	w2, _ := g.SpawnAnimal("ridgewolf", 7, 9)
	// both wolves hold their ground, attacking; every 20 ticks the other
	// one is a tile nearer
	hold := func(id entity.EntityID, x, y int) {
		a, ok := g.animals.Get(id)
		if !ok {
			return
		}
		a.Target, a.Mode, a.Path, a.PathI, a.RetryAt = shooter, animal.Attack, nil, 0, 1<<40
		g.animals.Set(id, a)
		g.positions.Set(id, Position{X: x, Y: y})
	}
	var target entity.EntityID
	switches := 0
	for i := 0; i < 300; i++ {
		if (i/20)%2 == 0 {
			hold(w1, 12, 4) // 2 tiles
			hold(w2, 7, 9)  // 3 tiles
		} else {
			hold(w1, 13, 3)
			hold(w2, 8, 8)
		}
		g.Step(1)
		g.SetNeeds(shooter, needs.Full, needs.Full)
		if j, _ := g.jobs.Get(shooter); j.Kind == job.Attack && j.Target != target {
			if target != 0 {
				switches++
			}
			target = j.Target
		}
	}
	if switches > 1 {
		t.Fatalf("target thrash: switched %d times", switches)
	}
	if _, ok := g.lastStrike[shooter]; !ok {
		t.Fatal("a colonist keeping its target finishes aiming and fires")
	}
}

func TestNativeDefenceOrdersResolveLiveTarget(t *testing.T) {
	g := New(Config{Seed: 1, Width: 60, Height: 60})
	ada, _ := g.SpawnNamedPawn(10, 9, "Ada")
	enemy := spawnRaiderAt(t, g, 30, 30, 1)
	mid, ok := DevOpenMap(g, -1)
	if !ok {
		t.Fatal("second map")
	}
	g.ViewMap(mid)
	away, err := g.SpawnNamedPawn(10, 9, "Away")
	if err != nil {
		t.Fatal(err)
	}
	g.ViewMap(0)
	tu := buildNow(t, g, construction.SentryTurret, item.Steel, construction.South, 5, 5)
	mo := buildNow(t, g, construction.Mortar, item.Steel, construction.South, 10, 5)
	hold := command.Command{Kind: command.SetTurretHold, Target: uint64(tu), X: 5, Y: 5, Enabled: true}
	stale := hold
	stale.Target = uint64(mo)
	applyNow(g, stale)
	if g.combat.Turrets.Held(tu) {
		t.Fatal("stale turret order changed current building")
	}
	applyNow(g, hold)
	if !g.combat.Turrets.Held(tu) {
		t.Fatal("valid hold was ignored")
	}
	aim := command.Command{Kind: command.SetMortarTarget, Target: uint64(mo), X: 10, Y: 5, X2: 40, Y2: 40, Pawn: uint64(ada)}
	applyNow(g, aim)
	want := g.combat.Mortars.State(mo)
	if !want.Forced || want.Assigned != ada || want.TX != 40 || want.TY != 40 {
		t.Fatalf("valid aim: %+v", want)
	}
	for _, c := range []command.Command{
		{Kind: command.SetMortarTarget, Target: uint64(tu), X: 10, Y: 5, X2: 35, Y2: 35},
		{Kind: command.SetMortarTarget, Target: uint64(mo), X: 10, Y: 5, X2: 60, Y2: 35},
		{Kind: command.SetMortarTarget, Target: uint64(mo), X: 10, Y: 5, X2: 35, Y2: 35, Pawn: uint64(enemy)},
		{Kind: command.SetMortarTarget, Target: uint64(mo), X: 10, Y: 5, X2: 35, Y2: 35, Pawn: 9999},
		{Kind: command.SetMortarTarget, Target: uint64(mo), X: 10, Y: 5, X2: 35, Y2: 35, Pawn: uint64(away)},
		{Kind: command.ClearMortarTarget, Target: uint64(tu), X: 10, Y: 5},
	} {
		applyNow(g, c)
		if got := g.combat.Mortars.State(mo); got != want {
			t.Fatalf("rejected command %+v changed mortar: %+v", c, got)
		}
	}
	applyNow(g, command.Command{Kind: command.ClearMortarTarget, Target: uint64(mo), X: 10, Y: 5})
	if g.combat.Mortars.State(mo).Forced {
		t.Fatal("valid clear failed")
	}
	// Legacy coordinate-only command remains supported.
	hold.Target = 0
	hold.Enabled = false
	applyNow(g, hold)
	if g.combat.Turrets.Held(tu) {
		t.Fatal("legacy hold release ignored")
	}
}
