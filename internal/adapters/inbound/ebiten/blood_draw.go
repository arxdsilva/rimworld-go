package ebiten

import (
	"image"
	"image/color"
	"math"
	"strings"
	"sync"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// Blood on screen (SIM-020 / ART-013, ART §16.2): stains on the ground
// (procedural splat, pool and drip textures, faded by their amount and
// dried by age), blood on hurt creatures (lit.kage Custom3, set through
// r.figureBlood around figure layers), and sprays scaled by damage.

// --- floor stains (C1) ------------------------------------------------------

// stainShape is a stain's texture family, chosen by its amount.
type stainShape uint8

const (
	stainDrip  stainShape = iota // a few small drops (amount < 0.2)
	stainSplat                   // a blob with droplets thrown along one axis (< 0.6)
	stainPool                    // a smooth pool with a darker rim
)

// Stain texture sizes (px) and variant counts.
const (
	splatPx, splatN = 128, 6
	poolPx, poolN   = 160, 3
	dripPx, dripN   = 64, 4
	stainPad        = 2 // transparent px between textures in the atlas
)

// stainDryTicks is how long fresh blood takes to dry to its dark colour:
// one in-game day.
const stainDryTicks = clock.TicksPerDay

// stainMaxAlpha is a full stain's opacity (alpha = amount·0.85).
const stainMaxAlpha = 0.85

// Stain colours, fresh and dried (0..1).
var (
	bloodFresh = rgbHex(0x7a1414)
	bloodDry   = rgbHex(0x4a1d1a)
	ichorFresh = rgbHex(0xa8a030)
	ichorDry   = rgbHex(0x5c5a1c)
)

func rgbHex(h uint32) [3]float32 {
	return [3]float32{float32(h>>16&0xff) / 255, float32(h>>8&0xff) / 255, float32(h&0xff) / 255}
}

// stainShapeOf is the texture family and the drawn size (in tiles) for a
// stain's amount: drips 0.5 tile; splats 0.55–0.9 growing with the
// amount; pools 0.8–1.1.
func stainShapeOf(amount float64) (stainShape, float64) {
	switch {
	case amount < 0.2:
		return stainDrip, 0.5
	case amount < 0.6:
		return stainSplat, 0.55 + 0.35*(amount-0.2)/0.4
	}
	return stainPool, 0.8 + 0.3*clamp01((amount-0.6)/0.4)
}

// stainColour is a stain's colour at an age: fresh to dried over the
// first in-game day.
func stainColour(kind string, age uint64) [3]float32 {
	fresh, dry := bloodFresh, bloodDry
	if kind == "ichor" {
		fresh, dry = ichorFresh, ichorDry
	}
	t := float32(min(age, stainDryTicks)) / stainDryTicks
	return lerp3(fresh, dry, t)
}

// stainLook is how one stain is drawn: its texture, centre (world
// tiles), size (tiles), rotation and colour scale (straight alpha).
type stainLook struct {
	tex    int // index into the stain atlas
	cx, cy float64
	size   float64
	rot    float64
	cs     colorScale
}

// stainLookOf places and colours a stain: rotated Variant·60°, offset up
// to ±0.15 tile by a hash of its tile, alpha amount·0.85.
func stainLookOf(s game.StainView) stainLook {
	shape, size := stainShapeOf(s.Amount)
	th := tileHash(s.X, s.Y)
	c := stainColour(s.Kind, s.AgeTicks)
	return stainLook{
		tex:  stainTexIndex(shape, int(s.Variant)),
		cx:   float64(s.X) + 0.5 + 0.3*(hash01(th, 0x5a1)-0.5),
		cy:   float64(s.Y) + 0.5 + 0.3*(hash01(th, 0x5a2)-0.5),
		size: size,
		rot:  float64(s.Variant) * math.Pi / 3,
		cs:   colorScale{c[0], c[1], c[2], float32(stainMaxAlpha * clamp01(s.Amount))},
	}
}

// stainTexIndex is the atlas slot of a shape's variant: splats 0..5,
// pools 6..8, drips 9..12.
func stainTexIndex(shape stainShape, variant int) int {
	switch shape {
	case stainSplat:
		return variant % splatN
	case stainPool:
		return splatN + variant%poolN
	}
	return splatN + poolN + variant%dripN
}

// stainPixels is one stain texture: white (a pool's rim darker) with
// alpha, deterministic per shape and variant, transparent at its border.
func stainPixels(shape stainShape, variant int) *image.NRGBA {
	seed := uint64(shape)<<8 | uint64(variant)
	h := func(k ...uint64) float64 { return hash01(append([]uint64{0x57a1, seed}, k...)...) }
	px := dripPx
	switch shape {
	case stainSplat:
		px = splatPx
	case stainPool:
		px = poolPx
	}
	img := image.NewNRGBA(image.Rect(0, 0, px, px))
	c := float64(px) / 2
	// blob is an irregular disc: radius r0 modulated by a few harmonics
	type blob struct {
		x, y, r0 float64
		amp      [3]float64
		phase    [3]float64
	}
	var blobs []blob
	add := func(x, y, r0, wobble float64, k uint64) {
		b := blob{x: x, y: y, r0: r0}
		for i := range b.amp {
			b.amp[i] = wobble * (0.4 + 0.6*h(k, uint64(i), 1)) / float64(i+1)
			b.phase[i] = 2 * math.Pi * h(k, uint64(i), 2)
		}
		blobs = append(blobs, b)
	}
	switch shape {
	case stainSplat:
		add(c, c, 0.22*float64(px), 0.22, 0)
		n := 6 + int(h(10)*7) // 6..12 droplets thrown along the x axis
		for i := 0; i < n; i++ {
			side := 1.0
			if h(11, uint64(i)) < 0.3 {
				side = -1 // most fly one way
			}
			d := (0.28 + 0.13*h(12, uint64(i))) * float64(px)
			off := (h(13, uint64(i)) - 0.5) * 0.2 * float64(px)
			r := (0.012 + 0.03*h(14, uint64(i))) * float64(px)
			add(c+side*d, c+off, r, 0.1, uint64(20+i))
		}
	case stainPool: // a main pool with 2..3 lobes run off its edge
		add(c, c, 0.34*float64(px), 0.12, 0)
		n := 2 + int(h(10)*2)
		for i := 0; i < n; i++ {
			a := 2*math.Pi*(float64(i)+h(11, uint64(i))*0.6)/float64(n) + h(15)*math.Pi
			d := (0.12 + 0.08*h(12, uint64(i))) * float64(px)
			r := (0.14 + 0.06*h(13, uint64(i))) * float64(px)
			add(c+d*math.Cos(a), c+d*math.Sin(a), r, 0.1, uint64(20+i))
		}
	default:
		n := 3 + int(h(10)*3) // 3..5 drops
		for i := 0; i < n; i++ {
			a := 2 * math.Pi * h(11, uint64(i))
			d := (0.08 + 0.24*h(12, uint64(i))) * float64(px)
			r := (0.05 + 0.06*h(13, uint64(i))) * float64(px)
			add(c+d*math.Cos(a), c+d*math.Sin(a), r, 0.08, uint64(20+i))
		}
	}
	for y := 1; y < px-1; y++ {
		for x := 1; x < px-1; x++ {
			fx, fy := float64(x)+0.5, float64(y)+0.5
			edge := math.Inf(-1) // px inside the union's boundary (<0: outside)
			for _, b := range blobs {
				dx, dy := fx-b.x, fy-b.y
				d := math.Hypot(dx, dy)
				a := math.Atan2(dy, dx)
				r := b.r0
				for i := range b.amp {
					r += b.r0 * b.amp[i] * math.Sin(float64(i+2)*a+b.phase[i])
				}
				edge = max(edge, r-d)
			}
			cover := clamp01(edge + 0.5) // antialiased edge
			if cover <= 0 {
				continue
			}
			// a little mottling, so the stain reads as liquid, not a decal
			grain := 0.88 + 0.12*valueNoise(fx/7, fy/7, seed^0x9e)
			white := 1.0
			if shape == stainPool {
				white = 0.55 + 0.45*smoothstep01(edge/7) // a darker rim
				grain = 0.72 + 0.28*valueNoise(fx/16, fy/16, seed^0x9f)
				grain = max(grain, 1-smoothstep01(edge/10)) // the rim stays solid
			}
			v := uint8(math.Round(255 * white))
			img.SetNRGBA(x, y, color.NRGBA{v, v, v, uint8(math.Round(255 * cover * grain))})
		}
	}
	return img
}

// stainAtlas is every stain texture on one image (one texture page, so
// all stains draw in one batch), built once.
var stainAtlas struct {
	once  sync.Once
	img   *ebiten.Image
	rects [splatN + poolN + dripN]image.Rectangle
}

// stainAtlasImg builds the atlas on first use: splats in the first row,
// pools then drips in the second.
func stainAtlasImg() (*ebiten.Image, *[splatN + poolN + dripN]image.Rectangle) {
	stainAtlas.once.Do(func() {
		w := max(splatN*(splatPx+stainPad), poolN*(poolPx+stainPad)+dripN*(dripPx+stainPad))
		src := image.NewNRGBA(image.Rect(0, 0, w, splatPx+poolPx+2*stainPad))
		put := func(i int, shape stainShape, v, x, y int) {
			p := stainPixels(shape, v)
			r := p.Bounds().Add(image.Pt(x, y))
			for row := 0; row < p.Bounds().Dy(); row++ {
				copy(src.Pix[src.PixOffset(r.Min.X, r.Min.Y+row):], p.Pix[p.PixOffset(0, row):p.PixOffset(0, row+1)])
			}
			stainAtlas.rects[i] = r
		}
		for v := 0; v < splatN; v++ {
			put(stainTexIndex(stainSplat, v), stainSplat, v, v*(splatPx+stainPad), 0)
		}
		y := splatPx + stainPad
		for v := 0; v < poolN; v++ {
			put(stainTexIndex(stainPool, v), stainPool, v, v*(poolPx+stainPad), y)
		}
		for v := 0; v < dripN; v++ {
			put(stainTexIndex(stainDrip, v), stainDrip, v, poolN*(poolPx+stainPad)+v*(dripPx+stainPad), y)
		}
		stainAtlas.img = ebiten.NewImageFromImage(src)
	})
	return stainAtlas.img, &stainAtlas.rects
}

// stainBatch is one frame's stain vertices (pooled: renderers in
// parallel tests never share one).
type stainBatch struct {
	verts []ebiten.Vertex
	idx   []uint32
}

var stainBatches = sync.Pool{New: func() any { return new(stainBatch) }}

// buildStains fills b with the visible stains of w as lit quads (flat,
// sky and lamp from each stain's tile) and returns how many it kept.
func (r *Renderer) buildStains(b *stainBatch, w game.WorldView, sw, sh int, tile float64) int {
	b.verts, b.idx = b.verts[:0], b.idx[:0]
	if len(w.Stains) == 0 || tile <= 0 {
		return 0
	}
	_, rects := stainAtlasImg()
	var q [4]ebiten.Vertex
	n := 0
	for _, s := range w.Stains {
		if s.Amount <= 0 {
			continue
		}
		l := stainLookOf(s)
		sx, sy := r.worldToScreenF(l.cx, l.cy)
		px := l.size * tile
		if rad := float32(px * 0.75); !visible(sx-rad, sy-rad, 2*rad, 2*rad, sw, sh) {
			continue
		}
		rc := rects[l.tex]
		tw, th := float64(rc.Dx()), float64(rc.Dy())
		geo := ebiten.GeoM{}
		geo.Translate(-tw/2, -th/2)
		geo.Scale(px/tw, px/th)
		geo.Rotate(l.rot)
		geo.Translate(float64(sx), float64(sy))
		sky, lamp := r.lightTile(s.X, s.Y)
		litQuad(&q, float32(rc.Min.X), float32(rc.Min.Y), float32(tw), float32(th), geo, l.cs, sky, lamp, 0)
		base := uint32(len(b.verts))
		b.verts = append(b.verts, q[:]...)
		for _, i := range litIndices {
			b.idx = append(b.idx, base+i)
		}
		n++
	}
	return n
}

// drawStains draws the world's blood and ichor stains in one lit batch,
// after the ground cover and before shadows and objects. Off-screen
// stains are skipped.
func (r *Renderer) drawStains(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	if len(w.Stains) == 0 || r.cast != nil {
		return
	}
	b := stainBatches.Get().(*stainBatch)
	defer stainBatches.Put(b)
	if r.buildStains(b, w, sw, sh, tile) == 0 {
		return
	}
	atlas, _ := stainAtlasImg()
	sh2 := r.litShader()
	if sh2 == nil { // shader failed to compile: unlit
		op := &ebiten.DrawTrianglesOptions{Filter: ebiten.FilterLinear, ColorScaleMode: ebiten.ColorScaleModePremultipliedAlpha}
		screen.DrawTriangles32(b.verts, b.idx, atlas, op)
		return
	}
	r.lit.syncUniforms()
	r.lit.opts.Images[0] = atlas
	screen.DrawTrianglesShader32(b.verts, b.idx, sh2, &r.lit.opts)
}

// --- blood on creatures (C2) --------------------------------------------------

// Figure blood numbers.
const (
	bloodCoverMax = 0.75                       // the most a wound alone covers
	bloodFreshAdd = 0.2                        // extra coverage while a hit is fresh
	bloodWetTicks = 2.0 * clock.TicksPerSecond // a hit stays wet 2 s (at 1×)
	bloodWetMax   = 15
)

// encodeBlood packs a figure's stain for lit.kage Custom3:
// 32·ichor + 2·wetStep + coverage, coverage clamped to [0, 1) and
// wetStep to 0..15. No coverage is no blood (0), wet or not.
func encodeBlood(coverage float64, wetStep int, ichor bool) float32 {
	if coverage <= 0 {
		return 0
	}
	w := min(coverage, 0.999) + 2*float64(min(max(wetStep, 0), bloodWetMax))
	if ichor {
		w += 32
	}
	return float32(w)
}

// decodeBlood is lit.kage's decode of Custom3 (its Go twin).
func decodeBlood(w float32) (coverage float64, wetStep int, ichor bool) {
	v := float64(w)
	if v >= 32 {
		ichor, v = true, v-32
	}
	wetStep = int(math.Floor(v / 2))
	return v - 2*float64(wetStep), wetStep, ichor
}

// pawnBloodCoverage is how much of a pawn's body and clothes are
// bloodied: lost health, stronger while a wound is open.
func pawnBloodCoverage(p game.PawnView) float64 {
	k := 0.8
	if p.Wounded {
		k = 1.2
	}
	return min(bloodCoverMax, float64(100-min(max(p.Health, 0), 100))/100*k)
}

// animalBloodCoverage is an animal's: lost health.
func animalBloodCoverage(an game.AnimalView) float64 {
	return min(bloodCoverMax, float64(100-min(max(an.Health, 0), 100))/100)
}

// bloodWet is the wet sheen step and extra coverage of a hit age ticks
// old (ok false: never hit): 15 fading to 0 over bloodWetTicks.
func bloodWet(age float64, ok bool) (wetStep int, add float64) {
	if !ok || age >= bloodWetTicks {
		return 0, 0
	}
	return int(math.Round(bloodWetMax * (1 - max(age, 0)/bloodWetTicks))), bloodFreshAdd
}

// isHiveSpecies reports the hive insects, which bleed ichor.
func isHiveSpecies(key string) bool {
	switch key {
	case "hive_beetle", "hive_brute", "hive_crawler":
		return true
	}
	return false
}

// pawnBlood is the Custom3 value for p's body and apparel this frame (0
// in the shadow pass and for a patient being carried).
func (r *Renderer) pawnBlood(p game.PawnView) float32 {
	if r.cast != nil || p.CarriedBy != 0 {
		return 0
	}
	return r.creatureBlood(pawnBloodCoverage(p), p.ID, p.HitTick, p.X, p.Y, false)
}

// animalBlood is the Custom3 value for an animal's figure this frame.
func (r *Renderer) animalBlood(an game.AnimalView) float32 {
	if r.cast != nil {
		return 0
	}
	return r.creatureBlood(animalBloodCoverage(an), an.ID, an.HitTick, an.X, an.Y, isHiveSpecies(an.Species))
}

func (r *Renderer) creatureBlood(cov float64, id entity.EntityID, hitTick uint64, x, y int, ichor bool) float32 {
	var wet int
	var add float64
	// only a recent hit can still be wet (hitAge scans the traces)
	if hitTick != 0 && r.animT()-float64(hitTick) < 8*bloodWetTicks {
		wet, add = bloodWet(r.hitAge(id, hitTick, x, y))
	}
	return encodeBlood(cov+add, wet, ichor)
}

// layerBlood is the blood a pawn layer carries: bodies and apparel take
// the figure's, hair stays clean.
func layerBlood(key string, blood float32) float32 {
	if strings.HasPrefix(key, "pawn/hair_") {
		return 0
	}
	return blood
}

// bloodHash and bloodNoise are lit.kage's (Go twins in float32): a
// hash-based value noise over atlas pixel coordinates.
func bloodHash(x, y float32) float32 {
	fr := func(v float32) float32 { return v - float32(math.Floor(float64(v))) }
	p0, p1, p2 := fr(x*0.1031), fr(y*0.1031), fr(x*0.1031)
	d := p0*(p1+33.33) + p1*(p2+33.33) + p2*(p0+33.33)
	p0, p1, p2 = p0+d, p1+d, p2+d
	return fr((p0 + p1) * p2)
}

func bloodNoise(x, y float32) float32 {
	ix, iy := float32(math.Floor(float64(x))), float32(math.Floor(float64(y)))
	fx, fy := x-ix, y-iy
	ux, uy := fx*fx*(3-2*fx), fy*fy*(3-2*fy)
	a, b := bloodHash(ix, iy), bloodHash(ix+1, iy)
	c, d := bloodHash(ix, iy+1), bloodHash(ix+1, iy+1)
	top, bot := a+(b-a)*ux, c+(d-c)*ux
	return top + (bot-top)*uy
}

// Blood blotch noise: two octaves (atlas px per noise cell; the plan's 6
// and 2.5 read as speckle at play zooms), stretched about 0.5 so the
// masked share tracks the coverage (raw value noise bunches near 0.5:
// coverage 0.2 painted 3%, 0.75 painted 94%).
const (
	bloodNoiseCoarse  = 10.0
	bloodNoiseFine    = 4.0
	bloodNoiseStretch = 2.2
)

// bloodMask is lit.kage's blotch mask (0..1) at a source pixel for a
// coverage: two octaves of value noise thresholded by the coverage with
// a soft edge, so blotches grow with lost health. The shader multiplies
// it by the sprite's own alpha.
func bloodMask(srcPos [2]float32, coverage float32) float32 {
	if coverage <= 0 {
		return 0
	}
	nz := bloodNoise(srcPos[0]/bloodNoiseCoarse, srcPos[1]/bloodNoiseCoarse)*0.65 +
		bloodNoise(srcPos[0]/bloodNoiseFine, srcPos[1]/bloodNoiseFine)*0.35
	nz = (nz-0.5)*bloodNoiseStretch + 0.5
	t := min(max((nz-(coverage-0.05))/0.1, 0), 1)
	return 1 - t*t*(3-2*t)
}

// --- sprays (C3) -------------------------------------------------------------

// sprayFor is a hit's spray: 4 + damage/2 drops (at most 16), 1.25×
// faster for hits of 10 damage or more.
func sprayFor(dmg int) (n int, speed float64) {
	n = min(4+max(dmg, 0)/2, 16)
	speed = bloodSpeed
	if dmg >= 10 {
		speed *= 1.25
	}
	return n, speed
}

// sprayKind is what a hit target bleeds: ichor for hive insects.
func sprayKind(w game.WorldView, target entity.EntityID) partKind {
	if target != 0 {
		for _, an := range w.Animals {
			if an.ID == target {
				if isHiveSpecies(an.Species) {
					return partIchor
				}
				break
			}
		}
	}
	return partBlood
}
