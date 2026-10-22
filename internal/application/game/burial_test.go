package game

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// burGame is a bare map with Ada standing at (3, 5).
func burGame(t *testing.T, w, h int) (*Game, entity.EntityID) {
	t.Helper()
	g := New(Config{Seed: 3, Width: w, Height: h})
	ada, err := g.SpawnNamedPawn(3, 5, "Ada")
	if err != nil {
		t.Fatal(err)
	}
	g.SetNeeds(ada, needs.Full, needs.Full)
	return g, ada
}

// burKill spawns a named colonist at (x, y) and kills it there.
func burKill(t *testing.T, g *Game, x, y int, name string) entity.EntityID {
	t.Helper()
	id, err := g.SpawnNamedPawn(x, y, name)
	if err != nil {
		t.Fatal(err)
	}
	g.die(id, health.CauseInjury)
	return id
}

// burBody is the corpse (or bones) of pawn of.
func burBody(g *Game, of entity.EntityID) (entity.EntityID, item.Stack, bool) {
	for _, id := range g.itemIDs {
		if st, ok := g.stacks.Get(id); ok && st.Of == of && (st.Kind == item.CorpseHuman || st.Kind == item.Bones) {
			return id, st, true
		}
	}
	return 0, item.Stack{}, false
}

// burGrave places a finished grave at (x, y) and returns its id.
func burGrave(t *testing.T, g *Game, f construction.Facing, x, y int) entity.EntityID {
	t.Helper()
	placeBuilt(t, g, construction.Grave, f, x, y)
	id := g.layerBldgAt(x, y, construction.LayerObject)
	if b, ok := g.bldgs.Get(id); !ok || b.Kind != construction.Grave {
		t.Fatalf("no grave at %d,%d", x, y)
	}
	return id
}

// burRun steps one tick at a time (keeping every colonist fed and
// rested) until done reports true or limit ticks pass.
func burRun(g *Game, limit int, done func() bool) bool {
	for i := 0; i < limit; i++ {
		if i%100 == 0 {
			for _, id := range g.pawnIDs {
				if g.alive(id) {
					g.SetNeeds(id, needs.Full, needs.Full)
				}
			}
		}
		g.Step(1)
		if done() {
			return true
		}
	}
	return false
}

func burRemembers(g *Game, id entity.EntityID, k mood.Kind) bool {
	for _, m := range g.memories[id].Prune(uint64(g.clock.Now())) {
		if m.Kind == k {
			return true
		}
	}
	return false
}

func burThinks(g *Game, id entity.EntityID, k mood.Kind) bool {
	for _, th := range g.thoughts(id) {
		if th.Kind == k {
			return true
		}
	}
	return false
}

func burMessage(g *Game, text string) bool {
	for _, m := range g.World().Messages {
		if m.Text == text {
			return true
		}
	}
	return false
}

func TestColonistDeathSpawnsCorpse(t *testing.T) {
	g, _ := burGame(t, 16, 10)
	bo, _ := g.SpawnNamedPawn(8, 5, "Bo")
	g.equip[bo] = item.Rifle
	g.die(bo, health.CauseInjury)

	cid, st, ok := burBody(g, bo)
	if !ok || st.Kind != item.CorpseHuman || st.Qty != 1 || st.X != 8 || st.Y != 5 {
		t.Fatalf("Bo's corpse should lie where Bo fell: %+v (found %v)", st, ok)
	}
	if st.DiedAt != uint64(g.clock.Now()) {
		t.Fatalf("the corpse remembers when Bo died: %d", st.DiedAt)
	}
	if _, armed := g.equip[bo]; armed {
		t.Fatal("the dead drop their weapon")
	}
	if n := itemsOnGround(g, "rifle"); n != 1 {
		t.Fatalf("Bo's rifle should be on the ground, found %d", n)
	}
	pv := pawnView(g, bo)
	if pv.Status != "dead" || !pv.Embodied {
		t.Fatalf("a dead colonist is drawn as its body: %+v", pv)
	}
	for _, iv := range g.World().Items {
		if iv.ID == cid && (iv.Of != bo || iv.OfName != "Bo" || iv.Kind != "human corpse") {
			t.Fatalf("corpse view: %+v", iv)
		}
	}
}

func TestRaiderDeathLeavesCorpseNotRemoved(t *testing.T) {
	g, _ := raidGame(t)
	g.Raid(1)
	rid := g.raiderIDs[0]
	weapon := g.equip[rid]
	g.die(rid, health.CauseInjury)
	g.Step(2 * clock.TicksPerHour) // long past the old half-hour linger
	if len(g.raiderIDs) != 1 || g.raiderIDs[0] != rid {
		t.Fatal("a dead raider is no longer cleared away: its body stays")
	}
	if _, st, ok := burBody(g, rid); !ok || st.Kind != item.CorpseHuman {
		t.Fatal("a dead raider leaves a corpse")
	}
	if _, ok := stackOf(g, weapon); !ok {
		t.Fatalf("its %s should be left behind", weapon)
	}
	if pv := pawnView(g, rid); !pv.Embodied || !pv.Hostile {
		t.Fatalf("the raider is drawn as its body: %+v", pv)
	}
	if s, n := g.Raiders(); s != 0 || n != 1 {
		t.Fatalf("raiders %d/%d", s, n)
	}
}

func TestCorpseRotsToBones(t *testing.T) {
	g, _ := burGame(t, 16, 10)
	jumpTo(g, 15, 0) // summer: full rot speed
	bo := burKill(t, g, 8, 5, "Bo")
	cid, st, _ := burBody(g, bo)
	st.Rot = 2*st.Shelf() - 1
	g.stacks.Set(cid, st)
	g.Step(climateInterval)
	_, st, ok := burBody(g, bo)
	if !ok || st.Kind != item.Bones || st.Of != bo {
		t.Fatalf("a long-rotten body becomes Bo's bones, not nothing: %+v %v", st, ok)
	}
	g.Step(10 * climateInterval)
	if _, st, ok = burBody(g, bo); !ok || st.Kind != item.Bones || st.Rotten() {
		t.Fatal("bones never spoil")
	}
	for _, iv := range g.World().Items {
		if iv.Kind == "bones" && (iv.OfName != "Bo" || iv.FreshDays != -1) {
			t.Fatalf("bones view: %+v", iv)
		}
	}
}

func TestGraveBuildAndBury(t *testing.T) {
	g, ada := burGame(t, 24, 12)
	bo := burKill(t, g, 8, 5, "Bo")
	g.Enqueue(command.Command{Kind: command.DesignateBuild, Build: uint8(construction.Grave), Facing: uint8(construction.South), X: 16, Y: 4})

	seen := map[string]bool{}
	gid := entity.EntityID(0)
	buried := burRun(g, 6000, func() bool {
		pv := pawnView(g, ada)
		seen[pv.Action] = true
		if pv.Action == ActionHaul && pv.Carrying != "human corpse" {
			t.Fatalf("Ada carries the body: %+v", pv)
		}
		if gid == 0 {
			if id := g.layerBldgAt(16, 4, construction.LayerObject); id != 0 {
				gid = id
				if !g.grid.Walkable(16, 4) || !g.grid.Walkable(16, 5) {
					t.Fatal("an empty grave is walkable")
				}
			}
		}
		_, _, there := burBody(g, bo)
		return !there
	})
	if !buried {
		t.Fatal("Ada should dig a grave and bury Bo")
	}
	for _, a := range []string{ActionDig, ActionHaul, ActionBury} {
		if !seen[a] {
			t.Fatalf("Ada never showed %q (saw %v)", a, seen)
		}
	}
	if seen[ActionBuild] {
		t.Fatal("digging a grave reads as dig, not build")
	}
	bv, ok := building(g, 16, 4, "grave")
	if !ok || bv.Occupant != "Bo" || !strings.Contains(bv.Epitaph, "Bo") || bv.Headstone != headstoneOf(bo) {
		t.Fatalf("the grave records Bo: %+v", bv)
	}
	rec := g.graves[gid]
	if rec.Occupant != bo || rec.Name != "Bo" || rec.Cause != health.CauseInjury.String() || rec.DiedDay != 1 || rec.BuriedDay != 1 {
		t.Fatalf("grave record %+v", rec)
	}
	if !burMessage(g, "Bo was laid to rest") {
		t.Fatal("the burial is announced")
	}
	if g.grid.Walkable(16, 4) || g.grid.Walkable(16, 5) {
		t.Fatal("a filled grave is a mound: it blocks movement")
	}
	if p := pos(g, ada); p == (Position{16, 4}) || p == (Position{16, 5}) {
		t.Fatalf("Ada is not left standing in the mound: %+v", p)
	}
	if _, held := g.reserve.EntityOwner(gid); held {
		t.Fatal("the grave's reservation is released")
	}
}

func TestBuryPrefersNearestCorpseAndGrave(t *testing.T) {
	g, ada := burGame(t, 30, 12)
	far := burKill(t, g, 20, 5, "Far")
	near := burKill(t, g, 7, 5, "Near")
	gFar := burGrave(t, g, construction.South, 26, 4)
	gNear := burGrave(t, g, construction.South, 10, 4)
	burGrave(t, g, construction.South, 1, 1) // nearer Ada, further from the body

	if !g.assignBury(ada) {
		t.Fatal("there is a body and an empty grave")
	}
	j, _ := g.jobs.Get(ada)
	nearID, _, _ := burBody(g, near)
	if j.Kind != job.Bury || j.Target != nearID || j.Dest != gNear {
		t.Fatalf("Ada buries the nearest body in the grave nearest it: %+v", j)
	}
	if o, _ := g.reserve.EntityOwner(nearID); o != ada {
		t.Fatal("the body is reserved")
	}
	if o, _ := g.reserve.EntityOwner(gNear); o != ada {
		t.Fatal("the grave is reserved")
	}

	// a second hauler takes what is left
	cy, _ := g.SpawnNamedPawn(4, 6, "Cy")
	if !g.assignBury(cy) {
		t.Fatal("the far body and grave are still free")
	}
	j, _ = g.jobs.Get(cy)
	farID, _, _ := burBody(g, far)
	if j.Target != farID || j.Dest != gFar {
		t.Fatalf("Cy takes the unreserved body and grave: %+v", j)
	}
}

func TestNoBuryWithoutEmptyGrave(t *testing.T) {
	g, ada := burGame(t, 24, 12)
	if g.assignBury(ada) {
		t.Fatal("nobody to bury")
	}
	bo := burKill(t, g, 8, 5, "Bo")
	if g.assignBury(ada) {
		t.Fatal("no grave: nothing to bury in")
	}
	full := burGrave(t, g, construction.South, 14, 4)
	cy := burKill(t, g, 12, 8, "Cy")
	cid, _, _ := burBody(g, cy)
	g.layInGrave(full, cid)
	if g.assignBury(ada) {
		t.Fatal("a filled grave takes nobody else")
	}
	empty := burGrave(t, g, construction.South, 18, 4)
	dee, _ := g.SpawnNamedPawn(1, 1, "Dee")
	g.reserve.TryEntity(dee, empty)
	if g.assignBury(ada) {
		t.Fatal("a grave reserved by another hauler is taken")
	}
	g.reserve.ReleaseOwner(dee)
	if !g.assignBury(ada) {
		t.Fatal("the empty grave is free again")
	}
	if _, _, ok := burBody(g, bo); !ok {
		t.Fatal("assigning does not move the body yet")
	}
}

func TestPyreCremateBill(t *testing.T) {
	g, ada := burGame(t, 24, 12)
	placeBuilt(t, g, construction.Pyre, construction.South, 12, 4)
	pyre := g.layerBldgAt(12, 4, construction.LayerObject)
	if bills := g.bills[pyre]; len(bills) != 1 || bills[0].Recipe != recipe.Cremate || bills[0].Mode != recipe.Forever {
		t.Fatalf("a new pyre cremates forever: %+v", bills)
	}
	g.SpawnStack(item.Wood, 10, 6, 8)
	bo := burKill(t, g, 8, 5, "Bo")
	cid, st, _ := burBody(g, bo)
	st.Rot = st.Shelf() + 1 // rotten bodies still burn
	g.stacks.Set(cid, st)

	burning, cremating := false, false
	done := burRun(g, 4000, func() bool {
		if bv, ok := building(g, 12, 4, "funeral pyre"); ok && bv.Burning {
			burning = true
		}
		if pawnView(g, ada).Action == ActionCremate {
			cremating = true
		}
		_, _, there := burBody(g, bo)
		return !there
	})
	if !done {
		t.Fatal("Ada should cremate Bo")
	}
	if !burning || !cremating {
		t.Fatalf("the pyre burns (%v) while Ada cremates (%v)", burning, cremating)
	}
	if bv, _ := building(g, 12, 4, "funeral pyre"); bv.Burning {
		t.Fatal("the pyre goes out when the work is done")
	}
	if w := g.stockOf(item.Wood); w != 8 {
		t.Fatalf("cremation burns 2 wood: %d left", w)
	}
	if !burMessage(g, "Bo was cremated") {
		t.Fatal("the cremation is announced")
	}
	if !burRemembers(g, ada, mood.LaidToRest) {
		t.Fatal("cremating a friend lays them to rest")
	}
	if g.bills[pyre][0].Done != 1 {
		t.Fatal("the bill counts the cremation")
	}
	// Bo's body is gone for good: loading the game does not bring it back
	var buf bytes.Buffer
	if err := g.Save(&buf, "ash"); err != nil {
		t.Fatal(err)
	}
	h, _, err := Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, ok := burBody(h, bo); ok {
		t.Fatal("a cremated colonist gets no new corpse on load")
	}
}

func TestColonistDiedThought(t *testing.T) {
	g, ada := raidGame(t)
	a := uint64ToID(ada)
	cy, _ := g.SpawnNamedPawn(16, 10, "Cy")
	g.Raid(1)
	g.die(g.raiderIDs[0], health.CauseInjury)
	if burRemembers(g, a, mood.ColonistDied) {
		t.Fatal("a dead raider is no loss to the colony")
	}
	bo, _ := g.SpawnNamedPawn(14, 10, "Bo")
	g.die(bo, health.CauseInjury)
	if !burRemembers(g, a, mood.ColonistDied) || !burRemembers(g, cy, mood.ColonistDied) {
		t.Fatal("every living colonist mourns Bo")
	}
	if burRemembers(g, bo, mood.ColonistDied) {
		t.Fatal("Bo does not mourn itself")
	}
}

func TestUnburiedCorpseThought(t *testing.T) {
	g, ada := burGame(t, 30, 12)
	g.positions.Set(ada, Position{3, 3})
	bo := burKill(t, g, 10, 3, "Bo")
	cid, st, _ := burBody(g, bo)
	if !burThinks(g, ada, mood.UnburiedCorpse) {
		t.Fatal("a body 7 tiles off, both outdoors, is upsetting")
	}
	st.X = 12
	g.stacks.Set(cid, st)
	if burThinks(g, ada, mood.UnburiedCorpse) {
		t.Fatal("9 tiles off is out of sight")
	}
	st.X, st.Kind = 5, item.Bones
	g.stacks.Set(cid, st)
	if burThinks(g, ada, mood.UnburiedCorpse) {
		t.Fatal("bones are not a corpse")
	}
	// a room: the body inside, Ada outside the wall
	box(t, g, 15, 1, 22, 8, [2]int{15, 4})
	st.X, st.Y, st.Kind = 17, 4, item.CorpseHuman
	g.stacks.Set(cid, st)
	g.positions.Set(ada, Position{12, 4})
	if burThinks(g, ada, mood.UnburiedCorpse) {
		t.Fatal("a body in another room is out of sight")
	}
	g.positions.Set(ada, Position{20, 4})
	if !burThinks(g, ada, mood.UnburiedCorpse) {
		t.Fatal("a body in the same room is upsetting")
	}
	if burThinks(g, bo, mood.UnburiedCorpse) {
		t.Fatal("the dead have no thoughts")
	}
}

func TestLaidToRestThought(t *testing.T) {
	g, ada := burGame(t, 24, 12)
	cy, _ := g.SpawnNamedPawn(1, 10, "Cy")
	g.labor.Set(cy, 0) // Cy does no work: only Ada digs and buries
	g.prio[cy] = work.Priorities{}
	bo := burKill(t, g, 8, 5, "Bo")
	burGrave(t, g, construction.South, 14, 4)
	if !burRun(g, 4000, func() bool { _, _, there := burBody(g, bo); return !there }) {
		t.Fatal("Bo should be buried")
	}
	if !burRemembers(g, ada, mood.LaidToRest) || !burRemembers(g, cy, mood.LaidToRest) {
		t.Fatal("every living colonist is comforted, not just the gravedigger")
	}
	if burRemembers(g, bo, mood.LaidToRest) {
		t.Fatal("the dead remember nothing")
	}
}

func TestFilledGraveBlocksAndCannotDeconstruct(t *testing.T) {
	g, ada := burGame(t, 24, 12)
	full := burGrave(t, g, construction.East, 10, 4) // (10,4) and (11,4)
	empty := burGrave(t, g, construction.South, 14, 4)
	g.positions.Set(ada, Position{11, 4})
	bo := burKill(t, g, 8, 8, "Bo")
	cid, _, _ := burBody(g, bo)
	g.layInGrave(full, cid)
	if g.grid.Walkable(10, 4) || g.grid.Walkable(11, 4) {
		t.Fatal("a filled grave blocks movement")
	}
	if p := pos(g, ada); p == (Position{11, 4}) {
		t.Fatal("anyone standing on the grave steps off it")
	}
	if !g.grid.Walkable(14, 4) || !g.grid.Walkable(14, 5) {
		t.Fatal("an empty grave stays walkable")
	}
	g.Enqueue(command.Command{Kind: command.DesignateDeconstruct, X: 9, Y: 3, X2: 15, Y2: 6})
	g.Step(1)
	if b, _ := g.bldgs.Get(full); b.Deconstruct {
		t.Fatal("a filled grave cannot be deconstructed")
	}
	if b, _ := g.bldgs.Get(empty); !b.Deconstruct {
		t.Fatal("an empty grave can be")
	}
}

func TestDigBuryCremateActions(t *testing.T) {
	g, ada := burGame(t, 24, 12)
	build(g, construction.Grave, construction.South, 8, 2)
	g.Step(1)
	fid := g.layerFrameAt(8, 2, construction.LayerObject)
	g.jobs.Set(ada, job.Record{Kind: job.Construct, State: job.Do, Target: fid, TX: 8, TY: 2, WorkLeft: 200})
	if pv := pawnView(g, ada); pv.Action != ActionDig || pv.ActionProgress != 0.5 {
		t.Fatalf("digging a grave: %q %v", pv.Action, pv.ActionProgress)
	}

	gid := burGrave(t, g, construction.South, 14, 4)
	bo := burKill(t, g, 3, 6, "Bo")
	cid, st, _ := burBody(g, bo)
	g.jobs.Set(ada, job.Record{Kind: job.Bury, State: job.Do, Target: cid, Dest: gid, DestX: 14, DestY: 4, WorkLeft: buryWork})
	if pv := pawnView(g, ada); pv.Action != "" {
		t.Fatalf("picking the body up is not filling yet: %q", pv.Action)
	}
	st.Carrier = ada
	g.stacks.Set(cid, st)
	g.jobs.Set(ada, job.Record{Kind: job.Bury, State: job.Goto, Target: cid, Carry: cid, Dest: gid, DestX: 14, DestY: 4, WorkLeft: buryWork})
	if pv := pawnView(g, ada); pv.Action != ActionHaul || pv.Carrying != "human corpse" {
		t.Fatalf("carrying a body: %q %q", pv.Action, pv.Carrying)
	}
	g.jobs.Set(ada, job.Record{Kind: job.Bury, State: job.Do, Target: cid, Carry: cid, Dest: gid, DestX: 14, DestY: 4, WorkLeft: buryWork / 4})
	if pv := pawnView(g, ada); pv.Action != ActionBury || pv.ActionProgress != 0.75 {
		t.Fatalf("filling the grave: %q %v", pv.Action, pv.ActionProgress)
	}

	placeBuilt(t, g, construction.Pyre, construction.South, 4, 9)
	pyre := g.layerBldgAt(4, 9, construction.LayerObject)
	g.jobs.Set(ada, job.Record{Kind: job.Bill, State: job.Do, Target: cid, Carry: cid, Dest: pyre, DestX: 4, DestY: 10, WorkLeft: 150})
	if pv := pawnView(g, ada); pv.Action != ActionCremate {
		t.Fatalf("burning a body on a pyre: %q", pv.Action)
	}
	if bv, _ := building(g, 4, 9, "funeral pyre"); !bv.Burning {
		t.Fatal("the pyre burns while it cremates")
	}
}

func TestSaveRoundTripGravesAndCorpses(t *testing.T) {
	g, _ := burGame(t, 24, 12)
	bo := burKill(t, g, 8, 5, "Bo")
	burGrave(t, g, construction.South, 14, 4)
	if !burRun(g, 4000, func() bool { _, _, there := burBody(g, bo); return !there }) {
		t.Fatal("Bo should be buried")
	}
	cy := burKill(t, g, 4, 9, "Cy") // a fresh corpse, waiting for a grave
	dee := burKill(t, g, 6, 9, "Dee")
	did, st, _ := burBody(g, dee)
	st.Kind = item.Bones
	g.stacks.Set(did, st)

	var buf bytes.Buffer
	if err := g.Save(&buf, "graveyard"); err != nil {
		t.Fatal(err)
	}
	h, _, err := Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	sameState(t, "after load", g, h)
	if !reflect.DeepEqual(g.graves, h.graves) || len(h.graves) != 1 {
		t.Fatalf("graves differ: %+v vs %+v", g.graves, h.graves)
	}
	if h.grid.Walkable(14, 4) {
		t.Fatal("the mound still blocks after loading")
	}
	for _, id := range []entity.EntityID{cy, dee} {
		n := 0
		for _, iid := range h.itemIDs {
			if s, _ := h.stacks.Get(iid); s.Of == id {
				n++
			}
		}
		if n != 1 {
			t.Fatalf("pawn %d has %d bodies after load", id, n)
		}
	}
	g.Step(500)
	h.Step(500)
	sameState(t, "running on", g, h)
}

func TestOldSaveDeadColonistGetsCorpse(t *testing.T) {
	g, _ := raidGame(t)
	// an older build: death left the pawn lying there, no corpse item
	bo, _ := g.SpawnNamedPawn(8, 5, "Bo")
	g.vital.Set(bo, health.Record{Status: health.Dead, Cause: health.CauseInjury})
	g.equip[bo] = item.Knife
	g.Raid(1)
	rid := g.raiderIDs[0]
	g.vital.Set(rid, health.Record{Status: health.Dead, Cause: health.CauseInjury})
	rp, _ := g.positions.Get(rid)
	// and one cremated under this build: no body, an ash record
	ash, _ := g.SpawnNamedPawn(2, 2, "Ash")
	g.vital.Set(ash, health.Record{Status: health.Dead, Cause: health.CauseInjury})
	g.recordAshes(ash)

	var buf bytes.Buffer
	if err := g.Save(&buf, "old"); err != nil {
		t.Fatal(err)
	}
	h, _, err := Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if _, st, ok := burBody(h, bo); !ok || st.X != 8 || st.Y != 5 || st.Kind != item.CorpseHuman {
		t.Fatalf("Bo gets a corpse where it lay: %+v %v", st, ok)
	}
	if _, st, ok := burBody(h, rid); !ok || st.X != rp.X || st.Y != rp.Y {
		t.Fatal("a dead raider from an old save gets a corpse too")
	}
	if _, _, ok := burBody(h, ash); ok {
		t.Fatal("ashes stay ashes")
	}
	if pv := pawnView(h, bo); !pv.Embodied {
		t.Fatal("Bo is drawn as its body")
	}
	if n := itemsOnGround(h, "knife"); n != 1 {
		t.Fatal("Bo's knife is dropped with the body")
	}
	// loading again adds nothing
	buf.Reset()
	if err := h.Save(&buf, "old"); err != nil {
		t.Fatal(err)
	}
	k, _, err := Load(&buf)
	if err != nil {
		t.Fatal(err)
	}
	sameState(t, "second load", h, k)
}

func TestBuriedRaiderLeavesTheRaiderList(t *testing.T) {
	g, ada := raidGame(t)
	g.Raid(1)
	rid := g.raiderIDs[0]
	name := g.name(rid)
	g.die(rid, health.CauseInjury)
	// a raider's body starts forbidden (UI-016): the player allows it
	gid := burGrave(t, g, construction.South, 12, 4)
	cid, _, _ := burBody(g, rid)
	if g.assignBury(uint64ToID(ada)) {
		t.Fatal("a forbidden body is not buried")
	}
	g.setForbidden(cid, false)
	if !burRun(g, 6000, func() bool { _, _, there := burBody(g, rid); return !there }) {
		t.Fatal("the raider's body should be buried")
	}
	if g.isRaider(rid) || len(g.raiderIDs) != 0 {
		t.Fatal("a buried raider leaves the raider list")
	}
	if rec := g.graves[gid]; rec.Name != name || rec.Occupant != rid {
		t.Fatalf("the grave keeps the raider's name: %+v", rec)
	}
	if burRemembers(g, uint64ToID(ada), mood.ColonistDied) {
		t.Fatal("nobody mourns a raider")
	}
}

func TestBuryFailsCleanlyWhenTheGraveGoes(t *testing.T) {
	g, ada := burGame(t, 24, 12)
	bo := burKill(t, g, 6, 5, "Bo")
	gid := burGrave(t, g, construction.South, 18, 4)
	if !burRun(g, 2000, func() bool { return pawnView(g, ada).Carrying == "human corpse" }) {
		t.Fatal("Ada should pick Bo up")
	}
	g.removeBuilding(gid)
	if !burRun(g, 1500, func() bool { j, _ := g.jobs.Get(ada); return j.Kind != job.Bury }) {
		t.Fatal("the burial should fail once the grave is gone")
	}
	cid, st, ok := burBody(g, bo)
	if !ok || st.Carrier != 0 {
		t.Fatalf("with the grave gone the body is put down: %+v %v", st, ok)
	}
	if _, held := g.reserve.EntityOwner(cid); held {
		t.Fatal("the body is free for the next hauler")
	}
}

func TestEpitaphNamesTheCause(t *testing.T) {
	cases := map[health.Cause]string{
		health.CauseInjury: " of wounds", health.CauseHypothermia: " of the cold", health.CauseHeatstroke: " of heatstroke",
		health.CauseInfection: " of infection", health.CauseNoFood: " of hunger", health.CauseNone: "",
	}
	for c, want := range cases {
		if got := causePhrase(c.String()); got != want {
			t.Errorf("%v: %q, want %q", c, got, want)
		}
	}
	r := graveRec{Name: "Bo", Cause: health.CauseHypothermia.String(), DiedDay: 3, BuriedDay: 5}
	if e := epitaph(r); e != "Bo. Died day 3 of the cold; laid to rest day 5." {
		t.Fatalf("epitaph %q", e)
	}
}
