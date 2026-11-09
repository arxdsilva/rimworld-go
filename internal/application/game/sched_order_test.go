package game

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"rimworld-go/internal/application/game/cadence"
)

// runPhase runs phase's systems (its lifted colony work and cadence
// gates included) for the current map at the current tick, as Step would.
func runPhase(g *Game, phase string) {
	g.systems.PassPhase(phase, primary(g), uint64(g.clock.Now()))
}

var updateOrder = flag.Bool("update", false, "rewrite sched/testdata/order.golden")

// TestPhaseOrderGolden: the phase systems' run order, scopes and cadence
// (phaseSystems, game/sched) are pinned in sched/testdata/order.golden,
// so a reorder shows as a diff (-update rewrites it). Each phase's
// systems run together, in Phases() order (the observer hears each once
// a pass), and every lifted cadence gate is its game/cadence Calendar row.
func TestPhaseOrderGolden(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 16, Height: 16})
	var b strings.Builder
	var phases []string
	cal := map[string]cadence.Site{}
	for _, s := range cadence.Calendar() {
		cal[s.Name] = s
	}
	row := map[string]string{"heat": "heat", "rot": "rot", "wear": "wear", PhaseClimate: "climate", PhaseMedical: "medical",
		"losses": "story", "raids-over": "story", "story": "story", "social-day": "social-day"}
	for _, st := range g.systems.Snapshot() {
		fmt.Fprintf(&b, "%-9s %-12s %-6s every %-5d offset %d\n", st.Phase, st.Name, st.Scope, st.Every, st.Offset)
		phases = append(phases, st.Phase)
		if site, ok := cal[row[st.Name]]; ok && (site.Every != st.Every || site.Offset != st.Offset) {
			t.Errorf("%s: every %d offset %d, the calendar's %s %d %d", st.Name, st.Every, st.Offset, site.Name, site.Every, site.Offset)
		}
	}
	if got := slices.Compact(phases); !slices.Equal(got, Phases()) {
		t.Fatalf("phases run as %v, want %v", got, Phases())
	}
	path := filepath.Join("sched", "testdata", "order.golden")
	if *updateOrder {
		if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if b.String() != string(want) {
		t.Fatalf("phase order changed (go test -run TestPhaseOrderGolden -update):\n%s\nwant:\n%s", b.String(), want)
	}
}
