package ebiten

import (
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"strconv"
	"strings"
	"testing"
	"time"

	"rimworld-go/internal/adapters/inbound/ebiten/dialogs"
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/adapters/inbound/ebiten/inspector"
	"rimworld-go/internal/adapters/inbound/ebiten/worldview"
	"rimworld-go/internal/domain/economy/item"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/caravan"
	"rimworld-go/internal/domain/world/planet"
)

// journey_world_test.go: the full-screen world view (MAP-2). Every journey
// runs on a wizard colony (it has a planet and peoples); the sim states a
// gizmo needs (a caravan standing on a settlement) are set up with the
// game's own commands and steps, and the gizmo itself is pressed with the
// mouse.

// worldColony is a paused wizard colony on the tiny world, with its planet.
func worldColony(t *testing.T) (*journey, *planet.Planet) {
	t.Helper()
	j := newMenuJourney(t)
	wizardToColony(t, j)
	if j.a.renderer.speed != speedPaused {
		j.key("Space", 0)
	}
	params, ok := j.g.WorldParams()
	if !ok {
		t.Fatal("the wizard colony has no world")
	}
	return j, planet.Build(params)
}

// worldOpen opens the world view with F8.
func worldOpen(j *journey) {
	j.t.Helper()
	if !j.a.world.Open() {
		j.key("F8", 0)
	}
	if !j.a.world.Open() {
		j.t.Fatal("F8 did not open the world view")
	}
	j.update(worldFly)
}

// siteNamed is the settlement view at tile.
func siteAtTile(j *journey, tile int) game.WorldSiteView {
	j.t.Helper()
	for _, s := range j.g.WorldSites() {
		if s.Tile == tile {
			return s
		}
	}
	j.t.Fatalf("no settlement on tile %d", tile)
	return game.WorldSiteView{}
}

// worldFind flies the globe to a settlement with the search box (type the
// name, click the first result) and leaves the box open.
func worldFind(j *journey, s game.WorldSiteView) {
	j.t.Helper()
	if !hasWidget(j.a, "world.search.field") {
		j.clickID("world.search")
	}
	j.clickID("world.search.field")
	j.typeText(s.Name)
	j.update(2)
	if !j.clickID("world.search.result.0") {
		j.t.Fatal("the search result is disabled")
	}
	if want := "tile:" + strconv.Itoa(s.Tile); j.a.world.Selected() != want {
		j.t.Fatalf("search selected %q, want %q", j.a.world.Selected(), want)
	}
	j.update(worldFly)
}

// worldPick clicks the marker widget id and expects key selected.
func worldPick(j *journey, id, key string) {
	j.t.Helper()
	if !hasWidget(j.a, id) {
		j.t.Fatalf("no marker %s on screen", id)
	}
	j.settle()
	if !j.clickID(id) {
		j.t.Fatalf("marker %s refused the click", id)
	}
	if got := j.a.world.Selected(); got != key {
		j.t.Fatalf("clicking %s selected %q, want %q", id, got, key)
	}
}

// nearestFriend is a visitable settlement and its hop distance from home.
func nearestSite(j *journey, p *planet.Planet, hostile bool) (game.WorldSiteView, int) {
	j.t.Helper()
	var best game.WorldSiteView
	bd := 1 << 30
	home := j.g.WorldMap().Tile
	var sc planet.HopScratch
	for _, s := range j.g.WorldSites() {
		if (s.Relation == "hostile") != hostile {
			continue
		}
		if d := p.HopsWith(&sc, home, s.Tile, 200); d >= 0 && d < bd {
			best, bd = s, d
		}
	}
	if bd == 1<<30 {
		j.t.Fatalf("no settlement (hostile %v) in %+v", hostile, j.g.WorldSites())
	}
	return best, bd
}

// sendCaravan forms a caravan of pawn to tile with the game's own command
// (setup) and runs the world until it is standing there.
func sendCaravan(j *journey, pawn entity.EntityID, tile int) game.CaravanView {
	j.t.Helper()
	j.g.Enqueue(command.Command{Kind: command.FormCaravan, Name: strconv.FormatUint(uint64(pawn), 10), Count: tile})
	for i := 0; i < 2500; i++ {
		j.g.Step(60)
		for _, c := range j.g.WorldMap().Caravans {
			if c.Tile == tile && c.State != caravan.Travelling.Label() && c.State != caravan.Forming.Label() {
				return c
			}
		}
	}
	j.t.Fatalf("the caravan never reached tile %d: %+v", tile, j.g.WorldMap().Caravans)
	return game.CaravanView{}
}

// formCaravan forms a caravan of pawn bound for tile with the game's own
// command and runs the world until it is on the road. The long walk itself
// is TestJourneyWorldTravelAndReturn's; a journey about what happens once
// there follows this with game.DevArrive.
func formCaravan(j *journey, pawn entity.EntityID, tile int) game.CaravanView {
	j.t.Helper()
	j.g.Enqueue(command.Command{Kind: command.FormCaravan, Name: strconv.FormatUint(uint64(pawn), 10), Count: tile})
	for i := 0; i < 500; i++ {
		j.g.Step(60)
		for _, c := range j.g.WorldMap().Caravans {
			if c.State == caravan.Travelling.Label() {
				return c
			}
		}
	}
	j.t.Fatalf("the caravan never left for tile %d: %+v", tile, j.g.WorldMap().Caravans)
	return game.CaravanView{}
}

// TestJourneyWorldViewOpensSelectsAndCloses: F8 shows the globe with the
// colony selected; Esc clears the selection, then closes; a double click on
// the colony's marker enters it.
func TestJourneyWorldViewOpensSelectsAndCloses(t *testing.T) {
	t.Parallel()
	j := newMenuJourney(t)
	wizardToColony(t, j)
	if j.a.renderer.speed != speedPaused {
		j.key("Space", 0)
	}
	j.key("F8", 0)
	if !j.a.world.Open() {
		t.Fatal("F8 did not open the world view")
	}
	j.update(worldFly)
	if j.a.world.Selected() != "map:0" {
		t.Fatalf("selected %q, want the colony", j.a.world.Selected())
	}
	if !hasWidget(j.a, "world.act.jump") || !hasWidget(j.a, "world.obj.map.0") {
		t.Fatal("no gizmos or no colony marker")
	}
	_ = j.a.hud() // the layout with the view open never panics
	j.draw(2)
	j.key("Escape", 0)
	if !j.a.world.Open() || j.a.world.Selected() != "" {
		t.Fatalf("Esc: open %v selected %q", j.a.world.Open(), j.a.world.Selected())
	}
	j.key("Escape", 0)
	if j.a.world.Open() {
		t.Fatal("the second Esc did not close the view")
	}
	j.draw(2) // the map draws again
	j.key("F8", 0)
	j.update(worldFly)
	first := j.clickID("world.obj.map.0")
	second := j.clickID("world.obj.map.0")
	if !first || !second {
		t.Fatal("the colony marker refused a click")
	}
	if j.a.world.Open() {
		t.Fatal("a double click on the colony did not enter it")
	}
}

// reason is gizmo id's disabled reason, and whether it is disabled.
func gizmoWhy(j *journey, id string) (string, bool) {
	j.t.Helper()
	w, ok := j.a.uiSt().Find(id)
	if !ok {
		j.t.Fatalf("no gizmo %s", id)
	}
	return w.Reason, w.Disabled
}

// atSettlement is a wizard colony whose caravan stands on its nearest
// friendly settlement, selected in the world view.
func atSettlement(t *testing.T) (j *journey, p *planet.Planet, s game.WorldSiteView, c game.CaravanView) {
	t.Helper()
	j, p = worldColony(t)
	s, _ = nearestSite(j, p, false)
	c = sendCaravan(j, j.g.Snapshot()[0].ID, s.Tile)
	worldOpen(j)
	worldFind(j, s)
	worldPick(j, "world.obj.caravan."+strconv.FormatUint(c.ID, 10), "caravan:"+strconv.FormatUint(c.ID, 10))
	return j, p, s, c
}

// bareTile is a landable tile within two hops of home that holds no
// settlement.
func bareTile(j *journey, p *planet.Planet) int {
	j.t.Helper()
	home := j.g.WorldMap().Tile
	taken := map[int]bool{}
	for _, s := range j.g.WorldSites() {
		taken[s.Tile] = true
	}
	for _, a := range p.Neighbours(home) {
		for _, b := range append([]int32{a}, p.Neighbours(int(a))...) {
			if ok, _ := p.ValidLanding(int(b)); ok && !taken[int(b)] && int(b) != home {
				return int(b)
			}
		}
	}
	j.t.Fatal("no bare landable tile near home")
	return 0
}

// enteredMap presses Enter on the caravan standing on a settlement and
// selects the map that opened.
func enteredMap(t *testing.T) (j *journey, mapID game.MapID, tile int) {
	t.Helper()
	j, p := worldColony(t)
	tile = bareTile(j, p)
	c := sendCaravan(j, j.g.Snapshot()[0].ID, tile)
	worldOpen(j)
	ck := "caravan:" + strconv.FormatUint(uint64(c.ID), 10)
	worldPick(j, "world.obj.caravan."+strconv.FormatUint(uint64(c.ID), 10), ck)
	if !j.clickID("world.act.enter") {
		why, _ := gizmoWhy(j, "world.act.enter")
		t.Fatalf("Enter is disabled: %q", why)
	}
	if cmd, ok := j.lastCmd(command.EnterMap); !ok || cmd.Target != uint64(c.ID) {
		t.Fatalf("EnterMap %+v (sent %v), want caravan %d", cmd, ok, c.ID)
	}
	j.g.Step(60)
	j.update(worldFly)
	for _, m := range j.g.Maps() {
		if m.ID != 0 {
			mapID = m.ID
		}
	}
	if mapID == 0 {
		t.Fatalf("Enter opened no map: %+v", j.g.Maps())
	}
	key := strconv.FormatUint(uint64(mapID), 10)
	worldPick(j, "world.obj.map."+key, "map:"+key)
	return j, mapID, tile
}

// TestJourneyWorldEnterAndRefusedGizmos: Enter on a caravan standing on a
// friendly settlement opens its map (EnterMap). Settle at the colony cap is
// refused with its reason and sends nothing.
func TestJourneyWorldEnterAndRefusedGizmos(t *testing.T) {
	t.Parallel()
	j, _, _, _ := atSettlement(t)
	for _, g := range []struct{ id, why string }{
		{"world.act.settle", "most colonies"},
	} {
		why, off := gizmoWhy(j, g.id)
		if !off || !strings.Contains(why, g.why) {
			t.Fatalf("%s: disabled %v, reason %q, want it refused for %q", g.id, off, why, g.why)
		}
		if j.clickID(g.id) {
			t.Fatalf("a click on the disabled %s went through", g.id)
		}
	}
	for _, k := range []command.Kind{command.SettleHere, command.AttackSettlement} {
		if _, sent := j.lastCmd(k); sent {
			t.Fatalf("a refused gizmo sent command kind %v", k)
		}
	}
	if len(j.g.Maps()) != 1 {
		t.Fatalf("%d maps before Enter, want 1", len(j.g.Maps()))
	}
}

// TestJourneyWorldReformCaravan: the map a caravan entered reforms it.
func TestJourneyWorldReformCaravan(t *testing.T) {
	t.Parallel()
	j, id, _ := enteredMap(t)
	if !j.clickID("world.act.reform") {
		why, _ := gizmoWhy(j, "world.act.reform")
		t.Fatalf("Reform is disabled: %q", why)
	}
	if cmd, ok := j.lastCmd(command.CaravanReform); !ok || cmd.Target != uint64(id) {
		t.Fatalf("CaravanReform %+v (sent %v), want map %d", cmd, ok, id)
	}
	j.g.Step(60)
	j.update(worldFly)
	if len(j.g.WorldMap().Caravans) != 1 {
		t.Fatalf("no caravan after Reform: %+v", j.g.WorldMap().Caravans)
	}
}

// TestJourneyWorldAbandonMapAsksFirst: Abandon on a map asks to confirm;
// Cancel sends nothing, Abandon sends AbandonMap.
func TestJourneyWorldAbandonMapAsksFirst(t *testing.T) {
	t.Parallel()
	j, id, _ := enteredMap(t)
	if !j.clickID("world.act.abandon") || j.a.uiModalTop() != "world.abandon" {
		t.Fatalf("Abandon opened modal %q, want the confirm", j.a.uiModalTop())
	}
	j.clickID("modal.world.abandon.cancel")
	if _, sent := j.lastCmd(command.AbandonMap); sent || j.a.uiModalTop() != "" {
		t.Fatalf("Cancel: command sent %v, modal %q", sent, j.a.uiModalTop())
	}
	// The press was the dialog's: the map stays selected, nothing under it.
	if !j.clickID("world.act.abandon") {
		t.Fatal("Cancel's click reached the globe and changed the selection")
	}
	j.clickID("modal.world.abandon.ok")
	if cmd, ok := j.lastCmd(command.AbandonMap); !ok || cmd.Target != uint64(id) {
		t.Fatalf("AbandonMap %+v (sent %v), want map %d", cmd, ok, id)
	}
	j.g.Step(60)
	j.update(1)
	for _, m := range j.g.Maps() {
		if m.ID == id {
			t.Fatalf("map %d is still open after Abandon", id)
		}
	}
}

// TestJourneyWorldTravelAndReturn: Travel asks for a tile on the globe,
// shows the route bar, and Confirm sends CaravanTravel; Return sends
// CaravanReturn.
func TestJourneyWorldTravelAndReturn(t *testing.T) {
	t.Parallel()
	j, _, s, c := atSettlement(t)
	if !j.clickID("world.act.travel") {
		t.Fatal("Travel is disabled")
	}
	if why, off := gizmoWhy(j, "world.act.confirm"); !off || why == "" {
		t.Fatalf("Confirm before a tile is picked: disabled %v, reason %q", off, why)
	}
	for _, o := range globeRing {
		clickGlobe(j, o[0], o[1])
		if w, ok := j.a.uiSt().Find(ConfirmWidget); ok && !w.Disabled {
			break
		}
	}
	if w, _ := j.a.uiSt().Find(ConfirmWidget); w.Disabled {
		t.Fatalf("no tile around %s gives a route (%q)", s.Name, w.Reason)
	}
	j.clickID(ConfirmWidget)
	cmd, ok := j.lastCmd(command.CaravanTravel)
	if !ok || cmd.Target != c.ID || cmd.Count == s.Tile {
		t.Fatalf("CaravanTravel %+v (sent %v), want caravan %d to another tile than %d", cmd, ok, c.ID, s.Tile)
	}
	if hasWidget(j.a, ConfirmWidget) {
		t.Fatal("the route bar is still up after Confirm")
	}
	j.update(worldFly)
	if !j.clickID("world.act.return") {
		why, _ := gizmoWhy(j, "world.act.return")
		t.Fatalf("Return is disabled: %q", why)
	}
	if rc, ok := j.lastCmd(command.CaravanReturn); !ok || rc.Target != c.ID {
		t.Fatalf("CaravanReturn %+v (sent %v), want caravan %d", rc, ok, c.ID)
	}
}

// ConfirmWidget is the route bar's Confirm.
const ConfirmWidget = "world.act.confirm"

// podsReady builds a launcher and a loaded transport pod on a wizard
// colony and presses Launch: the world view is up in targeting mode. goods
// loads steel instead of a colonist.
func podsReady(t *testing.T, goods, cap bool) (j *journey, pod entity.EntityID, home int) {
	t.Helper()
	j, _ = worldColony(t)
	if cap {
		raiseCap(j)
	}
	ax9Research(j)
	pawn := j.g.Snapshot()[0]
	var px, py int
	found := false
	for r := 3; r < 20 && !found; r++ {
		for _, d := range [][2]int{{r, 0}, {-r, 0}, {0, r}, {0, -r}} {
			x, y := pawn.X+d[0], pawn.Y+d[1]
			if j.g.BuildNow("pod launcher", "steel", "south", x-1, y) && j.g.BuildNow("transport pod", "steel", "south", x, y) {
				px, py, found = x, y, true
				break
			}
		}
	}
	if !found {
		t.Fatal("no room for a pod and its launcher")
	}
	j.g.FuelCampfire(px-1, py)
	pod = wdBuilding(j, px, py)
	if goods {
		wdSelect(j, px, py)
		if !j.clickID(inspector.LoadPodsBtnID) || j.a.uiModalTop() != dialogs.LoadPodsID {
			t.Fatalf("Load pods opened %q", j.a.uiModalTop())
		}
		total := map[string]int{}
		for _, it := range j.world().Items {
			if k, ok := item.ParseKind(it.Kind); ok && !k.IsCorpse() && !it.Forbidden && !it.Carried {
				total[it.Kind] += it.Qty
			}
		}
		picked := ""
		for _, k := range []string{"silver", "herbal medicine", "meal", "components"} {
			if total[k] > 0 && dialogs.GoodsKg(k, total[k]) <= 30 {
				picked = k
				break
			}
		}
		if picked == "" {
			t.Fatalf("no light goods on the ground: %v", total)
		}
		if !j.clickID("pods.goods."+strings.ReplaceAll(picked, " ", "_")+".all") || !j.clickID("modal."+dialogs.LoadPodsID+".pods.load") {
			t.Fatalf("no row for %s, or Load is disabled", picked)
		}
		want := dialogs.GoodsKg(picked, total[picked])
		j.key("Digit3", 0)
		j.until(func() bool { return ax9PodLoad(j, pod) > 0 }, 1500)
		j.until(func() bool { return ax9PodLoad(j, pod) >= want }, 600)
		j.key("Space", 0)
		j.tick(600) // the hauler walks off the pod's tile
	} else {
		ax9LoadOne(t, j, pod, px, py, uint64(pawn.ID))
	}
	wdSelect(j, px, py)
	if !j.clickID(inspector.LaunchPodsBtnID) || !j.a.world.Open() {
		t.Fatalf("Launch did not open the world view (top %q)", j.a.uiModalTop())
	}
	home = j.g.WorldMap().Tile
	j.until(func() bool { return hasWidget(j.a, "world.pod.target."+strconv.Itoa(home)) }, 300)
	return j, pod, home
}

// podSettlement is a settlement in the pods' range that the target list shows.
func podSettlement(j *journey, relation string) game.WorldSiteView {
	j.t.Helper()
	for _, s := range j.g.WorldSites() {
		if (relation == "" || s.Relation == relation) && hasWidget(j.a, "world.pod.target."+strconv.Itoa(s.Tile)) {
			return s
		}
	}
	j.t.Fatalf("no %q settlement in the target list", relation)
	return game.WorldSiteView{}
}

// TestJourneyWorldPodModes: the landing menu's rows each send LaunchPods in
// their mode at the target they were opened on.
func TestJourneyWorldPodModes(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name  string
		mode  uint8
		goods bool
		home  bool // the target is the colony's own tile, else a settlement
	}{
		{"drop", command.PodArriveDrop, false, true},
		{"dropcentre", command.PodArriveDropCentre, false, true},
		{"attackedge", command.PodArriveAttackEdge, false, false},
		{"attackcentre", command.PodArriveAttackCentre, false, false},
		{"gift", command.PodArriveGift, true, false},
		{"visit", command.PodArriveVisit, false, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			j, pod, tile := podsReady(t, c.goods, false)
			if !c.home {
				tile = podSettlement(j, "").Tile
			}
			j.clickID("world.pod.target." + strconv.Itoa(tile))
			if !j.clickID("world.pod." + c.name) {
				w, _ := j.a.uiSt().Find("world.pod." + c.name)
				t.Fatalf("%s is disabled: %q", c.name, w.Reason)
			}
			if j.a.uiModalTop() == worldview.AttackID { // MAP-15: a base that is not hostile asks first
				j.clickID("modal." + worldview.AttackID + ".ok")
			}
			cmd, ok := j.lastCmd(command.LaunchPods)
			if !ok || cmd.Mode != c.mode || cmd.Count != tile || cmd.Target != uint64(pod) {
				t.Fatalf("LaunchPods %+v (sent %v), want pod %d to %d mode %d", cmd, ok, pod, tile, c.mode)
			}
			j.update(1)
			if pods := j.world().AX.Pods; len(pods) != 1 {
				t.Fatalf("pods in flight %+v", pods)
			}
		})
	}
}

// TestJourneyWorldPodPickSpot: Pick spot switches to the target map, leaves
// a placement cursor, and a click on a cell sends the launch with it.
func TestJourneyWorldPodPickSpot(t *testing.T) {
	t.Parallel()
	j, pod, home := podsReady(t, false, false)
	j.clickID("world.pod.target." + strconv.Itoa(home))
	if !j.clickID("world.pod.spot") {
		w, _ := j.a.uiSt().Find("world.pod.spot")
		t.Fatalf("Pick spot is disabled: %q", w.Reason)
	}
	if j.a.world.Open() || !j.a.world.Spotting() {
		t.Fatalf("view open %v, cursor up %v: want the map with a placement cursor", j.a.world.Open(), j.a.world.Spotting())
	}
	if _, sent := j.lastCmd(command.LaunchPods); sent {
		t.Fatal("the launch left before a cell was picked")
	}
	if got := j.g.ViewedMap(); got != 0 {
		t.Fatalf("viewing map %d, want the colony", got)
	}
	x, y := j.g.Snapshot()[0].X, j.g.Snapshot()[0].Y
	j.lookAt(x, y)
	j.update(2)
	j.settle()
	j.clickTile(x, y, inputframe.Left, 0)
	cmd, ok := j.lastCmd(command.LaunchPods)
	if !ok || cmd.Mode != command.PodArriveSpot || cmd.Count != home || cmd.Target != uint64(pod) || cmd.X2 != x || cmd.Y2 != y {
		t.Fatalf("LaunchPods %+v (sent %v), want pod %d to %d, spot (%d, %d)", cmd, ok, pod, home, x, y)
	}
	if j.a.world.Spotting() {
		t.Fatal("the cursor is still up after the click")
	}
}

// raiseCap presses Options, Gameplay, Max colonies + from the pause menu and
// returns to the colony: the setting leaves the client as SetColonyCap.
func raiseCap(j *journey) {
	j.t.Helper()
	n := j.a.settings.Gameplay.Colonies()
	j.key("Escape", 0)
	j.clickID("pause.options")
	j.clickID("options.tab.gameplay")
	j.clickID("options.maxcolonies.inc")
	if c, ok := j.lastCmd(command.SetColonyCap); !ok || c.Count != n+1 {
		j.t.Fatalf("SetColonyCap %+v (sent %v), want a cap of %d", c, ok, n+1)
	}
	j.key("Escape", 0) // Options -> the pause menu
	j.clickID("pause.resume")
	if j.a.nav.screen() != ports.ScreenColony || j.a.nav.overlay() == overlayPause {
		j.t.Fatalf("back on screen %s overlay %v, want the colony", j.a.nav.screen(), j.a.nav.overlay())
	}
}

// globeCentre is the world view's globe centre: it flies to the selection.
func globeCentre(j *journey) (int, int) { return j.a.sw / 2, j.a.sh / 2 }

// clickGlobe clicks the globe at an offset from its centre (a bare tile: no
// marker is that far from the selection), through the real mouse.
func clickGlobe(j *journey, dx, dy int) {
	j.t.Helper()
	cx, cy := globeCentre(j)
	j.update(1)
	j.f.clickAt(j.t, j.a, cx+dx, cy+dy)
	j.f.Cur.Mx, j.f.Cur.My = -50, -50
	j.update(1)
}

// globeRing are click offsets around the globe's centre, nearest first: a
// grid of 40-pixel steps.
var globeRing = func() [][2]int {
	var out [][2]int
	for r := 1; r <= 7; r++ {
		for dy := -r; dy <= r; dy++ {
			for dx := -r; dx <= r; dx++ {
				if max(dx, -dx, dy, -dy) == r {
					out = append(out, [2]int{dx * 40, dy * 40})
				}
			}
		}
	}
	return out
}()

// TestJourneyWorldPodFound: with the cap raised, Found colony on a bare
// tile the globe click picked sends LaunchPods in that mode.
func TestJourneyWorldPodFound(t *testing.T) {
	t.Parallel()
	j, pod, home := podsReady(t, false, true)
	for _, o := range globeRing {
		clickGlobe(j, o[0], o[1])
		if !hasWidget(j.a, "world.pod.found") {
			continue
		}
		if w, _ := j.a.uiSt().Find("world.pod.found"); w.Disabled {
			j.clickID("world.pod.cancel")
			continue
		}
		j.clickID("world.pod.found")
		cmd, ok := j.lastCmd(command.LaunchPods)
		if !ok || cmd.Mode != command.PodArriveFound || cmd.Target != uint64(pod) || cmd.Count == home {
			t.Fatalf("LaunchPods %+v (sent %v), want pod %d founding a tile other than home %d", cmd, ok, pod, home)
		}
		return
	}
	t.Fatal("no bare tile around the colony accepts Found colony")
}

// TestJourneyWorldPodsAreMarkedAndPickable: a flight shows its pods on the
// globe; a click on them selects the group.
func TestJourneyWorldPodsAreMarkedAndPickable(t *testing.T) {
	t.Parallel()
	j, _, _ := podsReady(t, false, false)
	s := podSettlement(j, "")
	j.clickID("world.pod.target." + strconv.Itoa(s.Tile))
	j.clickID("world.pod.visit")
	j.g.Step(1800) // the pods leave the colony's tile
	j.update(worldFly)
	if !hasWidget(j.a, "world.obj.pod.0") {
		t.Fatalf("no pod marker; flights %+v", j.world().AX.Pods)
	}
	worldPick(j, "world.obj.pod.0", "pod:0")
	if !hasWidget(j.a, "world.pod.cancel") && j.a.world.Selected() != "pod:0" {
		t.Fatal("the pod group was not selected")
	}
}

// TestJourneyWorldPickSiteMapAndBareTile: a site map the game opened is
// picked from its marker (Reform is refused with nobody on it); a click away
// from every marker selects the bare tile and shows its card.
func TestJourneyWorldPickSiteMapAndBareTile(t *testing.T) {
	t.Parallel()
	j, p := worldColony(t)
	s, _ := nearestSite(j, p, false)
	id, ok := game.DevOpenMap(j.g, s.Tile)
	if !ok {
		t.Fatal("no site map")
	}
	worldOpen(j)
	worldFind(j, s)
	worldPick(j, "world.obj.map."+strconv.FormatUint(uint64(id), 10), "map:"+strconv.FormatUint(uint64(id), 10))
	if w, ok := j.a.uiSt().Find("world.act.reform"); !ok || !w.Disabled || w.Reason == "" {
		t.Fatalf("Reform on an empty site map: found %v, disabled %v, reason %q", ok, w.Disabled, w.Reason)
	}
	if j.clickID("world.act.reform") {
		t.Fatal("a disabled Reform took the click")
	}
	if _, sent := j.lastCmd(command.CaravanReform); sent {
		t.Fatal("a disabled gizmo sent CaravanReform")
	}
	for _, o := range globeRing {
		j.key("Escape", 0)
		clickGlobe(j, o[0], o[1])
		if strings.HasPrefix(j.a.world.Selected(), "tile:") {
			return
		}
	}
	t.Fatalf("no click picked a bare tile; selected %q", j.a.world.Selected())
}

// capRecSim records the commands a session sends its simulation.
type capRecSim struct {
	*game.Game
	got []command.Command
}

func (r *capRecSim) Enqueue(c command.Command) { r.got = append(r.got, c); r.Game.Enqueue(c) }

// continueWithCap sets the saved colony cap, presses Continue and returns
// the SetColonyCap commands the new session sent.
func continueWithCap(t *testing.T, cap int) []command.Command {
	t.Helper()
	rec := &capRecSim{}
	saves := &flowSaves{slots: []ports.SaveSlot{{Name: "kept", Path: "saves/kept.sav", Colonists: 1, Modified: time.Unix(500, 0)}}}
	hooks := saves.hooks()
	load := hooks.Load
	hooks.Load = func(path string) (Simulation, string, error) {
		s, name, err := load(path)
		if err == nil {
			rec.Game = s.(*game.Game)
			return rec, name, nil
		}
		return s, name, err
	}
	j := newMenuJourney(t, func(c *AppConfig) { c.Saves = hooks })
	j.a.settings.Gameplay.MaxColonies = cap
	if !j.clickID("menu.continue") {
		t.Fatal("Continue is disabled with a save listed")
	}
	var out []command.Command
	for _, c := range rec.got {
		if c.Kind == command.SetColonyCap {
			out = append(out, c)
		}
	}
	return out
}

// TestJourneyWorldColonyCapReachesTheSession: Continue sends the Options
// cap to the loaded colony when it is above one, and nothing at the default.
func TestJourneyWorldColonyCapReachesTheSession(t *testing.T) {
	t.Parallel()
	if got := continueWithCap(t, 3); len(got) != 1 || got[0].Count != 3 {
		t.Fatalf("SetColonyCap sent %+v at a cap of 3, want one with Count 3", got)
	}
	if got := continueWithCap(t, 1); len(got) != 0 {
		t.Fatalf("SetColonyCap sent %+v at the default cap of 1, want none", got)
	}
}

// TestJourneyWorldSettleHere: with the cap raised, Settle on a caravan
// standing on a bare landable tile founds a colony (SettleHere).
func TestJourneyWorldSettleHere(t *testing.T) {
	t.Parallel()
	j, p := worldColony(t)
	raiseCap(j)
	tile := bareTile(j, p)
	c := sendCaravan(j, j.g.Snapshot()[0].ID, tile)
	worldOpen(j)
	worldPick(j, "world.obj.caravan."+strconv.FormatUint(uint64(c.ID), 10), "caravan:"+strconv.FormatUint(uint64(c.ID), 10))
	if !j.clickID("world.act.settle") {
		why, _ := gizmoWhy(j, "world.act.settle")
		t.Fatalf("Settle is disabled with the cap raised: %q", why)
	}
	if cmd, ok := j.lastCmd(command.SettleHere); !ok || cmd.Target != uint64(c.ID) {
		t.Fatalf("SettleHere %+v (sent %v), want caravan %d", cmd, ok, c.ID)
	}
	j.g.Step(60)
	if len(j.g.Maps()) < 2 {
		t.Fatalf("Settle founded no colony: %+v", j.g.Maps())
	}
}

// TestJourneyWorldAttackHostileBase: a caravan waiting at a hostile
// settlement attacks it (AttackSettlement); Enter is refused there.
func TestJourneyWorldAttackHostileBase(t *testing.T) {
	t.Parallel()
	j, p := worldColony(t)
	s, _ := nearestSite(j, p, true)
	c := formCaravan(j, j.g.Snapshot()[0].ID, s.Tile)
	game.DevArrive(j.g, entity.EntityID(c.ID), s.Tile)
	worldOpen(j)
	worldFind(j, s)
	worldPick(j, "world.obj.caravan."+strconv.FormatUint(uint64(c.ID), 10), "caravan:"+strconv.FormatUint(uint64(c.ID), 10))
	if why, off := gizmoWhy(j, "world.act.enter"); !off || !strings.Contains(why, "hostile") {
		t.Fatalf("Enter at a hostile base: disabled %v, reason %q", off, why)
	}
	if !j.clickID("world.act.attack") {
		why, _ := gizmoWhy(j, "world.act.attack")
		t.Fatalf("Attack is disabled at %s: %q", s.Name, why)
	}
	if cmd, ok := j.lastCmd(command.AttackSettlement); !ok || cmd.Target != uint64(c.ID) {
		t.Fatalf("AttackSettlement %+v (sent %v), want caravan %d", cmd, ok, c.ID)
	}
}

// TestJourneyWorldDemandLetterAnswers: a demand on the road asks by letter;
// Give and Refuse in the letter's window answer it.
func TestJourneyWorldDemandLetterAnswers(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		button string
		mode   uint8
	}{{"give", 0}, {"refuse", 1}} {
		j, p := worldColony(t)
		s, _ := nearestSite(j, p, true)
		c := formCaravan(j, j.g.Snapshot()[0].ID, s.Tile)
		game.DevDemand(j.g, entity.EntityID(c.ID), item.Stack{Kind: item.Silver, Qty: 100})
		j.g.Step(1)
		j.update(1)
		var letter game.LetterView
		for _, l := range j.g.World().Letters {
			if l.Demand {
				letter = l
			}
		}
		if letter.ID == 0 {
			t.Fatalf("no demand letter: %+v", j.g.World().Letters)
		}
		if !j.clickID("letter." + strconv.FormatUint(letter.ID, 10)) {
			t.Fatal("the letter's row is disabled")
		}
		if !j.clickID("modal.letter." + strconv.FormatUint(letter.ID, 10) + "." + tc.button) {
			t.Fatalf("no %s button on the letter", tc.button)
		}
		cmd, ok := j.lastCmd(command.EncounterAnswer)
		if !ok || cmd.Target != letter.ID || cmd.Mode != tc.mode {
			t.Fatalf("%s: EncounterAnswer %+v (sent %v), want letter %d mode %d", tc.button, cmd, ok, letter.ID, tc.mode)
		}
		j.g.Step(1)
		for _, l := range j.g.World().Letters {
			if l.ID == letter.ID {
				t.Fatalf("%s: the letter stays after its answer", tc.button)
			}
		}
	}
}
