package ebiten

import (
	"slices"
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/domain/economy/item"
)

// journey_stk_test.go (STK-ST): stacks of 1000 and the dumping stockpile.

// storedAt is the loose stack the read model shows on tile (x, y).
func storedAt(j *journey, x, y int) (kind string, qty int) {
	for _, it := range j.world().Items {
		if !it.Carried && it.X == x && it.Y == y {
			return it.Kind, it.Qty
		}
	}
	return "", 0
}

// TestJourneyDumpTakesCorpseAndChunk: a dumping stockpile painted from
// the Architect shows its vanilla defaults in its pane (Normal priority,
// corpses and chunks), and the colonist hauls a carcass and a stone chunk
// lying on open ground into it, one per tile.
func TestJourneyDumpTakesCorpseAndChunk(t *testing.T) {
	t.Parallel()
	j := zoneJourney(t)
	z := j.layZone("zones", "dump", 8, 1, 9, 2)
	p := j.a.arch.Zones.Pane(z.ID)
	if !strings.HasPrefix(p.Title, "Dumping stockpile ") {
		t.Fatalf("pane title %q", p.Title)
	}
	body := strings.Join(p.Lines, " | ")
	if !slices.Contains(p.Lines, "Priority: Normal") || !strings.Contains(body, "Accepts: ") ||
		strings.Contains(body, "Accepts: everything") {
		t.Fatalf("pane lines %q, want Normal priority and a corpse/chunk filter", body)
	}
	if _, err := j.g.SpawnStack(item.CorpseAnimal, 1, 14, 10); err != nil {
		t.Fatal(err)
	}
	if _, err := j.g.SpawnStack(item.GraniteChunk, 1, 15, 12); err != nil {
		t.Fatal(err)
	}
	inDump := func() (corpse, chunk int) {
		for _, it := range j.world().Items {
			if it.Carried || it.X < 8 || it.X > 9 || it.Y < 1 || it.Y > 2 {
				continue
			}
			switch {
			case it.Animal != "" && strings.HasSuffix(it.Kind, "corpse"): // the view names the species
				corpse += it.Qty
			case it.Kind == item.GraniteChunk.String():
				chunk += it.Qty
			}
		}
		return corpse, chunk
	}
	j.key("Digit3", 0)
	for i := 0; i < 8000; i++ {
		if c, k := inDump(); c == 1 && k == 1 {
			break
		}
		careFeedHostileFree(j)
		j.update(1)
	}
	j.key("Space", 0)
	if c, k := inDump(); c != 1 || k != 1 {
		t.Fatalf("the dump holds %d carcass and %d chunk, want one of each", c, k)
	}
	for _, it := range j.world().Items {
		if !it.Carried && it.X >= 8 && it.X <= 9 && it.Y >= 1 && it.Y <= 2 && it.Qty != 1 {
			t.Fatalf("a dump tile holds %d %s, want one per tile", it.Qty, it.Kind)
		}
	}
}

// TestJourneyStockpileTileStacksPastOldLimit: a one-tile stockpile painted
// from the Architect takes 240 wood from three piles in one stack (the
// old wiki limit was 75), and the read model shows it.
func TestJourneyStockpileTileStacksPastOldLimit(t *testing.T) {
	t.Parallel()
	j := zoneJourney(t)
	for _, x := range []int{13, 14, 15} {
		if _, err := j.g.SpawnStack(item.Wood, 80, x, 12); err != nil {
			t.Fatal(err)
		}
	}
	j.openArch("zones", "stockpile")
	j.dragTiles(inputframe.Left, 8, 1, 8, 1, 0)
	j.leaveArch()
	j.key("Digit3", 0)
	for i := 0; i < 12000; i++ {
		if _, q := storedAt(j, 8, 1); q == 240 {
			break
		}
		careFeedHostileFree(j)
		j.update(1)
	}
	j.key("Space", 0)
	if k, q := storedAt(j, 8, 1); k != item.Wood.String() || q != 240 {
		t.Fatalf("the stockpile tile holds %d %s, want 240 wood in one stack", q, k)
	}
}
