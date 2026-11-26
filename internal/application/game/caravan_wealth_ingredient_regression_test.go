// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/caravan"
	"testing"
)

func TestPhysicalQuestWealthAfterNativeTrade(t *testing.T) {
	g := worldGame(t)
	tile := nearDest(t, g)
	pawn := g.pawnIDs[0]
	cid := visitingMarket(t, g, tile, pawn)
	st := item.Stack{Kind: item.Parka, Qty: 1, Stuff: item.Cloth, Quality: 4, Wear: 10}
	travelOf(g).Carry(cid, st)
	book := g.sk.content.Items
	if book.StackValue(st) == book.Value(st.Kind) {
		t.Fatal("fixture physical and base values identical")
	}
	value := func() int {
		n := 0
		for _, s := range travelOf(g).Goods(cid) {
			n += book.StackValue(s) * s.Qty
		}
		return n
	}
	oldValue, oldWealth := value(), threatBase(g, true).Wealth
	g.Enqueue(command.Command{Kind: command.WorldCaravanTrade, Target: cid, Pawn: uint64(pawn), Name: trade.EncodeOffer([]trade.Delta{{Kind: item.Parka, Qty: -1}})})
	g.ApplyPending()
	if cargoQty(travelOf(g).Goods(cid), item.Parka) != 0 {
		t.Fatal("native sale did not happen")
	}
	want := float64(value() - oldValue)
	got := threatBase(g, true).Wealth - oldWealth
	t.Logf("unit base=%d physical=%d old cargo=%d new cargo=%d expected wealth delta=%g actual=%g", book.Value(st.Kind), book.StackValue(st), oldValue, value(), want, got)
	h := saveLoad(t, g)
	h.worldS().planet, h.worldS().planetTried = testPlanet().Fork(), true
	if h.StateHash() != g.StateHash() {
		t.Fatal("save diverged")
	}
	if threatBase(h, true).Wealth != threatBase(g, true).Wealth {
		t.Fatal("wealth not reproduced after save")
	}
	if got != want {
		t.Fatalf("physical wealth changed by %g, expected %g", got, want)
	}
}

func TestMarketPaceWithStashedPackAnimal(t *testing.T) {
	g := worldGame(t)
	tile := nearDest(t, g)
	pawn := g.pawnIDs[0]
	cid := visitingMarket(t, g, tile, pawn)
	x, y := ax9Free(t, g)
	aid, err := g.SpawnAnimal("horse", x, y)
	if err != nil {
		t.Fatal(err)
	}
	g.animals.Ref(aid).Tame = true
	if !g.stashPawn(aid, entity.EntityID(cid)) {
		t.Fatal("stash pack animal")
	}
	cv := fleetOf(g).At(entity.EntityID(cid))
	cv.C.Animals = append(cv.C.Animals, uint64(aid))
	travelOf(g).Carry(cid, item.Stack{Kind: item.Silver, Qty: 900})
	market, _ := fleetOf(g).Market().At(tile)
	kind := item.Wood
	if market.Stock.Kind == trade.SpacerSettlement {
		kind = item.Steel
	}
	g.Enqueue(command.Command{Kind: command.WorldCaravanTrade, Target: cid, Pawn: uint64(pawn), Name: trade.EncodeOffer([]trade.Delta{{Kind: kind, Qty: 20}})})
	g.ApplyPending()
	goods := travelOf(g).Goods(cid)
	if cargoQty(goods, kind) != 20 {
		t.Fatalf("native purchase failed: %v", goods)
	}
	grams := 0
	for _, s := range goods {
		grams += s.Qty * g.sk.content.Items.MassGrams(s.Kind)
	}
	cap := int(caravan.PersonCarryKg+caravan.PackKg(animal.BodySize(animal.Horse))) * 1000
	want := int32(grams * 1000 / cap)
	cv = fleetOf(g).At(entity.EntityID(cid))
	t.Logf("mass=%d capacity=%d expected load=%d actual=%d mount=%d speed=%d", grams, cap, want, cv.C.Load, cv.C.Mount, cv.C.Speed())
	if cv.C.Load-want > 2 || want-cv.C.Load > 2 {
		t.Fatalf("trade recalculated load without native stashed pack animal: got %d want %d", cv.C.Load, want)
	}
}

func TestForbiddenLaidIngredientAtCompletion(t *testing.T) {
	p := newPackBill(t)
	g := p.g
	p.runTo(t, "laid ingredients", func(p *packBill) bool {
		j, _ := g.jobs.Get(p.ada)
		return j.Kind == job.Bill && j.Carry != 0 && len(j.Route) > 0 && j.Dest == p.bench
	})
	j, _ := g.jobs.Get(p.ada)
	sid := j.Route[0]
	before, ok := g.stacks.Get(sid)
	if !ok || before.Carrier != 0 {
		t.Fatal("not a laid ingredient")
	}
	g.Enqueue(command.Command{Kind: command.SetForbidden, Target: uint64(sid), Enabled: true})
	g.ApplyPending()
	after, ok := g.stacks.Get(sid)
	if !ok || !after.Forbidden || after.X != before.X || after.Y != before.Y {
		t.Fatalf("fixture: forbidden command moved/removed ingredient: before=%+v after=%+v exists=%v", before, after, ok)
	}
	t.Logf("forbidden laid ingredient id=%d kind=%v qty=%d at %d,%d", sid, after.Kind, after.Qty, after.X, after.Y)
	for i := 0; i < 6000 && !p.done(); i++ {
		g.SetNeeds(p.ada, needs.Full, needs.Full)
		g.Step(1)
		current, ok := g.stacks.Get(sid)
		if !ok {
			t.Fatalf("forbidden ingredient was consumed or removed, completed=%v tick=%d", p.done(), i)
		}
		if !current.Forbidden {
			t.Fatal("forbidden flag unexpectedly cleared")
		}
		if nj, _ := g.jobs.Get(p.ada); nj.Kind != job.Bill {
			break
		}
	}
	if p.done() {
		t.Fatal("bill completed despite forbidden reserved ingredient")
	}
}
