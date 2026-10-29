package craft

import (
	"testing"

	"rimworld-go/internal/domain/building/construction"
)

// TestCabinetBonus: 6 % per tool cabinet within 6 tiles, at most two, and
// the cached count follows the buildings through StructVer.
func TestCabinetBonus(t *testing.T) {
	t.Parallel()
	f := newFake()
	c := New(f).Cabinets
	f.build(1, construction.Workbench, 10, 5)
	if got := c.Bonus(1); got != 0 {
		t.Fatalf("no cabinets: %d", got)
	}
	f.build(2, construction.ToolCabinet, 16, 2) // 6 tiles away: counts
	if got := c.Bonus(1); got != 6 {
		t.Fatalf("one cabinet: %d", got)
	}
	f.build(3, construction.ToolCabinet, 8, 8)
	f.build(4, construction.ToolCabinet, 11, 8)
	if got := c.Bonus(1); got != 12 {
		t.Fatalf("three cabinets: %d, want the cap of 12", got)
	}
	f.build(5, construction.ToolCabinet, 17, 5) // 7 tiles away
	f.remove(3)
	f.remove(4)
	if got := c.Bonus(1); got != 6 {
		t.Fatalf("after removing two, one far: %d", got)
	}
	if got := c.Bonus(99); got != 0 {
		t.Fatalf("no bench: %d", got)
	}
}
