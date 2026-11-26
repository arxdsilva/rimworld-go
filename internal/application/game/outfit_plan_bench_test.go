// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"testing"

	"rimworld-go/internal/domain/economy/item"
)

func BenchmarkNativeOutfitPlanning(b *testing.B) {
	g := New(Config{Seed: 8, Width: 40, Height: 20})
	id, err := g.SpawnNamedPawn(5, 5, "Ada")
	if err != nil {
		b.Fatal(err)
	}
	for i, k := range []item.Kind{
		item.Parka, item.Duster, item.Tuque, item.TShirt, item.Pants,
		item.CowboyHat, item.FlakVest, item.FlakJacket, item.PlateArmor,
	} {
		if _, err := g.SpawnStack(k, 1, 10+i, 7); err != nil {
			b.Fatal(err)
		}
	}
	d := outfitOf(g)
	for _, mode := range []struct {
		name string
		full bool
	}{{"optimistic", false}, {"full", true}} {
		b.Run(mode.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				d.Plan(id, mode.full)
			}
		})
	}
}
