package ebiten

import (
	"rimworld-go/internal/application/game"
)

// Light sources (PLAY-020 §3b, PLAY-021): every building the renderer
// lights the world with comes from appendLightSources, and nothing else in
// the package tests a building's kind for light. The light grid (light.go)
// and the creature-shadow point lights (shadow_cast.go) both read its
// list. Whether a building gives light is the simulation's call
// (BuildingView.Glow: a powered lamp, a burning torch or campfire), never
// a kind check here.

// lightKind is how a source lights: its falloff and its shadow height.
type lightKind uint8

const (
	lightLamp     lightKind = iota + 1 // a point lamp: smooth falloff, walls and rooms stop it, sprites shade toward it
	lightCampfire                      // a burning campfire: 1 − d/3.5 within 3 tiles, flat
)

// lightSource is one light: the building's id (shadow flicker and tie
// breaks), its tile and its reach in tiles.
type lightSource struct {
	ID     uint64
	X, Y   int
	Radius float64
	Kind   lightKind
}

// lampDefaultRadius is a lamp's reach when its sprite metadata has no
// light_radius.
const lampDefaultRadius = 6.0

// appendLightSources appends w's light sources to dst, in building order:
// every building whose view says it Glows. A campfire lights as a
// campfire (lightCampfire, campfireReach: the renderer keeps its 3-tile
// look, though the sim's GlowRadius for it is 5); anything else is a point
// lamp reaching lampRadius (the standing lamp's metadata light_radius) for
// a lamp, else its GlowRadius (a torch), else lampRadius. It reuses dst,
// so it allocates only while dst grows.
func appendLightSources(dst []lightSource, w *game.WorldView, lampRadius float64) []lightSource {
	for i := range w.Buildings {
		b := &w.Buildings[i]
		if !b.Glow {
			continue
		}
		if b.Kind == "campfire" {
			dst = append(dst, lightSource{ID: uint64(b.ID), X: b.X, Y: b.Y, Radius: campfireReach, Kind: lightCampfire})
			continue
		}
		reach := lampRadius
		if b.Kind != "lamp" && b.GlowRadius > 0 {
			reach = float64(b.GlowRadius)
		}
		dst = append(dst, lightSource{ID: uint64(b.ID), X: b.X, Y: b.Y, Radius: reach, Kind: lightLamp})
	}
	return dst
}

// lampRadiusOf is the lamp sprite's light_radius from c's metadata (read
// without decoding the sprite), lampDefaultRadius without one.
func lampRadiusOf(c *artCatalog) float64 {
	if c != nil {
		if sp := c.sprites[buildSprite("lamp", "", 0, false, false)]; sp != nil && sp.meta.LightRadius > 0 {
			return sp.meta.LightRadius
		}
	}
	return lampDefaultRadius
}
