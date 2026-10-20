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
	if d := DefOf(ColonistDied); d.Value != -6 || d.Duration != 3*day {
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
