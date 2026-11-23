// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package overlay

import (
	"testing"

	"rimworld-go/internal/domain/combat/attack"
)

func cardSum(s ShotInfo) int {
	n := 0
	for _, f := range s.Factors {
		n += f.Pct
	}
	return n
}

// TestShotCardAddsUp (SHT-201): over thousands of deterministic reports
// (downed, clamped and covered ones among them) the card's lines sum to its
// headline, which is the report's total in percent.
func TestShotCardAddsUp(t *testing.T) {
	t.Parallel()
	x := uint64(88172645463325252)
	next := func(n uint64) attack.Permille {
		x ^= x << 13
		x ^= x >> 7
		x ^= x << 17
		return attack.Permille(x % n)
	}
	var buf []ShotFactor
	var clamped, downed, covered int
	const reports = 6000
	for i := 0; i < reports; i++ {
		rep := attack.Report{
			Band: next(1200), Shooter: 500 + next(1100), Weather: 500 + next(600),
			Size: 300 + next(1500), Light: 500 + next(600), Cover: next(900),
			InRange: true, InSight: true, Melee: i%11 == 0,
		}
		if i%3 == 0 {
			rep.Downed = 1000 + next(8000)
		}
		s := ShotInfoOf(buf[:0], rep, 0, 0, 3, 3)
		buf = s.Factors
		if s.Chance != int(rep.Total())/10 {
			t.Fatalf("report %d: headline %d, total %d", i, s.Chance, rep.Total())
		}
		if got := cardSum(s); got != s.Chance {
			t.Fatalf("report %d (%+v): lines %v sum %d, headline %d", i, rep, s.Factors, got, s.Chance)
		}
		if !rep.Melee {
			clamped += b2i(rep.Aim() < attack.Permille(int(rep.Band)))
			downed += b2i(rep.Downed != 0)
			covered += b2i(rep.Cover != 0)
		}
	}
	if clamped == 0 || downed == 0 || covered == 0 {
		t.Fatalf("the sample missed a case: clamped %d downed %d covered %d", clamped, downed, covered)
	}
}

func labels(s ShotInfo) map[string]int {
	m := map[string]int{}
	for _, f := range s.Factors {
		m[f.Label] = f.Pct
	}
	return m
}

// TestShotCardLabels: a downed target close by shows its "downed target"
// line and the "cap" that clamps it; a melee card is the one "melee" line;
// the base line is the weapon band.
func TestShotCardLabels(t *testing.T) {
	t.Parallel()
	rep := attack.Report{Band: 700, Shooter: 1000, Weather: 1000, Size: 1000, Light: 1000,
		Downed: attack.DownedFactor(2), InRange: true, InSight: true}
	s := ShotInfoOf(nil, rep, 0, 0, 2, 0)
	m := labels(s)
	if s.Factors[0].Label != "weapon" || m["weapon"] != 70 {
		t.Fatalf("base line %+v", s.Factors)
	}
	if m["downed target"] <= 0 || m["cap"] >= 0 {
		t.Fatalf("a downed target at 2 tiles: want a downed and a cap line, got %+v", s.Factors)
	}
	if s.Chance != 100 || cardSum(s) != 100 {
		t.Fatalf("a close downed target is capped at 100: %d / %d", s.Chance, cardSum(s))
	}
	melee := ShotInfoOf(nil, attack.Report{Band: 800, Melee: true, InRange: true, InSight: true}, 0, 0, 1, 0)
	if len(melee.Factors) != 1 || melee.Factors[0].Label != "melee" || melee.Factors[0].Pct != 80 {
		t.Fatalf("melee card %+v", melee.Factors)
	}
}

// TestShotCardBlocked keeps the blocked reasons: range before sight.
func TestShotCardBlocked(t *testing.T) {
	t.Parallel()
	if s := ShotInfoOf(nil, attack.Report{Band: 500, InSight: true}, 0, 0, 1, 1); !s.Blocked || s.Reason != "out of range" {
		t.Fatalf("%+v", s)
	}
	if s := ShotInfoOf(nil, attack.Report{Band: 500, InRange: true}, 0, 0, 1, 1); !s.Blocked || s.Reason != "no line of sight" {
		t.Fatalf("%+v", s)
	}
	if s := ShotInfoOf(nil, attack.Report{Band: 500, InRange: true, InSight: true}, 0, 0, 1, 1); s.Blocked {
		t.Fatalf("%+v", s)
	}
}
