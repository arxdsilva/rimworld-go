package ebiten

import (
	"image/color"

	"rimworld-go/internal/application/game"

	"rimworld-go/internal/domain/sim/entity"
)

// Explosive animals (emberbuck, cinder rat) burst when they die. The sim
// stamps each burst; this draws it once, as particles emitted from the
// renderer's per-frame advance (never from Draw): an orange flash, embers,
// sparks and a puff of smoke.

// burstEvent is one stamped explosion: who burst, where, and when.
type burstEvent struct {
	ID   entity.EntityID
	X, Y int
	Tick uint64
}

type burstKey struct {
	id   entity.EntityID
	tick uint64
}

// burstTracker remembers which stamps already burst so each draws once.
type burstTracker struct {
	seen map[burstKey]struct{}
}

// maxBurstsTracked bounds the seen set (old stamps are long gone by then).
const maxBurstsTracked = 256

var burstFlashClr = color.RGBA{255, 168, 56, 255}

// Emit bursts every event it has not seen yet. Call it from advance with
// the world's burst stamps.
func (t *burstTracker) Emit(r *Renderer, events []burstEvent) {
	if t.seen == nil || len(t.seen) > maxBurstsTracked {
		t.seen = map[burstKey]struct{}{}
	}
	for _, ev := range events {
		k := burstKey{ev.ID, ev.Tick}
		if _, done := t.seen[k]; done {
			continue
		}
		t.seen[k] = struct{}{}
		r.emitBurst(ev)
	}
}

// emitBurst throws one explosion's particles, deterministic in the stamp.
func (r *Renderer) emitBurst(ev burstEvent) {
	x, y := float64(ev.X)+0.5, float64(ev.Y)+0.4
	seed := uint64(ev.ID)*0x9E3779B1 ^ ev.Tick*0x85EBCA6B
	r.cue(voiceBurst, x, y, volBurst) // the bang (animal_sounds.go)

	// the flash: one big, short-lived ember that swells and fades
	if len(r.parts) < maxParticles {
		r.parts = append(r.parts, particle{Kind: partEmber, X: x, Y: y, Born: r.present,
			Life: 9, Size: 0.3, Grow: 0.03, Clr: burstFlashClr})
	}
	spread := func(kind partKind, n int, s uint64, speed float64) {
		from := len(r.parts)
		r.burst(kind, x, y, n, s)
		for i := from; i < len(r.parts); i++ {
			r.parts[i].Vx *= speed
			r.parts[i].Vy *= speed
		}
	}
	spread(partEmber, 16, seed^0x1, 5)
	spread(partSpark, 8, seed^0x2, 1.4)
	spread(partSmoke, 6, seed^0x3, 6)
}

// emitWorldBursts bursts the world's new explosive-death stamps (each
// once; a stamp is keyed by its tile and tick).
func (r *Renderer) emitWorldBursts(w game.WorldView) {
	if len(w.Bursts) == 0 {
		return
	}
	evs := make([]burstEvent, 0, len(w.Bursts))
	for _, b := range w.Bursts {
		evs = append(evs, burstEvent{ID: entity.EntityID(uint64(uint32(b.X))<<32 | uint64(uint32(b.Y))), X: b.X, Y: b.Y, Tick: b.Tick})
	}
	r.bursts.Emit(r, evs)
}
