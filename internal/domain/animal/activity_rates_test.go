// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package animal

import "testing"

func TestAnimalRateSpanAndNextDecision(t *testing.T) {
	for _, tc := range []struct {
		now               uint64
		per, period, span int
		want              int
	}{
		{now: 0, per: 18, period: 6, span: 5, want: 0},
		{now: 3, per: 12, period: 6, span: 2, want: 4},
		{now: 8, per: 12, period: 6, span: 2, want: 4},
	} {
		if got := RateSpan(tc.now, tc.per, tc.period, tc.span); got != tc.want {
			t.Fatalf("rate span %+v: got %d", tc, got)
		}
	}
	for id := uint64(1); id <= 20; id++ {
		for now := uint64(0); now < 300; now += 23 {
			next := NextDecision(id, now)
			if next <= now || (next+id)%DecideEvery != 0 {
				t.Fatalf("id %d now %d next %d", id, now, next)
			}
		}
	}
}
