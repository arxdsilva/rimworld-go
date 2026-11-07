package game

import (
	"rimworld-go/internal/application/game/factionview"
	"rimworld-go/internal/application/game/setupview"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/scenario"
	"rimworld-go/internal/domain/world/planet"
)

// Pre-game read models for the wizard (NG-1 §2.2, §2.7, §2.8). Owned by SETUP.

// BuiltinScenarios lists the built-in scenarios as views, in display order.
func BuiltinScenarios() []ScenarioView {
	defs := scenario.Builtins()
	out := make([]ScenarioView, 0, len(defs))
	for _, d := range defs {
		out = append(out, ScenarioViewOf(d))
	}
	return out
}

// ScenarioViewOf renders a scenario's details panel and problems.
func ScenarioViewOf(d scenario.Def) ScenarioView {
	return ScenarioView{
		Key: d.Key, Name: d.Name, Summary: d.Summary, Builtin: d.Builtin,
		Hardness: d.HardnessPips(), Problems: d.Validate(), Lines: scenarioLines(d), Def: d,
	}
}

// NewPlanetBuilder starts generating a world (the wizard drives Step).
func NewPlanetBuilder(p planet.Params) *planet.Builder { return planet.NewBuilder(p) }

// FactionViewsOf is the Factions table for generated factions (hidden
// types such as the Swarm are marked Hidden).
func FactionViewsOf(fs []faction.Faction) []FactionView { return factionview.All(fs) }

// Map size options (NG-1 §2.6, §6.9).
var mapSides = [...]int{200, 225, 250, 275, 300, 350, 400, 450, 500}

const (
	preReleaseSide = 120
	largeSide      = 350
	reasonMapGate  = "Not available yet" // NG-1 §2.7: shown until the engine gate passes
)

// MapSizes lists the map size options, gated by MaxEnabledSide; while no
// listed size is enabled a 120 "pre-release" size leads the list.
func MapSizes() []MapSizeView {
	var out []MapSizeView
	if MaxEnabledSide < mapSides[0] {
		out = append(out, MapSizeView{Side: preReleaseSide, Enabled: true, Reason: "pre-release"})
	}
	for _, s := range mapSides {
		v := MapSizeView{Side: s, Enabled: s <= MaxEnabledSide, Large: s >= largeSide}
		if !v.Enabled {
			v.Reason = reasonMapGate
		}
		out = append(out, v)
	}
	return out
}

// ---- scenario details: game/setupview builds the panel's lines ----

func itemLabel(k item.Kind) string { return setupview.ItemLabel(k) }

// scenarioLines is the key-value body of a scenario's details panel.
func scenarioLines(d scenario.Def) []DetailLine { return setupview.Scenario(d) }
