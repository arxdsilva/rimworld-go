package game

import (
	"reflect"
	"strings"
	"testing"

	"rimworld-go/internal/domain/ai/job"
	rules "rimworld-go/internal/domain/building/aqua"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/research"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/temperature"
	"rimworld-go/internal/domain/world/weather"
)

// fishWorld is a small map in steady warm air with Ada, a good hand with
// animals. Every stock's comfort band but trout's holds 25 C.
func fishWorld(t *testing.T, seed uint64) (*Game, entity.EntityID) {
	t.Helper()
	g := New(Config{Seed: seed, Width: 30, Height: 20})
	bareGround(g)
	g.climate = temperature.Climate{YearMean: int(temperature.C(25))}
	g.HoldWeather()
	g.SetWeather(weather.Clear, 0)
	jumpTo(g, 10, 12)
	ada, err := g.SpawnNamedPawn(3, 3, "Ada")
	if err != nil {
		t.Fatal(err)
	}
	setSkill(g, ada, skill.Animals, 10)
	g.SetNeeds(ada, needs.Full, needs.Full)
	return g, ada
}

// poweredTank is a finished tank at (10, 4) on a fuelled generator.
func poweredTank(t *testing.T, g *Game) (tank, gen entity.EntityID) {
	t.Helper()
	tank = buildNow(t, g, construction.FishTank, item.Steel, construction.South, 10, 4)
	gen = onPower(t, g, 10, 4, 12, 4)
	if !g.powered(tank) {
		t.Fatal("the tank has no power")
	}
	return tank, gen
}

func setStock(g *Game, tank entity.EntityID, s rules.Species) {
	g.Enqueue(command.Command{Kind: command.SetTankStock, Target: uint64(tank), Count: int(s)})
	g.Step(1)
}

func tankView(t *testing.T, g *Game, tank entity.EntityID) TankView {
	t.Helper()
	for _, b := range g.World().Buildings {
		if b.ID == tank {
			if b.Tank == nil {
				t.Fatal("a fish tank without a tank view")
			}
			return *b.Tank
		}
	}
	t.Fatalf("no building %d in the world view", tank)
	return TankView{}
}

// stepKeep runs ticks in slices with Ada's needs topped up, until done (nil:
// never) reports true, and returns whether it did.
func stepKeep(g *Game, ada entity.EntityID, ticks int, done func() bool) bool {
	for left := ticks; left > 0; left -= 50 {
		g.SetNeeds(ada, needs.Full, needs.Full)
		g.Step(50)
		if done != nil && done() {
			return true
		}
	}
	return false
}

// fastForward moves the clock by ticks, running only the climate step that
// burns feed and steps tanks. Days of real Step are too slow under -race, and
// nothing a colonist does matters while a tank grows; the generator is kept
// full so the power holds.
func fastForward(g *Game, gen entity.EntityID, ticks int) {
	if over := int(g.clock.Now()) % climateInterval; over != 0 {
		g.clock.Advance(climateInterval - over)
	}
	for left := ticks; left > 0; left -= climateInterval {
		g.clock.Advance(climateInterval)
		if gen != 0 {
			g.setFuel(gen, g.fuelCap(gen))
		}
		g.burnFuel()
	}
}

func fishOnGround(g *Game) (qty int, stacks []item.Stack) {
	for _, id := range g.itemIDs {
		if st, ok := g.stacks.Get(id); ok && st.Kind == item.Fish {
			qty += st.Qty
			stacks = append(stacks, st)
		}
	}
	return qty, stacks
}

// fishCycle plays one tank's whole life and returns the final state hash.
func fishCycle(t *testing.T, seed uint64) uint64 {
	t.Helper()
	g, ada := fishWorld(t, seed)
	setSkill(g, ada, skill.Construction, 10)

	// build: real materials, a real frame, research on
	g.lab.Prog = research.Complete()
	spawn(t, g, item.Steel, 75, 3, 8)
	spawn(t, g, item.Steel, 75, 4, 8)
	spawn(t, g, item.Components, 2, 5, 8)
	g.Enqueue(command.Command{Kind: command.DesignateBuild, Build: uint8(construction.FishTank), Stuff: uint8(item.Steel), X: 10, Y: 4})
	var tank entity.EntityID
	if !stepKeep(g, ada, 40000, func() bool {
		tank = g.layerBldgAt(10, 4, construction.LayerObject)
		return tank != 0
	}) {
		t.Fatal("the tank was never built")
	}
	gen := onPower(t, g, 10, 4, 12, 4)

	// stock and feed: the same colonist, two works
	spawn(t, g, item.Hay, 30, 3, 6)
	setStock(g, tank, rules.Tilapia)
	heldWhileWorking := false
	if !stepKeep(g, ada, clock.TicksPerDay, func() bool {
		if j, _ := g.jobs.Get(ada); j.Kind == job.StockTank {
			if owner, held := g.reserve.EntityOwner(tank); held && owner == ada {
				heldWhileWorking = true
			}
		}
		v := tankView(t, g, tank)
		return v.Stocked && v.FeedPct == 100
	}) {
		t.Fatalf("never stocked and fed: %+v", tankView(t, g, tank))
	}
	if !heldWhileWorking {
		t.Fatal("the tank was not reserved while its job ran")
	}

	// grow: real ticks first, to prove the climate phase reaches the tanks
	if stepKeep(g, ada, 5*clock.TicksPerHour, nil); tankView(t, g, tank).GrowthPct == 0 {
		t.Fatalf("no growth after five hours: %+v", tankView(t, g, tank))
	}
	fastForward(g, gen, rules.DefOf(rules.Tilapia).GrowTicks)
	v := tankView(t, g, tank)
	if !v.Ripe || !v.Stocked || v.HealthPct != 100 {
		t.Fatalf("after the grow time: %+v", v)
	}
	if v.FeedPct >= 100 || v.FeedPct == 0 {
		t.Fatalf("feed %d%% after six days: a stocked tank eats and a full one lasts", v.FeedPct)
	}

	// net: fish on the ground, allowed, as many as Ada's skill gets, and the
	// tank starts over by itself
	if !stepKeep(g, ada, clock.TicksPerDay, func() bool { q, _ := fishOnGround(g); return q > 0 }) { // a night's sleep may come first
		t.Fatalf("never netted: %+v", tankView(t, g, tank))
	}
	qty, stacks := fishOnGround(g)
	if want := rules.NetYield(rules.Tilapia, 10); qty != want {
		t.Fatalf("netted %d fish, want %d", qty, want)
	}
	for _, st := range stacks {
		if st.Forbidden {
			t.Fatalf("netted fish start forbidden: %+v", st)
		}
	}
	if v := tankView(t, g, tank); v.Ripe || !v.Stocked || v.GrowthPct != 0 {
		t.Fatalf("after netting: %+v", v)
	}

	// cook-able: a stove's meal takes them
	in := recipe.DefOf(recipe.SimpleMeal).Input
	if sid, st := g.ingredientFor(ada, in); sid == 0 || st.Kind != item.Fish {
		t.Fatalf("a simple meal cannot use the catch: %d %+v", sid, st)
	}
	return g.StateHash()
}

func TestFishTankWholeCycleIsDeterministic(t *testing.T) {
	t.Parallel()
	if a, b := fishCycle(t, 7), fishCycle(t, 7); a != b {
		t.Fatalf("same seed, different state: %x vs %x", a, b)
	}
}

// TestFishTankResearchGate: no research, no tank.
func TestFishTankResearchGate(t *testing.T) {
	t.Parallel()
	g := NewColony(Config{Seed: 3, Width: 20, Height: 20})
	if p := g.lab.LockedBy("build:fish tank"); p != research.DefOf(research.Aquaculture).Label {
		t.Fatalf("a fish tank is locked by %q", p)
	}
	g.lab.Prog = research.Complete()
	if p := g.lab.LockedBy("build:fish tank"); p != "" {
		t.Fatalf("still locked by %q with all research done", p)
	}
}

// TestFishTankSaveRoundTrip: a stocked, half-grown tank and a job in hand come
// back identical and keep running in lock step.
func TestFishTankSaveRoundTrip(t *testing.T) {
	t.Parallel()
	g, ada := fishWorld(t, 5)
	g.climate = temperature.Temperate // what a load derives for this map; day 10 is warm enough
	tank, gen := poweredTank(t, g)
	g.setFuel(tank, rules.FeedFull)
	setStock(g, tank, rules.Tilapia)
	if !stepKeep(g, ada, 20000, func() bool { return tankView(t, g, tank).Stocked }) {
		t.Fatal("never stocked")
	}
	fastForward(g, gen, 2*clock.TicksPerDay)
	before := tankView(t, g, tank)
	if before.GrowthPct == 0 {
		t.Fatalf("no growth to save: %+v", before)
	}
	h := saveLoad(t, g)
	sameState(t, "after load", g, h)
	if !reflect.DeepEqual(tankView(t, h, tank), before) {
		t.Fatalf("view after load %+v, before %+v", tankView(t, h, tank), before)
	}
	if h.StateHash() != g.StateHash() {
		t.Fatal("a save changed the state hash")
	}
	for i := 0; i < 4; i++ {
		g.Step(2500)
		h.Step(2500)
		sameState(t, "running on", g, h)
	}
	if h.StateHash() != g.StateHash() {
		t.Fatal("the loaded game ran away from the original")
	}
}

// TestFishTankSavedPerMap: each map keeps its own tanks through a save.
func TestFishTankSavedPerMap(t *testing.T) {
	t.Parallel()
	g, _, _, _, _, _ := twoMaps(t)
	first := buildNow(t, g, construction.FishTank, item.Steel, construction.South, 10, 2)
	g.mapState = g.maps[1]
	second := buildNow(t, g, construction.FishTank, item.Steel, construction.South, 12, 2)
	tanksApply(g, second, int(rules.Trout))
	g.mapState = g.view
	tanksApply(g, first, int(rules.Tilapia))
	h := saveLoad(t, g)
	a, b := h.maps[0].tanks.Export(), h.maps[1].tanks.Export()
	if len(a) != 1 || a[0].ID != first || a[0].Tank.Species != rules.Tilapia {
		t.Fatalf("map 0 tanks %+v", a)
	}
	if len(b) != 1 || b[0].ID != second || b[0].Tank.Species != rules.Trout {
		t.Fatalf("map 1 tanks %+v", b)
	}
}

// TestFishTankAbsentFromSaveUntilUsed: a colony with no tank in use saves and
// hashes as it did before tanks existed, even one that has looked at a tank's
// view or cleared its stock.
func TestFishTankAbsentFromSaveUntilUsed(t *testing.T) {
	t.Parallel()
	plain, _ := fishWorld(t, 9)
	if plain.tanks != nil {
		t.Fatal("tanks exist before any is used")
	}
	want := plain.StateHash()
	touched, _ := fishWorld(t, 9)
	tanksOf(touched)
	if touched.StateHash() != want {
		t.Fatal("an empty set of tanks moved the state hash")
	}

	g, _ := fishWorld(t, 9)
	tank := buildNow(t, g, construction.FishTank, item.Steel, construction.South, 10, 4)
	if tankView(t, g, tank).Stocked {
		t.Fatal("a new tank is stocked")
	}
	if got := g.tanks.Export(); got != nil {
		t.Fatalf("an unused tank saves state: %+v", got)
	}
	setStock(g, tank, rules.Tilapia)
	if len(g.tanks.Export()) != 1 {
		t.Fatal("a chosen stock is not saved")
	}
	setStock(g, tank, rules.None)
	if got := g.tanks.Export(); got != nil {
		t.Fatalf("a cleared stock still saves state: %+v", got)
	}
}

// TestFishTankRefuelHysteresis: asks below a quarter, fills to full, and asks
// nothing in between, so it can't flap; an unused tank asks for nothing.
func TestFishTankRefuelHysteresis(t *testing.T) {
	t.Parallel()
	g, ada := fishWorld(t, 2)
	tank, _ := poweredTank(t, g)
	g.setFuel(g.bldgIDs[len(g.bldgIDs)-1], 0) // the generator: its want must not hide the tank's
	spawn(t, g, item.Hay, 40, 3, 6)

	wants := func(fuel int) bool {
		g.setFuel(tank, fuel)
		g.jobs.Set(ada, job.Record{})
		got := g.assignRefuel(ada)
		j, _ := g.jobs.Get(ada)
		g.reserve.ReleaseOwner(ada)
		g.jobs.Set(ada, job.Record{})
		return got && j.Dest == tank
	}
	if wants(0) {
		t.Fatal("a tank with no stock chosen asks for feed")
	}
	tanksApply(g, tank, int(rules.Tilapia))
	for _, c := range []struct {
		fuel int
		want bool
	}{{0, true}, {rules.FeedBelow - 1, true}, {rules.FeedBelow, false}, {rules.FeedFull / 2, false}, {rules.FeedFull, false}} {
		if got := wants(c.fuel); got != c.want {
			t.Fatalf("feed %d: wants %v, expected %v", c.fuel, got, c.want)
		}
	}

	// a real delivery fills it, and feed falling to just above a quarter
	// starts no second trip
	g.setFuel(tank, rules.FeedBelow-1)
	if !stepKeep(g, ada, 15000, func() bool { return g.fuel[tank] >= rules.FeedFull-rules.FeedItemTicks }) {
		t.Fatalf("feed reached only %d of %d", g.fuel[tank], rules.FeedFull)
	}
	g.setFuel(tank, rules.FeedBelow)
	trips := 0
	for i := 0; i < 40; i++ {
		g.SetNeeds(ada, needs.Full, needs.Full)
		g.Step(25)
		g.setFuel(tank, rules.FeedBelow)
		if j, _ := g.jobs.Get(ada); j.Kind == job.Refuel && j.Dest == tank {
			trips++
		}
	}
	if trips != 0 {
		t.Fatalf("%d refuel samples at exactly a quarter", trips)
	}
}

// TestFishTankEatsOnlyStockedAndPowered: feed burns while fish live and the
// power is on, and nowhere else.
func TestFishTankEatsOnlyStockedAndPowered(t *testing.T) {
	t.Parallel()
	g, ada := fishWorld(t, 4)
	tank, gen := poweredTank(t, g)
	b, _ := g.bldgs.Get(tank)
	g.setFuel(tank, rules.FeedFull)
	if g.burning(tank, b) {
		t.Fatal("no stock chosen: nothing eats")
	}
	setStock(g, tank, rules.Tilapia)
	if g.burning(tank, b) {
		t.Fatal("a chosen stock not yet in the water: nothing eats")
	}
	if !stepKeep(g, ada, 20000, func() bool { return tankView(t, g, tank).Stocked }) {
		t.Fatal("never stocked")
	}
	if !g.burning(tank, b) {
		t.Fatal("a stocked, powered tank does not eat")
	}
	g.setFuel(gen, 0)
	g.balancePower(uint64(g.clock.Now()))
	if g.burning(tank, b) {
		t.Fatal("a dark tank eats")
	}
}

// TestFishTankPowerBlinkDoesNotFlapWork (AGENTS.md: no flapping): a grid
// blinking faster than the job takes neither starts nor stops work. An empty
// tank gets one stock job, which runs to its end; a stocked tank gets none.
func TestFishTankPowerBlinkDoesNotFlapWork(t *testing.T) {
	t.Parallel()
	g, ada := fishWorld(t, 6)
	tank, gen := poweredTank(t, g)
	g.setFuel(tank, rules.FeedFull)
	setStock(g, tank, rules.Tilapia)

	starts, was := 0, false
	blink := func(ticks int) {
		on := true
		for left := ticks; left > 0; left -= 50 {
			if on = !on; on {
				g.setFuel(gen, g.fuelCap(gen))
			} else {
				g.setFuel(gen, 0)
			}
			g.balancePower(uint64(g.clock.Now()))
			g.SetNeeds(ada, needs.Full, needs.Full)
			g.Step(50)
			j, _ := g.jobs.Get(ada)
			is := j.Kind == job.StockTank || j.Kind == job.NetTank
			if is && !was {
				starts++
			}
			was = is
		}
	}
	blink(clock.TicksPerHour * 4)
	if !tankView(t, g, tank).Stocked {
		t.Fatalf("a blinking grid kept the tank from being stocked: %+v", tankView(t, g, tank))
	}
	if starts != 1 {
		t.Fatalf("%d stock jobs for one empty tank under a blinking grid", starts)
	}
	starts = 0
	blink(clock.TicksPerHour * 4)
	if starts != 0 {
		t.Fatalf("%d jobs at a stocked tank that needs nothing", starts)
	}
}

// TestFishTankUnreachableIsPutOff: a handler who can't reach a tank waits the
// cooldown out instead of trying each tick.
func TestFishTankUnreachableIsPutOff(t *testing.T) {
	t.Parallel()
	g, ada := fishWorld(t, 8)
	tank, _ := poweredTank(t, g)
	for y := 2; y <= 4; y++ {
		for x := 2; x <= 4; x++ {
			if x != 3 || y != 3 {
				buildNow(t, g, construction.Wall, item.Wood, construction.South, x, y)
			}
		}
	}
	g.Step(1) // the walls in the reach index
	setStock(g, tank, rules.Tilapia)
	if j, _ := g.jobs.Get(ada); j.Kind == job.StockTank {
		t.Fatal("a walled-in handler took the job")
	}
	if g.tanks.Assign(ada) {
		t.Fatal("a walled-in handler can be given the job")
	}
	if !g.postponedFor(ada, job.StockTank, tank, 0, 0) {
		t.Fatal("the tank out of reach is not put off")
	}
}

// TestFishTankOutOfSkillIsLeftAlone: trout ask Animals 8 of whoever handles
// them, and the picker says so.
func TestFishTankOutOfSkillIsLeftAlone(t *testing.T) {
	t.Parallel()
	g, ada := fishWorld(t, 8)
	tank, _ := poweredTank(t, g)
	setSkill(g, ada, skill.Animals, 3)
	setStock(g, tank, rules.Trout)
	if g.tanks.Assign(ada) {
		t.Fatal("Animals 3 took a job trout keep for Animals 8")
	}
	if g.postponedFor(ada, job.StockTank, tank, 0, 0) {
		t.Fatal("not skilled enough is no reason for a cooldown")
	}
	var reason string
	for _, o := range tankView(t, g, tank).Options {
		if o.ID == int(rules.Trout) {
			reason = o.Reason
		}
	}
	if !strings.Contains(reason, "8") {
		t.Fatalf("trout's reason %q", reason)
	}
	setSkill(g, ada, skill.Animals, 8)
	if !g.tanks.Assign(ada) {
		t.Fatal("Animals 8 can't take trout")
	}
}

// TestFishTankView covers the read model and the view cache.
func TestFishTankView(t *testing.T) {
	t.Parallel()
	g, ada := fishWorld(t, 3)
	tank, gen := poweredTank(t, g)
	basin := buildNow(t, g, construction.HydroponicsBasin, item.Steel, construction.South, 20, 4)

	for _, b := range g.World().Buildings {
		if b.ID == basin && b.Tank != nil {
			t.Fatal("a basin has a tank view")
		}
		if b.ID == tank && b.Fuel {
			t.Fatal("a tank's feed reads from TankView.FeedPct, not the fuel bar")
		}
	}
	v := tankView(t, g, tank)
	if v.StockID != int(rules.None) || v.Stocked || v.Trouble != "" || len(v.Options) != int(rules.NumSpecies)-1 {
		t.Fatalf("a new tank: %+v", v)
	}
	setStock(g, tank, rules.Tilapia)
	if !stepKeep(g, ada, 20000, func() bool { return tankView(t, g, tank).Stocked }) {
		t.Fatal("never stocked")
	}
	v = tankView(t, g, tank)
	if v.StockID != int(rules.Tilapia) || v.Stock == "" || v.Trouble != "No feed" {
		t.Fatalf("stocked with no feed: %+v", v)
	}
	g.setFuel(tank, rules.FeedFull/2)
	g.viewGen++
	if v = tankView(t, g, tank); v.FeedPct != 50 || v.Trouble != "" {
		t.Fatalf("half fed: %+v", v)
	}
	g.setFuel(gen, 0)
	g.balancePower(uint64(g.clock.Now()))
	g.viewGen++
	if v = tankView(t, g, tank); v.Trouble != "No power" {
		t.Fatalf("no power: %+v", v)
	}
}

// TestFishTankStockDiesWithoutPower: a day in the dark kills a stock, the
// message says why, and it can be raised again once the power is back.
func TestFishTankStockDiesWithoutPower(t *testing.T) {
	t.Parallel()
	g, ada := fishWorld(t, 11)
	tank, gen := poweredTank(t, g)
	g.setFuel(tank, rules.FeedFull)
	setStock(g, tank, rules.Tilapia)
	if !stepKeep(g, ada, 20000, func() bool { return tankView(t, g, tank).Stocked }) {
		t.Fatal("never stocked")
	}
	g.setFuel(gen, 0)
	g.balancePower(uint64(g.clock.Now()))
	if !g.powered(tank) {
		fastForward(g, 0, clock.TicksPerDay+clock.TicksPerHour)
	} else {
		t.Fatal("still powered")
	}
	g.viewGen++
	if v := tankView(t, g, tank); v.Stocked || v.StockID != int(rules.Tilapia) {
		t.Fatalf("after a dark day: %+v", v)
	}
	died, warned := 0, 0
	for _, m := range g.messages {
		if strings.Contains(m.Text, "died without power") {
			died++
		}
		if strings.Contains(m.Text, "are dying: no power") {
			warned++
		}
	}
	if died != 1 || warned != 1 {
		t.Fatalf("%d death and %d warning messages", died, warned)
	}
	if g.tanks.Assign(ada) {
		t.Fatal("restocking asked for with no power")
	}
	g.setFuel(gen, g.fuelCap(gen))
	g.balancePower(uint64(g.clock.Now()))
	if !g.tanks.Assign(ada) {
		t.Fatal("no restock once the power is back")
	}
}

// TestFishTankRemovalDropsARipeStock: removed ripe, its fish stay on the
// ground; removed growing, they are lost. Either way the state is gone.
func TestFishTankRemovalDropsARipeStock(t *testing.T) {
	t.Parallel()
	for _, ripe := range []bool{true, false} {
		g, ada := fishWorld(t, 12)
		tank, gen := poweredTank(t, g)
		g.setFuel(tank, rules.FeedFull)
		setStock(g, tank, rules.Tilapia)
		if !stepKeep(g, ada, 20000, func() bool { return tankView(t, g, tank).Stocked }) {
			t.Fatal("never stocked")
		}
		if ripe {
			fastForward(g, gen, rules.DefOf(rules.Tilapia).GrowTicks)
		}
		g.removeBuildingAs(tank, false)
		qty, _ := fishOnGround(g)
		switch {
		case ripe && qty != rules.NetYield(rules.Tilapia, 0):
			t.Fatalf("ripe tank removed: %d fish", qty)
		case !ripe && qty != 0:
			t.Fatalf("growing tank removed: %d fish", qty)
		case g.tanks.Export() != nil:
			t.Fatalf("state survived: %+v", g.tanks.Export())
		}
	}
}

// TestFishFeedsLikeMeat: a meat trough takes fish; a plant one does not.
func TestFishFeedsLikeMeat(t *testing.T) {
	t.Parallel()
	if !troughFeeds(TroughMeat, item.Fish) || !troughFeeds(TroughAny, item.Fish) || troughFeeds(TroughPlants, item.Fish) {
		t.Fatal("a trough treats fish differently from meat")
	}
}

// TestRefuelReachesATankPastAnEmptyTorch: a wood burner with no wood in sight
// does not stop the hauler from feeding the tank behind it in the queue.
func TestRefuelReachesATankPastAnEmptyTorch(t *testing.T) {
	t.Parallel()
	g, ada := fishWorld(t, 13)
	tank, _ := poweredTank(t, g)
	tanksApply(g, tank, int(rules.Tilapia))
	g.setFuel(tank, rules.FeedBelow-1)
	placeFuel(t, g, construction.TorchLamp, 20, 15)
	spawn(t, g, item.Hay, 10, 3, 6)
	if !g.assignRefuel(ada) {
		t.Fatal("no refuel job")
	}
	if j, _ := g.jobs.Get(ada); j.Dest != tank {
		t.Fatalf("refuel went to %d, not the tank %d", j.Dest, tank)
	}
}
