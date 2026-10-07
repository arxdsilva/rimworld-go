// Package work is colonist work types: enable flags and 1-4 priorities.
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
	m = m.Set(Research, true)
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

// Priorities are per-work-type priorities: 1 is done first, 4 last, 0
// is off. Within one priority, work types go in Order.
type Priorities [NumTypes]uint8

// Lowest is the lowest (last-done) priority.
const Lowest = 4

// DefaultPriority is what a newly enabled work type gets.
const DefaultPriority = 3

// DefaultPriorities enables the default colonist work at priority 3.
func DefaultPriorities() Priorities {
	var p Priorities
	m := DefaultColonist()
	for t := Type(0); t < NumTypes; t++ {
		if m.Enabled(t) {
			p[t] = DefaultPriority
		}
	}
	return p
}

// Mask is the enabled set (priority > 0).
func (p Priorities) Mask() Mask {
	var m Mask
	for t := Type(0); t < NumTypes; t++ {
		m = m.Set(t, p[t] > 0)
	}
	return m
}

// Set returns a copy with t at priority v (clamped to 0..Lowest).
func (p Priorities) Set(t Type, v int) Priorities {
	if t >= NumTypes {
		return p
	}
	p[t] = uint8(max(0, min(Lowest, v)))
	return p
}

// Order is the tie-break among work types sharing a priority.
var Order = []Type{Doctor, Haul, Construct, Cook, Craft, Hunt, Grow, Mine, Research}
