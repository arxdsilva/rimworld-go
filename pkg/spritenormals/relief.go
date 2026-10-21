// Port of spritefusion-sprite-normals (MIT, (c) 2026 Hugo Duprez), https://github.com/Hugo-Dz/spritefusion-sprite-normals @ d14b067. See THIRD_PARTY_NOTICES.md.

package spritenormals

import (
	"image"
	"math"
)

func fma32(a, b, c float32) float32 {
	return float32(math.FMA(float64(a), float64(b), float64(c)))
}

func sqrt32(v float32) float32 { return float32(math.Sqrt(float64(v))) }

// padding is the margin that keeps every opaque pixel's 3x3 neighbourhood
// inside the field after the Gaussians.
func (s Settings) padding() int {
	return int(math.Ceil(float64(3*max(s.ShapeSoftness, s.DetailScale*2)))) + 3
}

// relief renders the normal map of one island image (pixels outside the
// island are fully transparent).
func relief(rgba *image.NRGBA, s Settings) *image.NRGBA {
	pad := s.padding()
	w, h := rgba.Rect.Dx(), rgba.Rect.Dy()
	width, height := w+2*pad, h+2*pad
	opacity, light := newField(width, height), newField(width, height)
	mask := make([]bool, width*height)
	low, high := float32(1), float32(0)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			p := rgba.Pix[y*rgba.Stride+x*4:]
			i := (y+pad)*width + x + pad
			alpha := float32(p[3]) / 255
			if alpha <= s.AlphaThreshold {
				continue
			}
			mask[i] = true
			opacity.data[i] = alpha
			v := lightness(p[0], p[1], p[2])
			light.data[i] = v
			low, high = min(low, v), max(high, v)
		}
	}
	// A constant height offset has no normal; centring avoids
	// cancellation noise on flat art.
	for i := range light.data {
		if mask[i] {
			light.data[i] -= (low + high) * 0.5
		}
	}
	shape := edt(mask, width, height)
	reach := s.ShapeRadius
	for i, d := range shape.data {
		inset := max(d-0.5, 0)
		shape.data[i] = float32(s.ShapeStrength*reach) * float32(math.Log1p(float64(inset/reach)))
	}
	combined := gaussian(shape, s.ShapeSoftness)
	fine := maskedGaussian(light, opacity, s.DetailScale)
	broad := maskedGaussian(light, opacity, s.DetailScale*2)
	for i := range combined.data {
		combined.data[i] += s.DetailStrength * (float32(0.7*fine.data[i]) + float32(0.3*broad.data[i]))
	}
	out := neutralMap(rgba)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := (y+pad)*width + x + pad
			if !mask[i] {
				continue
			}
			dx, dy := sobel(combined, i)
			n := slopesToNormal(dx, dy, s)
			o := out.Pix[y*out.Stride+x*4:]
			o[0], o[1], o[2] = encode(n[0]), encode(n[1]), encode(n[2])
		}
	}
	return out
}

func encode(v float32) uint8 {
	return uint8(math.Round(float64(float32(v*0.5+0.5) * 255)))
}

func slopesToNormal(dx, dyImage float32, s Settings) [3]float32 {
	x := -dx * s.NormalStrength
	y := dyImage * s.NormalStrength
	if s.DirectX {
		y = -y
	}
	if s.InvertX {
		x = -x
	}
	if s.InvertY {
		y = -y
	}
	length := sqrt32(fma32(x, x, fma32(y, y, 1)))
	return [3]float32{x / length, y / length, 1 / length}
}

// neutralMap is (128, 128, 255) with src's alpha everywhere.
func neutralMap(src *image.NRGBA) *image.NRGBA {
	w, h := src.Rect.Dx(), src.Rect.Dy()
	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			o := out.Pix[y*out.Stride+x*4:]
			o[0], o[1], o[2], o[3] = 128, 128, 255, src.Pix[y*src.Stride+x*4+3]
		}
	}
	return out
}
