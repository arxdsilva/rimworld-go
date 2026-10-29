package ebiten

import (
	"image/color"
	artpkg "rimworld-go/internal/adapters/inbound/ebiten/art"
	camerapkg "rimworld-go/internal/adapters/inbound/ebiten/camera"
	"rimworld-go/internal/adapters/inbound/ebiten/inspector"
	creaturepkg "rimworld-go/internal/adapters/inbound/ebiten/render/creature"
	"rimworld-go/internal/adapters/inbound/ebiten/render/cue"
	motionpkg "rimworld-go/internal/adapters/inbound/ebiten/render/motion"
	"rimworld-go/internal/adapters/inbound/ebiten/render/overlay"
	"rimworld-go/internal/adapters/inbound/ebiten/render/world"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/adapters/inbound/ebiten/render/creature"
	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// Pawns and animals are drawn by render/creature (ARCH-004 wave 4.3):
// figures and poses, the animal body player, held items, work and combat
// animation, particles, bursts, graves and pyres, the overlays above
// creatures, floating numbers and the attack traces. These aliases and
// one-line wrappers keep the root's call sites unchanged; the last mover
// deletes this file (SOLID_GO.md §11). The renderer's anims field holds
// the layer (animState is creature.Set).

// faceAttack turns the visual of an attacker standing at (x, y) towards
// its latest target (creature.FaceAttack).
func faceAttack(v pawnVisual, traces []game.TraceView, x, y int) pawnVisual {
	v.facing = creature.FaceAttack(v.facing, traces, x, y)
	return v
}

// creatures is the renderer's creature layer, built on first use over
// the renderer's painter, motion state and light layer, with its live art
// and its particles, floating numbers and item pops (renderer fields
// other root code still reads) wired in.
func (r *Renderer) creatures() *creature.Set {
	if r.anims == nil {
		p := r.painter()
		r.anims = creature.New(&p, &r.mo, r.litLayer())
		r.anims.Host = creatureHost{r}
		r.anims.Parts, r.anims.Floaters, r.anims.ItemBorn = &r.parts, &r.floaters, &r.itemBorn
	}
	r.anims.Paint.Art = r.art
	return r.anims
}

// creatureView is the view the creature layer reads (spriteView: the
// renderer's live camera, which a bake moves) with the animation clock,
// tick and effects detail copied in: tests and the advance drive these
// entry points without Renderer.Draw filling the view. The caller puts
// frame back after the call.
func (r *Renderer) creatureView() (v *render.View, frame camerapkg.Camera) {
	v, frame = r.spriteView()
	v.AnimT, v.Fx = r.animT(), r.fx
	if r.game != nil {
		v.Tick = r.game.Tick()
	}
	return v, frame
}

// creatureCull is the frame's culling for the creature layer.
func (r *Renderer) creatureCull() creature.Cull {
	l := &r.lod
	return creature.Cull{X0: l.VX0, Y0: l.VY0, X1: l.VX1, Y1: l.VY1, ViewSet: l.ViewSet,
		MapTier: l.Tier == world.Map, MotionPad: motionViewPad, MinTile: world.CreatureMinTile}
}

// creatureMarks is creatureCull with the animal pads at tile and the
// white pixel the marks draw with.
func (r *Renderer) creatureMarks(tile float64) creature.Cull {
	cl := r.creatureCull()
	cl.Pad, cl.PadS = r.animalPads(tile)
	cl.White = r.lod.WhiteImg()
	return cl
}

// creatureHost is what the creature layer reads of the renderer
// (creature.Host).
type creatureHost struct{ r *Renderer }

func (h creatureHost) FigureLayers(dst []creature.Layer, p game.PawnView, fs string) []creature.Layer {
	for _, l := range h.r.pawnFigureLayers(p, fs) {
		dst = append(dst, creature.Layer{Key: l.Key, Tint: l.Tint})
	}
	return dst
}

func (h creatureHost) PawnBlood(p game.PawnView) float32        { return h.r.pawnBlood(p) }
func (h creatureHost) AnimalBlood(an game.AnimalView) float32   { return h.r.animalBlood(an) }
func (h creatureHost) LayerBlood(key string, b float32) float32 { return overlay.LayerBlood(key, b) }
func (h creatureHost) ColonyCloths() []string                   { return h.r.pipe().ColonyCloths() }
func (h creatureHost) ItemSprite(kind string) string            { return world.ItemSprite(kind) }

func (h creatureHost) Visual(id entity.EntityID) creature.Visual {
	v := h.r.last[id]
	return creature.Visual{Facing: v.facing, LyingSince: v.lyingSince}
}

func (h creatureHost) StoneItem(key, kind string, cs sprite.ColorScale) (string, sprite.ColorScale) {
	return h.r.pipe().StoneItem(key, kind, cs)
}

func (h creatureHost) HostileMarker(dst *ebiten.Image, p game.PawnView, wx, wy, lift, tile float64) {
	h.r.pipe().Lb.DrawHostileMarker(dst, p, wx, wy, lift, tile)
}

func (h creatureHost) HealthColor(pct int) color.RGBA { return inspector.HealthColor(pct) }

func (h creatureHost) TurretMuzzle(id entity.EntityID) (creaturepkg.Vec, bool) {
	if b, ok := h.r.buildingByID(id); ok && h.r.art != nil {
		return turretMuzzle(h.r.art, b)
	}
	return creaturepkg.Vec{}, false
}

func (h creatureHost) BuildingAt(w game.WorldView, x, y int) (game.BuildingView, bool) {
	return inspector.BuildingAt(w, x, y)
}

func (h creatureHost) Spray(w game.WorldView, target entity.EntityID, damage int) (creaturepkg.PartKind, int, float64) {
	n, speed := sprayFor(damage)
	return sprayKind(w, target), n, speed
}

func (h creatureHost) WorkCue(act string, x, y float64) {
	if id, ok := cue.ActionSounds[act]; ok {
		h.r.cue(id, x, y, 1)
	}
}

func (h creatureHost) BurstCue(x, y float64)       { h.r.cue(cue.VoiceBurst, x, y, cue.VolBurst) }
func (h creatureHost) TraceSound(t game.TraceView) { h.r.cueBus().TraceSound(t) }

// --- the layer's entry points under their old names ----------------------

func (r *Renderer) drawSpritePose(screen *ebiten.Image, key string, tint color.NRGBA, wx, wy, tile float64, ps motionpkg.Pose) {
	c := r.creatures()
	v, frame := r.creatureView()
	c.DrawSpritePose(v, r.spritePass(), screen, key, tint, wx, wy, tile, ps)
	v.Cam = frame
}

func (r *Renderer) burst(kind creaturepkg.PartKind, x, y float64, n int, seed uint64) {
	c := r.creatures()
	v, frame := r.creatureView()
	c.Burst(v, kind, x, y, n, seed)
	v.Cam = frame
}

// animate runs once per presentation frame after the simulation step:
// the fires' particles (fire.go, fire_world_draw.go), then the creature
// layer's footsteps, work strikes and combat effects.
func (r *Renderer) animate(w game.WorldView, pawns []game.PawnView, running bool) {
	if !running {
		return
	}
	r.emitFireParticles(w)
	r.emitBlazeParticles(w)
	c, cl := r.creatures(), r.creatureCull()
	v, frame := r.creatureView()
	c.Animate(v, &cl, w, pawns, running)
	v.Cam = frame
}

func (r *Renderer) syncItems(w game.WorldView) {
	c := r.creatures()
	v, frame := r.creatureView()
	c.SyncItems(v, w)
	v.Cam = frame
}

func (r *Renderer) pawnFigurePoseOf(p game.PawnView) creaturepkg.FigurePose {
	c := r.creatures()
	v, frame := r.creatureView()
	fp := c.FigurePoseOf(v, p)
	v.Cam = frame
	return fp
}

func (r *Renderer) drawPawnFigure(dst *ebiten.Image, p game.PawnView, wx, wy, tile float64, fp creaturepkg.FigurePose) {
	c := r.creatures()
	v, frame := r.creatureView()
	c.DrawPawnFigure(v, r.spritePass(), dst, p, wx, wy, tile, fp)
	v.Cam = frame
}

func (r *Renderer) drawFigureIn(screen *ebiten.Image, p game.PawnView, f artpkg.Facing, wx, wy, tile float64, ps motionpkg.Pose) {
	c := r.creatures()
	v, frame := r.creatureView()
	c.DrawFigureIn(v, r.spritePass(), screen, p, f, wx, wy, tile, ps)
	v.Cam = frame
}

func (r *Renderer) hitAge(id entity.EntityID, hitTick uint64, x, y int) (float64, bool) {
	c := r.creatures()
	v, frame := r.creatureView()
	age, ok := c.HitAge(v, id, hitTick, x, y)
	v.Cam = frame
	return age, ok
}

func (r *Renderer) interpAnimal(an game.AnimalView) (float64, float64) {
	c := r.creatures()
	v, frame := r.creatureView()
	x, y := c.AnimalAt(v, an)
	v.Cam = frame
	return x, y
}

func (r *Renderer) drawTraces(screen *ebiten.Image, w game.WorldView, tile float64) {
	c := r.creatures()
	v, frame := r.creatureView()
	c.DrawTraces(v, screen, w, r.game.Snapshot(), tile)
	v.Cam = frame
}

func (r *Renderer) animalBaseKey(an game.AnimalView) string {
	c := r.creatures()
	v, frame := r.creatureView()
	k := c.AnimalBaseKey(v, an)
	v.Cam = frame
	return k
}

func (r *Renderer) animalMirrored(an game.AnimalView) bool {
	c := r.creatures()
	v, frame := r.creatureView()
	m := c.AnimalMirrored(v, an)
	v.Cam = frame
	return m
}

func (r *Renderer) drawAnimalFigure(dst *ebiten.Image, an game.AnimalView, wx, wy, tile float64) {
	c := r.creatures()
	v, frame := r.creatureView()
	c.DrawAnimalFigure(v, r.spritePass(), dst, an, wx, wy, tile)
	v.Cam = frame
}

// onThreatPause brings an off-screen chase into the colony viewport when
// its warning has just paused the game (creature.Set.OnThreatPause).
func (r *Renderer) onThreatPause(w game.WorldView, tick uint64) {
	c := r.creatures()
	vp := r.cueBus().Voice.View
	v, frame := r.creatureView()
	x, y, ok := c.OnThreatPause(v, w, r.game.Snapshot(), tick, vp)
	v.Cam = frame
	if ok {
		r.centerOnView(x, y, vp, vp)
	}
}

func (r *Renderer) emitWorldBursts(w game.WorldView) {
	c := r.creatures()
	v, frame := r.creatureView()
	c.EmitWorldBursts(v, w)
	v.Cam = frame
}
