package research

import "testing"

func TestTreeAndUnlocks(t *testing.T) {
	t.Parallel()
	var pr Progress
	if !pr.Available(Traps) || pr.Available(Gunsmithing) {
		t.Fatal("gunsmithing needs traps first")
	}
	if p, ok := pr.Locks("build:spike trap"); !ok || p != Traps {
		t.Fatal("traps lock spike traps")
	}
	if _, ok := pr.Locks("build:wall"); ok {
		t.Fatal("walls are never locked")
	}
	if pr.Add(Traps, 100) || pr.Done(Traps) {
		t.Fatal("not finished yet")
	}
	if !pr.Add(Traps, DefOf(Traps).Cost) || !pr.Done(Traps) || pr[Traps] != DefOf(Traps).Cost {
		t.Fatal("finishing caps at the cost")
	}
	if _, ok := pr.Locks("build:spike trap"); ok || !pr.Available(Gunsmithing) {
		t.Fatal("traps researched: spike traps open, gunsmithing available")
	}
	full := Complete()
	for _, d := range All() {
		if !full.Done(d.Project) {
			t.Fatalf("%s not complete", d.Name)
		}
	}
	if p, ok := Parse("herbalism"); !ok || p != Herbalism {
		t.Fatal("parse")
	}
}
