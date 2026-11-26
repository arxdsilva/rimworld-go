// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"testing"

	"rimworld-go/internal/domain/world/plant"
)

func TestPlantGrowthViewReportsCurrentGrowthInputs(t *testing.T) {
	g := New(Config{Seed: 91, Width: 24, Height: 20})
	id, err := g.SpawnPlantSpecies("rice", 5, 5, 0)
	if err != nil {
		t.Fatal(err)
	}
	p, ok := g.plants.Get(id)
	if !ok {
		t.Fatal("missing planted rice")
	}
	v, ok := g.PlantGrowth(id)
	if !ok {
		t.Fatal("missing growth view")
	}
	at := int(g.temperatureAt(5, 5))
	now := g.growNowOf()
	if v.Fertility != plantFertility(g, p) ||
		v.Temperature != at || v.TemperatureFactor != plant.TempFactor(at) ||
		v.Rate != g.growRate(p, at, now.gp, v.Light) ||
		v.LightFactor != plant.LightFactor(v.Light) {
		t.Fatalf("view %+v does not match real growth inputs", v)
	}
}
