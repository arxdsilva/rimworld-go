package quest

import (
	"testing"

	"rimworld-go/internal/domain/sim/clock"
)

func TestDefs_Windows(t *testing.T) {
	hour := uint64(clock.TicksPerHour)
	cases := []struct {
		k                 KindID
		auto              bool
		accept, duration  uint64
		pay               Pay
		delivery          Delivery
		joins, violent    bool
		choice, gwItems   bool
		giverFail, target bool
	}{
		{Joiner, false, 7 * hour, 0, OnAccept, Home, true, true, false, false, true, false},
		{OreLump, true, 0, 30 * day, OnSuccess, AtSite, false, true, false, false, false, false},
		{Stash, true, 0, 16 * day, OnSuccess, AtSite, false, true, false, false, false, false},
		{Refugee, true, 0, 10 * day, OnSuccess, Home, true, true, false, false, false, false},
		{Camp, false, 6 * day, 17 * day, OnSuccess, Home, false, true, true, true, true, true},
		{Rescue, true, 0, 23 * day, OnSuccess, Home, true, true, false, false, false, true},
		{Request, false, 276000, 16 * day, OnSuccess, IntoCaravan, false, false, true, false, true, false},
		{Talks, true, 0, 0, OnSuccess, IntoCaravan, false, false, false, false, false, false},
	}
	for _, c := range cases {
		d := DefOf(c.k)
		got := []any{d.Kind, d.AutoAccept, d.AcceptTicks, d.DurationTicks, d.RewardAt, d.Delivery,
			d.Joins, d.Violent, d.Choice, d.GoodwillPlusItems, d.FailOnGiverHostile, d.HasTarget}
		want := []any{c.k, c.auto, c.accept, c.duration, c.pay, c.delivery,
			c.joins, c.violent, c.choice, c.gwItems, c.giverFail, c.target}
		for i := range got {
			if got[i] != want[i] {
				t.Errorf("kind %d field %d = %v, want %v", c.k, i, got[i], want[i])
			}
		}
		if c.auto != (d.AcceptTicks == 0) {
			t.Errorf("kind %d: AutoAccept and AcceptTicks disagree", c.k)
		}
	}
}

func TestDefs_Weights(t *testing.T) {
	for k := KindID(1); k < NumKinds; k++ {
		w := DefOf(k).Weight
		if k == OreLump {
			if w != 0 {
				t.Fatalf("ore lump weight %d: only the scanner offers it", w)
			}
			continue
		}
		if w <= 0 {
			t.Fatalf("kind %d weight %d, want > 0", k, w)
		}
	}
}

func TestDefs_FreshAndUnknown(t *testing.T) {
	a := Defs()
	if len(a) != int(NumKinds) || a[KindNone] != (Def{}) {
		t.Fatalf("Defs len %d, none %+v", len(a), a[KindNone])
	}
	a[Camp].Weight = -1
	if Defs()[Camp].Weight == -1 {
		t.Fatal("Defs shares state between calls")
	}
	if DefOf(NumKinds) != (Def{}) || DefOf(255) != (Def{}) {
		t.Fatal("an unknown kind must give the zero Def")
	}
}
