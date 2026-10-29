package ebiten

import (
	camerapkg "rimworld-go/internal/adapters/inbound/ebiten/camera"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/adapters/inbound/ebiten/render/creature"
	"rimworld-go/internal/adapters/inbound/ebiten/render/cue"
	cuepkg "rimworld-go/internal/adapters/inbound/ebiten/render/cue"
	"rimworld-go/internal/adapters/inbound/ebiten/render/env"
	worldpkg "rimworld-go/internal/adapters/inbound/ebiten/render/world"
	"rimworld-go/internal/application/game"
)

// The renderer's sound cues live in render/cue (ARCH-004 wave 4.3). These
// aliases and one-line wrappers keep the root's call sites unchanged; the
// last mover deletes this file (SOLID_GO.md §11). The App's sound director
// (sound_director.go) stays here: it decides what is heard from the cues
// the bus hands it.

// spatial places a world point (tiles) for the ear (cue.Spatial).
func spatial(r *Renderer, x, y float64, v camerapkg.Viewport) (vol, pan float64, ok bool) {
	return cue.Spatial(r.cueView(), x, y, v)
}

// cueBus is the renderer's sound cues, with the renderer's lookups wired
// in on first use.
func (r *Renderer) cueBus() *cue.Bus {
	if r.sound == nil {
		r.sound = cue.New()
	}
	if r.sound.Host == nil {
		r.sound.Host = cueHost{r}
	}
	return r.sound
}

// cueView is the frame view the cue layer reads, with the renderer's
// camera, presentation frame, tick and map size copied in: the listeners
// run in advance, before Draw fills the view.
func (r *Renderer) cueView() *render.View {
	v := &r.view
	v.Present = r.present
	if !r.lod.Baking {
		v.Cam = r.cam
	}
	if r.game != nil {
		v.Tick, v.MapW, v.MapH = r.game.Tick(), r.game.Width(), r.game.Height()
	}
	return v
}

// cueHost is what the cue layer reads of the renderer (cue.Host).
type cueHost struct{ r *Renderer }

func (h cueHost) Terrain(w game.WorldView, x, y int) (uint8, bool) { return env.TerrainCode(w, x, y) }

func (h cueHost) IsWater(code uint8) bool { return worldpkg.IsWaterCode[code] }

func (h cueHost) PlantsOn(plants []game.PlantView, x0, y0, x1, y1 int, buf []int32) []int32 {
	return h.r.pipe().PlantsOn(plants, x0, y0, x1, y1, buf)
}

func (h cueHost) Chopping(p game.PawnView) bool {
	act, _, ok := creature.ActionOf(p)
	return ok && act == game.ActionChop
}

func (h cueHost) AnimalAt(an game.AnimalView) (float64, float64) { return h.r.interpAnimal(an) }

func (h cueHost) ListenCaskets(w game.WorldView) { h.r.overlays().ListenCaskets(w) }

func (r *Renderer) cue(id string, x, y, vol float64) { r.cueBus().Cue(id, x, y, vol) }

func (r *Renderer) listenWorld(w game.WorldView, pawns []game.PawnView) {
	r.cueBus().ListenWorld(r.cueView(), w, pawns)
}

func (r *Renderer) listenAnimals(w game.WorldView) {
	r.cueBus().ListenAnimals(r.cueView(), w, r.mo.Stepped)
}

func (r *Renderer) listenThunder() { r.cueBus().ListenThunder(r.cueView()) }

func (r *Renderer) machineAmbience(want map[string]cuepkg.Loop, w game.WorldView, v camerapkg.Viewport) {
	r.cueBus().Machines(r.cueView(), want, w, v)
}

func (r *Renderer) waterOnScreen(w game.WorldView, v camerapkg.Viewport) (water, shore float64) {
	return r.cueBus().WaterOnScreen(r.cueView(), w, v)
}
