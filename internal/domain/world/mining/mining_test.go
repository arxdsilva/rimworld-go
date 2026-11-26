// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package mining

import "testing"

func TestPublishedMiningFactors(t *testing.T) {
	for lv, want := range []int{600, 700, 800, 850, 900, 925, 950, 975, 1000, 1010, 1020, 1030, 1040, 1050, 1060, 1070, 1080, 1090, 1100, 1120, 1130} {
		if got := Yield(lv, 1000, 1000); got != want {
			t.Fatalf("level %d yield %d want %d", lv, got, want)
		}
	}
	for _, c := range []struct {
		s            int32
		speed, yield int
	}{{0, 500, 800}, {500, 750, 900}, {1000, 1000, 1000}, {1500, 1000, 1000}} {
		if SightSpeed(c.s) != c.speed || Yield(8, 1000, c.s) != c.yield {
			t.Fatalf("sight %+v", c)
		}
	}
	if Yield(8, 500, 1000) != 850 || Yield(0, 0, 0) != 336 || Yield(20, 1500, 1500) != 1130 {
		t.Fatal("manipulation/caps")
	}
	if Speed(0) != 40 || Speed(8) != 1000 || Speed(20) != 2440 || Amount(40, 975) != 39 || Amount(2, 600) != 1 {
		t.Fatal("speed/rounding")
	}
}
