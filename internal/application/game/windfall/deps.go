// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package windfall

import (
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/geology"
)

// Host is everything the windfalls need from the simulation, as
// consumer-owned roles (.agents/specs/SOLID_GO.md §7, §10). Every method
// takes and returns values.
type Host interface {
	Ground
	Goods
	Strangers
	Story
	Letters
}

// Ground is the map the windfalls land on.
type Ground interface {
	Home() (x, y int)
	Size() (w, h int)
	// PodSpot is a free landing tile near (cx, cy), not one of taken.
	PodSpot(cx, cy int, taken [][2]int) (x, y int, ok bool)
	// MeteorFree reports whether a meteor cell may land on (x, y): inside
	// the map's inset, unroofed, dry walkable ground with no plant,
	// building, frame, zone or stack on it and no pawn or building within 1.
	MeteorFree(x, y int) bool
	// Rock raises rough rock holding ore on (x, y) (granite where the
	// ground has no stone).
	Rock(x, y int, ore geology.Ore)
}

// Goods lands cargo.
type Goods interface {
	// Drop lands qty of k as forbidden stack(s) on (x, y).
	Drop(k item.Kind, qty, x, y int)
}

// Strangers is the crashed stranger's side of the colony.
type Strangers interface {
	Colonists() int
	// Crash puts a faction-less visitor on (x, y), hurt by a blunt blow
	// of hurt.
	Crash(x, y, hurt int) (id entity.EntityID, ok bool)
	Name(id entity.EntityID) string
}

// Story is the storyteller's timed events.
type Story interface {
	SongOn(now uint64) bool
	StartSong(now, ticks uint64) bool
}

// Letters tells the player.
type Letters interface {
	// Letter posts a good letter at (x, y), focused on focus (0: none).
	Letter(title string, x, y int, focus entity.EntityID, format string, args ...any)
}

// Dice rolls on the game's main stream (*randx.Rand satisfies it).
type Dice interface {
	IntN(n int) int
}
