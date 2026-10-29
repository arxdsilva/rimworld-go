package ebiten

import (
	"image/color"
	"rimworld-go/internal/adapters/inbound/ebiten/architect"
	"rimworld-go/internal/adapters/inbound/ebiten/art"
	camerapkg "rimworld-go/internal/adapters/inbound/ebiten/camera"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	creaturepkg "rimworld-go/internal/adapters/inbound/ebiten/render/creature"
	"rimworld-go/internal/adapters/inbound/ebiten/render/cue"
	"rimworld-go/internal/adapters/inbound/ebiten/render/env"
	"rimworld-go/internal/adapters/inbound/ebiten/render/overlay"
	overlaypkg "rimworld-go/internal/adapters/inbound/ebiten/render/overlay"
	worldpkg "rimworld-go/internal/adapters/inbound/ebiten/render/world"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// The map overlays live in render/overlay (ARCH-004 wave 4.3): stains and
// dirt, fog, roof, home, plan, paths, zones, bridges, turrets, arrivals,
// fire, the power grid and the snow-clearing and allowed areas. These
// aliases and one-line wrappers keep the root's call sites unchanged; the
// last mover deletes this file (SOLID_GO.md §11). The renderer's ov field
// holds the layer, built on first use (overlays).

func turretMuzzle(c *art.Catalog, b *game.BuildingView) (creaturepkg.Vec, bool) {
	p, ok := overlay.TurretMuzzle(c, b)
	return creaturepkg.Vec{X: p.X, Y: p.Y}, ok
}

// overlays is the renderer's map overlays, built on first use with the
// renderer's other layers wired in (overlay.Host), and its painter's
// catalog kept current (the App sets r.art after New).
func (r *Renderer) overlays() *overlay.Set {
	if r.ov == nil {
		p := r.painter()
		r.ov = overlay.New(&p, r.litLayer())
		r.ov.Host = overlay.Host{
			Cue:      r.cue,
			Burst:    r.overlayBurst,
			Flame:    creaturepkg.DrawFlameScreen,
			Puff:     r.drawPuff,
			PuffAt:   env.PuffAt,
			BuildKey: architect.BuildSprite,
			HitAge:   r.hitAge,
			Flash:    r.flashStrike,
		}
	}
	r.ov.P.Art = r.art
	return r.ov
}

// overlayBurst emits an overlay's particles (overlay.Host.Burst).
func (r *Renderer) overlayBurst(p overlay.Particle, x, y float64, n int, seed uint64) {
	kind := creaturepkg.PartEmber
	if p == overlay.Smoke {
		kind = creaturepkg.PartSmoke
	}
	r.burst(kind, x, y, n, seed)
}

// flashStrike shows a sim lightning strike of tick: the flash now and the
// thunder rolling in after its delay (overlay.Host.Flash).
func (r *Renderer) flashStrike(tick uint64) {
	e := r.envLayer()
	e.FlashBucket, e.FlashAt, e.FlashSeen = tick/env.LightningBucket, r.present, true
	r.cueBus().ThunderAt = r.present + cue.ThunderDelay(tick)
}

// overlayView is the frame view the overlays read: the sprite view (the
// renderer's live camera, which a bake moves) with the map size, tick,
// selection and options copied in, since tests drive these entry points
// without Renderer.Draw filling the view. The caller puts frame back.
func (r *Renderer) overlayView() (v *render.View, frame camerapkg.Camera) {
	v, frame = r.spriteView()
	if r.game != nil {
		v.MapW, v.MapH, v.Tick = r.game.Width(), r.game.Height(), r.game.Tick()
	}
	v.Selected, v.HasSelect = r.selected, r.hasSelect
	v.ReducedMotion, v.ReducedFlashing = r.reducedMotion, r.reducedFlashing
	return v, frame
}

// partView is overlayView for the per-building and per-run entry points
// the world pass calls many times a frame: the sprite view and the
// options, without the simulation's map size and tick.
func (r *Renderer) partView() (v *render.View, frame camerapkg.Camera) {
	v, frame = r.spriteView()
	v.ReducedMotion, v.ReducedFlashing = r.reducedMotion, r.reducedFlashing
	return v, frame
}

// overlayChunks is the frame's chunk lists and view as the overlays walk
// them (lod.go).
func (r *Renderer) overlayChunks() overlay.Chunks {
	l := &r.lod
	return overlay.Chunks{Views: l.Views, VX0: l.VX0, VY0: l.VY0, VX1: l.VX1, VY1: l.VY1, ViewSet: l.ViewSet,
		Live: l.Tier == worldpkg.Live, WholeMap: l.Tier == worldpkg.Map, Baking: l.Baking}
}

// areasOf is the simulation's allowed areas (asked for only while one is
// armed or hovered).
func (r *Renderer) areasOf() []game.AreaView { return r.game.Areas() }

// ---- the toggles

func (r *Renderer) planOverlayOn() bool { return r.overlays().Plan.Shown() }

// togglePowerOverlayIfAny tolerates a nil renderer (menus).
func (r *Renderer) togglePowerOverlayIfAny() {
	if r != nil {
		r.overlays().Power.Toggle()
	}
}

// ---- the layers

func (r *Renderer) drawZones(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	v, frame := r.overlayView()
	ch := r.overlayChunks()
	r.overlays().Zones.DrawZones(v, r.spritePass(), &ch, screen, w, sw, sh, tile)
	v.Cam = frame
}

func (r *Renderer) drawZoneLabels(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	v, frame := r.overlayView()
	r.overlays().Zones.DrawZoneLabels(v, screen, w, sw, sh, tile)
	v.Cam = frame
}

func (r *Renderer) drawZoneLabel(screen *ebiten.Image, z game.ZoneView, g *overlaypkg.ZoneGeom, sw, sh int, tile float64) {
	v, frame := r.partView()
	r.overlays().Zones.DrawZoneLabel(v, screen, z, g, sw, sh, tile)
	v.Cam = frame
}

func (r *Renderer) drawZoneHighlight(screen *ebiten.Image, w game.WorldView, z game.ZoneView, sw, sh int, tile float64) {
	v, frame := r.overlayView()
	ch := r.overlayChunks()
	r.overlays().Zones.DrawZoneHighlight(v, &ch, screen, w, z, sw, sh, tile)
	v.Cam = frame
}

func (r *Renderer) zoneLabelAt(w game.WorldView, mx, my int, tile float64, shown uint16) (game.ZoneView, bool) {
	v, frame := r.overlayView()
	defer func() { v.Cam = frame }()
	return r.overlays().Zones.ZoneLabelAt(v, w, mx, my, tile, shown)
}

func (r *Renderer) drawRoofOverlay(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	r.setView(sw, sh, tile)
	v, frame := r.overlayView()
	r.overlays().DrawRoofOverlay(v, screen, w, r.areasOf, sw, sh, tile)
	v.Cam = frame
}

func (r *Renderer) drawRoofUnsupported(screen *ebiten.Image, x, y int, tile float64) {
	v, frame := r.overlayView()
	r.overlays().DrawRoofUnsupported(v, screen, x, y, tile)
	v.Cam = frame
}

// pawnBlood is the Custom3 blood value for p's figure this frame.
func (r *Renderer) pawnBlood(p game.PawnView) float32 {
	v := r.lightView()
	if p.HitTick != 0 {
		v.AnimT = r.animT()
	}
	return r.overlays().PawnBlood(v, r.spritePass(), p)
}

// animalBlood is the Custom3 blood value for an animal's figure this frame.
func (r *Renderer) animalBlood(an game.AnimalView) float32 {
	v := r.lightView()
	if an.HitTick != 0 {
		v.AnimT = r.animT()
	}
	return r.overlays().AnimalBlood(v, r.spritePass(), an)
}

func (r *Renderer) buildingByID(id entity.EntityID) (*game.BuildingView, bool) {
	return r.overlays().BuildingByID(&r.view, id)
}

func (r *Renderer) drawRangeRing(dst *ebiten.Image, x, y int, radius float64, c color.NRGBA) {
	v, frame := r.partView()
	r.overlays().Wiring.DrawRangeRing(v, dst, x, y, radius, c)
	v.Cam = frame
}

func (r *Renderer) emitFireParticles(w game.WorldView) {
	v, frame := r.overlayView()
	r.overlays().Fires.EmitFireParticles(v, w)
	v.Cam = frame
}

func (r *Renderer) emitBlazeParticles(w game.WorldView) {
	v, frame := r.overlayView()
	r.overlays().Fires.EmitBlazeParticles(v, w)
	v.Cam = frame
}

func (r *Renderer) showStrikes(w game.WorldView) {
	v, frame := r.overlayView()
	r.overlays().Fires.ShowStrikes(v, w)
	v.Cam = frame
}

// blazeAmbience adds one positioned amb.fire loop per fire cluster, the
// loudest few (overlay.Set.BlazeAmbience).
func (r *Renderer) blazeAmbience(want map[string]cue.Loop, w game.WorldView, vp camerapkg.Viewport) {
	r.overlays().Fires.BlazeAmbience(w,
		func(x, y float64) (float64, float64, bool) { return spatial(r, x, y, vp) },
		func(key string, vol, pan float64) { want[key] = cue.Loop{ID: "amb.fire", Vol: vol, Pan: pan} })
}

func (r *Renderer) drawPowerOverlay(screen *ebiten.Image, w *game.WorldView, sw, sh int, tile float64) {
	v, frame := r.overlayView()
	ch := r.overlayChunks()
	r.overlays().DrawPowerOverlay(v, &ch, &r.links, screen, w, sw, sh, tile)
	v.Cam = frame
}

func (r *Renderer) drawPowerLink(dst *ebiten.Image, from, to [2]int, c color.NRGBA, tile float64) {
	v, frame := r.partView()
	r.overlays().Wiring.DrawPowerLink(v, dst, from, to, c, tile)
	v.Cam = frame
}

// ---- kept in the root: a hit's blood spray is a particle (render/creature)

// sprayFor is a hit's spray: 4 + damage/2 drops (at most 16), 1.25×
// faster for hits of 10 damage or more.
func sprayFor(dmg int) (n int, speed float64) {
	n = min(4+max(dmg, 0)/2, 16)
	speed = creaturepkg.BloodSpeed
	if dmg >= 10 {
		speed *= 1.25
	}
	return n, speed
}

// sprayKind is what a hit target bleeds: ichor for hive insects.
func sprayKind(w game.WorldView, target entity.EntityID) creaturepkg.PartKind {
	if target != 0 {
		for i := range w.Animals {
			if an := &w.Animals[i]; an.ID == target {
				if overlaypkg.IsHiveSpecies(an.Species) {
					return creaturepkg.PartIchor
				}
				break
			}
		}
	}
	return creaturepkg.PartBlood
}
