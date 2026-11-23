// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package body

import "testing"

func oneWound(kind WoundKind, dmg int, scar bool) Body {
	return Body{Wounds: []Wound{{Part: Torso, Dmg: dmg, Kind: kind, Permanent: scar}}}
}

// TestPainPerKind: wiki Injury, Pain: 12.5 permille per HP, burn 18.75,
// scar 6.25, whatever the kind of a tended or open wound.
func TestPainPerKind(t *testing.T) {
	cases := []struct {
		name string
		b    Body
		want int32
	}{
		{"cut", oneWound(Cut, 8, false), 100},
		{"bite", oneWound(Bite, 8, false), 100},
		{"scratch", oneWound(Scratch, 8, false), 100},
		{"stab", oneWound(Stab, 8, false), 100},
		{"gunshot", oneWound(Gunshot, 8, false), 100},
		{"bruise", oneWound(Bruise, 8, false), 100},
		{"frostbite", oneWound(Frostbite, 8, false), 100},
		{"surgical", oneWound(Surgical, 8, false), 100},
		{"burn", oneWound(Burn, 8, false), 150},
		{"scar 16", oneWound(Cut, 16, true), 100},
		{"scar 3", oneWound(Cut, 3, true), 18},
		{"burn scar", oneWound(Burn, 16, true), 100},
	}
	for _, c := range cases {
		b := c.b
		if got := b.Pain(); got != c.want {
			t.Errorf("%s: pain %d, want %d", c.name, got, c.want)
		}
	}
}

// TestPainStdUnchanged: bodies with no burn and no scar read as the old
// 12.5 permille formula, at any Scale.
func TestPainStdUnchanged(t *testing.T) {
	for _, scale := range []int32{0, 500, 1300} {
		for _, dmg := range []int{1, 7, 13, 40, 99} {
			b := Body{Scale: scale, Wounds: []Wound{{Part: Torso, Dmg: dmg, Kind: Cut}, {Part: Torso, Dmg: 3, Kind: Bruise, Tended: true}}}
			hurt := int32(dmg + 3)
			if scale > 0 && scale != 1000 {
				hurt = hurt * 1000 / scale
			}
			want := hurt * 125 / 10
			if want > 1000 {
				want = 1000
			}
			if got := b.Pain(); got != want {
				t.Errorf("scale %d dmg %d: pain %d, want %d", scale, dmg, got, want)
			}
		}
	}
}

// TestPainShockConstants lives in the health package (pain_shock_test.go).

// TestPainBurnClampAndPainkiller: burns clamp at 1000, a tended burn hurts
// as an open one, Scale scales the burn bucket, and the painkiller halves
// after the per-kind sum.
func TestPainBurnClampAndPainkiller(t *testing.T) {
	huge := oneWound(Burn, 100, false)
	if got := huge.Pain(); got != 1000 {
		t.Fatalf("burn clamp: %d", got)
	}
	open := oneWound(Burn, 20, false)
	tended := oneWound(Burn, 20, false)
	tended.Wounds[0].Tended = true
	if open.Pain() != 375 || tended.Pain() != 375 {
		t.Fatalf("open %d tended %d, want 375", open.Pain(), tended.Pain())
	}
	big := oneWound(Burn, 20, false)
	big.Scale = 500 // damage against a half-size body doubles
	if got := big.Pain(); got != 750 {
		t.Fatalf("scale 500 burn: %d", got)
	}
	mixed := Body{Wounds: []Wound{{Part: Torso, Dmg: 16, Kind: Burn}, {Part: Torso, Dmg: 8, Kind: Cut}, {Part: Torso, Dmg: 16, Kind: Cut, Permanent: true}}}
	if got := mixed.Pain(); got != 300+100+100 {
		t.Fatalf("mixed: %d", got)
	}
	mixed.Wounds = []Wound{{Part: Torso, Dmg: 32, Kind: Burn}}
	before := mixed.Pain() // 600
	mixed.Numb(100)
	if got := mixed.Pain(); got != before/2 {
		t.Fatalf("painkiller: %d, want %d", got, before/2)
	}
}

// TestPainRateExactTotals pins each rate to the unit: the damage is the
// largest below the 1000 clamp, so a rate off by one changes the total.
func TestPainRateExactTotals(t *testing.T) {
	cases := []struct {
		name string
		b    Body
		want int32
	}{
		{"std 79 HP", oneWound(Cut, 79, false), 987},
		{"burn 53 HP", oneWound(Burn, 53, false), 993},
		{"scar 159 HP", oneWound(Cut, 159, true), 993},
	}
	for _, c := range cases {
		b := c.b
		if got := b.Pain(); got != c.want {
			t.Errorf("%s: pain %d, want %d", c.name, got, c.want)
		}
	}
}
