package game

import (
	"fmt"
	"strings"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/trait"
	"rimworld-go/internal/domain/research"
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
func FactionViewsOf(fs []faction.Faction) []FactionView {
	out := make([]FactionView, 0, len(fs))
	for _, f := range fs {
		out = append(out, factionView(f))
	}
	return out
}

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

// ---- scenario details (the panel's key-value lines) ----

func prettyKey(s string) string {
	s = strings.ReplaceAll(s, "_", " ")
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func itemLabel(k item.Kind) string { return prettyKey(k.String()) }

func techLabel(t faction.Tech) string {
	switch t {
	case faction.Neolithic:
		return "Neolithic"
	case faction.Spacer:
		return "Spacer"
	}
	return "Industrial"
}

func arrivalLabel(a scenario.Arrival) string {
	switch a {
	case scenario.Standing:
		return "Standing"
	case scenario.WalkIn:
		return "Walk in"
	}
	return "Drop pods"
}

func playerFactionLabel(t faction.Type) string {
	if t == faction.Clans {
		return "Clan"
	}
	return "Settlers"
}

func kvLine(k, v string) DetailLine { return DetailLine{Text: k + "\t" + v} }

func headingLine(s string) DetailLine { return DetailLine{Heading: true, Text: s} }

// stackLine is "Rifle ×2" for a starting or scattered stack.
func stackLine(s scenario.Stack, scattered bool) DetailLine {
	text := fmt.Sprintf("%s ×%d", itemLabel(s.Kind), s.Qty)
	tip := itemLabel(s.Kind)
	if scattered {
		where := "anywhere on the map"
		if s.Near {
			where = "near the start"
		}
		text += fmt.Sprintf(" in %d piles, %s", max(s.Piles, 1), where)
		tip += "\nScattered around the landing site."
	} else {
		tip += "\nIn the starting kit."
	}
	if s.Kind.IsFood() {
		tip += " Food."
	}
	return DetailLine{Text: text, Tip: tip}
}

// mergeStacks adds up stacks of the same item (starting kit lines).
func mergeStacks(in []scenario.Stack) []scenario.Stack {
	var out []scenario.Stack
	for _, s := range in {
		merged := false
		for i := range out {
			if out[i].Kind == s.Kind {
				out[i].Qty += s.Qty
				merged = true
				break
			}
		}
		if !merged {
			out = append(out, s)
		}
	}
	return out
}

// scenarioLines is the key-value body of a scenario's details panel.
func scenarioLines(d scenario.Def) []DetailLine {
	e := d.Apply()
	var out []DetailLine
	out = append(out, kvLine("Player faction", playerFactionLabel(d.Faction)))
	people := fmt.Sprintf("%d", d.People)
	if d.ChooseFrom > d.People {
		people += fmt.Sprintf(", chosen from %d candidates", d.ChooseFrom)
	}
	out = append(out, kvLine("Starting people", people))
	out = append(out, kvLine("Arrival", arrivalLabel(d.Arrival)))
	out = append(out, kvLine("Tech level", techLabel(d.Tech)))
	out = append(out, kvLine("Research cost", fmt.Sprintf("%d%% of normal", e.ResearchCostPct)))
	if len(e.Research) > 0 {
		out = append(out, headingLine("Research granted"))
		for _, p := range e.Research {
			def := research.DefOf(p)
			out = append(out, DetailLine{Text: def.Label, Tip: def.Label + "\n" + def.Desc})
		}
	}
	if items := mergeStacks(e.StartItems); len(items) > 0 {
		out = append(out, headingLine("Starting equipment and supplies"))
		for _, s := range items {
			out = append(out, stackLine(s, false))
		}
	}
	var animals []string
	for _, p := range d.Parts {
		if p.Kind == scenario.PartStartAnimal {
			animals = append(animals, p.Summary())
		}
	}
	if len(animals) > 0 {
		out = append(out, headingLine("Starting animals"))
		for _, a := range animals {
			out = append(out, DetailLine{Text: a})
		}
	}
	if len(e.Scatter) > 0 {
		out = append(out, headingLine("Scattered on the map"))
		for _, s := range e.Scatter {
			out = append(out, stackLine(s, true))
		}
	}
	var rules []DetailLine
	for _, p := range d.Parts {
		switch p.Kind {
		case scenario.PartForcedTrait:
			rules = append(rules, DetailLine{Text: p.Kind.Label() + ": " + p.Summary(), Tip: trait.DefOf(p.Trait).Label + "\n" + trait.DefOf(p.Trait).Desc})
		case scenario.PartCondition:
			rules = append(rules, DetailLine{Text: p.Kind.Label() + ": " + p.Summary(), Tip: p.Cond.Label() + "\n" + p.Cond.Desc()})
		case scenario.PartAgeRange, scenario.PartForcedCondition, scenario.PartNoClothes,
			scenario.PartScheduledIncident, scenario.PartDisabledIncident, scenario.PartDisallowBuilding:
			rules = append(rules, DetailLine{Text: p.Kind.Label() + ": " + p.Summary(), Tip: p.Kind.Label() + "\n" + p.Kind.Desc()})
		}
	}
	if len(rules) > 0 {
		out = append(out, headingLine("Special rules"))
		out = append(out, rules...)
	}
	return out
}
