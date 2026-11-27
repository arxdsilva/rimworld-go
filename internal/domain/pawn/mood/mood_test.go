// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package mood

import (
	"rimworld-go/internal/domain/sim/clock"
	"testing"
)

// cb is the core thought book the tests read.
var cb = Core()

func TestMemoriesAndLevel(t *testing.T) {
	t.Parallel()
	var m Memories
	m = cb.Add(m, AteRaw, 100)
	m = cb.Add(m, AteRaw, 200) // refresh, not duplicate
	m = cb.Add(m, SleptInNiceRoom, 100)
	if len(m) != 2 || m[0].Expires != 200+cb.DefOf(AteRaw).Duration {
		t.Fatalf("memories = %+v", m)
	}
	ts := cb.Active(m, Hungry)
	want := Base + cb.DefOf(AteRaw).Value + cb.DefOf(SleptInNiceRoom).Value + cb.DefOf(Hungry).Value
	if Level(ts) != want || len(ts) != 3 {
		t.Fatalf("level = %d want %d", Level(ts), want)
	}
	m = m.Prune(100 + cb.DefOf(SleptInNiceRoom).Duration)
	if len(m) != 1 || m[0].Kind != AteRaw {
		t.Fatalf("after prune = %+v", m)
	}
	if Label(10) != "miserable" || Label(90) != "joyful" {
		t.Fatal("labels")
	}
}

func TestOppositeThoughtsReplaceEachOther(t *testing.T) {
	t.Parallel()
	var m Memories
	m = cb.Add(cb.Add(m, AteWithoutTable, 1), AteRaw, 1)
	m = cb.Add(cb.Add(m, AteAtTable, 2), AteMeal, 2)
	if len(m) != 2 {
		t.Fatalf("memories = %+v", m)
	}
	for _, x := range m {
		if x.Kind == AteWithoutTable || x.Kind == AteRaw {
			t.Fatal("newer meal replaces the opposite memory")
		}
	}
	m = cb.Add(m, SleptInNiceRoom, 3).ForgetGroupOf(SleptInAwfulRoom)
	if len(m) != 2 {
		t.Fatal("forget drops the room-quality group only")
	}
}

func TestDeathAndBurialThoughts(t *testing.T) {
	t.Parallel()
	// ENG-003 balance override: -3 for six days.
	if d := cb.DefOf(ColonistDied); d.Value != -3 || d.Duration != 6*day {
		t.Fatalf("colonist died: %+v", d)
	}
	if d := cb.DefOf(UnburiedCorpse); d.Value != -2 || d.Duration != 0 {
		t.Fatalf("unburied corpse is a state thought: %+v", d)
	}
	var m Memories
	m = cb.Add(m, LaidToRest, 0)
	m = cb.Add(m, LaidToRest, 10)
	if len(m) != 1 || Level(cb.Active(m)) != Base+3 || m[0].Expires != 10+day {
		t.Fatalf("laid to rest: +3 for a day, never stacked: %+v", m)
	}
}

// TestSocialThoughtDefs (ENG-003): every kind has a label, stacking
// defaults to one memory at full weight, the "stack" kinds stack five
// deep at 900 permille, and a memory's Value overrides its def's.
func TestSocialThoughtDefs(t *testing.T) {
	t.Parallel()
	for k := Kind(0); k < numKinds; k++ {
		if cb.DefOf(k).Label == "" {
			t.Fatalf("kind %d has no label", k)
		}
	}
	if d := cb.DefOf(SleptInBed); d.StackLimit != 1 || d.StackFactor != 1000 {
		t.Fatalf("default stacking: %+v", d)
	}
	for _, k := range []Kind{Insulted, KindWords} {
		if d := cb.DefOf(k); d.StackLimit != 10 || d.StackFactor != 900 {
			t.Fatalf("%s stacking: %+v", d.Label, d)
		}
	}
	for _, k := range []Kind{BondedDied, BondedLost, BondedReleased} {
		if d := cb.DefOf(k); d.StackLimit != 5 || d.StackFactor != 750 {
			t.Fatalf("%s stacking: %+v", d.Label, d)
		}
	}
	if d := cb.DefOf(Catharsis); d.Value != 40 || d.Duration != 3*day || d.StackLimit != 5 || d.StackFactor != 750 {
		t.Fatalf("catharsis: %+v", d)
	}
	if d := cb.DefOf(GotMarried); d.Value != 40 || d.Duration != 30*day {
		t.Fatalf("got married: %+v", d)
	}
	if d := cb.DefOf(Grief); d.Value != 0 || d.Duration != 0 {
		t.Fatalf("grief: %+v", d)
	}
	m := Memories{{Kind: Grief, Expires: 10, Subject: 7, Value: -12}, {Kind: Catharsis, Expires: 10}}
	if got := Level(cb.Active(m)); got != Base-12+40 {
		t.Fatalf("Value override: level %d", got)
	}
	if cb.DefOf(numKinds).Label != "" {
		t.Fatal("out of range")
	}
}

func TestAddForStacksAndRefreshesOldest(t *testing.T) {
	t.Parallel()
	var m Memories
	lim := int(cb.DefOf(Insulted).StackLimit)
	for i := 0; i < lim; i++ {
		m = cb.AddFor(m, Insulted, 7, uint64(100+i))
	}
	if len(m) != lim {
		t.Fatalf("len %d want %d", len(m), lim)
	}
	m = cb.AddFor(m, Insulted, 7, 9000)
	if len(m) != lim || m[len(m)-1].Expires != 9000+cb.DefOf(Insulted).Duration {
		t.Fatalf("oldest event was not replaced in event order: %+v", m)
	}
	m = cb.AddFor(m, Insulted, 8, 10) // the cap is per thought kind, not subject
	if len(m) != lim || m[len(m)-1].Subject != 8 {
		t.Fatalf("per-kind cap after another subject: %+v", m)
	}
	// non-stacking kind refreshes
	m = cb.AddFor(cb.AddFor(nil, GotMarried, 1, 5), GotMarried, 1, 50)
	if len(m) != 1 || m[0].Expires != 50+cb.DefOf(GotMarried).Duration {
		t.Fatalf("%+v", m)
	}
}

func TestAddForGroupReplacement(t *testing.T) {
	t.Parallel()
	m := cb.AddFor(cb.AddFor(nil, SleptOnGround, 0, 1), SleptInBed, 0, 2)
	if len(m) != 1 || m[0].Kind != SleptInBed {
		t.Fatalf("%+v", m)
	}
}

func TestAddValueAndDiminishingActive(t *testing.T) {
	t.Parallel()
	m := cb.AddValue(nil, Grief, 3, -20, 0)
	if m[0].Value != -20 {
		t.Fatalf("%+v", m)
	}
	m = cb.AddValue(m, Grief, 3, -30, 5) // limit 1: replaced
	if len(m) != 1 || m[0].Value != -30 {
		t.Fatalf("%+v", m)
	}
	var s Memories
	for i := 0; i < 3; i++ {
		s = cb.AddFor(s, KindWords, uint64(i+1), 0)
	}
	ts := cb.Active(s)
	if ts[0].Value != 5 || ts[1].Value != 5 || ts[2].Value != 4 {
		t.Fatalf("%+v", ts)
	}
}

func TestApproach(t *testing.T) {
	t.Parallel()
	if got := Approach(50000, 90000, 2500); got != 62000 {
		t.Fatalf("rise %d", got)
	}
	if got := Approach(50000, 10000, 2500); got != 42000 {
		t.Fatalf("fall %d", got)
	}
	if got := Approach(50000, 50500, 2500); got != 50500 {
		t.Fatalf("snap up %d", got)
	}
	if got := Approach(50000, 49500, 2500); got != 49500 {
		t.Fatalf("snap down %d", got)
	}
	if got := Approach(50000, 50000, 99); got != 50000 {
		t.Fatalf("same %d", got)
	}
}

// A memory's own label replaces the def's in Active; values and stacking
// are unchanged, and a refresh takes the new label.
func TestMemoryLabelOverride(t *testing.T) {
	var m Memories
	m = cb.AddLabeledUntil(m, Grief, 7, -20, "my wife died", 0, day)
	m = cb.AddLabeledUntil(m, Grief, 8, -10, "", 0, day)
	ts := cb.Active(m)
	if len(ts) != 2 || ts[0].Label != "my wife died" || ts[0].Value != -20 {
		t.Fatalf("labelled grief %+v", ts)
	}
	if ts[1].Label != cb.DefOf(Grief).Label || ts[1].Value != -10 {
		t.Fatalf("unlabelled grief keeps the def's line: %+v", ts[1])
	}
	m = cb.AddLabeledUntil(m, Grief, 7, -16, "my lover died", 5, day+5)
	if ts := cb.Active(m); len(ts) != 2 || ts[0].Label != "my lover died" || ts[0].Value != -16 {
		t.Fatalf("refreshed grief %+v", ts)
	}
}

// Conditional thought suppression is deterministic and leaves saved
// memories unaffected. A full-probability test pins the classification.
func TestNullifyConditionalKeepsMemories(t *testing.T) {
	active := []Thought{
		{Kind: Hungry, Value: -5},
		{Kind: AteMeal, Value: 5, Memory: true},
		{Kind: BrawlerRanged, Value: 10},
		{Kind: AnestheticWoozy, Value: 10},
	}
	got := NullifyConditional(active, 10000, 17, 3, 77)
	if len(got) != 1 || got[0].Kind != AteMeal {
		t.Fatalf("conditional thought suppression: %+v", got)
	}
	for i := uint64(1); i <= 25; i++ {
		first := NullifyConditional([]Thought{{Kind: Hungry, Value: -5}}, 100, 17, i, 77)
		again := NullifyConditional([]Thought{{Kind: Hungry, Value: -5}}, 100, 17, i, 77)
		if len(first) != len(again) {
			t.Fatalf("same pawn/time varied for %d", i)
		}
	}
}

func TestAppendActiveAtFiltersExpiredWithoutChangingMemories(t *testing.T) {
	t.Parallel()
	memories := Memories{
		{Kind: AteRaw, Expires: 10},
		{Kind: SleptInNiceRoom, Expires: 20},
		{Kind: AteWithoutTable, Expires: 10},
	}
	thoughts := cb.AppendActiveAt(nil, memories, 10, Hungry)
	if len(thoughts) != 2 || thoughts[0].Kind != SleptInNiceRoom || thoughts[1].Kind != Hungry {
		t.Fatalf("expired memories must not be counted: %+v", thoughts)
	}
	if len(memories) != 3 || memories[0].Kind != AteRaw || memories[1].Kind != SleptInNiceRoom || memories[2].Kind != AteWithoutTable {
		t.Fatalf("memory backing array changed: %+v", memories)
	}
	if got := cb.AppendActiveAt(nil, memories, 20); len(got) != 0 {
		t.Fatalf("expiry boundary is inclusive: %+v", got)
	}
}

func TestPruneClearsExpiredLabels(t *testing.T) {
	t.Parallel()
	live := Memory{Kind: SleptInNiceRoom, Expires: 20}
	m := Memories{{Kind: Grief, Expires: 10, Label: "an expired grief"}, live}
	m = m.Prune(10)
	if len(m) != 1 || m[0] != live {
		t.Fatalf("pruned memories = %+v", m)
	}
	if tail := m[:cap(m)][1]; tail != (Memory{}) {
		t.Fatalf("expired memory still retained in backing array: %+v", tail)
	}
}

func TestStackingDiminishesToZero(t *testing.T) {
	t.Parallel()
	m := make(Memories, 128)
	for i := range 64 {
		m[i*2] = Memory{Kind: KindWords, Subject: uint64(i + 1), Expires: 200}
		m[i*2+1] = Memory{Kind: Insulted, Subject: uint64(i + 1), Expires: 200}
	}
	ts := cb.AppendActiveAt(nil, m, 100)
	positive, negative := 0, 0
	for i := 0; i < 64; i++ {
		if i < 10 {
			positive += ts[i*2].Value
			negative += ts[i*2+1].Value
		} else if ts[i*2].Value != 0 || ts[i*2+1].Value != 0 {
			t.Fatalf("stack %d exceeded the Core cap: %+v", i, ts[i*2:i*2+2])
		}
	}
	if positive != 33 || negative != -33 {
		t.Fatalf("rounded cumulative stacks: kindness %d, insult %d", positive, negative)
	}
}

func TestGroupReplacementReleasesRemovedMemories(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		edit func(Memories) Memories
	}{
		{"Add", func(m Memories) Memories { return cb.Add(m, SleptInBed, 100) }},
		{"AddFor", func(m Memories) Memories { return cb.AddFor(m, SleptInBed, 1, 100) }},
		{"ForgetGroupOf", func(m Memories) Memories { return m.ForgetGroupOf(SleptInBed) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := Memories{
				{Kind: SleptOutside, Expires: 200, Label: "old sleep"},
				{Kind: KindWords, Expires: 200, Label: "retained"},
				{Kind: SleptOnGround, Expires: 200, Label: "old ground"},
			}
			got := tc.edit(m)
			if got[0].Label != "retained" {
				t.Fatalf("kept memories = %+v", got)
			}
			for _, removed := range m[len(got):] {
				if removed != (Memory{}) {
					t.Fatalf("removed memory remains referenced: %+v", removed)
				}
			}
		})
	}
}

func TestUnscaledMemoriesKeepTheirValues(t *testing.T) {
	t.Parallel()
	memories := Memories{
		{Kind: Grief, Subject: 1, Expires: 200, Value: -20},
		{Kind: Grief, Subject: 2, Expires: 200, Value: -10},
		{Kind: Grief, Subject: 3, Expires: 200, Value: -30},
	}
	got := cb.AppendActiveAt(nil, memories, 100)
	for i, want := range []int{-20, -10, -30} {
		if got[i].Value != want {
			t.Fatalf("grief %d = %d, want %d", i, got[i].Value, want)
		}
	}
}

func TestAddLabeledUntilRefreshesExpiry(t *testing.T) {
	t.Parallel()
	m := cb.AddLabeledUntil(nil, Grief, 7, -20, "my wife died", 0, 1200)
	if len(m) != 1 || m[0].Expires != 1200 || m[0].Value != -20 || m[0].Label != "my wife died" {
		t.Fatalf("first grief %+v", m)
	}
	m = cb.AddLabeledUntil(m, Grief, 8, -14, "my sibling died", 0, 900)
	m = cb.AddLabeledUntil(m, Grief, 7, -16, "my lover died", 0, 1800)
	if len(m) != 2 || m[0].Expires != 1800 || m[0].Value != -16 || m[0].Label != "my lover died" || m[1].Expires != 900 {
		t.Fatalf("refreshed grief %+v", m)
	}
	if got := cb.AppendActiveAt(nil, m, 900); len(got) != 1 || got[0].Value != -16 {
		t.Fatalf("expired grief remained active: %+v", got)
	}
}

func TestMemoryInsertionDiscardsExpiredEntries(t *testing.T) {
	t.Parallel()
	now := uint64(1000)
	base := Memories{
		{Kind: Insulted, Subject: 7, Expires: 999, Label: "expired insult"},
		{Kind: KindWords, Subject: 1, Expires: 999, Label: "expired kindness"},
		{Kind: SleptInNiceRoom, Expires: 1001},
	}
	for _, tc := range []struct {
		name string
		add  func(Memories) Memories
	}{
		{"Add", func(m Memories) Memories { return cb.Add(m, AteRaw, now) }},
		{"AddFor", func(m Memories) Memories { return cb.AddFor(m, Insulted, 7, now) }},
		{"AddLabeled", func(m Memories) Memories { return cb.AddLabeled(m, KindWords, 1, 5, "fresh", now) }},
		{"AddLabeledUntil", func(m Memories) Memories {
			return cb.AddLabeledUntil(m, Grief, 7, -10, "grief", now, now+10)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.add(append(Memories(nil), base...))
			if len(got) != 2 {
				t.Fatalf("%s still stores expired memories: %+v", tc.name, got)
			}
			for _, m := range got {
				if m.Expires <= now || m.Label == "expired insult" || m.Label == "expired kindness" {
					t.Fatalf("%s retained expired memory: %+v", tc.name, got)
				}
			}
		})
	}
}

func TestCatharsisStacksAndExpiresAtCoreLimit(t *testing.T) {
	t.Parallel()
	var m Memories
	for i := range 7 {
		m = cb.Add(m, Catharsis, uint64(i)*clock.TicksPerHour)
	}
	if len(m) != 5 {
		t.Fatalf("catharsis cap %d", len(m))
	}
	ts := cb.AppendActiveAt(nil, m, 6*clock.TicksPerHour)
	total := 0
	for _, thought := range ts {
		total += thought.Value
	}
	if total != 122 {
		t.Fatalf("catharsis stacked %d, want 122", total)
	}
	if len(cb.AppendActiveAt(nil, m, 3*clock.TicksPerDay+6*clock.TicksPerHour)) != 0 {
		t.Fatal("catharsis persisted beyond three days")
	}
}

func TestSocialStackCapIsPerThoughtKind(t *testing.T) {
	t.Parallel()
	var m Memories
	for i := range 15 {
		m = cb.AddFor(m, KindWords, uint64(i+1), uint64(i))
	}
	if len(m) != 10 {
		t.Fatalf("kind words cap: %d", len(m))
	}
	total := 0
	for _, thought := range cb.Active(m) {
		total += thought.Value
	}
	if total != 33 {
		t.Fatalf("kind words total: %d", total)
	}
	if m[0].Subject != 6 || m[len(m)-1].Subject != 15 {
		t.Fatalf("oldest event replacement order: %+v", m)
	}
}
