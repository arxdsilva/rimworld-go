// Package zone is stockpile rectangles with a food+wood filter.
package zone

// Stockpile is a painted storage rectangle. Filters are food and wood only.
type Stockpile struct {
	MinX, MinY, MaxX, MaxY int
	AllowFood              bool
	AllowWood              bool
	AllowStone             bool
}

// Normalize orders the corners.
func Normalize(x1, y1, x2, y2 int) (minX, minY, maxX, maxY int) {
	minX, maxX = x1, x2
	if maxX < minX {
		minX, maxX = maxX, minX
	}
	minY, maxY = y1, y2
	if maxY < minY {
		minY, maxY = maxY, minY
	}
	return
}

// Contains reports whether (x,y) is inside s.
func (s Stockpile) Contains(x, y int) bool {
	return x >= s.MinX && x <= s.MaxX && y >= s.MinY && y <= s.MaxY
}
