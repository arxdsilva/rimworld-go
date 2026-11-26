// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package axview

import (
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/world/filth"
)

// Buildable is the read model of one buildable definition in the
// Architect menu (game.BuildableView).
type Buildable struct {
	ID        uint8 // value for command.Command.Build
	Kind      string
	Wood      int
	Work      int
	Rotatable bool
	Floor     bool // floor layer: can sit under furniture
	Blocks    bool // completed building blocks movement
	StoneOK   bool // can be built from stone blocks
	W, H      int  // footprint facing south
	Bench     bool // runs bills
	// Stuffs are the materials it may be built from, default first ("wood",
	// "stone"; a carpet lists the carpet items). Carpet marks the entrance
	// carpet, installed from a crafted carpet item (SIM-021).
	Stuffs []string
	// Catches is the percent of tracked dirt each material wipes off,
	// parallel to Stuffs; 0 for non-carpets.
	Catches []int
	Carpet  bool
	// Locked is why this colony cannot build it ("" when it can):
	// (*game.Game).Buildables sets game.ReasonScenario for a disallowed building.
	Locked string
}

func Buildables() []Buildable {
	defs := construction.All()
	out := make([]Buildable, 0, len(defs))
	for _, d := range defs {
		out = append(out, Buildable{
			ID: uint8(d.Kind), Kind: d.Name, Wood: d.Wood, Work: d.Work, Rotatable: d.Rotatable,
			Floor: d.Layer == construction.LayerFloor, Blocks: d.BlocksMove, StoneOK: d.StoneOK,
			W: max(d.Size[0], 1), H: max(d.Size[1], 1), Bench: d.Bench,
		})
		for _, s := range construction.Stuffs(d.Kind) {
			out[len(out)-1].Stuffs = append(out[len(out)-1].Stuffs, s.String())
			out[len(out)-1].Catches = append(out[len(out)-1].Catches, filth.Catch(s)/10)
		}
		out[len(out)-1].Carpet = d.Carpet
	}
	return out
}
