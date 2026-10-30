package game

import (
	"bytes"
	"compress/gzip"
	"encoding/gob"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/internal/domain/world/terrain"
	"rimworld-go/internal/domain/world/vault"
)

// PLAY-017 §5: haulers carry only the colony's own things; what was found
// on the map waits for the player's haul mark.

// haulYard is a 24x12 map with a stockpile over x 16..22, y 1..10 and
// Ada, who only hauls, at (1, 1).
func haulYard(t *testing.T) (*Game, entity.EntityID) {
	t.Helper()
	g := New(Config{Seed: 1, Width: 24, Height: 12})
	ada, err := g.SpawnNamedPawn(1, 1, "Ada")
	if err != nil {
		t.Fatal(err)
	}
	onlyWork(g, ada, work.Haul)
	g.Enqueue(command.Command{Kind: command.DesignateStockpile, X: 16, Y: 1, X2: 22, Y2: 10})
	g.Step(1)
	return g, ada
}

// foundAt drops a found (unclaimed) stack at (x, y) and returns its id.
func foundAt(t *testing.T, g *Game, k item.Kind, qty, x, y int) entity.EntityID {
	t.Helper()
	g.spawnUnclaimed(k, qty, x, y, false)
	id := g.itemAt(x, y)
	if st, ok := g.stacks.Get(id); !ok || st.Kind != k || !st.Unclaimed || st.HaulMarked || st.Forbidden {
		t.Fatalf("no found %s at %d,%d: %+v", k, x, y, st)
	}
	return id
}

// runFed steps n rounds of four cells' walk, keeping pawns fed and rested.
func runFed(g *Game, n int, pawns ...entity.EntityID) {
	for range n {
		for _, id := range pawns {
			g.SetNeeds(id, needs.Full, needs.Full)
		}
		g.Step(TicksPerCell * 4)
	}
}

// storedQty is how much of kind k lies in a stockpile that takes it, and
// whether every stored stack of it is the colony's and unmarked.
func storedQty(g *Game, k item.Kind) (qty int, claimed bool) {
	claimed = true
	for _, id := range g.itemIDs {
		st, ok := g.stacks.Get(id)
		if !ok || st.Kind != k || st.Carrier != 0 || !g.inMatchingStockpile(st) {
			continue
		}
		qty += st.Qty
		claimed = claimed && !st.Unclaimed && !st.HaulMarked
	}
	return qty, claimed
}

// stillFound fails unless stack id lies untouched at (x, y): on the
// ground, unclaimed and unmarked.
func stillFound(t *testing.T, g *Game, id entity.EntityID, x, y int) {
	t.Helper()
	st, ok := g.stacks.Get(id)
	if !ok || st.Carrier != 0 || st.X != x || st.Y != y || !st.Unclaimed || st.HaulMarked {
		t.Fatalf("stack %d should still lie found at %d,%d: %+v %v", id, x, y, st, ok)
	}
}

func TestMapFindsSpawnUnclaimed(t *testing.T) {
	t.Parallel()
	g := siteGame(t, "ember_clan", nil)
	// siteFixture: a chunk at (30,30), ruin loot at (41,21), rubble at (42,21)
	found := map[[2]int]bool{{30, 30}: true, {41, 21}: true, {42, 21}: true}
	seen, colony := 0, 0
	for _, id := range g.itemIDs {
		st, _ := g.stacks.Get(id)
		if found[[2]int{st.X, st.Y}] {
			seen++
			if !st.Unclaimed || st.HaulMarked || st.Forbidden {
				t.Fatalf("map find at %d,%d: %+v", st.X, st.Y, st)
			}
			continue
		}
		colony++
		if st.Unclaimed || st.HaulMarked {
			t.Fatalf("the start kit is the colony's: %+v", st)
		}
	}
	if seen != 3 || colony == 0 {
		t.Fatalf("%d map finds (want 3), %d colony stacks", seen, colony)
	}
	if iv, ok := itemView(g, g.itemAt(30, 30)); !ok || !iv.Unclaimed || iv.HaulMarked {
		t.Fatalf("the item view shows the chunk found: %+v", iv)
	}
}

func TestVaultLootAndCasketWeaponsSpawnUnclaimed(t *testing.T) {
	t.Parallel()
	g := vaultGame(t, nil, nil)
	for _, at := range [][2]int{{66, 67}, {69, 67}} {
		st, ok := g.stacks.Get(g.itemAt(at[0], at[1]))
		if !ok || !st.Unclaimed || st.Forbidden {
			t.Fatalf("vault loot at %v: %+v %v", at, st, ok)
		}
	}
	// the remains' weapons lie found (and forbidden, like any raider's)
	g, _, _, _ = openOne(t, vault.Remains)
	weapons := 0
	for _, sid := range g.itemIDs {
		if st, _ := g.stacks.Get(sid); st.Kind.IsWeapon() && st.X > v1X0 && st.X < v1X1 && st.Y > v1Y0 && st.Y < v1Y1 {
			weapons++
			if !st.Unclaimed {
				t.Fatalf("a casket weapon is found: %+v", st)
			}
		}
	}
	if weapons == 0 {
		t.Fatal("no casket weapons")
	}
	// remains with no way out leave their weapon in the casket, found
	h := New(Config{Seed: 1, Width: 5, Height: 5})
	for y := range 5 {
		for x := range 5 {
			h.setTerrain(x, y, terrain.Rock)
		}
	}
	h.casketRemains(construction.Building{Kind: construction.SleeperCasket, X: 2, Y: 2}, nil)
	if st, ok := h.stacks.Get(h.itemAt(2, 2)); !ok || !st.Kind.IsWeapon() || !st.Unclaimed || st.Forbidden {
		t.Fatalf("the casket's weapon is found: %+v %v", st, ok)
	}
}

func TestRaiderAndVisitorGearSpawnUnclaimed(t *testing.T) {
	t.Parallel()
	g, ada := raidGame(t)
	g.Raid(1)
	rid := g.raiderIDs[0]
	weapon := g.equip[rid]
	g.die(rid, health.CauseInjury)
	if st, ok := stackOf(g, weapon); !ok || !st.Unclaimed || !st.Forbidden {
		t.Fatalf("a raider's %s is found and forbidden: %+v %v", weapon, st, ok)
	}
	// a visitor's weapon is found but allowed
	vid := g.entities.Next()
	g.peopleReg().add(&person{ID: vid, Where: whereVisitor})
	g.positions.Set(vid, Position{X: 3, Y: 3})
	g.equip[vid] = item.Spear
	g.dropRaiderGear(vid)
	if st, ok := g.stacks.Get(g.itemAt(3, 3)); !ok || st.Kind != item.Spear || !st.Unclaimed || st.Forbidden {
		t.Fatalf("a visitor's spear is found, not forbidden: %+v %v", st, ok)
	}
	// a colonist's weapon (beside its body) stays the colony's
	col := uint64ToID(ada)
	g.positions.Set(col, Position{X: 26, Y: 17})
	g.equip[col] = item.Rifle
	g.die(col, health.CauseInjury)
	clubs := 0
	for _, id := range g.itemIDs {
		st, _ := g.stacks.Get(id)
		if st.Kind != item.Rifle || max(abs(st.X-26), abs(st.Y-17)) > 2 {
			continue
		}
		clubs++
		if st.Unclaimed || st.Forbidden {
			t.Fatalf("a colonist's rifle is the colony's: %+v", st)
		}
	}
	if clubs != 1 {
		t.Fatalf("%d rifles by the colonist's body", clubs)
	}
}

func TestHaulerLeavesFoundThingsButHaulsColonyOutput(t *testing.T) {
	t.Parallel()
	g, ada := haulYard(t)
	chunk := foundAt(t, g, item.GraniteChunk, 1, 4, 5)
	carcass := foundAt(t, g, item.CorpseAnimal, 1, 6, 5)
	// the colony's own output, dropped the way its makers drop it
	colony := []struct {
		what string
		k    item.Kind
		x, y int
	}{
		{"crafted output", item.Knife, 3, 8},
		{"zone harvest", item.Potatoes, 5, 8},
		{"start cargo", item.Food, 7, 8},
		{"chopped wood", item.Wood, 9, 8},
		{"mined chunk", item.SlateChunk, 11, 8},
		{"hunted kill", item.CorpseAnimal, 13, 8},
	}
	for _, c := range colony {
		g.dropStack(c.k, 1, c.x, c.y)
	}
	runFed(g, 150, ada)
	for _, c := range colony {
		if q, claimed := storedQty(g, c.k); q != 1 || !claimed {
			t.Errorf("%s: %d stored (claimed %v), want 1", c.what, q, claimed)
		}
	}
	stillFound(t, g, chunk, 4, 5)
	stillFound(t, g, carcass, 6, 5)
}

func TestHaulMarkGetsAFoundStackStored(t *testing.T) {
	t.Parallel()
	g, ada := haulYard(t)
	wood := foundAt(t, g, item.Wood, 150, 4, 5) // more than one load
	carcass := foundAt(t, g, item.CorpseAnimal, 1, 6, 9)
	steel, _ := g.SpawnStack(item.Steel, 5, 5, 5) // the colony's
	g.Enqueue(command.Command{Kind: command.DesignateHaul, X: 5, Y: 6, X2: 3, Y2: 4, Enabled: true})
	g.Enqueue(command.Command{Kind: command.SetHaulMark, Target: uint64(carcass), Enabled: true})
	g.ApplyPending()
	for _, id := range []entity.EntityID{wood, carcass} {
		if st, _ := g.stacks.Get(id); !st.HaulMarked || !st.Unclaimed {
			t.Fatalf("stack %d is marked: %+v", id, st)
		}
	}
	if st, _ := g.stacks.Get(steel); st.HaulMarked {
		t.Fatal("only found stacks take a haul mark")
	}
	if iv, _ := itemView(g, wood); !iv.Unclaimed || !iv.HaulMarked {
		t.Fatalf("the item view shows the mark: %+v", iv)
	}
	for range 250 {
		g.SetNeeds(ada, needs.Full, needs.Full)
		g.Step(TicksPerCell * 4)
		for _, id := range g.itemIDs {
			st, _ := g.stacks.Get(id)
			if st.Kind != item.Wood {
				continue
			}
			if st.Carrier != 0 && (st.Unclaimed || st.HaulMarked) {
				t.Fatalf("a load picked up is the colony's: %+v", st)
			}
			if st.Carrier == 0 && st.X == 4 && st.Y == 5 && (!st.Unclaimed || !st.HaulMarked) {
				t.Fatalf("what is left where it was found keeps its mark: %+v", st)
			}
		}
	}
	if q, claimed := storedQty(g, item.Wood); q != 150 || !claimed {
		t.Fatalf("wood stored %d (claimed %v), want 150", q, claimed)
	}
	if q, claimed := storedQty(g, item.CorpseAnimal); q != 1 || !claimed {
		t.Fatalf("carcass stored %d (claimed %v)", q, claimed)
	}
	for _, iv := range g.World().Items {
		if iv.Unclaimed || iv.HaulMarked {
			t.Fatalf("a stored stack shows claimed and unmarked: %+v", iv)
		}
	}
}

func TestUnmarkAndCancelStopHauling(t *testing.T) {
	t.Parallel()
	g, ada := haulYard(t)
	a := foundAt(t, g, item.GraniteChunk, 1, 4, 5)
	b := foundAt(t, g, item.MarbleChunk, 1, 8, 5)
	c := foundAt(t, g, item.SlateChunk, 1, 10, 9)
	g.Enqueue(command.Command{Kind: command.DesignateHaul, X: 0, Y: 0, X2: 12, Y2: 10, Enabled: true})
	g.ApplyPending()
	for _, id := range []entity.EntityID{a, b, c} {
		if st, _ := g.stacks.Get(id); !st.HaulMarked {
			t.Fatalf("stack %d marked: %+v", id, st)
		}
	}
	g.Enqueue(command.Command{Kind: command.SetHaulMark, Target: uint64(a), Enabled: false})
	g.Enqueue(command.Command{Kind: command.DesignateCancel, X: 8, Y: 5, X2: 8, Y2: 5})
	g.Enqueue(command.Command{Kind: command.DesignateHaul, X: 10, Y: 9, X2: 10, Y2: 9, Enabled: false})
	g.ApplyPending()
	runFed(g, 120, ada)
	stillFound(t, g, a, 4, 5)
	stillFound(t, g, b, 8, 5)
	stillFound(t, g, c, 10, 9)
}

func TestUnmarkedStackDroppedFromAHaulUnderWay(t *testing.T) {
	t.Parallel()
	g, ada := haulYard(t)
	a := foundAt(t, g, item.GraniteChunk, 1, 12, 9)
	g.Enqueue(command.Command{Kind: command.SetHaulMark, Target: uint64(a), Enabled: true})
	for i := 0; i < 60; i++ {
		if j, _ := g.jobs.Get(ada); j.Kind == job.Haul && j.Target == a && j.Carry == 0 {
			break
		}
		g.Step(1)
	}
	if j, _ := g.jobs.Get(ada); j.Kind != job.Haul || j.Target != a || j.Carry != 0 {
		t.Fatalf("Ada heads for the marked chunk: %+v", j)
	}
	// unmarked while she walks: she leaves it
	g.Enqueue(command.Command{Kind: command.SetHaulMark, Target: uint64(a), Enabled: false})
	runFed(g, 60, ada)
	stillFound(t, g, a, 12, 9)
}

func TestDeliveryLeftoverOfAFoundStackIsStored(t *testing.T) {
	t.Parallel()
	g, ada := haulYard(t)
	foundAt(t, g, item.Wood, 150, 4, 5) // ruin loot: more than a load
	g.placeFrameStuff(construction.Wall, item.Wood, construction.South, 9, 5)
	fid := g.frameIDs[0]
	fr, _ := g.frames.Get(fid)
	need := fr.Need
	runFed(g, 150, ada)
	if fr, _ := g.frames.Get(fid); fr.Wood != need {
		t.Fatalf("the frame got %d wood, want %d", fr.Wood, need)
	}
	// the load taken (a full one) is the colony's: what the wall did not
	// use is stored; the rest stays found where it lay
	if q, claimed := storedQty(g, item.Wood); q != item.CarryLimit-need || !claimed {
		t.Fatalf("stored %d wood (claimed %v), want %d", q, claimed, item.CarryLimit-need)
	}
	st, ok := g.stacks.Get(g.itemAt(4, 5))
	if !ok || st.Qty != 150-item.CarryLimit || !st.Unclaimed {
		t.Fatalf("the found remainder stays: %+v %v", st, ok)
	}
}

func TestBillUsesFoundIngredients(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 12, Height: 6})
	id, _ := g.SpawnNamedPawn(1, 1, "Ada")
	placeBuilt(t, g, construction.StonecutterTable, construction.South, 5, 2)
	foundAt(t, g, item.GraniteChunk, 2, 1, 4)
	g.Enqueue(command.Command{Kind: command.AddBill, X: 5, Y: 2, Recipe: uint8(recipe.CutGranite), Mode: uint8(recipe.DoCount), Count: 1})
	runFed(g, 50, id)
	if g.stockOf(item.GraniteBlocks) != recipe.CutYield || g.stockOf(item.GraniteChunk) != 1 {
		t.Fatalf("blocks %d chunks %d", g.stockOf(item.GraniteBlocks), g.stockOf(item.GraniteChunk))
	}
	if st, ok := stackOf(g, item.GraniteBlocks); !ok || st.Unclaimed {
		t.Fatalf("the bill's output is the colony's: %+v", st)
	}
	if st, ok := stackOf(g, item.GraniteChunk); !ok || !st.Unclaimed {
		t.Fatalf("the chunk left over was never picked up: %+v", st)
	}
}

func TestHaulClaimMergeRules(t *testing.T) {
	t.Parallel()
	a := item.Stack{Kind: item.Wood, Qty: 3, X: 1, Y: 1}
	found := a
	found.Unclaimed = true
	marked := found
	marked.HaulMarked = true
	if item.CanMerge(a, found) || item.CanMerge(found, marked) {
		t.Fatal("found, marked and colony stacks keep apart")
	}
	if !item.CanMerge(found, found) || !item.CanMerge(marked, marked) {
		t.Fatal("like stacks merge")
	}
	g := New(Config{Seed: 1, Width: 10, Height: 8})
	f1 := foundAt(t, g, item.Wood, 3, 4, 4)
	g.spawnUnclaimed(item.Wood, 2, 4, 4, false)
	if st, _ := g.stacks.Get(f1); st.Qty != 5 || !st.Unclaimed {
		t.Fatalf("found stacks merge: %+v", st)
	}
	g.dropStack(item.Wood, 4, 4, 4)
	if st, _ := g.stacks.Get(f1); st.Qty != 5 {
		t.Fatalf("colony wood merged into the found stack: %+v", st)
	}
	// raid loot is found and forbidden: it joins only its like
	g.spawnUnclaimed(item.Steel, 2, 6, 6, true)
	g.spawnUnclaimed(item.Steel, 3, 6, 6, true)
	if st, _ := g.stacks.Get(g.itemAt(6, 6)); st.Qty != 5 || !st.Unclaimed || !st.Forbidden {
		t.Fatalf("raid loot merges: %+v", st)
	}
	// a rack or a wall moving things keeps what they were
	g.dropStackOf(item.Stack{Kind: item.Meat, Qty: 2, Unclaimed: true, HaulMarked: true}, 1, 6)
	if st, _ := g.stacks.Get(g.itemAt(1, 6)); !st.Unclaimed || !st.HaulMarked {
		t.Fatalf("dropStackOf keeps the claim: %+v", st)
	}
	g.clearItemsFrom(4, 4, [][2]int{{4, 4}})
	moved := 0
	for _, id := range g.itemIDs {
		if st, _ := g.stacks.Get(id); st.Kind == item.Wood && st.Unclaimed {
			moved += st.Qty
			if st.X == 4 && st.Y == 4 {
				t.Fatal("cleared off the tile")
			}
		}
	}
	if moved != 5 {
		t.Fatalf("clearItemsFrom keeps the claim: %d found wood", moved)
	}
	// set down on a stockpile that takes it, a found stack is stored
	g.Enqueue(command.Command{Kind: command.DesignateStockpile, X: 8, Y: 1, X2: 9, Y2: 2})
	g.ApplyPending()
	g.spawnUnclaimed(item.GraniteChunk, 1, 8, 1, false)
	if st, _ := g.stacks.Get(g.itemAt(8, 1)); st.Unclaimed || st.HaulMarked {
		t.Fatalf("a stack set down in a stockpile is the colony's: %+v", st)
	}
}

func TestDroppedCarryIsTheColonys(t *testing.T) {
	t.Parallel()
	g, ada := haulYard(t)
	sid := foundAt(t, g, item.GraniteChunk, 1, 1, 1)
	st, _ := g.stacks.Get(sid)
	st.Carrier = ada // a bill's whole-stack carry
	g.putStack(sid, st)
	g.jobs.Set(ada, job.Record{Kind: job.Bill, Carry: sid, State: job.Goto})
	g.interrupt(ada, job.Interrupted)
	if st, _ := g.stacks.Get(sid); st.Carrier != 0 || st.Unclaimed || st.HaulMarked {
		t.Fatalf("what a colonist put down is the colony's: %+v", st)
	}
}

func TestWorkersLeaveTheirOutputToHaulers(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 24, Height: 12})
	chopper, _ := g.SpawnNamedPawn(1, 2, "Ada")
	miner, _ := g.SpawnNamedPawn(1, 6, "Bo")
	onlyWork(g, chopper, work.Grow)
	onlyWork(g, miner, work.Mine)
	g.SpawnPlantSpecies("tree", 6, 2, plant.DefOf(plant.Tree).GrowTicks)
	g.setTerrain(8, 6, terrain.Rock)
	g.Enqueue(command.Command{Kind: command.DesignateCut, X: 0, Y: 0, X2: 12, Y2: 4})
	g.Enqueue(command.Command{Kind: command.DesignateMine, X: 8, Y: 6, X2: 8, Y2: 6})
	g.Enqueue(command.Command{Kind: command.DesignateStockpile, X: 16, Y: 1, X2: 22, Y2: 10})
	never := func() {
		t.Helper()
		for _, id := range g.itemIDs {
			if st, _ := g.stacks.Get(id); st.Carrier == chopper || st.Carrier == miner {
				t.Fatalf("a worker with Haul off carried %+v", st)
			}
		}
	}
	for range 120 {
		runFed(g, 1, chopper, miner)
		never()
	}
	wood, wok := stackOf(g, item.Wood)
	chunk, cok := stackOf(g, item.GraniteChunk)
	if !wok || !cok || g.inMatchingStockpile(wood) || g.inMatchingStockpile(chunk) || wood.Unclaimed || chunk.Unclaimed {
		t.Fatalf("output lies at the work spots, the colony's: wood %+v %v, chunk %+v %v", wood, wok, chunk, cok)
	}
	// a hauler takes both in
	hauler, _ := g.SpawnNamedPawn(1, 10, "Cora")
	onlyWork(g, hauler, work.Haul)
	for range 150 {
		runFed(g, 1, chopper, miner, hauler)
		never()
	}
	if q, _ := storedQty(g, item.Wood); q != plant.DefOf(plant.Tree).Wood {
		t.Fatalf("stored wood %d, want %d", q, plant.DefOf(plant.Tree).Wood)
	}
	if q, _ := storedQty(g, item.GraniteChunk); q != MineYield {
		t.Fatalf("stored chunks %d, want %d", q, MineYield)
	}
}

func TestForcedHaulTakesAFoundStack(t *testing.T) {
	t.Parallel()
	g, ada := haulYard(t)
	sid := foundAt(t, g, item.GraniteChunk, 1, 5, 5)
	g.Enqueue(command.Command{Kind: command.Prioritize, Pawn: uint64(ada), Order: command.OrderHaul, X: 5, Y: 5})
	runFed(g, 60, ada)
	if q, claimed := storedQty(g, item.GraniteChunk); q != 1 || !claimed {
		t.Fatalf("the ordered haul stores it, the colony's: %d %v", q, claimed)
	}
	if st, ok := g.stacks.Get(sid); ok && st.X == 5 && st.Y == 5 {
		t.Fatalf("still where it was found: %+v", st)
	}
}

func TestHaulClaimSaveRoundTrip(t *testing.T) {
	t.Parallel()
	g, _ := haulYard(t)
	marked := foundAt(t, g, item.GraniteChunk, 1, 4, 5)
	g.setHaulMark(marked, true)
	g.spawnUnclaimed(item.Steel, 3, 6, 5, true)
	colony, _ := g.SpawnStack(item.MarbleChunk, 1, 8, 5) // a v3 save is not migrated
	h := saveLoad(t, g)
	if st, _ := h.stacks.Get(marked); !st.Unclaimed || !st.HaulMarked {
		t.Fatalf("marked find after load: %+v", st)
	}
	if st, _ := h.stacks.Get(h.itemAt(6, 5)); !st.Unclaimed || !st.Forbidden || st.HaulMarked {
		t.Fatalf("raid loot after load: %+v", st)
	}
	if st, _ := h.stacks.Get(colony); st.Unclaimed {
		t.Fatalf("a colony chunk after a v3 load: %+v", st)
	}
	if g.StateHash() != h.StateHash() {
		t.Fatal("state differs after a round trip")
	}
}

// loadAsVersion saves g, rewrites the header to version v and loads it.
func loadAsVersion(t *testing.T, g *Game, v int) *Game {
	t.Helper()
	var buf bytes.Buffer
	if err := g.Save(&buf, "old"); err != nil {
		t.Fatal(err)
	}
	zr, err := gzip.NewReader(&buf)
	if err != nil {
		t.Fatal(err)
	}
	dec := gob.NewDecoder(zr)
	var info SaveInfo
	var s saveState
	if err := dec.Decode(&info); err != nil {
		t.Fatal(err)
	}
	if err := dec.Decode(&s); err != nil {
		t.Fatal(err)
	}
	info.Version = v
	var out bytes.Buffer
	zw := gzip.NewWriter(&out)
	enc := gob.NewEncoder(zw)
	if err := enc.Encode(info); err != nil {
		t.Fatal(err)
	}
	if err := enc.Encode(&s); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	h, _, err := Load(&out)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestMigrateClaimsFromV2(t *testing.T) {
	t.Parallel()
	g, ada := haulYard(t)
	loose, _ := g.SpawnStack(item.GraniteChunk, 1, 4, 5)
	carcass, _ := g.SpawnStack(item.CorpseAnimal, 1, 6, 5)
	stored, _ := g.SpawnStack(item.GraniteChunk, 1, 18, 5) // in the stockpile
	wood, _ := g.SpawnStack(item.Wood, 5, 8, 5)            // not a map find's kind
	body, _ := g.SpawnStack(item.CorpseHuman, 1, 10, 5)
	carried, _ := g.SpawnStack(item.MarbleChunk, 1, 1, 1)
	st, _ := g.stacks.Get(carried)
	st.Carrier = ada
	g.putStack(carried, st)
	reserved, _ := g.SpawnStack(item.SlateChunk, 1, 12, 5)
	if !g.reserve.TryEntity(ada, reserved) {
		t.Fatal("reserve")
	}
	h := loadAsVersion(t, g, 2)
	for _, c := range []struct {
		what  string
		id    entity.EntityID
		found bool
	}{
		{"loose chunk", loose, true}, {"loose carcass", carcass, true}, {"stored chunk", stored, false},
		{"wood", wood, false}, {"human body", body, false}, {"carried chunk", carried, false}, {"reserved chunk", reserved, false},
	} {
		if st, _ := h.stacks.Get(c.id); st.Unclaimed != c.found || st.HaulMarked {
			t.Errorf("%s: unclaimed %v, want %v", c.what, st.Unclaimed, c.found)
		}
	}
	// and a v2 game's hauler now leaves the migrated finds alone
	runFed(h, 60, ada)
	stillFound(t, h, loose, 4, 5)
	stillFound(t, h, carcass, 6, 5)
}
