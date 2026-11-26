// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package cue

import (
	"strconv"

	"rimworld-go/internal/adapters/inbound/ebiten/camera"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/application/game"
)

// The hiss of a burning turret fuse (MAP-21): a positioned loop at each
// sparking turret, whatever its power state. Whether it plays is a pure
// function of this frame's world (BuildingView.Sparking): a turret that blew
// up, was crated or stopped sparking adds no key, so the director fades the
// loop. Nothing is remembered between frames.

// fuseHissID is the generated loop (audiogen structure recipes).
const fuseHissID = "amb.turret_fuse_hiss"

// fuseHeard keeps the loudest MaxMachineLoops sparking turrets of a frame,
// sorted loudest first, in a fixed array: no slice, map or sort.
type fuseHeard struct {
	hs [MaxMachineLoops]struct {
		id       uint64
		vol, pan float64
	}
	n int
}

// hear offers a sparking building; it is kept when on screen and among the
// loudest so far. The position is the footprint centre, as for the hums.
func (f *fuseHeard) hear(bd *game.BuildingView, rv *render.View, v camera.Viewport) {
	x, y := float64(bd.X)+float64(max(bd.W, 1))/2, float64(bd.Y)+float64(max(bd.H, 1))/2
	vol, pan, ok := Spatial(rv, x, y, v)
	if !ok {
		return
	}
	k := f.n
	if f.n < len(f.hs) {
		f.n++
	} else if vol <= f.hs[f.n-1].vol { // #nosec G602 -- else-branch: n == len(hs) == MaxMachineLoops > 0
		return
	} else {
		k = f.n - 1
	}
	for ; k > 0 && f.hs[k-1].vol < vol; k-- {
		f.hs[k] = f.hs[k-1]
	}
	f.hs[k].id, f.hs[k].vol, f.hs[k].pan = uint64(bd.ID), vol, pan
}

// into writes the kept turrets' loops under "fuse:<id>".
func (f *fuseHeard) into(want map[string]Loop) {
	for _, h := range f.hs[:f.n] {
		want["fuse:"+strconv.FormatUint(h.id, 10)] = Loop{ID: fuseHissID, Vol: h.vol, Pan: h.pan}
	}
}
