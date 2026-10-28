package ebiten

import (
	"strings"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// bedSim is the stub with the prisoner-bed and gathering read models.
type bedSim struct {
	*stubSim
	prison     map[entity.EntityID]bool
	gatherings []game.GatheringView
}

func (s *bedSim) BedForPrisoners(id entity.EntityID) bool { return s.prison[id] }
func (s *bedSim) Gatherings() []game.GatheringView        { return s.gatherings }

func bedStub() *bedSim {
	s := &stubSim{pawns: []game.PawnView{
		{ID: 1, Name: "Ada", Status: "alive"},
		{ID: 2, Name: "Bo", Status: "alive"},
		{ID: 3, Name: "Cy", Status: "dead"},
	}}
	s.world.Buildings = []game.BuildingView{{ID: 40, Kind: "double bed", X: 5, Y: 5, W: 2, H: 2, Claimant: 1}}
	return &bedSim{stubSim: s, prison: map[entity.EntityID]bool{}}
}

// The bed inspector shows an owner row per slot, a picker that sends
// AssignBedOwner, and the prisoner toggle that sends SetBedForPrisoners.
func TestBedInspectorOwnersAndPrisonerToggle(t *testing.T) {
	t.Parallel()
	s := bedStub()
	a, f := colonyApp(t, s.stubSim)
	a.renderer.game = s
	a.selectCand(selCand{selBuilding, 40})
	b, ok := a.selectedBuilding()
	if !ok {
		t.Fatal("bed not selected")
	}
	ids := map[string]cmdButton{}
	for _, bt := range a.buildingCommands(40) {
		ids[bt.ID] = bt
	}
	for _, id := range []string{bedOwnerID(0), bedOwnerID(1), "bldg.bed.prisoners"} {
		if _, ok := ids[id]; !ok {
			t.Fatalf("no %s command: %v", id, ids)
		}
	}
	if ids["bldg.bed.prisoners"].Disabled || ids["bldg.bed.prisoners"].On {
		t.Fatalf("toggle %+v", ids["bldg.bed.prisoners"])
	}
	if bedOwner(b, 0) != 1 || bedOwner(b, 1) != 0 || a.pawnName(1) != "Ada" {
		t.Fatal("owner rows read wrong")
	}

	a.applyBuildingClick(bedOwnerID(1))
	f.frame(t, a)
	if a.uiModalTop() != bedOwnerModalID {
		t.Fatalf("top modal %q, want the owner picker", a.uiModalTop())
	}
	if !hasWidget(a, bedOwnerPickID(0)) || !hasWidget(a, bedOwnerPickID(2)) || hasWidget(a, bedOwnerPickID(3)) {
		t.Fatal("picker lists nobody and the living colonists only")
	}
	press(t, a, f, bedOwnerPickID(2))
	got := cmdsOf(s.stubSim, command.AssignBedOwner)
	if len(got) != 1 || got[0].Target != 40 || got[0].Pawn != 2 || got[0].Index != 1 {
		t.Fatalf("AssignBedOwner %+v", got)
	}
	if a.uiModalTop() == bedOwnerModalID {
		t.Fatal("picker stays open after a pick")
	}

	a.applyBuildingClick("bldg.bed.prisoners")
	pr := cmdsOf(s.stubSim, command.SetBedForPrisoners)
	if len(pr) != 1 || pr[0].Target != 40 || !pr[0].Enabled {
		t.Fatalf("SetBedForPrisoners %+v", pr)
	}
	s.prison[40] = true
	for _, bt := range a.buildingCommands(40) {
		if bt.ID == "bldg.bed.prisoners" && !bt.On {
			t.Fatal("toggle does not read the mark back")
		}
	}

	// without the read model the toggle is disabled and says why
	a.renderer.game = s.stubSim
	for _, bt := range a.buildingCommands(40) {
		if bt.ID == "bldg.bed.prisoners" && (!bt.Disabled || bt.Reason == "") {
			t.Fatalf("toggle without the read model %+v", bt)
		}
	}
	scr := ebiten.NewImage(a.sw, a.sh)
	a.drawBuildingMark(scr)
}

// The banner shows while a gathering runs, from the read model only.
func TestGatheringBanner(t *testing.T) {
	t.Parallel()
	s := bedStub()
	a, f := colonyApp(t, s.stubSim)
	a.renderer.game = s
	if _, _, ok := a.gatheringBanner(a.sw); ok {
		t.Fatal("banner with nothing under way")
	}
	s.gatherings = []game.GatheringView{{Kind: "wedding", Phase: "ceremony", Hosts: []string{"Ada", "Bo"}, X: 3, Y: 4}}
	text, r, ok := a.gatheringBanner(a.sw)
	if !ok || text != "Wedding of Ada and Bo: ceremony" {
		t.Fatalf("banner %q %v", text, ok)
	}
	l := a.hud()
	if r.Y < l.Bar.Bottom() || r.Y < l.Messages.Bottom() || r.X <= 0 || r.X+r.W >= float32(a.sw) {
		t.Fatalf("banner rect %+v not under the bar %+v and messages %+v", r, l.Bar, l.Messages)
	}
	if cx := r.X + r.W/2; cx < float32(a.sw)/2-1 || cx > float32(a.sw)/2+1 {
		t.Fatalf("banner rect %+v is not centred on the bar (screen centre %d)", r, a.sw/2)
	}
	for name, other := range map[string]rect{"right column": l.Right, "resources": l.Resources, "messages": l.Messages} {
		if !other.Empty() && r.X < other.Right() && other.X < r.Right() && r.Y < other.Bottom() && other.Y < r.Bottom() {
			t.Fatalf("banner %+v overlaps the %s %+v", r, name, other)
		}
	}
	a.gatheringUI(a.sw)
	if tip := tipOf(t, a, "hud.gathering"); !strings.Contains(tip, "Wedding of Ada and Bo") {
		t.Fatalf("tip %q", tip)
	}
	scr := ebiten.NewImage(a.sw, a.sh)
	a.drawGatheringBanner(scr, a.sw)
	if got := gatheringText(game.GatheringView{Kind: "party", Phase: "under way", Hosts: []string{"Cy"}}); got != "Party hosted by Cy: under way" {
		t.Fatalf("party line %q", got)
	}
	a.renderer.game = s.stubSim
	if _, _, ok := a.gatheringBanner(a.sw); ok {
		t.Fatal("banner without the read model")
	}
	f.frame(t, a)
}

// With notices up the banner sits below them, never over them (PLAY-018).
func TestGatheringBannerClearsTheMessages(t *testing.T) {
	t.Parallel()
	s := bedStub()
	s.world.Messages = []game.MessageView{{Tick: 1, Text: "A long notice about the wedding preparations"}, {Tick: 2, Text: "Another"}}
	a, _ := colonyApp(t, s.stubSim)
	a.renderer.game = s
	s.gatherings = []game.GatheringView{{Kind: "party", Phase: "under way", Hosts: []string{"Ada"}}}
	_, r, ok := a.gatheringBanner(a.sw)
	l := a.hud()
	if !ok || l.Messages.Empty() {
		t.Fatalf("banner %v, messages %+v", ok, l.Messages)
	}
	if r.Y < l.Messages.Bottom() {
		t.Fatalf("banner %+v is not below the messages %+v", r, l.Messages)
	}
}
