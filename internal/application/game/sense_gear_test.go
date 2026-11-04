package game

import (
	"bytes"
	"reflect"
	"rimworld-go/internal/domain/gameplay"
	"slices"
	"testing"

	"rimworld-go/internal/application/game/maps"
	"rimworld-go/internal/application/game/raid"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/inventory"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/caravan"
	"rimworld-go/pkg/mathx"
)

// packGame is a blank colony with two colonists and a pile of steel at Ada's
// feet (20 of it, nothing else of that kind anywhere).
func packGame(t *testing.T) (g *Game, ada, bo, pile entity.EntityID) {
	t.Helper()
	g = New(Config{Seed: 2, Width: 40, Height: 40})
	ada, _ = g.SpawnNamedPawn(10, 10, "Ada")
	bo, _ = g.SpawnNamedPawn(20, 20, "Bo")
	pile = stack(t, g, item.Steel, 20, 10, 11)
	return g, ada, bo, pile
}

// packed stows the whole pile in id's pack.
func packed(t *testing.T, g *Game, id, pile entity.EntityID, n int) {
	t.Helper()
	if !senseOf(g).Gear.Stow(id, pile, n) {
		t.Fatalf("could not stow %d of %d", n, pile)
	}
}

// inPlay sums kind k over everything that can hold it: every map's stacks,
// every pack and every caravan's goods.
func inPlay(g *Game, k item.Kind) int {
	total := 0
	for _, m := range g.maps {
		m.stacks.All(func(_ entity.EntityID, st item.Stack) {
			if st.Kind == k {
				total += st.Qty
			}
		})
	}
	s := senseOf(g)
	for _, p := range s.Packs { // a sum: the order doesn't matter
		for _, e := range p.Items {
			if e.Stack.Kind == k {
				total += e.Stack.Qty
			}
		}
	}
	for i := range g.worldS().caravans {
		for _, st := range travelOf(g).Goods(g.worldSt.caravans[i].C.ID) {
			if st.Kind == k {
				total += st.Qty
			}
		}
	}
	return total
}

// groundNear is the quantity of k lying loose within a tile of (x, y) on the
// current map: a stack set down beside what is there already (a corpse) spills
// to the next tile.
func groundNear(g *Game, k item.Kind, x, y int) int {
	n := 0
	for _, id := range g.itemIDs {
		if st, ok := g.stacks.Get(id); ok && st.Kind == k && st.Carrier == 0 && mathx.Abs(st.X-x) <= 1 && mathx.Abs(st.Y-y) <= 1 {
			n += st.Qty
		}
	}
	return n
}

// packOnRoad puts id on a caravan with the given other members.
func packOnRoad(t *testing.T, g *Game, ids ...entity.EntityID) entity.EntityID {
	t.Helper()
	w := g.worldS()
	cid := g.entities.Next()
	w.caravans = append(w.caravans, maps.Caravan{C: caravan.Caravan{ID: uint64(cid), State: caravan.Travelling, FoodDays: maps.FoodDays}})
	for _, id := range ids {
		if !g.stashPawn(id, cid) {
			t.Fatalf("could not stash %d", id)
		}
		c := fleetOf(g).At(cid)
		c.C.Members = append(c.C.Members, uint64(id))
	}
	return cid
}

func TestPackIsOffTheMap(t *testing.T) {
	t.Parallel()
	g, ada, _, pile := packGame(t)
	packed(t, g, ada, pile, 20)
	if g.stacks.Has(pile) || slices.Contains(g.itemIDs, pile) || g.itemAt(10, 11) != 0 {
		t.Fatal("a stowed stack is still on the map")
	}
	w := g.World()
	for _, it := range w.Items {
		if it.Kind == item.Steel.String() {
			t.Fatalf("the item list shows a stowed stack: %+v", it)
		}
	}
	if w.Steel != 0 {
		t.Fatalf("the colony's stock counts a pack: %d steel", w.Steel)
	}
	e := senseOf(g).Packs[ada].Items[0]
	if e.Stack.X != 0 || e.Stack.Y != 0 || e.Stack.Carrier != 0 || e.Stack.Qty != 20 {
		t.Fatalf("not off the map: %+v", e.Stack)
	}
	g.Step(300)
	if inPlay(g, item.Steel) != 20 || groundNear(g, item.Steel, 10, 11) != 0 {
		t.Fatalf("something saw the pack: %d in play", inPlay(g, item.Steel))
	}
}

func TestPackSpillsOnDeath(t *testing.T) {
	t.Parallel()
	g, ada, _, pile := packGame(t)
	packed(t, g, ada, pile, 20)
	pos, _ := g.positions.Get(ada)
	g.die(ada, health.CauseNone)
	if senseOf(g).Gear.Has(ada) || senseOf(g).Export() != nil {
		t.Fatalf("a dead pawn keeps a pack: %+v", senseOf(g).Packs)
	}
	if n := groundNear(g, item.Steel, pos.X, pos.Y); n != 20 {
		t.Fatalf("%d steel where it fell, want 20", n)
	}
}

// TestPackSpillKeepsTheMake: what falls is the stack that went in, whatever
// it was made of or how old it is.
func TestPackSpillKeepsTheMake(t *testing.T) {
	t.Parallel()
	g, ada, _, _ := packGame(t)
	in := item.Stack{Kind: item.Rifle, Qty: 1, Quality: 4, Wear: 3, Stuff: item.Steel, Book: 77}
	sid := stack(t, g, in.Kind, 1, 12, 12)
	st, _ := g.stacks.Get(sid)
	in.X, in.Y = st.X, st.Y
	g.putStack(sid, in)
	packed(t, g, ada, sid, 1)
	g.die(ada, health.CauseNone)
	pos, _ := g.positions.Get(ada)
	found := false
	for _, id := range g.itemIDs {
		st, _ := g.stacks.Get(id)
		if st.Kind == item.Rifle {
			found = true
			if mathx.Abs(st.X-pos.X) > 1 || mathx.Abs(st.Y-pos.Y) > 1 {
				t.Fatalf("fell at (%d, %d), the pawn at %+v", st.X, st.Y, pos)
			}
			want := in
			want.X, want.Y = st.X, st.Y
			if !reflect.DeepEqual(st, want) {
				t.Fatalf("changed in the pack\n got %+v\nwant %+v", st, want)
			}
		}
	}
	if !found {
		t.Fatal("the rifle vanished")
	}
}

// TestPackNeverVanishes walks every way a pawn leaves play, summing the kind
// over the maps, the packs and the caravans before and after.
func TestPackNeverVanishes(t *testing.T) {
	t.Parallel()
	for name, leave := range map[string]func(t *testing.T, g *Game, ada, bo entity.EntityID){
		"death": func(_ *testing.T, g *Game, ada, _ entity.EntityID) { g.die(ada, health.CauseNone) },
		"downed then stripped": func(t *testing.T, g *Game, ada, _ entity.EntityID) {
			g.hurt(ada, health.DownedInjury+10, 0)
			h := careGear{g}
			if !h.HasGear(ada) || !(ordersHost{g}).Strippable(ada) {
				t.Fatal("a pack is not something to strip")
			}
			if !h.StripGear(ada) {
				t.Fatal("nothing stripped")
			}
		},
		"arrested":    func(_ *testing.T, g *Game, ada, bo entity.EntityID) { g.makePrisoner(ada, bo) },
		"off the map": func(_ *testing.T, g *Game, ada, _ entity.EntityID) { g.removeRaider(ada) },
		"kidnapped": func(_ *testing.T, g *Game, ada, _ entity.EntityID) {
			g.raidGroups = append(g.raidGroups, raid.Group{Size: 1})
			rid := spawnRaider(g, 0, 0, item.Revolver, 11, 10)
			raidKidnap{g}.Off(rid, ada)
		},
		"left on the ship": func(t *testing.T, g *Game, ada, _ entity.EntityID) {
			casket := buildNow(t, g, construction.StasisCasket, item.Steel, construction.South, 30, 30)
			if !g.stashPawn(ada, casket) {
				t.Fatal("no casket")
			}
			shipHost{g}.Escaped(ada)
		},
		"died in a casket": func(t *testing.T, g *Game, ada, _ entity.EntityID) {
			casket := buildNow(t, g, construction.StasisCasket, item.Steel, construction.South, 30, 30)
			if !g.stashPawn(ada, casket) {
				t.Fatal("no casket")
			}
			dieAway(g, ada, health.CauseNone)
		},
		"died on the road": func(t *testing.T, g *Game, ada, bo entity.EntityID) {
			packOnRoad(t, g, ada, bo)
			dieAway(g, ada, health.CauseNone)
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			g, ada, bo, pile := packGame(t)
			packed(t, g, ada, pile, 15)
			before := inPlay(g, item.Steel)
			leave(t, g, ada, bo)
			if after := inPlay(g, item.Steel); after != before {
				t.Fatalf("%d steel in play, was %d", after, before)
			}
			if senseOf(g).Gear.Has(ada) {
				t.Fatalf("still in the pack: %+v", senseOf(g).Packs[ada])
			}
		})
	}
}

func TestPackRoadDeathGoesToTheCaravan(t *testing.T) {
	t.Parallel()
	g, ada, bo, pile := packGame(t)
	packed(t, g, ada, pile, 15)
	cid := packOnRoad(t, g, ada, bo)
	dieAway(g, ada, health.CauseNone)
	goods := travelOf(g).Goods(uint64(cid))
	if len(goods) != 1 || goods[0].Kind != item.Steel || goods[0].Qty != 15 {
		t.Fatalf("the caravan's goods: %+v", goods)
	}
}

// TestPackLostWithClosedMap: a map dropped with someone on it takes everything
// on it; the pack leaves no entry behind for a pawn who is gone.
func TestPackLostWithClosedMap(t *testing.T) {
	t.Parallel()
	g, _, _, _ := packGame(t)
	id, ok := DevOpenMap(g, -1)
	if !ok {
		t.Fatal("no map")
	}
	home := g.mapState
	g.mapState = mapByID(g, uint32(id))
	left, _ := g.SpawnNamedPawn(5, 5, "Left")
	sid := stack(t, g, item.Steel, 5, 5, 6)
	packed(t, g, left, sid, 5)
	g.mapState = home
	closeMap(g, uint32(id))
	if s := senseOf(g); s.Gear.Has(left) || len(s.FromPack) != 0 {
		t.Fatalf("an entry outlived the map: %+v", s.Packs)
	}
}

func TestPackGoesWithTheFinalWipe(t *testing.T) {
	t.Parallel()
	g, ada, _, pile := packGame(t)
	packed(t, g, ada, pile, 5)
	mapsFound{g}.Wipe()
	if s := senseOf(g); s.Gear.Has(ada) || inPlay(g, item.Steel) != 0 {
		t.Fatalf("the last colony's end kept something: %+v, %d steel", s.Packs, inPlay(g, item.Steel))
	}
}

func TestPackTravelsWithPawn(t *testing.T) {
	t.Parallel()
	g, ada, _, pile := packGame(t)
	packed(t, g, ada, pile, 20)
	packed(t, g, ada, stack(t, g, item.Meal, 2, 10, 11), 2) // food: the road doesn't rot it
	want := inventory.Pack{Items: slices.Clone(senseOf(g).Packs[ada].Items), Next: senseOf(g).Packs[ada].Next}
	if !g.stashPawn(ada, 99) {
		t.Fatal("stash failed")
	}
	g.Step(600) // two climate passes with the pawn away
	if got := senseOf(g).Packs[ada]; !reflect.DeepEqual(got, want) {
		t.Fatalf("the road changed the pack\n got %+v\nwant %+v", got, want)
	}
	if !g.unstashPawn(ada, 5, 5) {
		t.Fatal("unstash failed")
	}
	if got := senseOf(g).Packs[ada]; !reflect.DeepEqual(got, want) {
		t.Fatalf("coming home changed the pack: %+v", got)
	}
}

func TestPackRots(t *testing.T) {
	t.Parallel()
	g, ada, _, _ := packGame(t)
	sid := stack(t, g, item.Meal, 2, 10, 11)
	packed(t, g, ada, sid, 2)
	pos, _ := g.positions.Get(ada)
	rate := item.RotRate(int(g.temperatureAt(pos.X, pos.Y)))
	if rate == 0 {
		t.Fatal("fixture: frozen air")
	}
	senseOf(g).Gear.Rot(climateInterval)
	st := senseOf(g).Packs[ada].Items[0].Stack
	if want := climateInterval * rate / 1000; st.Rot != want {
		t.Fatalf("rot %d, want %d", st.Rot, want)
	}
	before := st.Rot
	g.Step(climateInterval) // and the climate phase runs it
	if got := senseOf(g).Packs[ada].Items[0].Stack.Rot; got <= before {
		t.Fatalf("the climate pass did not rot the pack: %d, was %d", got, before)
	}
	// spoiled food is gone, with a word, and an emptied pack is no pack
	p := senseOf(g).Packs[ada]
	p.Items[0].Stack.Rot = p.Items[0].Stack.Shelf()
	senseOf(g).Gear.Rot(0)
	if _, ok := senseOf(g).Packs[ada]; ok {
		t.Fatal("spoiled food stayed in the pack")
	}
	if len(g.messages) == 0 || g.messages[len(g.messages)-1].Text != "2 "+item.Meal.String()+" rotted away in Ada's pack" {
		t.Fatalf("messages: %+v", g.messages)
	}
}

func TestPackInWealth(t *testing.T) {
	t.Parallel()
	g, ada, _, pile := packGame(t)
	before := g.Wealth()
	packed(t, g, ada, pile, 20)
	if got := g.Wealth(); got != before {
		t.Fatalf("stowing moved the wealth from %d to %d", before, got)
	}
	if !g.stashPawn(ada, 99) {
		t.Fatal("stash failed")
	}
	if got, want := g.Wealth(), before-20*item.Steel.Value(); got != want {
		t.Fatalf("wealth counts a pack off this map: %d, want %d", got, want)
	}
	if !g.unstashPawn(ada, 5, 5) {
		t.Fatal("unstash failed")
	}
	g.die(ada, health.CauseNone)
	if got := g.Wealth(); got < before {
		t.Fatalf("the spill lost wealth: %d, was %d", got, before)
	}
}

func TestPackRoundTrip(t *testing.T) {
	t.Parallel()
	g, ada, bo, pile := packGame(t)
	g.Step(1) // the rooms exist: a load keeps them, and only they differ otherwise
	packed(t, g, ada, pile, 12)
	meal := stack(t, g, item.Meal, 3, 20, 21)
	packed(t, g, bo, meal, 3)
	if _, ok := senseOf(g).Gear.Unstow(bo, item.Meal); !ok {
		t.Fatal("unstow")
	}
	m2 := stack(t, g, item.Meal, 2, 20, 21)
	packed(t, g, bo, m2, 2)
	want, hash := senseOf(g).Export(), g.StateHash()
	if len(want.Packs) != 2 || len(want.FromPack) != 1 {
		t.Fatalf("fixture: %+v", want)
	}
	var buf bytes.Buffer
	if err := g.Save(&buf, "pack"); err != nil {
		t.Fatal(err)
	}
	h, _, err := Load(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if got := senseOf(h).Export(); !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip\n got %+v\nwant %+v", got, want)
	}
	if got := h.StateHash(); got != hash {
		t.Fatalf("state hash %#x, want %#x", got, hash)
	}
	// and both go on the same way, rot in the packs included
	g.Step(600)
	h.Step(600)
	if a, b := g.StateHash(), h.StateHash(); a != b {
		t.Fatalf("after 600 ticks: %#x, %#x", a, b)
	}
	// the loaded pack spills where the pawn is
	h.die(ada, health.CauseNone)
	if inPlay(h, item.Steel) != 20 {
		t.Fatalf("after loading: %d steel in play", inPlay(h, item.Steel))
	}
}

// TestEmptiedPackSavesAsNone: a colony that used a pack and emptied it hashes
// like one that didn't carry it.
func TestEmptiedPackSavesAsNone(t *testing.T) {
	t.Parallel()
	g, ada, _, pile := packGame(t)
	packed(t, g, ada, pile, 20)
	if _, ok := senseOf(g).Gear.Unstow(ada, item.Steel); !ok {
		t.Fatal("unstow")
	}
	delete(senseOf(g).FromPack, ada) // CSE-037 clears it on landing
	if s := senseOf(g).Export(); s != nil {
		t.Fatalf("an emptied pack still saves: %+v", s)
	}
}

func TestUnstowIsAnOrdinaryCarriedStack(t *testing.T) {
	t.Parallel()
	g, ada, _, pile := packGame(t)
	packed(t, g, ada, pile, 20)
	stack, ok := senseOf(g).Gear.Unstow(ada, item.Steel)
	if !ok {
		t.Fatal("unstow")
	}
	st, has := g.stacks.Get(stack)
	pos, _ := g.positions.Get(ada)
	if !has || st.Carrier != ada || st.Qty != 20 || st.X != pos.X || st.Y != pos.Y || !slices.Contains(g.itemIDs, stack) {
		t.Fatalf("carried stack %+v", st)
	}
	if senseOf(g).FromPack[ada] != stack || senseOf(g).Gear.Has(ada) || inPlay(g, item.Steel) != 20 {
		t.Fatalf("after unstow: %+v, %d in play", senseOf(g).FromPack, inPlay(g, item.Steel))
	}
	j, _ := g.jobs.Get(ada)
	j.Kind, j.Carry = job.Haul, stack
	g.jobs.Set(ada, j)
	senseOf(g).Rules = senseOf(g).Rules.With(gameplay.PutBack, false) // on, it goes back: CSE-037
	g.dropCarry(ada)                                                  // an ordinary load: it falls as any other
	if groundNear(g, item.Steel, pos.X, pos.Y) != 20 {
		t.Fatal("dropping what came out of the pack lost it")
	}
}

func TestPackInPawnView(t *testing.T) {
	t.Parallel()
	g, ada, _, pile := packGame(t)
	view := func() PawnView {
		for _, v := range g.Snapshot() {
			if v.ID == ada {
				return v
			}
		}
		t.Fatal("no view")
		return PawnView{}
	}
	if v := view(); v.Pack != nil || v.PackGrams != 0 || v.PackLimitGrams != 40000 {
		t.Fatalf("empty pack view: %+v %d %d", v.Pack, v.PackGrams, v.PackLimitGrams)
	}
	packed(t, g, ada, pile, 20)
	v := view()
	if len(v.Pack) != 1 || v.Pack[0].Name != item.Steel.String() || v.Pack[0].Qty != 20 || v.Pack[0].ID == 0 || v.PackGrams != 20*item.Steel.MassGrams() {
		t.Fatalf("pack view: %+v %d", v.Pack, v.PackGrams)
	}
}
