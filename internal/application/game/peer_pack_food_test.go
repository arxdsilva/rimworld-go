// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"bytes"
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/needs"
)

func TestNativeHungryResidentEatsMealFromNearbyPeerPack(t *testing.T) {
	g := New(Config{Seed: 119, Width: 35, Height: 24})
	donor, err := g.SpawnNamedPawn(12, 12, "Donor")
	if err != nil {
		t.Fatal(err)
	}
	eater, err := g.SpawnNamedPawn(13, 12, "Eater")
	if err != nil {
		t.Fatal(err)
	}
	pack := senseOf(g).Gear.Pack(donor)
	pack.Add(item.Stack{Kind: item.Meal, Qty: 2})
	senseOf(g).Packs[donor] = pack
	g.SetNeeds(donor, needs.Full, needs.Full)
	g.SetNeeds(eater, 1000, needs.Full)
	if _, _, _, _, ok := g.peerPackedMeal(eater, eater); !ok || !g.seekFood(eater) {
		t.Fatal("starving resident must find an edible, reachable peer meal")
	}

	ate := false
	for i := 0; i < 1500; i++ {
		g.Step(1)
		if n, _ := g.need.Get(eater); n.Hunger > needs.EatAt {
			ate = true
			break
		}
	}
	if !ate {
		j, _ := g.jobs.Get(eater)
		t.Fatalf("resident never ate from the neighboring pack: job %+v", j)
	}
	remaining := senseOf(g).Gear.Pack(donor)
	if len(remaining.Items) != 1 || remaining.Items[0].Stack.Qty != 1 {
		t.Fatalf("peer pack consumed wrong quantity: %+v", remaining)
	}
	var buf bytes.Buffer
	if err := g.Save(&buf, "pack-food"); err != nil {
		t.Fatal(err)
	}
	h, _, err := Load(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if g.StateHash() != h.StateHash() {
		t.Fatal("peer-pack consumption changed during native save/load")
	}
	g.Step(30)
	h.Step(30)
	if g.StateHash() != h.StateHash() {
		t.Fatal("peer-pack consumption continued differently after save/load")
	}
}

func TestNativePeerPackedMealsRejectRottenAndUnreachable(t *testing.T) {
	g := New(Config{Seed: 120, Width: 35, Height: 24})
	donor, _ := g.SpawnNamedPawn(12, 12, "Donor")
	eater, _ := g.SpawnNamedPawn(13, 12, "Eater")
	p := senseOf(g).Gear.Pack(donor)
	p.Add(item.Stack{Kind: item.Meal, Qty: 2, Rot: item.Meal.ShelfTicks()})
	senseOf(g).Packs[donor] = p
	if _, _, _, _, ok := g.peerPackedMeal(eater, eater); ok {
		t.Fatal("rotten meal selected from peer pack")
	}

	isolated := New(Config{Seed: 121, Width: 35, Height: 24})
	stranded, _ := isolated.SpawnNamedPawn(25, 12, "PackOnOtherSide")
	hungry, _ := isolated.SpawnNamedPawn(8, 12, "Hungry")
	p = senseOf(isolated).Gear.Pack(stranded)
	p.Add(item.Stack{Kind: item.Meal, Qty: 2})
	senseOf(isolated).Packs[stranded] = p
	for y := 0; y < isolated.Height(); y++ {
		isolated.grid.SetWalkable(17, y, false)
	}
	isolated.Step(1)
	if _, _, _, _, ok := isolated.peerPackedMeal(hungry, hungry); ok {
		t.Fatal("unreachable donor's packed meal selected")
	}
}
