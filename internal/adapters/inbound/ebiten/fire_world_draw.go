package ebiten

import (
	"image/color"
	"math"
	"sort"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

// World fire presentation (PLAY-015 §5.3): the flame sprite of every
// burning cell by size with an animated frame, embers and smoke, orange
// light (light.go reads fireLightReach), one crackling loop per cluster of
// fire (sound_director.go), the Extinguish order for drafted pawns, the ash
// decals and the sim-driven lightning. It reads WorldView.Fires only.

// Size classes of a flame: small < 34 <= medium < 67 <= large.
const (
	fireMediumFrom = 34
	fireLargeFrom  = 67
	fireFrames     = 4
	fireFrameTicks = 5 // presentation frames per flame frame
)

// fireClassOf names the flame size class of a fire of the given size.
func fireClassOf(size int) string {
	switch {
	case size >= fireLargeFrom:
		return "large"
	case size >= fireMediumFrom:
		return "medium"
	}
	return "small"
}

// fireFrameOf is the animation frame of the flame on (x, y): the frame
// counter advances every fireFrameTicks presentation frames, offset by a
// hash of the cell so neighbouring flames do not flicker in step.
func fireFrameOf(present uint64, x, y int) int {
	off := mix64(uint64(x)*73856093^uint64(y)*19349663) % fireFrames
	return int((present/fireFrameTicks + off) % fireFrames)
}

// fireSpriteKey is the effect sprite of a flame.
func fireSpriteKey(size int, present uint64, x, y int) string {
	return "effect/fire_" + fireClassOf(size) + "_" + string(rune('0'+fireFrameOf(present, x, y)))
}

// fireEmberEvery and fireSmokeEvery are the presentation frames between
// one ember (smoke puff) and the next per flame: bigger fires emit more.
func fireEmberEvery(size int) uint64 {
	switch fireClassOf(size) {
	case "large":
		return 8
	case "medium":
		return 14
	}
	return 26
}

func fireSmokeEvery(size int) uint64 {
	switch fireClassOf(size) {
	case "large":
		return 10
	case "medium":
		return 18
	}
	return 32
}

// fireLightReach is how many tiles around a burning cell its light
// reaches; fireLightStrength is the light at the cell itself (0..1).
func fireLightReach(size int) int {
	switch fireClassOf(size) {
	case "large":
		return 4
	case "medium":
		return 3
	}
	return 2
}

func fireLightStrength(size int) float32 {
	switch fireClassOf(size) {
	case "large":
		return 1
	case "medium":
		return 0.8
	}
	return 0.55
}

// fireScale is the fallback (sprite-less) flame scale by size.
func fireScale(size int) float64 { return 0.5 + 0.9*float64(min(max(size, 1), 100))/100 }

// fireFx is the renderer's fire state: the clusters (rebuilt when the
// fire list's version moves) and the sim strikes already shown.
type fireFx struct {
	clusterVer uint64
	clusterOK  bool
	clusters   []fireCluster
	strikeSeen uint64
}

// fireCluster is a group of 8-connected burning cells: one crackle loop.
type fireCluster struct {
	Key     [2]int  // the smallest cell, a stable id while the blaze lives
	X, Y    float64 // centre, world tiles
	N       int
	MaxSize int
}

// fireClusters groups the burning cells by 8-connectivity, biggest first.
func fireClusters(fires []game.FireView) []fireCluster {
	if len(fires) == 0 {
		return nil
	}
	idx := make(map[[2]int]int, len(fires))
	for i, f := range fires {
		idx[[2]int{f.X, f.Y}] = i
	}
	seen := make([]bool, len(fires))
	var out []fireCluster
	var stack []int
	for i := range fires {
		if seen[i] {
			continue
		}
		seen[i] = true
		stack = append(stack[:0], i)
		c := fireCluster{Key: [2]int{fires[i].X, fires[i].Y}}
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

// fireLoopGain is a cluster's loudness: a lone small fire is half, a blaze
// of a dozen cells full.
func fireLoopGain(c fireCluster) float64 {
	return math.Min(1, 0.5+float64(c.N)/24+float64(c.MaxSize)/400)
}

// fireClustersOf caches the clusters of w until its fire list changes.
func (r *Renderer) fireClustersOf(w game.WorldView) []fireCluster {
	f := &r.fireFx
	if !f.clusterOK || f.clusterVer != w.FireVer || (len(w.Fires) == 0) != (len(f.clusters) == 0) {
		f.clusters, f.clusterVer, f.clusterOK = fireClusters(w.Fires), w.FireVer, true
	}
	return f.clusters
}

// drawWorldFires draws the flame of every visible burning cell, above the
// objects and under the weather and fog. A missing sprite falls back to
// the procedural flame, so the fire is visible before the art lands.
func (r *Renderer) drawWorldFires(screen *ebiten.Image, w game.WorldView, tile float64) {
	if len(w.Fires) == 0 {
		return
	}
	for _, f := range w.Fires {
		if !r.onScreenTile(screen, f.X, f.Y, tile) {
			continue
		}
		key := fireSpriteKey(f.Size, r.present, f.X, f.Y)
		sp, img := r.art.sprite(key), r.art.image(key, color.NRGBA{})
		sx, sy := r.worldToScreenF(float64(f.X)+0.5, float64(f.Y)+0.9)
		if sp == nil || img == nil {
			t := float64(r.present) + float64(f.X*31+f.Y*17)
			drawFlameScreen(screen, sx, sy-float32(tile*0.25), t, tile, fireScale(f.Size))
			continue
		}
		geo := ebiten.GeoM{}
		geo.Translate(-float64(sp.meta.Origin[0]), -float64(sp.meta.Origin[1]))
		geo.Scale(tile/artTilePx, tile/artTilePx)
		geo.Translate(float64(sx), float64(sy))
		screen.DrawImage(img, &ebiten.DrawImageOptions{GeoM: geo, Filter: ebiten.FilterLinear})
	}
}

// maxBlazeEmits caps the particles one frame's blaze may emit.
const maxBlazeEmits = 40

// emitBlazeParticles sends embers and smoke up from the burning cells, a
// rate by size, staggered by cell so they do not puff together.
func (r *Renderer) emitBlazeParticles(w game.WorldView) {
	emitted := 0
	for _, f := range w.Fires {
		if emitted >= maxBlazeEmits {
			return
		}
		ph := uint64(f.X*7 + f.Y*13)
		seed := uint64(f.X)*104729 + uint64(f.Y)*1299709 + r.present
		x, y := float64(f.X)+0.5, float64(f.Y)+0.5-0.3
		if (r.present+ph)%fireEmberEvery(f.Size) == 0 {
			r.burst(partEmber, x, y, 1, seed)
			emitted++
		}
		if (r.present+ph)%fireSmokeEvery(f.Size) == 0 {
			r.burst(partSmoke, x, y-0.2, 1, seed^0x5bd1e995)
			emitted++
		}
	}
}

// --- ash decals -------------------------------------------------------------

// ashKind is the stain kind the sim leaves where something burned out.
const ashKind = "ash"

// ashKey is the decal of an ash stain: one of overlay/ash_0..2 by variant.
func ashKey(variant uint8) string { return "overlay/ash_" + string(rune('0'+variant%3)) }

// drawAshStains draws the burnt-ground decals under objects.
func (r *Renderer) drawAshStains(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	for _, s := range w.Stains {
		if s.Kind != ashKind || s.Amount <= 0 {
			continue
		}
		a := float32(clamp01(s.Amount))
		r.drawSpriteCS(screen, ashKey(s.Variant), color.NRGBA{}, float64(s.X), float64(s.Y), tile, sw, sh, colorScale{a, a, a, a})
	}
}

// --- lightning from the sim ---------------------------------------------------

// onSimStrike shows one sim lightning strike: the flash now, the crack at
// the strike and the thunder rolling in after its delay.
func (r *Renderer) onSimStrike(x, y int, tick uint64) {
	e := &r.env
	e.flashBucket, e.flashAt, e.flashSeen = tick/lightningBucket, r.present, true
	r.thunderAt = r.present + thunderDelay(tick)
	r.cue("sfx.lightning_strike", float64(x)+0.5, float64(y)+0.5, 1)
}

// strikeFreshTicks is how recent a sim strike must be to flash: older ones
// in the view (a loaded game keeps the last hour) stay silent.
const strikeFreshTicks = 60

// showStrikes flashes and cracks once for every sim strike not yet shown.
func (r *Renderer) showStrikes(w game.WorldView) {
	f := &r.fireFx
	now := uint64(r.game.Tick())
	for _, s := range w.Strikes {
		if s.Tick <= f.strikeSeen {
			continue
		}
		f.strikeSeen = s.Tick
		if s.Tick+strikeFreshTicks >= now {
			r.onSimStrike(s.X, s.Y, s.Tick)
		}
	}
}

// --- Extinguish order ---------------------------------------------------------

// fireOn is the burning cell at (x, y) in w, if any.
func fireOn(w game.WorldView, x, y int) (game.FireView, bool) {
	for _, f := range w.Fires {
		if f.X == x && f.Y == y {
			return f, true
		}
	}
	return game.FireView{}, false
}

// fireOrders is the right-click menu of a drafted pawn on a burning cell:
// walk there, or beat the fire out (a forced order, anywhere). Nil when
// the pawn is not drafted or the cell is not burning.
func fireOrders(p game.PawnView, w game.WorldView, tx, ty int) []game.OrderOption {
	if !p.Drafted {
		return nil
	}
	if _, ok := fireOn(w, tx, ty); !ok {
		return nil
	}
	return []game.OrderOption{
		{Label: "Go here", Cmd: command.Command{Kind: command.MovePawn, Pawn: uint64(p.ID), X: tx, Y: ty}},
		{Label: "Extinguish fire", Cmd: command.Command{Kind: command.Prioritize, Pawn: uint64(p.ID), X: tx, Y: ty, Order: command.OrderExtinguish}},
	}
}

// floatIcon is the icon of an order-menu option ("" for none).
func floatIcon(o game.OrderOption) string {
	if o.Cmd.Kind == command.Prioritize && o.Cmd.Order == command.OrderExtinguish {
		return "extinguish"
	}
	return ""
}

// --- fire loop ----------------------------------------------------------------

// maxBlazeLoops is how many fire clusters are heard at once.
const maxBlazeLoops = 3

// blazeAmbience adds one positioned amb.fire loop per fire cluster (the
// loudest few): the cluster's centre sets the pan and distance, its size
// the loudness.
func (r *Renderer) blazeAmbience(want map[string]loopWant, w game.WorldView, v viewport) {
	type heardFire struct {
		key      string
		vol, pan float64
	}
	var heard []heardFire
	for _, c := range r.fireClustersOf(w) {
		vol, pan, ok := spatial(r, c.X, c.Y, v)
		if !ok {
			continue
		}
		heard = append(heard, heardFire{"blaze:" + uiNum(c.Key[0]) + ":" + uiNum(c.Key[1]), vol * fireLoopGain(c), pan})
	}
	sort.Slice(heard, func(i, j int) bool { return heard[i].vol > heard[j].vol })
	for i, h := range heard {
		if i == maxBlazeLoops {
			break
		}
		want[h.key] = loopWant{id: "amb.fire", vol: h.vol, pan: h.pan}
	}
}

// --- light --------------------------------------------------------------------

// blazeLight fills a w×h lamp-light grid (reusing buf) from the burning
// cells: each lights the tiles within fireLightReach(size) by a linear
// falloff to its strength, the brightest source winning. Cost is the
// light squares of the fires, run once per fire-list version.
func blazeLight(buf []float32, fires []game.FireView, w, h int) []float32 {
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
		reach, str := fireLightReach(f.Size), fireLightStrength(f.Size)
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
