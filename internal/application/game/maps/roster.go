// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package maps

import (
	"slices"

	"rimworld-go/internal/domain/sim/entity"
)

// Roster reads one open map's authoritative residents, enemies and ore.
// The map package decides who counts toward a site's closure.
type Roster interface {
	Colonists() []entity.EntityID
	RaiderIDs() []entity.EntityID
	AnimalIDs() []entity.EntityID
	Prisoner(entity.EntityID) bool
	Threat(entity.EntityID) bool
	Tame(entity.EntityID) bool
	Manhunter(entity.EntityID) bool
	OreCell(int) bool
}

// CountSite checks whether people, threats or ore remain on a map.
func CountSite(r Roster, cells []int) Census {
	c := Census{Ours: len(r.Colonists())}
	for _, id := range r.RaiderIDs() {
		if r.Prisoner(id) {
			c.Ours++
		} else if r.Threat(id) {
			c.Hostiles++
		}
	}
	for _, id := range r.AnimalIDs() {
		if r.Tame(id) {
			c.Ours++
		} else if r.Manhunter(id) {
			c.Hostiles++
		}
	}
	for _, i := range cells {
		if r.OreCell(i) {
			c.Ore++
		}
	}
	return c
}

// Aboard picks the colony's people, captives and tame animals in stable order.
func Aboard(r Roster) (people, animals []entity.EntityID) {
	people = slices.Clone(r.Colonists())
	for _, id := range r.RaiderIDs() {
		if r.Prisoner(id) {
			people = append(people, id)
		}
	}
	slices.Sort(people)
	for _, id := range r.AnimalIDs() {
		if r.Tame(id) {
			animals = append(animals, id)
		}
	}
	return people, animals
}
