package combat

import (
	"testing"

	"rimworld-go/internal/domain/combat/projectile"
)

// TestTraceViews (moved from the game): traces younger than the lifetime,
// aged, every field carried.
func TestTraceViews(t *testing.T) {
	t.Parallel()
	tr := []projectile.Trace{
		{Tick: 100, Weapon: "old"},
		{FromX: 1, FromY: 2, ToX: 3, ToY: 4, Tick: 990, Hit: true, Melee: true, Damage: 5, Building: true, FromBuilding: true,
			Attacker: 7, Target: 8, Weapon: "rifle", Stage: projectile.Landed},
	}
	got := TraceViews(tr, 1000)
	want := TraceView{FromX: 1, FromY: 2, ToX: 3, ToY: 4, Age: 10, Hit: true, Melee: true, Damage: 5, Building: true,
		FromBuilding: true, Attacker: 7, Target: 8, Weapon: "rifle", Stage: projectile.Landed}
	if 1000-100 < projectile.Lifetime || len(got) != 1 || got[0] != want {
		t.Fatalf("views %+v", got)
	}
}
