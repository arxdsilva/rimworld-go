package ebiten

import (
	"image"
	"image/color"
	"math"
	"sync"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

// Tracked-in dirt on screen (SIM-021 / ART §16.3): boot prints on a lightly
// dirty tile, a growing smudge once it is dirtier. Procedural textures from
// one atlas, drawn lit in one batch after the floors and before blood.

// Dirt texture sizes (px) and variant counts. Every texture is one tile
// square, its marks inside the inscribed circle so a rotated quad stays in
// its tile.
const (
	dirtPx       = 96
	dirtPad      = 2
	dirtVariants = 4 // per print count, and smudges
	dirtMaxPrint = 3 // most boot prints on one tile
	dirtPrintN   = dirtMaxPrint * dirtVariants
	dirtTexN     = dirtPrintN + dirtVariants
)

// dirtSmudgeFrom is the amount at which a tile's boot prints give way to a
// smudge (ART §16.3).
const dirtSmudgeFrom = 0.25

// dirtAlpha is the opacity of a tile's dirt: 0.15 + 0.6·amount.
func dirtAlpha(amount float64) float32 { return float32(0.15 + 0.6*clamp01(amount)) }

// Dirt colours, light to heavy (0..1).
var (
	dirtLight = rgbHex(0x6b5238)
	dirtHeavy = rgbHex(0x4a3826)
)

// dirtColour is the soil brown of a tile at an amount.
func dirtColour(amount float64) [3]float32 {
	return lerp3(dirtLight, dirtHeavy, float32(clamp01(amount)))
}

// dirtPrintCount is how many boot prints a lightly dirty tile shows, 1..3,
// growing with the amount; 0 means the tile shows a smudge instead.
func dirtPrintCount(amount float64) int {
	if amount >= dirtSmudgeFrom {
		return 0
	}
	return min(1+int(max(amount, 0)*12), dirtMaxPrint)
}

// dirtTexIndex is the atlas slot of a print group (n prints, a variant)
// or, for n 0, a smudge variant: prints 0..11, smudges 12..15.
func dirtTexIndex(n, variant int) int {
	v := ((variant % dirtVariants) + dirtVariants) % dirtVariants
	if n <= 0 {
		return dirtPrintN + v
	}
	return (min(n, dirtMaxPrint)-1)*dirtVariants + v
}

// dirtLook is how one dirty tile is drawn: its texture, centre (world
// tiles), size (tiles), rotation and colour scale (straight alpha).
type dirtLook struct {
	tex    int
	cx, cy float64
	size   float64
	rot    float64
	cs     colorScale
}

// dirtLookOf places and colours a dirty tile: prints (size 1) or a smudge
// growing from 0.55 to 1 tile, turned by a hash of the tile (so the same
// tile always looks the same) and nudged up to ±0.08 tile off its centre.
func dirtLookOf(d game.DirtView) dirtLook {
	th := tileHash(d.X, d.Y)
	n := dirtPrintCount(d.Amount)
	size := 1.0
	if n == 0 {
		size = 0.55 + 0.45*clamp01((d.Amount-dirtSmudgeFrom)/(1-dirtSmudgeFrom))
	}
	c := dirtColour(d.Amount)
	return dirtLook{
		tex:  dirtTexIndex(n, int(th>>9)),
		cx:   float64(d.X) + 0.5 + 0.16*(hash01(th, 0xd1)-0.5),
		cy:   float64(d.Y) + 0.5 + 0.16*(hash01(th, 0xd2)-0.5),
		size: size,
		rot:  2 * math.Pi * hash01(th, 0xd3),
		cs:   colorScale{c[0], c[1], c[2], dirtAlpha(d.Amount)},
	}
}

// --- procedural textures ----------------------------------------------------

// dirtLocal turns the offset (dx, dy) by -rot: the point in a shape's own
// frame.
func dirtLocal(dx, dy, rot float64) (u, v float64) {
	s, c := math.Sincos(-rot)
	return dx*c - dy*s, dx*s + dy*c
}

// dirtEllipse is the antialiased coverage (0..1) of the point (u, v) by an
// ellipse centred (cx, cy) with semi-axes rx, ry, in its own frame.
func dirtEllipse(u, v, cx, cy, rx, ry float64) float64 {
	d := math.Hypot((u-cx)/rx, (v-cy)/ry) // 1 on the rim
	return clamp01((1-d)*math.Min(rx, ry) + 0.5)
}

// dirtPixels is one dirt texture: white with alpha, deterministic per
// slot, transparent at its border.
func dirtPixels(tex int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, dirtPx, dirtPx))
	seed := uint64(tex) + 0xd17
	h := func(k ...uint64) float64 { return hash01(append([]uint64{0xd1a7, seed}, k...)...) }
	c := float64(dirtPx) / 2
	set := func(x, y int, cover, grain float64) {
		if cover <= 0 {
			return
		}
		img.SetNRGBA(x, y, color.NRGBA{255, 255, 255, uint8(math.Round(255 * clamp01(cover*grain)))})
	}
	if tex >= dirtPrintN { // a smudge: a soft blot, satellites and a scuff streak
		type blot struct{ x, y, rx, ry, rot float64 }
		blots := []blot{{c, c, 0.27 * dirtPx * (0.9 + 0.2*h(1)), 0.25 * dirtPx * (0.9 + 0.2*h(2)), 2 * math.Pi * h(3)}}
		for i := 0; i < 3+int(h(4)*3); i++ { // 3..5 satellites
			a, d := 2*math.Pi*h(5, uint64(i)), (0.16+0.1*h(6, uint64(i)))*dirtPx
			r := (0.05 + 0.07*h(7, uint64(i))) * dirtPx
			blots = append(blots, blot{c + d*math.Cos(a), c + d*math.Sin(a), r, r * (0.7 + 0.5*h(8, uint64(i))), 2 * math.Pi * h(9, uint64(i))})
		}
		sa := 2 * math.Pi * h(10) // a scuff streak dragged through the middle
		blots = append(blots, blot{c, c, 0.4 * dirtPx, 0.035 * dirtPx, sa})
		for y := 1; y < dirtPx-1; y++ {
			for x := 1; x < dirtPx-1; x++ {
				fx, fy := float64(x)+0.5, float64(y)+0.5
				cover := 0.0
				for i, b := range blots {
					k := 1.0
					if i == len(blots)-1 {
						k = 0.55 // the streak is fainter
					}
					u, v := dirtLocal(fx-b.x, fy-b.y, b.rot)
					cover = math.Max(cover, k*dirtEllipse(u, v, 0, 0, b.rx, b.ry))
				}
				// soft, mottled edge: the dirt thins towards the rim
				fall := 1 - smoothstep01(math.Hypot(fx-c, fy-c)/(0.48*dirtPx))
				grain := 0.62 + 0.38*valueNoise(fx/6, fy/6, seed^0x5e)
				set(x, y, cover*(0.55+0.45*fall), grain)
			}
		}
		return img
	}
	n := tex/dirtVariants + 1 // boot prints in the group
	v := tex % dirtVariants
	flip := 1.0
	if v%2 == 1 {
		flip = -1 // which foot comes first
	}
	spin := (h(20) - 0.5) * 0.5 // the group's heading
	type print struct{ x, y, rot float64 }
	var prints []print
	for i := 0; i < n; i++ {
		along := (float64(i) - float64(n-1)/2) * 0.22 * dirtPx
		side := flip * float64(2*(i%2)-1) * 0.05 * dirtPx
		px := side + (h(21, uint64(i))-0.5)*0.02*dirtPx
		py := along + (h(22, uint64(i))-0.5)*0.03*dirtPx - 0.03*dirtPx
		// rotate the group about the centre
		s, co := math.Sincos(spin)
		prints = append(prints, print{c + px*co - py*s, c + px*s + py*co, spin + (h(23, uint64(i))-0.5)*0.3})
	}
	for y := 1; y < dirtPx-1; y++ {
		for x := 1; x < dirtPx-1; x++ {
			fx, fy := float64(x)+0.5, float64(y)+0.5
			cover, tread := 0.0, 1.0
			for _, p := range prints {
				// sole: toe end towards -y; heel: a smaller pad behind it
				u, v := dirtLocal(fx-p.x, fy-p.y, p.rot)
				sole := dirtEllipse(u, v, 0, 0, 0.066*dirtPx, 0.1*dirtPx)
				heel := dirtEllipse(u, v, 0, 0.15*dirtPx, 0.052*dirtPx, 0.04*dirtPx)
				if cv := math.Max(sole, heel); cv > cover {
					cover, tread = cv, 1
					if sole > heel { // cross-ribs on the sole
						tread = 0.78 + 0.22*(0.5+0.5*math.Cos(v*1.9))
					}
				}
			}
			grain := 0.8 + 0.2*valueNoise(fx/5, fy/5, seed^0x5f)
			set(x, y, cover, grain*tread)
		}
	}
	return img
}

// dirtAtlas is every dirt texture on one image (one texture page, so all
// dirt draws in one batch), built once.
var dirtAtlas struct {
	once  sync.Once
	img   *ebiten.Image
	rects [dirtTexN]image.Rectangle
}

// dirtAtlasImg builds the atlas on first use: one row, slot i at
// i·(dirtPx+dirtPad).
func dirtAtlasImg() (*ebiten.Image, *[dirtTexN]image.Rectangle) {
	dirtAtlas.once.Do(func() {
		src := image.NewNRGBA(image.Rect(0, 0, dirtTexN*(dirtPx+dirtPad), dirtPx))
		for i := 0; i < dirtTexN; i++ {
			p := dirtPixels(i)
			r := p.Bounds().Add(image.Pt(i*(dirtPx+dirtPad), 0))
			for row := 0; row < dirtPx; row++ {
				copy(src.Pix[src.PixOffset(r.Min.X, r.Min.Y+row):], p.Pix[p.PixOffset(0, row):p.PixOffset(0, row+1)])
			}
			dirtAtlas.rects[i] = r
		}
		dirtAtlas.img = ebiten.NewImageFromImage(src)
	})
	return dirtAtlas.img, &dirtAtlas.rects
}

var dirtBatches = sync.Pool{New: func() any { return new(stainBatch) }}

// buildDirt fills b with the visible dirty tiles of w as lit quads (flat,
// sky and lamp from each tile) and returns how many it kept.
func (r *Renderer) buildDirt(b *stainBatch, w game.WorldView, sw, sh int, tile float64) int {
	b.verts, b.idx = b.verts[:0], b.idx[:0]
	if len(w.Dirt) == 0 || tile <= 0 {
		return 0
	}
	_, rects := dirtAtlasImg()
	var q [4]ebiten.Vertex
	n := 0
	for _, d := range w.Dirt {
		if d.Amount <= 0 {
			continue
		}
		// whole tiles off screen are skipped before any hashing
		tx, ty := r.worldToScreenF(float64(d.X), float64(d.Y))
		if !visible(tx-float32(tile)*0.25, ty-float32(tile)*0.25, float32(tile)*1.5, float32(tile)*1.5, sw, sh) {
			continue
		}
		l := dirtLookOf(d)
		sx, sy := r.worldToScreenF(l.cx, l.cy)
		px := l.size * tile
		rc := rects[l.tex]
		tw, th := float64(rc.Dx()), float64(rc.Dy())
		geo := ebiten.GeoM{}
		geo.Translate(-tw/2, -th/2)
		geo.Scale(px/tw, px/th)
		geo.Rotate(l.rot)
		geo.Translate(float64(sx), float64(sy))
		sky, lamp := r.lightTile(d.X, d.Y)
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

// drawDirt draws the tracked-in dirt on the ground in one lit batch, after
// the floors and before the blood stains. Off-screen tiles are skipped.
func (r *Renderer) drawDirt(screen *ebiten.Image, w game.WorldView, sw, sh int, tile float64) {
	if len(w.Dirt) == 0 || r.cast != nil {
		return
	}
	b := dirtBatches.Get().(*stainBatch)
	defer dirtBatches.Put(b)
	if r.buildDirt(b, w, sw, sh, tile) == 0 {
		return
	}
	atlas, _ := dirtAtlasImg()
	sh2 := r.litShader()
	if sh2 == nil { // shader failed to compile: unlit
		op := &ebiten.DrawTrianglesOptions{Filter: ebiten.FilterLinear, ColorScaleMode: ebiten.ColorScaleModePremultipliedAlpha}
		screen.DrawTriangles32(b.verts, b.idx, atlas, op)
		return
	}
	r.lit.SyncUniforms()
	r.lit.Opts.Images[0] = atlas
	screen.DrawTrianglesShader32(b.verts, b.idx, sh2, &r.lit.Opts)
}
