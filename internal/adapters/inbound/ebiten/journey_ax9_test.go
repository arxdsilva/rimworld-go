package ebiten

// journey_ax9_test.go (AX-9): stasis caskets, transport pods, the mineral
// scanner and the hitching spot through the real UI loop. Every AX-9 tool
// is laid from the Architect's Misc category; a colonist climbs into a
// casket from the right-click menu and the pane's Eject lets her out; the
// scanner's pane picks a target mineral; the pod pane's Load pods opens
// the load dialog, a colonist walks in and Unload sets her down; and on a
// wizard colony (it has a world) the loaded pods' Launch opens the World
// tab's target picker, a target in range is picked and the pods fly there.

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/dialogs"
	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/adapters/inbound/ebiten/inspector"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/research"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// ax9Research finishes the AX-9 projects (and what they need).
func ax9Research(j *journey) {
	for _, p := range []research.Project{research.Stasis, research.TransportPods, research.MineralScanning} {
		rsFinish(j, int(p))
	}
}

// ax9Sleeper is the sleeper of the stasis casket at c ("" none).
func ax9Sleeper(j *journey, c [2]int) string {
	for _, b := range j.world().Buildings {
		if b.X == c[0] && b.Y == c[1] && b.Kind == "stasis casket" {
			return b.AX.Sleeper
		}
	}
	return ""
}

// ax9PodLoad is the load (kg) of the transport pod id.
func ax9PodLoad(j *journey, id entity.EntityID) int {
	for _, b := range j.world().Buildings {
		if b.ID == id {
			return b.AX.LoadKg
		}
	}
	return -1
}

// ax9Option opens the options menu of the selection's command id (its
// corner arrow) and picks the row starting label.
func ax9Option(t *testing.T, j *journey, id, label string) {
	t.Helper()
	cmds := j.a.insp.Commands().SelectionCommands()
	i := slices.IndexFunc(cmds, func(b inspector.Button) bool { return b.ID == id })
	if i < 0 {
		t.Fatalf("no %s command", id)
	}
	j.update(1)
	if !j.clickID("cmd.options." + strconv.Itoa(i)) {
		t.Fatalf("no options arrow on %s", id)
	}
	j.pick(label)
}

// ax9LoadOne selects pod, opens the load dialog from its pane, picks
// colonist pawn and loads, then runs until the colonist is aboard.
func ax9LoadOne(t *testing.T, j *journey, pod entity.EntityID, x, y int, pawn uint64) {
	t.Helper()
	wdSelect(j, x, y)
	if !j.clickID(inspector.LoadPodsBtnID) || j.a.uiModalTop() != dialogs.LoadPodsID {
		t.Fatalf("Load pods opened %q", j.a.uiModalTop())
	}
	if !j.clickID("pods.pawn." + strconv.FormatUint(pawn, 10)) {
		t.Fatal("no colonist row in the load dialog")
	}
	if !j.clickID("modal." + dialogs.LoadPodsID + ".pods.load") {
		t.Fatal("Load is disabled with a colonist picked")
	}
	if c, ok := j.lastCmd(command.LoadPods); !ok || !c.Enabled || c.Target != uint64(pod) || c.Name != strconv.FormatUint(pawn, 10) {
		t.Fatalf("LoadPods %+v ok=%v", c, ok)
	}
	// the colonist steps into the pod: stop on that tick, before anyone
	// else walks onto the pod's tile and takes the click below
	if !j.ffWatch(6000*speedVeryFast.ticksPerFrame(), ffStride, func() bool { return ax9PodLoad(j, pod) >= 60 }, nil) {
		t.Fatalf("the pod's load is %d kg after 6000 Very fast Updates' ticks, want the colonist's 60", ax9PodLoad(j, pod))
	}
}

func TestJourneyAX9CasketScannerPods(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jSize(48, 36), jPawns([2]int{10, 10}, [2]int{12, 10}))
	ax9Research(j)
	for _, tool := range []struct {
		id, kind string
		x, y     int
	}{
		{"stasis_casket", "stasis casket", 30, 4}, {"transport_pod", "transport pod", 34, 4},
		{"pod_launcher", "pod launcher", 36, 4}, {"mineral_scanner", "long-range mineral scanner", 40, 8},
	} {
		ax4Lay(t, j, "misc", tool.id, tool.kind, 0, tool.x, tool.y)
	}
	// the hitching spot costs nothing and takes no work: it stands at once
	j.lookAt(30, 12)
	j.update(1)
	j.openArch("misc", "hitching_spot")
	j.clickTile(30, 12, inputframe.Left, 0)
	j.update(1)
	j.leaveArch()
	if b, ok := ax9At(j, 30, 12); !ok || b.Kind != "caravan hitching spot" {
		t.Fatalf("no hitching spot at (30, 12): %+v", b)
	}

	// a colonist climbs into a casket from the right-click menu; Eject
	pawn := uint64(j.g.Snapshot()[0].ID)
	casket := [2]int{18, 6}
	wdBuild(j, "stasis casket", casket[0], casket[1])
	careSelect(j, pawn)
	if rows := careRightClick(j, casket[0], casket[1]); !slices.Contains(rows, "Enter stasis casket") {
		t.Fatalf("casket menu %q lacks Enter", rows)
	}
	j.pick("Enter stasis casket")
	if c, ok := j.lastCmd(command.EnterCasket); !ok || c.Pawn != pawn {
		t.Fatalf("EnterCasket %+v ok=%v", c, ok)
	}
	j.key("Digit3", 0)
	j.until(func() bool { return ax9Sleeper(j, casket) != "" }, 4000)
	j.key("Space", 0)
	cid := wdSelect(j, casket[0], casket[1])
	if lines := wbPaneLines(j, cid); !strings.Contains(lines, "sleeper:") || !strings.Contains(lines, "stand still") {
		t.Fatalf("casket pane %q", lines)
	}
	if !j.clickID(inspector.EjectCasketBtnID) {
		t.Fatal("no Eject button")
	}
	if c, ok := j.lastCmd(command.EjectCasket); !ok || c.Target != uint64(cid) {
		t.Fatalf("EjectCasket %+v ok=%v", c, ok)
	}
	j.update(2)
	if ax9Sleeper(j, casket) != "" {
		t.Fatal("the sleeper is still in the casket")
	}

	// a downed colonist carried in from the right-click menu, and woken
	bob := uint64(j.g.Snapshot()[1].ID)
	careDown(j, bob)
	careSelect(j, pawn)
	bp := j.pawn(bob)
	rows := careRightClick(j, bp.X, bp.Y)
	if !slices.ContainsFunc(rows, func(r string) bool { return strings.HasSuffix(r, "to stasis casket") }) {
		t.Fatalf("downed colonist menu %q lacks the carry row", rows)
	}
	j.pick("Carry ")
	if c, ok := j.lastCmd(command.EnterCasket); !ok || c.Worker != bob || c.Target != uint64(cid) {
		t.Fatalf("carry EnterCasket %+v ok=%v", c, ok)
	}
	j.key("Digit3", 0)
	j.until(func() bool { return ax9Sleeper(j, casket) != "" }, 4000)
	j.key("Space", 0)
	wdSelect(j, casket[0], casket[1])
	j.clickID(inspector.EjectCasketBtnID)
	j.update(2)

	// the scanner's pane and its target menu
	wdBuild(j, "long-range mineral scanner", 24, 16)
	sid := wdSelect(j, 24, 16)
	if lines := wbPaneLines(j, sid); !strings.Contains(lines, "scanning") || !strings.Contains(lines, "looking for: any mineral") {
		t.Fatalf("scanner pane %q", lines)
	}
	ax9Option(t, j, inspector.ScanTargetBtnID, "Gold")
	if c, ok := j.lastCmd(command.SetScannerTarget); !ok || c.Stuff != uint8(item.Gold) {
		t.Fatalf("SetScannerTarget %+v ok=%v", c, ok)
	}
	j.update(1)
	if lines := wbPaneLines(j, sid); !strings.Contains(lines, "looking for: gold") {
		t.Fatalf("scanner pane after the target %q", lines)
	}

	// the hitching spot's note
	wdBuild(j, "caravan hitching spot", 8, 20)
	if lines := wbPaneLines(j, wdSelect(j, 8, 20)); !strings.Contains(lines, "gather here") {
		t.Fatalf("hitching spot pane %q", lines)
	}

	// pods: load a colonist through the dialog, then unload
	wdBuild(j, "pod launcher", 13, 24)
	wdBuild(j, "transport pod", 14, 24)
	j.g.FuelCampfire(13, 24)
	pod := wdBuilding(j, 14, 24)
	if lines := wbPaneLines(j, wdSelect(j, 14, 24)); !strings.Contains(lines, "load  0 / 150 kg") || !strings.Contains(lines, "range  66 tiles") {
		t.Fatalf("pod pane %q", lines)
	}
	ax9LoadOne(t, j, pod, 14, 24, pawn)
	wdSelect(j, 14, 24)
	if !j.clickID(inspector.UnloadPodsBtnID) {
		t.Fatal("no Unload button")
	}
	if c, ok := j.lastCmd(command.LoadPods); !ok || c.Enabled {
		t.Fatalf("Unload sent %+v", c)
	}
	j.update(2)
	if ax9PodLoad(j, pod) != 0 || !slices.ContainsFunc(j.g.Snapshot(), func(p game.PawnView) bool { return uint64(p.ID) == pawn }) {
		t.Fatal("unload left the colonist aboard")
	}
}

// TestJourneyAX9PodsLaunch: on a wizard colony, loaded pods' Launch opens
// the World tab's picker; a target in range is picked from its list and
// the pods fly there (shown in the world view), landing as a drop or a
// visit.
func TestJourneyAX9PodsLaunch(t *testing.T) {
	t.Parallel()
	j := newMenuJourney(t)
	wizardToColony(t, j)
	if j.a.renderer.speed != speedPaused {
		j.key("Space", 0)
	}
	ax9Research(j)
	pawn := j.g.Snapshot()[0]
	var px, py int
	found := false
	for r := 3; r < 20 && !found; r++ {
		for _, d := range [][2]int{{r, 0}, {-r, 0}, {0, r}, {0, -r}} {
			x, y := pawn.X+d[0], pawn.Y+d[1]
			if j.g.BuildNow("pod launcher", "steel", "south", x-1, y) {
				if j.g.BuildNow("transport pod", "steel", "south", x, y) {
					px, py, found = x, y, true
					break
				}
			}
		}
	}
	if !found {
		t.Fatal("no room for a pod and its launcher")
	}
	j.g.FuelCampfire(px-1, py)
	pod := wdBuilding(j, px, py)
	ax9LoadOne(t, j, pod, px, py, uint64(pawn.ID))
	wdSelect(j, px, py)
	if !j.clickID(inspector.LaunchPodsBtnID) || !j.a.world.Open() {
		t.Fatalf("Launch did not open the world view (top %q)", j.a.uiModalTop())
	}
	const podDrop, podVisit = "world.pod.drop", "world.pod.visit"
	home := j.g.WorldMap().Tile
	homeRow := "world.pod.target." + strconv.Itoa(home)
	j.until(func() bool { _, ok := j.a.uiSt().Find(homeRow); return ok }, 200) // the planet is charted, the list up
	target, mode := home, podDrop
	for _, s := range j.g.WorldSites() {
		if s.Visitable && hasWidget(j.a, "world.pod.target."+strconv.Itoa(s.Tile)) {
			j.clickID("world.pod.target." + strconv.Itoa(s.Tile))
			target, mode = s.Tile, podVisit
			break
		}
	}
	if target == home {
		j.clickID(homeRow)
	}
	if !j.clickID(mode) {
		t.Fatalf("the %s button is disabled on target %d", mode, target)
	}
	c, ok := j.lastCmd(command.LaunchPods)
	if !ok || c.Target != uint64(pod) || c.Count != target {
		t.Fatalf("LaunchPods %+v ok=%v, want pod %d to %d", c, ok, pod, target)
	}
	j.update(1)
	if pods := j.world().AX.Pods; len(pods) != 1 || int(pods[0].To) != target {
		t.Fatalf("pods in flight %+v", pods)
	}
	j.ffUntil(func() bool { return len(j.world().AX.Pods) == 0 }, 8000)
	if mode == podVisit && len(j.g.WorldMap().Caravans) != 1 {
		t.Fatalf("the visitors are no caravan: %+v", j.g.WorldMap().Caravans)
	}
	if mode == podDrop && !slices.ContainsFunc(j.g.Snapshot(), func(p game.PawnView) bool { return p.ID == pawn.ID }) {
		t.Fatal("the dropped colonist is not back on the map")
	}
}

// ax9At is the building view covering (x, y).
func ax9At(j *journey, x, y int) (game.BuildingView, bool) {
	for _, b := range j.world().Buildings {
		if x >= b.X && x < b.X+max(1, b.W) && y >= b.Y && y < b.Y+max(1, b.H) {
			return b, true
		}
	}
	return game.BuildingView{}, false
}

// TestJourneyAX9CaravanPacks: on a wizard colony the Form caravan dialog
// lists goods with their amounts beside the colonists; a colonist and
// all of one kind of goods are sent: the caravan gathers to load them.
func TestJourneyAX9CaravanPacks(t *testing.T) {
	t.Parallel()
	j := newMenuJourney(t)
	wizardToColony(t, j)
	if j.a.renderer.speed != speedPaused {
		j.key("Space", 0)
	}
	j.key("F8", 0)
	j.update(2) // the colony is selected on opening: its gizmos are up
	if !j.clickID("world.act.form") {
		t.Fatal("Form caravan is disabled")
	}
	j.update(1)
	member := j.g.Snapshot()[0].ID
	j.clickID("form.member." + strconv.FormatUint(uint64(member), 10))
	total := map[string]int{}
	for _, it := range j.world().Items {
		if k, ok := item.ParseKind(it.Kind); ok && !k.IsCorpse() && !it.Forbidden && !it.Carried {
			total[it.Kind] += it.Qty
		}
	}
	picked := ""
	for k, n := range total {
		if dialogs.GoodsKg(k, n) <= 30 && (picked == "" || k < picked) {
			picked = k
		}
	}
	if picked != "" && !j.clickID("form.goods."+strings.ReplaceAll(picked, " ", "_")+".all") {
		t.Fatalf("no goods row for %s", picked)
	}
	if picked == "" {
		t.Skip("no light loose goods on this colony")
	}
	dest := -1
	for _, s := range j.g.WorldSites() {
		if s.Visitable {
			dest = s.Tile
			break
		}
	}
	j.clickID("form.dest." + strconv.Itoa(dest))
	if !j.clickID("modal." + dialogs.FormCaravanID + ".form.send") {
		t.Fatal("Send is disabled with a colonist, light goods and a destination")
	}
	c, ok := j.lastCmd(command.FormCaravan)
	if !ok || !strings.Contains(c.Name, ":") || !strings.HasPrefix(c.Name, strconv.FormatUint(uint64(member), 10)+",") {
		t.Fatalf("FormCaravan %+v ok=%v, want the colonist then the goods", c, ok)
	}
	if cs := j.g.WorldMap().Caravans; len(cs) != 1 || cs[0].State != "Forming" {
		t.Fatalf("caravans %+v", cs)
	}
}
