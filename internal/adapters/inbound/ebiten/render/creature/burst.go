package creature

import (
	"image/color"

	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/application/game"

	"rimworld-go/internal/domain/sim/entity"
)

// Explosive animals (emberbuck, cinder rat) burst when they die. The sim
// stamps each burst; this draws it once, as particles emitted from the
// renderer's per-frame advance (never from Draw): an orange flash, embers,
// sparks and a puff of smoke.

// BurstEvent is one stamped explosion: who burst, where, and when.
type BurstEvent struct {
	ID   entity.EntityID
	X, Y int
	Tick uint64
	// Blast: a weapon's explosion (DEF-2), voiced by cue.Bus.ListenBursts,
	// so it raises no creature bang of its own.
	Blast bool
}

type burstKey struct {
	id   entity.EntityID
	tick uint64
}

// BurstTracker remembers which stamps already burst so each draws once.
type BurstTracker struct {
	seen map[burstKey]struct{}
}

// maxBurstsTracked bounds the seen set (old stamps are long gone by then).
const maxBurstsTracked = 256

var burstFlashClr = color.RGBA{255, 168, 56, 255}

// Emit bursts every event it has not seen yet into c. Call it from advance
// with the world's burst stamps.
func (t *BurstTracker) Emit(c *Set, v *render.View, events []BurstEvent) {
	if t.seen == nil || len(t.seen) > maxBurstsTracked {
		t.seen = map[burstKey]struct{}{}
	}
	for _, ev := range events {
		k := burstKey{ev.ID, ev.Tick}
		if _, done := t.seen[k]; done {
			continue
		}
		t.seen[k] = struct{}{}
		c.emitBurst(v, ev)
	}
}

// emitBurst throws one explosion's particles, deterministic in the stamp.
func (c *Set) emitBurst(v *render.View, ev BurstEvent) {
	x, y := float64(ev.X)+0.5, float64(ev.Y)+0.4
	seed := uint64(ev.ID)*0x9E3779B1 ^ ev.Tick*0x85EBCA6B
	if !ev.Blast {
		c.Host.BurstCue(x, y) // the bang (render/cue)
	}

	// the flash: one big, short-lived ember that swells and fades
	if len(*c.Parts) < MaxParticles {
		*c.Parts = append(*c.Parts, Particle{Kind: PartEmber, X: x, Y: y, Born: v.Present,
			Life: 9, Size: 0.3, Grow: 0.03, Clr: burstFlashClr})
	}
	spread := func(kind PartKind, n int, s uint64, speed float64) {
		from := len(*c.Parts)
		c.Burst(v, kind, x, y, n, s)
		for i := from; i < len(*c.Parts); i++ {
			ps := *c.Parts
			ps[i].Vx *= speed
			ps[i].Vy *= speed
		}
	}
	spread(PartEmber, 16, seed^0x1, 5)
	spread(PartSpark, 8, seed^0x2, 1.4)
	spread(PartSmoke, 6, seed^0x3, 6)
}

// EmitWorldBursts bursts the world's new explosive-death stamps (each
// once; a stamp is keyed by its tile and tick).
func (c *Set) EmitWorldBursts(v *render.View, w game.WorldView) {
	if len(w.Bursts) == 0 {
		return
	}
	evs := c.burstBuf[:0] // reused each frame
	for _, b := range w.Bursts {
		evs = append(evs, BurstEvent{ID: entity.EntityID(uint64(uint32(b.X))<<32 | uint64(uint32(b.Y))), X: b.X, Y: b.Y, Tick: b.Tick,
			Blast: b.Blast != ""})
	}
	c.burstBuf = evs
	c.bursts.Emit(c, v, evs)
}
