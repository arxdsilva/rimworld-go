// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package maps

import (
	"slices"
	"testing"

	"rimworld-go/internal/domain/sim/entity"
)

type siteRosterFixture struct {
	people, raiders, animals          []entity.EntityID
	prisoner, threat, tame, manhunter map[entity.EntityID]bool
	ore                               map[int]bool
}

func (f siteRosterFixture) Colonists() []entity.EntityID      { return f.people }
func (f siteRosterFixture) RaiderIDs() []entity.EntityID      { return f.raiders }
func (f siteRosterFixture) AnimalIDs() []entity.EntityID      { return f.animals }
func (f siteRosterFixture) Prisoner(id entity.EntityID) bool  { return f.prisoner[id] }
func (f siteRosterFixture) Threat(id entity.EntityID) bool    { return f.threat[id] }
func (f siteRosterFixture) Tame(id entity.EntityID) bool      { return f.tame[id] }
func (f siteRosterFixture) Manhunter(id entity.EntityID) bool { return f.manhunter[id] }
func (f siteRosterFixture) OreCell(i int) bool                { return f.ore[i] }

func TestCensusAndAboardShareNativeRosterRules(t *testing.T) {
	t.Parallel()
	f := siteRosterFixture{
		people:    []entity.EntityID{9, 3},
		raiders:   []entity.EntityID{5, 7},
		animals:   []entity.EntityID{11, 12, 13},
		prisoner:  map[entity.EntityID]bool{5: true},
		threat:    map[entity.EntityID]bool{7: true},
		tame:      map[entity.EntityID]bool{11: true},
		manhunter: map[entity.EntityID]bool{12: true},
		ore:       map[int]bool{1: true, 3: true},
	}
	got := CountSite(f, []int{1, 2, 3})
	if got.Ours != 4 || got.Hostiles != 2 || got.Ore != 2 {
		t.Fatalf("wrong map census: %+v", got)
	}
	people, animals := Aboard(f)
	if !slices.Equal(people, []entity.EntityID{3, 5, 9}) ||
		!slices.Equal(animals, []entity.EntityID{11}) {
		t.Fatalf("wrong people/animals: %v/%v", people, animals)
	}
	people[0] = 99
	if !slices.Equal(f.people, []entity.EntityID{9, 3}) {
		t.Fatal("census exposed the authoritative pawn slice")
	}
}
