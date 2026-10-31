package game

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal"
)

// Animal products (SIM-026, plan 4.4): a grown producer's Product fills
// in tickLife (animal_life.go); handlers collect it (game/herd collect.go,
// moved there by FOOD-3: the species' wiki amounts and the handler's
// gather yield).

// collectVerb is the job line verb of a milking or shearing job ("collect
// ember oil" for an emberbuck).
func (g *Game) collectVerb(j job.Record) string {
	if a, ok := g.animals.Get(j.Target); ok && animal.EcologyOf(a.Species).Product == animal.EmberOil {
		return "collect ember oil"
	}
	return j.Kind.String()
}
