// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"bytes"
	"os"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/pawn/needs"
	"testing"
)

// These bytes were written by the actual pinned v20 writer with its
// valid legacy bench/input contract, not synthesized by the new loader.
func TestHistoricalProstheticBills(t *testing.T) {
	for _, mode := range []string{"idle", "active"} {
		t.Run(mode, func(t *testing.T) {
			raw, err := os.ReadFile("testdata/historical-v20-prosthetic-" + mode + ".sav")
			if err != nil {
				t.Fatal(err)
			}
			g, info, err := Load(bytes.NewReader(raw))
			if err != nil || info.Version != 20 {
				t.Fatalf("load %v %v", info, err)
			}
			ada := g.pawnIDs[0]
			bench := g.bldgIDs[0]
			bl := g.craft.Bills.Of(bench)
			if len(bl) != 1 || bl[0].Recipe != recipe.MakeProstheticLeg || bl[0].Done != 0 {
				t.Fatalf("lost intent %+v", bl)
			}
			totalSteel := func() int {
				n := 0
				for _, id := range g.itemIDs {
					if st, ok := g.stacks.Get(id); ok && st.Kind == item.Steel {
						n += st.Qty
					}
				}
				return n
			}
			before := totalSteel()
			if mode == "active" {
				j, _ := g.jobs.Get(ada)
				if j.Kind != job.Bill || j.Carry == 0 || j.State != job.Do {
					t.Fatalf("historical job not active %+v", j)
				}
				g.phaseJobs()
				j, _ = g.jobs.Get(ada)
				if j.State != job.Fail {
					t.Fatalf("incompatible active bill not refused %+v", j)
				}
			} else {
				if g.craft.Shop.Assign(ada, work.Mask(0).Set(work.Craft, true).Set(work.Smith, true)) {
					t.Fatal("legacy workbench recipe admitted")
				}
			}
			for i := 0; i < 60; i++ {
				g.SetNeeds(ada, needs.Full, needs.Full)
				g.Step(1)
			}
			bl = g.craft.Bills.Of(bench)
			if len(bl) != 1 || bl[0].Done != 0 || g.stockOf(item.ProstheticLeg) != 0 || totalSteel() != before {
				t.Fatalf("invalid bill consumed or produced: bill %+v steel %d->%d prosthetic %d", bl, before, totalSteel(), g.stockOf(item.ProstheticLeg))
			}
		})
	}
}
