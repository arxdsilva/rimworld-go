// Package render holds the frame context the renderer's draw layers read:
// View, filled once per frame by the root Renderer from one read of the
// simulation, and Trace, the test hooks (nil in production). Layers get
// a *View and never reach back into the Renderer (SOLID_GO.md §7).
package render

import (
	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/camera"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// View is one frame's context: the camera and the art catalog, the
// destination, this frame's world and pawns, the clocks and the options
// the layers read.
type View struct {
	Cam camera.Camera // this frame's camera, a value copy: the transforms below inline from it
	Art *art.Catalog  // the sprite catalog (nil without art)

	SW, SH     int     // the destination in map pixels
	Tile       float64 // tile size in map pixels (zoom x TileSize)
	Zoom, Dev  float64 // camera zoom and the device factor (render scale folded in)
	MapW, MapH int

	W     game.WorldView // this frame's world
	Pawns []game.PawnView
	Tick  clock.Tick
	Date  clock.Date

	Present uint64  // presentation frame counter: animation reads it, the sim never does
	AnimT   float64 // animation clock in seconds (motion.go)
	Alpha   float64 // interpolation between the last two ticks
	Paused  bool

	Fx              float64 // particle and weather detail multiplier (Options)
	ReducedMotion   bool
	ReducedFlashing bool
	Cached          bool // the sim caches its read models, so identity memos hold

	Selected  entity.EntityID
	HasSelect bool

	Trace *Trace // nil in production
}

// Trace is the renderer's test hooks: which ground layers drew in what
// order, and the marks drawn over creatures (chase lines, name plates).
type Trace struct {
	Layer      func(name string)
	AnimalMark func(kind string, a, b, c, d float32)
}

// LayerDrawn notes a ground layer for the layer-order tests.
func (v *View) LayerDrawn(name string) {
	if v.Trace != nil && v.Trace.Layer != nil {
		v.Trace.Layer(name)
	}
}

// MarkDrawn notes a creature mark for the overlay tests.
func (v *View) MarkDrawn(kind string, a, b, c, d float32) {
	if v.Trace != nil && v.Trace.AnimalMark != nil {
		v.Trace.AnimalMark(kind, a, b, c, d)
	}
}

// Tracing reports a trace is installed (callers skip measuring for it otherwise).
func (v *View) Tracing() bool { return v.Trace != nil && v.Trace.AnimalMark != nil }

// WorldToScreenF is a world point in map pixels.
func (v *View) WorldToScreenF(wx, wy float64) (float32, float32) {
	t := v.Cam.Tile()
	return float32(wx*t - v.Cam.OffsetX), float32(wy*t - v.Cam.OffsetY)
}

// WorldToScreen is a tile's top-left in map pixels.
func (v *View) WorldToScreen(tx, ty int) (float32, float32) {
	return v.WorldToScreenF(float64(tx), float64(ty))
}
