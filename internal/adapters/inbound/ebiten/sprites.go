package ebiten

import ebiten "github.com/hajimehoshi/ebiten/v2"

// tileHash is the stable per-tile hash behind variant picks.
func tileHash(x, y int) uint64 {
	return uint64(uint32(x))*0x9E3779B97F4A7C15 ^ uint64(uint32(y))*0xBF58476D1CE4E5B9
}

func drawImage(dst, src *ebiten.Image, x, y, w, h float64, flipX bool) {
	if src == nil {
		return
	}
	sw, sh := src.Bounds().Dx(), src.Bounds().Dy()
	if sw < 1 || sh < 1 {
		return
	}
	op := &ebiten.DrawImageOptions{}
	sx := w / float64(sw)
	sy := h / float64(sh)
	if flipX {
		op.GeoM.Scale(-sx, sy)
		op.GeoM.Translate(w, 0)
	} else {
		op.GeoM.Scale(sx, sy)
	}
	op.GeoM.Translate(x, y)
	op.Filter = ebiten.FilterLinear
	dst.DrawImage(src, op)
}
