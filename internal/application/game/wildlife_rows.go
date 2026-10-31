package game

import (
	"sort"

	"rimworld-go/internal/domain/animal"
)

// WildlifeRow is one wild species on the map, for the Animals tab's Wild
// list (UI checklist 19.4).
type WildlifeRow struct {
	Species     string // key
	Label       string
	Count       int
	Diet        string
	Predator    bool
	TameChance  int // percent per attempt
	Revenge     int // FOOD-3: percent a hurt one turns on its hunter (wiki, the difficulty's)
	Wildness    int // FOOD-3: percent (wiki); MinHandling the Animals level taming needs
	MinHandling int
	Meat        int // per animal
	Hunted      int // marked for hunting
	TameMarked  int // marked for taming
}

// Wildlife lists the wild species on the map, most numerous first (ties
// by key).
func (g *Game) Wildlife() []WildlifeRow {
	rows := map[animal.Species]*WildlifeRow{}
	for _, id := range g.animalIDs {
		a, ok := g.animals.Get(id)
		if !ok || a.Tame {
			continue
		}
		r := rows[a.Species]
		if r == nil {
			d := animal.DefOf(a.Species)
			r = &WildlifeRow{Species: a.Species.String(), Label: a.Species.Label(),
				Diet: animal.EcologyOf(a.Species).Diet.String(), Predator: d.Predator, TameChance: d.TameChance, Meat: d.Meat,
				Revenge: g.revengePct(d.Revenge), Wildness: animal.Wildness(a.Species) / 10, MinHandling: animal.MinHandling(animal.Wildness(a.Species))}
			rows[a.Species] = r
		}
		r.Count++
		if a.Designated {
			r.Hunted++
		}
		if a.TameMark {
			r.TameMarked++
		}
	}
	out := make([]WildlifeRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, *r)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Count > out[j].Count || out[i].Count == out[j].Count && out[i].Species < out[j].Species
	})
	return out
}
