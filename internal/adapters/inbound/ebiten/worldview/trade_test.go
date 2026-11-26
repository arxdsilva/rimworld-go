// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package worldview

import (
	"strconv"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/domain/sim/command"
)

// tradeGame is a colony game whose caravan 7 stands on a settlement.
func tradeGame(t *testing.T, why string) (*fakeShell, *View, int) {
	t.Helper()
	g := colonyGame(t)
	tile := g.sites[1].Tile
	g.world.Caravans[0].Tile, g.world.Caravans[0].Dest = tile, tile
	g.world.Caravans[0].TradeWhy = why
	sh, v := newFake(g)
	v.Show("caravan:7")
	frame(sh, v, -1, -1, inputframe.Raw{})
	return sh, v, tile
}

func tradeCmds(sh *fakeShell) (n int, target uint64) {
	for _, c := range sh.cmds {
		if c.Kind == command.TradeSettlement {
			n, target = n+1, c.Target
		}
	}
	return n, target
}

func TestWorldTradeGizmo(t *testing.T) {
	t.Parallel()
	t.Run("enabled", func(t *testing.T) {
		sh, v, _ := tradeGame(t, "")
		w, ok := sh.kit.Find("world.act.trade")
		if !ok || w.Disabled {
			t.Fatalf("trade gizmo %v enabled=%v", ok, ok && !w.Disabled)
		}
		clickWidget(t, sh, v, "world.act.trade")
		if n, target := tradeCmds(sh); n != 1 || target != 7 {
			t.Fatalf("sent %d, target %d", n, target)
		}
	})
	t.Run("disabled", func(t *testing.T) {
		sh, v, _ := tradeGame(t, "Nobody here to trade with.")
		w, ok := sh.kit.Find("world.act.trade")
		if !ok || !w.Disabled {
			t.Fatal("trade gizmo not disabled")
		}
		if g := (inspector{v}).tradeGizmo(7, "Nobody here to trade with."); g.why != "Nobody here to trade with." || g.run != nil {
			t.Fatalf("gizmo %+v", g)
		}
		clickWidget(t, sh, v, "world.act.trade")
		if n, _ := tradeCmds(sh); n != 0 {
			t.Fatalf("a disabled gizmo sent %d commands", n)
		}
	})
}

func TestWorldTradeFromSettlement(t *testing.T) {
	t.Parallel()
	t.Run("caravan on the tile", func(t *testing.T) {
		sh, v, tile := tradeGame(t, "")
		v.selectKey("settlement:" + strconv.Itoa(tile))
		frame(sh, v, -1, -1, inputframe.Raw{})
		clickWidget(t, sh, v, "world.act.trade")
		if n, target := tradeCmds(sh); n != 1 || target != 7 {
			t.Fatalf("sent %d, target %d", n, target)
		}
	})
	t.Run("lowest id wins", func(t *testing.T) {
		sh, v, tile := tradeGame(t, "")
		v.objs = append(v.objs, Object{Kind: KindCaravan})
		v.objs[len(v.objs)-1].Caravan.ID, v.objs[len(v.objs)-1].Caravan.Tile = 3, tile
		gz := (inspector{v}).settlementTrade(tile)
		gz.run()
		if _, target := tradeCmds(sh); target != 3 {
			t.Fatalf("target %d, want 3", target)
		}
	})
	t.Run("no caravan there", func(t *testing.T) {
		sh, v, _ := tradeGame(t, "")
		other := sh.env.Game.(*fakeGame).sites[0].Tile
		v.selectKey("settlement:" + strconv.Itoa(other))
		frame(sh, v, -1, -1, inputframe.Raw{})
		w, ok := sh.kit.Find("world.act.trade")
		if !ok || !w.Disabled {
			t.Fatal("trade not disabled with no caravan")
		}
		if g := (inspector{v}).settlementTrade(other); g.why != "Send a caravan here to trade." {
			t.Fatalf("why %q", g.why)
		}
		clickWidget(t, sh, v, "world.act.trade")
		if n, _ := tradeCmds(sh); n != 0 {
			t.Fatal("sent with no caravan")
		}
	})
}
