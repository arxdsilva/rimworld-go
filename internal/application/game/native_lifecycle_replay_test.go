// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"bytes"
	"os"
	"testing"

	"rimworld-go/internal/application/game/herd"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/animal/training"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/building/zone"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/caravan"
)

func TestHistoricalPluginReloadAndStorage(t *testing.T) {
	raw, err := os.ReadFile("testdata/historical-v20-plugin-filter.gob.gz")
	if err != nil {
		t.Fatal(err)
	}
	book := pluginItems("another.cup", "review3.cup")
	g, _, err := LoadContent(bytes.NewReader(raw), Content{Items: book})
	if err != nil {
		t.Fatal(err)
	}
	k, _ := book.KindOf("review3.cup")
	at := func(x, y int) bool { return x == 10 && y == 8 }
	check := func(g *Game) {
		t.Helper()
		ss := stacksOf(g, k)
		if len(ss) != 1 {
			t.Fatalf("plugin stacks: %v", ss)
		}
		if _, _, ok := g.storageCell(ss[0], ss[0].X, ss[0].Y, at, zone.PriorityLow-1); !ok {
			t.Fatal("restored plugin filter cannot admit plugin load")
		}
		if _, _, ok := g.storageCell(item.Stack{Kind: item.BlissCircuit, Qty: 1}, ss[0].X, ss[0].Y, at, zone.PriorityLow-1); ok {
			t.Fatal("new native kind admitted")
		}
	}
	check(g)
	var b bytes.Buffer
	if err := g.Save(&b, "fixer"); err != nil {
		t.Fatal(err)
	}
	h, _, err := LoadContent(&b, Content{Items: book})
	if err != nil {
		t.Fatal(err)
	}
	check(h)
	if h.StateHash() != g.StateHash() {
		t.Fatal("restored hash diverged")
	}
}

func TestHaulStableNoWorkAndInvalidation(t *testing.T) {
	g := feedField(64, 64)
	h := animalHaulHost{haulHost{g}}
	var dogs []entity.EntityID
	for i := 0; i < 40; i++ {
		id, err := g.SpawnAnimal("husky", 2+i, 2)
		if err != nil {
			t.Fatal(err)
		}
		g.TameAnimal(id, "carrier")
		saveTraining(g, id, training.Maintenance{DecayAt: 1 << 40, HaulProgress: training.Needed(training.Haul) | herd.TrainToggled})
		dogs = append(dogs, id)
	}
	for i := 0; i < 512; i++ {
		stack(t, g, item.Wood, 5, 2+i%32, 8+i/32)
	}
	stored := g.zones.Paint(zone.KindStockpile, zone.NewStockpile(), zone.RectOf(2, 32, 33, 35), nil)
	if stored == 0 {
		t.Fatal("missing stored-stock fixture")
	}
	for i := 0; i < 128; i++ {
		stack(t, g, item.Wood, g.sk.content.Items.StackLimit(item.Wood), 2+i%32, 32+i/32)
	}
	for i := 0; i < 128; i++ {
		sid := stack(t, g, item.Wood, 5, 2+i%32, 40+i/32)
		st, _ := g.stacks.Get(sid)
		st.Forbidden = true
		g.putStack(sid, st)
	}
	zid := g.zones.Paint(zone.KindStockpile, zone.NewStockpile(), zone.RectOf(40, 8, 42, 10), nil)
	g.zones.Update(zid, func(z *zone.Zone) { z.Custom = true; z.Filter = (zone.Filter{}).With(item.Steel, true) })
	offers := func(id entity.EntityID) []herd.HaulOffer { a, _ := g.animals.Get(id); return h.HaulOffers(id, a) }
	if got := offers(dogs[0]); len(got) != 0 {
		t.Fatalf("unexpected offers %v", got)
	}
	s, c, p := g.idx.animalHaul.Counts()
	if s != uint64(len(g.itemIDs)) || c == 0 || p != 0 {
		t.Fatalf("initial work %d %d %d, items=%d", s, c, p, len(g.itemIDs))
	}
	for i := 0; i < 8; i++ {
		for _, id := range dogs {
			if len(offers(id)) != 0 {
				t.Fatal("unexpected offer")
			}
		}
	}
	if s2, c2, p2 := g.idx.animalHaul.Counts(); s2 != s || c2 != c || p2 != p {
		t.Fatalf("stable no-work rescanned: %d/%d/%d -> %d/%d/%d", s, c, p, s2, c2, p2)
	}
	t.Logf("320 repeated animal queries: source scans=%d, candidate checks=%d, destination searches=%d unchanged", s, c, p)
	g.zones.Update(zid, func(z *zone.Zone) { z.Filter = (zone.Filter{}).With(item.Wood, true) })
	if got := offers(dogs[0]); len(got) == 0 {
		t.Fatal("filter edit did not invalidate negative cache")
	}
}

func TestHaulDestinationReachAndClaims(t *testing.T) {
	g := feedField(24, 18)
	dog, err := g.SpawnAnimal("husky", 8, 8)
	if err != nil {
		t.Fatal(err)
	}
	g.TameAnimal(dog, "carrier")
	a, _ := g.animals.Get(dog)
	h := animalHaulHost{haulHost{g}}
	offers := func() []herd.HaulOffer { return h.HaulOffers(dog, a) }
	wallRing(t, g, 9, 7, 11, 9)
	sid := stack(t, g, item.Wood, 5, 4, 8)
	g.zones.Paint(zone.KindStockpile, zone.NewStockpile(), zone.RectOf(10, 8, 10, 8), nil)
	if len(offers()) != 0 {
		t.Fatal("inaccessible destination admitted")
	}
	zid := g.zones.Paint(zone.KindStockpile, zone.NewStockpile(), zone.RectOf(2, 3, 2, 3), nil)
	o := offers()
	if len(o) != 1 || o[0].Source != sid || o[0].X != 2 || o[0].Y != 3 {
		t.Fatalf("alternate destination %v", o)
	}
	owner, err := g.SpawnNamedPawn(1, 1, "human")
	if err != nil {
		t.Fatal(err)
	}
	if !g.reserve.TryEntity(owner, sid) {
		t.Fatal("fixture claim")
	}
	if len(offers()) != 0 {
		t.Fatal("human claimed source still admitted")
	}
	g.reserve.ReleaseOwner(owner)
	if !g.reserve.TryTile(owner, 2, 3) {
		t.Fatal("fixture tile claim")
	}
	if len(offers()) != 0 {
		t.Fatal("human claimed destination still admitted")
	}
	g.reserve.ReleaseOwner(owner)
	if len(offers()) != 1 {
		t.Fatal("release failed to invalidate")
	}
	applyNow(g, command.Command{Kind: command.SetForbidden, Target: uint64(sid), Enabled: true})
	if len(offers()) != 0 {
		t.Fatal("forbidden source admitted")
	}
	applyNow(g, command.Command{Kind: command.SetForbidden, Target: uint64(sid), Enabled: false})
	if len(offers()) != 1 {
		t.Fatal("unforbid did not invalidate")
	}
	paintArea(g, 1, 3, 4, 8, 10)
	assign(g, dog, 1)
	if len(offers()) != 0 {
		t.Fatal("outside-area destination admitted")
	}
	paintArea(g, 1, 2, 3, 8, 10)
	if len(offers()) != 1 {
		t.Fatal("expanded area did not invalidate")
	}
	g.zones.Update(zid, func(z *zone.Zone) { z.Custom = true; z.Filter = (zone.Filter{}).With(item.Steel, true) })
	if len(offers()) != 0 {
		t.Fatal("filter edit failed to invalidate")
	}
	assign(g, dog, command.AreaUnrestricted)
	wall := g.layerBldgAt(9, 8, construction.LayerObject)
	if wall == 0 {
		t.Fatal("missing enclosing wall")
	}
	g.removeBuildingAs(wall, false)
	o = offers()
	if len(o) != 1 || o[0].X != 10 || o[0].Y != 8 {
		t.Fatalf("opened wall did not invalidate destination: %v", o)
	}
	buildNow(t, g, construction.Door, item.Wood, construction.South, 9, 8)
	if len(offers()) != 1 {
		t.Fatal("tame animal cannot use new door")
	}
	_, before, _ := g.idx.animalHaul.Counts()
	applyNow(g, command.Command{Kind: command.SetDoorHoldOpen, X: 9, Y: 8, Enabled: true})
	if len(offers()) != 1 {
		t.Fatal("held-open door incorrectly blocked")
	}
	_, after, _ := g.idx.animalHaul.Counts()
	if after <= before {
		t.Fatal("door change did not invalidate admission")
	}
	full := stack(t, g, item.Wood, g.sk.content.Items.StackLimit(item.Wood), 10, 8)
	if len(offers()) != 0 {
		t.Fatal("full destination admitted")
	}
	st, _ := g.stacks.Get(full)
	st.Qty -= 5
	g.putStack(full, st)
	if len(offers()) != 1 {
		t.Fatal("released storage capacity did not invalidate")
	}
}

func TestForbiddenIngredientReplay(t *testing.T) {
	for _, pack := range []bool{true, false} {
		name := "ordinary"
		if pack {
			name = "pack"
		}
		t.Run(name, func(t *testing.T) {
			var p *packBill
			if pack {
				p = newPackBill(t)
			} else {
				p = newPackBill(t, offHabits)
			}
			g := p.g
			p.runTo(t, "laid", func(p *packBill) bool {
				j, _ := g.jobs.Get(p.ada)
				return j.Kind == job.Bill && j.Carry != 0 && len(j.Route) > 0 && j.Dest == p.bench
			})
			j, _ := g.jobs.Get(p.ada)
			sid := j.Route[0]
			applyNow(g, command.Command{Kind: command.SetForbidden, Target: uint64(sid), Enabled: true})
			h := saveLoad(t, g)
			ended := false
			for i := 0; i < 6000; i++ {
				for _, sim := range []*Game{g, h} {
					sim.SetNeeds(p.ada, needs.Full, needs.Full)
					sim.Step(1)
				}
				if i%20 == 0 && g.StateHash() != h.StateHash() {
					t.Fatalf("replay diverged at %d", i)
				}
				st, ok := g.stacks.Get(sid)
				if !ok || !st.Forbidden || p.done() {
					t.Fatal("forbidden input consumed or bill completed")
				}
				j, _ := g.jobs.Get(p.ada)
				if j.Kind != job.Bill || j.State == job.Fail {
					ended = true
					break
				}
			}
			if !ended {
				t.Fatal("bill never terminated")
			}
			if g.StateHash() != h.StateHash() {
				t.Fatal("terminal replay diverged")
			}
			p.balanced(t, 0)
			// Suspend the row to observe normal terminal cleanup without immediate reassignment.
			bills := g.craft.Bills.Of(p.bench)
			bills[0].Suspended = true
			g.craft.Bills.Set(p.bench, bills)
			g.Step(1)
			p.noBillClaims(t)
			p.balanced(t, 1)
			applyNow(g, command.Command{Kind: command.SetForbidden, Target: uint64(sid), Enabled: false})
			bills = g.craft.Bills.Of(p.bench)
			bills[0].Suspended = false
			g.craft.Bills.Set(p.bench, bills)
			p.runTo(t, "subsequent valid bill", func(p *packBill) bool { return p.done() })
			p.balanced(t, 2)
		})
	}
}

func TestRescueChangedClimateReplay(t *testing.T) {
	g, ada, doc, pin := heatwaveColony(t, 320)
	x, y := tileOf(g, ada)
	g.Enqueue(prioritize(command.OrderRescue, doc, ada, x, y, false))
	for i := 0; i < 3000 && g.care.Carry.Carrier(ada) != doc; i++ {
		g.Step(1)
		pin()
		fed(g, doc)
	}
	j, _ := g.jobs.Get(doc)
	if g.care.Carry.Carrier(ada) != doc || j.Dest == 0 {
		t.Fatal("fixture: not carrying")
	}
	b, _ := g.bldgs.Get(j.Dest)
	g.thermo.Temps[g.roomAt(b.X, b.Y)] = 320
	h := saveLoad(t, g)
	for i := 0; i < 3000 && g.care.Carry.Carrier(ada) == doc; i++ {
		for _, sim := range []*Game{g, h} {
			sim.thermo.Temps[sim.roomAt(b.X, b.Y)] = 320
			sim.Step(1)
			fed(sim, doc)
		}
		if i%20 == 0 && g.StateHash() != h.StateHash() {
			t.Fatalf("rescue replay diverged at %d", i)
		}
	}
	for _, sim := range []*Game{g, h} {
		if sim.care.Carry.Carrier(ada) != 0 || sim.bedUnder(ada) == j.Dest {
			t.Fatal("stale rescue committed")
		}
	}
	if g.StateHash() != h.StateHash() {
		t.Fatal("terminal rescue replay diverged")
	}
}

func TestPhysicalRecoveryPatchedValueReplay(t *testing.T) {
	g, c, source := recoveryGame(t)
	core := item.Core()
	defs := make([]item.Def, 0, item.CoreKinds)
	for k := range item.CoreKinds {
		defs = append(defs, core.Def(item.Kind(k)))
	}
	defs[item.Parka].Value *= 2
	book := item.NewBook(defs, core.Keys())
	g.sk.content.Items = book
	sid := entity.EntityID(c.Target)
	source.Kind, source.Qty, source.Stuff, source.Quality, source.Wear = item.Parka, 1, item.Cloth, 4, 10
	g.putStack(sid, source)
	before := threatBase(g, true).Wealth
	want := float64(book.StackValue(source))
	if want == float64(book.Value(item.Parka)) {
		t.Fatal("fixture has no condition/material effect")
	}
	applyNow(g, c)
	if got := threatBase(g, true).Wealth - before; got != want {
		t.Fatalf("actual site recovery added wealth %g, want %g", got, want)
	}
	// Another physical lot differs in material, wear, quality and quantity.
	sid = stack(t, g, item.Parka, 2, source.X, source.Y)
	second, _ := g.stacks.Get(sid)
	second.Stuff, second.Quality, second.Wear = item.Leather, 2, 25
	g.putStack(sid, second)
	c.Target = uint64(sid)
	before = threatBase(g, true).Wealth
	want = float64(book.StackValue(second) * second.Qty)
	applyNow(g, c)
	if got := threatBase(g, true).Wealth - before; got != want {
		t.Fatalf("second lot wealth %g want %g", got, want)
	}
	var buf bytes.Buffer
	if err := g.Save(&buf, "fixer"); err != nil {
		t.Fatal(err)
	}
	h, _, err := LoadContent(&buf, Content{Items: book})
	if err != nil {
		t.Fatal(err)
	}
	h.worldS().planet, h.worldS().planetTried = testPlanet().Fork(), true
	if h.StateHash() != g.StateHash() || threatBase(h, true).Wealth != threatBase(g, true).Wealth {
		t.Fatal("physical patched cargo diverged across save")
	}
}

func TestMarketPaceRepeatedTradeReturnReplay(t *testing.T) {
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
		t.Fatal("stash horse")
	}
	cv := fleetOf(g).At(entity.EntityID(cid))
	cv.C.Animals = append(cv.C.Animals, uint64(aid))
	travelOf(g).Carry(cid, item.Stack{Kind: item.Silver, Qty: 900})
	market, _ := fleetOf(g).Market().At(tile)
	kind := item.Wood
	if market.Stock.Kind == trade.SpacerSettlement {
		kind = item.Steel
	}
	check := func(sim *Game) int32 {
		t.Helper()
		grams := 0
		for _, s := range travelOf(sim).Goods(cid) {
			grams += s.Qty * sim.sk.content.Items.MassGrams(s.Kind)
		}
		cap := int(caravan.PersonCarryKg+caravan.PackKg(animal.BodySize(animal.Horse))) * 1000
		v := fleetOf(sim).At(entity.EntityID(cid))
		want := int32(grams * 1000 / cap)
		if v.C.Load != want || v.C.Mount != caravan.MountFactor(animal.Horse) {
			t.Fatalf("load/mount %d/%d want %d/%d", v.C.Load, v.C.Mount, want, caravan.MountFactor(animal.Horse))
		}
		return want
	}
	for i := 1; i <= 2; i++ {
		applyNow(g, command.Command{Kind: command.WorldCaravanTrade, Target: cid, Pawn: uint64(pawn), Name: trade.EncodeOffer([]trade.Delta{{Kind: kind, Qty: 10}})})
		if cargoQty(travelOf(g).Goods(cid), kind) != i*10 {
			t.Fatal("purchase failed")
		}
		check(g)
	}
	h := saveLoad(t, g)
	h.worldS().planet, h.worldS().planetTried = testPlanet().Fork(), true
	if check(h) != check(g) || g.StateHash() != h.StateHash() {
		t.Fatal("market save replay diverged")
	}
	for _, sim := range []*Game{g, h} {
		applyNow(sim, command.Command{Kind: command.CaravanReturn, Target: cid})
		check(sim)
	}
	for i := 0; i < 90; i++ {
		for _, sim := range []*Game{g, h} {
			worldForward(sim, 2500)
		}
		if g.StateHash() != h.StateHash() {
			t.Fatalf("return replay diverged after %d", i)
		}
		if fleetOf(g).At(entity.EntityID(cid)) == nil {
			return
		}
	}
	t.Fatal("caravan did not return")
}
