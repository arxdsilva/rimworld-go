package thermo

import (
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/temperature"
)

// Air exchange and the thermostat band.
const (
	// doorExchange and ventExchange: an open door or vent mixes the air of
	// the places it joins at this many hundredths of a degree-cell per
	// degree of difference per interval, never past their common
	// temperature. The wiki has open vents stronger than doors and gives
	// no figures: original tuning.
	doorExchange = 150
	ventExchange = 300
	// residualLeak (permille per interval): a breached room's air loses
	// about half its difference from the outside in an hour (original
	// tuning, the pre-TEMP rate at the 120-tick step).
	residualLeak = 29
	// BandLow and BandHigh are the thermostat band's edges below and above
	// its target, tenths of a degree: a heater works below target -1 C
	// and idles above target +0.5 C; a cooler works above target +0.5 C
	// and idles below target -1 C. A working device pushes only up to its
	// target, so it holds there instead of overshooting into the band.
	BandLow  = 10
	BandHigh = 5
)

// Step moves every room's air one temperature.EqualiseInterval: the
// thermostats read the air, the walls and roofs equalise with the
// outside, open doors and vents mix, then the sources push their heat.
func (m *Model) Step() {
	m.sync()
	m.ensure()
	h := m.h
	out := h.Outdoor().Milli()
	for _, s := range m.c.sites {
		if s.kind == construction.Heater || s.kind == construction.Cooler {
			m.thermostat(s)
		}
	}
	for r, e := range m.c.exp {
		if r < len(m.fine) {
			m.fine[r] = e.Equalise(m.fine[r], out)
		}
	}
	for _, l := range m.c.links {
		if h.Open(l.id) {
			m.exchange(l, out)
		}
	}
	for _, s := range m.c.sites {
		m.pushSite(s)
	}
	m.pushBodies()
	m.decayResidual(h.Outdoor())
	for r, t := range m.fine {
		m.Temps[r] = t.Temp()
	}
	m.ver++
}

// thermostat updates heater or cooler s's working flag from its room's
// air (hysteresis between BandLow and BandHigh).
func (m *Model) thermostat(s site) {
	if s.room < 0 || s.room >= len(m.Temps) {
		return
	}
	target, active := m.h.Thermostat(s.id)
	t, heats := int(m.Temps[s.room]), s.kind == construction.Heater
	switch {
	case heats && !active && t < target-BandLow, !heats && !active && t > target+BandHigh:
		active = true
	case heats && active && t > target+BandHigh, !heats && active && t < target-BandLow:
		active = false
	}
	m.h.SetThermoActive(s.id, active)
}

// cells is room r's cell count (at least 1).
func (m *Model) cells(r int) int {
	if r < len(m.c.exp) {
		return max(1, m.c.exp[r].Cells)
	}
	return 1
}

// exchange mixes the air across open door or vent l.
func (m *Model) exchange(l link, out temperature.Milli) {
	k := doorExchange
	if l.vent {
		k = ventExchange
	}
	a, b := l.a, l.b
	if a < 0 { // the outdoors on one side: always b's
		a, b = b, a
	}
	if a >= len(m.fine) || b >= len(m.fine) {
		return
	}
	ta, ca := m.fine[a], m.cells(a)
	if b < 0 {
		k = min(k, 100*ca) // never past the outside air
		m.fine[a] = ta + temperature.Milli(int64(out-ta)*int64(k)/int64(100*ca))
		return
	}
	tb, cb := m.fine[b], m.cells(b)
	k = min(k, 100*ca*cb/(ca+cb)) // never past their common air
	q := int64(tb-ta) * int64(k) / 100
	m.fine[a] += temperature.Milli(q / int64(ca))
	m.fine[b] -= temperature.Milli(q / int64(cb))
}

// pushSite applies site s's heat (a cooler's hot side too).
func (m *Model) pushSite(s site) {
	if s.room >= len(m.fine) {
		return
	}
	before := m.fine[s.room]
	p := m.pushOf(s, before)
	if p.rate == 0 {
		return
	}
	m.fine[s.room] = apply(before, p, m.cells(s.room))
	if s.kind == construction.Cooler && s.other >= 0 && s.other < len(m.fine) {
		moved := int64(before-m.fine[s.room]) * int64(m.cells(s.room)) // degree-milli-cells taken out
		m.fine[s.other] += temperature.Milli(moved * coolerHotPct / 100 / int64(m.cells(s.other)))
	}
}

// pushBodies adds the creatures' and fires' heat to their rooms.
func (m *Model) pushBodies() {
	sum := m.roomHeat()
	for r, v := range sum {
		if v != 0 && r < len(m.fine) {
			m.fine[r] = apply(m.fine[r], push{rate: v}, m.cells(r))
		}
	}
}

// roomHeat is the creatures' (air at most bodyMax) and fires' heat per
// room now, hundredths of heat/s, in the reused scratch.
func (m *Model) roomHeat() []int {
	grid, w, gh := m.h.RoomGrid()
	if len(grid) != w*gh {
		grid, w, gh = nil, 0, 0
	}
	if n := len(m.fine); cap(m.sum) < n {
		m.sum = make([]int, n)
	} else {
		m.sum = m.sum[:n]
		clear(m.sum)
	}
	at := func(x, y int) int {
		if x < 0 || y < 0 || x >= w || y >= gh {
			return -1
		}
		return grid[y*w+x]
	}
	m.bodies = m.h.AppendBodies(m.bodies[:0])
	for _, b := range m.bodies {
		if r := at(b.X, b.Y); r >= 0 && r < len(m.sum) && m.fine[r] <= bodyMax {
			m.sum[r] += bodyHeat(b)
		}
	}
	m.fires = m.h.AppendFires(m.fires[:0])
	for _, f := range m.fires {
		if r := at(f.X, f.Y); r >= 0 && r < len(m.sum) {
			m.sum[r] += fireRate * f.Size / 100
		}
	}
	return m.sum
}

// decayResidual fades breached air toward the outside.
func (m *Model) decayResidual(raw temperature.Temp) {
	for i, t := range m.Residual {
		t += (raw - t) * residualLeak / 1000
		if d := t - raw; d < 5 && d > -5 {
			delete(m.Residual, i)
			continue
		}
		m.Residual[i] = t
	}
}

// BuildingRate is building id's heat into its room now, tenths of heat/s
// (a cooler: its cold side, negative; 0 for anything idle or not a heat
// source).
func (m *Model) BuildingRate(id entity.EntityID) int {
	m.ensure()
	for _, s := range m.c.sites {
		if s.id == id && s.room < len(m.fine) {
			return m.pushOf(s, m.fine[s.room]).rate / 10
		}
	}
	return 0
}

// RoomRates is each room's net heat now, tenths of heat/s: its sources
// (a cooler's hot side with its share), creatures and fires.
func (m *Model) RoomRates() []int {
	m.sync()
	m.ensure()
	out := make([]int, len(m.fine))
	copy(out, m.roomHeat())
	for _, s := range m.c.sites {
		if s.room >= len(out) {
			continue
		}
		p := m.pushOf(s, m.fine[s.room]).rate
		out[s.room] += p
		if s.kind == construction.Cooler && s.other >= 0 && s.other < len(out) {
			out[s.other] -= p * coolerHotPct / 100
		}
	}
	for r := range out {
		out[r] /= 10
	}
	return out
}
