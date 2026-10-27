package power

import (
	"cmp"
	"slices"
)

// Dev is one device of a net for Balance. Want is the W a consumer would
// draw now (its Draw while working, Idle otherwise; 0 while switched
// off), Out the W a producer makes now (after fuel, daylight, wind or a
// flare). Stored and Cap are a battery's energy and capacity in W·ticks.
// ShedUntil is the tick a shed consumer stays off until (0: not shed; a
// past tick: still shed, waiting for headroom). Balance sets Powered (a
// consumer ran; false for every other role), Stored and ShedUntil. IDs
// are unique.
type Dev struct {
	ID          uint64
	Role        Role
	Class       Class
	Want, Out   int
	Stored, Cap int64
	ShedUntil   uint64
	Powered     bool
}

// NetOut is one net's balance: W generated and W drawn by the consumers
// that ran, the batteries' energy and capacity (W·ticks) after it, and
// whether consumers are shed (newly or still held off).
type NetOut struct {
	Gen, Load   int
	Stored, Cap int64
	Brownout    bool
}

// StoredWh is the batteries' energy in whole Wh.
func (o NetOut) StoredWh() int { return int(o.Stored / WTicksPerWh) }

// CapWh is the batteries' capacity in whole Wh.
func (o NetOut) CapWh() int { return int(o.Cap / WTicksPerWh) }

// Balance integrates one net over dt ticks ending at tick now: generation
// covers the load, a surplus charges the batteries (with a loss) and a
// deficit draws on them; past that, consumers are shed by Class, the
// newest id first within a class, for a minimum time. It updates devs in
// place and returns the net's totals.
//
// In order: each battery leaks SelfDischargeW over dt. The consumers that
// want power and are not shed run if their load fits in generation plus
// the batteries' energy spread over the span (dt, at least one tick);
// otherwise they are shed one by one (lamps, benches, coolers, heaters;
// the newest first) until it fits, each off until now+ShedTicks. When
// nothing had to be shed, consumers whose hold is over come back, the
// most needed first (heaters first, the oldest first within a class),
// each only if the net carries the load with it plus HeadroomPct, the
// batteries' energy counted over ShedTicks (or dt, if longer) so a nearly
// empty battery brings nothing back. Then a surplus charges the
// batteries at ChargePct and a deficit drains them, split equally (the
// remainder to the lowest ids, a full or empty battery's share to the
// others). A span of 0 moves no energy. The result does not depend on
// the order of devs. Devices on no net (Graph.Net 0) are not balanced:
// the caller marks them unpowered.
func Balance(devs []Dev, dt int, now uint64) NetOut {
	dt = max(dt, 0)
	span := int64(max(dt, 1))
	var out NetOut
	var gen int64
	var bbuf [16]int
	bats := bbuf[:0]
	for i := range devs {
		d := &devs[i]
		d.Powered = false
		switch d.Role {
		case Producer:
			gen += int64(max(d.Out, 0))
		case Storage:
			c := max(d.Cap, 0)
			d.Stored = min(max(d.Stored, 0), c)
			d.Stored -= min(d.Stored, SelfDischargeW*int64(dt))
			out.Stored += d.Stored
			out.Cap += c
			bats = append(bats, i)
		}
	}
	out.Gen = int(gen)

	var load int64
	waiting := 0
	for i := range devs {
		d := &devs[i]
		switch {
		case d.Role != Consumer:
		case d.Want <= 0: // switched off: draws nothing; a finished hold is dropped
			if d.ShedUntil != 0 && now >= d.ShedUntil {
				d.ShedUntil = 0
			}
		case d.ShedUntil == 0:
			d.Powered = true
			load += int64(d.Want)
		case now >= d.ShedUntil:
			waiting++
		}
	}
	var cbuf [64]int
	if avail := gen + out.Stored/span; load > avail {
		order := consumers(devs, cbuf[:0], func(d *Dev) bool { return d.Powered })
		for _, i := range order {
			if load <= avail {
				break
			}
			d := &devs[i]
			d.Powered, d.ShedUntil = false, now+ShedTicks
			load -= int64(d.Want)
		}
	} else if waiting > 0 {
		back := gen + out.Stored/max(span, ShedTicks)
		order := consumers(devs, cbuf[:0], func(d *Dev) bool { return !d.Powered && d.Want > 0 && d.ShedUntil != 0 && now >= d.ShedUntil })
		for j := len(order) - 1; j >= 0; j-- {
			d := &devs[order[j]]
			if (load+int64(d.Want))*(100+HeadroomPct) <= back*100 {
				d.Powered, d.ShedUntil = true, 0
				load += int64(d.Want)
			}
		}
	}
	for i := range devs {
		if d := &devs[i]; d.Role == Consumer && d.Want > 0 && d.ShedUntil != 0 {
			out.Brownout = true
			break
		}
	}
	out.Load = int(load)

	if dt == 0 || len(bats) == 0 || gen == load {
		return out
	}
	slices.SortFunc(bats, func(a, b int) int { return cmp.Compare(devs[a].ID, devs[b].ID) })
	if gen > load {
		out.Stored += spread(devs, bats, (gen-load)*int64(dt)*ChargePct/100, true)
	} else {
		out.Stored -= spread(devs, bats, (load-gen)*int64(dt), false)
	}
	return out
}

// consumers lists the consumers keep accepts in shed order: lamps,
// benches, coolers, heaters, defences (PLAY-024); the newest (largest id)
// first in a class.
func consumers(devs []Dev, buf []int, keep func(*Dev) bool) []int {
	for i := range devs {
		if devs[i].Role == Consumer && keep(&devs[i]) {
			buf = append(buf, i)
		}
	}
	slices.SortFunc(buf, func(a, b int) int {
		da, db := &devs[a], &devs[b]
		if c := cmp.Compare(da.Class, db.Class); c != 0 {
			return c
		}
		return cmp.Compare(db.ID, da.ID)
	})
	return buf
}

// spread charges (or drains) e W·ticks into (out of) the batteries bats,
// sorted by id: an equal share each, the remainder one each to the lowest
// ids, and what a full (empty) one cannot take passed on to the others. It
// returns what moved.
func spread(devs []Dev, bats []int, e int64, charge bool) int64 {
	room := func(d *Dev) int64 {
		if charge {
			return max(d.Cap, 0) - d.Stored
		}
		return d.Stored
	}
	moved := int64(0)
	for e > 0 {
		n := int64(0)
		for _, i := range bats {
			if room(&devs[i]) > 0 {
				n++
			}
		}
		if n == 0 {
			break
		}
		share, rem, k := e/n, e%n, int64(0)
		for _, i := range bats {
			d := &devs[i]
			r := room(d)
			if r <= 0 {
				continue
			}
			amt := share
			if k < rem {
				amt++
			}
			k++
			amt = min(amt, r)
			if charge {
				d.Stored += amt
			} else {
				d.Stored -= amt
			}
			e -= amt
			moved += amt
		}
	}
	return moved
}
