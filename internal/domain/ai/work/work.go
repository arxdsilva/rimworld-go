// Package work is colonist work-type enable flags (not 1–4 priorities).
package work

// Type is a labor column. Only Haul, Construct, and Grow are wired.
type Type uint8

const (
	Firefight Type = iota
	Doctor
	Grow
	Cook
	Construct
	Hunt
	Haul
	Clean
	Research
	Mine
	Craft
	NumTypes
)

// Mask is a bitset of enabled work types.
type Mask uint16

// DefaultColonist enables Haul, Construct, and Grow.
func DefaultColonist() Mask {
	var m Mask
	m = m.Set(Haul, true)
	m = m.Set(Construct, true)
	m = m.Set(Grow, true)
	m = m.Set(Mine, true)
	m = m.Set(Cook, true)
	m = m.Set(Hunt, true)
	m = m.Set(Craft, true)
	m = m.Set(Doctor, true)
	return m
}

// Enabled reports whether t is on.
func (m Mask) Enabled(t Type) bool {
	return m&(1<<t) != 0
}

// Set returns a copy with t enabled or disabled.
func (m Mask) Set(t Type, on bool) Mask {
	bit := Mask(1 << t)
	if on {
		return m | bit
	}
	return m &^ bit
}
