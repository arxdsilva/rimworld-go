// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package mood

import (
	"reflect"
	"slices"
	"testing"
)

// coreTunings is every core thought's tuning, in kind order.
func coreTunings() []Tuning {
	list := make([]Tuning, numKinds)
	for k := range list {
		d := cb.DefOf(Kind(k))
		list[k] = Tuning{Label: d.Label, Value: d.Value, Duration: d.Duration, StackLimit: d.StackLimit, StackFactor: d.StackFactor}
	}
	return list
}

// patched is a book with edit applied to the tuning of kind k.
func patched(t *testing.T, k Kind, edit func(*Tuning)) Book {
	t.Helper()
	list := coreTunings()
	edit(&list[k])
	b, ok := NewBook(list)
	if !ok {
		t.Fatal("NewBook rejected a full list")
	}
	return b
}

func TestZeroBookIsCore(t *testing.T) {
	t.Parallel()
	var zero Book
	if zero.Len() != int(numKinds) || cb.Len() != int(numKinds) {
		t.Fatalf("len zero %d core %d", zero.Len(), cb.Len())
	}
	for k := Kind(0); k <= numKinds; k++ {
		if zero.DefOf(k) != cb.DefOf(k) || zero.Key(k) != cb.Key(k) {
			t.Fatalf("kind %d: zero book differs from Core()", k)
		}
	}
	if zero.DefOf(numKinds) != (Def{}) || zero.Key(numKinds) != "" {
		t.Fatal("out of range is empty")
	}
	if !slices.Equal(zero.Keys(), cb.Keys()) {
		t.Fatal("Keys differ")
	}
	if k, ok := zero.KindOf("core.hungry"); !ok || k != Hungry {
		t.Fatalf("zero KindOf = %d %v", k, ok)
	}
}

func TestNewBookValidatesLengthAndCopies(t *testing.T) {
	t.Parallel()
	for _, n := range []int{0, 1, int(numKinds) - 1, int(numKinds) + 1} {
		b, ok := NewBook(make([]Tuning, n))
		if ok || b.DefOf(Hungry) != cb.DefOf(Hungry) {
			t.Fatalf("length %d: ok %v, want (Core(), false)", n, ok)
		}
	}
	list := coreTunings()
	list[Hungry].Value = -99
	b, ok := NewBook(list)
	if !ok || b.DefOf(Hungry).Value != -99 {
		t.Fatalf("patched value: ok %v def %+v", ok, b.DefOf(Hungry))
	}
	if b.DefOf(Hungry).Group != cb.DefOf(Hungry).Group || b.DefOf(SleptOnGround).Group != cb.DefOf(SleptOnGround).Group {
		t.Fatal("groups stay core's")
	}
	list[Hungry].Value = 77 // mutating the input afterwards
	if b.DefOf(Hungry).Value != -99 {
		t.Fatal("the book shares the caller's slice")
	}
	if cb.DefOf(Hungry).Value == -99 {
		t.Fatal("core was patched")
	}
	list[Hungry].StackLimit, list[Hungry].StackFactor = 0, 0
	if b2, _ := NewBook(list); b2.DefOf(Hungry).StackLimit != 1 || b2.DefOf(Hungry).StackFactor != 1000 {
		t.Fatalf("defaults apply to patched zeros: %+v", b2.DefOf(Hungry))
	}
}

func TestPatchedLineAndValueShow(t *testing.T) {
	t.Parallel()
	b := patched(t, AteRaw, func(tu *Tuning) { tu.Value, tu.Label = -1, "raw patched" })
	b = patchedOn(t, b, Hungry, func(tu *Tuning) { tu.Value, tu.Label = -9, "starving patched" })
	m := b.Add(nil, AteRaw, 100)
	for name, ts := range map[string][]Thought{
		"Active":         b.Active(m, Hungry),
		"AppendActiveAt": b.AppendActiveAt(nil, m, 100, Hungry),
	} {
		if len(ts) != 2 || ts[0].Label != "raw patched" || ts[0].Value != -1 || ts[1].Label != "starving patched" || ts[1].Value != -9 {
			t.Fatalf("%s: %+v", name, ts)
		}
	}
	core := cb.Active(m, Hungry)
	delta := (-1 - cb.DefOf(AteRaw).Value) + (-9 - cb.DefOf(Hungry).Value)
	if got, want := Level(b.Active(m, Hungry)), Level(core)+delta; got != want {
		t.Fatalf("level %d want %d", got, want)
	}
}

// patchedOn patches kind k of an already patched book.
func patchedOn(t *testing.T, b Book, k Kind, edit func(*Tuning)) Book {
	t.Helper()
	list := make([]Tuning, numKinds)
	for i := range list {
		d := b.DefOf(Kind(i))
		list[i] = Tuning{Label: d.Label, Value: d.Value, Duration: d.Duration, StackLimit: d.StackLimit, StackFactor: d.StackFactor}
	}
	edit(&list[k])
	out, ok := NewBook(list)
	if !ok {
		t.Fatal("NewBook rejected a full list")
	}
	return out
}

func TestMemoryOverridesBeatPatchedDef(t *testing.T) {
	t.Parallel()
	b := patched(t, Grief, func(tu *Tuning) { tu.Value, tu.Label = -50, "patched grief" })
	m := Memories{
		{Kind: Grief, Subject: 1, Expires: 200, Value: -7, Label: "own line"},
		{Kind: Grief, Subject: 2, Expires: 200},
	}
	ts := b.AppendActiveAt(nil, m, 100)
	if ts[0].Value != -7 || ts[0].Label != "own line" {
		t.Fatalf("own value and label win: %+v", ts[0])
	}
	if ts[1].Value != -50 || ts[1].Label != "patched grief" {
		t.Fatalf("empty memory reads the patched def: %+v", ts[1])
	}
}

func TestDurationPatchAppliesToNewMemoriesOnly(t *testing.T) {
	t.Parallel()
	b := patched(t, AteRaw, func(tu *Tuning) { tu.Duration = 123 })
	old := cb.Add(nil, AteRaw, 1000)
	oldExpiry := old[0].Expires
	if got := b.Add(nil, AteRaw, 1000); got[0].Expires != 1000+123 {
		t.Fatalf("new memory expires %d", got[0].Expires)
	}
	if got := b.AddLabeled(nil, AteRaw, 0, 0, "", 1000); got[0].Expires != 1123 {
		t.Fatalf("AddLabeled expires %d", got[0].Expires)
	}
	if ts := b.AppendActiveAt(nil, old, 1001); len(ts) != 1 || old[0].Expires != oldExpiry {
		t.Fatalf("a core-added memory keeps its stored expiry: %+v %+v", ts, old)
	}
}

func TestStackPatchesFollowTheBook(t *testing.T) {
	t.Parallel()
	limit := patched(t, KindWords, func(tu *Tuning) { tu.StackLimit = 2 })
	var m Memories
	for i := range 5 {
		m = limit.AddFor(m, KindWords, uint64(i+1), uint64(i))
	}
	if len(m) != 2 {
		t.Fatalf("patched stack limit caps at %d", len(m))
	}
	// an old save with more memories than the cap reads the extras as zero
	long := Memories{{Kind: KindWords, Subject: 1, Expires: 9}, {Kind: KindWords, Subject: 2, Expires: 9}, {Kind: KindWords, Subject: 3, Expires: 9}}
	if ts := limit.AppendActiveAt(nil, long, 0); ts[2].Value != 0 {
		t.Fatalf("extra memory above the cap: %+v", ts)
	}

	factor := patched(t, KindWords, func(tu *Tuning) { tu.StackFactor = 500 })
	m = nil
	for i := range 3 {
		m = factor.AddFor(m, KindWords, uint64(i+1), 0)
	}
	// value 5 at weights 1, 0.5, 0.25: cumulative 5, 7.5, 8.75 round to 5, 8, 9.
	ts := factor.Active(m)
	if ts[0].Value != 5 || ts[1].Value != 3 || ts[2].Value != 1 {
		t.Fatalf("stacked values %+v", ts)
	}
	if got := ts[0].Value + ts[1].Value + ts[2].Value; got != 9 {
		t.Fatalf("stacked total %d, want 9", got)
	}
}

// A save written under a higher stack limit and loaded under a patched
// limit of one reads one memory per kind and subject: extras count zero,
// other subjects keep their own slot, and an expired copy frees it.
func TestStackLimitOneCapsOverCapSavesPerSubject(t *testing.T) {
	t.Parallel()
	one := patched(t, KindWords, func(tu *Tuning) { tu.StackLimit = 1 })
	v := one.DefOf(KindWords).Value
	long := Memories{
		{Kind: KindWords, Subject: 1, Expires: 9},
		{Kind: KindWords, Subject: 1, Expires: 9},
		{Kind: KindWords, Subject: 2, Expires: 9},
		{Kind: KindWords, Subject: 1, Expires: 9},
	}
	ts := one.AppendActiveAt(nil, long, 0)
	if want := []int{v, 0, v, 0}; ts[0].Value != want[0] || ts[1].Value != want[1] || ts[2].Value != want[2] || ts[3].Value != want[3] {
		t.Fatalf("per-subject cap at limit 1: %+v, want values %v", ts, want)
	}
	expired := Memories{{Kind: KindWords, Subject: 1, Expires: 3}, {Kind: KindWords, Subject: 1, Expires: 9}}
	if ts := one.AppendActiveAt(nil, expired, 5); len(ts) != 1 || ts[0].Value != v {
		t.Fatalf("an expired copy must not take the slot: %+v", ts)
	}
}

// The limit-1 repeat filter only skips a scan, never changes a value: two
// thousand memories (enough pairs to collide in the filter) in a fixed
// shuffled order, with repeats and expired copies, read as a plain
// earlier-copy scan says.
func TestStackLimitOneFilterMatchesScan(t *testing.T) {
	t.Parallel()
	kinds := []Kind{Grief, KindWords}
	one := patched(t, KindWords, func(tu *Tuning) { tu.StackLimit = 1 })
	var m Memories
	for i := range 2000 {
		m = append(m, Memory{
			Kind:    kinds[i%len(kinds)],
			Subject: uint64(i*7919%331) + 1<<40*uint64(i/1000), // non-ascending, repeats
			Value:   -(i%9 + 1),
			Expires: uint64(5 + i%5), // read at 6: two in five expired
		})
	}
	ts := one.AppendActiveAt(nil, m, 6)
	j, repeats := 0, 0
	for i, x := range m {
		if x.Expires <= 6 {
			continue
		}
		want := x.Value
		for _, y := range m[:i] {
			if y.Kind == x.Kind && y.Subject == x.Subject && y.Expires > 6 {
				want, repeats = 0, repeats+1
				break
			}
		}
		if ts[j].Value != want {
			t.Fatalf("memory %d %+v reads %d, want %d", i, x, ts[j].Value, want)
		}
		j++
	}
	if j != len(ts) || repeats < 100 {
		t.Fatalf("%d thoughts for %d live memories, %d repeats", len(ts), j, repeats)
	}
}

// A weight that decays to zero keeps contributing nothing; only the first
// memory of a stack starts at full weight.
func TestStackWeightDecayedToZeroStaysZero(t *testing.T) {
	t.Parallel()
	b := patched(t, KindWords, func(tu *Tuning) { tu.StackLimit, tu.StackFactor = 20, 1 })
	v := b.DefOf(KindWords).Value
	var m Memories
	for i := range 8 {
		m = append(m, Memory{Kind: KindWords, Subject: uint64(i + 1), Expires: 9})
	}
	ts := b.Active(m)
	sum := 0
	for i, x := range ts {
		sum += x.Value
		if i > 0 && x.Value != 0 {
			t.Fatalf("memory %d restarted at full weight: %+v", i, ts)
		}
	}
	if ts[0].Value != v || sum != v {
		t.Fatalf("stack total %d (first %d), want %d", sum, ts[0].Value, v)
	}
}

func TestStatesOnlyFastPathMatchesSlowPath(t *testing.T) {
	t.Parallel()
	states := []Kind{Hungry, Tired, Imprisoned, Cold, Bored}
	b := patched(t, Tired, func(tu *Tuning) { tu.Value, tu.Label = -8, "worn out" })
	for _, book := range []Book{cb, b} {
		fast := book.AppendActiveAt(nil, nil, 10, states...)
		slow := book.AppendActiveAt(nil, Memories{{Kind: AteRaw, Expires: 5}}, 10, states...) // expired: states only
		if !reflect.DeepEqual(fast, slow) || len(fast) != len(states) {
			t.Fatalf("fast %+v slow %+v", fast, slow)
		}
		if got := book.Active(nil, states...); !reflect.DeepEqual(got, fast) {
			t.Fatalf("Active %+v", got)
		}
		for i, k := range states {
			if fast[i].Memory || fast[i].Kind != k || fast[i].Value != book.DefOf(k).Value {
				t.Fatalf("state %d: %+v", i, fast[i])
			}
		}
	}
	if got := cb.Active(nil); len(got) != 0 {
		t.Fatalf("no states: %+v", got)
	}
}

func TestStatesOnlyDoesNotAllocate(t *testing.T) {
	dst := make([]Thought, 0, 5)
	states := []Kind{Hungry, Tired, Cold, Bored, Imprisoned}
	if n := testing.AllocsPerRun(100, func() { benchThoughts = cb.AppendActiveAt(dst[:0], nil, 1000, states...) }); n != 0 {
		t.Fatalf("%v allocs/op", n)
	}
	if n := testing.AllocsPerRun(100, func() { _ = Core() }); n != 0 {
		t.Fatalf("Core allocates %v", n)
	}
}

func TestBookKeys(t *testing.T) {
	t.Parallel()
	keys := cb.Keys()
	if len(keys) != 114 {
		t.Fatalf("%d keys", len(keys))
	}
	seen := map[string]bool{}
	for k, key := range keys {
		if seen[key] || key != cb.Key(Kind(k)) {
			t.Fatalf("key %q duplicated or inconsistent", key)
		}
		seen[key] = true
		if got, ok := cb.KindOf(key); !ok || got != Kind(k) {
			t.Fatalf("KindOf(%q) = %d %v", key, got, ok)
		}
	}
	for _, bad := range []string{"core.nope", "hungry", "mod.hungry", "", "core."} {
		if _, ok := cb.KindOf(bad); ok {
			t.Fatalf("KindOf(%q) found a kind", bad)
		}
	}
}

func TestCodeValuedList(t *testing.T) {
	t.Parallel()
	want := []Kind{
		Malnourished, PartnerOpinion, Grief, AugmentFan, NaturalPurist, Difficulty, LovesWork, TaintedApparel, SharedBed,
		ComfortMood, OutdoorsMood, BeautyMood,
		SleptInBedroom, SleptInBarracks, SleptInHospital, AteInDiningRoom, EnjoyedRecRoom,
		ExpectationsExtremelyLow, ExpectationsVeryLow, ExpectationsLow, ExpectationsModerate, ExpectationsHigh, ExpectationsSkyHigh,
	}
	var got []Kind
	for k := Kind(0); k <= numKinds; k++ {
		if CodeValued(k) {
			got = append(got, k)
		}
	}
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("code-valued kinds %v, want %v", got, want)
	}
}
