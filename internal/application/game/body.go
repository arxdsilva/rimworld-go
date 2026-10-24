package game

import (
	"slices"

	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/sim/entity"
)

// Bodies (ENG-003, M1): g.bodies holds each humanlike's body; nil until
// the first one is made. Saved as Bodies.

// exportBodies is the Bodies save field in id order (nil: none).
func (g *Game) exportBodies() []entry[body.Body] {
	return exportMap(g.bodies)
}

// importBodies restores exportBodies' state.
func (g *Game) importBodies(es []entry[body.Body]) {
	g.bodies = importMap(es)
}

// exportMap lists a pointer map's entries in id order (nil when empty).
func exportMap[T any](m map[entity.EntityID]*T) []entry[T] {
	if len(m) == 0 {
		return nil
	}
	out := make([]entry[T], 0, len(m))
	for id, v := range m {
		out = append(out, entry[T]{id, *v})
	}
	slices.SortFunc(out, func(a, b entry[T]) int {
		switch {
		case a.ID < b.ID:
			return -1
		case a.ID > b.ID:
			return 1
		}
		return 0
	})
	return out
}

// importMap rebuilds exportMap's map (nil when there are no entries).
func importMap[T any](es []entry[T]) map[entity.EntityID]*T {
	if len(es) == 0 {
		return nil
	}
	m := make(map[entity.EntityID]*T, len(es))
	for _, e := range es {
		v := e.V
		m[e.ID] = &v
	}
	return m
}
