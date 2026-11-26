// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"rimworld-go/internal/application/game/saveutil"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// Feeding troughs (SIM-024): a pen's tame animals eat from them once the
// grass is gone; haulers fill them with feed matching the filter.

// Trough feed filters.
const (
	TroughPlants = animal.TroughPlants
	TroughMeat   = animal.TroughMeat
	TroughAny    = animal.TroughAny
)

var troughFilterNames = [...]string{"plants", "meat", "any"}

// troughState is what one trough holds (saved).
type troughState struct {
	Filter uint8
	Food   map[item.Kind]int
	Edit   command.ConfigEdit
}

// troughFill is how many food items trough id holds.
func (g *Game) troughFill(id entity.EntityID) int {
	n := 0
	for _, q := range g.troughs[id].Food {
		n += q
	}
	return n
}

// setTroughFilter applies a SetTroughFilter command.
func (g *Game) setTroughFilter(id entity.EntityID, f uint8) command.ConfigEdit {
	if f > TroughAny {
		return command.ConfigEdit{}
	}
	if _, ok := g.isTrough(id); !ok {
		return command.ConfigEdit{}
	}
	t := g.troughs[id]
	t.Filter = f
	t.Edit = command.ConfigEdit{Revision: t.Edit.Revision + 1, Tick: uint64(g.Tick()), Kind: command.SetTroughFilter}
	g.troughs[id] = t
	return t.Edit
}

func cloneSaveTrough(t troughState) troughState {
	t.Food = saveutil.OwnMap(true, t.Food)
	return t
}
