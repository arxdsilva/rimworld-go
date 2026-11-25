// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package planet

import (
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/world/temperature"
	"testing"
)

func TestCalendarTravelClimate(t *testing.T) {
	p := Build(Params{SeedText: "calendar", Coverage: 5, Temp: 2, Rain: 2})
	q := p.Fork()
	q.Params.Calendar = clock.CalendarPolicy{DaysPerQuadrum: 20}
	n := 0
	for i := range p.Center {
		if !p.Land(i) {
			continue
		}
		a, b := p.climate(i), q.climate(i)
		if b.Calendar != q.Params.Calendar || a.PeakDay*4/3 != b.PeakDay {
			t.Fatal("site phase")
		}
		if a.DailyMean(clock.Tick(a.PeakDay*clock.TicksPerDay)) != b.DailyMean(clock.Tick(b.PeakDay*clock.TicksPerDay)) {
			t.Fatal("peak")
		}
		if p.MoveDifficulty(i, a.PeakDay) != q.MoveDifficulty(i, b.PeakDay) || q.MoveDifficulty(i, 3) != q.MoveDifficulty(i, 83) {
			t.Fatal("travel season")
		}
		for _, c := range []temperature.Climate{a, b} {
			count := 0
			for d := 0; d < c.Calendar.YearDays(); d++ {
				if c.DailyMean(clock.Tick(d*clock.TicksPerDay)) > 30 {
					count++
				}
			}
			if count != c.GrowingDays(30) {
				t.Fatalf("growing days=%d, want %d", c.GrowingDays(30), count)
			}
		}
		n++
	}
	if n == 0 {
		t.Fatal("no land")
	}
}
