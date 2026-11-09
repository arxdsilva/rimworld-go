// Package pixeldiff compares two rendered frames so a render change can
// prove it is byte-identical (batching, reordering) or within a tolerance
// (an anti-aliasing swap) before it lands. It depends on the standard
// library only.
package pixeldiff

import (
	"fmt"
	"image"
	"image/color"
)

// Report is the outcome of comparing frame A with frame B. A pixel's delta
// is the largest per-channel |A-B| at that pixel.
type Report struct {
	Width, Height int
	SizeMismatch  bool
	Differing     int // pixels whose delta is above zero
	EdgePixels    int
	MaxChannel    [4]int // per-channel max |delta| (R, G, B, A)
	Max           int    // max delta over all pixels
	NonEdgeMax    int
	EdgeMax       int
	NonEdgeMean   float64 // mean delta over non-edge pixels
	EdgeMean      float64 // mean delta over edge pixels
	Bounds        image.Rectangle
}

// Policy is the tolerance a Report must meet.
type Policy struct {
	Name       string
	Exact      bool
	NonEdgeMax int
	EdgeMax    int
	EdgeMean   float64
}

// Exact requires byte-equal frames.
var Exact = Policy{Name: "exact", Exact: true}

// AA allows an anti-aliasing swap: off-edge pixels move by at most 1/255,
// edge pixels by at most 32/255 with a mean of at most 6/255.
var AA = Policy{Name: "aa", NonEdgeMax: 1, EdgeMax: 32, EdgeMean: 6}

// ByName returns the policy called "exact" or "aa".
func ByName(name string) (Policy, bool) {
	switch name {
	case "exact":
		return Exact, true
	case "aa":
		return AA, true
	}
	return Policy{}, false
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// Compare measures how far b is from a. edge reports whether pixel (x, y)
// is an edge pixel; nil treats every pixel as non-edge.
func Compare(a, b *image.RGBA, edge func(x, y int) bool) Report {
	r := Report{Width: a.Bounds().Dx(), Height: a.Bounds().Dy()}
	if a.Bounds().Size() != b.Bounds().Size() {
		r.SizeMismatch = true
		return r
	}
	var edgeSum, nonSum, nonEdgeN int
	for y := 0; y < r.Height; y++ {
		ra := a.Pix[y*a.Stride:]
		rb := b.Pix[y*b.Stride:]
		for x := 0; x < r.Width; x++ {
			d := 0
			for c := 0; c < 4; c++ {
				v := abs(int(ra[x*4+c]) - int(rb[x*4+c]))
				if v > r.MaxChannel[c] {
					r.MaxChannel[c] = v
				}
				if v > d {
					d = v
				}
			}
			isEdge := edge != nil && edge(x, y)
			if isEdge {
				r.EdgePixels++
				edgeSum += d
				if d > r.EdgeMax {
					r.EdgeMax = d
				}
			} else {
				nonEdgeN++
				nonSum += d
				if d > r.NonEdgeMax {
					r.NonEdgeMax = d
				}
			}
			if d > 0 {
				r.Differing++
				if d > r.Max {
					r.Max = d
				}
				p := image.Rect(x, y, x+1, y+1)
				if r.Bounds.Empty() {
					r.Bounds = p
				} else {
					r.Bounds = r.Bounds.Union(p)
				}
			}
		}
	}
	if r.EdgePixels > 0 {
		r.EdgeMean = float64(edgeSum) / float64(r.EdgePixels)
	}
	if nonEdgeN > 0 {
		r.NonEdgeMean = float64(nonSum) / float64(nonEdgeN)
	}
	return r
}

// Pass reports whether r meets the policy, with a one-line reason.
func (r Report) Pass(p Policy) (bool, string) {
	switch {
	case r.SizeMismatch:
		return false, "frame sizes differ"
	case p.Exact:
		if r.Differing != 0 {
			return false, fmt.Sprintf("%d pixels differ (max %d) within %v", r.Differing, r.Max, r.Bounds)
		}
		return true, "byte-identical"
	case r.NonEdgeMax > p.NonEdgeMax:
		return false, fmt.Sprintf("non-edge max %d > %d", r.NonEdgeMax, p.NonEdgeMax)
	case r.EdgeMax > p.EdgeMax:
		return false, fmt.Sprintf("edge max %d > %d", r.EdgeMax, p.EdgeMax)
	case r.EdgeMean > p.EdgeMean:
		return false, fmt.Sprintf("edge mean %.2f > %.2f", r.EdgeMean, p.EdgeMean)
	}
	return true, fmt.Sprintf("within %s: %d pixels differ, max %d", p.Name, r.Differing, r.Max)
}

func luma(p []byte) int { return (299*int(p[0]) + 587*int(p[1]) + 114*int(p[2])) / 1000 }

// EdgeMask marks the pixels near an anti-aliased edge. A pixel is a seed when
// in img or any of others its alpha or luma differs from any of its eight
// neighbours by at least threshold, or when its alpha is partial (0 < a <
// 255). The mask is the seeds grown by one pixel in every direction. The
// result is an edge predicate for Compare; pass both frames so an edge that
// exists in only one of them still counts.
func EdgeMask(img *image.RGBA, threshold int, others ...*image.RGBA) func(x, y int) bool {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	seed := make([]bool, w*h)
	for _, im := range append([]*image.RGBA{img}, others...) {
		markSeeds(im, w, h, threshold, seed)
	}
	mask := make([]bool, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if !seed[y*w+x] {
				continue
			}
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					if nx, ny := x+dx, y+dy; nx >= 0 && ny >= 0 && nx < w && ny < h {
						mask[ny*w+nx] = true
					}
				}
			}
		}
	}
	return func(x, y int) bool { return x >= 0 && y >= 0 && x < w && y < h && mask[y*w+x] }
}

func markSeeds(img *image.RGBA, w, h, threshold int, seed []bool) {
	at := func(x, y int) []byte { o := y*img.Stride + x*4; return img.Pix[o : o+4] }
	bw, bh := min(w, img.Bounds().Dx()), min(h, img.Bounds().Dy())
	for y := 0; y < bh; y++ {
		for x := 0; x < bw; x++ {
			p := at(x, y)
			if p[3] > 0 && p[3] < 255 {
				seed[y*w+x] = true
				continue
			}
			l := luma(p)
			for dy := -1; dy <= 1 && !seed[y*w+x]; dy++ {
				for dx := -1; dx <= 1; dx++ {
					nx, ny := x+dx, y+dy
					if (dx == 0 && dy == 0) || nx < 0 || ny < 0 || nx >= bw || ny >= bh {
						continue
					}
					q := at(nx, ny)
					if abs(int(p[3])-int(q[3])) >= threshold || abs(l-luma(q)) >= threshold {
						seed[y*w+x] = true
						break
					}
				}
			}
		}
	}
}

// HeatMap returns |A-B| per channel amplified 8x (alpha forced opaque),
// for review. Differing pixels show up bright on black.
func HeatMap(a, b *image.RGBA) *image.RGBA {
	w, h := min(a.Bounds().Dx(), b.Bounds().Dx()), min(a.Bounds().Dy(), b.Bounds().Dy())
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			oa, ob := y*a.Stride+x*4, y*b.Stride+x*4
			d := 0
			var c [3]int
			for i := 0; i < 3; i++ {
				c[i] = abs(int(a.Pix[oa+i])-int(b.Pix[ob+i])) * 8
				d = max(d, c[i])
			}
			d = max(d, abs(int(a.Pix[oa+3])-int(b.Pix[ob+3]))*8)
			if d > 0 && d < 64 { // keep a 1/255 difference visible
				d = 64
			}
			v := min(d, 255)
			out.SetRGBA(x, y, color.RGBA{uint8(v), uint8(v), uint8(v), 255})
		}
	}
	return out
}

// SideBySide places a left and b right with a 2px magenta gutter.
func SideBySide(a, b *image.RGBA) *image.RGBA {
	h := max(a.Bounds().Dy(), b.Bounds().Dy())
	aw, bw := a.Bounds().Dx(), b.Bounds().Dx()
	out := image.NewRGBA(image.Rect(0, 0, aw+2+bw, h))
	for y := 0; y < h; y++ {
		for x := aw; x < aw+2; x++ {
			out.SetRGBA(x, y, color.RGBA{255, 0, 255, 255})
		}
		if y < a.Bounds().Dy() {
			copy(out.Pix[y*out.Stride:y*out.Stride+aw*4], a.Pix[y*a.Stride:y*a.Stride+aw*4])
		}
		if y < b.Bounds().Dy() {
			o := y*out.Stride + (aw+2)*4
			copy(out.Pix[o:o+bw*4], b.Pix[y*b.Stride:y*b.Stride+bw*4])
		}
	}
	return out
}
