// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package mind

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/pawn/mentalbreak"
)

// breakEnd is one way a break comes to an end in the fake world.
type breakEnd struct {
	name string
	kind mentalbreak.Kind
	end  func(f *fakeHost, m *Mind)
}

var breakEnds = []breakEnd{
	{"natural", mentalbreak.SadWander, func(f *fakeHost, m *Mind) {
		_, until, _ := f.Break(1)
		f.now = until
		m.Tick(1, job.Record{})
	}},
	{"targeted insult victim gone", mentalbreak.TargetedInsult, func(f *fakeHost, m *Mind) {
		f.dead[m.Target(1)] = true
		m.Tick(1, job.Record{})
		f.dead[2], f.dead[3] = false, false
	}},
	{"catatonic recovers", mentalbreak.Catatonic, func(f *fakeHost, m *Mind) {
		_, until, _ := f.Break(1)
		f.now = until
		m.Downed(1)
	}},
	{"arrest or player End", mentalbreak.SadWander, func(_ *fakeHost, m *Mind) { m.End(1) }},
	{"interrupted berserk", mentalbreak.Berserk, func(_ *fakeHost, m *Mind) { m.End(1) }},
	{"downed", mentalbreak.HideInRoom, func(f *fakeHost, m *Mind) {
		f.down[1] = true
		m.Downed(1)
		f.down[1] = false // tended and back on its feet
	}},
	{"spree turns into a fight", mentalbreak.InsultingSpree, func(f *fakeHost, m *Mind) {
		m.StartFight(1, 2)
		m.EndFight(1, 2)
	}},
}

// TestBreakRetryCooldownDoesNotFlap is the no-flap test of a break's end:
// however a break ends, the colonist starts no new one for BreakRetryTicks
// (not at rock-bottom mood, not with thresholds pushed above catharsis'
// +40, not across a save), and the first hourly check after it may break.
func TestBreakRetryCooldownDoesNotFlap(t *testing.T) {
	moods := []struct{ mood, shift int }{
		{0, 0},   // rock bottom, no catharsis (an interrupted break)
		{45, 20}, // catharsis-lifted mood under a brittle colonist's 50
	}
	for _, e := range breakEnds {
		for _, md := range moods {
			f := newFake(1, 2, 3)
			m := New(f)
			f.canHide = true
			f.rolls = []uint32{0} // every roll that is made breaks
			m.Force(1, e.kind)
			f.now += 10
			e.end(f, m)
			if _, _, ok := f.Break(1); ok {
				t.Fatalf("%s: the break did not end", e.name)
			}
			f.mood[1], f.shift[1] = md.mood, md.shift
			open := f.now + BreakRetryTicks
			saved := false
			for toCheck(f, 1); f.now < open; toCheck(f, 1) {
				if !saved && f.now > open-BreakRetryTicks/2 {
					s := m.Export() // a save in the middle of the window
					m = New(f)
					m.Import(s)
					saved = true
				}
				ri, started := f.ri, len(f.started)
				if m.Tick(1, job.Record{}) || len(f.started) != started || f.ri != ri {
					t.Fatalf("%s at mood %d shift %d: a new break (or roll) %d ticks after the end, inside the %d-tick retry window",
						e.name, md.mood, md.shift, f.now+BreakRetryTicks-open, BreakRetryTicks)
				}
			}
			if !saved {
				t.Fatalf("%s: the window held no hourly check after a save", e.name)
			}
			if !m.Tick(1, job.Record{}) {
				t.Fatalf("%s at mood %d shift %d: no break at the first check after the retry window", e.name, md.mood, md.shift)
			}
		}
	}
}

// TestBreakRetryWindowOutlastsAFight: a colonist dragged into a social
// fight during its retry window keeps the window (a fight is no mood break
// and does not reset it).
func TestBreakRetryWindowOutlastsAFight(t *testing.T) {
	f := newFake(1, 2, 3)
	m := New(f)
	f.rolls = []uint32{0}
	m.Force(1, mentalbreak.SadWander)
	m.End(1)
	open := f.now + BreakRetryTicks
	f.mood[1] = 0
	f.now += 100
	m.StartFight(2, 1)
	m.EndFight(2, 1)
	for toCheck(f, 1); f.now < open; toCheck(f, 1) {
		if m.Tick(1, job.Record{}) {
			t.Fatalf("a fight cleared the retry window: a break %d ticks before it closes", open-f.now)
		}
	}
	if !m.Tick(1, job.Record{}) {
		t.Fatal("no break after the window")
	}
}
