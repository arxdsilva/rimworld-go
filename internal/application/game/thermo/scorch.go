package thermo

import "rimworld-go/internal/domain/world/temperature"

// Spontaneous ignition (TEMP-6, wiki: Temperature, Fire: flammable items,
// floors and structures catch fire at about 235 C).
const (
	// IgniteAt is the air at which flammables may catch fire on their own.
	IgniteAt = temperature.Temp(2350)
	// igniteMin and igniteMax: the chance (ppm per flammable cell per
	// interval) at IgniteAt, rising straight to igniteMax at igniteTop
	// (original tuning: the wiki gives no rate; a hot storeroom of a dozen
	// stacks catches within a minute or so).
	igniteMin = 5_000
	igniteMax = 200_000
	igniteTop = temperature.Temp(10_000)
	// IgniteCooldown is the least time between two spontaneous ignitions
	// on the map (original tuning: once one fire is lit, it spreads by the
	// fire rules; the air does not keep lighting new ones every step).
	IgniteCooldown = 250
)

// IgniteChance is the chance (ppm per flammable cell per interval) that
// air t lights a flammable cell (0 below IgniteAt).
func IgniteChance(t temperature.Temp) int {
	if t < IgniteAt {
		return 0
	}
	t = min(t, igniteTop)
	return igniteMin + (igniteMax-igniteMin)*int(t-IgniteAt)/int(igniteTop-IgniteAt)
}

// Hottest is the hottest air of any room or breached cell now: nothing
// can scorch while it is below the thresholds.
func (m *Model) Hottest() temperature.Temp {
	hot := temperature.Temp(-2730)
	for _, t := range m.Temps {
		hot = max(hot, t)
	}
	for _, t := range m.Residual {
		hot = max(hot, t)
	}
	return hot
}

// HotRooms appends every room whose air is at least t.
func (m *Model) HotRooms(dst []int, t temperature.Temp) []int {
	for r, v := range m.Temps {
		if v >= t {
			dst = append(dst, r)
		}
	}
	return dst
}

// EachCell calls fn for room r's cells in cell order until it returns
// false.
func (m *Model) EachCell(r int, fn func(i int) bool) {
	m.ensure()
	if r < 0 || r+1 >= len(m.c.start) {
		return
	}
	for _, i := range m.c.cells[m.c.start[r]:m.c.start[r+1]] {
		if !fn(int(i)) {
			return
		}
	}
}

// MayIgnite reports the ignition cooldown over at tick now.
func (m *Model) MayIgnite(now uint64) bool { return now >= m.igniteAt }

// Ignited starts the ignition cooldown at tick now.
func (m *Model) Ignited(now uint64) { m.igniteAt = now + IgniteCooldown }
