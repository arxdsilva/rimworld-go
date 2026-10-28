package light

import (
	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/application/game"
)

// Light sources (PLAY-020 §3b, PLAY-021): every building the renderer
// lights the world with comes from AppendSources, and nothing else in
// the package tests a building's kind for light. The light grid (light.go)
// and the creature-shadow point lights (shadow_cast.go) both read its
// list. Whether a building gives light is the simulation's call
// (BuildingView.Glow: a powered lamp, a burning torch or campfire), never
// a kind check here.

// Kind is how a source lights: its falloff and its shadow height.
type Kind uint8

const (
	Lamp     Kind = iota + 1 // a point lamp: smooth falloff, walls and rooms stop it, sprites shade toward it
	Campfire                 // a burning campfire: 1 − d/3.5 within 3 tiles, flat
)

// Source is one light: the building's id (shadow flicker and tie
// breaks), its tile and its reach in tiles.
type Source struct {
	ID     uint64
	X, Y   int
	Radius float64
	Kind   Kind
}

// LampDefaultRadius is a lamp's reach when its sprite metadata has no
// light_radius.
const LampDefaultRadius = 6.0

// AppendSources appends w's light sources to dst, in building order:
// every building whose view says it Glows. A campfire lights as a
// campfire (Campfire, CampfireReach: the renderer keeps its 3-tile
// look, though the sim's GlowRadius for it is 5); anything else is a point
// lamp reaching lampRadius (the standing lamp's metadata light_radius) for
// a lamp, else its GlowRadius (a torch), else lampRadius. It reuses dst,
// so it allocates only while dst grows.
func AppendSources(dst []Source, w *game.WorldView, lampRadius float64) []Source {
	for i := range w.Buildings {
		b := &w.Buildings[i]
		if !b.Glow {
			continue
		}
		if b.Kind == "campfire" {
			dst = append(dst, Source{ID: uint64(b.ID), X: b.X, Y: b.Y, Radius: CampfireReach, Kind: Campfire})
			continue
		}
		reach := lampRadius
		if b.Kind != "lamp" && b.GlowRadius > 0 {
			reach = float64(b.GlowRadius)
		}
		dst = append(dst, Source{ID: uint64(b.ID), X: b.X, Y: b.Y, Radius: reach, Kind: Lamp})
	}
	return dst
}

// LampSprite is the standing lamp's sprite key (the renderer's
// buildSprite("lamp", …)), whose metadata light_radius is a lamp's reach.
const LampSprite = "furniture/lamp_standing"

// LampRadiusOf is the lamp sprite's light_radius from c's metadata (read
// without decoding the sprite), LampDefaultRadius without one.
func LampRadiusOf(c *art.Catalog) float64 {
	if c != nil {
		if sp := c.Sprites[LampSprite]; sp != nil && sp.Meta.LightRadius > 0 {
			return sp.Meta.LightRadius
		}
	}
	return LampDefaultRadius
}
