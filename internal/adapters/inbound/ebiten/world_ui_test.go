package ebiten

import (
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
	for _, b := range a.hudTabButtons(a.sw, a.sh) {
		if b.id == "tab.world" && b.disabled {
			t.Fatal("the World tab is disabled")
		}
	}
	a.toggleWorld()
	f.frame(t, a)
	if a.uiModalTop() != worldModalID {
		t.Fatalf("top modal %q", a.uiModalTop())
	}
	if !hasWidget(a, "world.caravan.40") {
		t.Fatal("no row for the caravan")
	}
	press(t, a, f, "world.caravan.40.return")
	if n := len(s.cmds); n != 1 || s.cmds[0].Kind != command.CaravanReturn || s.cmds[0].Target != 40 {
		t.Fatalf("commands %+v, want CaravanReturn 40", s.cmds)
	}
	if w, _ := a.uiSt().find("modal." + worldModalID + ".world.form"); !w.disabled || !strings.Contains(w.reason, "no world map") {
		t.Fatalf("Form caravan without a world map: %+v", w)
	}
}

func TestFormCaravanDialog(t *testing.T) {
	t.Parallel()
	s := worldStub()
	a, f := colonyApp(t, s.stubSim)
	a.renderer.game = s
	a.openFormCaravan(&worldUI{picked: -1})
	f.frame(t, a)
	send := "modal." + formCaravanID + ".form.send"
	if w, _ := a.uiSt().find(send); !w.disabled || w.reason != formNoMembers {
		t.Fatalf("Send with nothing picked: %+v", w)
	}
	if w, _ := a.uiSt().find("form.member.2"); !w.disabled {
		t.Fatal("a downed colonist can be picked")
	}
	if w, _ := a.uiSt().find("form.dest.11"); !w.disabled {
		t.Fatal("a hostile settlement can be picked")
	}
	press(t, a, f, "form.member.1")
	f.frame(t, a)
	if w, _ := a.uiSt().find(send); !w.disabled || w.reason != formNoDest {
		t.Fatalf("Send without a destination: %+v", w)
	}
	press(t, a, f, "form.dest.9")
	press(t, a, f, send)
	if n := len(s.cmds); n != 1 || s.cmds[0].Kind != command.FormCaravan || s.cmds[0].Name != "1" || s.cmds[0].Count != 9 {
		t.Fatalf("commands %+v, want FormCaravan of 1 to 9", s.cmds)
	}
}

func TestAssignAreaColumnAndManager(t *testing.T) {
	t.Parallel()
	s := worldStub()
	s.areas = []game.AreaView{{Index: 0, Name: "Yard", Cells: 40}, {Index: 1, Name: "Fields", Cells: 9}}
	s.area = map[entity.EntityID]int{1: 0}
	a, f := colonyApp(t, s.stubSim)
	a.renderer.game = s
	a.openAssign()
	f.frame(t, a)
	if w, _ := a.uiSt().find("assign.area.1"); w.label != "Yard" {
		t.Fatalf("Ada's area button %q", w.label)
	}
	press(t, a, f, "assign.area.1")
	if n := len(s.cmds); n != 1 || s.cmds[0].Kind != command.AssignArea || s.cmds[0].Pawn != 1 || s.cmds[0].Index != 1 {
		t.Fatalf("commands %+v, want Ada to area 1", s.cmds)
	}
	press(t, a, f, "assign.areas")
	f.frame(t, a)
	if a.uiModalTop() != areasModalID || !hasWidget(a, "areas.name.1") {
		t.Fatalf("area manager not open: %q", a.uiModalTop())
	}
	press(t, a, f, "areas.paint.1")
	if a.areaPaintIndex() != 1 {
		t.Fatal("Paint did not pick the area")
	}
	press(t, a, f, "modal."+areasModalID+".areas.new")
	last := s.cmds[len(s.cmds)-1]
	if last.Kind != command.RenameArea || last.Index != 2 || last.Name != "Area 3" {
		t.Fatalf("New area emitted %+v", last)
	}
}

func TestAreaHelpers(t *testing.T) {
	t.Parallel()
	areas := []game.AreaView{{Index: 0, Name: "A"}, {Index: 1, Name: "B"}}
	if nextArea(areas, -1) != 0 || nextArea(areas, 0) != 1 || nextArea(areas, 1) != -1 || nextArea(nil, -1) != -1 {
		t.Fatal("area cycle")
	}
	if areaLabel(areas, 1) != "B" || areaLabel(areas, -1) != "Unrestricted" {
		t.Fatal("area labels")
	}
	if memberList([]entity.EntityID{3, 12}) != "3,12" {
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
	st := &worldUI{picked: -1}
	st.params, st.hasMap = s.WorldParams()
	var p *planet.Planet
	for i := 0; i < 400 && p == nil; i++ {
		p = a.worldPlanetFor(st)
	}
	if p == nil {
		t.Fatal("the world was never charted")
	}
	if q := a.worldPlanetFor(&worldUI{params: st.params, hasMap: true}); q != p {
		t.Fatal("a reopened tab charts the world again")
	}
	a.toggleWorld()
	for i := 0; i < 3; i++ {
		f.frame(t, a)
	}
	if a.uiModalTop() != worldModalID || hasWidget(a, "modal."+worldModalID+".world.form") == false {
		t.Fatal("World tab not open")
	}
	_ = drawFrame(t, a, f) // the globe and its caravan marker draw in the modal
	if st := a.uiModalTop(); st != worldModalID {
		t.Fatalf("top modal after a draw %q", st)
	}
	if w, _ := a.uiSt().find("modal." + worldModalID + ".world.form"); w.disabled {
		t.Fatalf("Form caravan disabled with a world map: %q", w.reason)
	}
}
