// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package windfall

import (
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/world/geology"
)

// Host is everything a windfall needs from the simulation.
type Host interface {
	Ground
	Goods
	Story
	Post
}

// Ground is the map around home.
type Ground interface {
	// Home is the colony's home cell.
	Home() (x, y int)
	// Colonists is how many colonists stand on the map.
	Colonists() int
	// MeteorFree reports whether a meteor cell may land on (x, y).
	MeteorFree(x, y int) bool
	// Rock raises rough rock bearing ore at (x, y).
	Rock(x, y int, ore geology.Ore)
	// DropFree reports whether a supply stack may land on (x, y): pod-free,
	// unroofed, unzoned, no item, and not beside a cell in taken.
	DropFree(x, y int, taken [][2]int) bool
}

// Goods lands items.
type Goods interface {
	// Drop lands one forbidden stack of qty k at (x, y).
	Drop(k item.Kind, qty, x, y int)
}

// Story is the storyteller's sky song state and the colonists' mood.
type Story interface {
	// SongEnds is the sky song's end tick while it plays at now, else 0.
	SongEnds(now uint64) uint64
	// StartSong starts the sky song at now lasting ticks; false when one
	// plays.
	StartSong(now, ticks uint64) bool
	// Cheer gives every colonist the sky song memory, expiring at until,
	// and returns how many it reached.
	Cheer(now, until uint64) int
}

// Post sends letters. The parameter names title and format are
// load-bearing: TestCatalogIsCurrent scans them.
type Post interface {
	// Letter posts a good letter at (x, y).
	Letter(title string, x, y int, format string, args ...any)
}

// Dice is the simulation's random stream (*randx.Rand satisfies it).
type Dice interface {
	IntN(n int) int
}
