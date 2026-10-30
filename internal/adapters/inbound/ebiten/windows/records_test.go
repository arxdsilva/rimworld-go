package windows

import (
	"testing"

	"rimworld-go/internal/application/game"
)

func TestRecordRows(t *testing.T) {
	t.Parallel()
	ps := []game.PawnView{{Name: "B", Kills: 1, DamageDealt: 9}, {Name: "A", Kills: 3}, {Name: "Idle"}, {Name: "Raider", Hostile: true, Kills: 9}, {Name: "C", Kills: 1, DamageDealt: 20}}
	rows := RecordRows(ps, func(p game.PawnView) string { return p.Name })
	want := []string{"A", "C", "B"}
	if len(rows) != len(want) {
		t.Fatalf("rows %+v", rows)
	}
	for i, n := range want {
		if rows[i].Name != n {
			t.Fatalf("row %d %q, want %q", i, rows[i].Name, n)
		}
	}
}
