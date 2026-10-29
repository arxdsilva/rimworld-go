package ebiten

import (
	"rimworld-go/internal/adapters/inbound/ebiten/architect"
	"rimworld-go/internal/adapters/inbound/ebiten/dialogs"
	windowspkg "rimworld-go/internal/adapters/inbound/ebiten/windows"
	"strings"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/planet"
)

// globeTabSim is the stub with the World tab's read models (no globe: its
// params are not set, so the tab skips charting).
type globeTabSim struct {
	*stubSim
	sites []game.WorldSiteView
	area  map[entity.EntityID]int
}

func (s *globeTabSim) WorldParams() (planet.Params, bool) { return planet.Params{}, false }
func (s *globeTabSim) WorldSites() []game.WorldSiteView   { return s.sites }
func (s *globeTabSim) AreaOf(id entity.EntityID) int {
	if a, ok := s.area[id]; ok {
		return a
	}
	return -1
}

func worldStub() *globeTabSim {
	s := &stubSim{pawns: []game.PawnView{
		{ID: 1, Name: "Ada", Status: "alive"},
		{ID: 2, Name: "Bo", Status: "downed"},
	}}
	s.worldMap = game.WorldMapView{Tile: 7, Caravans: []game.CaravanView{
		{ID: 40, Members: []string{"Cora"}, Tile: 8, Dest: 9, Progress: 25, State: "Travelling"},
	}}
	return &globeTabSim{stubSim: s, sites: []game.WorldSiteView{
		{Tile: 9, Name: "Ashford", Faction: "Settlers", Relation: "neutral", Visitable: true},
		{Tile: 11, Name: "Redmoor", Faction: "Raiders", Relation: "hostile"},
	}}
}

func TestWorldTabListsCaravansAndReturns(t *testing.T) {
	t.Parallel()
	s := worldStub()
	a, f := colonyApp(t, s.stubSim)
	a.renderer.game = s
	for _, b := range a.chrome.Tabs().Buttons(a.sw, a.sh) {
		if b.ID == "tab.world" && b.Disabled {
			t.Fatal("the World tab is disabled")
		}
	}
	a.win.World().Toggle()
	f.Frame(t, a)
	if a.uiModalTop() != windowspkg.WorldModalID {
		t.Fatalf("top modal %q", a.uiModalTop())
	}
	if !hasWidget(a, "world.caravan.40") {
		t.Fatal("no row for the caravan")
	}
	press(t, a, f, "world.caravan.40.return")
	if n := len(s.cmds); n != 1 || s.cmds[0].Kind != command.CaravanReturn || s.cmds[0].Target != 40 {
		t.Fatalf("commands %+v, want CaravanReturn 40", s.cmds)
	}
	if w, _ := a.uiSt().Find("modal." + windowspkg.WorldModalID + ".world.form"); !w.Disabled || !strings.Contains(w.Reason, "no world map") {
		t.Fatalf("Form caravan without a world map: %+v", w)
	}
}

func TestFormCaravanDialog(t *testing.T) {
	t.Parallel()
	s := worldStub()
	a, f := colonyApp(t, s.stubSim)
	a.renderer.game = s
	a.deal.OpenFormCaravan(func() int { return -1 })
	f.Frame(t, a)
	send := "modal." + dialogs.FormCaravanID + ".form.send"
	if w, _ := a.uiSt().Find(send); !w.Disabled || w.Reason != dialogs.FormNoMembers {
		t.Fatalf("Send with nothing picked: %+v", w)
	}
	if w, _ := a.uiSt().Find("form.member.2"); !w.Disabled {
		t.Fatal("a downed colonist can be picked")
	}
	if w, _ := a.uiSt().Find("form.dest.11"); !w.Disabled {
		t.Fatal("a hostile settlement can be picked")
	}
	press(t, a, f, "form.member.1")
	f.Frame(t, a)
	if w, _ := a.uiSt().Find(send); !w.Disabled || w.Reason != dialogs.FormNoDest {
		t.Fatalf("Send without a destination: %+v", w)
	}
	press(t, a, f, "form.dest.9")
	press(t, a, f, send)
	if n := len(s.cmds); n != 1 || s.cmds[0].Kind != command.FormCaravan || s.cmds[0].Name != "1" || s.cmds[0].Count != 9 {
		t.Fatalf("commands %+v, want FormCaravan of 1 to 9", s.cmds)
	}
}

// The Assign tab no longer has an Area column (areas live in Schedule and
// Animals); its "Manage areas" button still opens the manager.
func TestAssignTabHasNoAreaColumnAndOpensManager(t *testing.T) {
	t.Parallel()
	s := worldStub()
	s.areas = []game.AreaView{{Index: 0, ID: 1, Name: "Yard", Cells: 40}, {Index: 1, ID: 2, Name: "Fields", Cells: 9}}
	s.area = map[entity.EntityID]int{1: 1}
	a, f := colonyApp(t, s.stubSim)
	a.renderer.game = s
	a.win.Assign().Launch()
	f.Frame(t, a)
	if hasWidget(a, "assign.area.1") {
		t.Fatal("the Assign tab still has an Area button")
	}
	if !hasWidget(a, windowspkg.AssignDrugID(1)) {
		t.Fatal("the drug policy column is gone")
	}
	press(t, a, f, "assign.areas")
	f.Frame(t, a)
	if a.uiModalTop() != architect.AreasModalID || !hasWidget(a, "areas.name.2") {
		t.Fatalf("area manager not open: %q", a.uiModalTop())
	}
}

func TestAreaHelpers(t *testing.T) {
	t.Parallel()
	if dialogs.MemberList([]entity.EntityID{3, 12}) != "3,12" {
		t.Fatal("member list")
	}
}

// chartedSim reports a small real world, so the tab charts a globe.
type chartedSim struct{ *globeTabSim }

func (s *chartedSim) WorldParams() (planet.Params, bool) {
	return planet.Params{SeedText: "world-tab", Coverage: 5, Rain: 2, Temp: 2, Pop: 1}, true
}

func TestWorldTabChartsTheGlobe(t *testing.T) {
	t.Parallel()
	s := &chartedSim{worldStub()}
	a, f := colonyApp(t, s.stubSim)
	a.renderer.game = s
	// the charting and its cache: windows TestWorldTabChartsThePlanetOnce
	a.win.World().Toggle()
	for i := 0; i < 3; i++ {
		f.Frame(t, a)
	}
	if a.uiModalTop() != windowspkg.WorldModalID || hasWidget(a, "modal."+windowspkg.WorldModalID+".world.form") == false {
		t.Fatal("World tab not open")
	}
	_ = drawFrame(t, a, f) // the globe and its caravan marker draw in the modal
	if st := a.uiModalTop(); st != windowspkg.WorldModalID {
		t.Fatalf("top modal after a draw %q", st)
	}
	if w, _ := a.uiSt().Find("modal." + windowspkg.WorldModalID + ".world.form"); w.Disabled {
		t.Fatalf("Form caravan disabled with a world map: %q", w.Reason)
	}
}
