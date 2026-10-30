package raid

import "testing"

// Whole-game raid tests (raiders, sieges, sappers, hive swarms, saves)
// stay in internal/application/game: raids_test.go, raid_plans_test.go,
// hive_test.go.

func TestThreatOdds(t *testing.T) {
	t.Parallel()
	count := func(winter bool, day int) map[Threat]int {
		out := map[Threat]int{}
		for roll := 0; roll < ThreatRolls; roll++ {
			out[PickThreat(roll, day, winter)]++
		}
		return out
	}
	// SIM-025 turned one raid face into a mad animal from day 6, in every
	// season (Fire falls back to a raid when none can go mad).
	if c := count(false, 5); c[ThreatRaid] != ThreatRolls {
		t.Fatalf("before day 6 only raids: %v", c)
	}
	if c := count(false, 7); c[ThreatMad] != 1 || c[ThreatRaid] != ThreatRolls-1 {
		t.Fatalf("day 7: raids and a mad animal: %v", c)
	}
	if c := count(false, 10); c[ThreatManhunters] != 2 || c[ThreatHive] != 0 || c[ThreatMad] != 1 || c[ThreatRaid] != 5 {
		t.Fatalf("day 10: %v", c)
	}
	if c := count(false, 12); c[ThreatManhunters] != 2 || c[ThreatHive] != 1 || c[ThreatMad] != 1 || c[ThreatRaid] != 4 {
		t.Fatalf("day 12: %v", c)
	}
	if c := count(true, 12); c[ThreatPack] != 4 || c[ThreatHive] != 1 || c[ThreatMad] != 1 || c[ThreatRaid] != 2 {
		t.Fatalf("winter: %v", c)
	}
}

func TestPlanNames(t *testing.T) {
	t.Parallel()
	for p, want := range map[Plan]string{Assault: "assault", Siege: "siege", Sappers: "sappers", 9: "assault"} {
		if p.String() != want {
			t.Errorf("plan %d: %q, want %q", p, p, want)
		}
	}
}

// rockHost is the ground Rock needs; any other host call panics.
type rockHost struct {
	Host
	rock  map[[2]int]bool
	mined int
}

func (h *rockHost) Rock(x, y int) bool { return h.rock[[2]int{x, y}] }
func (h *rockHost) Mine(x, y int)      { h.mined++; delete(h.rock, [2]int{x, y}) }

func TestRockOpensAtItsHitPoints(t *testing.T) {
	t.Parallel()
	h := &rockHost{rock: map[[2]int]bool{{3, 4}: true}}
	r := New(h, Roles{}).Rock
	r.Dig(5, 5, RockHP) // open ground: nothing to dig
	r.Dig(3, 4, RockHP-1)
	if h.mined != 0 || r.Export()[[2]int{3, 4}] != RockHP-1 {
		t.Fatalf("one short of its hit points the rock holds: %v", r.Export())
	}
	saved := r.Export()
	back := New(h, Roles{}).Rock
	back.Import(nil) // an older save keeps the empty map
	back.Import(saved)
	back.Dig(3, 4, 1)
	if h.mined != 1 || len(back.Export()) != 0 {
		t.Fatalf("the restored rock opens on the last blow: mined %d, left %v", h.mined, back.Export())
	}
}
