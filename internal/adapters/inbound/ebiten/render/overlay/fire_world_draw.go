package overlay

import (
	"image/color"
	"math"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
)

// World fire presentation (PLAY-015 §5.3): the flame sprite of every
// burning cell by size with an animated frame, embers and smoke, orange
// light (light.go reads FireLightReach), one crackling loop per cluster of
// fire (sound_director.go), the Extinguish order for drafted pawns, the ash
// decals and the sim-driven lightning. It reads WorldView.Fires only.

// Size classes of a flame: small < 34 <= medium < 67 <= large.
const (
	fireMediumFrom = 34
	fireLargeFrom  = 67
	FireFrames     = 4
	fireFrameTicks = 5 // presentation frames per flame frame
)

// FireClassOf names the flame size class of a fire of the given size.
func FireClassOf(size int) string {
	switch {
	case size >= fireLargeFrom:
		return "large"
	case size >= fireMediumFrom:
		return "medium"
	}
	return "small"
}

// FireFrameOf is the animation frame of the flame on (x, y): the frame
// counter advances every fireFrameTicks presentation frames, offset by a
// hash of the cell so neighbouring flames do not flicker in step.
func FireFrameOf(present uint64, x, y int) int {
	off := art.Mix64(uint64(x)*73856093^uint64(y)*19349663) % FireFrames
	return int((present/fireFrameTicks + off) % FireFrames)
}

// FireSpriteKey is the effect sprite of a flame.
func FireSpriteKey(size int, present uint64, x, y int) string {
	return "effect/fire_" + FireClassOf(size) + "_" + string(rune('0'+FireFrameOf(present, x, y)))
}

// FireEmberEvery and FireSmokeEvery are the presentation frames between
// one ember (smoke puff) and the next per flame: bigger fires emit more.
func FireEmberEvery(size int) uint64 {
	switch FireClassOf(size) {
	case "large":
		return 8
	case "medium":
		return 14
	}
	return 26
}

// FireSmokeEvery is FireEmberEvery for smoke puffs.
func FireSmokeEvery(size int) uint64 {
	switch FireClassOf(size) {
	case "large":
		return 10
	case "medium":
		return 18
	}
	return 32
}

// FireLightReach is how many tiles around a burning cell its light
// reaches; FireLightStrength is the light at the cell itself (0..1).
func FireLightReach(size int) int {
	switch FireClassOf(size) {
	case "large":
		return 4
	case "medium":
		return 3
	}
	return 2
}

// FireLightStrength is the light a fire of the size gives at its cell (0..1).
func FireLightStrength(size int) float32 {
	switch FireClassOf(size) {
	case "large":
		return 1
	case "medium":
		return 0.8
	}
	return 0.55
}

// fireScale is the fallback (sprite-less) flame scale by size.
func fireScale(size int) float64 { return 0.5 + 0.9*float64(min(max(size, 1), 100))/100 }

// FireFx is the renderer's fire state: the clusters (rebuilt when the
// fire list's version moves) and the sim strikes already shown.
type FireFx struct {
	clusterVer uint64
	clusterOK  bool
	clusters   []FireCluster
	strikeSeen uint64
}

// FireCluster is a group of 8-connected burning cells: one crackle loop.
type FireCluster struct {
	Key     [2]int  // the smallest cell, a stable id while the blaze lives
	X, Y    float64 // centre, world tiles
	N       int
	MaxSize int
}

// FireClusters groups the burning cells by 8-connectivity, biggest first.
func FireClusters(fires []game.FireView) []FireCluster {
	if len(fires) == 0 {
		return nil
	}
	idx := make(map[[2]int]int, len(fires))
	for i, f := range fires {
		idx[[2]int{f.X, f.Y}] = i
	}
	seen := make([]bool, len(fires))
	var out []FireCluster
	var stack []int
	for i := range fires {
		if seen[i] {
			continue
		}
		seen[i] = true
		stack = append(stack[:0], i)
		c := FireCluster{Key: [2]int{fires[i].X, fires[i].Y}}
		var sx, sy float64
		for len(stack) > 0 {
			f := fires[stack[len(stack)-1]]
			stack = stack[:len(stack)-1]
			c.N++
			sx += float64(f.X) + 0.5
			sy += float64(f.Y) + 0.5
			c.MaxSize = max(c.MaxSize, f.Size)
			if f.Y < c.Key[1] || (f.Y == c.Key[1] && f.X < c.Key[0]) {
				c.Key = [2]int{f.X, f.Y}
			}
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					if j, ok := idx[[2]int{f.X + dx, f.Y + dy}]; ok && !seen[j] {
						seen[j] = true
						stack = append(stack, j)
					}
				}
			}
		}
		c.X, c.Y = sx/float64(c.N), sy/float64(c.N)
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].N != out[j].N {
			return out[i].N > out[j].N
		}
		return out[i].Key[1]*100000+out[i].Key[0] < out[j].Key[1]*100000+out[j].Key[0]
	})
	return out
}

// FireLoopGain is a cluster's loudness: a lone small fire is half, a blaze
// of a dozen cells full.
func FireLoopGain(c FireCluster) float64 {
	return math.Min(1, 0.5+float64(c.N)/24+float64(c.MaxSize)/400)
}

// fireClustersOf caches the clusters of w until its fire list changes.
func (o *Fires) fireClustersOf(w game.WorldView) []FireCluster {
	f := &o.fx
	if !f.clusterOK || f.clusterVer != w.FireVer || (len(w.Fires) == 0) != (len(f.clusters) == 0) {
		f.clusters, f.clusterVer, f.clusterOK = FireClusters(w.Fires), w.FireVer, true
	}
	return f.clusters
}

// DrawWorldFires draws the flame of every visible burning cell, above the
// objects and under the weather and fog. A missing sprite falls back to
// the procedural flame, so the fire is visible before the art lands.
func (o *Fires) DrawWorldFires(v *render.View, screen *ebiten.Image, w game.WorldView, tile float64) {
	if len(w.Fires) == 0 {
		return
	}
	for _, f := range w.Fires {
		if !onScreenTile(v, screen, f.X, f.Y, tile) {
			continue
		}
		key := FireSpriteKey(f.Size, v.Present, f.X, f.Y)
		sp, img := v.Art.Sprite(key), v.Art.Image(key, color.NRGBA{})
		sx, sy := v.WorldToScreenF(float64(f.X)+0.5, float64(f.Y)+0.9)
		if sp == nil || img == nil {
			t := float64(v.Present) + float64(f.X*31+f.Y*17)
			o.Host.Flame(screen, sx, sy-float32(tile*0.25), t, tile, fireScale(f.Size))
			continue
		}
		geo := ebiten.GeoM{}
		geo.Translate(-float64(sp.Meta.Origin[0]), -float64(sp.Meta.Origin[1]))
		geo.Scale(tile/art.TilePx, tile/art.TilePx)
		geo.Translate(float64(sx), float64(sy))
		screen.DrawImage(img, &ebiten.DrawImageOptions{GeoM: geo, Filter: ebiten.FilterLinear})
	}
}

// maxBlazeEmits caps the particles one frame's blaze may emit.
const maxBlazeEmits = 40

// EmitBlazeParticles sends embers and smoke up from the burning cells, a
// rate by size, staggered by cell so they do not puff together.
func (o *Fires) EmitBlazeParticles(v *render.View, w game.WorldView) {
	emitted := 0
	for _, f := range w.Fires {
		if emitted >= maxBlazeEmits {
			return
		}
		ph := uint64(f.X*7 + f.Y*13)
		seed := uint64(f.X)*104729 + uint64(f.Y)*1299709 + v.Present
		x, y := float64(f.X)+0.5, float64(f.Y)+0.5-0.3
		if (v.Present+ph)%FireEmberEvery(f.Size) == 0 {
			o.Host.Burst(Ember, x, y, 1, seed)
			emitted++
		}
		if (v.Present+ph)%FireSmokeEvery(f.Size) == 0 {
			o.Host.Burst(Smoke, x, y-0.2, 1, seed^0x5bd1e995)
			emitted++
		}
	}
}

// --- ash decals -------------------------------------------------------------

// ashKind is the stain kind the sim leaves where something burned out.
const ashKind = "ash"

// AshKey is the decal of an ash stain: one of overlay/ash_0..2 by variant.
func AshKey(variant uint8) string { return "overlay/ash_" + string(rune('0'+variant%3)) }

// DrawAshStains draws the burnt-ground decals under objects.
func (o *Fires) DrawAshStains(v *render.View, ps sprite.Pass, screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	for _, s := range w.Stains {
		if s.Kind != ashKind || s.Amount <= 0 {
			continue
		}
		a := float32(gfx.Clamp01(s.Amount))
		o.P.DrawCS(v, ps, screen, AshKey(s.Variant), color.NRGBA{}, float64(s.X), float64(s.Y), tile, sw, sh, sprite.ColorScale{a, a, a, a})
	}
}

// --- lightning from the sim ---------------------------------------------------

// OnSimStrike shows one sim lightning strike: the flash now, the crack at
// the strike and the thunder rolling in after its delay.
func (o *Fires) OnSimStrike(x, y int, tick uint64) {
	o.Host.Flash(tick)
	o.Host.Cue("sfx.lightning_strike", float64(x)+0.5, float64(y)+0.5, 1)
}

// strikeFreshTicks is how recent a sim strike must be to flash: older ones
// in the view (a loaded game keeps the last hour) stay silent.
const strikeFreshTicks = 60

// ShowStrikes flashes and cracks once for every sim strike not yet shown.
func (o *Fires) ShowStrikes(v *render.View, w game.WorldView) {
	f := &o.fx
	now := uint64(v.Tick)
	for _, s := range w.Strikes {
		if s.Tick <= f.strikeSeen {
			continue
		}
		f.strikeSeen = s.Tick
		if s.Tick+strikeFreshTicks >= now {
			o.OnSimStrike(s.X, s.Y, s.Tick)
		}
	}
}

// --- fire loop ----------------------------------------------------------------

// maxBlazeLoops is how many fire clusters are heard at once.
const maxBlazeLoops = 3

// BlazeAmbience adds one positioned amb.fire loop per fire cluster (the
// loudest few): the cluster's centre sets the pan and distance, its size
// the loudness. place puts a world point for the ear (ok false: not
// heard); want adds a loop.
func (o *Fires) BlazeAmbience(w game.WorldView, place func(x, y float64) (vol, pan float64, ok bool), want func(key string, vol, pan float64)) {
	type heardFire struct {
		key      string
		vol, pan float64
	}
	var heard []heardFire
	for _, c := range o.fireClustersOf(w) {
		vol, pan, ok := place(c.X, c.Y)
		if !ok {
			continue
		}
		heard = append(heard, heardFire{"blaze:" + uikit.Num(c.Key[0]) + ":" + uikit.Num(c.Key[1]), vol * FireLoopGain(c), pan})
	}
	sort.Slice(heard, func(i, j int) bool { return heard[i].vol > heard[j].vol })
	for i, h := range heard {
		if i == maxBlazeLoops {
			break
		}
		want(h.key, h.vol, h.pan)
	}
}

// --- light --------------------------------------------------------------------

// BlazeLight fills a w×h lamp-light grid (reusing buf) from the burning
// cells: each lights the tiles within FireLightReach(size) by a linear
// falloff to its strength, the brightest source winning. Cost is the
// light squares of the fires, run once per fire-list version.
func BlazeLight(buf []float32, fires []game.FireView, w, h int) []float32 {
	if len(fires) == 0 || w <= 0 || h <= 0 {
		return buf[:0]
	}
	n := w * h
	if cap(buf) < n {
		buf = make([]float32, n)
	}
	buf = buf[:n]
	clear(buf)
	for _, f := range fires {
		reach, str := FireLightReach(f.Size), FireLightStrength(f.Size)
		for dy := -reach; dy <= reach; dy++ {
			y := f.Y + dy
			if y < 0 || y >= h {
				continue
			}
			for dx := -reach; dx <= reach; dx++ {
				x := f.X + dx
				if x < 0 || x >= w {
					continue
				}
				d := math.Hypot(float64(dx), float64(dy))
				if d > float64(reach) {
					continue
				}
				if v := str * float32(1-d/(float64(reach)+0.5)); v > buf[y*w+x] {
					buf[y*w+x] = v
				}
			}
		}
	}
	return buf
}
