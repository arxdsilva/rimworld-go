package wealth

import "testing"

type counter struct{ n, calls int }

func (c *counter) Wealth() int { c.calls++; return c.n }

func TestTrackerWrapsOneFullPass(t *testing.T) {
	c := &counter{n: 500}
	tr := New(c)
	if tr.Value() != 500 || c.calls != 1 {
		t.Fatalf("value %d after %d passes, want 500 after 1", tr.Value(), c.calls)
	}
	tr.Step(60)
	c.n = 900
	if tr.Value() != 500 {
		t.Fatalf("value %d, want the stepped 500 until the next Step", tr.Value())
	}
	tr.Step(120)
	if tr.Value() != 900 || len(tr.History()) != 0 {
		t.Fatalf("value %d, history %v: want 900 and no samples yet", tr.Value(), tr.History())
	}
}

func TestTrackerSaveRoundTrip(t *testing.T) {
	tr := New(&counter{n: 7})
	if tr.Export() != nil {
		t.Fatal("an empty tracker saves nil")
	}
	tr.Import(&Save{History: []Sample{{Tick: 10, Value: 3}}, Next: 70})
	got := tr.Export()
	if got == nil || len(got.History) != 1 || got.History[0] != (Sample{Tick: 10, Value: 3}) || got.Next != 70 {
		t.Fatalf("round trip %+v", got)
	}
	tr.Import(nil)
	if tr.Export() != nil || tr.Value() != 7 {
		t.Fatal("Import(nil) empties the tracker")
	}
}
