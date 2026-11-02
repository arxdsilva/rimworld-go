// Package setupview builds the pre-game wizard's scenario details panel
// (NG-1 §2.2): the key-value lines of a scenario, as plain values the game
// package turns into its DetailLine read model. It was moved out of
// package game unchanged (AX-1 budget payer); it is pure and holds no
// state.
package setupview

import (
	"fmt"
	"strings"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/trait"
	"rimworld-go/internal/domain/research"
	"rimworld-go/internal/domain/scenario"
)

// Line is one details line: field for field game.DetailLine (a heading,
// "key\tvalue" text, a hover tip).
type Line struct {
	Heading bool
	Text    string
	Tip     string
}

// Pretty is a key as a label: underscores to spaces, first letter up.
func Pretty(s string) string {
	s = strings.ReplaceAll(s, "_", " ")
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// ItemLabel is an item kind as a label ("Herbal medicine").
func ItemLabel(k item.Kind) string { return Pretty(k.String()) }

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

func kvLine(k, v string) Line { return Line{Text: k + "\t" + v} }

func headingLine(s string) Line { return Line{Heading: true, Text: s} }

// stackLine is "Rifle ×2" for a starting or scattered stack.
func stackLine(s scenario.Stack, scattered bool) Line {
	text := fmt.Sprintf("%s ×%d", ItemLabel(s.Kind), s.Qty)
	tip := ItemLabel(s.Kind)
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
	return Line{Text: text, Tip: tip}
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

// Scenario is the key-value body of a scenario's details panel.
func Scenario(d scenario.Def) []Line {
	e := d.Apply()
	var out []Line
	out = append(out, kvLine("Player faction", playerFactionLabel(d.Faction)))
	people := fmt.Sprintf("%d", d.People)
	if d.ChooseFrom > d.People {
		people += fmt.Sprintf(", chosen from %d candidates", d.ChooseFrom)
	}
	out = append(out, kvLine("Starting people", people))
	out = append(out, kvLine("Arrival", arrivalLabel(d.Arrival)))
	out = append(out, kvLine("Tech level", research.LevelOf(d.Tech).String()))
	out = append(out, kvLine("Research cost", fmt.Sprintf("%d%% of normal", e.ResearchCostPct)))
	if len(e.Research) > 0 {
		out = append(out, headingLine("Research granted"))
		for _, p := range e.Research {
			def := research.DefOf(p)
			out = append(out, Line{Text: def.Label, Tip: def.Label + "\n" + def.Desc})
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
			out = append(out, Line{Text: a})
		}
	}
	if len(e.Scatter) > 0 {
		out = append(out, headingLine("Scattered on the map"))
		for _, s := range e.Scatter {
			out = append(out, stackLine(s, true))
		}
	}
	var rules []Line
	for _, p := range d.Parts {
		switch p.Kind {
		case scenario.PartForcedTrait:
			rules = append(rules, Line{Text: p.Kind.Label() + ": " + p.Summary(), Tip: trait.DefOf(p.Trait).Label + "\n" + trait.DefOf(p.Trait).Desc})
		case scenario.PartCondition:
			rules = append(rules, Line{Text: p.Kind.Label() + ": " + p.Summary(), Tip: p.Cond.Label() + "\n" + p.Cond.Desc()})
		case scenario.PartAgeRange, scenario.PartForcedCondition, scenario.PartNoClothes,
			scenario.PartScheduledIncident, scenario.PartDisabledIncident, scenario.PartDisallowBuilding:
			rules = append(rules, Line{Text: p.Kind.Label() + ": " + p.Summary(), Tip: p.Kind.Label() + "\n" + p.Kind.Desc()})
		}
	}
	if len(rules) > 0 {
		out = append(out, headingLine("Special rules"))
		out = append(out, rules...)
	}
	return out
}
