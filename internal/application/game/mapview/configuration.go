// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package mapview

import (
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"slices"
)

// Configuration is a detached, on-demand native configuration read for
// a single map at Tick. Source edit stamps identify applied commands,
// not the actor, authorisation, or ownership of the configuration.
type Configuration struct {
	Map     uint32
	Tick    uint64
	Benches []BenchConfig
	Pens    []PenConfig
	Troughs []TroughConfig
	// Mutations are successful edits in execution order on this map during
	// the last Step or ApplyPending. Not persisted; read before next batch.
	Mutations []command.ConfigMutation
}

type BenchConfig struct {
	ID          entity.EntityID
	Kind        construction.Kind
	X, Y        int
	WorkSpot    [2]int
	HasWorkSpot bool
	Bills       []recipe.Bill
	LastEdit    command.ConfigEdit
}

type PenConfig struct {
	PenView
	LastEdit command.ConfigEdit
	// Occupants are living animals physically inside the enclosed pen,
	// including animals not assigned to it.
	Occupants []entity.EntityID
}

type TroughConfig struct {
	ID, Pen entity.EntityID
	X, Y    int
	Filter  uint8
	// Initialized is false before the simulation first chooses its filter.
	Initialized bool
	LastEdit    command.ConfigEdit
}

func DetachPen(v PenView) PenView {
	v.Bits = slices.Clone(v.Bits)
	v.Settings.Species = slices.Clone(v.Settings.Species)
	v.Settings.AutoCut = slices.Clone(v.Settings.AutoCut)
	return v
}
