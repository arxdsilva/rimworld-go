package ebiten

import (
	"strings"
	"testing"

	"rimworld-go/internal/application/game"
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
	f.frame(t, a)
	for _, tab := range []string{"health", "social"} {
		id := "inspector." + tab
		for _, b := range a.inspectorButtons() {
			if b.id == id && b.disabled {
				t.Fatalf("%s is disabled: %s", id, b.reason)
			}
		}
		click(t, a, id)
		f.frame(t, a)
		if a.inspectTab != tab || !a.kitTabOn() || a.bioTabOn() {
			t.Fatalf("tab = %q after clicking %s", a.inspectTab, id)
		}
	}
	p := s.pawns[0]
	if a.healthLines(p) != nil || a.socialLines(p) != nil {
		t.Fatal("no read model: no lines (the tab shows No data yet.)")
	}
	s.health = map[entity.EntityID]game.HealthView{1: {Capacities: []game.CapacityView{{Label: "Moving", Pct: 85}}, Pain: 12,
		Parts: []game.PartView{{Label: "left leg", Status: "missing"}}}}
	s.social = map[entity.EntityID]game.SocialView{1: {Relations: []game.RelationRowView{{Name: "Bo", Label: "Sister", OpinionTo: 30, OpinionFrom: -5}},
		Log: []game.SocialLogLineView{{Text: "Ada and Bo chatted."}}}}
	h := strings.Join(a.healthLines(p), "|")
	if !strings.Contains(h, "Moving 85%") || !strings.Contains(h, "Pain 12%") || !strings.Contains(h, "left leg: missing") {
		t.Errorf("health lines = %q", h)
	}
	so := strings.Join(a.socialLines(p), "|")
	if !strings.Contains(so, "Sister Bo  +30 / -5") || !strings.Contains(so, "Ada and Bo chatted.") {
		t.Errorf("social lines = %q", so)
	}
	f.frame(t, a) // draws with data
	click(t, a, "inspector.character")
	if a.kitTabOn() {
		t.Fatal("Character tab still shows the kit tab")
	}
}

// TestWardenWorkColumn (ENG-003): the Warden column sends WorkWarden and
// reads the Social skill.
func TestWardenWorkColumn(t *testing.T) {
	t.Parallel()
	if len(workColumns) != 13 || workColumns[12] != "Ward" || workHelp[12] == "" {
		t.Fatalf("columns = %v", workColumns)
	}
	if cmd, ok := workTypeForCol(12); !ok || cmd != command.WorkWarden {
		t.Fatal("warden column not live")
	}
	if !workColSkill[12].ok {
		t.Fatal("warden column has no skill")
	}
}
