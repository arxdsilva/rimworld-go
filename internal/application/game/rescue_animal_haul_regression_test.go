// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"rimworld-go/internal/application/game/herd"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/animal/training"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"testing"
)

func TestRescueRevalidatesChangedPatientClimate(t *testing.T) {
	g, ada, doc, pin := heatwaveColony(t, 320)
	x, y := tileOf(g, ada)
	g.Enqueue(prioritize(command.OrderRescue, doc, ada, x, y, false))
	for i := 0; i < 3000 && g.care.Carry.Carrier(ada) != doc; i++ {
		g.Step(1)
		pin()
		fed(g, doc)
	}
	j, _ := g.jobs.Get(doc)
	if g.care.Carry.Carrier(ada) != doc || j.Dest == 0 {
		t.Fatalf("fixture no bed trip: %+v", j)
	}
	b, _ := g.bldgs.Get(j.Dest)
	if !(careReach{g}).AirHeals(ada, b.X, b.Y, 0) {
		t.Fatal("fixture target not healing")
	}
	// The patient's room becomes hot while the doctor is en route; outdoor air remains safe.
	g.thermo.Temps[g.roomAt(b.X, b.Y)] = 320
	if (careReach{g}).AirHeals(ada, b.X, b.Y, 0) || (careReach{g}).BedWhy(j.Dest, doc) != "" {
		t.Fatal("fixture did not isolate clinical failure")
	}
	if _, _, ok := (careReach{g}).HealingSpot(ada); !ok {
		t.Fatal("fixture needs alternative healing spot")
	}
	for i := 0; i < 3000 && g.care.Carry.Carrier(ada) == doc; i++ {
		g.Step(1)
		fed(g, doc)
		g.thermo.Temps[g.roomAt(b.X, b.Y)] = 320
	}
	if g.bedUnder(ada) == j.Dest {
		t.Fatalf("placed heatstroke patient into newly nonhealing bed %d, position=%v", j.Dest, g.positionOf(ada))
	}
	if g.care.Carry.Carrier(ada) != 0 {
		t.Fatal("carrier still holds patient")
	}
}

func TestCapturePrisonOpensDuringCarry(t *testing.T) {
	g, doc, bo, raider, bed := prisonGame(t)
	g.Enqueue(command.Command{Kind: command.Capture, Pawn: uint64(doc), Target: uint64(raider)})
	for i := 0; i < 2000 && g.care.Carry.Carrier(raider) != doc; i++ {
		g.Step(1)
		fed(g, doc, bo)
	}
	if g.care.Carry.Carrier(raider) != doc {
		t.Fatal("fixture: no carry")
	}
	g.removeBuildingAs(g.layerBldgAt(1, 3, construction.LayerObject), false)
	if (careReach{g}).PrisonRoom(bed) {
		t.Fatal("fixture prison still enclosed")
	}
	for i := 0; i < 2000 && g.care.Carry.Carrier(raider) == doc; i++ {
		g.Step(1)
		fed(g, doc, bo)
	}
	if g.prison.Is(raider) || g.bedUnder(raider) == bed || g.care.Carry.Carrier(raider) != 0 {
		t.Fatal("invalid custody after enclosure lost")
	}
	g.Step(2) // native phaseAI releases terminal job claims on the next tick
	if owner, ok := g.reserve.EntityOwner(bed); ok && owner == doc {
		t.Fatal("bed reservation leaked")
	}
}

func TestAnimalHaulInterrupted(t *testing.T) {
	for _, how := range []string{"down", "death", "offmap", "training_loss"} {
		t.Run(how, func(t *testing.T) {
			g := New(Config{Seed: 935, Width: 24, Height: 18})
			dog, err := g.SpawnAnimal("husky", 8, 8)
			if err != nil {
				t.Fatal(err)
			}
			g.TameAnimal(dog, "Carrier")
			g.zones.Paint(zone.KindStockpile, zone.NewStockpile(), zone.RectOf(14, 8, 16, 9), nil)
			if _, err = g.SpawnStack(item.Wood, 5, 9, 8); err != nil {
				t.Fatal(err)
			}
			saveTraining(g, dog, training.Maintenance{DecayAt: 1 << 40, HaulProgress: training.Needed(training.Haul) | herd.TrainToggled})
			for i := 0; i < 3000 && trainingAt(g, dog).Task.Carry == 0; i++ {
				g.Step(1)
			}
			task := trainingAt(g, dog).Task
			if task.Carry == 0 {
				t.Fatal("fixture: no carried stack")
			}
			at := g.positionOf(dog)
			switch how {
			case "down":
				a, _ := g.animals.Get(dog)
				g.hurt(dog, a.HP()*9/10, 0)
				g.Step(1)
				a, _ = g.animals.Get(dog)
				if !a.Downed {
					t.Fatalf("fixture: not downed %+v", a)
				}
			case "death":
				g.KillAnimal(dog)
			case "offmap":
				if !g.stashPawn(dog, entity.EntityID(99)) {
					t.Fatal("fixture: stash failed")
				}
			case "training_loss":
				m := trainingAt(g, dog)
				m.DecayAt = uint64(g.clock.Now()) + 1
				saveTraining(g, dog, m)
				for i := 0; i < 120 && trainingAt(g, dog).Task.Source != 0; i++ {
					at = g.positionOf(dog)
					g.Step(1)
				}
				a, _ := g.animals.Get(dog)
				if herd.TrainLearned(a, training.Haul, uint64(g.clock.Now()), trainingAt(g, dog)) {
					t.Fatal("fixture: training did not decay")
				}
			}
			if m := trainingAt(g, dog); m.Task.Source != 0 {
				t.Fatalf("task retained %+v", m)
			}
			if owner, ok := g.reserve.TileOwner(task.X, task.Y); ok && owner == dog {
				t.Fatal("tile reservation leaked")
			}
			st, ok := g.stacks.Get(task.Carry)
			if !ok || st.Carrier != 0 || st.Qty != 5 || st.X != at.X || st.Y != at.Y {
				t.Fatalf("bad interruption cargo: %+v want ground at %+v", st, at)
			}
		})
	}
}

func TestForbidInFlightRescue(t *testing.T) {
	g, doc, bo, hospital, _ := rescueScene(t)
	pickedUp(t, g, doc, bo)
	j, _ := g.jobs.Get(doc)
	if j.Dest != hospital {
		t.Fatalf("fixture dest %d != %d", j.Dest, hospital)
	}
	g.Enqueue(command.Command{Kind: command.SetForbidden, Target: uint64(hospital), Enabled: true})
	for i := 0; i < 2000 && g.care.Carry.Carrier(bo) == doc; i++ {
		g.Step(1)
		fed(g, doc)
	}
	b, _ := g.bldgs.Get(hospital)
	if !b.Forbidden || construction.BedSlot(b, uint64(bo)) >= 0 || g.bedUnder(bo) == hospital || g.care.Carry.Carrier(bo) != 0 {
		t.Fatalf("forbid did not terminate carry %+v", b)
	}
	if owner, ok := g.reserve.EntityOwner(hospital); ok && owner == doc {
		t.Fatal("forbidden bed claim leaked")
	}
	if v, _ := g.vital.Get(bo); v.Status == health.Dead {
		t.Fatal("fixture patient died")
	}
	if j, _ := g.jobs.Get(doc); j.Kind == job.Rescue && j.Active() && j.Dest == hospital {
		t.Fatal("stale job remains active")
	}
}

func TestAnimalHaulSkipsUnreachableNearestSource(t *testing.T) {
	g := feedField(24, 18)
	dog, err := g.SpawnAnimal("husky", 8, 8)
	if err != nil {
		t.Fatal(err)
	}
	g.TameAnimal(dog, "Carrier")
	setAnimal(g, dog, func(a *animal.State) { a.NextThink = 1 << 40 })
	wallRing(t, g, 9, 7, 11, 9)
	near := stack(t, g, item.Wood, 5, 10, 8)
	far := stack(t, g, item.Wood, 5, 4, 8)
	g.zones.Paint(zone.KindStockpile, zone.NewStockpile(), zone.RectOf(2, 3, 3, 4), nil)
	saveTraining(g, dog, training.Maintenance{DecayAt: 1 << 40, HaulProgress: training.Needed(training.Haul) | herd.TrainToggled})
	a, _ := g.animals.Get(dog)
	h := animalHaulHost{haulHost{g}}
	if !h.Visible(10, 8) || !h.Visible(4, 8) || !h.Allowed(dog, 10, 8) || !h.Allowed(dog, 4, 8) {
		t.Fatal("fixture: visibility or area")
	}
	blocked := h.GoTo(dog, a, 10, 8)
	open := h.GoTo(dog, a, 4, 8)
	if len(blocked.Path) != 0 || len(open.Path) == 0 {
		t.Fatalf("fixture: expected near blocked/far open; paths %v / %v", blocked.Path, open.Path)
	}
	for i := 0; i < 3000; i++ {
		g.Step(1)
		if trainingAt(g, dog).Task.Source == far {
			return
		}
	}
	st, _ := g.stacks.Get(far)
	a, _ = g.animals.Get(dog)
	if st.Carrier == dog || st.X != 4 || st.Y != 8 {
		return
	}
	t.Fatalf("reachable stack %d never chosen with inaccessible nearer stack %d; position %v, retry %d, task %+v", far, near, g.positionOf(dog), a.RetryAt, trainingAt(g, dog).Task)
}
