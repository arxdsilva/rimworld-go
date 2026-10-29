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
	c := r.creatures()
	v, frame := r.creatureView()
	c.StepDust(v, id, m, heavy)
	v.Cam = frame
}

// drawParticlesNoScreen ages particles without drawing (tests).
func (r *Renderer) drawParticlesNoScreen() {
	c := r.creatures()
	v, frame := r.creatureView()
	c.AgeParticles(v)
	v.Cam = frame
}

func (r *Renderer) standingPose(p game.PawnView, m motionpkg.MoveState) (motionpkg.Pose, creaturepkg.HeldItem) {
	c := r.creatures()
	v, frame := r.creatureView()
	ps, h := c.StandingPose(v, p, m)
	v.Cam = frame
	return ps, h
}

func (r *Renderer) drawFigure(screen *ebiten.Image, p game.PawnView, wx, wy, tile float64, ps motionpkg.Pose) {
	r.drawFigureIn(screen, p, artpkg.FaceSouth, wx, wy, tile, ps)
}

func (r *Renderer) drawLyingPawn(screen *ebiten.Image, p game.PawnView, vis pawnVisual, wx, wy, tile float64) {
	c := r.creatures()
	v, frame := r.creatureView()
	c.DrawLyingPawn(v, r.spritePass(), screen, p, creature.Visual{Facing: vis.facing, LyingSince: vis.lyingSince}, wx, wy, tile)
	v.Cam = frame
}

func (r *Renderer) animateWork(w game.WorldView, pawns []game.PawnView) {
	c := r.creatures()
	v, frame := r.creatureView()
	c.AnimateWork(v, w, pawns)
	v.Cam = frame
}

func (r *Renderer) carryPose(p game.PawnView, m motionpkg.MoveState) creaturepkg.ActionPose {
	c := r.creatures()
	v, frame := r.creatureView()
	a := c.CarryPose(v, p, m)
	v.Cam = frame
	return a
}

func (r *Renderer) workPose(p game.PawnView, m motionpkg.MoveState) creaturepkg.ActionPose {
	c := r.creatures()
	v, frame := r.creatureView()
	a := c.WorkPose(v, p, m)
	v.Cam = frame
	return a
}

func (r *Renderer) rigOf(p game.PawnView, f artpkg.Facing) creaturepkg.HeldRig {
	c := r.creatures()
	v, frame := r.creatureView()
	rig := c.RigOf(v, p, f)
	v.Cam = frame
	return rig
}

func (r *Renderer) heldPlace(rig creaturepkg.HeldRig, h creaturepkg.HeldItem, wx, wy, tile float64, ps motionpkg.Pose) (creaturepkg.HeldPlacement, bool) {
	c := r.creatures()
	v, frame := r.creatureView()
	pl, ok := c.HeldPlace(v, rig, h, wx, wy, tile, ps)
	v.Cam = frame
	return pl, ok
}

func (r *Renderer) drawHeld(screen *ebiten.Image, rig creaturepkg.HeldRig, h creaturepkg.HeldItem, wx, wy, tile float64, ps motionpkg.Pose, behind bool) {
	c := r.creatures()
	v, frame := r.creatureView()
	c.DrawHeld(v, r.spritePass(), screen, rig, h, wx, wy, tile, ps, behind)
	v.Cam = frame
}

func (r *Renderer) heldPoint(rig creaturepkg.HeldRig, h creaturepkg.HeldItem, wx, wy float64, ps motionpkg.Pose, attach string) (creaturepkg.Vec, bool) {
	c := r.creatures()
	v, frame := r.creatureView()
	pt, ok := c.HeldPoint(v, rig, h, wx, wy, ps, attach)
	v.Cam = frame
	return pt, ok
}

func (r *Renderer) hitReaction(p game.PawnView, m motionpkg.MoveState) creaturepkg.ActionPose {
	c := r.creatures()
	v, frame := r.creatureView()
	a := c.HitReaction(v, p, m)
	v.Cam = frame
	return a
}

func (r *Renderer) combatPose(p game.PawnView, m motionpkg.MoveState) creaturepkg.ActionPose {
	c := r.creatures()
	v, frame := r.creatureView()
	a := c.CombatPose(v, p, m)
	v.Cam = frame
	return a
}

func (r *Renderer) combatOffset(p game.PawnView) (dx, dy float64, flash float32) {
	c := r.creatures()
	v, frame := r.creatureView()
	dx, dy, flash = c.CombatOffset(v, p)
	v.Cam = frame
	return dx, dy, flash
}

func (r *Renderer) fallPose(p game.PawnView, vis pawnVisual) motionpkg.Pose {
	c := r.creatures()
	v, frame := r.creatureView()
	ps := c.FallPose(v, p, creature.Visual{Facing: vis.facing, LyingSince: vis.lyingSince})
	v.Cam = frame
	return ps
}

func (r *Renderer) pawnHeldPoint(p game.PawnView, attach string) (creaturepkg.Vec, bool) {
	c := r.creatures()
	v, frame := r.creatureView()
	pt, ok := c.PawnHeldPoint(v, p, attach)
	v.Cam = frame
	return pt, ok
}

func (r *Renderer) shotPoints(t game.TraceView, pawns []game.PawnView) (muzzle, port creaturepkg.Vec) {
	c := r.creatures()
	v, frame := r.creatureView()
	muzzle, port = c.ShotPoints(v, t, pawns)
	v.Cam = frame
	return muzzle, port
}

func (r *Renderer) animateCombat(w game.WorldView, pawns []game.PawnView) {
	c := r.creatures()
	v, frame := r.creatureView()
	c.AnimateCombat(v, w, pawns)
	v.Cam = frame
}

func (r *Renderer) drawAnimalOverlays(screen *ebiten.Image, w game.WorldView, tile float64, sw, sh int) {
	c, cl := r.creatures(), r.creatureMarks(tile)
	v, frame := r.creatureView()
	c.DrawAnimalOverlays(v, r.spritePass(), &cl, screen, w, tile)
	v.Cam = frame
}

func (r *Renderer) traceList(w game.WorldView) []creaturepkg.ReactTrace {
	c := r.creatures()
	v, frame := r.creatureView()
	rs := c.TraceList(v, w)
	v.Cam = frame
	return rs
}

func (r *Renderer) traceSpots(w game.WorldView) map[[2]int]creaturepkg.Vec {
	c := r.creatures()
	v, frame := r.creatureView()
	spots := c.TraceSpots(v, w, r.game.Snapshot())
	v.Cam = frame
	return spots
}

func (r *Renderer) traceMuzzle(tr game.TraceView) (creaturepkg.Vec, bool) {
	c := r.creatures()
	v, frame := r.creatureView()
	pt, ok := c.TraceMuzzle(v, tr)
	v.Cam = frame
	return pt, ok
}

func (r *Renderer) meleeTrail(tr game.TraceView) []creaturepkg.Vec {
	c := r.creatures()
	v, frame := r.creatureView()
	pts := c.MeleeTrail(v, tr)
	v.Cam = frame
	return pts
}

func (r *Renderer) animalFrameKey(an game.AnimalView) string {
	c := r.creatures()
	v, frame := r.creatureView()
	k := c.AnimalFrameKey(v, an)
	v.Cam = frame
	return k
}

func (r *Renderer) animalFrame(an game.AnimalView) string {
	c := r.creatures()
	v, frame := r.creatureView()
	k := c.AnimalFrame(v, an)
	v.Cam = frame
	return k
}

func (r *Renderer) animalRestFallback(an game.AnimalView) bool {
	c := r.creatures()
	v, frame := r.creatureView()
	ok := c.AnimalRestFallback(v, an)
	v.Cam = frame
	return ok
}

func (r *Renderer) animalFigureScale(an game.AnimalView) (spritepkg.ColorScale, bool) {
	c := r.creatures()
	v, frame := r.creatureView()
	cs, ok := c.AnimalFigureScale(v, an)
	v.Cam = frame
	return cs, ok
}

func (r *Renderer) animalGait(an game.AnimalView) float64 {
	c := r.creatures()
	v, frame := r.creatureView()
	g := c.AnimalGait(v, an)
	v.Cam = frame
	return g
}

func (r *Renderer) animalShow(an game.AnimalView) creaturepkg.BodyShow {
	c := r.creatures()
	v, frame := r.creatureView()
	s := c.AnimalShow(v, an)
	v.Cam = frame
	return s
}

func (r *Renderer) animalTop(an game.AnimalView, tile float64) float64 {
	c := r.creatures()
	v, frame := r.creatureView()
	h := c.AnimalTop(v, an, tile, world.CreatureMinTile)
	v.Cam = frame
	return h
}

func (r *Renderer) animalPose(an game.AnimalView) motionpkg.Pose {
	c := r.creatures()
	v, frame := r.creatureView()
	ps := c.AnimalPose(v, an)
	v.Cam = frame
	return ps
}

func (r *Renderer) floatTrace(t game.TraceView, seed uint64) {
	c := r.creatures()
	v, frame := r.creatureView()
	c.FloatTrace(v, t, seed)
	v.Cam = frame
}

func (r *Renderer) drawFloaters(screen *ebiten.Image, tile float64) {
	c := r.creatures()
	v, frame := r.creatureView()
	c.DrawFloaters(v, screen, tile)
	v.Cam = frame
}

func (r *Renderer) workBarRects(pawns []game.PawnView, tile float64, dst []creaturepkg.WorkBarRect) []creaturepkg.WorkBarRect {
	c, cl := r.creatures(), r.creatureCull()
	v, frame := r.creatureView()
	dst = c.WorkBarRects(v, &cl, pawns, tile, dst)
	v.Cam = frame
	return dst
}

func (r *Renderer) drawWorkProgress(screen *ebiten.Image, pawns []game.PawnView, tile float64) {
	c, cl := r.creatures(), r.creatureCull()
	v, frame := r.creatureView()
	c.DrawWorkProgress(v, &cl, screen, pawns, tile)
	v.Cam = frame
}

func (r *Renderer) deceasedOf(it game.ItemView) (game.PawnView, bool) {
	c := r.creatures()
	v, frame := r.creatureView()
	p, ok := c.DeceasedOf(v, it)
	v.Cam = frame
	return p, ok
}

func (r *Renderer) drawCorpseItem(screen *ebiten.Image, it game.ItemView, tile float64, sw, sh int) {
	c := r.creatures()
	v, frame := r.creatureView()
	c.DrawCorpseItem(v, r.spritePass(), screen, it, tile, sw, sh)
	v.Cam = frame
}

func (r *Renderer) drawGrave(screen *ebiten.Image, b game.BuildingView, tile float64, sw, sh int) {
	c := r.creatures()
	v, frame := r.creatureView()
	c.DrawGrave(v, r.spritePass(), screen, b, tile, sw, sh)
	v.Cam = frame
}

func (r *Renderer) emitPyre(b game.BuildingView) {
	c := r.creatures()
	v, frame := r.creatureView()
	c.EmitPyre(v, b)
	v.Cam = frame
}

func (r *Renderer) drawPyre(screen *ebiten.Image, b game.BuildingView, tile float64, sw, sh int) {
	c := r.creatures()
	v, frame := r.creatureView()
	c.DrawPyre(v, r.spritePass(), screen, b, tile, sw, sh)
	v.Cam = frame
}

// emitBursts bursts evs through tracker tr into the renderer's
// particles (tests).
func (r *Renderer) emitBursts(tr *creaturepkg.BurstTracker, evs []creaturepkg.BurstEvent) {
	c := r.creatures()
	v, frame := r.creatureView()
	tr.Emit(c, v, evs)
	v.Cam = frame
}
