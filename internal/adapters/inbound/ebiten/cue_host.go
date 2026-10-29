package ebiten

import (
	camerapkg "rimworld-go/internal/adapters/inbound/ebiten/camera"
	"rimworld-go/internal/adapters/inbound/ebiten/render/creature"
	"rimworld-go/internal/adapters/inbound/ebiten/render/cue"
	"rimworld-go/internal/adapters/inbound/ebiten/render/env"
	"rimworld-go/internal/adapters/inbound/ebiten/render/world"
	"rimworld-go/internal/application/game"
)

// The renderer's sound cues live in render/cue (ARCH-004 wave 4.3); the
// renderer feeds them from its advance (advanceTicks) and answers their
// lookups (cueHost). The App's sound director (sound_director.go) decides
// what is heard from the cues the bus hands it, and asks for the frame's
// ambient loops here.

// spatial places a world point (tiles) for the ear (cue.Spatial).
func spatial(r *Renderer, x, y float64, v camerapkg.Viewport) (vol, pan float64, ok bool) {
	return cue.Spatial(r.liveView(), x, y, v)
}

// cueHost is what the cue layer reads of the renderer (cue.Host).
type cueHost struct{ r *Renderer }

func (h cueHost) Terrain(w game.WorldView, x, y int) (uint8, bool) { return env.TerrainCode(w, x, y) }

func (h cueHost) IsWater(code uint8) bool { return world.IsWaterCode[code] }

func (h cueHost) PlantsOn(plants []game.PlantView, x0, y0, x1, y1 int, buf []int32) []int32 {
	return h.r.pipe().PlantsOn(plants, x0, y0, x1, y1, buf)
}

func (h cueHost) Chopping(p game.PawnView) bool {
	act, _, ok := creature.ActionOf(p)
	return ok && act == game.ActionChop
}

func (h cueHost) AnimalAt(an game.AnimalView) (float64, float64) { return h.r.interpAnimal(an) }

func (h cueHost) ListenCaskets(w game.WorldView) { h.r.overlays().ListenCaskets(w) }

// machineAmbience adds the machines' positioned loops (cue.Bus.Machines).
func (r *Renderer) machineAmbience(want map[string]cue.Loop, w game.WorldView, v camerapkg.Viewport) {
	r.cueBus().Machines(r.liveView(), want, w, v)
}

// waterOnScreen is how much water and shore the view shows, for the
// ambient river (cue.Bus.WaterOnScreen).
func (r *Renderer) waterOnScreen(w game.WorldView, v camerapkg.Viewport) (water, shore float64) {
	return r.cueBus().WaterOnScreen(r.liveView(), w, v)
}

// blazeAmbience adds one positioned amb.fire loop per fire cluster, the
// loudest few (overlay.Fires.BlazeAmbience).
func (r *Renderer) blazeAmbience(want map[string]cue.Loop, w game.WorldView, vp camerapkg.Viewport) {
	r.overlays().Fires.BlazeAmbience(w,
		func(x, y float64) (float64, float64, bool) { return spatial(r, x, y, vp) },
		func(key string, vol, pan float64) { want[key] = cue.Loop{ID: "amb.fire", Vol: vol, Pan: pan} })
}
