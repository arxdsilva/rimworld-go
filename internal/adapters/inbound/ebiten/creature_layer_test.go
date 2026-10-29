package ebiten

import (
	artpkg "rimworld-go/internal/adapters/inbound/ebiten/art"
	creaturepkg "rimworld-go/internal/adapters/inbound/ebiten/render/creature"
	motionpkg "rimworld-go/internal/adapters/inbound/ebiten/render/motion"
	spritepkg "rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/adapters/inbound/ebiten/render/world"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/render/creature"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// itemSpriteAt is itemSpriteSpecies without a species.
func itemSpriteAt(kind string, x, y int) string { return world.ItemSpriteSpecies(kind, "", x, y) }

func (r *Renderer) stepDust(_ *creaturepkg.Set, id entity.EntityID, m motionpkg.MoveState, heavy bool) {
	c, v := r.pipe().Creatures, r.liveView()
	c.StepDust(v, id, m, heavy)
}

// drawParticlesNoScreen ages particles without drawing (tests).
func (r *Renderer) drawParticlesNoScreen() {
	c, v := r.pipe().Creatures, r.liveView()
	c.AgeParticles(v)
}

func (r *Renderer) standingPose(p game.PawnView, m motionpkg.MoveState) (motionpkg.Pose, creaturepkg.HeldItem) {
	c, v := r.pipe().Creatures, r.liveView()
	ps, h := c.StandingPose(v, p, m)
	return ps, h
}

func (r *Renderer) drawFigure(screen *ebiten.Image, p game.PawnView, wx, wy, tile float64, ps motionpkg.Pose) {
	r.pipe().Creatures.DrawFigureIn(r.liveView(), r.pipe().Pass(), screen, p, artpkg.FaceSouth, wx, wy, tile, ps)
}

func (r *Renderer) drawLyingPawn(screen *ebiten.Image, p game.PawnView, vis pawnVisual, wx, wy, tile float64) {
	c, v := r.pipe().Creatures, r.liveView()
	c.DrawLyingPawn(v, r.pipe().Pass(), screen, p, creature.Visual{Facing: vis.facing, LyingSince: vis.lyingSince}, wx, wy, tile)
}

func (r *Renderer) animateWork(w game.WorldView, pawns []game.PawnView) {
	c, v := r.pipe().Creatures, r.liveView()
	c.AnimateWork(v, w, pawns)
}

func (r *Renderer) carryPose(p game.PawnView, m motionpkg.MoveState) creaturepkg.ActionPose {
	c, v := r.pipe().Creatures, r.liveView()
	a := c.CarryPose(v, p, m)
	return a
}

func (r *Renderer) workPose(p game.PawnView, m motionpkg.MoveState) creaturepkg.ActionPose {
	c, v := r.pipe().Creatures, r.liveView()
	a := c.WorkPose(v, p, m)
	return a
}

func (r *Renderer) rigOf(p game.PawnView, f artpkg.Facing) creaturepkg.HeldRig {
	c, v := r.pipe().Creatures, r.liveView()
	rig := c.RigOf(v, p, f)
	return rig
}

func (r *Renderer) heldPlace(rig creaturepkg.HeldRig, h creaturepkg.HeldItem, wx, wy, tile float64, ps motionpkg.Pose) (creaturepkg.HeldPlacement, bool) {
	c, v := r.pipe().Creatures, r.liveView()
	pl, ok := c.HeldPlace(v, rig, h, wx, wy, tile, ps)
	return pl, ok
}

func (r *Renderer) drawHeld(screen *ebiten.Image, rig creaturepkg.HeldRig, h creaturepkg.HeldItem, wx, wy, tile float64, ps motionpkg.Pose, behind bool) {
	c, v := r.pipe().Creatures, r.liveView()
	c.DrawHeld(v, r.pipe().Pass(), screen, rig, h, wx, wy, tile, ps, behind)
}

func (r *Renderer) heldPoint(rig creaturepkg.HeldRig, h creaturepkg.HeldItem, wx, wy float64, ps motionpkg.Pose, attach string) (creaturepkg.Vec, bool) {
	c, v := r.pipe().Creatures, r.liveView()
	pt, ok := c.HeldPoint(v, rig, h, wx, wy, ps, attach)
	return pt, ok
}

func (r *Renderer) hitReaction(p game.PawnView, m motionpkg.MoveState) creaturepkg.ActionPose {
	c, v := r.pipe().Creatures, r.liveView()
	a := c.HitReaction(v, p, m)
	return a
}

func (r *Renderer) combatPose(p game.PawnView, m motionpkg.MoveState) creaturepkg.ActionPose {
	c, v := r.pipe().Creatures, r.liveView()
	a := c.CombatPose(v, p, m)
	return a
}

func (r *Renderer) combatOffset(p game.PawnView) (dx, dy float64, flash float32) {
	c, v := r.pipe().Creatures, r.liveView()
	dx, dy, flash = c.CombatOffset(v, p)
	return dx, dy, flash
}

func (r *Renderer) fallPose(p game.PawnView, vis pawnVisual) motionpkg.Pose {
	c, v := r.pipe().Creatures, r.liveView()
	ps := c.FallPose(v, p, creature.Visual{Facing: vis.facing, LyingSince: vis.lyingSince})
	return ps
}

func (r *Renderer) pawnHeldPoint(p game.PawnView, attach string) (creaturepkg.Vec, bool) {
	c, v := r.pipe().Creatures, r.liveView()
	pt, ok := c.PawnHeldPoint(v, p, attach)
	return pt, ok
}

func (r *Renderer) shotPoints(t game.TraceView, pawns []game.PawnView) (muzzle, port creaturepkg.Vec) {
	c, v := r.pipe().Creatures, r.liveView()
	muzzle, port = c.ShotPoints(v, t, pawns)
	return muzzle, port
}

func (r *Renderer) animateCombat(w game.WorldView, pawns []game.PawnView) {
	c, v := r.pipe().Creatures, r.liveView()
	c.AnimateCombat(v, w, pawns)
}

// creatureMarks is the frame's creature culling with the animal pads at
// tile and the white pixel the marks draw with.
func (r *Renderer) creatureMarks(tile float64) creaturepkg.Cull {
	p := r.pipe()
	cl := p.CreatureCull()
	cl.Pad, cl.PadS = r.animalPads(tile)
	cl.White = p.LOD.WhiteImg()
	return cl
}

func (r *Renderer) drawAnimalOverlays(screen *ebiten.Image, w game.WorldView, tile float64, sw, sh int) {
	c, cl := r.pipe().Creatures, r.creatureMarks(tile)
	v := r.liveView()
	c.DrawAnimalOverlays(v, r.pipe().Pass(), &cl, screen, w, tile)
}

func (r *Renderer) traceList(w game.WorldView) []creaturepkg.ReactTrace {
	c, v := r.pipe().Creatures, r.liveView()
	rs := c.TraceList(v, w)
	return rs
}

func (r *Renderer) traceSpots(w game.WorldView) map[[2]int]creaturepkg.Vec {
	c, v := r.pipe().Creatures, r.liveView()
	spots := c.TraceSpots(v, w, r.game.Snapshot())
	return spots
}

func (r *Renderer) traceMuzzle(tr game.TraceView) (creaturepkg.Vec, bool) {
	c, v := r.pipe().Creatures, r.liveView()
	pt, ok := c.TraceMuzzle(v, tr)
	return pt, ok
}

func (r *Renderer) meleeTrail(tr game.TraceView) []creaturepkg.Vec {
	c, v := r.pipe().Creatures, r.liveView()
	pts := c.MeleeTrail(v, tr)
	return pts
}

func (r *Renderer) animalFrameKey(an game.AnimalView) string {
	c, v := r.pipe().Creatures, r.liveView()
	k := c.AnimalFrameKey(v, an)
	return k
}

func (r *Renderer) animalFrame(an game.AnimalView) string {
	c, v := r.pipe().Creatures, r.liveView()
	k := c.AnimalFrame(v, an)
	return k
}

func (r *Renderer) animalRestFallback(an game.AnimalView) bool {
	c, v := r.pipe().Creatures, r.liveView()
	ok := c.AnimalRestFallback(v, an)
	return ok
}

func (r *Renderer) animalFigureScale(an game.AnimalView) (spritepkg.ColorScale, bool) {
	c, v := r.pipe().Creatures, r.liveView()
	cs, ok := c.AnimalFigureScale(v, an)
	return cs, ok
}

func (r *Renderer) animalGait(an game.AnimalView) float64 {
	c, v := r.pipe().Creatures, r.liveView()
	g := c.AnimalGait(v, an)
	return g
}

func (r *Renderer) animalShow(an game.AnimalView) creaturepkg.BodyShow {
	c, v := r.pipe().Creatures, r.liveView()
	s := c.AnimalShow(v, an)
	return s
}

func (r *Renderer) animalTop(an game.AnimalView, tile float64) float64 {
	c, v := r.pipe().Creatures, r.liveView()
	h := c.AnimalTop(v, an, tile, world.CreatureMinTile)
	return h
}

func (r *Renderer) animalPose(an game.AnimalView) motionpkg.Pose {
	c, v := r.pipe().Creatures, r.liveView()
	ps := c.AnimalPose(v, an)
	return ps
}

func (r *Renderer) floatTrace(t game.TraceView, seed uint64) {
	c, v := r.pipe().Creatures, r.liveView()
	c.FloatTrace(v, t, seed)
}

func (r *Renderer) drawFloaters(screen *ebiten.Image, tile float64) {
	c, v := r.pipe().Creatures, r.liveView()
	c.DrawFloaters(v, screen, tile)
}

func (r *Renderer) workBarRects(pawns []game.PawnView, tile float64, dst []creaturepkg.WorkBarRect) []creaturepkg.WorkBarRect {
	c, cl, v := r.pipe().Creatures, r.pipe().CreatureCull(), r.liveView()
	dst = c.WorkBarRects(v, &cl, pawns, tile, dst)
	return dst
}

func (r *Renderer) drawWorkProgress(screen *ebiten.Image, pawns []game.PawnView, tile float64) {
	c, cl, v := r.pipe().Creatures, r.pipe().CreatureCull(), r.liveView()
	c.DrawWorkProgress(v, &cl, screen, pawns, tile)
}

func (r *Renderer) deceasedOf(it game.ItemView) (game.PawnView, bool) {
	c, v := r.pipe().Creatures, r.liveView()
	p, ok := c.DeceasedOf(v, it)
	return p, ok
}

func (r *Renderer) drawCorpseItem(screen *ebiten.Image, it game.ItemView, tile float64, sw, sh int) {
	c, v := r.pipe().Creatures, r.liveView()
	c.DrawCorpseItem(v, r.pipe().Pass(), screen, it, tile, sw, sh)
}

func (r *Renderer) drawGrave(screen *ebiten.Image, b game.BuildingView, tile float64, sw, sh int) {
	c, v := r.pipe().Creatures, r.liveView()
	c.DrawGrave(v, r.pipe().Pass(), screen, b, tile, sw, sh)
}

func (r *Renderer) emitPyre(b game.BuildingView) {
	c, v := r.pipe().Creatures, r.liveView()
	c.EmitPyre(v, b)
}

func (r *Renderer) drawPyre(screen *ebiten.Image, b game.BuildingView, tile float64, sw, sh int) {
	c, v := r.pipe().Creatures, r.liveView()
	c.DrawPyre(v, r.pipe().Pass(), screen, b, tile, sw, sh)
}

// emitBursts bursts evs through tracker tr into the renderer's
// particles (tests).
func (r *Renderer) emitBursts(tr *creaturepkg.BurstTracker, evs []creaturepkg.BurstEvent) {
	c, v := r.pipe().Creatures, r.liveView()
	tr.Emit(c, v, evs)
}
