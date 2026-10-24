package body

// Capacity is a bodily ability derived from the parts (append only).
type Capacity uint8

const (
	Consciousness Capacity = iota
	Moving
	Manipulation
	Sight
	Hearing
	Talking
	Breathing
	NumCaps
)

// Capacity is capacity c in permille (1000: healthy).
func (b *Body) Capacity(c Capacity) int32 {
	return 1000 // TODO(M1)
}

// Pain is the body's pain in permille.
func (b *Body) Pain() int32 {
	return 0 // TODO(M1)
}
