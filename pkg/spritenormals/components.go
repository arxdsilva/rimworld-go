// Port of spritefusion-sprite-normals (MIT, (c) 2026 Hugo Duprez), https://github.com/Hugo-Dz/spritefusion-sprite-normals @ d14b067. See THIRD_PARTY_NOTICES.md.

package spritenormals

// component is one 8-connected island of opaque pixels.
type component struct {
	pixels                 []int
	minX, minY, maxX, maxY int
}

// components finds eight-connected islands, which keep diagonal
// pixel-art strokes whole.
func components(mask []bool, width, height int) []component {
	visited := make([]bool, len(mask))
	var result []component
	for start := range mask {
		if !mask[start] || visited[start] {
			continue
		}
		part := component{pixels: []int{start}, minX: start % width, minY: start / width, maxX: start % width, maxY: start / width}
		visited[start] = true
		for cursor := 0; cursor < len(part.pixels); cursor++ {
			index := part.pixels[cursor]
			x, y := index%width, index/width
			part.minX, part.maxX = min(part.minX, x), max(part.maxX, x)
			part.minY, part.maxY = min(part.minY, y), max(part.maxY, y)
			for ny := max(y-1, 0); ny <= min(y+1, height-1); ny++ {
				for nx := max(x-1, 0); nx <= min(x+1, width-1); nx++ {
					n := ny*width + nx
					if mask[n] && !visited[n] {
						visited[n] = true
						part.pixels = append(part.pixels, n)
					}
				}
			}
		}
		result = append(result, part)
	}
	return result
}
