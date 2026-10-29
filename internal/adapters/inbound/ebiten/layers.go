package ebiten

import (
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/adapters/inbound/ebiten/render/creature"
	"rimworld-go/internal/adapters/inbound/ebiten/render/cue"
	"rimworld-go/internal/adapters/inbound/ebiten/render/env"
	"rimworld-go/internal/adapters/inbound/ebiten/render/light"
	"rimworld-go/internal/adapters/inbound/ebiten/render/overlay"
	"rimworld-go/internal/adapters/inbound/ebiten/render/world"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// The renderer's draw layers (ARCH-004) are composed here: the world
// pipeline (render/world) holds every layer, the zoom state, the link grid
// and the stone cache, and draws the frame in its fixed order. The
// renderer builds it on first use (tests build bare Renderers and set art
// afterwards) and implements the layers' host interfaces (pipeSource,
// overlayHost here; creatureHost, envHost, cueHost, shadowHost beside the
// layers they serve).

// pipe is the renderer's world pipeline and, through it, every draw layer
// (Lit, Env, Creatures, Overlays, Cues, LOD, Links, Stone, Paint): built on
// first use, its painter's catalog kept current (the App sets r.art after
// New; tests swap it). Every layer draws with the pipeline's painter.
func (r *Renderer) pipe() *world.Pipeline {
	if r.wp == nil {
		r.lit.Blaze = overlay.BlazeLight // a burning cell's reach and strength live with the fires
		wp := world.New(
			world.Layers{Lit: &r.lit, Env: &r.env.Layer, Cues: r.cueBus(), Shadows: &r.env.Shadow, Motion: &r.mo},
			world.Hosts{Index: envHost{r}, Stamp: envHost{r}, Casters: shadowHost{r}},
			world.Shared{Cam: &r.cam, Cast: &r.cast, Blood: &r.figureBlood},
			&r.view, pipeSource{r})
		p := &wp.Paint
		e := &r.env.Layer
		e.P, e.Lit, e.Keys = p, &r.lit, world.PlantKeys.PlantKeyTable
		e.Water, e.SnowC, e.Trees = &r.water, &r.snowc, &r.treeFx
		c := creature.New(p, &r.mo, &r.lit)
		c.Host = creatureHost{r}
		c.Parts, c.Floaters, c.ItemBorn = &r.parts, &r.floaters, &r.itemBorn
		o := overlay.New(p, &r.lit)
		h := overlayHost{r}
		o.Host = overlay.Host{
			Cue:      wp.Cues.Cue,
			Burst:    h.burst,
			Flame:    creature.DrawFlameScreen,
			Puff:     wp.Lb.DrawPuff,
			PuffAt:   env.PuffAt,
			BuildKey: world.BuildSprite,
			HitAge:   h.hitAge,
			Flash:    h.flash,
		}
		wp.Creatures, wp.Overlays = c, o
		r.wp = wp
	}
	r.wp.SetArt(r.art)
	return r.wp
}

// overlays is the map overlays (render/overlay) of the pipeline: the HUD's
// toggles, the Architect's previews and forced overlays reach them here.
func (r *Renderer) overlays() *overlay.Set { return r.pipe().Overlays }

// cueBus is the renderer's sound cues, with the renderer's lookups wired
// in on first use (the App's sound director reads it before any frame is
// drawn, so it does not wait for the pipeline).
func (r *Renderer) cueBus() *cue.Bus {
	if r.sound == nil {
		r.sound = cue.New()
	}
	if r.sound.Host == nil {
		r.sound.Host = cueHost{r}
	}
	return r.sound
}

// liveView is the frame view (render.View) brought up to date for a layer
// entry point outside the pipeline's frame: the advance's animation and
// listeners, the HUD's and the Architect's previews, the minimap, tests.
// It copies in the art, the clocks, the map size, the selection and the
// options, and the camera unless a bake is running (the bake moved it).
// Renderer.Draw fills the destination; the layers read the one view.
func (r *Renderer) liveView() *render.View {
	v := &r.view
	v.Art, v.Present, v.Fx = r.art, r.present, r.fx
	if r.wp == nil || !r.wp.LOD.Baking {
		v.Cam = r.cam
	}
	v.AnimT = 0
	if r.game != nil {
		t := r.game.Tick()
		v.Tick, v.AnimT = t, r.mo.AnimT(t)
		v.MapW, v.MapH = r.game.Width(), r.game.Height()
	}
	v.Selected, v.HasSelect = r.selected, r.hasSelect
	v.ReducedMotion, v.ReducedFlashing = r.reducedMotion, r.reducedFlashing
	return v
}

// pipeSource is the renderer as the pipeline's world.Source (its
// DrawOrderRoute is in order_queue_ui.go).
type pipeSource struct{ r *Renderer }

func (s pipeSource) World() game.WorldView                { return s.r.game.World() }
func (s pipeSource) Snapshot() []game.PawnView            { return s.r.game.Snapshot() }
func (s pipeSource) Date() clock.Date                     { return s.r.game.Date() }
func (s pipeSource) ChunkLayer(cx, cy int) game.ChunkView { return s.r.game.ChunkLayer(cx, cy) }
func (s pipeSource) PlantsIn(plants []game.PlantView, x0, y0, x1, y1 int, buf []int32) ([]int32, bool) {
	g := s.r.game
	if g == nil {
		return buf, false
	}
	if cur := g.World(); light.First(cur.Plants) == light.First(plants) && len(cur.Plants) == len(plants) {
		return g.PlantsIn(x0, y0, x1, y1, buf), true
	}
	return buf, false
}

// overlayHost is the renderer behind the overlays' callbacks
// (overlay.Host): the creature layer's particles and hit ages, the
// environment's lightning flash and the cue bus's thunder.
type overlayHost struct{ r *Renderer }

// burst emits an overlay's particles (overlay.Host.Burst).
func (h overlayHost) burst(p overlay.Particle, x, y float64, n int, seed uint64) {
	kind := creature.PartEmber
	if p == overlay.Smoke {
		kind = creature.PartSmoke
	}
	h.r.wp.Creatures.Burst(&h.r.view, kind, x, y, n, seed)
}

// hitAge is how long ago creature id was hit (overlay.Host.HitAge).
func (h overlayHost) hitAge(id entity.EntityID, hitTick uint64, x, y int) (float64, bool) {
	return h.r.wp.Creatures.HitAge(&h.r.view, id, hitTick, x, y)
}

// flash shows a sim lightning strike of tick: the flash now and the
// thunder rolling in after its delay (overlay.Host.Flash).
func (h overlayHost) flash(tick uint64) {
	r := h.r
	e := &r.env.Layer
	e.FlashBucket, e.FlashAt, e.FlashSeen = tick/env.LightningBucket, r.present, true
	r.cueBus().ThunderAt = r.present + cue.ThunderDelay(tick)
}
