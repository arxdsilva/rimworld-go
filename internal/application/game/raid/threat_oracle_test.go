// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package raid

import (
	"testing"

	"rimworld-go/internal/domain/incident"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/world/biome"
)

// The frozen reference: the die and its fallback ring as they were before
// the book drew the threats. Literal days, no definition reads.
type frozenThreat uint8

const (
	fzRaid frozenThreat = iota
	fzPack
	fzManhunters
	fzHive
	fzMad
)

func frozenPick(roll, day int, winter bool) frozenThreat {
	switch {
	case winter && roll < 4:
		return fzPack
	case !winter && roll < 2 && day >= 8:
		return fzManhunters
	case roll == 7 && day >= 12:
		return fzHive
	case roll == 6 && day >= 6:
		return fzMad
	}
	return fzRaid
}

func frozenAllowed(roll, day int, winter bool, ok func(frozenThreat) bool) (frozenThreat, bool) {
	k := frozenPick(roll, day, winter)
	if ok(k) {
		return k, true
	}
	alts := [...]frozenThreat{fzPack, fzManhunters, fzHive}
	for i := range alts {
		a := alts[(roll+i)%len(alts)]
		if a == fzHive && day < 12 {
			continue
		}
		if ok(a) {
			return a, true
		}
	}
	return 0, false
}

var frozenID = map[frozenThreat]incident.ID{
	fzRaid: incident.Raid, fzPack: incident.PredatorPack, fzManhunters: incident.ManhunterPack,
	fzHive: incident.HiveSwarm, fzMad: incident.MadAnimal,
}

// TestThreatPickMatchesDie: the book's pick is the old die's pick, roll for
// roll, over every season, day and combination of allowed threats.
func TestThreatPickMatchesDie(t *testing.T) {
	t.Parallel()
	book := incident.Core()
	cases := 0
	for _, winter := range []bool{false, true} {
		for _, day := range []int{0, 5, 6, 7, 8, 11, 12, 13, 30} {
			for mask := range 16 {
				allow := map[frozenThreat]bool{
					fzRaid: mask&1 != 0, fzHive: mask&2 != 0, fzPack: mask&4 != 0, fzManhunters: mask&8 != 0, fzMad: true,
				}
				okOld := func(k frozenThreat) bool { return allow[k] }
				okNew := func(id incident.ID) bool {
					for k, v := range frozenID {
						if v == id {
							return allow[k]
						}
					}
					return false
				}
				for roll := range incident.ThreatRolls {
					wantK, wantOK := frozenAllowed(roll, day, winter, okOld)
					got, gotOK := book.PickThreat(uint64(day)*clock.TicksPerDay, biome.TemperateForest,
						incident.Odds{Winter: winter}, roll, okNew)
					cases++
					if gotOK != wantOK || wantOK && got != frozenID[wantK] {
						t.Fatalf("winter %v day %d mask %04b roll %d: got %d,%v want %d,%v",
							winter, day, mask, roll, got, gotOK, frozenID[wantK], wantOK)
					}
				}
			}
		}
	}
	if cases != 2*9*16*8 {
		t.Fatalf("ran %d cases", cases)
	}
}

// TestThreatSharesMatchDie: with every threat allowed, the faces each event
// holds by season and day band are the die's documented shares (of 8).
func TestThreatSharesMatchDie(t *testing.T) {
	t.Parallel()
	book := incident.Core()
	type share struct{ pack, manhunt, hive, mad, raid int }
	for _, c := range []struct {
		winter   bool
		day      int
		want     share
		scenario string
	}{
		{false, 0, share{0, 0, 0, 0, 8}, "summer 0-5"},
		{false, 5, share{0, 0, 0, 0, 8}, "summer 0-5"},
		{false, 6, share{0, 0, 0, 1, 7}, "summer 6-7"},
		{false, 7, share{0, 0, 0, 1, 7}, "summer 6-7"},
		{false, 8, share{0, 2, 0, 1, 5}, "summer 8-11"},
		{false, 11, share{0, 2, 0, 1, 5}, "summer 8-11"},
		{false, 12, share{0, 2, 1, 1, 4}, "summer 12+"},
		{false, 30, share{0, 2, 1, 1, 4}, "summer 12+"},
		{true, 0, share{4, 0, 0, 0, 4}, "winter 0-5"},
		{true, 5, share{4, 0, 0, 0, 4}, "winter 0-5"},
		{true, 6, share{4, 0, 0, 1, 3}, "winter 6-11"},
		{true, 11, share{4, 0, 0, 1, 3}, "winter 6-11"},
		{true, 12, share{4, 0, 1, 1, 2}, "winter 12+"},
		{true, 30, share{4, 0, 1, 1, 2}, "winter 12+"},
	} {
		var got share
		for roll := range incident.ThreatRolls {
			id, ok := book.PickThreat(uint64(c.day)*clock.TicksPerDay, biome.TemperateForest,
				incident.Odds{Winter: c.winter}, roll, func(incident.ID) bool { return true })
			if !ok {
				t.Fatalf("%s day %d roll %d: no threat", c.scenario, c.day, roll)
			}
			switch id {
			case incident.PredatorPack:
				got.pack++
			case incident.ManhunterPack:
				got.manhunt++
			case incident.HiveSwarm:
				got.hive++
			case incident.MadAnimal:
				got.mad++
			case incident.Raid:
				got.raid++
			default:
				t.Fatalf("%s day %d roll %d: unexpected %d", c.scenario, c.day, roll, id)
			}
		}
		if got != c.want {
			t.Errorf("%s day %d: shares %+v, want %+v", c.scenario, c.day, got, c.want)
		}
	}
}
