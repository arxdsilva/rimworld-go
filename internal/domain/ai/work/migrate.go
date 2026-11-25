// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package work

// LegacyMask widens pre-v20 masks while preserving disabled work choices.
func LegacyMask(version int, old uint16, prev Mask, hasPrevious bool) Mask {
	// Historical v1 writers included both 11- and 12-column masks.
	// All genuine legacy columns have stable bit positions; do not
	// discard an enabled column merely because the header predates it.
	m := Mask(old & uint16(1<<Patient-1))
	if hasPrevious && version < 7 {
		m = m.Set(PlantCut, prev.Enabled(PlantCut))
	}
	craft := m.Enabled(Craft)
	for _, t := range [...]Type{Patient, BedRest, Basic} {
		m = m.Set(t, true)
	}
	return m.Set(Smith, craft).Set(Tailor, craft).Set(Art, craft)
}

// LegacyPriorities widens one historical row. Explicit preferences take
// precedence over enables; a mask-only writer retains its native defaults.
func LegacyPriorities(version int, old uint16, prev Mask, had bool, p Priorities, explicit bool) Priorities {
	if !explicit {
		m := LegacyMask(version, old, prev, had)
		if had && prev.Enabled(Clean) {
			m = m.Set(Clean, true)
		}
		for t := Type(0); t < Patient; t++ {
			if m.Enabled(t) {
				p[t] = DefaultPriority
			}
		}
	}
	for _, t := range [...]Type{Patient, BedRest, Basic} {
		p[t] = DefaultPriority
	}
	p[Smith], p[Tailor], p[Art] = p[Craft], p[Craft], p[Craft]
	return p
}
