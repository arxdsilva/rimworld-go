// Port of spritefusion-sprite-normals (MIT, (c) 2026 Hugo Duprez), https://github.com/Hugo-Dz/spritefusion-sprite-normals @ d14b067. See THIRD_PARTY_NOTICES.md.

package spritenormals

import "math"

// linearLUT holds every 8-bit sRGB channel value linearised once.
var linearLUT = func() (t [256]float32) {
	for v := range t {
		c := float64(float32(v) / 255)
		if c <= 0.04045 {
			t[v] = float32(c / 12.92)
		} else {
			t[v] = float32(math.Pow((c+0.055)/1.055, 2.4))
		}
	}
	return t
}()

func cbrt32(v float32) float32 { return float32(math.Cbrt(float64(v))) }

// lightness is the Oklab L of an sRGB colour. Products are wrapped in
// float32 conversions so the compiler never fuses them (Rust does not).
func lightness(r8, g8, b8 uint8) float32 {
	r, g, b := linearLUT[r8], linearLUT[g8], linearLUT[b8]
	l := cbrt32(float32(0.41222146*r) + float32(0.53633255*g) + float32(0.051445995*b))
	m := cbrt32(float32(0.2119035*r) + float32(0.6806995*g) + float32(0.10739696*b))
	s := cbrt32(float32(0.08830246*r) + float32(0.28171885*g) + float32(0.6299787*b))
	return float32(0.21045426*l) + float32(0.7936178*m) - float32(0.004072047*s)
}
