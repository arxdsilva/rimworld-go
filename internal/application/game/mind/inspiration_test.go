// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package mind

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/pawn/stat"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// fakeMuses is the fake host's Muses role: skill levels by pawn (0 when
// unset) and the inspiration news.
type fakeMuses struct {
	skill            map[entity.EntityID]uint8
	passions         map[entity.EntityID]skill.Passion
	slow, nonviolent map[entity.EntityID]bool
	inspired         []entity.EntityID
	faded            []entity.EntityID
}

func (f *fakeHost) SkillLevel(id entity.EntityID, _ skill.ID) uint8      { return f.skill[id] }
func (f *fakeHost) Passion(id entity.EntityID, _ skill.ID) skill.Passion { return f.passions[id] }
func (f *fakeHost) CanRun(id entity.EntityID) bool                       { return !f.slow[id] }
func (f *fakeHost) CanShoot(id entity.EntityID) bool                     { return !f.nonviolent[id] }
func (f *fakeHost) Inspired(id entity.EntityID, _ string)                { f.inspired = append(f.inspired, id) }
func (f *fakeHost) InspirationEnded(id entity.EntityID, _ string) {
	f.faded = append(f.faded, id)
}

// creativeFake is a fake with colonist 1 at Crafting CreativityMinSkill
// and every roll 0 (an inspiration whenever one may start).
func creativeFake() (*fakeHost, *Mind) {
	f := newFake(1)
	f.skill = map[entity.EntityID]uint8{1: CreativityMinSkill}
	return f, New(f)
}

func TestMTBHours(t *testing.T) {
	t.Parallel()
	cases := map[int]int{49: 0, 0: 0, 50: 210 * 24, 70: 130 * 24, 100: 10 * 24, 120: 10 * 24}
	for mood, want := range cases {
		if got := MTBHours(mood); got != want {
			t.Errorf("MTBHours(%d) = %d, want %d", mood, got, want)
		}
	}
}

func TestInspirationStartsAndCanRestartWithoutCooldown(t *testing.T) {
	t.Parallel()
	f, m := creativeFake()
	f.mood[1] = 49
	m.Inspire.Check(1)
	if _, ok := m.Inspire.Of(1); ok {
		t.Fatal("inspired below the minimum mood")
	}
	f.mood[1] = 90
	m.Inspire.Check(1)
	in, ok := m.Inspire.Of(1)
	if !ok || in.Kind != Creativity || in.Until != InspireDuration || len(f.inspired) != 1 {
		t.Fatalf("inspiration %+v %v, news %v", in, ok, f.inspired)
	}
	if in.Kind.String() != "inspired creativity" || NoInspiration.String() != "" {
		t.Fatal("labels")
	}
	if !m.Inspire.Use(1, Creativity) || m.Inspire.Use(1, Creativity) {
		t.Fatal("an inspiration is spent exactly once")
	}
	f.now = clock.TicksPerHour
	m.Inspire.Check(1)
	if in, ok := m.Inspire.Of(1); !ok || in.Kind != Creativity || len(f.inspired) != 2 {
		t.Fatal("Core allows another inspiration on the next eligible hourly roll")
	}
}

func TestInspirationNeedsSkill(t *testing.T) {
	t.Parallel()
	f, m := creativeFake()
	f.skill[1] = CreativityMinSkill - 1
	f.mood[1] = 100
	for h := range 100 {
		f.now = uint64(h) * clock.TicksPerHour
		m.Inspire.Check(1)
	}
	if in, ok := m.Inspire.Of(1); ok && in.Kind == Creativity {
		t.Fatal("an unskilled colonist got creativity")
	}
}

func TestInspirationFades(t *testing.T) {
	t.Parallel()
	f, m := creativeFake()
	m.Inspire.Start(1, Creativity)
	f.now = InspireDuration
	if _, ok := m.Inspire.Of(1); ok {
		t.Fatal("a faded inspiration still counts")
	}
	if m.Inspire.Use(1, Creativity) {
		t.Fatal("a faded inspiration was spent")
	}
	m.Inspire.Check(1)
	if len(f.faded) != 1 {
		t.Fatalf("expired inspiration must be announced once: %v", f.faded)
	}
	f.now += clock.TicksPerHour
	m.Inspire.Check(1)
	if _, ok := m.Inspire.Of(1); !ok {
		t.Fatal("expired inspiration prevented the next hourly roll")
	}
}

// TestInspirationNoFlap: mood fluctuations do not terminate an active
// inspiration. A new one can be won after the previous 8-day duration.
func TestInspirationNoFlap(t *testing.T) {
	t.Parallel()
	f, m := creativeFake()
	starts, ends, was := 0, 0, false
	const hours = 30 * clock.HoursPerDay
	for h := range hours {
		f.now = uint64(h) * clock.TicksPerHour
		f.mood[1] = InspireMinMood - 1 + 2*(h%2) // 49, 51, 49, ...
		m.Inspire.Check(1)
		_, on := m.Inspire.Of(1)
		if on && !was {
			starts++
		}
		if !on && was {
			ends++
		}
		was = on
	}
	period := InspireDuration / clock.TicksPerHour
	if max := hours/period + 1; starts == 0 || starts > max || ends > starts {
		t.Fatalf("%d starts, %d ends in %d hours (at most %d)", starts, ends, hours, max)
	}
}

// TestInspirationCheckFromTickAndSave: old saves retain the active
// inspiration; legacy saved cooldown timestamps no longer block new rolls.
func TestInspirationCheckFromTickAndSave(t *testing.T) {
	t.Parallel()
	f, m := creativeFake()
	f.mood[1] = 100
	for h := range 2 * clock.TicksPerHour {
		f.now = uint64(h)
		m.Tick(1, job.Record{})
	}
	if _, ok := m.Inspire.Of(1); !ok {
		t.Fatal("the hourly check never ran")
	}
	s := m.Export()
	s.InspireCalm = map[entity.EntityID]uint64{1: f.now + 40*clock.TicksPerDay}
	n := New(f)
	n.Import(s)
	if _, ok := n.Inspire.Of(1); !ok || len(n.Export().InspireCalm) != 0 {
		t.Fatalf("legacy cooldown not dropped on load: %+v", n.Export())
	}
	n.Inspire.Use(1, Creativity)
	n.Inspire.Check(1)
	if _, ok := n.Inspire.Of(1); !ok {
		t.Fatal("historical cooldown blocked a new inspiration")
	}
	n.Import(Save{}) // an older save: none
	if _, ok := n.Inspire.Of(1); ok {
		t.Fatal("an older save kept an inspiration")
	}
}

func TestCoreInspirationEligibilityAndHistoricalIDs(t *testing.T) {
	t.Parallel()
	if Creativity != 1 || GoFrenzy != 2 || Recruitment != 3 || Surgery != 4 ||
		Taming != 5 || Trade != 6 || ShootFrenzy != 7 || WorkFrenzy != 8 {
		t.Fatal("saved inspiration Kind IDs changed")
	}
	f, m := creativeFake()
	if got := m.Inspire.choose(1); got != Creativity {
		t.Fatalf("skilled creative chose %v", got)
	}
	f.skill[1] = 0
	if m.Inspire.eligible(1, Creativity) || m.Inspire.eligible(1, Surgery) ||
		m.Inspire.eligible(1, Recruitment) || m.Inspire.eligible(1, Taming) || m.Inspire.eligible(1, Trade) {
		t.Fatal("zero-skill colonist got a skilled inspiration")
	}
	if !m.Inspire.eligible(1, GoFrenzy) || !m.Inspire.eligible(1, ShootFrenzy) ||
		!m.Inspire.eligible(1, WorkFrenzy) {
		t.Fatal("general inspiration prerequisites")
	}
	f.slow = map[entity.EntityID]bool{1: true}
	f.nonviolent = map[entity.EntityID]bool{1: true}
	if m.Inspire.eligible(1, GoFrenzy) || m.Inspire.eligible(1, ShootFrenzy) ||
		m.Inspire.choose(1) != WorkFrenzy {
		t.Fatal("trait and work-incapacity exclusions")
	}
	f.skill[1] = 3
	if !m.Inspire.eligible(1, Surgery) || !m.Inspire.eligible(1, Taming) ||
		!m.Inspire.eligible(1, Recruitment) || !m.Inspire.eligible(1, Trade) {
		t.Fatal("required skills did not unlock Core inspirations")
	}
	f.passions = map[entity.EntityID]skill.Passion{1: skill.Major}
	if got := m.Inspire.weight(1, Recruitment); got != 50 {
		t.Fatalf("major passion weight %d", got)
	}
	f.passions[1] = skill.Minor
	if got := m.Inspire.weight(1, Recruitment); got != 25 {
		t.Fatalf("minor passion weight %d", got)
	}
}

func TestCoreInspirationUseAndExpiry(t *testing.T) {
	t.Parallel()
	f, m := creativeFake()
	for k := Creativity; k < numInspirations; k++ {
		m.Inspire.Start(1, k)
		if !m.Inspire.Has(1, k) || !m.Inspire.Use(1, k) || m.Inspire.Use(1, k) {
			t.Fatalf("one-use inspiration %v", k)
		}
		f.now += clock.TicksPerHour
	}
	m.Inspire.Start(1, WorkFrenzy)
	f.now += InspireDuration
	if m.Inspire.Has(1, WorkFrenzy) {
		t.Fatal("timed frenzy outlasted eight days")
	}
	m.Inspire.Check(1)
	if _, ok := m.Inspire.Of(1); ok {
		t.Fatal("expired frenzy remained active")
	}
}

func TestInspiredStatMultipliers(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		k       InspirationKind
		s       stat.ID
		v, want int32
	}{
		{GoFrenzy, stat.MoveSpeed, 1000, 1400},
		{WorkFrenzy, stat.WorkSpeed, 1000, 1800},
		{ShootFrenzy, stat.ShootAcc, 0, 8},
		{Creativity, stat.MoveSpeed, 1000, 1000},
	} {
		if got := AdjustStat(tc.s, tc.v, tc.k); got != tc.want {
			t.Errorf("kind %v stat %v: %d want %d", tc.k, tc.s, got, tc.want)
		}
	}
}

func TestDeadColonistDropsInspiration(t *testing.T) {
	t.Parallel()
	f, m := creativeFake()
	m.Inspire.Start(1, WorkFrenzy)
	f.dead[1] = true
	m.Downed(1)
	if _, ok := m.Inspire.Of(1); ok || len(m.Export().Inspire) != 0 {
		t.Fatal("the dead colonist retained an active or saved inspiration")
	}
}

func TestDownedLivingPawnRetainsInspiration(t *testing.T) {
	t.Parallel()
	f, m := creativeFake()
	m.Inspire.Start(1, WorkFrenzy)
	f.down[1] = true
	m.Downed(1)
	if !m.Inspire.Has(1, WorkFrenzy) {
		t.Fatal("temporary incapacity ended an active inspiration")
	}
}
