// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"bytes"
	"os"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"testing"
)

func TestHistoricalConstruction(t *testing.T) {
	raw, err := os.ReadFile("testdata/historical-v20-construction.sav")
	if err != nil {
		t.Fatal(err)
	}
	g, info, err := Load(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if info.Version != 20 {
		t.Fatalf("not historical writer: %d", info.Version)
	}
	b, ok := g.bldgs.Get(2)
	if !ok || b.Kind != construction.Table || b.MaterialBasis.Primary != item.MaterialV20 {
		t.Fatalf("historical basis lost: %+v", b)
	}
	f, ok := g.frames.Get(3)
	if !ok || f.Wood != 8 || f.Need != 75 || f.WorkLeft != 1 {
		t.Fatalf("bad active frame rebase: %+v", f)
	}
	partial, ok := g.frames.Get(4)
	if !ok || partial.Wood != 1 || partial.Need != 28 {
		t.Fatalf("bad partial frame rebase: %+v", partial)
	}
	j, _ := g.jobs.Get(1)
	if j.Kind != job.Construct || j.Target != 3 {
		t.Fatalf("active job not imported: %+v", j)
	}
	g.doConstruct(1, j)
	after, ok := g.frames.Get(3)
	if !ok || after != f {
		t.Fatalf("underpaid frame finished or changed: %+v", after)
	}
	var out bytes.Buffer
	if err := g.Save(&out, "rebased"); err != nil {
		t.Fatal(err)
	}
	g, _, err = Load(bytes.NewReader(out.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if next, _ := g.frames.Get(3); next != f {
		t.Fatalf("second import changed frame: %+v", next)
	}
	before := g.stockOf(item.Wood)
	g.removeBuilding(2)
	if got := g.stockOf(item.Wood) - before; got != 1 {
		t.Fatalf("old table refunded %d, want 1", got)
	}
	before = g.stockOf(item.Wood)
	g.cancelFrame(4)
	if got := g.stockOf(item.Wood) - before; got != 1 {
		t.Fatalf("partial frame refunded %d, want 1", got)
	}
}

// Written by the actual v20 executable, before the floor recipe exemption
// was corrected. Its native active swap has only one steel delivered.
func TestV20PendingFloorSwapMigration(t *testing.T) {
	raw, err := os.ReadFile("testdata/historical-v20-pending-floor-swap.sav")
	if err != nil {
		t.Fatal(err)
	}
	g, info, err := Load(bytes.NewReader(raw))
	if err != nil || info.Version != 20 {
		t.Fatalf("historical load %v %v", info, err)
	}
	fid, fr := swapFrameOn(t, g, 5, 4, construction.LayerFloor)
	bid := g.layerBldgAt(5, 4, construction.LayerFloor)
	if fr.Swaps != bid || !fr.SwapNewRecipe || fr.Need != 3 || fr.Wood != 1 || fr.ExtraNeed(0) != 12 || fr.ExtraGot[0] != 0 || fr.WorkLeft != 1 || fr.Ready() {
		t.Fatalf("wrong historical swap rebase %+v", fr)
	}
	p := g.pawnIDs[0]
	j, _ := g.jobs.Get(p)
	if j.Kind != job.Construct || j.Target != fid {
		t.Fatalf("historical active job lost %+v", j)
	}
	g.doConstruct(p, j)
	after, ok := g.frames.Get(fid)
	if !ok || after != fr {
		t.Fatal("underpaid historical job built sterile floor")
	}
	g = saveLoad(t, g)
	after, ok = g.frames.Get(fid)
	if !ok || after != fr {
		t.Fatal("second load changed migrated recipe")
	}
	g.cancelFrame(fid)
	b, _ := g.bldgs.Get(bid)
	if b.Kind != construction.Floor || g.stockOf(item.Steel) != 1 || g.stockOf(item.Silver) != 0 {
		t.Fatalf("historical cancel lost or fabricated materials %+v", b)
	}
}
