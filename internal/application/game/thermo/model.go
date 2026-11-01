package thermo

import (
	"slices"

	"rimworld-go/internal/domain/world/room"
	"rimworld-go/internal/domain/world/temperature"
)

// Model is the rooms' air. Temps and Residual are the state the rest of
// the game reads (and saves); fine is Temps at working precision.
type Model struct {
	// Temps is each room's air, by room id, tenths of a degree C.
	Temps []temperature.Temp
	// Residual is the air of cells a breached room left behind, by cell
	// index: it fades into the outside over a few hours.
	Residual map[int]temperature.Temp

	h    Host
	fine []temperature.Milli
	// ver moves whenever some air may have changed (WorldView.TempVer;
	// saved, so a loaded game reads the same); dirty: a rebuild touched
	// the residual air.
	ver   uint64
	dirty bool
	gen   uint64 // room rebuilds so far (cache key)
	c     cache
	// igniteAt is the tick the spontaneous ignition cooldown ends
	// (scorch.go; saved).
	igniteAt uint64
	// step scratch: the creatures and fires, heat per room
	bodies []Body
	fires  []Fire
	sum    []int
}

// New is an empty model over host h.
func New(h Host) *Model { return &Model{h: h} }

// Ver moves whenever any cell's air may have changed.
func (m *Model) Ver() uint64 { return m.ver }

// sync takes Temps written from outside (fixtures, a load) as the truth:
// a room whose tenths no longer match its fine value restarts from them.
func (m *Model) sync() {
	if len(m.fine) != len(m.Temps) {
		m.fine = make([]temperature.Milli, len(m.Temps))
		for i, t := range m.Temps {
			m.fine[i] = t.Milli()
		}
		return
	}
	for i, t := range m.Temps {
		if m.fine[i].Temp() != t {
			m.fine[i] = t.Milli()
		}
	}
}

// set writes room r's air at working precision and its tenths.
func (m *Model) set(r int, t temperature.Milli) {
	m.fine[r], m.Temps[r] = t, t.Temp()
}

// cellAir is what cell i of the old grid held: its old room's air, the
// residual it was left with, or else the outside air.
func (m *Model) cellAir(old []int, i int, outside temperature.Milli) temperature.Milli {
	o := room.Outdoors
	if i < len(old) {
		o = old[i]
	}
	switch t, ok := m.Residual[i]; {
	case o >= 0 && o < len(m.fine):
		return m.fine[o]
	case ok:
		return t.Milli()
	}
	return outside
}

// Remap carries the air across a full room rebuild from old to grid (n
// rooms): each new room starts at the average of what its cells held,
// cells that left a room for the outdoors keep their air as residual.
// With no old grid (the first build) every room starts at indoor.
func (m *Model) Remap(old, grid []int, n int, outside, indoor temperature.Temp) {
	m.sync()
	m.gen++
	fine := make([]temperature.Milli, n)
	if old == nil {
		for i := range fine {
			fine[i] = indoor.Milli()
		}
		m.adopt(fine)
		return
	}
	if m.Residual == nil {
		m.Residual = map[int]temperature.Temp{}
	}
	sum, cnt := make([]int, n), make([]int, n)
	out := outside.Milli()
	for i, r := range grid {
		if r < 0 {
			if o := atOr(old, i); r == room.Outdoors && o >= 0 && o < len(m.Temps) {
				m.residual(i, m.Temps[o]) // a breached room keeps its air for a while
			}
			continue
		}
		sum[r] += int(m.cellAir(old, i, out))
		cnt[r]++
		m.unresidual(i)
	}
	for r := range fine {
		if cnt[r] > 0 {
			fine[r] = temperature.Milli(sum[r] / cnt[r])
		}
	}
	m.adopt(fine)
}

// RemapChanged is Remap for an incremental rebuild: a room with exactly
// an old room's cells keeps its air; a changed room averages its cells
// as Remap does. Cells of unchanged rooms hold no residual (it is only
// kept for outdoor cells and dropped when a cell joins a room), so the
// result is identical to Remap's.
func (m *Model) RemapChanged(rm Changes, old, grid []int, n int, outside temperature.Temp) {
	m.sync()
	m.gen++
	if m.Residual == nil {
		m.Residual = map[int]temperature.Temp{}
	}
	rm.EachChanged(func(i int) {
		if o := old[i]; grid[i] == room.Outdoors && o >= 0 && o < len(m.Temps) {
			m.residual(i, m.Temps[o])
		}
	})
	fine := make([]temperature.Milli, n)
	out := outside.Milli()
	for r := range fine {
		if s := rm.Same(r); s >= 0 && s < len(m.fine) {
			fine[r] = m.fine[s]
			continue
		}
		sum, cnt := 0, 0
		rm.EachCell(r, func(i int) {
			sum += int(m.cellAir(old, i, out))
			cnt++
			m.unresidual(i)
		})
		if cnt > 0 {
			fine[r] = temperature.Milli(sum / cnt)
		}
	}
	m.adopt(fine)
}

// adopt makes fine the rooms' air. Ver moves only when some air did:
// a rebuild that changes nothing (a loaded game's first) reads the same.
func (m *Model) adopt(fine []temperature.Milli) {
	if !slices.Equal(fine, m.fine) {
		m.dirty = true
	}
	if m.dirty {
		m.ver++
		m.dirty = false
	}
	m.fine = fine
	m.Temps = make([]temperature.Temp, len(fine))
	for i, t := range fine {
		m.Temps[i] = t.Temp()
	}
}

// residual keeps t as cell i's breached air.
func (m *Model) residual(i int, t temperature.Temp) {
	if old, ok := m.Residual[i]; !ok || old != t {
		m.Residual[i], m.dirty = t, true
	}
}

// unresidual drops cell i's breached air (it joined a room).
func (m *Model) unresidual(i int) {
	if _, ok := m.Residual[i]; ok {
		delete(m.Residual, i)
		m.dirty = true
	}
}

func atOr(s []int, i int) int {
	if i < len(s) {
		return s[i]
	}
	return room.Outdoors
}

// Save is the model's state on disk beside RoomTemps and Residual: the
// air at working precision and the view version.
type Save struct {
	Fine []int32
	Ver  uint64
	// IgniteAt is the ignition cooldown's end (TEMP-6; 0 in older saves).
	IgniteAt uint64
}

// Export is the model's Save.
func (m *Model) Export() Save {
	m.sync()
	out := make([]int32, len(m.fine))
	for i, t := range m.fine {
		out[i] = int32(t)
	}
	return Save{Fine: out, Ver: m.ver, IgniteAt: m.igniteAt}
}

// Import restores the saved air: temps and residual as saved, fine from
// the save when it matches temps (an older save: from temps).
func (m *Model) Import(temps []temperature.Temp, residual map[int]temperature.Temp, s Save) {
	m.Temps, m.Residual, m.fine, m.ver, m.igniteAt = temps, residual, nil, s.Ver, s.IgniteAt
	if len(s.Fine) == len(temps) {
		m.fine = make([]temperature.Milli, len(s.Fine))
		for i, t := range s.Fine {
			m.fine[i] = temperature.Milli(t)
		}
	}
	m.sync()
	m.c = cache{}
}
