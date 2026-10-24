package mood

import "testing"

func TestMemoriesAndLevel(t *testing.T) {
	t.Parallel()
	var m Memories
	m = m.Add(AteRaw, 100)
	m = m.Add(AteRaw, 200) // refresh, not duplicate
	m = m.Add(SleptInNiceRoom, 100)
	if len(m) != 2 || m[0].Expires != 200+DefOf(AteRaw).Duration {
		t.Fatalf("memories = %+v", m)
	}
	ts := Active(m, Hungry)
	want := Base + DefOf(AteRaw).Value + DefOf(SleptInNiceRoom).Value + DefOf(Hungry).Value
	if Level(ts) != want || len(ts) != 3 {
		t.Fatalf("level = %d want %d", Level(ts), want)
	}
	m = m.Prune(100 + DefOf(SleptInNiceRoom).Duration)
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
	m = m.Add(AteWithoutTable, 1).Add(AteRaw, 1)
	m = m.Add(AteAtTable, 2).Add(AteMeal, 2)
	if len(m) != 2 {
		t.Fatalf("memories = %+v", m)
	}
	for _, x := range m {
		if x.Kind == AteWithoutTable || x.Kind == AteRaw {
			t.Fatal("newer meal replaces the opposite memory")
		}
	}
	m = m.Add(SleptInNiceRoom, 3).ForgetGroupOf(SleptInAwfulRoom)
	if len(m) != 2 {
		t.Fatal("forget drops the room-quality group only")
	}
}

func TestDeathAndBurialThoughts(t *testing.T) {
	t.Parallel()
	// ENG-003 balance override: -3 for six days.
	if d := DefOf(ColonistDied); d.Value != -3 || d.Duration != 6*day {
		t.Fatalf("colonist died: %+v", d)
	}
	if d := DefOf(UnburiedCorpse); d.Value != -2 || d.Duration != 0 {
		t.Fatalf("unburied corpse is a state thought: %+v", d)
	}
	var m Memories
	m = m.Add(LaidToRest, 0)
	m = m.Add(LaidToRest, 10)
	if len(m) != 1 || Level(Active(m)) != Base+3 || m[0].Expires != 10+day {
		t.Fatalf("laid to rest: +3 for a day, never stacked: %+v", m)
	}
}

// TestSocialThoughtDefs (ENG-003): every kind has a label, stacking
// defaults to one memory at full weight, the "stack" kinds stack five
// deep at 900 permille, and a memory's Value overrides its def's.
func TestSocialThoughtDefs(t *testing.T) {
	t.Parallel()
	for k := Kind(0); k < numKinds; k++ {
		if DefOf(k).Label == "" {
			t.Fatalf("kind %d has no label", k)
		}
	}
	if d := DefOf(SleptInBed); d.StackLimit != 1 || d.StackFactor != 1000 {
		t.Fatalf("default stacking: %+v", d)
	}
	for _, k := range []Kind{Insulted, KindWords, Lovin, BondedDied, BondedLost, BondedGivenAway, BondedReleased} {
		if d := DefOf(k); d.StackLimit != 5 || d.StackFactor != 900 {
			t.Fatalf("%s stacking: %+v", d.Label, d)
		}
	}
	if d := DefOf(GotMarried); d.Value != 40 || d.Duration != 30*day {
		t.Fatalf("got married: %+v", d)
	}
	if d := DefOf(Grief); d.Value != 0 || d.Duration != 0 {
		t.Fatalf("grief: %+v", d)
	}
	m := Memories{{Kind: Grief, Expires: 10, Subject: 7, Value: -12}, {Kind: Catharsis, Expires: 10}}
	if got := Level(Active(m)); got != Base-12+30 {
		t.Fatalf("Value override: level %d", got)
	}
	if DefOf(numKinds).Label != "" {
		t.Fatal("out of range")
	}
}
