// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"rimworld-go/internal/application/game/herd"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/animal/training"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
	"testing"
)

func TestFemaleLayerDoesNotInitiateMating(t *testing.T) {
	g := New(Config{Seed: 331, Width: 24, Height: 24})
	f1 := spawnSexed(t, g, "emu", 8, 8, true, true, "One")
	f2 := spawnSexed(t, g, "emu", 9, 8, true, true, "Two")
	for day := uint64(1); day <= 40; day++ {
		now := day*clock.TicksPerDay + animal.LayTick(animal.Emu, uint64(f1))
		advance(g, now)
		a, _ := g.animals.Get(f1)
		g.tickLife(f1, &a, now)
		if a.Mate != 0 || animalOf(g, f2).Mate != 0 {
			t.Fatalf("unfertilized female initiates mating at day %d: %d -> %d; partner mate=%d", day, f1, a.Mate, animalOf(g, f2).Mate)
		}
		g.animals.Set(f1, a)
	}
}

func TestTravelTransferPreservesMaintenance(t *testing.T) {
	g := New(Config{Seed: 332, Width: 24, Height: 24})
	pet := spawnSexed(t, g, "timber_wolf", 8, 8, false, true, "Traveler")
	want := training.Maintenance{DecayAt: 123456, TameLoss: 3}
	g.worldS().trainingMaintenance = map[entity.EntityID]training.Maintenance{pet: want}
	a, _ := g.animals.Get(pet)
	h := newTravelHost(g)
	h.RemoveAnimal(pet)
	h.PutAnimal(pet, a, 9, 8)
	if got := g.worldS().trainingMaintenance[pet]; got != want {
		t.Fatalf("travel takeAnimal/PutAnimal boundary erased maintenance: got %+v want %+v", got, want)
	}
}

func TestRetamedAnimalHasFreshMaintenance(t *testing.T) {
	g := New(Config{Seed: 333, Width: 24, Height: 24})
	pet := spawnSexed(t, g, "timber_wolf", 8, 8, false, true, "Returner")
	g.worldS().trainingMaintenance = map[entity.EntityID]training.Maintenance{pet: {DecayAt: 100, TameLoss: 4}}
	g.releaseAnimal(pet)
	g.TameAnimal(pet, "Returner")
	a, _ := g.animals.Get(pet)
	decayAnimalTraining(g, pet, &a, 100)
	if !a.Tame {
		t.Fatalf("freshly retamed animal immediately feral due to stale ownership maintenance: %+v", g.worldS().trainingMaintenance[pet])
	}
}

func TestRaccoonMaintenanceAvailable(t *testing.T) {
	g := New(Config{Seed: 334, Width: 24, Height: 24})
	id := spawnSexed(t, g, "raccoon", 8, 8, false, true, "Pet")
	a, _ := g.animals.Get(id)
	decayAnimalTraining(g, id, &a, 100)
	interval := training.DecayInterval(animal.Wildness(a.Species))
	for i := uint64(1); i <= 5; i++ {
		now := 100 + i*interval
		if g.trainable(id, &a, now) {
			return
		}
		decayAnimalTraining(g, id, &a, now)
	}
	if !a.Tame {
		t.Fatal("non-penned tame raccoon cannot receive any maintenance session and inevitably becomes wild")
	}
}

func TestForeignPackCannotBeTamed(t *testing.T) {
	g := visitorGame(t)
	if !g.caravanArrive(uint64(g.clock.Now())) {
		t.Fatal("no caravan")
	}
	id := caravanPack(g, g.trade.caravan)
	a, ok := g.animals.Get(id)
	if !ok || a.Tame {
		t.Fatal("invalid foreign pack fixture")
	}
	pos, _ := g.positions.Get(id)
	g.markTame(pos.X, pos.Y, pos.X, pos.Y, true)
	a, _ = g.animals.Get(id)
	if a.TameMark {
		t.Fatal("foreign pack marked tame")
	}
	g.TameAnimal(id, "captured")
	a, _ = g.animals.Get(id)
	if a.Tame {
		t.Fatal("foreign pack ownership changed")
	}
}

func TestRaccoonMaintenanceSession(t *testing.T) {
	g := New(Config{Seed: 334, Width: 24, Height: 24})
	handler, _ := g.SpawnNamedPawn(8, 7, "Handler")
	id := spawnSexed(t, g, "raccoon", 8, 8, false, true, "Pet")
	keepFed(g, id)
	interval := training.DecayInterval(animal.Wildness(animal.Raccoon))
	now := interval + uint64(herd.TrainCooldown) + 100
	advance(g, now)
	a, _ := g.animals.Get(id)
	a.TrainDecay = 1
	g.animals.Set(id, a)
	want := a.Trained
	g.worldS().trainingMaintenance[id] = training.Maintenance{DecayAt: now, TameLoss: 2}
	if !g.assignTrain(handler) {
		t.Fatal("maintenance handler job not admitted")
	}
	j, _ := g.jobs.Get(handler)
	if j.Kind != job.Train || j.Target != id {
		t.Fatalf("wrong maintenance job %+v", j)
	}
	// Execute the ordinary train job at its actual adjacent work cell.
	g.positions.Set(handler, Position{X: 8, Y: 7})
	j.State = job.Do
	j.WorkLeft = 1
	g.jobs.Set(handler, j)
	for i := 0; i < 1000; i++ {
		g.clock.Advance(1)
		j, _ = g.jobs.Get(handler)
		if j.State == job.Complete {
			break
		}
		g.doTrain(handler, j)
	}
	a, _ = g.animals.Get(id)
	m := g.worldS().trainingMaintenance[id]
	if a.Trained != want || !a.Tame || m.TameLoss != 0 || m.DecayAt <= now {
		t.Fatalf("maintenance changed skill or did not reset deadline: %+v %+v", a.Trained, m)
	}
}
