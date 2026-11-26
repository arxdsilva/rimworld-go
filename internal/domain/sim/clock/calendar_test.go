// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package clock

import "testing"

func TestCalendarRationalBoundaries(t *testing.T) {
	for _, q := range []int{0, 15, 16, 20, 60} {
		p := CalendarPolicy{DaysPerQuadrum: q}
		n := 0
		for d := 1; d <= p.YearDays(); d++ {
			if p.BiologicalDayDue(uint64(d) * TicksPerDay) {
				n++
			}
		}
		if n != DaysPerYear {
			t.Fatalf("q=%d age rolls=%d", q, n)
		}
		if d := DateAtPolicy(Tick(p.YearDays()*TicksPerDay), p); d.Year != 1 || d.Quadrum != 0 || d.Day != 0 {
			t.Fatalf("q=%d date=%v", q, d)
		}
		for a := uint64(1); a < 180; a++ {
			d := p.CalendarDays(a)
			if p.BiologicalDays(d) != a || p.BiologicalDays(d-1) >= a {
				t.Fatalf("q=%d age=%d day=%d", q, a, d)
			}
		}
	}
}
