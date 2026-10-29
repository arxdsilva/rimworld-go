package ebiten

import (
	"image/color"
	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/inspector"
	"rimworld-go/internal/adapters/inbound/ebiten/render/creature"
	"rimworld-go/internal/adapters/inbound/ebiten/render/cue"
	"rimworld-go/internal/adapters/inbound/ebiten/render/overlay"
	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/adapters/inbound/ebiten/render/world"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// Pawns and animals are drawn by render/creature (ARCH-004 wave 4.3):
// figures and poses, the animal body player, held items, work and combat
// animation, particles, bursts, graves and pyres, the overlays above
// creatures, floating numbers and the attack traces. The renderer keeps the
// creatures' visual records (pawnVisual, syncPawns), steps the layer once a
// frame (animate) and answers its lookups (creatureHost).

// faceAttack turns the visual of an attacker standing at (x, y) towards
// its latest target (creature.FaceAttack).
func faceAttack(v pawnVisual, traces []game.TraceView, x, y int) pawnVisual {
	v.facing = creature.FaceAttack(v.facing, traces, x, y)
	return v
}

// turretMuzzle is the muzzle of turret b, where its shots start.
func turretMuzzle(c *art.Catalog, b *game.BuildingView) (creature.Vec, bool) {
	p, ok := overlay.TurretMuzzle(c, b)
	return creature.Vec{X: p.X, Y: p.Y}, ok
}

// sprayFor is a hit's spray: 4 + damage/2 drops (at most 16), 1.25×
// faster for hits of 10 damage or more.
func sprayFor(dmg int) (n int, speed float64) {
	n = min(4+max(dmg, 0)/2, 16)
	speed = creature.BloodSpeed
	if dmg >= 10 {
		speed *= 1.25
	}
	return n, speed
}

// sprayKind is what a hit target bleeds: ichor for hive insects.
func sprayKind(w game.WorldView, target entity.EntityID) creature.PartKind {
	if target != 0 {
		for i := range w.Animals {
			if an := &w.Animals[i]; an.ID == target {
				if overlay.IsHiveSpecies(an.Species) {
					return creature.PartIchor
				}
				break
			}
		}
	}
	return creature.PartBlood
}

// animate runs once per presentation frame after the simulation step:
// the fires' particles (render/overlay), then the creature layer's
// footsteps, work strikes and combat effects.
func (r *Renderer) animate(w game.WorldView, pawns []game.PawnView, running bool) {
	if !running {
		return
	}
	p, v := r.pipe(), r.liveView()
	p.Overlays.Fires.EmitFireParticles(v, w)
	p.Overlays.Fires.EmitBlazeParticles(v, w)
	cl := p.CreatureCull()
	p.Creatures.Animate(v, &cl, w, pawns, running)
}

// interpAnimal is where an animal is drawn: its motion at animT, or its
// cell before the first sync (creature.Set.AnimalAt).
func (r *Renderer) interpAnimal(an game.AnimalView) (float64, float64) {
	return r.pipe().Creatures.AnimalAt(r.liveView(), an)
}

// creatureHost is what the creature layer reads of the renderer
// (creature.Host). The layer calls it inside a frame the renderer has
// prepared (Draw, or an entry point's liveView), so it reads the frame
// view as it is.
type creatureHost struct{ r *Renderer }

// FigureLayers lists p's sprites bottom first in art facing fs
// (pawnFigureLayers, portrait.go).
func (h creatureHost) FigureLayers(dst []creature.Layer, p game.PawnView, fs string) []creature.Layer {
	for _, l := range pawnFigureLayers(h.r.art, p, fs) {
		dst = append(dst, creature.Layer{Key: l.Key, Tint: l.Tint})
	}
	return dst
}

func (h creatureHost) PawnBlood(p game.PawnView) float32 {
	return h.r.wp.Overlays.PawnBlood(&h.r.view, h.r.wp.Pass(), p)
}

func (h creatureHost) AnimalBlood(an game.AnimalView) float32 {
	return h.r.wp.Overlays.AnimalBlood(&h.r.view, h.r.wp.Pass(), an)
}

func (h creatureHost) LayerBlood(key string, b float32) float32 { return overlay.LayerBlood(key, b) }
func (h creatureHost) ColonyCloths() []string                   { return h.r.wp.ColonyCloths() }
func (h creatureHost) ItemSprite(kind string) string            { return world.ItemSprite(kind) }

func (h creatureHost) Visual(id entity.EntityID) creature.Visual {
	v := h.r.last[id]
	return creature.Visual{Facing: v.facing, LyingSince: v.lyingSince}
}

func (h creatureHost) StoneItem(key, kind string, cs sprite.ColorScale) (string, sprite.ColorScale) {
	return h.r.wp.StoneItem(key, kind, cs)
}

func (h creatureHost) HostileMarker(dst *ebiten.Image, p game.PawnView, wx, wy, lift, tile float64) {
	h.r.wp.Lb.DrawHostileMarker(dst, p, wx, wy, lift, tile)
}

func (h creatureHost) HealthColor(pct int) color.RGBA { return inspector.HealthColor(pct) }

func (h creatureHost) TurretMuzzle(id entity.EntityID) (creature.Vec, bool) {
	if b, ok := h.r.wp.Overlays.BuildingByID(&h.r.view, id); ok && h.r.art != nil {
		return turretMuzzle(h.r.art, b)
	}
	return creature.Vec{}, false
}

func (h creatureHost) BuildingAt(w game.WorldView, x, y int) (game.BuildingView, bool) {
	return inspector.BuildingAt(w, x, y)
}

func (h creatureHost) Spray(w game.WorldView, target entity.EntityID, damage int) (creature.PartKind, int, float64) {
	n, speed := sprayFor(damage)
	return sprayKind(w, target), n, speed
}

func (h creatureHost) WorkCue(act string, x, y float64) {
	if id, ok := cue.ActionSounds[act]; ok {
		h.r.cueBus().Cue(id, x, y, 1)
	}
}

func (h creatureHost) BurstCue(x, y float64)       { h.r.cueBus().Cue(cue.VoiceBurst, x, y, cue.VolBurst) }
func (h creatureHost) TraceSound(t game.TraceView) { h.r.cueBus().TraceSound(t) }
