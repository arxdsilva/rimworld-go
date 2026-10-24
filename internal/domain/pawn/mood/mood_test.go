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

func TestAddForStacksAndRefreshesOldest(t *testing.T) {
	t.Parallel()
	var m Memories
	lim := int(DefOf(Insulted).StackLimit)
	for i := 0; i < lim; i++ {
		m = m.AddFor(Insulted, 7, uint64(100+i))
	}
	if len(m) != lim {
		t.Fatalf("len %d want %d", len(m), lim)
	}
	m = m.AddFor(Insulted, 7, 9000)
	if len(m) != lim || m[0].Expires != 9000+DefOf(Insulted).Duration {
		t.Fatalf("oldest not refreshed: %+v", m)
	}
	m = m.AddFor(Insulted, 8, 10) // other subject stacks separately
	if len(m) != lim+1 {
		t.Fatalf("other subject: %d", len(m))
	}
	// non-stacking kind refreshes
	m = Memories(nil).AddFor(CheatedOn, 1, 5).AddFor(CheatedOn, 1, 50)
	if len(m) != 1 || m[0].Expires != 50+DefOf(CheatedOn).Duration {
		t.Fatalf("%+v", m)
	}
}

func TestAddForGroupReplacement(t *testing.T) {
	t.Parallel()
	m := Memories(nil).AddFor(SleptOnGround, 0, 1).AddFor(SleptInBed, 0, 2)
	if len(m) != 1 || m[0].Kind != SleptInBed {
		t.Fatalf("%+v", m)
	}
}

func TestAddValueAndDiminishingActive(t *testing.T) {
	t.Parallel()
	m := Memories(nil).AddValue(Grief, 3, -20, 0)
	if m[0].Value != -20 {
		t.Fatalf("%+v", m)
	}
	m = m.AddValue(Grief, 3, -30, 5) // limit 1: replaced
	if len(m) != 1 || m[0].Value != -30 {
		t.Fatalf("%+v", m)
	}
	var s Memories
	for i := 0; i < 3; i++ {
		s = s.AddFor(KindWords, uint64(i+1), 0)
	}
	ts := Active(s)
	if ts[0].Value != 5 || ts[1].Value != 4 || ts[2].Value != 3 {
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
	m = m.AddLabeled(Grief, 7, -20, "my wife died", 0)
	m = m.AddValue(Grief, 8, -10, 0)
	ts := Active(m)
	if len(ts) != 2 || ts[0].Label != "my wife died" || ts[0].Value != -20 {
		t.Fatalf("labelled grief %+v", ts)
	}
	if ts[1].Label != DefOf(Grief).Label || ts[1].Value != -10 {
		t.Fatalf("unlabelled grief keeps the def's line: %+v", ts[1])
	}
	m = m.AddLabeled(Grief, 7, -16, "my lover died", 5)
	if ts := Active(m); len(ts) != 2 || ts[0].Label != "my lover died" || ts[0].Value != -16 {
		t.Fatalf("refreshed grief %+v", ts)
	}
}
