// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"rimworld-go/internal/application/game/cadence"
	"rimworld-go/internal/application/scripting"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/pawngen"
	"rimworld-go/internal/domain/sim/entity"
)

// bothHooks is a fake's cadence hooks: on_pawns and on_pawn.
func bothHooks() []scripting.Hook { return []scripting.Hook{scripting.OnPawns, scripting.OnPawn} }

// cadenceEvents is f's on_pawns and on_pawn events, in order.
func cadenceEvents(f *fakeScripts) []scripting.Event {
	var out []scripting.Event
	for _, ev := range f.events {
		if ev.Hook == scripting.OnPawns || ev.Hook == scripting.OnPawn {
			out = append(out, ev)
		}
	}
	return out
}

// dueScan is the due set by a scan: every home colonist cadence.Due at now.
func dueScan(g *Game, now uint64) []entity.EntityID {
	var out []entity.EntityID
	for _, id := range g.homeColonists() {
		if cadence.Due(now, scripting.PawnEvery, cadence.OffPawnScripts, id) {
			out = append(out, id)
		}
	}
	return out
}

func pawnIDsOf(ps []scripting.Pawn) []entity.EntityID {
	var out []entity.EntityID
	for _, p := range ps {
		out = append(out, entity.EntityID(p.ID))
	}
	return out
}

// pawnScriptRuns is how often the pawn-scripts system ran (an observer
// must be set for the scheduler to count).
func pawnScriptRuns(g *Game) uint64 {
	for _, st := range g.systems.Snapshot() {
		if st.Name == "pawn-scripts" {
			return st.Runs
		}
	}
	return 0
}

// TestPawnScriptsEachColonistOncePer60Ticks: over 60 ticks each living
// home colonist is due exactly once, on its cadence tick, ascending in
// both hooks' events; still so after a spawn, a death, an enlist, a
// caravan leaving and one coming back, and with a downed colonist.
func TestPawnScriptsEachColonistOncePer60Ticks(t *testing.T) {
	t.Parallel()
	f := &fakeScripts{uses: bothHooks()}
	g := New(Config{Seed: 3, Width: 32, Height: 32, Scripts: f})
	c := spawnColonists(t, g, 130, 0)
	caravanStash(t, g, c[3]) // away before the first window
	window := func(stage string, must, never []entity.EntityID) map[entity.EntityID]scripting.Pawn {
		t.Helper()
		want := g.homeColonists()
		f.events = nil
		g.Step(scripting.PawnEvery)
		seen := map[entity.EntityID]scripting.Pawn{}
		pawns, pawn := f.pawnEvents(scripting.OnPawns), f.pawnEvents(scripting.OnPawn)
		if !reflect.DeepEqual(pawns, pawn) {
			t.Fatalf("%s: on_pawns %v, on_pawn %v", stage, pawns, pawn)
		}
		for _, ev := range cadenceEvents(f) {
			ids := pawnIDsOf(ev.Pawns)
			if !slices.IsSorted(ids) || len(ids) == 0 {
				t.Fatalf("%s: tick %d: %v", stage, ev.Tick, ids)
			}
			for i, id := range ids {
				if !cadence.Due(ev.Tick, scripting.PawnEvery, cadence.OffPawnScripts, id) {
					t.Fatalf("%s: %d not due at %d", stage, id, ev.Tick)
				}
				if _, dup := seen[id]; dup && ev.Hook == scripting.OnPawns {
					t.Fatalf("%s: %d twice", stage, id)
				}
				seen[id] = ev.Pawns[i]
			}
		}
		if len(seen) != len(want) {
			t.Fatalf("%s: %d colonists due, want %d", stage, len(seen), len(want))
		}
		for _, id := range append(slices.Clone(want), must...) {
			if _, ok := seen[id]; !ok {
				t.Fatalf("%s: %d never due", stage, id)
			}
		}
		for _, id := range never {
			if _, ok := seen[id]; ok {
				t.Fatalf("%s: %d due", stage, id)
			}
		}
		return seen
	}
	window("first", c[:3], c[3:4])
	g.Step(17) // mid-cycle
	born := spawnColonists(t, g, 1, 31)[0]
	g.die(c[0], health.CauseInjury)
	recruit := spawnRaiderAt(t, g, 31, 31, 0)
	g.makePrisoner(recruit, c[1])
	g.enlist(recruit)
	caravanStash(t, g, c[2])
	if !g.unstashPawn(c[3], 30, 30) {
		t.Fatal("unstash")
	}
	setDowned(g, c[4])
	seen := window("after", []entity.EntityID{born, recruit, c[3], c[4]}, []entity.EntityID{c[0], c[2]})
	if !seen[c[4]].Downed {
		t.Fatalf("the downed colonist: %+v", seen[c[4]])
	}
}

// TestPawnScriptsIndexMatchesScan: on every tick of a scenario of
// arrivals and departures the index's due set is the scan's.
func TestPawnScriptsIndexMatchesScan(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 7, Width: 40, Height: 40, Scripts: &fakeScripts{uses: bothHooks()}})
	c := spawnColonists(t, g, 40, 1)
	var wanderers []entity.EntityID
	for tick := range 200 {
		switch tick {
		case 5:
			spawnColonists(t, g, 3, 36)
		case 20:
			if wanderers, _ = walkInWanderers(g, []uint64{101, 202}, pawngen.Rules{}); len(wanderers) == 0 {
				t.Fatal("fixture: no wanderers walked in")
			}
		case 40:
			r := spawnRaiderAt(t, g, 39, 39, 0)
			g.makePrisoner(r, c[0])
			g.enlist(r)
		case 60:
			g.makePrisoner(c[1], c[0]) // arrested, still on the map
		case 80:
			caravanStash(t, g, c[2])
			caravanStash(t, g, wanderers[0])
		case 100:
			if !g.unstashPawn(c[2], 38, 38) {
				t.Fatal("unstash")
			}
		case 120:
			g.die(c[3], health.CauseInjury)
		case 140:
			setDowned(g, c[4])
		}
		g.Step(1)
		now := uint64(g.clock.Now())
		due, _ := duePawns(g, now)
		if got, want := pawnIDsOf(due), dueScan(g, now); !slices.Equal(got, want) {
			t.Fatalf("tick %d: index %v, scan %v", tick, got, want)
		}
	}
	if !slices.Contains(g.homeColonists(), wanderers[1]) || slices.Contains(g.homeColonists(), c[1]) {
		t.Fatal("fixture: the scenario did not run")
	}
}

// TestPawnScriptsSaveLoadSameVisits: arrivals noted after the index was
// built (a spawn, an enlist, a caravan's return) and an arrest are seen
// the same by a continuous run and by one saved mid-cycle and loaded.
func TestPawnScriptsSaveLoadSameVisits(t *testing.T) {
	t.Parallel()
	f := &fakeScripts{uses: bothHooks()}
	g := New(Config{Seed: 5, Width: 32, Height: 32, Scripts: f})
	c := spawnColonists(t, g, 12, 2)
	g.makePrisoner(c[0], c[1]) // a prisoner before the build, enlisted after
	if !g.stashPawn(c[2], g.entities.Next()) {
		t.Fatal("stash")
	}
	g.Step(1)
	if !g.scripts.pawns.From(g.maps[0]) || slices.Contains(g.scripts.pawns.Due(cadence.Next(0, scripting.PawnEvery, cadence.OffPawnScripts, c[0]), cadence.OffPawnScripts), c[0]) {
		t.Fatal("fixture: the index is not built, or holds the prisoner")
	}
	g.Step(23)
	born := spawnColonists(t, g, 1, 20)[0]
	g.enlist(c[0])
	if !g.unstashPawn(c[2], 20, 25) {
		t.Fatal("unstash")
	}
	g.makePrisoner(c[3], c[1])
	g.Step(7) // mid-cycle
	h := saveLoad(t, g)
	hf := &fakeScripts{uses: bothHooks()}
	UseScripts(h, hf)
	f.events = nil
	g.Step(2 * scripting.PawnEvery)
	h.Step(2 * scripting.PawnEvery)
	a, b := cadenceEvents(f), cadenceEvents(hf)
	if len(a) == 0 || !reflect.DeepEqual(a, b) {
		t.Fatalf("continuous %d events, loaded %d; differ", len(a), len(b))
	}
	if g.StateHash() != h.StateHash() {
		t.Fatal("state hashes differ")
	}
	seen := f.pawnEvents(scripting.OnPawns)
	var all []entity.EntityID
	for _, ids := range seen {
		all = append(all, ids...)
	}
	for _, id := range []entity.EntityID{born, c[0], c[2]} {
		if !slices.Contains(all, id) {
			t.Fatalf("arrival %d never due", id)
		}
	}
	if slices.Contains(all, c[3]) {
		t.Fatalf("the arrested %d is due", c[3])
	}
}

// TestPawnScriptsQuietWhenUnused: no cadence hook in use, no event and no
// index; no host, the system never runs; nobody due, no Fire; one hook
// in use, only that one fires.
func TestPawnScriptsQuietWhenUnused(t *testing.T) {
	t.Parallel()
	f := &fakeScripts{}
	g := New(Config{Seed: 3, Width: 24, Height: 24, Scripts: f})
	spawnColonists(t, g, 10, 1)
	g.SetPhaseObserver(func(string, time.Duration) {})
	g.Step(2 * scripting.PawnEvery)
	scriptPawns(g, uint64(g.clock.Now()))
	if n := len(cadenceEvents(f)); n != 0 || g.scripts.pawns.From(g.maps[0]) || pawnScriptRuns(g) != 0 {
		t.Fatalf("unused: %d events, index built %v, runs %d", n, g.scripts.pawns.From(g.maps[0]), pawnScriptRuns(g))
	}
	bare := New(Config{Seed: 3, Width: 16, Height: 16})
	spawnColonists(t, bare, 3, 1)
	bare.SetPhaseObserver(func(string, time.Duration) {})
	bare.Step(scripting.PawnEvery)
	if pawnScriptRuns(bare) != 0 || bare.scripts.pawns.From(bare.maps[0]) {
		t.Fatal("no host: the system ran")
	}
	empty := &fakeScripts{uses: bothHooks()}
	none := New(Config{Seed: 3, Width: 16, Height: 16, Scripts: empty})
	none.SetPhaseObserver(func(string, time.Duration) {})
	none.Step(scripting.PawnEvery)
	if n := len(cadenceEvents(empty)); n != 0 || pawnScriptRuns(none) != scripting.PawnEvery {
		t.Fatalf("no colonists: %d events, %d runs", n, pawnScriptRuns(none))
	}
	f.uses = []scripting.Hook{scripting.OnPawn}
	g.Step(scripting.PawnEvery)
	for _, ev := range cadenceEvents(f) {
		if ev.Hook != scripting.OnPawn {
			t.Fatalf("on_pawns fired unused: %+v", ev)
		}
	}
	if len(cadenceEvents(f)) == 0 {
		t.Fatal("on_pawn never fired")
	}
}

// TestPawnScriptsVisitsOnlyTheBucket: a tick inspects only its bucket's
// ids (at most ceil(N/60)), and a period's ticks inspect each id once.
func TestPawnScriptsVisitsOnlyTheBucket(t *testing.T) {
	t.Parallel()
	for _, n := range []int{60, 600} {
		g := New(Config{Seed: 3, Width: 40, Height: 40, Scripts: &fakeScripts{uses: bothHooks()}})
		spawnColonists(t, g, n, 0)
		now, sum, most := uint64(g.clock.Now()), 0, (n+scripting.PawnEvery-1)/scripting.PawnEvery
		for tick := now; tick < now+scripting.PawnEvery; tick++ {
			due, visited := duePawns(g, tick)
			if b := len(g.scripts.pawns.Due(tick, cadence.OffPawnScripts)); visited != b || visited > most || len(due) != visited {
				t.Fatalf("n=%d tick %d: visited %d, bucket %d, due %d, at most %d", n, tick, visited, b, len(due), most)
			}
			sum += visited
		}
		if sum != n {
			t.Fatalf("n=%d: visited %d in a period", n, sum)
		}
	}
}

// countScripts is a host whose scripts define the cadence hooks and do
// nothing: it counts the colonists it is shown.
type countScripts struct{ seen int }

func (c *countScripts) Fire(ev scripting.Event, _ scripting.World, _ scripting.PawnReader) scripting.Result {
	c.seen += len(ev.Pawns)
	return scripting.Result{}
}
func (c *countScripts) Uses(h scripting.Hook) bool {
	return h == scripting.OnPawns || h == scripting.OnPawn
}

// BenchmarkPawnScriptsTick: one op is a period (60 ticks) of the cadence
// path, at 60 and 600 colonists: the cost follows N/60, not the entities.
func BenchmarkPawnScriptsTick(b *testing.B) {
	for _, n := range []int{60, 600} {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			h := &countScripts{}
			g := New(Config{Seed: 3, Width: 40, Height: 40, Scripts: h})
			spawnColonists(b, g, n, 0)
			now := uint64(g.clock.Now())
			b.ReportAllocs()
			for b.Loop() {
				for tick := now; tick < now+scripting.PawnEvery; tick++ {
					scriptPawns(g, tick)
				}
			}
			if h.seen == 0 {
				b.Fatal("no colonist was due")
			}
		})
	}
}

// TestPawnScriptsCalendarRow: the cadence's calendar row is the port's
// period at the cadence offset.
func TestPawnScriptsCalendarRow(t *testing.T) {
	t.Parallel()
	for _, s := range cadence.Calendar() {
		if s.Name == "pawn-scripts" {
			if s.Every != scripting.PawnEvery || s.Offset != cadence.OffPawnScripts || !s.PerEntity {
				t.Fatalf("%+v", s)
			}
			return
		}
	}
	t.Fatal("no pawn-scripts row")
}

// pawnIDsGrowth lists the calls in src that grow a pawnIDs list (append,
// withID or slices.Insert with a pawnIDs first argument, on any
// receiver) outside addPawnID, as "file:line".
func pawnIDsGrowth(fset *token.FileSet, name, src string) ([]string, error) {
	f, err := parser.ParseFile(fset, name, src, 0)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, d := range f.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok && fn.Name.Name == "addPawnID" && fn.Recv == nil {
			continue
		}
		ast.Inspect(d, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 || !growCall(call.Fun) {
				return true
			}
			if pawnIDsExpr(call.Args[0]) {
				out = append(out, fset.Position(call.Pos()).String())
			}
			return true
		})
	}
	return out, nil
}

// growCall: append, withID or slices.Insert.
func growCall(fun ast.Expr) bool {
	switch f := fun.(type) {
	case *ast.Ident:
		return f.Name == "append" || f.Name == "withID"
	case *ast.SelectorExpr:
		x, ok := f.X.(*ast.Ident)
		return ok && x.Name == "slices" && f.Sel.Name == "Insert"
	}
	return false
}

// pawnIDsExpr: x.pawnIDs (any x) or a bare pawnIDs.
func pawnIDsExpr(e ast.Expr) bool {
	switch x := e.(type) {
	case *ast.SelectorExpr:
		return x.Sel.Name == "pawnIDs"
	case *ast.Ident:
		return x.Name == "pawnIDs"
	}
	return false
}

// TestPawnIDsGrowOnlyThroughAddPawnID: no non-test file of the package
// grows a pawnIDs list but addPawnID, which notes the arrival for the
// scripts' cadence index (the visible-set invariant).
func TestPawnIDsGrowOnlyThroughAddPawnID(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var bad []string
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		found, err := pawnIDsGrowth(fset, name, string(src))
		if err != nil {
			t.Fatal(err)
		}
		bad = append(bad, found...)
	}
	if len(bad) > 0 {
		t.Fatalf("pawnIDs grows outside addPawnID (scripts.go), so the scripts' cadence misses the arrival: %v", bad)
	}
}

// TestPawnIDsGrowthCheckSeesEachForm: the check catches every way a
// pawnIDs list is grown, and lets reads and addPawnID through.
func TestPawnIDsGrowthCheckSeesEachForm(t *testing.T) {
	t.Parallel()
	for src, want := range map[string]int{
		"g.pawnIDs = append(g.pawnIDs, 1)":                               1,
		"a, g.pawnIDs = x, append(g.pawnIDs, id)":                        1,
		"m.pawnIDs = withID(m.pawnIDs, id)":                              1,
		"g.raiderIDs, g.pawnIDs = withoutID(r, p), withID(g.pawnIDs, p)": 1,
		"pawnIDs = append(pawnIDs, id)":                                  1,
		"s.pawnIDs = slices.Insert(s.pawnIDs, 0, id)":                    1,
		"on = append(on, m.pawnIDs...)":                                  0,
		"g.raiderIDs = withID(g.raiderIDs, id)":                          0,
		"g.pawnIDs = withoutID(g.pawnIDs, id)":                           0,
	} {
		file := "package game\nfunc f() {\n\t" + src + "\n}\nfunc addPawnID() { g.pawnIDs = append(g.pawnIDs, 1) }\n"
		got, err := pawnIDsGrowth(token.NewFileSet(), "x.go", file)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != want {
			t.Fatalf("%q: %v, want %d", src, got, want)
		}
	}
}
