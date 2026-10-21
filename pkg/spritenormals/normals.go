// Port of spritefusion-sprite-normals (MIT, (c) 2026 Hugo Duprez), https://github.com/Hugo-Dz/spritefusion-sprite-normals @ d14b067. See THIRD_PARTY_NOTICES.md.

// Package spritenormals makes normal maps for pixel-art sprites: each
// opaque island of a sprite becomes a soft bevel dome (from an exact
// distance transform of its silhouette) plus surface detail (from its
// Oklab lightness), and the height field's Sobel slopes become normals.
// It is a faithful Go port of spritefusion-sprite-normals by Hugo Duprez.
package spritenormals

import (
	"errors"
	"fmt"
	"image"
	"math"
	"strconv"
)

// Settings mirrors spritefusion-sprite-normals NormalSettings; the
// defaults are identical.
type Settings struct {
	AlphaThreshold, ShapeRadius, ShapeStrength, ShapeSoftness float32
	DetailStrength, DetailScale, NormalStrength               float32
	// DirectX flips the green channel (y down); the default is OpenGL
	// (y up). InvertX and InvertY flip the slopes.
	DirectX, InvertX, InvertY bool
}

// DefaultSettings is spritefusion's default: alpha threshold 0.03, shape
// radius 4, strength 1.15, softness 1.15, detail strength 6, detail scale
// 0.6, normal strength 1.5, OpenGL format.
func DefaultSettings() Settings {
	return Settings{AlphaThreshold: 0.03, ShapeRadius: 4, ShapeStrength: 1.15, ShapeSoftness: 1.15,
		DetailStrength: 6, DetailScale: 0.6, NormalStrength: 1.5}
}

// maxPixels is the per-island processing limit, padding included.
const maxPixels = 4194304

// Validate reports a setting outside the ranges spritefusion accepts.
func (s Settings) Validate() error {
	for _, c := range []struct {
		name          string
		value, lo, hi float32
	}{
		{"alpha-threshold", s.AlphaThreshold, 0, 0.99},
		{"shape-radius", s.ShapeRadius, 0.5, 512},
		{"shape-strength", s.ShapeStrength, 0, 8},
		{"shape-softness", s.ShapeSoftness, 0, 8},
		{"detail-strength", s.DetailStrength, 0, 8},
		{"detail-scale", s.DetailScale, 0.6, 8},
		{"normal-strength", s.NormalStrength, 0, 8},
	} {
		v := float64(c.value)
		if math.IsNaN(v) || math.IsInf(v, 0) || c.value < c.lo || c.value > c.hi {
			return fmt.Errorf("invalid setting: %s must be finite and between %s and %s", c.name,
				strconv.FormatFloat(float64(c.lo), 'g', -1, 32), strconv.FormatFloat(float64(c.hi), 'g', -1, 32))
		}
	}
	return nil
}

// Generate returns the normal map of src: same size, same alpha; fully
// transparent pixels are the neutral (128, 128, 255). Disconnected
// silhouettes are filtered independently.
func Generate(src *image.NRGBA, s Settings) (*image.NRGBA, error) {
	if err := s.Validate(); err != nil {
		return nil, fmt.Errorf("spritenormals: %w", err)
	}
	w, h := src.Rect.Dx(), src.Rect.Dy()
	pad := s.padding()
	if w <= 0 || h <= 0 || (w+2*pad)*(h+2*pad) > maxPixels {
		return nil, errors.New("spritenormals: image is empty or exceeds the 4194304-pixel processing limit")
	}
	out := neutralMap(src)
	mask := make([]bool, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			mask[y*w+x] = float32(src.Pix[y*src.Stride+x*4+3])/255 > s.AlphaThreshold
		}
	}
	for _, part := range components(mask, w, h) {
		iw, ih := part.maxX-part.minX+1, part.maxY-part.minY+1
		island := image.NewNRGBA(image.Rect(0, 0, iw, ih))
		for _, idx := range part.pixels {
			sx, sy := idx%w, idx/w
			copy(island.Pix[(sy-part.minY)*island.Stride+(sx-part.minX)*4:][:4], src.Pix[sy*src.Stride+sx*4:][:4])
		}
		gen := relief(island, s)
		for _, idx := range part.pixels {
			sx, sy := idx%w, idx/w
			copy(out.Pix[sy*out.Stride+sx*4:][:4], gen.Pix[(sy-part.minY)*gen.Stride+(sx-part.minX)*4:][:4])
		}
	}
	out.Rect = src.Rect
	return out, nil
}
