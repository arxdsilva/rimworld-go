package travel

import (
	"testing"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/sim/entity"
)

// A hold weighs its people at 60 kg, its goods by their wiki masses
// (rounded up) and is worth its goods' market value.
func TestHoldMassAndValue(t *testing.T) {
	h := Hold{Pawns: []entity.EntityID{7}, Items: []item.Stack{{Kind: item.Steel, Qty: 30}}}
	h.addItem(item.Stack{Kind: item.Steel, Qty: 10, X: 3, Y: 4})
	if len(h.Items) != 1 || h.Items[0].Qty != 40 {
		t.Fatalf("like stacks did not merge: %+v", h.Items)
	}
	if kg := h.massKg(animalKg); kg != 60+20 {
		t.Fatalf("mass %d kg, want 80", kg)
	}
	if v := h.value(); v != 40*item.Steel.Value() {
		t.Fatalf("value %d", v)
	}
}

// The saved form keeps everything and nothing more (an empty state saves
// nothing).
func TestSaveRoundTrip(t *testing.T) {
	if (&State{}).Export() != nil || (*State)(nil).Export() != nil {
		t.Fatal("an empty state saved something")
	}
	s := &State{}
	s.setSleeper(4, 9)
	s.holdOf(5).addItem(item.Stack{Kind: item.Gold, Qty: 3})
	s.sites = []Site{{Tile: 11, Mineral: item.Jade, Qty: 700, Expires: 99}}
	s.flights = []Flight{{ID: 1, From: 2, To: 3, Arrive: 50, Pods: 2}}
	s.setScan(6, Scan{Work: 12, For: item.Gold})
	back := Import(s.Export())
	if !back.Asleep(9) || back.holds[5].Items[0].Qty != 3 || back.sites[0] != s.sites[0] || back.flights[0].Pods != 2 ||
		back.scans[6] != (Scan{Work: 12, For: item.Gold}) {
		t.Fatalf("round trip lost state: %+v", back.Export())
	}
}

// Ring visits rings outward in row order; EdgeTile draws in a fixed
// order from its roll and falls back to the first dry cell.
func TestRingAndEdgeTile(t *testing.T) {
	var seen [][2]int
	x, y, ok := Ring(5, 5, 2, func(x, y int) bool { seen = append(seen, [2]int{x, y}); return x == 6 && y == 6 })
	if !ok || x != 6 || y != 6 || seen[0] != [2]int{5, 5} || seen[1] != [2]int{4, 4} {
		t.Fatalf("ring found (%d, %d) after %v", x, y, seen)
	}
	rolls := []int{1, 3, 0, 7}
	i := 0
	roll := func(n int) int { v := rolls[i%len(rolls)] % n; i++; return v }
	ex, ey, ok := EdgeTile(10, 8, roll, func(int, int) bool { return true }, func(x, y int) bool { return false })
	if !ok || ex != 3 || ey != 7 {
		t.Fatalf("fallback edge (%d, %d) ok=%v, want the first dry cell (3, 7)", ex, ey, ok)
	}
}

// The settlement list skips the defeated, names unnamed settlements and
// marks hostile peoples unvisitable; the id list parser keeps each
// accepted id once, ascending.
func TestSettlementViewsAndIDs(t *testing.T) {
	fs := []faction.Faction{
		{Name: "Ash", Type: faction.Settlers, Relation: faction.Hostile, Settlements: []int{4}},
		{Name: "Gone", Type: faction.Settlers, Defeated: true, Settlements: []int{5}},
	}
	v := SettlementViews(fs, func(int) string { return "" })
	if len(v) != 1 || v[0].Name != "a settlement of Ash" || v[0].Visitable || v[0].Relation != "hostile" {
		t.Fatalf("views %+v", v)
	}
	ids := ParseIDs("9, 3,x,9,4", func(id entity.EntityID) bool { return id != 4 })
	if len(ids) != 2 || ids[0] != 3 || ids[1] != 9 {
		t.Fatalf("ids %v", ids)
	}
	if JoinNames([]string{"A", "B", "C"}) != "A, B and C" || JoinNames(nil) != "Nobody" {
		t.Fatal("join names")
	}
}
