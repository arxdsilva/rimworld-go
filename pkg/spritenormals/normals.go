// Port of spritefusion-sprite-normals (MIT, (c) 2026 Hugo Duprez), https://github.com/Hugo-Dz/spritefusion-sprite-normals @ d14b067. See THIRD_PARTY_NOTICES.md.

// Package spritenormals makes normal maps for pixel-art sprites: each
// opaque island of a sprite becomes a soft bevel dome (from an exact
// distance transform of its silhouette) plus surface detail (from its
// Oklab lightness), and the height field's Sobel slopes become normals.
// It is a faithful Go port of spritefusion-sprite-normals by Hugo Duprez.
package spritenormals

import (
	"fmt"
	"image"
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

// Validate reports a setting outside the ranges spritefusion accepts.
func (s Settings) Validate() error {
	return nil // ART-010 W2: the ranges of settings.rs
}

// Generate returns the normal map of src: same size, same alpha; fully
// transparent pixels are the neutral (128, 128, 255).
func Generate(src *image.NRGBA, s Settings) (*image.NRGBA, error) {
	if err := s.Validate(); err != nil {
		return nil, fmt.Errorf("spritenormals: %w", err)
	}
	// ART-010 W2: the relief pipeline. Stub: a flat map.
	b := src.Bounds()
	out := image.NewNRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			i := out.PixOffset(x, y)
			out.Pix[i], out.Pix[i+1], out.Pix[i+2], out.Pix[i+3] = 128, 128, 255, src.Pix[src.PixOffset(x, y)+3]
		}
	}
	return out, nil
}
