// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package clock

// CalendarPolicy selects the number of days in a quadrum. Zero retains the
// original 15-day calendar. The longest admitted quadrum is bounded by the
// representable tick length on every supported architecture.
type CalendarPolicy struct {
	DaysPerQuadrum int
	Revision       int
}

func (p CalendarPolicy) QuadrumDays() int {
	if p.DaysPerQuadrum < DaysPerQuadrum {
		return DaysPerQuadrum
	}
	return min(p.DaysPerQuadrum, int((1<<31-1)/TicksPerDay/QuadrumsPerYear))
}
func (p CalendarPolicy) YearDays() int { return p.QuadrumDays() * QuadrumsPerYear }
func Policy(ps ...CalendarPolicy) CalendarPolicy {
	if len(ps) == 0 {
		return CalendarPolicy{}
	}
	p := ps[0]
	if p.DaysPerQuadrum < DaysPerQuadrum {
		p.DaysPerQuadrum = 0
	} else {
		p.DaysPerQuadrum = p.QuadrumDays()
	}
	return p
}

// BiologicalDays maps elapsed calendar days into the original 60-day year.
// Whole-year decomposition avoids multiplication by an unbounded tick age.
func (p CalendarPolicy) BiologicalDays(days uint64) uint64 {
	y := uint64(p.YearDays())
	return days/y*DaysPerYear + (days%y)*DaysPerYear/y
}

// BiologicalDayDue reports the once-daily crossing of a biological day.
func (p CalendarPolicy) BiologicalDayDue(now uint64) bool {
	d := now / TicksPerDay
	return d == 0 || p.BiologicalDays(d) != p.BiologicalDays(d-1)
}

// CalendarDays rounds up the elapsed calendar days needed for a biological age.
func (p CalendarPolicy) CalendarDays(days uint64) uint64 {
	y := uint64(p.YearDays())
	return days/DaysPerYear*y + ((days%DaysPerYear)*y+DaysPerYear-1)/DaysPerYear
}
