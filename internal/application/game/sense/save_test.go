package sense

import (
	"reflect"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/gameplay"
	"rimworld-go/internal/domain/pawn/inventory"
	"rimworld-go/internal/domain/sim/command"
)

func TestDefaultsExportNothing(t *testing.T) {
	t.Parallel()
	s := New(Roles{})
	if got := s.Export(); got != nil {
		t.Fatalf("a new colony exports %+v, want nil", got)
	}
	s.Import(nil)
	if got := s.Export(); got != nil {
		t.Fatalf("after importing nil: %+v, want nil", got)
	}
	if s.TidyAt == nil || s.Packs == nil || s.NoClean == nil {
		t.Fatal("importing nil left a nil map, which a component would write to")
	}
}

func TestExportImportKeepsEverything(t *testing.T) {
	t.Parallel()
	s := New(Roles{})
	s.Rules = s.Rules.With(gameplay.OddMeatNormal, true).WithLimit(gameplay.TidyStation, 9)
	s.Resume[7] = job.Record{Kind: job.Haul}
	s.TidyAt[9], s.TidyAt[3] = 90, 30
	s.HaulAt[4], s.SpareAt[5], s.UnloadAt[6] = 40, 50, 60
	s.NoClean[12], s.NoClean[11] = true, true
	s.TopOffOn[8] = true
	s.Packs[2] = inventory.Pack{Next: 3}
	s.FromPack[2] = 44

	out := s.Export()
	if out == nil {
		t.Fatal("nil export with state in it")
	}
	if out.TidyAt[0].Pawn != 3 || out.TidyAt[1].Pawn != 9 || out.NoClean[0] != 11 || out.NoClean[1] != 12 {
		t.Fatalf("lists not in id order: %+v %+v", out.TidyAt, out.NoClean)
	}
	back := New(Roles{})
	back.Import(out)
	if again := back.Export(); !reflect.DeepEqual(out, again) {
		t.Fatalf("round trip\n got %+v\nwant %+v", again, out)
	}
}

// TestApplyRules: the commands change the rules, and an index past the table
// changes nothing.
func TestApplyRules(t *testing.T) {
	t.Parallel()
	s := New(Roles{})
	s.Apply(command.Command{Kind: command.SetGameplayToggle, Index: int(gameplay.CleanBeforeWork), Enabled: false})
	if s.Rules.On(gameplay.CleanBeforeWork) {
		t.Fatal("toggle stayed on")
	}
	s.Apply(command.Command{Kind: command.SetGameplayLimit, Index: int(gameplay.TidyTend), Count: 7})
	if s.Rules.LimitOf(gameplay.TidyTend) != 7 {
		t.Fatal("limit not set")
	}
	before := s.Rules
	for _, c := range []command.Command{
		{Kind: command.SetGameplayToggle, Index: int(gameplay.NumToggles), Enabled: true},
		{Kind: command.SetGameplayToggle, Index: -1, Enabled: true},
		{Kind: command.SetGameplayLimit, Index: int(gameplay.NumLimits), Count: 1},
		{Kind: command.SetGameplayLimit, Index: -1, Count: 1},
	} {
		s.Apply(c)
	}
	if s.Rules != before {
		t.Fatalf("a bad index changed the rules: %+v", s.Rules)
	}
}
