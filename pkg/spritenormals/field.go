// Port of spritefusion-sprite-normals (MIT, (c) 2026 Hugo Duprez), https://github.com/Hugo-Dz/spritefusion-sprite-normals @ d14b067. See THIRD_PARTY_NOTICES.md.

package spritenormals

import "math"

// field is a row-major scalar image.
type field struct {
	width, height int
	data          []float32
}

func newField(width, height int) field {
	return field{width: width, height: height, data: make([]float32, width*height)}
}

func (f field) clone() field {
	return field{width: f.width, height: f.height, data: append([]float32(nil), f.data...)}
}

// gaussian is a separable Gaussian, radius ceil(3 sigma), zero-extended
// and normalised; below sigma 0.05 it is the identity (a copy).
func gaussian(input field, sigma float32) field {
	if sigma < 0.05 {
		return input.clone()
	}
	radius := int(math.Ceil(float64(sigma * 3)))
	kernel := make([]float32, 0, 2*radius+1)
	for i := -radius; i <= radius; i++ {
		t := float32(i) / sigma
		kernel = append(kernel, float32(math.Exp(float64(-0.5*(t*t)))))
	}
	var total float32
	for _, v := range kernel {
		total += v
	}
	for i := range kernel {
		kernel[i] /= total
	}
	w, h := input.width, input.height
	horizontal := newField(w, h)
	for y := 0; y < h; y++ {
		out, src := horizontal.data[y*w:(y+1)*w], input.data[y*w:(y+1)*w]
		for k, weight := range kernel {
			offset := k - radius
			skip := min(max(offset, -offset), w)
			if offset < 0 {
				addScaled(out[skip:], src[:w-skip], weight)
			} else {
				addScaled(out[:w-skip], src[skip:], weight)
			}
		}
	}
	result := newField(w, h)
	for y := 0; y < h; y++ {
		out := result.data[y*w : (y+1)*w]
		for k, weight := range kernel {
			if row := y + k - radius; row >= 0 && row < h {
				addScaled(out, horizontal.data[row*w:(row+1)*w], weight)
			}
		}
	}
	return result
}

// addScaled is out += src*weight; the inner float32 conversion forbids
// the fused multiply-add that Rust does not use.
func addScaled(out, src []float32, weight float32) {
	src = src[:len(out)]
	for i, s := range src {
		out[i] += float32(s * weight)
	}
}

// maskedGaussian is the opacity-weighted normalised convolution
// G(v*alpha)/max(G(alpha), 1e-8); hidden RGB never contributes.
func maskedGaussian(input, weight field, sigma float32) field {
	weighted := input.clone()
	for i, a := range weight.data {
		weighted.data[i] *= a
	}
	result := gaussian(weighted, sigma)
	den := gaussian(weight, sigma)
	for i, d := range den.data {
		result.data[i] /= max(d, 1e-8)
	}
	return result
}

// sobel is the normalised Sobel correlation at a pixel away from the
// border: dx grows with slopes rising to the right, dy downward.
func sobel(input field, index int) (dx, dy float32) {
	d := input.data
	up, down := index-input.width, index+input.width
	dx = (d[up+1] + float32(2*d[index+1]) + d[down+1] - d[up-1] - float32(2*d[index-1]) - d[down-1]) * 0.125
	dy = (d[down-1] + float32(2*d[down]) + d[down+1] - d[up-1] - float32(2*d[up]) - d[up+1]) * 0.125
	return dx, dy
}
