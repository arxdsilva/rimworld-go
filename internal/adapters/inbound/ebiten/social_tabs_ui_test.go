package ebiten

import (
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/windows"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// TestHealthSocialTabs (ENG-003 C4): the Health and Social tabs are live,
// say "No data yet." while their read models report nothing, and list the
// read models' lines once they do.
func TestHealthSocialTabs(t *testing.T) {
	t.Parallel()
	s := bioSim()
	a, f := colonyApp(t, s)
	a.renderer.Select(1)
	f.Frame(t, a)
	for _, tab := range []string{"health", "social"} {
		id := "inspector." + tab
		if b, ok := findButton(a, id); !ok || b.Disabled {
			t.Fatalf("%s is missing or disabled: %+v", id, b)
		}
		openPawnTab(t, a, tab) // the pane's tab click
		f.Frame(t, a)
		if !a.insp.Tabs().KitTabOn() || a.insp.Tabs().BioTabOn() {
			t.Fatalf("tab = %q after opening %s", a.insp.Tab, id)
		}
		tb, _ := paneTabByID(a.insp.Pawn().PawnPane(1), id)
		tb.Declare(innerFor(1280, 720, tb)) // no data yet: the tab says so
	}
	p := s.pawns[0]
	if a.insp.Tabs().HealthLines(p) != nil || a.insp.Tabs().SocialLines(p) != nil {
		t.Fatal("no read model: no lines (the tab shows No data yet.)")
	}
	s.health = map[entity.EntityID]game.HealthView{1: {Capacities: []game.CapacityView{{Label: "Moving", Pct: 85}}, Pain: 12,
		Parts: []game.PartView{{Label: "left leg", Status: "missing"}}}}
	s.social = map[entity.EntityID]game.SocialView{1: {Relations: []game.RelationRowView{{Name: "Bo", Label: "Sister", OpinionTo: 30, OpinionFrom: -5}},
		Log: []game.SocialLogLineView{{Text: "Ada and Bo chatted."}}}}
	h := strings.Join(a.insp.Tabs().HealthLines(p), "|")
	if !strings.Contains(h, "Moving 85%") || !strings.Contains(h, "Pain 12%") || !strings.Contains(h, "left leg: missing") {
		t.Errorf("health lines = %q", h)
	}
	so := strings.Join(a.insp.Tabs().SocialLines(p), "|")
	if !strings.Contains(so, "Sister Bo  +30 / -5") || !strings.Contains(so, "Ada and Bo chatted.") {
		t.Errorf("social lines = %q", so)
	}
	for _, tab := range []string{"health", "social"} {
		a.insp.Tab = tab
		tb, _ := paneTabByID(a.insp.Pawn().PawnPane(1), "inspector."+tab)
		tb.Declare(innerFor(1280, 720, tb)) // declares with data
		drawFrame(t, a, f)
	}
	a.insp.Tab = "character" // the pane itself: no tab open
	if a.insp.Tabs().KitTabOn() {
		t.Fatal("Character tab still shows the kit tab")
	}
}

// TestWardenWorkColumn (ENG-003): the Warden column sends WorkWarden and
// reads the Social skill.
func TestWardenWorkColumn(t *testing.T) {
	t.Parallel()
	if len(windows.WorkColumns) != 14 || windows.WorkColumns[12] != "Ward" || windows.WorkHelp[12] == "" {
		t.Fatalf("columns = %v", windows.WorkColumns)
	}
	if cmd, ok := windows.WorkTypeForCol(12); !ok || cmd != command.WorkWarden {
		t.Fatal("warden column not live")
	}
	if !workColSkill[12].ok {
		t.Fatal("warden column has no skill")
	}
	// FOOD-2: Plant cutting follows Warden, sends WorkPlantCut and reads Plants.
	if windows.WorkColumns[13] != "Cut" || windows.WorkHelp[13] == "" {
		t.Fatalf("columns = %v", windows.WorkColumns)
	}
	if cmd, ok := windows.WorkTypeForCol(13); !ok || cmd != command.WorkPlantCut {
		t.Fatal("plant cutting column not live")
	}
	if s := workColSkill[13]; !s.ok || s.sk != skill.Plants {
		t.Fatal("plant cutting column does not read Plants")
	}
}
