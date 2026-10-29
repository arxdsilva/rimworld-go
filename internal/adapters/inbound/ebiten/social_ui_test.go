package ebiten

import (
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/inspector"
	"rimworld-go/internal/adapters/inbound/ebiten/render/world"
	"rimworld-go/internal/adapters/inbound/ebiten/windows"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// socialFixture is Ada's Social read model: a wife, a rival who is dead,
// a prisoner and a log with a weighty line.
func socialFixture() game.SocialView {
	return game.SocialView{
		Relations: []game.RelationRowView{
			{Other: 2, Name: "Bo", Label: "Wife", OpinionTo: 75, OpinionFrom: 40,
				TermsTo:     []game.OpinionTermView{{Label: "Pretty", Value: 20}, {Label: "Kind words", Value: 10}},
				TermsFrom:   []game.OpinionTermView{{Label: "Insulted me", Value: -12}},
				RomanceNote: "Bo is open to romance."},
			{Other: 3, Name: "Cy", Label: "Rival", Status: "dead", OpinionTo: -70, OpinionFrom: -10},
			{Other: 4, Name: "Di", Label: "Acquaintance", Status: "prisoner", OpinionTo: 0, OpinionFrom: 5,
				TermsTo: []game.OpinionTermView{{Label: "Met", Value: 1}}},
		},
		Log: []game.SocialLogLineView{
			{Tick: 100, Text: "Ada chatted with Bo.", Icon: "social_chitchat"},
			{Tick: 900, Text: "Ada and Bo got married.", Icon: "social_proposal", Weighty: true},
			{Tick: 500, Text: "Ada insulted Cy.", Icon: "social_insult"},
		},
	}
}

func TestOpinionBands(t *testing.T) {
	t.Parallel()
	for v, want := range map[int]int{-100: 0, -60: 0, -59: 1, -21: 1, -20: 2, 0: 2, 20: 2, 21: 3, 59: 3, 60: 4, 61: 4, 100: 4} {
		if got := inspector.OpinionBand(v); got != want {
			t.Errorf("opinionBand(%d) = %d, want %d", v, got, want)
		}
	}
	seen := map[[4]uint8]bool{}
	for _, v := range []int{-80, -40, 0, 40, 80} {
		c := inspector.OpinionColor(v)
		seen[[4]uint8{c.R, c.G, c.B, c.A}] = true
	}
	if len(seen) != 5 {
		t.Errorf("five bands should have five colours, got %d", len(seen))
	}
}

func TestRelationTipOrder(t *testing.T) {
	t.Parallel()
	r := socialFixture().Relations[0]
	want := "Opinion of Bo: 75\nPretty: +20\nKind words: +10\n\nBo is open to romance.\n\nBo's opinion of me: 40\nInsulted me: -12"
	if got := inspector.RelationTip(r); got != want {
		t.Errorf("tip =\n%q\nwant\n%q", got, want)
	}
	r.RomanceNote = ""
	if got := inspector.RelationTip(r); strings.Contains(got, "romance") || strings.Contains(got, "\n\n\n") {
		t.Errorf("no note, no extra blank: %q", got)
	}
}

func TestStatusTags(t *testing.T) {
	t.Parallel()
	if inspector.StatusTag("") != "" || inspector.StatusTag("dead") != "dead" || inspector.StatusTag("off-map") != "off-map" ||
		inspector.StatusTag("prisoner") != "prisoner" || inspector.StatusTag("kidnapped") != "kidnapped" || inspector.StatusTag("caravan") != "in caravan" {
		t.Error("status tags")
	}
}

func TestSocialTabRowsAndEmpty(t *testing.T) {
	t.Parallel()
	s := bioSim()
	s.social = map[entity.EntityID]game.SocialView{1: socialFixture()}
	a, f := colonyApp(t, s)
	a.renderer.Select(1)
	f.Frame(t, a)
	openPawnTab(t, a, "social") // the pane's Social tab
	f.Frame(t, a)
	f.Frame(t, a)
	if !a.insp.Tabs().KitTabOn() {
		t.Fatal("social tab not open")
	}
	tip := tipOf(t, a, "social.row.0")
	if !strings.HasPrefix(tip, "Opinion of Bo: 75\nPretty: +20") || !strings.Contains(tip, "Bo's opinion of me: 40") {
		t.Errorf("row 0 tip = %q", tip)
	}
	for _, id := range []string{"social.row.1", "social.row.2"} {
		if !hasWidget(a, id) {
			t.Errorf("no %s", id)
		}
	}
	if hasWidget(a, "social.row.3") {
		t.Error("only three relations")
	}
	// Newest log line first.
	log := inspector.LogNewestFirst(socialFixture().Log)
	if log[0].Tick != 900 || log[1].Tick != 500 || log[2].Tick != 100 {
		t.Errorf("log order = %v", log)
	}
	drawFrame(t, a, f)

	// Empty read model: the empty state, no row widgets.
	s.social = map[entity.EntityID]game.SocialView{1: {}}
	f.Frame(t, a)
	if hasWidget(a, "social.row.0") {
		t.Error("empty state has rows")
	}
	if inspector.SocialEmptyText != "No relationships yet" {
		t.Errorf("empty text = %q", inspector.SocialEmptyText)
	}
	drawFrame(t, a, f)
}

func TestSocialRowsScroll(t *testing.T) {
	t.Parallel()
	s := bioSim()
	var v game.SocialView
	for i := 0; i < 40; i++ {
		v.Relations = append(v.Relations, game.RelationRowView{Other: uint64(10 + i), Name: "P", Label: "Friend", OpinionTo: i})
	}
	s.social = map[entity.EntityID]game.SocialView{1: v}
	a, f := colonyApp(t, s)
	a.renderer.Select(1)
	f.Frame(t, a)
	openPawnTab(t, a, "social") // the pane's Social tab
	f.Frame(t, a)
	f.Frame(t, a)
	if hasWidget(a, "social.row.39") {
		t.Error("an off-screen row should not be declared")
	}
	if !hasWidget(a, "social.rows") {
		t.Error("the relation list is a scroll view")
	}
	drawFrame(t, a, f)
}

func TestInteractionBubbleRecent(t *testing.T) {
	t.Parallel()
	p := game.PawnView{Interaction: "insult", InteractTick: 1000}
	if !world.InteractionRecent(p, 1000) || !world.InteractionRecent(p, 1299) {
		t.Error("recent within 300 ticks")
	}
	if world.InteractionRecent(p, 1301) {
		t.Error("stale after 300 ticks")
	}
	if world.InteractionRecent(game.PawnView{InteractTick: 1000}, 1000) {
		t.Error("no interaction, no bubble")
	}
	if world.InteractionRecent(p, 900) != true {
		t.Error("a tick before the interaction (load) still draws")
	}
	if world.InteractionIconKey("insult") != "ui_icon/social_insult" || world.InteractionIconKey("nonsense") != "" {
		t.Error("icon keys")
	}
	for _, k := range []string{"chitchat", "deep_talk", "slight", "insult", "kind_words", "romance", "rebuff", "proposal", "breakup", "fight", "lovin"} {
		if world.InteractionIconKey(k) == "" {
			t.Errorf("kind %s unknown", k)
		}
	}
}

func TestInteractionBubblesDrawWithoutArt(t *testing.T) {
	t.Parallel()
	s := drawSim()
	s.tick = clock.Tick(1010)
	s.pawns[0].Interaction, s.pawns[0].InteractTick = "romance", 1000
	s.pawns[1].SocialFight = true
	s.pawns[1].Interaction, s.pawns[1].InteractTick = "fight", 1000
	a, f := colonyApp(t, s)
	a.renderer.speed = speedPaused
	img := drawFrame(t, a, f)
	if img == nil {
		t.Fatal("no frame")
	}
}

func TestBioRelationLines(t *testing.T) {
	t.Parallel()
	rows := append(socialFixture().Relations, game.RelationRowView{Name: "Ed", Label: "Friend"})
	got := inspector.BioRelationLines(rows)
	if len(got) != 3 || got[0] != "Wife: Bo" || got[1] != "Rival: Cy" || got[2] != "Acquaintance: Di" {
		t.Errorf("lines = %v", got)
	}
	if len(inspector.BioRelationLines(nil)) != 0 {
		t.Error("none")
	}
}

func TestBioTabShowsRelations(t *testing.T) {
	t.Parallel()
	s := bioSim()
	s.social = map[entity.EntityID]game.SocialView{1: socialFixture()}
	a, f := colonyApp(t, s)
	a.renderer.Select(1)
	f.Frame(t, a)
	openPawnTab(t, a, "bio") // the pane's Bio tab
	f.Frame(t, a)
	if !hasWidget(a, "bio.relations") {
		t.Fatal("Bio tab has no relations block")
	}
	if tip := tipOf(t, a, "bio.relations"); !strings.Contains(tip, "Wife: Bo") {
		t.Errorf("tip = %q", tip)
	}
	drawFrame(t, a, f)
}

func TestHistoryTab(t *testing.T) {
	t.Parallel()
	s := bioSim()
	s.history = []game.HistoryEntryView{
		{Tick: 100, Text: "Colony founded.", Kind: "colony"},
		{Tick: 90000, Text: "Ada and Bo married.", Kind: "social", Pawns: []uint64{1, 2}},
	}
	a, f := colonyApp(t, s)
	for _, b := range a.chrome.Tabs().Buttons(a.sw, a.sh) {
		if b.ID == "tab.history" && b.Disabled {
			t.Fatal("history tab disabled")
		}
	}
	press(t, a, f, "tab.history")
	f.Frame(t, a)
	if a.uiModalTop() != windows.HistoryModalID {
		t.Fatalf("modal = %q", a.uiModalTop())
	}
	f.Frame(t, a)
	if !hasWidget(a, "history.row.0") || !hasWidget(a, "history.row.1") {
		t.Fatal("history rows missing")
	}
	rows := windows.HistoryRows(s.history)
	if rows[0].Text != "Ada and Bo married." || !strings.Contains(rows[0].When, "Y") {
		t.Errorf("rows = %+v", rows)
	}
	drawFrame(t, a, f)
	// Clicking the newest entry selects its first pawn.
	press(t, a, f, "history.row.0")
	if !a.renderer.HasSelect() || a.renderer.Selected() != 1 {
		t.Errorf("selected = %v", a.renderer.Selected())
	}
}

func TestHistoryEmpty(t *testing.T) {
	t.Parallel()
	a, f := colonyApp(t, bioSim())
	a.win.History().Launch()
	drawFrame(t, a, f)
	drawFrame(t, a, f)
}
