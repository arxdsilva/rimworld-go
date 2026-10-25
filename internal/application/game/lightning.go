package game

import (
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/world/weather"
)

// Lightning (PLAY-015 §5.2): during a storm a bolt strikes a random
// outdoor cell every 1 to 3 in-game hours and sets it alight when there
// is something to burn. The schedule is a pure hash of the stream seed and
// the time: one strike per strikeSlot, at a hashed moment in the slot's
// middle hour, so two strikes are 1 to 3 hours apart. Nothing is drawn
// from g.rng and nothing about the schedule needs saving. The renderer
// flashes and thunders from WorldView.Strikes.

const (
	lightningSalt = 0x11647B01
	// strikeSlot is one strike's window: 2 hours, the bolt falling in its
	// middle hour.
	strikeSlot = 2 * clock.TicksPerHour
	// strikeKeep is how long a strike stays in the read model.
	strikeKeep = clock.TicksPerHour
	// strikeTries is how many cells a bolt tries before it gives up on
	// finding open sky (a map under a mountain).
	strikeTries = 8
)

// StrikeView is one lightning strike: where and when (tick).
type StrikeView struct {
	X, Y int
	Tick uint64
}

// strikeTick is the tick of slot's strike.
func strikeTick(seed, slot uint64) uint64 {
	return slot*strikeSlot + clock.TicksPerHour/2 + fireMix(seed^slot)%clock.TicksPerHour
}

// stepLightning strikes when the storm's schedule says so (called every
// tick from the weather phase) and forgets strikes older than strikeKeep.
func (g *Game) stepLightning() {
	now := uint64(g.clock.Now())
	if rt := &g.fire.rt; len(rt.strikes) > 0 && now-rt.strikes[0].Tick > strikeKeep {
		kept := rt.strikes[:0]
		for _, s := range rt.strikes {
			if now-s.Tick <= strikeKeep {
				kept = append(kept, s)
			}
		}
		rt.strikes = kept
	}
	if g.weather.Kind != weather.Storm {
		return
	}
	seed := g.streams.seed ^ lightningSalt
	slot := now / strikeSlot
	if strikeTick(seed, slot) != now {
		return
	}
	x, y, ok := g.strikeCell(seed, slot)
	if !ok {
		return
	}
	g.fire.rt.strikes = append(g.fire.rt.strikes, StrikeView{X: x, Y: y, Tick: now})
	g.viewGen++
	if g.ignite(x, y, lightningFireSize) {
		g.message(x, y, "Lightning started a fire!")
	}
}

// strikeCell is the outdoor cell slot's bolt hits.
func (g *Game) strikeCell(seed, slot uint64) (int, int, bool) {
	w, h := uint64(g.grid.Width()), uint64(g.grid.Height())
	if w == 0 || h == 0 {
		return 0, 0, false
	}
	for try := uint64(0); try < strikeTries; try++ {
		r := fireMix(seed ^ fireMix(slot<<8|try))
		x, y := int(r%w), int((r>>32)%h)
		if g.underSky(x, y) {
			return x, y, true
		}
	}
	return 0, 0, false
}
