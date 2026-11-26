// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"bytes"
	"os"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/economy/item"
	"testing"
)

func TestHistoricalPluginFilter(t *testing.T) {
	raw, err := os.ReadFile("testdata/historical-v20-plugin-filter.gob.gz")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name     string
		book     item.Book
		retained bool
	}{{"retained", pluginItems("review3.cup"), true}, {"removed", item.Core(), false}} {
		t.Run(tc.name, func(t *testing.T) {
			g, info, err := LoadContent(bytes.NewReader(raw), Content{Items: tc.book})
			if err != nil {
				t.Fatal(err)
			}
			v := zoneViewAt(t, g, 10, 8)
			z, ok := g.zones.Zone(zone.ID(v.ID))
			if !ok {
				t.Fatal("no restored zone")
			}
			k, has := tc.book.KindOf("review3.cup")
			t.Logf("v%d current core=%d historical slot=%d native=%s nativeAllowed=%v pluginSlot=%d pluginAllowed=%v filter=%v", info.Version, item.CoreKinds, item.BlissCircuit, item.BlissCircuit.String(), z.Allows(item.BlissCircuit), k, has && z.Allows(k), z.Filter)
			if tc.retained && (!has || !z.Allows(k) || len(stacksOf(g, k)) != 1) {
				t.Error("plugin identity or stack not preserved")
			}
			if !tc.retained && has {
				t.Error("removed plugin exists")
			}
			if z.Allows(item.BlissCircuit) {
				t.Error("old plugin filter bit leaked into newly appended native item")
			}
			want := 0
			if tc.retained {
				want = 1
			}
			if z.Filter.Len() != want {
				t.Errorf("filter has %d kinds, want %d", z.Filter.Len(), want)
			}
			g.Step(60)
		})
	}
}
