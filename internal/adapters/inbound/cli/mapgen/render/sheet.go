package render

import "image"

// Upscale repeats every pixel s×s times (nearest neighbour).
func Upscale(src *image.RGBA, s int) *image.RGBA {
	if s <= 1 {
		return src
	}
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx()*s, b.Dy()*s))
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			off := src.PixOffset(b.Min.X+x, b.Min.Y+y)
			px := src.Pix[off : off+4]
			for dy := 0; dy < s; dy++ {
				o := dst.PixOffset(x*s, y*s+dy)
				for dx := 0; dx < s; dx++ {
					copy(dst.Pix[o+dx*4:o+dx*4+4], px)
				}
			}
		}
	}
	return dst
}

// SheetCols is the contact sheet's tiles per row.
const SheetCols = 4

// SheetGap is the pixel gap between sheet tiles.
const SheetGap = 4

// SheetSize is the pixel size of a sheet of n tiles of tw×th: SheetCols
// per row, SheetGap between tiles, no outer border.
func SheetSize(n, tw, th int) (w, h int) {
	if n <= 0 {
		return 0, 0
	}
	cols := min(n, SheetCols)
	rows := (n + SheetCols - 1) / SheetCols
	return cols*tw + (cols-1)*SheetGap, rows*th + (rows-1)*SheetGap
}

// Sheet lays tiles (all tw×th) out row-major and draws labels[i] (if
// present) in each tile's top-left corner.
func Sheet(tiles []*image.RGBA, labels []uint64, tw, th, labelScale int) *image.RGBA {
	w, h := SheetSize(len(tiles), tw, th)
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	fill(out, 0, 0, w, h, Background)
	for i, t := range tiles {
		x := (i % SheetCols) * (tw + SheetGap)
		y := (i / SheetCols) * (th + SheetGap)
		for row := 0; row < th && row < t.Bounds().Dy(); row++ {
			so := t.PixOffset(0, row)
			do := out.PixOffset(x, y+row)
			n := min(tw, t.Bounds().Dx()) * 4
			copy(out.Pix[do:do+n], t.Pix[so:so+n])
		}
		if i < len(labels) {
			DrawNumber(out, x+2, y+2, labels[i], labelScale)
		}
	}
	return out
}
