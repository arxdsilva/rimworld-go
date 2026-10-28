package ebiten

import (
	"image/color"
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

type (
	animState      = creature.Set
	animalBody     = creature.Body
	bodyShow       = creature.BodyShow
	pawnFigurePose = creature.FigurePose
	vec            = creature.Vec
	heldItem       = creature.HeldItem
	heldRig        = creature.HeldRig
	heldStance     = creature.HeldStance
	heldPlacement  = creature.HeldPlacement
	reactTrace     = creature.ReactTrace
	floater        = creature.Floater
	particle       = creature.Particle
	partKind       = creature.PartKind
	burstEvent     = creature.BurstEvent
	burstTracker   = creature.BurstTracker
	actionPose     = creature.ActionPose
	actionDef      = creature.ActionDef
	partSpec       = creature.PartSpec
	partAt         = creature.PartAt
	workBarRect    = creature.WorkBarRect
	corpsePlan     = creature.CorpsePlan
)

const (
	aimReach          = creature.AimReach
	animalMarkAbove   = creature.AnimalMarkAbove
	atTarget          = creature.AtTarget
	bloodSpeed        = creature.BloodSpeed
	bodyMinHold       = creature.BodyMinHold
	bodyNeutralHold   = creature.BodyNeutralHold
	bodyReachTicks    = creature.BodyReachTicks
	breatheBase       = creature.BreatheBase
	buckleFrames      = creature.BuckleFrames
	carriedAhead      = creature.CarriedAhead
	clubRaised        = creature.ClubRaised
	clubRest          = creature.ClubRest
	clubStrike        = creature.ClubStrike
	corpseCarrySprite = creature.CorpseCarrySprite
	fallFrames        = creature.FallFrames
	fireTicks         = creature.FireTicks
	fistPush          = creature.FistPush
	floaterLife       = creature.FloaterLife
	gaitFallback      = creature.GaitFallback
	grazeDownMin      = creature.GrazeDownMin
	grazeDownSpan     = creature.GrazeDownSpan
	grazeLookMin      = creature.GrazeLookMin
	grazeLookSpan     = creature.GrazeLookSpan
	knifePush         = creature.KnifePush
	maxParticles      = creature.MaxParticles
	maxPhasePerDraw   = creature.MaxPhasePerDraw
	meleeTicks        = creature.MeleeTicks
	motionBench       = creature.MotionBench
	motionNone        = creature.MotionNone
	motionReach       = creature.MotionReach
	motionSwing       = creature.MotionSwing
	muzzleKick        = creature.MuzzleKick
	partBlood         = creature.PartBlood
	partCasing        = creature.PartCasing
	partChip          = creature.PartChip
	partDust          = creature.PartDust
	partEmber         = creature.PartEmber
	partIchor         = creature.PartIchor
	partLeaf          = creature.PartLeaf
	partNone          = creature.PartNone
	partShaving       = creature.PartShaving
	partSmoke         = creature.PartSmoke
	partSoil          = creature.PartSoil
	partSpark         = creature.PartSpark
	partSteam         = creature.PartSteam
	partStep          = creature.PartStep
	partSawdust       = creature.PartSawdust
	pawnFeetX         = creature.PawnFeetX
	pawnFeetY         = creature.PawnFeetY
	pyreFlameScale    = creature.PyreFlameScale
	runLean           = creature.RunLean
	staggerDist       = creature.StaggerDist
	staggerTicks      = creature.StaggerTicks
	stanceAimed       = creature.StanceAimed
	stanceCarried     = creature.StanceCarried
	stanceLoad        = creature.StanceLoad
	strikeAt          = creature.StrikeAt
	tracerTicks       = creature.TracerTicks
	workBarMinTile    = creature.WorkBarMinTile
	workBarW          = creature.WorkBarW
	workFace          = creature.WorkFace
)

var (
	deadCS        = creature.DeadCS
	legacyActions = creature.LegacyActions
	workTable     = creature.WorkTable
)

func actionOf(p game.PawnView) (string, actionDef, bool) { return creature.ActionOf(p) }
func aimKick(age float64, struck bool) float64           { return creature.AimKick(age, struck) }
func animalBarLift(size float64) float64                 { return creature.AnimalBarLift(size) }
func animalHeight(size float64) float64                  { return creature.AnimalHeight(size) }
func animalMarkerLift(size float64) float64              { return creature.AnimalMarkerLift(size) }
func barLift(h float64) float64                          { return creature.BarLift(h) }
func markerLift(h float64) float64                       { return creature.MarkerLift(h) }
func bedUnder(w game.WorldView, x, y int) (game.BuildingView, bool) {
	return creature.BedUnder(w, x, y)
}
func bodyAttach(sp *artSprite, name string) (x, y float64, ok bool) {
	return creature.BodyAttach(sp, name)
}
func bodyCentre(m artMeta) ([2]float64, bool)                 { return creature.BodyCentre(m) }
func bonesSprite(x, y int) string                             { return creature.BonesSprite(x, y) }
func carryPoses(c *artCatalog, f facing) (ground, slung pose) { return creature.CarryPoses(c, f) }
func casingDir(ux, uy float64) (float64, float64)             { return creature.CasingDir(ux, uy) }
func corpseFlies(it game.ItemView, present uint64) [][2]float64 {
	return creature.CorpseFlies(it, present)
}
func corpsePlanFor(c *artCatalog, it game.ItemView) corpsePlan { return creature.CorpsePlanFor(c, it) }
func det(g ebiten.GeoM) float64                                { return creature.Det(g) }
func facingKey(f facing) (key string, mirror bool)             { return creature.FacingKey(f) }
func fade(c color.RGBA, k float64) color.RGBA                  { return creature.Fade(c, k) }
func figureLying(p game.PawnView) bool                         { return creature.FigureLying(p) }
func graveSprite(b game.BuildingView) string                   { return creature.GraveSprite(b) }
func headstoneSprite(b game.BuildingView) string               { return creature.HeadstoneSprite(b) }
func headstoneTile(b game.BuildingView) (float64, float64)     { return creature.HeadstoneTile(b) }
func heldPivot(sp *artSprite, st heldStance) [2]float64        { return creature.HeldPivot(sp, st) }
func heldWeaponKey(weapon string) (key string, ok bool)        { return creature.HeldWeaponKey(weapon) }
func lyingPose(c *artCatalog, side float64) pose               { return creature.LyingPose(c, side) }
func mirrored(ps pose) pose                                    { return creature.Mirrored(ps) }
func neutralFrame(ph float64) bool                             { return creature.NeutralFrame(ph) }
func pawnHead(c *artCatalog) (x, y float64)                    { return creature.PawnHead(c) }
func pyreSprite(b game.BuildingView) string                    { return creature.PyreSprite(b) }
func readyHeld(p game.PawnView) heldItem                       { return creature.ReadyHeld(p) }
func standPose() pose                                          { return creature.StandPose() }
func traceAge(rt reactTrace, dur float64) float64              { return creature.TraceAge(rt, dur) }
func traceEnds(spots map[[2]int]vec, tr game.TraceView) (from, to vec) {
	return creature.TraceEnds(spots, tr)
}
func workStrikeAt(prev, cur float64, id uint64, period int) bool {
	return creature.WorkStrikeAt(prev, cur, id, period)
}
func workSwing(t float64, id uint64, period int) float64 { return creature.WorkSwing(t, id, period) }
func wrapAngle(a float64) float64                        { return creature.WrapAngle(a) }
func youngPose(ps pose, scale float64) pose              { return creature.YoungPose(ps, scale) }
func breathe(h float64, p uint64, id entity.EntityID) float64 {
	return creature.Breathe(h, p, id)
}
func chasedPawn(pawns []game.PawnView, id entity.EntityID) (game.PawnView, bool) {
	return creature.ChasedPawn(pawns, id)
}
func attackReaction(traces []game.TraceView, x, y int) (dx, dy float64, flash float32) {
	return creature.AttackReaction(traces, x, y)
}
func attackReactionF(traces []reactTrace, x, y int) (dx, dy float64, flash float32) {
	return creature.AttackReactionF(traces, x, y)
}
func meleeMotion(weapon string, cycle, age float64, struck bool) (lunge, angle, push float64) {
	return creature.MeleeMotion(weapon, cycle, age, struck)
}
func spritePoseGeo(m artMeta, sx, sy, tile float64, ps pose) ebiten.GeoM {
	return creature.SpritePoseGeo(m, sx, sy, tile, ps)
}
func staticShow(an game.AnimalView, m moveState, t, g float64) bodyShow {
	return creature.StaticShow(an, m, t, g)
}
func targetFacing(p game.PawnView, pawns []game.PawnView, animals []game.AnimalView, prev facing) (facing, bool) {
	return creature.TargetFacing(p, pawns, animals, prev)
}
func drawFlameScreen(screen *ebiten.Image, sx, sy float32, t, tile, k float64) {
	creature.DrawFlameScreen(screen, sx, sy, t, tile, k)
}

// faceAttack turns the visual of an attacker standing at (x, y) towards
// its latest target (creature.FaceAttack).
func faceAttack(v pawnVisual, traces []game.TraceView, x, y int) pawnVisual {
	v.facing = creature.FaceAttack(v.facing, traces, x, y)
	return v
}

// itemSpriteSpecies is the stack's sprite: a carcass's or the bones'
// per-tile variant (creature.CorpseSprite), else its item sprite.
func itemSpriteSpecies(kind, species string, x, y int) string {
	return world.ItemSpriteSpecies(kind, species, x, y)
}

// itemSpriteAt is itemSpriteSpecies without a species.
func itemSpriteAt(kind string, x, y int) string { return itemSpriteSpecies(kind, "", x, y) }

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
func (r *Renderer) creatureView() (v *render.View, frame camera) {
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
		MapTier: l.Tier == lodMap, MotionPad: motionViewPad, MinTile: creatureMinTile}
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
func (h creatureHost) LayerBlood(key string, b float32) float32 { return layerBlood(key, b) }
func (h creatureHost) ColonyCloths() []string                   { return h.r.colonyCloths() }
func (h creatureHost) ItemSprite(kind string) string            { return itemSprite(kind) }

func (h creatureHost) Visual(id entity.EntityID) creature.Visual {
	v := h.r.last[id]
	return creature.Visual{Facing: v.facing, LyingSince: v.lyingSince}
}

func (h creatureHost) StoneItem(key, kind string, cs sprite.ColorScale) (string, sprite.ColorScale) {
	return h.r.stoneItem(key, kind, cs)
}

func (h creatureHost) HostileMarker(dst *ebiten.Image, p game.PawnView, wx, wy, lift, tile float64) {
	h.r.drawHostileMarker(dst, p, wx, wy, lift, tile)
}

func (h creatureHost) HealthColor(pct int) color.RGBA { return healthColor(pct) }

func (h creatureHost) TurretMuzzle(id entity.EntityID) (vec, bool) {
	if b, ok := h.r.buildingByID(id); ok && h.r.art != nil {
		return turretMuzzle(h.r.art, b)
	}
	return vec{}, false
}

func (h creatureHost) BuildingAt(w game.WorldView, x, y int) (game.BuildingView, bool) {
	return buildingAt(w, x, y)
}

func (h creatureHost) Spray(w game.WorldView, target entity.EntityID, damage int) (partKind, int, float64) {
	n, speed := sprayFor(damage)
	return sprayKind(w, target), n, speed
}

func (h creatureHost) WorkCue(act string, x, y float64) {
	if id, ok := actionSounds[act]; ok {
		h.r.cue(id, x, y, 1)
	}
}

func (h creatureHost) BurstCue(x, y float64)       { h.r.cue(voiceBurst, x, y, volBurst) }
func (h creatureHost) TraceSound(t game.TraceView) { h.r.traceSound(t) }

// --- the layer's entry points under their old names ----------------------

func (r *Renderer) animState() *animState { return r.creatures() }

func (r *Renderer) drawSpritePose(screen *ebiten.Image, key string, tint color.NRGBA, wx, wy, tile float64, ps pose) {
	c := r.creatures()
	v, frame := r.creatureView()
	c.DrawSpritePose(v, r.spritePass(), screen, key, tint, wx, wy, tile, ps)
	v.Cam = frame
}

func (r *Renderer) stepDust(_ *animState, id entity.EntityID, m moveState, heavy bool) {
	c := r.creatures()
	v, frame := r.creatureView()
	c.StepDust(v, id, m, heavy)
	v.Cam = frame
}

func (r *Renderer) burst(kind partKind, x, y float64, n int, seed uint64) {
	c := r.creatures()
	v, frame := r.creatureView()
	c.Burst(v, kind, x, y, n, seed)
	v.Cam = frame
}

func (r *Renderer) burstDir(kind partKind, x, y float64, n int, seed uint64, dx, dy, speed float64) {
	c := r.creatures()
	v, frame := r.creatureView()
	c.BurstDir(v, kind, x, y, n, seed, dx, dy, speed)
	v.Cam = frame
}

func (r *Renderer) drawParticles(screen *ebiten.Image, tile float64) {
	c, cl := r.creatures(), r.creatureCull()
	v, frame := r.creatureView()
	c.DrawParticles(v, &cl, screen, tile)
	v.Cam = frame
}

// drawParticlesNoScreen ages particles without drawing (tests).
func (r *Renderer) drawParticlesNoScreen() {
	c := r.creatures()
	v, frame := r.creatureView()
	c.AgeParticles(v)
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

func (r *Renderer) drawWorkGlow(screen *ebiten.Image, tile float64) {
	c := r.creatures()
	v, frame := r.creatureView()
	c.DrawWorkGlow(v, screen, tile)
	v.Cam = frame
}

func (r *Renderer) poseFacing(p game.PawnView, m moveState) facing {
	c := r.creatures()
	v, frame := r.creatureView()
	f := c.PoseFacing(v, p, m)
	v.Cam = frame
	return f
}

func (r *Renderer) standingPose(p game.PawnView, m moveState) (pose, heldItem) {
	c := r.creatures()
	v, frame := r.creatureView()
	ps, h := c.StandingPose(v, p, m)
	v.Cam = frame
	return ps, h
}

func (r *Renderer) pawnFigurePoseOf(p game.PawnView) pawnFigurePose {
	c := r.creatures()
	v, frame := r.creatureView()
	fp := c.FigurePoseOf(v, p)
	v.Cam = frame
	return fp
}

func (r *Renderer) drawPawnFigure(dst *ebiten.Image, p game.PawnView, wx, wy, tile float64, fp pawnFigurePose) {
	c := r.creatures()
	v, frame := r.creatureView()
	c.DrawPawnFigure(v, r.spritePass(), dst, p, wx, wy, tile, fp)
	v.Cam = frame
}

func (r *Renderer) drawFigureIn(screen *ebiten.Image, p game.PawnView, f facing, wx, wy, tile float64, ps pose) {
	c := r.creatures()
	v, frame := r.creatureView()
	c.DrawFigureIn(v, r.spritePass(), screen, p, f, wx, wy, tile, ps)
	v.Cam = frame
}

func (r *Renderer) drawFigure(screen *ebiten.Image, p game.PawnView, wx, wy, tile float64, ps pose) {
	r.drawFigureIn(screen, p, faceSouth, wx, wy, tile, ps)
}

func (r *Renderer) drawLyingPawn(screen *ebiten.Image, p game.PawnView, vis pawnVisual, wx, wy, tile float64) {
	c := r.creatures()
	v, frame := r.creatureView()
	c.DrawLyingPawn(v, r.spritePass(), screen, p, creature.Visual{Facing: vis.facing, LyingSince: vis.lyingSince}, wx, wy, tile)
	v.Cam = frame
}

func (r *Renderer) drawSnoreFor(screen *ebiten.Image, id uint64, hx, hy, tile float64) {
	c := r.creatures()
	v, frame := r.creatureView()
	c.DrawSnoreFor(v, screen, id, hx, hy, tile)
	v.Cam = frame
}

func (r *Renderer) animateWork(w game.WorldView, pawns []game.PawnView) {
	c := r.creatures()
	v, frame := r.creatureView()
	c.AnimateWork(v, w, pawns)
	v.Cam = frame
}

func (r *Renderer) carryPose(p game.PawnView, m moveState) actionPose {
	c := r.creatures()
	v, frame := r.creatureView()
	a := c.CarryPose(v, p, m)
	v.Cam = frame
	return a
}

func (r *Renderer) workPose(p game.PawnView, m moveState) actionPose {
	c := r.creatures()
	v, frame := r.creatureView()
	a := c.WorkPose(v, p, m)
	v.Cam = frame
	return a
}

func (r *Renderer) idleHeld(p game.PawnView, m moveState) heldItem {
	return r.creatures().IdleHeld(p, m)
}

func (r *Renderer) rigOf(p game.PawnView, f facing) heldRig {
	c := r.creatures()
	v, frame := r.creatureView()
	rig := c.RigOf(v, p, f)
	v.Cam = frame
	return rig
}

func (r *Renderer) heldPlace(rig heldRig, h heldItem, wx, wy, tile float64, ps pose) (heldPlacement, bool) {
	c := r.creatures()
	v, frame := r.creatureView()
	pl, ok := c.HeldPlace(v, rig, h, wx, wy, tile, ps)
	v.Cam = frame
	return pl, ok
}

func (r *Renderer) drawHeld(screen *ebiten.Image, rig heldRig, h heldItem, wx, wy, tile float64, ps pose, behind bool) {
	c := r.creatures()
	v, frame := r.creatureView()
	c.DrawHeld(v, r.spritePass(), screen, rig, h, wx, wy, tile, ps, behind)
	v.Cam = frame
}

func (r *Renderer) heldPoint(rig heldRig, h heldItem, wx, wy float64, ps pose, attach string) (vec, bool) {
	c := r.creatures()
	v, frame := r.creatureView()
	pt, ok := c.HeldPoint(v, rig, h, wx, wy, ps, attach)
	v.Cam = frame
	return pt, ok
}

func (r *Renderer) lastStrike(id entity.EntityID, x, y int, dur float64) (reactTrace, float64, bool) {
	c := r.creatures()
	v, frame := r.creatureView()
	st, age, ok := c.LastStrike(v, id, x, y, dur)
	v.Cam = frame
	return st, age, ok
}

func (r *Renderer) hitAge(id entity.EntityID, hitTick uint64, x, y int) (float64, bool) {
	c := r.creatures()
	v, frame := r.creatureView()
	age, ok := c.HitAge(v, id, hitTick, x, y)
	v.Cam = frame
	return age, ok
}

func (r *Renderer) hitReaction(p game.PawnView, m moveState) actionPose {
	c := r.creatures()
	v, frame := r.creatureView()
	a := c.HitReaction(v, p, m)
	v.Cam = frame
	return a
}

func (r *Renderer) combatPose(p game.PawnView, m moveState) actionPose {
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

func (r *Renderer) fallPose(p game.PawnView, vis pawnVisual) pose {
	c := r.creatures()
	v, frame := r.creatureView()
	ps := c.FallPose(v, p, creature.Visual{Facing: vis.facing, LyingSince: vis.lyingSince})
	v.Cam = frame
	return ps
}

func (r *Renderer) pawnHeldPoint(p game.PawnView, attach string) (vec, bool) {
	c := r.creatures()
	v, frame := r.creatureView()
	pt, ok := c.PawnHeldPoint(v, p, attach)
	v.Cam = frame
	return pt, ok
}

func (r *Renderer) shotPoints(t game.TraceView, pawns []game.PawnView) (muzzle, port vec) {
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

func (r *Renderer) interpAnimal(an game.AnimalView) (float64, float64) {
	c := r.creatures()
	v, frame := r.creatureView()
	x, y := c.AnimalAt(v, an)
	v.Cam = frame
	return x, y
}

func (r *Renderer) drawAnimalOverlays(screen *ebiten.Image, w game.WorldView, tile float64, sw, sh int) {
	c, cl := r.creatures(), r.creatureMarks(tile)
	v, frame := r.creatureView()
	c.DrawAnimalOverlays(v, r.spritePass(), &cl, screen, w, tile)
	v.Cam = frame
}

func (r *Renderer) drawChaseLines(screen *ebiten.Image, w game.WorldView, tile float64) {
	c, cl := r.creatures(), r.creatureMarks(tile)
	v, frame := r.creatureView()
	c.DrawChaseLines(v, &cl, screen, w, tile)
	v.Cam = frame
}

func (r *Renderer) drawAnimalName(screen *ebiten.Image, name string, wx, wy float64) {
	c, cl := r.creatures(), r.creatureCull()
	cl.White = r.lod.WhiteImg()
	v, frame := r.creatureView()
	c.DrawAnimalName(v, &cl, screen, name, wx, wy)
	v.Cam = frame
}

func (r *Renderer) traceList(w game.WorldView) []reactTrace {
	c := r.creatures()
	v, frame := r.creatureView()
	rs := c.TraceList(v, w)
	v.Cam = frame
	return rs
}

func (r *Renderer) traceSpots(w game.WorldView) map[[2]int]vec {
	c := r.creatures()
	v, frame := r.creatureView()
	spots := c.TraceSpots(v, w, r.game.Snapshot())
	v.Cam = frame
	return spots
}

func (r *Renderer) drawTraces(screen *ebiten.Image, w game.WorldView, tile float64) {
	c := r.creatures()
	v, frame := r.creatureView()
	c.DrawTraces(v, screen, w, r.game.Snapshot(), tile)
	v.Cam = frame
}

func (r *Renderer) traceMuzzle(tr game.TraceView) (vec, bool) {
	c := r.creatures()
	v, frame := r.creatureView()
	pt, ok := c.TraceMuzzle(v, tr)
	v.Cam = frame
	return pt, ok
}

func (r *Renderer) meleeTrail(tr game.TraceView) []vec {
	c := r.creatures()
	v, frame := r.creatureView()
	pts := c.MeleeTrail(v, tr)
	v.Cam = frame
	return pts
}

func (r *Renderer) animalBaseKey(an game.AnimalView) string {
	c := r.creatures()
	v, frame := r.creatureView()
	k := c.AnimalBaseKey(v, an)
	v.Cam = frame
	return k
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

func (r *Renderer) animalMirrored(an game.AnimalView) bool {
	c := r.creatures()
	v, frame := r.creatureView()
	m := c.AnimalMirrored(v, an)
	v.Cam = frame
	return m
}

func (r *Renderer) animalResting(an game.AnimalView) bool {
	c := r.creatures()
	v, frame := r.creatureView()
	ok := c.AnimalResting(v, an)
	v.Cam = frame
	return ok
}

func (r *Renderer) animalRestFallback(an game.AnimalView) bool {
	c := r.creatures()
	v, frame := r.creatureView()
	ok := c.AnimalRestFallback(v, an)
	v.Cam = frame
	return ok
}

func (r *Renderer) animalFigureScale(an game.AnimalView) (colorScale, bool) {
	c := r.creatures()
	v, frame := r.creatureView()
	cs, ok := c.AnimalFigureScale(v, an)
	v.Cam = frame
	return cs, ok
}

func (r *Renderer) drawAnimalFigure(dst *ebiten.Image, an game.AnimalView, wx, wy, tile float64) {
	c := r.creatures()
	v, frame := r.creatureView()
	c.DrawAnimalFigure(v, r.spritePass(), dst, an, wx, wy, tile)
	v.Cam = frame
}

func (r *Renderer) animalGait(an game.AnimalView) float64 {
	c := r.creatures()
	v, frame := r.creatureView()
	g := c.AnimalGait(v, an)
	v.Cam = frame
	return g
}

func (r *Renderer) animalShow(an game.AnimalView) bodyShow {
	c := r.creatures()
	v, frame := r.creatureView()
	s := c.AnimalShow(v, an)
	v.Cam = frame
	return s
}

func (r *Renderer) animalRestTop(an game.AnimalView) float64 {
	c := r.creatures()
	v, frame := r.creatureView()
	h := c.AnimalRestTop(v, an)
	v.Cam = frame
	return h
}

func (r *Renderer) animalTop(an game.AnimalView, tile float64) float64 {
	c := r.creatures()
	v, frame := r.creatureView()
	h := c.AnimalTop(v, an, tile, creatureMinTile)
	v.Cam = frame
	return h
}

func (r *Renderer) animalPose(an game.AnimalView) pose {
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

func (r *Renderer) workBarRects(pawns []game.PawnView, tile float64, dst []workBarRect) []workBarRect {
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

func (r *Renderer) drawFlame(screen *ebiten.Image, cx, cy, t, tile, k float64) {
	c := r.creatures()
	v, frame := r.creatureView()
	c.DrawFlame(v, screen, cx, cy, t, tile, k)
	v.Cam = frame
}

func (r *Renderer) drawPyre(screen *ebiten.Image, b game.BuildingView, tile float64, sw, sh int) {
	c := r.creatures()
	v, frame := r.creatureView()
	c.DrawPyre(v, r.spritePass(), screen, b, tile, sw, sh)
	v.Cam = frame
}

func (r *Renderer) emitWorldBursts(w game.WorldView) {
	c := r.creatures()
	v, frame := r.creatureView()
	c.EmitWorldBursts(v, w)
	v.Cam = frame
}

// emitBursts bursts evs through tracker tr into the renderer's
// particles (tests).
func (r *Renderer) emitBursts(tr *burstTracker, evs []burstEvent) {
	c := r.creatures()
	v, frame := r.creatureView()
	tr.Emit(c, v, evs)
	v.Cam = frame
}
