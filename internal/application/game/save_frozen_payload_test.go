// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package game

import (
	"bytes"
	"testing"

	"rimworld-go/internal/application/game/herd"
	"rimworld-go/internal/application/game/letters"
	"rimworld-go/internal/application/game/rage"
	"rimworld-go/internal/application/game/raid"
	"rimworld-go/internal/application/game/savefmt"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/incident"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/drug"
	"rimworld-go/internal/domain/pawn/inventory"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/pawngen"
	"rimworld-go/internal/domain/pawn/trait"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/pkg/letterdata"
)

// Populate infrequent wire payloads deliberately. This is an ownership
// fixture, not a valid gameplay scenario: it does not step these synthetic
// active jobs, conditions or wildlife events.
func TestFrozenSaveRarePayloads(t *testing.T) {
	g, pawn, wall, stack, _, deer := twoMaps(t)
	g.ViewMap(1)
	j := job.Record{Path: []pathing.Point{{X: 3, Y: 3}, {X: 4, Y: 3}}, Route: []entity.EntityID{wall}, Pickups: []entity.EntityID{stack, 1}}
	g.jobs.Set(pawn, j)
	senseOf(g).Resume[pawn] = j
	pack := inventory.Pack{}
	pack.Add(item.Stack{Kind: item.Wood, Qty: 3})
	senseOf(g).Packs[pawn] = pack
	g.craft.Bills.Set(wall, []recipe.Bill{{Recipe: recipe.CampfireMeal, Forbid: []item.Kind{item.Wood, item.Silver}}})
	g.bodies = map[entity.EntityID]*body.Body{pawn: {Wounds: []body.Wound{{Dmg: 2}}, Replacements: []body.Replacement{{Kind: item.Wood}}, Implants: []body.Implant{{}}, Conds: []body.Condition{{}}}}
	g.drugs = map[entity.EntityID]*drug.State{pawn: {Highs: []drug.High{{Severity: 100}}, Addictions: []drug.Addiction{{Need: 500}}, Tolerance: []int32{1, 2}, LastDose: []uint64{3, 4}}}
	g.surg = &surgeryState{ops: map[entity.EntityID][]body.Operation{pawn: {{Part: 0}}}, under: map[entity.EntityID]uint64{pawn: 4}, rest: map[entity.EntityID]uint64{pawn: 5}, hosp: map[entity.EntityID]entity.EntityID{pawn: wall}}
	bio := g.sk.bios[pawn]
	bio.Traits = []trait.ID{1}
	bio.Conditions = []pawngen.Condition{1}
	g.sk.bios[pawn] = bio
	g.memories = map[entity.EntityID]mood.Memories{pawn: {{Expires: 1000, Value: 3}}}
	g.troughs = map[entity.EntityID]troughState{wall: {Food: map[item.Kind]int{item.Wood: 3}}}
	g.pens = []penSave{{Marker: wall, Settings: PenSettings{Species: []PenSpecies{{Key: "deer", Allow: true}}, AutoCut: []string{"oak"}}}}
	g.wl.Departs = []departure{{IDs: []entity.EntityID{deer}, Last: []Position{{X: 12, Y: 8}}}}
	g.wl.Bursts = []burstStamp{{X: 2, Y: 3}}
	g.wl.CropRaids = []cropRaid{{X: 2, Y: 3}}
	g.wl.Gnaw = gnawEvent{IDs: []entity.EntityID{deer}, Chew: []entity.EntityID{wall}}
	g.wl.Forage.GaveUp(deer, 1, 2, 1)
	g.wl.Forage.Far = map[entity.EntityID]bool{deer: true}
	g.wl.Herd = herd.Settings{Caps: map[string]herd.Caps{"deer": {Males: 3}}}
	g.wl.Rage = rage.Memory{Bash: map[entity.EntityID]rage.Bash{deer: {X: 1}}, Lock: map[entity.EntityID]rage.Lock{deer: {Foe: pawn}}, Call: map[uint32]rage.Call{1: {Foe: pawn}}}
	a, _ := g.animals.Get(deer)
	a.Path = []pathing.Point{{X: 12, Y: 8}, {X: 11, Y: 8}}
	a.Body = &body.Body{Wounds: []body.Wound{{Dmg: 1}}, Fitted: []item.Kind{item.Wood}, Conds: []body.Condition{{}}}
	g.animals.Set(deer, a)
	g.incidents.Last = map[incident.ID]uint64{1: 4}
	g.incidents.Queue = []incident.Queued{{ID: 1, At: 50}}
	g.raidGroups = []raid.Group{{Pods: []raid.Pod{}, Mortars: []entity.EntityID{wall}}}
	g.haul.storedAt = map[entity.EntityID]uint64{stack: 4}
	g.haul.deliveries = map[entity.EntityID][]haulShare{pawn: {{}}}
	fields := []letterdata.Field{{Name: "who", Value: "Bo"}}
	g.messages = append(g.messages, MessageView{Text: "test", Fields: &fields})
	g.letters.Import([]letters.Letter{{ID: 1, Fields: fields, Choices: letters.ShelterChoices()}}, 1)
	g.addHistory("test", "before", pawn, deer)
	g.Apply(command.Command{Kind: command.DesignateHarvest, Tick: uint64(g.Tick()) + 50})
	auditFrozen(t, g)
	stamp := savefmt.Stamp{Name: "rare"}
	var want bytes.Buffer
	if err := EncodeSaveAs(g, &want, stamp); err != nil {
		t.Fatal(err)
	}
	f, err := FreezeSaveAs(g, stamp)
	if err != nil {
		t.Fatal(err)
	}
	// Change nested storage in place (not just replace top-level pointers).
	j.Path[0].X++
	j.Route[0]++
	j.Pickups[0]++
	pack.Items[0].Stack.Qty++
	g.craft.Bills.Export()[wall][0].Forbid[0] = item.Steel
	g.bodies[pawn].Wounds[0].Dmg++
	g.drugs[pawn].Tolerance[0]++
	g.surg.ops[pawn][0].Part++
	g.sk.bios[pawn].Traits[0]++
	g.memories[pawn][0].Expires++
	g.troughs[wall].Food[item.Wood]++
	g.pens[0].Settings.AutoCut[0] = "pine"
	g.wl.Departs[0].Last[0].X++
	g.wl.Gnaw.IDs[0]++
	g.wl.Forage.Skip[deer][0].X++
	g.wl.Rage.Bash[deer] = rage.Bash{X: 9}
	a.Path[0].X++
	a.Body.Wounds[0].Dmg++
	g.incidents.Queue[0].At++
	fields[0].Value = "changed"
	g.history[len(g.history)-1].Pawns[0]++
	g.cmds.Items()[g.cmds.Len()-1].X++
	g.Apply(command.Command{Kind: command.DesignateHarvest, Tick: uint64(g.Tick()) + 25})
	if got := frozenRaw(t, f); !bytes.Equal(got, want.Bytes()) {
		t.Fatal("rare payload mutation escaped ownership boundary")
	}
}

func TestFrozenSaveWorldTradeMutation(t *testing.T) {
	g := worldGame(t)
	tile := nearDest(t, g)
	pawn := g.pawnIDs[0]
	cid := visitingMarket(t, g, tile, pawn)
	travelOf(g).Carry(cid, item.Stack{Kind: item.Silver, Qty: 800})
	travelOf(g).Carry(cid, item.Stack{Kind: item.Parka, Qty: 1, Stuff: item.Cloth, Quality: 4, Wear: 10})
	stamp := savefmt.Stamp{Name: "market"}
	var want bytes.Buffer
	if err := EncodeSaveAs(g, &want, stamp); err != nil {
		t.Fatal(err)
	}
	tick, hash := g.Tick(), g.StateHash()
	f, err := FreezeSaveAs(g, stamp)
	if err != nil {
		t.Fatal(err)
	}
	// Execute the real settlement trade after capture, changing both the
	// stock and cargo. The queued save must still describe the earlier visit.
	g.Enqueue(command.Command{Kind: command.WorldCaravanTrade, Target: cid, Pawn: uint64(pawn), Name: trade.EncodeOffer([]trade.Delta{{Kind: item.Parka, Qty: -1}})})
	g.ApplyPending()
	if cargoQty(travelOf(g).Goods(cid), item.Parka) != 0 {
		t.Fatal("native sale did not commit")
	}
	type result struct {
		c   []byte
		err error
	}
	done := make(chan result, 1)
	go func() {
		c, e := f.Encode(nil)
		if e != nil {
			done <- result{err: e}
			return
		}
		b, e := c.CanonicalBytes(nil)
		done <- result{b, e}
	}()
	worldForward(g, 300)
	r := <-done
	if r.err != nil || !bytes.Equal(r.c, want.Bytes()) {
		t.Fatalf("market mutation changed captured bytes: %v", r.err)
	}
	var z bytes.Buffer
	if err := savefmt.Compress(&z, r.c); err != nil {
		t.Fatal(err)
	}
	h, _, err := Load(&z)
	if err != nil || h.Tick() != tick || h.StateHash() != hash {
		t.Fatalf("captured market state changed: %v", err)
	}
	if cargoQty(travelOf(h).Goods(cid), item.Parka) != 1 {
		t.Fatal("captured cargo was sold by later transaction")
	}
}

// The upstream queue and the native market/frozen-save contracts meet here:
// a future exact-lot trade stays pending, survives detached serialization,
// and reaches the same market and authoritative state at its target tick.
func TestFrozenScheduledMarketReplay(t *testing.T) {
	g := worldGame(t)
	tile := nearDest(t, g)
	pawn := g.pawnIDs[0]
	cid := visitingMarket(t, g, tile, pawn)
	travelOf(g).Carry(cid, item.Stack{Kind: item.Silver, Qty: 800})
	travelOf(g).Carry(cid, item.Stack{Kind: item.Parka, Qty: 1, Stuff: item.Cloth, Quality: 4, Wear: 10})
	market := g.WorldMap().Markets[0]
	var lot uint64
	for _, row := range market.Rows {
		if row.ItemKind == item.Parka && row.CargoQty == 1 {
			lot = row.StackID
		}
	}
	if lot == 0 || market.Quote == 0 {
		t.Fatal("fixture has no exact sale lot or quote")
	}
	c := command.Command{Kind: command.WorldCaravanTrade, Target: cid, Pawn: uint64(pawn),
		Tick: uint64(g.Tick()) + 2, Quote: market.Quote,
		Name: trade.EncodeOffer([]trade.Delta{{Kind: item.Parka, StackID: lot, Qty: -1}})}
	g.Apply(c)
	pending := g.cmds.Items()[g.cmds.Len()-1]
	last, hash := g.cmds.Last(), g.StateHash()
	f, err := FreezeSaveAs(g, savefmt.Stamp{Name: "scheduled-market"})
	if err != nil {
		t.Fatal(err)
	}
	g.Step(2)
	if g.cmds.Len() != 1 || cargoQty(travelOf(g).Goods(cid), item.Parka) != 1 {
		t.Fatal("future trade executed early")
	}
	g.Step(1)
	if g.cmds.Len() != 0 || cargoQty(travelOf(g).Goods(cid), item.Parka) != 0 {
		t.Fatalf("due trade did not execute: %+v", g.WorldMap().MarketReply)
	}
	var z bytes.Buffer
	if err := savefmt.Compress(&z, frozenRaw(t, f)); err != nil {
		t.Fatal(err)
	}
	h, _, err := Load(&z)
	if err != nil {
		t.Fatal(err)
	}
	h.worldS().planet, h.worldS().planetTried = testPlanet().Fork(), true
	if h.StateHash() != hash || h.cmds.Len() != 1 || h.cmds.Items()[0] != pending || h.cmds.Last() != last {
		t.Fatal("captured command envelope, offer or counter changed")
	}
	h.Step(2)
	if h.cmds.Len() != 1 || cargoQty(travelOf(h).Goods(cid), item.Parka) != 1 {
		t.Fatal("restored future trade executed early")
	}
	h.Step(1)
	if h.cmds.Len() != 0 || cargoQty(travelOf(h).Goods(cid), item.Parka) != 0 || h.StateHash() != g.StateHash() {
		t.Fatal("restored exact-lot trade diverged")
	}
}
