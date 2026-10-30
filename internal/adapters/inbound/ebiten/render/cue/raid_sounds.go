package cue

import (
	"strconv"

	"rimworld-go/internal/adapters/inbound/ebiten/camera"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/application/game"
)

// Wave E raid sounds (DEF-4): a raid's drop pod falling and landing is
// voiced in ListenArrivals (signals.go, ArrivalView.Hostile); a grenade
// throw by weaponVoices (combat_sounds.go, sfx.grenade_throw). This file
// has the hum of a crashed ship part.

// shipPartKind is the building kind of a crashed ship part. R2 adds it to
// construction; until then nothing carries it and the hum stays silent.
const shipPartKind = "ship part"

// shipPartHum is the ambience loop near each crashed ship part, the
// nearest MaxMachineLoops of them. Positioned, quiet with distance and
// silent off screen, like the machine loops it is called from.
func (b *Bus) shipPartHum(rv *render.View, want map[string]Loop, w game.WorldView, v camera.Viewport) {
	type heard struct {
		id       uint64
		vol, pan float64
	}
	var hs [MaxMachineLoops]heard // the loudest few, kept sorted; no allocation
	n := 0
	for i := range w.Buildings {
		bd := &w.Buildings[i]
		if bd.Kind != shipPartKind {
			continue
		}
		x, y := float64(bd.X)+float64(max(bd.W, 1))/2, float64(bd.Y)+float64(max(bd.H, 1))/2
		vol, pan, ok := Spatial(rv, x, y, v)
		if !ok {
			continue
		}
		h := heard{uint64(bd.ID), vol, pan}
		k := n
		if n < len(hs) {
			n++
		} else if vol <= hs[n-1].vol {
			continue
		} else {
			k = n - 1
		}
		for ; k > 0 && hs[k-1].vol < h.vol; k-- {
			hs[k] = hs[k-1]
		}
		hs[k] = h
	}
	for _, h := range hs[:n] {
		want["shippart:"+strconv.FormatUint(h.id, 10)] = Loop{ID: "amb.ship_part_hum", Vol: h.vol, Pan: h.pan}
	}
}
