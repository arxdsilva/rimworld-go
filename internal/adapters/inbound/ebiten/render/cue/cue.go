// Package cue holds the sound cues the world raises: one-shot cues from
// world diffs, animal calls and hurts, arrivals, thunder, the machine
// ambience and what is on screen, handed to the App's sound director each
// frame. It reads the frame's *render.View, never the renderer or the
// audio device; the director (in the root) decides what is heard.
package cue

import (
	"math"

	"rimworld-go/internal/adapters/inbound/ebiten/camera"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/application/game"
)

// Host is what the cue layer needs from the renderer that it cannot read
// from the frame's View (SOLID_GO.md §10): its model-side lookups.
type Host interface {
	// Terrain is the read model's terrain code at (x, y).
	Terrain(w game.WorldView, x, y int) (code uint8, ok bool)
	// IsWater reports a terrain code is shallow or deep water.
	IsWater(code uint8) bool
	// PlantsOn appends the indexes into plants of the plants on the tiles
	// x0..x1 x y0..y1 (the slice, not the world: a pointer through an
	// interface call escapes, render/env.Lists).
	PlantsOn(plants []game.PlantView, x0, y0, x1, y1 int, buf []int32) []int32
	// Chopping reports a pawn is felling a tree.
	Chopping(p game.PawnView) bool
	// AnimalAt is where an animal is drawn, in tiles.
	AnimalAt(an game.AnimalView) (x, y float64)
	// ListenCaskets cues a casket opening (the vault layer's hook).
	ListenCaskets(w game.WorldView)
}

// Cue is one thing to hear: a sound id at a world point (tiles), or
// global (interface sounds, thunder) with no position.
type Cue struct {
	ID     string
	X, Y   float64
	Vol    float64
	Global bool
}

// Loop is one wanted loop of the ambience: a sound id with its volume and
// stereo pan.
type Loop struct {
	ID       string
	Vol, Pan float64
}

// Bus is the renderer's sound cues: the pending cues for the director,
// last frame's world to hear changes, the pending thunder and the animals'
// sound memory.
type Bus struct {
	Cues      []Cue
	Memo      Memo
	ThunderAt uint64
	// Voice is the animals' sound memory (animal_sounds.go).
	Voice Voice
	// SoundHas reports a sound id exists (the App wires Sound.Has; nil:
	// all do).
	SoundHas func(id string) bool
	// Host is wired by the root before the first listen.
	Host Host
}

// New is an empty bus.
func New() *Bus { return &Bus{} }

// Cue notes a sound at a world point (tiles).
func (b *Bus) Cue(id string, x, y, vol float64) {
	b.Cues = append(b.Cues, Cue{ID: id, X: x, Y: y, Vol: vol})
}

// Global notes a sound with no position.
func (b *Bus) Global(id string) {
	b.Cues = append(b.Cues, Cue{ID: id, Vol: 1, Global: true})
}

// Take hands this frame's cues to the director.
func (b *Bus) Take() []Cue {
	c := b.Cues
	b.Cues = nil
	return c
}

const (
	// MaxPerID is the copies of one sound the director plays per frame at
	// full zoom.
	MaxPerID = 3
	// PanWidth is how far left or right a cue can sit.
	PanWidth = 0.8
)

// ZoomNear is the zoom at or closer than which effects play at full
// volume and full stereo width.
const ZoomNear = 1.5

// ZoomEar is how the camera's distance shapes the effects: from ZoomNear
// out to the map's zoom-out limit (zoomOut) they get quieter, narrower in
// stereo and fewer per frame, reaching silence fully zoomed out. Each
// halving of the zoom is an equal step.
func ZoomEar(zoom, zoomOut float64) (gain, panScale float64, perID int) {
	span := math.Log2(ZoomNear / zoomOut)
	if zoom <= 0 || span <= 0 { // the map fits at ZoomNear: no room to fade
		return 1, 1, MaxPerID
	}
	far := math.Max(0, math.Min(1, math.Log2(ZoomNear/zoom)/span))
	gain = 1 - far
	panScale = 1 - 0.5*far
	switch {
	case far > 0.6:
		perID = 1
	case far > 0.3:
		perID = 2
	default:
		perID = MaxPerID
	}
	return gain, panScale, perID
}

// Spatial places a world point (tiles) for the ear: pan from where it is
// across the view, volume falling off to silence half a view beyond the
// edge, and fading with camera distance (ZoomEar) to silence fully zoomed
// out. ok is false when it is out of earshot.
func Spatial(rv *render.View, x, y float64, v camera.Viewport) (vol, pan float64, ok bool) {
	sx, sy := rv.WorldToScreenF(x, y)
	fx, fy := float64(sx), float64(sy)
	cx := (v.X0 + v.X1) / 2
	half := v.W() / 2
	if half <= 0 || v.H() <= 0 {
		return 0, 0, false
	}
	gain, panScale, _ := ZoomEar(rv.Cam.Zoom, rv.Cam.MinZoomFor(v, rv.MapW, rv.MapH))
	pan = math.Max(-1, math.Min(1, (fx-cx)/half)) * PanWidth * panScale
	dx := math.Max(0, math.Max(v.X0-fx, fx-v.X1)) / half
	dy := math.Max(0, math.Max(v.Y0-fy, fy-v.Y1)) / (v.H() / 2)
	vol = (1 - math.Max(dx, dy)) * gain
	if vol <= 0.01 {
		return 0, 0, false
	}
	return vol, pan, true
}
