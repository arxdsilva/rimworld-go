// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package maps

import (
	"fmt"
	"slices"
	"testing"

	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/caravan"
)

// TestCloneCopiesCaptives: a clone's captives are its own (#14).
func TestCloneCopiesCaptives(t *testing.T) {
	orig := Caravan{Captives: []entity.EntityID{7, 9}, Fed: 3}
	c := orig.Clone()
	c.Captives[0] = 42
	if !slices.Equal(orig.Captives, []entity.EntityID{7, 9}) {
		t.Fatalf("original captives changed through the clone: %v", orig.Captives)
	}
	if c.Fed != 3 {
		t.Fatalf("clone Fed = %d, want 3", c.Fed)
	}
}

// namesHost is a Host that only names people.
type namesHost struct{ Host }

func (namesHost) Name(id entity.EntityID) string  { return fmt.Sprint("p", id) }
func (namesHost) Cargo() Freight                  { return goodsOf{} }
func (namesHost) Captive(id entity.EntityID) bool { return id == 4 }

// TestViewsShowsTheWayHome: a caravan on its way back heads for home;
// members carry their names and health notes in order.
func TestViewsShowsTheWayHome(t *testing.T) {
	list := []Caravan{
		{C: caravan.Caravan{ID: 1, Members: []uint64{3, 4}, Tile: 5, Dest: 9, Progress: 250}, Home: 2},
		{C: caravan.Caravan{ID: 2, Members: []uint64{6}, Tile: 7, Dest: 8}, Home: 2, Back: true},
	}
	sick := func(id entity.EntityID) string {
		if id == 4 {
			return "flu"
		}
		return ""
	}
	got := Views(Of(&list, nil, namesHost{}), sick)
	if len(got) != 2 || got[0].Dest != 9 || got[0].Progress != 25 || got[1].Dest != 2 {
		t.Fatalf("views = %+v", got)
	}
	if !slices.Equal(got[0].Members, []string{"p3", "p4"}) || !slices.Equal(got[0].Health, []string{"", "flu"}) {
		t.Fatalf("members %v health %v", got[0].Members, got[0].Health)
	}
	if !slices.Equal(got[0].Prisoner, []bool{false, true}) {
		t.Fatalf("prisoner marks = %v", got[0].Prisoner)
	}
	if Views(Of(new([]Caravan), nil, namesHost{}), sick) != nil {
		t.Fatal("no caravans: want a nil list")
	}
}
