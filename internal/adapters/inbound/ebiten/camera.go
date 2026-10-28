package ebiten

import "math"

// mapViewport is the colony map's viewport in map pixels (physical x the
// render scale, world_scale.go): what the camera, edge scrolling, culling
// and the sound director use. The logical size is rounded up, so the
// right and bottom edges are capped at the screen.
func (a *App) mapViewport() viewport {
	k := a.scale.WorldK()
	v := colonyViewportPhysical(a.sw, a.sh, a.mapScale())
	if a.pw > 0 {
		v.X1 = math.Min(v.X1, float64(a.pw)*k)
	}
	if a.ph > 0 {
		v.Y1 = math.Min(v.Y1, float64(a.ph)*k)
	}
	return v
}
