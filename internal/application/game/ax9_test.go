package game

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	"rimworld-go/internal/application/game/travel"
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/health"
	"rimworld-go/internal/domain/pawn/needs"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/caravan"
)

// AX-9: stasis caskets, transport pods, the long-range mineral scanner
// and the caravan hitching spot, end to end in the game.

// ax9Idle switches id's work off, so only its orders move it.
func ax9Idle(g *Game, id entity.EntityID) {
	for wt := work.Type(0); wt < work.NumTypes; wt++ {
		g.SetWork(id, wt, false)
	}
}

// ax9View is building id's view.
func ax9View(t *testing.T, g *Game, id entity.EntityID) BuildingView {
	t.Helper()
	for _, b := range g.World().Buildings {
		if b.ID == id {
			return b
		}
	}
	t.Fatalf("no view of building %d", id)
	return BuildingView{}
}

// A colonist walks into a stasis casket: off the map, its hunger, age and
// blood loss stand still for a day; ejected it comes out as it went in,
// and a casket that is destroyed breaks open.
func TestStasisCasketSuspendsAndEjects(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 1, Width: 20, Height: 12})
	ada, _ := g.SpawnNamedPawn(2, 2, "Ada")
	ax9Idle(g, ada)
	casket := buildNow(t, g, construction.StasisCasket, item.Steel, construction.South, 8, 4)
	g.SetNeeds(ada, needs.Full/2, needs.Full)
	g.Enqueue(command.Command{Kind: command.EnterCasket, Pawn: uint64(ada), Target: uint64(casket)})
	for i := 0; i < 600 && !g.isStashed(ada); i++ {
		g.Step(1)
	}
	if !g.isStashed(ada) || travelOf(g).Sleeper(casket) != ada {
		t.Fatal("Ada did not lie down in the casket")
	}
	if v := ax9View(t, g, casket); v.AX.Sleeper != "Ada" || v.AX.SleeperID != uint64(ada) {
		t.Fatalf("casket view %+v", v.AX)
	}
	v, _ := g.vital.Get(ada)
	b := g.ensureBody(ada, &v)
	b.BloodLoss = 100
	if g.isPatient(ada, b) {
		t.Fatal("a sleeper is still a patient: its illness would progress")
	}
	hunger, _ := g.need.Get(ada)
	days := g.sk.bios[ada].AgeDays
	g.Step(60000)
	if n, _ := g.need.Get(ada); n.Hunger != hunger.Hunger {
		t.Fatalf("hunger moved in stasis: %d -> %d", hunger.Hunger, n.Hunger)
	}
	if g.sk.bios[ada].AgeDays != days {
		t.Fatal("the sleeper aged")
	}
	g.Enqueue(command.Command{Kind: command.EjectCasket, Target: uint64(casket)})
	g.Step(1)
	if g.isStashed(ada) || travelOf(g).Sleeper(casket) != 0 || !g.isPatient(ada, b) {
		t.Fatal("eject did not let Ada out (and her blood loss back into care)")
	}
	if v := ax9View(t, g, casket); v.AX.Sleeper != "" {
		t.Fatalf("empty casket still shows %q", v.AX.Sleeper)
	}
	g.Enqueue(command.Command{Kind: command.EnterCasket, Pawn: uint64(ada), Target: uint64(casket)})
	for i := 0; i < 600 && !g.isStashed(ada); i++ {
		g.Step(1)
	}
	g.removeBuilding(casket) // destroyed
	if g.isStashed(ada) {
		t.Fatal("a destroyed casket kept its sleeper")
	}
	if p, ok := g.positions.Get(ada); !ok || max(abs(p.X-8), abs(p.Y-4)) > 4 {
		t.Fatalf("Ada came out at %+v", p)
	}
}

// A colonist carries a downed colonist into a casket; the order menu
// offers both rows.
func TestStasisCasketCarryDowned(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 2, Width: 20, Height: 12})
	ada, _ := g.SpawnNamedPawn(2, 2, "Ada")
	bob, _ := g.SpawnNamedPawn(4, 8, "Bob")
	ax9Idle(g, ada)
	ax9Idle(g, bob)
	casket := buildNow(t, g, construction.StasisCasket, item.Steel, construction.South, 12, 4)
	down := func() {
		v, _ := g.vital.Get(bob)
		v.Status = health.Downed
		g.vital.Set(bob, v)
	}
	down()
	labels := func(x, y int) string {
		var out []string
		for _, o := range g.OrdersAt(ada, x, y) {
			out = append(out, o.Label)
		}
		return strings.Join(out, "|")
	}
	if l := labels(4, 8); !strings.Contains(l, "Carry Bob to stasis casket") {
		t.Fatalf("orders on Bob: %s", l)
	}
	if l := labels(12, 4); !strings.Contains(l, "Enter stasis casket") {
		t.Fatalf("orders on the casket: %s", l)
	}
	g.Enqueue(command.Command{Kind: command.EnterCasket, Pawn: uint64(ada), Target: uint64(casket), Worker: uint64(bob)})
	for i := 0; i < 1500 && !g.isStashed(bob); i++ {
		if !g.isStashed(bob) {
			down()
		}
		g.Step(1)
	}
	if !g.isStashed(bob) || travelOf(g).Sleeper(casket) != bob {
		t.Fatal("Bob was not carried into the casket")
	}
	if g.care.Carry.Patient(ada) != 0 {
		t.Fatal("Ada still carries someone")
	}
}

// ax9PodRig builds a transport pod at (x, y) with a launcher west of it
// holding fuel chemfuel.
func ax9PodRig(t *testing.T, g *Game, x, y, fuel int) (pod, launcher entity.EntityID) {
	t.Helper()
	pod = buildNow(t, g, construction.TransportPod, item.Steel, construction.South, x, y)
	launcher = buildNow(t, g, construction.PodLauncher, item.Steel, construction.South, x-1, y)
	g.setFuel(launcher, fuel*chemfuelTicks)
	return pod, launcher
}

// ax9Free finds a free 4x3 patch of dry ground near home: its top-left.
func ax9Free(t *testing.T, g *Game) (int, int) {
	t.Helper()
	hx, hy := g.home()
	for r := 3; r < 20; r++ {
		for _, d := range [][2]int{{r, 0}, {-r, 0}, {0, r}, {0, -r}, {r, r}, {-r, -r}} {
			x, y := hx+d[0], hy+d[1]
			ok := true
			for dy := 0; dy < 3 && ok; dy++ {
				for dx := 0; dx < 4 && ok; dx++ {
					ok = g.dryWalkable(x+dx, y+dy) && g.layerBldgAt(x+dx, y+dy, construction.LayerObject) == 0
				}
			}
			if ok {
				return x, y
			}
		}
	}
	t.Fatal("no free patch near home")
	return 0, 0
}

// Pods are loaded by weight (a colonist walks in, a hauler brings the
// steel), launched on their launcher's chemfuel to a settlement two tiles
// away, fly (shown in the world view) and set their passenger down there
// as a visiting caravan with the steel as cargo, which comes home with it.
func TestTransportPodsLoadLaunchVisit(t *testing.T) {
	t.Parallel()
	g := worldGame(t)
	dest := nearDest(t, g)
	x, y := ax9Free(t, g)
	pod, launcher := ax9PodRig(t, g, x+1, y, 150)
	ada := g.pawnIDs[0]
	sid, err := g.SpawnStack(item.Steel, 40, x+3, y+2)
	if err != nil {
		t.Fatal(err)
	}
	g.Enqueue(command.Command{Kind: command.LoadPods, Target: uint64(pod), Enabled: true,
		Name: strings.Join([]string{entityStr(ada), entityStr(sid) + ":30"}, ",")})
	loaded := func() bool {
		v := ax9View(t, g, pod)
		return g.isStashed(ada) && v.AX.LoadKg >= 60+15
	}
	for i := 0; i < 8000 && !loaded(); i++ {
		if i%500 == 0 {
			for _, id := range g.pawnIDs {
				g.SetNeeds(id, needs.Full, needs.Full)
			}
		}
		g.Step(1)
	}
	if v := ax9View(t, g, pod); !loaded() || v.AX.CapKg != 150 || v.AX.Range != 66 || v.AX.FuelUnits != 150 {
		t.Fatalf("pod not loaded: stashed %v view %+v", g.isStashed(ada), v.AX)
	}
	g.Enqueue(command.Command{Kind: command.LaunchPods, Target: uint64(pod), Count: dest, Mode: command.PodArriveVisit})
	g.Step(1)
	if g.bldgs.Has(pod) {
		t.Fatal("the launched pod is still on the map")
	}
	if f := g.fuel[launcher] / chemfuelTicks; f != 150-caravan.PodFuel(2, 1) {
		t.Fatalf("launcher fuel %d after a two-tile flight", f)
	}
	if pods := g.World().AX.Pods; len(pods) != 1 || int(pods[0].To) != dest || pods[0].Count != 1 {
		t.Fatalf("pods in flight %+v", pods)
	}
	g.Step(2*caravan.PodTicksPerTile + 260)
	if len(g.World().AX.Pods) != 0 {
		t.Fatal("the pods never landed")
	}
	c := g.caravanOf(ada)
	if c == nil || c.C.State != caravan.Visiting || c.C.Tile != dest {
		t.Fatalf("no visiting caravan at the settlement: %+v", c)
	}
	if h := g.worldSt.ax9.Export(); h == nil || len(h.Cargo) != 1 || h.Cargo[0].Hold.Items[0].Qty != 30 {
		t.Fatalf("cargo %+v", h)
	}
	steelOnMap := func() (n int) {
		g.stacks.All(func(_ entity.EntityID, st item.Stack) {
			if st.Kind == item.Steel {
				n += st.Qty
			}
		})
		return n
	}
	before := steelOnMap()
	g.caravanHome(c)
	if steel := steelOnMap() - before; steel != 30 || g.isStashed(ada) {
		t.Fatalf("home: %d steel set down, Ada stashed %v", steel, g.isStashed(ada))
	}
}

// entityStr is an id as a command list entry.
func entityStr(id entity.EntityID) string { return strconv.FormatUint(uint64(id), 10) }

// ax9Stock puts n of k straight into pod (a quick load).
func ax9Stock(g *Game, pod entity.EntityID, k item.Kind, n int) {
	travelOf(g)
	s := g.worldSt.ax9.Export()
	if s == nil {
		s = &travel.Save{}
	}
	s.Holds = append(s.Holds, travel.PodHold{Pod: pod, Hold: travel.Hold{Items: []item.Stack{{Kind: k, Qty: n}}}})
	g.worldSt.ax9 = travel.Import(s)
}

// A gift by pod raises the settlement's goodwill by the goods' value; a
// launch out of range or with a pod on no launcher is refused.
func TestTransportPodsGiftAndRange(t *testing.T) {
	t.Parallel()
	g := worldGame(t)
	dest := nearDest(t, g)
	x, y := ax9Free(t, g)
	pod, launcher := ax9PodRig(t, g, x+1, y, 150)
	ax9Stock(g, pod, item.Silver, 300)
	fac := g.factionOf(g.settlementFaction(dest))
	before := fac.Goodwill
	g.setFuel(launcher, 2*chemfuelTicks) // 2 chemfuel: under a tile
	g.Enqueue(command.Command{Kind: command.LaunchPods, Target: uint64(pod), Count: dest, Mode: command.PodArriveGift})
	g.Step(1)
	if !g.bldgs.Has(pod) || len(g.World().AX.Pods) != 0 {
		t.Fatal("pods flew out of range")
	}
	g.setFuel(launcher, 150*chemfuelTicks)
	g.Enqueue(command.Command{Kind: command.LaunchPods, Target: uint64(pod), Count: dest, Mode: command.PodArriveGift})
	g.Step(2*caravan.PodTicksPerTile + 260)
	want := faction.GiftGoodwill(300 * item.Silver.Value())
	if got := g.factionOf(g.settlementFaction(dest)).Goodwill - before; want <= 0 || got != want {
		t.Fatalf("goodwill +%d, want +%d", got, want)
	}
}

// Pods sent to the colony's own tile come down at its drop spot.
func TestTransportPodsDropHome(t *testing.T) {
	t.Parallel()
	g := worldGame(t)
	x, y := ax9Free(t, g)
	pod, _ := ax9PodRig(t, g, x+1, y, 150)
	ax9Stock(g, pod, item.Components, 7)
	g.Enqueue(command.Command{Kind: command.LaunchPods, Target: uint64(pod), Count: g.colonyTile(), Mode: command.PodArriveDrop})
	g.Step(caravan.PodTicksPerTile + 260)
	n := 0
	g.stacks.All(func(_ entity.EntityID, st item.Stack) {
		if st.Kind == item.Components {
			n += st.Qty
		}
	})
	if n < 7 || len(g.World().AX.Pods) != 0 {
		t.Fatalf("%d components on the map after the drop", n)
	}
}

// A scanner find lays a world mining site 6-25 tiles out, shown in the
// world view; a caravan arriving there mines it and carries the lump.
func TestMineralSiteFoundAndMined(t *testing.T) {
	t.Parallel()
	g := worldGame(t)
	if !travelOf(g).Find(item.Gold) {
		t.Fatal("no site found")
	}
	sites := g.World().AX.Sites
	if len(sites) != 1 || sites[0].Mineral != "gold" {
		t.Fatalf("sites %+v", sites)
	}
	if d := testPlanet().Hops(g.colonyTile(), int(sites[0].Tile), 30); d < travel.SiteMinDist || d > travel.SiteMaxDist {
		t.Fatalf("site %d tiles out", d)
	}
	w := g.worldS()
	w.caravans = append(w.caravans, caravanRec{C: caravan.Caravan{ID: 999999, Tile: int(sites[0].Tile), State: caravan.Arrived},
		Home: g.colonyTile()})
	c := &w.caravans[len(w.caravans)-1]
	g.caravanArrived(c, uint64(g.clock.Now()))
	if c.C.State != caravan.Visiting || len(g.World().AX.Sites) != 0 {
		t.Fatalf("the caravan did not mine the site: %+v", c.C)
	}
	s := g.worldSt.ax9.Export()
	if len(s.Cargo) != 1 || s.Cargo[0].Hold.Items[0].Kind != item.Gold || s.Cargo[0].Hold.Items[0].Qty < 320 {
		t.Fatalf("cargo %+v", s.Cargo)
	}
}

// A researcher works a powered mineral scanner; when the power goes the
// sitting fails and the scanner is put off (no flap).
func TestMineralScannerOperatedNoFlap(t *testing.T) {
	t.Parallel()
	g := New(Config{Seed: 3, Width: 24, Height: 16})
	sc := buildNow(t, g, construction.MineralScanner, item.Steel, construction.South, 5, 2)
	cells, _, _ := construction.Footprint(construction.MineralScanner, construction.South, 5, 2)
	cx := 0
	for _, c := range cells {
		cx = max(cx, c[0])
	}
	gen := onPower(t, g, cx, 2, cx+1, 2)
	if !g.powered(sc) {
		t.Fatal("scanner unpowered")
	}
	ada, _ := g.SpawnNamedPawn(1, 1, "Ada")
	for _, wt := range []work.Type{work.Haul, work.Construct, work.Clean} {
		g.SetWork(ada, wt, false)
	}
	for i := 0; i < 1500; i++ {
		g.Step(1)
		if j, _ := g.jobs.Get(ada); j.Kind == job.ScanMinerals && j.State == job.Do {
			break
		}
	}
	if j, _ := g.jobs.Get(ada); j.Kind != job.ScanMinerals || j.State != job.Do {
		t.Fatalf("not scanning: %v %v", j.Kind, j.State)
	}
	g.Step(300)
	if s := g.worldSt.ax9.Export(); s == nil || len(s.Scans) != 1 || s.Scans[0].Scan.Work <= 0 {
		t.Fatalf("no scan progress: %+v", s)
	}
	g.Enqueue(command.Command{Kind: command.SetScannerTarget, Target: uint64(sc), Stuff: uint8(item.Uranium)})
	g.Step(1)
	if v := ax9View(t, g, sc); v.AX.ScanFor != "uranium" {
		t.Fatalf("scan target %q", v.AX.ScanFor)
	}
	g.setFuel(gen, 0)
	g.balancePower(uint64(g.clock.Now()))
	starts := 0
	for i := 0; i < 400; i++ {
		if i == 5 {
			g.setFuel(gen, g.fuelCap(gen))
			g.balancePower(uint64(g.clock.Now()))
		}
		prev, _ := g.jobs.Get(ada)
		g.Step(1)
		if j, _ := g.jobs.Get(ada); j.Kind == job.ScanMinerals && prev.Kind != job.ScanMinerals {
			starts++
			t.Logf("tick %d: restart after %v %v %v (powered %v)", i, prev.Kind, prev.State, prev.Fail, g.powered(sc))
		}
	}
	if starts != 0 {
		t.Fatalf("the researcher flapped back onto the scanner %d times", starts)
	}
}

// A forming caravan gathers at the hitching spot and waits there before
// it heads for the edge; once it sets off it never goes back (no flap).
func TestHitchingSpotGathersCaravan(t *testing.T) {
	t.Parallel()
	g := worldGame(t)
	dest := nearDest(t, g)
	x, y := ax9Free(t, g)
	buildNow(t, g, construction.HitchingSpot, item.Wood, construction.South, x+1, y+1)
	ada := g.pawnIDs[0]
	g.Enqueue(command.Command{Kind: command.FormCaravan, Name: entityStr(ada), Count: dest})
	g.Step(1)
	c := g.caravanOf(ada)
	if c == nil || !travelOf(g).Gathering(c.C.ID) {
		t.Fatal("the caravan is not gathering at the spot")
	}
	if j, _ := g.jobs.Get(ada); j.Kind == job.ExitMap {
		t.Fatal("Ada heads straight for the edge")
	}
	at := false
	for i := 0; i < 3000 && !at; i++ {
		g.Step(1)
		p, _ := g.positions.Get(ada)
		at = max(abs(p.X-x-1), abs(p.Y-y-1)) <= 3
	}
	if !at {
		t.Fatal("Ada never reached the hitching spot")
	}
	g.Step(caravanFormCheck * 3)
	if j, _ := g.jobs.Get(ada); j.Kind == job.ExitMap {
		t.Fatal("left before the wait was over")
	}
	left := false
	for i := 0; i < 4000 && !left; i++ {
		g.Step(1)
		j, _ := g.jobs.Get(ada)
		left = j.Kind == job.ExitMap || g.isStashed(ada)
	}
	if !left {
		t.Fatal("the caravan never set off from the spot")
	}
	cid := g.caravanOf(ada).C.ID
	for i := 0; i < 600; i++ {
		g.Step(1)
		if travelOf(g).Gathering(cid) {
			t.Fatal("the caravan went back to gathering")
		}
		if j, ok := g.jobs.Get(ada); ok && j.Kind == job.Move && !g.isStashed(ada) {
			t.Fatal("Ada walked back to the spot")
		}
	}
}

// The travel state survives the world save block.
func TestTravelStateSaves(t *testing.T) {
	t.Parallel()
	g := worldGame(t)
	if !travelOf(g).Find(0) {
		t.Fatal("no site found")
	}
	x, y := ax9Free(t, g)
	pod, _ := ax9PodRig(t, g, x+1, y, 150)
	ax9Stock(g, pod, item.Steel, 12)
	s := g.exportWorld()
	if s == nil || s.AX9 == nil || len(s.AX9.Sites) != 1 || len(s.AX9.Holds) != 1 {
		t.Fatalf("saved %+v", s)
	}
	g.importWorld(s)
	again := g.exportWorld()
	if again.AX9 == nil || !slices.EqualFunc(again.AX9.Sites, s.AX9.Sites, func(a, b travel.Site) bool { return a == b }) ||
		again.AX9.Holds[0].Hold.Items[0].Qty != 12 {
		t.Fatalf("restored %+v", again.AX9)
	}
}
