package wizard

import (
	"slices"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/scenario"
	"rimworld-go/internal/platform/config"
)

func poolNicks(p *game.CandidatePool) []string {
	var out []string
	for _, v := range p.Views() {
		out = append(out, v.Nick)
	}
	return out
}

func prefPool(t *testing.T, names []string) (*game.CandidatePool, *Draft, *Flow) {
	t.Helper()
	sh, w := newFakeShell(t)
	sh.settings.General.PreferredNames = names
	ng := w.Draft()
	ng.Scenario = scenario.Builtins()[0]
	return pawnsPage{w}.newPool(ng), ng, w
}

func TestPreferredNamesGoToTheFirstCandidatesInOrder(t *testing.T) {
	p, _, _ := prefPool(t, []string{"Zzyx", "Qwop Lindqvist", "Vlad 'Zork' Park"})
	vs := p.Views()
	if vs[0].Nick != "Zzyx" || vs[1].Nick != "Qwop" || vs[1].Last != "Lindqvist" || vs[2].Nick != "Zork" || vs[2].First != "Vlad" {
		t.Fatalf("names = %+v", poolNicks(p))
	}
}

func TestPreferredNamesSkipUsedAndAreDeterministic(t *testing.T) {
	a, ng, w := prefPool(t, []string{"Zzyx", "Zzyx", "Qwop", "9bad"})
	b := pawnsPage{w}.newPool(ng)
	if !slices.Equal(poolNicks(a), poolNicks(b)) {
		t.Fatalf("not deterministic: %v vs %v", poolNicks(a), poolNicks(b))
	}
	n := poolNicks(a)
	if n[0] != "Zzyx" || n[1] != "Qwop" {
		t.Fatalf("duplicate and invalid names not skipped: %v", n)
	}
}

func TestPreferredNamesChangeRebuildsThePool(t *testing.T) {
	sh, w := newFakeShell(t)
	ng := w.Draft()
	ng.Scenario = scenario.Builtins()[0]
	k1 := pawnsPage{w}.poolKeyFor(ng)
	sh.settings.General.PreferredNames = []string{"Zed"}
	if k2 := (pawnsPage{w}).poolKeyFor(ng); k1 == k2 {
		t.Fatal("pool key ignores the preferred names")
	}
}

func TestTestMapSizesNeedDevModeAndTheOption(t *testing.T) {
	s := config.Settings{}
	base := len(MapSizeOptions())
	if len(MapSizeOptionsFor(s)) != base {
		t.Fatal("test sizes listed by default")
	}
	s.Developer.TestMapSizes = true
	if len(MapSizeOptionsFor(s)) != base {
		t.Fatal("test sizes listed without developer mode")
	}
	s.Gameplay.DevMode = true
	got := MapSizeOptionsFor(s)
	if len(got) != base+3 || got[0].Side != 60 || !got[0].Enabled {
		t.Fatalf("options = %+v", got)
	}
	if MapSizeLabel(got[0]) != "60 x 60 (test)" {
		t.Fatalf("label %q", MapSizeLabel(got[0]))
	}
}
