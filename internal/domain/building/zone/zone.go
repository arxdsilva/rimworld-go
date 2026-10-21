// Package zone is the per-tile zone map (stockpiles, grow zones, pens) and
// the legacy zone rectangles older saves hold.
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

// GrowZone is a painted growing area sown with one crop species.
type GrowZone struct {
	MinX, MinY, MaxX, MaxY int
	Crop                   uint8 // plant.Species
}

// Contains reports whether (x,y) is inside z.
func (z GrowZone) Contains(x, y int) bool {
	return x >= z.MinX && x <= z.MaxX && y >= z.MinY && y <= z.MaxY
}

// Contains reports whether (x,y) is inside s.
func (s Stockpile) Contains(x, y int) bool {
	return x >= s.MinX && x <= s.MaxX && y >= s.MinY && y <= s.MaxY
}
