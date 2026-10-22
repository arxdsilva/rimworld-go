package game

import (
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/scenario"
	"rimworld-go/internal/domain/world/planet"
)

// Pre-game read models for the wizard (NG-1 §2.2, §2.7, §2.8). Owned by SETUP.

// BuiltinScenarios lists the built-in scenarios as views.
func BuiltinScenarios() []ScenarioView { return nil } // NG-1 stub (SETUP)

// ScenarioViewOf renders a scenario's details panel and problems.
func ScenarioViewOf(d scenario.Def) ScenarioView { return ScenarioView{Def: d} } // NG-1 stub (SETUP)

// NewPlanetBuilder starts generating a world (the wizard drives Step).
func NewPlanetBuilder(p planet.Params) *planet.Builder { return planet.NewBuilder(p) }

// FactionViewsOf is the Factions table for generated factions.
func FactionViewsOf(fs []faction.Faction) []FactionView { return nil } // NG-1 stub (SETUP)

// MapSizes lists the map size options, gated by MaxEnabledSide.
func MapSizes() []MapSizeView { return nil } // NG-1 stub (SETUP)
