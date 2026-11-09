package sched

import (
	"strings"
	"testing"
	"time"
)

// names is the schedule's run order.
func names(s *Schedule) string {
	var out []string
	for _, st := range s.Snapshot() {
		out = append(out, st.Name)
	}
	return strings.Join(out, " ")
}

// own is a system with a store of its own: it conflicts with nothing.
func own(name, phase string, id StoreID, run func()) System {
	return System{Name: name, Phase: phase, Writes: []StoreID{id}, Run: run}
}

func nop() {}

// Ties are broken by phase rank, then name, whatever the table's order.
func TestTiesByPhaseRankThenName(t *testing.T) {
	s := Must([]string{"first", "second"}, []System{
		own("b", "second", 1, nop), own("z", "first", 2, nop), own("a", "second", 3, nop), own("m", "first", 4, nop),
	})
	if got, want := names(s), "m z a b"; got != want {
		t.Fatalf("order %q, want %q", got, want)
	}
}

// Edges win over the tie-break, both ways, and the wrapped phases need
// them: Legacy conflicts with everything in its phase.
func TestEdgesOrderAndLegacyNeedsThem(t *testing.T) {
	s := Must([]string{"p"}, []System{
		Wrap("a", "p", nop), Wrap("b", "p", nop).Then("a"), Wrap("c", "p", nop).Then("b"),
	})
	if got, want := names(s), "c b a"; got != want {
		t.Fatalf("order %q, want %q", got, want)
	}
	after := Wrap("b", "p", nop)
	after.After = []string{"a"}
	if got := names(Must([]string{"p"}, []System{after, Wrap("a", "p", nop)})); got != "a b" {
		t.Fatalf("After: %q", got)
	}
	if _, err := New([]string{"p"}, []System{Wrap("a", "p", nop), Wrap("b", "p", nop)}); err == nil || !strings.Contains(err.Error(), "share store") {
		t.Fatalf("two unordered legacy systems: %v", err)
	}
	amb := Wrap("b", "p", nop)
	amb.Ambiguous = []string{"a"}
	if _, err := New([]string{"p"}, []System{Wrap("a", "p", nop), amb}); err != nil {
		t.Fatalf("declared ambiguous: %v", err)
	}
	if _, err := New([]string{"p", "q"}, []System{Wrap("a", "p", nop), Wrap("b", "q", nop)}); err != nil {
		t.Fatalf("different phases need no edge: %v", err)
	}
	reader := System{Name: "r", Phase: "p", Reads: []StoreID{1}, Run: nop}
	if _, err := New([]string{"p"}, []System{own("w", "p", 1, nop), reader}); err == nil {
		t.Fatal("a reader and a writer of one store, unordered")
	}
	if _, err := New([]string{"p"}, []System{reader, {Name: "r2", Phase: "p", Reads: []StoreID{1}, Run: nop}}); err != nil {
		t.Fatalf("two readers: %v", err)
	}
}

// A cycle is an error that names its chain.
func TestCycleNamesTheChain(t *testing.T) {
	_, err := New([]string{"p"}, []System{
		own("a", "p", 1, nop).Then("b"), own("b", "p", 2, nop).Then("c"), own("c", "p", 3, nop).Then("a"), own("d", "p", 4, nop),
	})
	if err == nil || !strings.Contains(err.Error(), "a -> b -> c -> a") {
		t.Fatalf("cycle error %v", err)
	}
}

func TestBadDescriptors(t *testing.T) {
	for name, sys := range map[string][]System{
		"unknown phase": {own("a", "nowhere", 1, nop)},
		"twice":         {own("a", "p", 1, nop), own("a", "p", 2, nop)},
		"no name":       {own("", "p", 1, nop)},
		"no run":        {own("a", "p", 1, nil)},
		"unknown edge":  {own("a", "p", 1, nop).Then("b")},
	} {
		if _, err := New([]string{"p"}, sys); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

// RunIf, Every with Offset, and Scope gate each run; nothing else does.
func TestGates(t *testing.T) {
	runs := map[string]int{}
	count := func(n string) func() { return func() { runs[n]++ } }
	on := false
	s := Must([]string{"p"}, []System{
		own("always", "p", 1, count("always")),
		own("if", "p", 2, count("if")).When(func() bool { return on }),
		own("every", "p", 3, count("every")).At(10, 7), // ticks 3, 13, 23, ...
		own("colony", "p", 4, count("colony")).ForColony(),
	})
	var due []uint64
	for tick := range uint64(30) {
		on = tick%2 == 0
		before := runs["every"]
		s.Pass(tick%3 == 0, tick)
		if runs["every"] > before {
			due = append(due, tick)
		}
	}
	if runs["always"] != 30 || runs["if"] != 15 || runs["colony"] != 10 || len(due) != 3 || due[0] != 3 || due[2] != 23 {
		t.Fatalf("runs %v, every ran at %v", runs, due)
	}
}

// The observer hears each phase once a pass, after its last system, with
// the phase's time; Snapshot counts only the systems that ran, and only
// while observed.
func TestObserverAndSnapshot(t *testing.T) {
	var heard []string
	s := Must([]string{"p", "q"}, []System{
		own("a", "p", 1, nop), own("b", "p", 2, nop).At(2, 0), own("c", "q", 3, nop).ForColony(),
	})
	s.Pass(true, 0) // not observed
	s.Observe(func(phase string, d time.Duration) {
		if d < 0 {
			t.Fatal("negative time")
		}
		heard = append(heard, phase)
	})
	for tick := range uint64(4) {
		s.Pass(false, tick)
	}
	s.Observe(nil)
	s.Pass(true, 0)
	if got := strings.Join(heard, " "); got != "p q p q p q p q" {
		t.Fatalf("heard %q", got)
	}
	snap := s.Snapshot()
	if snap[0].Runs != 4 || snap[1].Runs != 2 || snap[2].Runs != 0 || snap[1].Every != 2 || snap[2].Scope != Colony {
		t.Fatalf("snapshot %+v", snap)
	}
	if snap[0].Max < snap[0].Mean {
		t.Fatalf("max %v below mean %v", snap[0].Max, snap[0].Mean)
	}
}

// A pass allocates nothing, observed or not.
func TestPassAllocatesNothing(t *testing.T) {
	n := 0
	s := Must([]string{"p"}, []System{
		own("a", "p", 1, func() { n++ }).When(func() bool { return n >= 0 }), own("b", "p", 2, func() { n++ }).At(3, 1).ForColony(),
	})
	tick := uint64(0)
	if a := testing.AllocsPerRun(100, func() { s.Pass(true, tick); tick++ }); a != 0 {
		t.Fatalf("%v allocs a pass", a)
	}
	s.Observe(func(string, time.Duration) {})
	if a := testing.AllocsPerRun(100, func() { s.Pass(false, tick); tick++ }); a != 0 {
		t.Fatalf("%v allocs an observed pass", a)
	}
}

func BenchmarkPass(b *testing.B) {
	sys := make([]System, 0, 24)
	prev := ""
	for i := range 24 {
		y := Wrap(string(rune('a'+i)), "p", nop)
		if prev != "" {
			y.After = []string{prev}
		}
		sys, prev = append(sys, y), y.Name
	}
	s := Must([]string{"p"}, sys)
	b.ReportAllocs()
	for i := range b.N {
		s.Pass(true, uint64(i))
	}
}
