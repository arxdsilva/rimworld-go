package ebiten

import (
	"errors"
	"fmt"
	"rimworld-go/internal/adapters/inbound/ebiten/mainmenu"
	"rimworld-go/internal/adapters/inbound/ebiten/options"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	wizardpkg "rimworld-go/internal/adapters/inbound/ebiten/wizard"
	"sort"
	"strings"
	"testing"
	"time"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/difficulty"
	"rimworld-go/internal/domain/scenario"
	"rimworld-go/internal/domain/storyteller"
	"rimworld-go/internal/platform/config"
	"rimworld-go/internal/platform/input"
)

// quickStart drives a menu App from the main menu to a running colony
// with the defaults: New Colony, Next through Scenario and Storyteller, a
// tiny (5%) world, a random landing site and Start.
func quickStart(t *testing.T, a *App) {
	t.Helper()
	toPawns(t, a)
	click(t, a, "wizard.start")
	if a.nav.screen() != ports.ScreenColony {
		t.Fatalf("quickStart ended on %s, want colony", a.nav.screen())
	}
}

// memStore is an in-memory ScenarioStore.
type memStore struct {
	files   map[string][]byte
	deleted []string
	saveErr error
	listErr error
}

func newMemStore() *memStore { return &memStore{files: map[string][]byte{}} }

func (m *memStore) List() ([]wizardpkg.ScenarioFile, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	var out []wizardpkg.ScenarioFile
	for n := range m.files {
		out = append(out, wizardpkg.ScenarioFile{Name: n, Modified: "2026-01-02 03:04"})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (m *memStore) Load(name string) ([]byte, error) {
	b, ok := m.files[name]
	if !ok {
		return nil, errors.New("no such file")
	}
	return b, nil
}

func (m *memStore) Save(name string, data []byte) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.files[name] = append([]byte(nil), data...)
	return nil
}

func (m *memStore) Delete(name string) error {
	m.deleted = append(m.deleted, name)
	delete(m.files, name)
	return nil
}

// putDef stores d as a scenario file.
func (m *memStore) putDef(t *testing.T, name string, d scenario.Def) {
	t.Helper()
	b, err := scenario.Encode(d)
	if err != nil {
		t.Fatal(err)
	}
	m.files[name] = b
}

// ngTest is a menu App with a fake input and an in-memory scenario store.
func ngTest(t *testing.T, mod func(*AppConfig)) (*App, *fakeInput, *memStore) {
	t.Helper()
	st := newMemStore()
	a := appWith(&stubSim{w: 8, h: 8}, func(c *AppConfig) {
		c.Scenarios = st
		if mod != nil {
			mod(c)
		}
	}, nil)
	a.finishBoot()
	f := fakeOn(a)
	f.Cur.Mx, f.Cur.My = -50, -50
	return a, f, st
}

// sizeTo sets the window size and runs a frame so the widgets follow it.
func sizeTo(t *testing.T, a *App, f *fakeInput, w, h int) {
	t.Helper()
	a.sw, a.sh = w, h
	f.Frame(t, a)
}

// widgetAt is the centre of the widget id declared this frame.
func widgetAt(t *testing.T, a *App, id string) (int, int) {
	t.Helper()
	w, ok := a.uiSt().Find(id)
	if !ok {
		t.Fatalf("no widget %q on %s", id, a.nav.screen())
	}
	x, y := w.R.Center()
	return int(x), int(y)
}

// press clicks the widget id with the mouse (two frames), the real path.
func press(t *testing.T, a *App, f *fakeInput, id string) {
	t.Helper()
	f.Frame(t, a)
	x, y := widgetAt(t, a, id)
	f.clickAt(t, a, x, y)
	f.Cur.Mx, f.Cur.My = -50, -50
}

// hasWidget reports id is declared this frame.
func hasWidget(a *App, id string) bool {
	_, ok := a.uiSt().Find(id)
	return ok
}

func rowKeys(a *App, ng *wizardpkg.Draft) []string {
	rows, _ := ng.Rows(a.cfg.Scenarios != nil)
	var out []string
	for _, r := range rows {
		switch r.Kind {
		case wizardpkg.RowHeader:
			out = append(out, "#"+r.Text)
		case wizardpkg.RowEntry:
			out = append(out, ng.Entries[r.Entry].Key)
		case wizardpkg.RowNote:
			out = append(out, "!"+r.Text)
		}
	}
	return out
}

func TestMainMenuNamesNewColonyAndKeepsIDs(t *testing.T) {
	t.Parallel()
	a, _, _ := ngTest(t, nil)
	btns := a.menuScreen.Buttons(a.sw, a.sh)
	if btns[0].ID != "menu.newgame" || btns[0].Label != "New Colony" {
		t.Fatalf("first button = %+v, want New Colony on menu.newgame while there are no saves", btns[0])
	}
	if btns[1].ID != "menu.continue" || !btns[1].Disabled || btns[1].Reason != "no saves yet" {
		t.Fatalf("continue = %+v, want it disabled after New Colony", btns[1])
	}
	for _, b := range btns {
		if b.W != mainmenu.RailW || b.H != mainmenu.BtnH || b.X != float32(a.sw-340) {
			t.Fatalf("%s is %vx%v at x=%v, want 300x40 on the right rail", b.ID, b.W, b.H, b.X)
		}
	}
	if btns[0].Y != 140 || btns[1].Y != 140+mainmenu.BtnH+mainmenu.BtnGap {
		t.Fatalf("rail starts at %v with a %v step", btns[0].Y, btns[1].Y-btns[0].Y)
	}
}

func TestNewColonyCreatesFreshDraft(t *testing.T) {
	t.Parallel()
	a, _, _ := ngTest(t, nil)
	if a.ng != nil {
		t.Fatal("no draft before New Colony")
	}
	click(t, a, "menu.newgame")
	ng := a.ng
	if ng == nil || a.nav.screen() != ports.ScreenScenario {
		t.Fatalf("draft=%v screen=%s", ng, a.nav.screen())
	}
	if ng.Storyteller != "steady" || ng.Preset != difficulty.Settled || ng.SaveMode != difficulty.ReloadAnytime {
		t.Fatalf("defaults = %s %v %v, want Orla Vane, Settled, Reload anytime", ng.Storyteller, ng.Preset, ng.SaveMode)
	}
	if sel := ng.Selected(); sel == nil || sel.Key != "broken_hull" {
		t.Fatalf("default scenario = %+v, want Broken Hull", sel)
	}
	if ng.Tile != -1 || ng.MapSize <= 0 || ng.MapSize > game.MaxEnabledSide {
		t.Fatalf("tile=%d mapSize=%d", ng.Tile, ng.MapSize)
	}
	// Edit it, leave through the main menu, come back: a fresh draft.
	ng.Storyteller, ng.Preset = "gambler", difficulty.Grim
	a.popScreen()
	if a.nav.screen() != ports.ScreenMainMenu || a.ng != nil {
		t.Fatalf("the main menu discards the draft: screen=%s ng=%v", a.nav.screen(), a.ng)
	}
	click(t, a, "menu.newgame")
	if a.ng == ng || a.ng.Storyteller != "steady" || a.ng.Preset != difficulty.Settled {
		t.Fatalf("New Colony must start a fresh draft: %+v", a.ng)
	}
}

func TestNewDraftUsesTheLastChoicesFromSettings(t *testing.T) {
	t.Parallel()
	a, _, _ := ngTest(t, nil)
	a.settings.Gameplay.LastStoryteller = "gambler"
	a.settings.Gameplay.LastDifficulty = "grim"
	a.settings.Gameplay.LastSaveMode = difficulty.Commitment.String()
	click(t, a, "menu.newgame")
	if ng := a.ng; ng.Storyteller != "gambler" || ng.Preset != difficulty.Grim || ng.SaveMode != difficulty.Commitment {
		t.Fatalf("draft = %s %v %v, want the remembered choices", ng.Storyteller, ng.Preset, ng.SaveMode)
	}
	// Garbage in the settings falls back to the defaults.
	a2, _, _ := ngTest(t, nil)
	a2.settings.Gameplay.LastStoryteller = "legacy" // hidden
	a2.settings.Gameplay.LastDifficulty = "custom"
	click(t, a2, "menu.newgame")
	if ng := a2.ng; ng.Storyteller != "steady" || ng.Preset != difficulty.Settled {
		t.Fatalf("draft = %s %v, want the defaults", ng.Storyteller, ng.Preset)
	}
}

func TestContinueLoadsNewestIncludingCommitment(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	slots := []ports.SaveSlot{
		{Name: "old", Path: "/s/old", Modified: base},
		{Name: "broken", Path: "/s/broken", Modified: base.Add(10 * time.Hour), Err: "unreadable"},
		{Name: "colony-commitment", Path: "/s/commit", Modified: base.Add(3 * time.Hour)},
		{Name: "mid", Path: "/s/mid", Modified: base.Add(time.Hour)},
	}
	var loaded []string
	a, _, _ := ngTest(t, func(c *AppConfig) {
		c.Saves = SaveHooks{
			List: func() ([]ports.SaveSlot, error) { return slots, nil },
			Load: func(path string) (Simulation, string, error) {
				loaded = append(loaded, path)
				return &stubSim{w: 8, h: 8}, "colony-commitment", nil
			},
		}
	})
	btns := a.menuScreen.Buttons(a.sw, a.sh)
	if btns[0].ID != "menu.continue" || btns[0].Disabled {
		t.Fatalf("with saves Continue leads the rail: %+v", btns[0])
	}
	click(t, a, "menu.continue")
	if len(loaded) != 1 || loaded[0] != "/s/commit" {
		t.Fatalf("loaded %v, want the newest loadable save (the commitment one)", loaded)
	}
	if a.nav.screen() != ports.ScreenColony {
		t.Fatalf("screen = %s, want colony", a.nav.screen())
	}
	if testApp(nil).newestSlot() != -1 {
		t.Fatal("no slots, no newest")
	}
}

// ---- Choose Scenario ------------------------------------------------------

func TestScenarioListShowsBuiltinsThenCustom(t *testing.T) {
	t.Parallel()
	a, f, st := ngTest(t, nil)
	d, _ := scenario.Builtin("bare_hands")
	d.Builtin, d.Key, d.Name = false, "", "Zeta camp"
	st.putDef(t, "zeta", d)
	d.Name = "alpha camp"
	st.putDef(t, "alpha", d)
	extra := wizardpkg.CloneDef(d)
	extra.Name = "Mid unsaved"
	a.wiz.Unsaved = append(a.wiz.Unsaved, extra)
	click(t, a, "menu.newgame")
	sizeTo(t, a, f, 1920, 1080) // every row fits, so every delete button is declared

	got := rowKeys(a, a.ng)
	want := []string{
		"#Built-in", "broken_hull", "ember_clan", "gilded_expedition", "bare_hands",
		"#Your scenarios", "file.alpha", "unsaved.0", "file.zeta",
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("rows = %v\nwant   %v", got, want)
	}
	for _, e := range a.ng.Entries {
		if e.Unsaved && !strings.HasSuffix(e.Name, "(unsaved)") {
			t.Errorf("unsaved scenario is named %q", e.Name)
		}
	}
	// Every custom row has a delete button; built-ins do not.
	a.uiSt().Scroll["scen.list"] = 1000 // down to the last rows
	f.Frame(t, a)
	for _, k := range []string{"file.alpha", "unsaved.0", "file.zeta"} {
		if !hasWidget(a, "scen.del."+k) {
			t.Errorf("no delete button for %s", k)
		}
	}
	if hasWidget(a, "scen.del.broken_hull") {
		t.Error("a built-in scenario must not be deletable")
	}
}

func TestScenarioListEmptyStates(t *testing.T) {
	t.Parallel()
	a, f, _ := ngTest(t, nil)
	click(t, a, "menu.newgame")
	f.Frame(t, a)
	if got := rowKeys(a, a.ng); got[len(got)-1] != "!No saved scenarios yet — use New scenario" {
		t.Fatalf("empty custom group note = %q", got[len(got)-1])
	}
	a.cfg.Scenarios = nil
	rows := rowKeys(a, a.ng)
	if !strings.Contains(rows[len(rows)-1], "unavailable") {
		t.Fatalf("without a store the note says so: %q", rows[len(rows)-1])
	}
	if hasWidget(a, "scen.import") {
		f.Frame(t, a)
	}
	w, _ := a.uiSt().Find("scen.import")
	if !w.Disabled {
		t.Fatal("Load file... needs a store")
	}
}

func TestScenarioSearchFilters(t *testing.T) {
	t.Parallel()
	a, f, st := ngTest(t, nil)
	d, _ := scenario.Builtin("bare_hands")
	d.Builtin, d.Key, d.Name = false, "", "Ember Clan II"
	st.putDef(t, "quiet", d)
	click(t, a, "menu.newgame")
	f.Frame(t, a)

	a.ng.Search = "ember clan"
	if got := strings.Join(rowKeys(a, a.ng), ","); got != "#Built-in,ember_clan,#Your scenarios,file.ember clan ii" && got != "#Built-in,ember_clan,#Your scenarios,file.quiet" {
		t.Fatalf("search 'ember clan' = %s", got)
	}
	a.ng.Search = "HULL BREACH" // summary text, case-insensitive
	if got := strings.Join(rowKeys(a, a.ng), ","); !strings.HasPrefix(got, "#Built-in,broken_hull,#Your scenarios,!No scenario matches") {
		t.Fatalf("search by summary = %s", got)
	}
	a.ng.Search = "zzz"
	for _, k := range rowKeys(a, a.ng) {
		if !strings.HasPrefix(k, "#") && !strings.HasPrefix(k, "!") {
			t.Fatalf("nothing matches 'zzz' yet row %q is listed", k)
		}
	}

	// Through the real field: Ctrl+F focuses it, typing filters.
	a.ng.Search = ""
	f.Key(t, a, "F", input.ModCtrl)
	f.Frame(t, a)
	if a.uiSt().Edit.ID != "scen.search" {
		t.Fatalf("Ctrl+F should focus the search field, editing %q", a.uiSt().Edit.ID)
	}
	f.TypeText(t, a, "bare")
	if a.ng.Search != "bare" {
		t.Fatalf("search = %q, want bare", a.ng.Search)
	}
	if got := strings.Join(rowKeys(a, a.ng), ","); !strings.Contains(got, "bare_hands") || strings.Contains(got, "ember_clan") {
		t.Fatalf("typed filter = %s", got)
	}
}

func TestScenarioDetailsFromParts(t *testing.T) {
	t.Parallel()
	text := func(ls []game.DetailLine) string {
		var b strings.Builder
		for _, l := range ls {
			if l.Heading {
				b.WriteString("## ")
			}
			b.WriteString(l.Text + "\n")
		}
		return b.String()
	}
	hull, _ := scenario.Builtin("broken_hull")
	got := text(wizardpkg.ScenarioLines(hull))
	for _, want := range []string{
		"Player faction\tSettlers", "Starting people\t3, chosen from 8 candidates", "Arrival\tDrop pods",
		"Tech level\tIndustrial", "Research cost\t100% of normal",
		"## Research granted\nFood preservation\nTraps", "## Starting equipment and supplies",
		"Rifle ×2", "Industrial medicine ×12", "Silver ×600",
		"## Starting animals\n1 random pet", "## Scattered on the map", "Steel ×400 in 4 piles, near the start",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("Broken Hull details lack %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "Special rules") {
		t.Errorf("Broken Hull has no special rules:\n%s", got)
	}
	var rifle game.DetailLine
	for _, l := range wizardpkg.ScenarioLines(hull) {
		if l.Text == "Rifle ×2" {
			rifle = l
		}
	}
	if !strings.Contains(rifle.Tip, "Rifle") {
		t.Errorf("item line has no tooltip: %+v", rifle)
	}

	ember, _ := scenario.Builtin("ember_clan")
	if got := text(wizardpkg.ScenarioLines(ember)); !strings.Contains(got, "Player faction\tClan") || !strings.Contains(got, "Research cost\t150% of normal") || !strings.Contains(got, "Arrival\tStanding") {
		t.Errorf("Ember Clan details:\n%s", got)
	}
	bare, _ := scenario.Builtin("bare_hands")
	got = text(wizardpkg.ScenarioLines(bare))
	if !strings.Contains(got, "## Special rules") || !strings.Contains(got, "No clothes") || !strings.Contains(got, "Forced condition: Exhausted") {
		t.Errorf("Bare Hands rules:\n%s", got)
	}

	// The layout wraps and totals the rows.
	L := wizardpkg.BuildDetail(hull, wizardpkg.ScenarioLines(hull), 400)
	if len(L.Rows) < 20 || L.Total <= 0 {
		t.Fatalf("layout has %d rows, height %v", len(L.Rows), L.Total)
	}
	narrow := wizardpkg.BuildDetail(hull, wizardpkg.ScenarioLines(hull), 200)
	if narrow.Total <= L.Total {
		t.Fatalf("a narrower panel must be taller: %v vs %v", narrow.Total, L.Total)
	}
}

func TestInvalidScenarioBlocksNext(t *testing.T) {
	t.Parallel()
	a, f, st := ngTest(t, nil)
	d, _ := scenario.Builtin("bare_hands")
	d.Builtin, d.Key, d.Name, d.People, d.ChooseFrom = false, "", "Broken one", 0, 0
	st.putDef(t, "bad", d)
	st.files["junk"] = []byte("{not json")
	click(t, a, "menu.newgame")
	f.Frame(t, a)

	press(t, a, f, "scen.row.file.bad")
	f.Frame(t, a)
	if sel := a.ng.Selected(); sel == nil || sel.Key != "file.bad" {
		t.Fatalf("the invalid scenario should still be selectable: %+v", sel)
	}
	w, _ := a.uiSt().Find("wizard.next")
	if !w.Disabled || w.Reason != "Fix 1 problem in the editor" {
		t.Fatalf("Next = disabled %v reason %q", w.Disabled, w.Reason)
	}
	click(t, a, "wizard.next")
	if a.nav.screen() != ports.ScreenScenario {
		t.Fatalf("Next on an invalid scenario moved to %s", a.nav.screen())
	}
	f.Key(t, a, "Enter", 0)
	if a.nav.screen() != ports.ScreenScenario {
		t.Fatalf("Enter on an invalid scenario moved to %s", a.nav.screen())
	}

	// An unreadable file greys out, explains and can't be selected.
	press(t, a, f, "scen.row.file.junk")
	if sel := a.ng.Selected(); sel.Key != "file.bad" {
		t.Fatalf("an unreadable file was selected: %s", sel.Key)
	}
	var junk wizardpkg.ScenEntry
	for _, e := range a.ng.Entries {
		if e.Key == "file.junk" {
			junk = e
		}
	}
	if junk.Err == "" || !strings.Contains(junk.Summary, "Can't read") {
		t.Fatalf("junk entry = %+v", junk)
	}
	jw, _ := a.uiSt().Find("scen.row.file.junk")
	if !strings.Contains(jw.Tip, "Can't read this file") {
		t.Fatalf("tooltip = %q", jw.Tip)
	}

	// A valid selection enables Next, which stamps a copy into the draft.
	press(t, a, f, "scen.row.ember_clan")
	f.Frame(t, a)
	w, _ = a.uiSt().Find("wizard.next")
	if w.Disabled {
		t.Fatalf("Next disabled for a valid scenario: %q", w.Reason)
	}
	click(t, a, "wizard.next")
	if a.nav.screen() != ports.ScreenStoryteller || a.ng.Scenario.Key != "ember_clan" || a.ng.Scenario.People != 5 {
		t.Fatalf("screen=%s draft scenario=%+v", a.nav.screen(), a.ng.Scenario)
	}
	// The draft holds a copy, not the list entry's slice.
	a.ng.Scenario.Parts[0].Qty = 1
	if a.ng.Entries[a.ng.Sel].Def.Parts[0].Qty == 1 {
		t.Fatal("the draft aliases the list entry")
	}
}

func TestEnterAdvancesFromScenario(t *testing.T) {
	t.Parallel()
	a, f, _ := ngTest(t, nil)
	click(t, a, "menu.newgame")
	f.Frame(t, a)
	f.Key(t, a, "Enter", 0)
	if a.nav.screen() != ports.ScreenStoryteller {
		t.Fatalf("Enter = %s, want storyteller", a.nav.screen())
	}
}

func TestScenarioListKeyboardSelection(t *testing.T) {
	t.Parallel()
	a, f, _ := ngTest(t, nil)
	click(t, a, "menu.newgame")
	f.Frame(t, a)
	press(t, a, f, "scen.row.broken_hull") // focuses the list
	f.Key(t, a, "ArrowDown", 0)
	f.Key(t, a, "ArrowDown", 0)
	if a.ng.Selected().Key != "gilded_expedition" {
		t.Fatalf("two Downs = %s", a.ng.Selected().Key)
	}
	f.Key(t, a, "ArrowUp", 0)
	if a.ng.Selected().Key != "ember_clan" {
		t.Fatalf("Up = %s", a.ng.Selected().Key)
	}
}

func TestDeleteCustomConfirms(t *testing.T) {
	t.Parallel()
	a, f, st := ngTest(t, nil)
	d, _ := scenario.Builtin("bare_hands")
	d.Builtin, d.Key, d.Name = false, "", "Doomed"
	st.putDef(t, "doomed", d)
	click(t, a, "menu.newgame")
	f.Frame(t, a)

	press(t, a, f, "scen.del.file.doomed")
	if a.uiModalTop() != "scen.delete" || len(st.deleted) != 0 {
		t.Fatalf("modal=%q deleted=%v: the delete button must only ask", a.uiModalTop(), st.deleted)
	}
	// Cancel keeps it.
	press(t, a, f, "modal.scen.delete.cancel")
	f.Frame(t, a)
	if a.uiSt().ModalOpen() || len(st.deleted) != 0 {
		t.Fatalf("cancel: modal=%v deleted=%v", a.uiSt().ModalOpen(), st.deleted)
	}
	// OK removes it and the list follows.
	press(t, a, f, "scen.del.file.doomed")
	press(t, a, f, "modal.scen.delete.ok")
	f.Frame(t, a)
	if len(st.deleted) != 1 || st.deleted[0] != "doomed" {
		t.Fatalf("deleted = %v", st.deleted)
	}
	for _, e := range a.ng.Entries {
		if e.File == "doomed" {
			t.Fatal("the deleted scenario is still listed")
		}
	}
	// An unsaved scenario is deleted without touching the store.
	a.wiz.Unsaved = append(a.wiz.Unsaved, d)
	a.ng.RefreshScenarios(a.cfg.Scenarios, a.wiz.Unsaved)
	f.Frame(t, a)
	press(t, a, f, "scen.del.unsaved.0")
	press(t, a, f, "modal.scen.delete.ok")
	f.Frame(t, a)
	if len(a.wiz.Unsaved) != 0 || len(st.deleted) != 1 {
		t.Fatalf("unsaved=%d deleted=%v", len(a.wiz.Unsaved), st.deleted)
	}
}

func TestLoadFileModalSelectsTheFile(t *testing.T) {
	t.Parallel()
	a, f, st := ngTest(t, nil)
	d, _ := scenario.Builtin("ember_clan")
	d.Builtin, d.Key, d.Name = false, "", "From disk"
	st.putDef(t, "fromdisk", d)
	click(t, a, "menu.newgame")
	f.Frame(t, a)
	press(t, a, f, "scen.import")
	if a.uiModalTop() != "scen.load" {
		t.Fatalf("modal = %q", a.uiModalTop())
	}
	f.Frame(t, a)
	press(t, a, f, "modal.scen.load.table.body.0")
	f.Frame(t, a)
	if a.uiSt().ModalOpen() || a.ng.Selected().Key != "file.fromdisk" {
		t.Fatalf("modal=%v selected=%s", a.uiSt().ModalOpen(), a.ng.Selected().Key)
	}
}

func TestBackDiscardConfirmOnlyWhenChanged(t *testing.T) {
	t.Parallel()
	a, f, _ := ngTest(t, nil)
	click(t, a, "menu.newgame")
	f.Frame(t, a)
	// Nothing changed: Back leaves at once.
	click(t, a, "wizard.back")
	if a.nav.screen() != ports.ScreenMainMenu || a.uiSt().ModalOpen() {
		t.Fatalf("an untouched draft: screen=%s modal=%v", a.nav.screen(), a.uiSt().ModalOpen())
	}
	// Changing a later step makes Back from Choose Scenario ask.
	click(t, a, "menu.newgame")
	click(t, a, "wizard.next")
	click(t, a, "story.pick.gambler")
	click(t, a, "wizard.back")
	f.Frame(t, a)
	click(t, a, "wizard.back")
	if a.nav.screen() != ports.ScreenScenario || a.uiModalTop() != "scen.discard" {
		t.Fatalf("screen=%s modal=%q, want the discard confirmation", a.nav.screen(), a.uiModalTop())
	}
	// Cancel stays, the draft survives.
	press(t, a, f, "modal.scen.discard.cancel")
	f.Frame(t, a)
	if a.nav.screen() != ports.ScreenScenario || a.ng == nil || a.ng.Storyteller != "gambler" {
		t.Fatalf("cancel left screen=%s draft=%v", a.nav.screen(), a.ng)
	}
	// Esc asks again; Discard leaves and drops the draft.
	f.Key(t, a, "Escape", 0)
	if a.uiModalTop() != "scen.discard" {
		t.Fatalf("Escape on a changed draft: modal=%q", a.uiModalTop())
	}
	press(t, a, f, "modal.scen.discard.ok")
	f.Frame(t, a)
	if a.nav.screen() != ports.ScreenMainMenu || a.ng != nil {
		t.Fatalf("discard: screen=%s draft=%v", a.nav.screen(), a.ng)
	}
}

// ---- Choose Storyteller and Custom Difficulty --------------------------------

func TestStorytellerPickIndependentOfDifficulty(t *testing.T) {
	t.Parallel()
	a, f, _ := ngTest(t, nil)
	click(t, a, "menu.newgame")
	click(t, a, "wizard.next")
	f.Frame(t, a)
	ng := a.ng
	click(t, a, "story.pick.hearth")
	if ng.Storyteller != "hearth" || ng.Preset != difficulty.Settled {
		t.Fatalf("picking a storyteller changed more: %s %v", ng.Storyteller, ng.Preset)
	}
	click(t, a, "diff.grim")
	if ng.Preset != difficulty.Grim || ng.Storyteller != "hearth" {
		t.Fatalf("picking a difficulty changed more: %s %v", ng.Storyteller, ng.Preset)
	}
	click(t, a, "save.commit")
	if ng.Storyteller != "hearth" || ng.Preset != difficulty.Grim || ng.SaveMode != difficulty.Commitment {
		t.Fatalf("draft = %s %v %v", ng.Storyteller, ng.Preset, ng.SaveMode)
	}
	// All three storytellers and all presets are on the screen.
	for _, p := range storyteller.Profiles() {
		if !hasWidget(a, "story.pick."+p.Key) {
			t.Errorf("no card for %s", p.Key)
		}
	}
	for _, n := range difficulty.Presets() {
		if !hasWidget(a, "diff."+n.Key) {
			t.Errorf("no row for %s", n.Key)
		}
	}
	if !hasWidget(a, "diff.custom") {
		t.Error("no Custom row")
	}
}

func TestCustomOpensEditorOnce(t *testing.T) {
	t.Parallel()
	a, f, _ := ngTest(t, nil)
	click(t, a, "menu.newgame")
	click(t, a, "wizard.next")
	f.Frame(t, a)
	ng := a.ng

	click(t, a, "diff.custom")
	f.Frame(t, a)
	if a.uiModalTop() != "diff.custom" {
		t.Fatalf("the first Custom opens the dialog, modal=%q", a.uiModalTop())
	}
	if ng.Preset != difficulty.Settled {
		t.Fatal("the preset only changes on Accept")
	}
	if ng.Cd.S != difficulty.Of(difficulty.Settled) {
		t.Fatal("the dialog starts from the selected preset")
	}
	ng.Cd.S.ThreatScale = 250
	press(t, a, f, "modal.diff.custom.accept")
	f.Frame(t, a)
	if a.uiSt().ModalOpen() || ng.Preset != difficulty.Custom || ng.Custom.ThreatScale != 250 || !ng.CustomOpen {
		t.Fatalf("accept: modal=%v preset=%v custom=%+v", a.uiSt().ModalOpen(), ng.Preset, ng.Custom)
	}
	// Later, Custom selects directly; Customize reopens.
	click(t, a, "diff.seasoned")
	click(t, a, "diff.custom")
	f.Frame(t, a)
	if a.uiSt().ModalOpen() || ng.Preset != difficulty.Custom {
		t.Fatalf("Custom opens the dialog only once: modal=%v preset=%v", a.uiSt().ModalOpen(), ng.Preset)
	}
	click(t, a, "diff.customize")
	f.Frame(t, a)
	if a.uiModalTop() != "diff.custom" || ng.Cd.S.ThreatScale != 250 {
		t.Fatalf("Customize: modal=%q cd=%+v", a.uiModalTop(), ng.Cd.S)
	}
}

func TestCustomDifficultyCancelRestores(t *testing.T) {
	t.Parallel()
	a, f, _ := ngTest(t, nil)
	click(t, a, "menu.newgame")
	click(t, a, "wizard.next")
	f.Frame(t, a)
	ng := a.ng
	before := ng.Custom
	click(t, a, "diff.custom")
	f.Frame(t, a)

	// Drag the Threat scale slider to its end.
	w, _ := a.uiSt().Find("diff.f.ts")
	track := uikit.SliderTrackRect(w.R)
	f.press(t, a, int(track.Right()), int(w.R.Y+9))
	f.letGo(t, a, int(track.Right()), int(w.R.Y+9))
	if ng.Cd.S.ThreatScale != 400 {
		t.Fatalf("dragged threat scale = %d, want 400", ng.Cd.S.ThreatScale)
	}
	press(t, a, f, "modal.diff.custom.cancel")
	f.Frame(t, a)
	if a.uiSt().ModalOpen() || ng.Custom != before || ng.Preset != difficulty.Settled || ng.CustomOpen {
		t.Fatalf("cancel must restore: modal=%v custom=%+v preset=%v", a.uiSt().ModalOpen(), ng.Custom, ng.Preset)
	}
	// Esc is a cancel too.
	click(t, a, "diff.custom")
	f.Frame(t, a)
	ng.Cd.S.CropYield = 300
	f.Key(t, a, "Escape", 0)
	if a.uiSt().ModalOpen() || ng.Custom != before {
		t.Fatalf("Esc: modal=%v custom=%+v", a.uiSt().ModalOpen(), ng.Custom)
	}
}

func TestCopyPresetFillsAllFields(t *testing.T) {
	t.Parallel()
	a, f, _ := ngTest(t, nil)
	click(t, a, "menu.newgame")
	click(t, a, "wizard.next")
	f.Frame(t, a)
	ng := a.ng
	click(t, a, "diff.custom")
	f.Frame(t, a)

	// Pick Merciless in the Start from dropdown, then Copy.
	press(t, a, f, "diff.from")
	if a.uiModalTop() != "pick.diff.from" {
		t.Fatalf("modal = %q", a.uiModalTop())
	}
	press(t, a, f, "modal.pick.diff.from.list.5")
	f.Frame(t, a)
	if ng.Cd.From != difficulty.Merciless || a.uiModalTop() != "diff.custom" {
		t.Fatalf("from=%v modal=%q", ng.Cd.From, a.uiModalTop())
	}
	press(t, a, f, "diff.copy")
	if ng.Cd.S != difficulty.Of(difficulty.Merciless) {
		t.Fatalf("copy = %+v\nwant %+v", ng.Cd.S, difficulty.Of(difficulty.Merciless))
	}
	// Reset goes back to the Start from preset after edits.
	ng.Cd.S.MoodOffset = 20
	press(t, a, f, "modal.diff.custom.reset")
	f.Frame(t, a)
	if ng.Cd.S != difficulty.Of(difficulty.Merciless) {
		t.Fatalf("reset = %+v", ng.Cd.S)
	}
}

func TestDependentFieldsDisabled(t *testing.T) {
	t.Parallel()
	a, f, _ := ngTest(t, nil)
	click(t, a, "menu.newgame")
	click(t, a, "wizard.next")
	f.Frame(t, a)
	ng := a.ng
	click(t, a, "diff.custom")
	f.Frame(t, a)

	ng.Cd.S.Adaptation, ng.Cd.S.TimeEscalation, ng.Cd.S.IgnoreWealth = false, false, true
	f.Frame(t, a)
	off := func(id string) (bool, string) {
		w, ok := a.uiSt().Find(id)
		if !ok {
			t.Fatalf("no widget %s", id)
		}
		return w.Disabled, w.Reason
	}
	if dis, _ := off("diff.f.as"); !dis {
		t.Error("adaptation strength must be disabled while adaptation is off")
	}
	if dis, why := off("diff.f.ep"); !dis || why != "Only used with time-based escalation" {
		t.Errorf("escalation per year: disabled=%v reason=%q", dis, why)
	}
	if dis, _ := off("diff.f.iw"); !dis {
		t.Error("ignore wealth needs time-based escalation")
	}
	if ng.Cd.S.IgnoreWealth {
		t.Error("turning time escalation off clears ignore wealth")
	}

	// Switch both on through the checkboxes.
	click(t, a, "diff.f.ad")
	click(t, a, "diff.f.te")
	f.Frame(t, a)
	for _, id := range []string{"diff.f.as", "diff.f.ep", "diff.f.iw"} {
		if dis, why := off(id); dis {
			t.Errorf("%s still disabled: %q", id, why)
		}
	}
	if !ng.Cd.S.Adaptation || !ng.Cd.S.TimeEscalation {
		t.Fatalf("toggles did not apply: %+v", ng.Cd.S)
	}
}

func TestSaveModeDefaultsAndPersistsInDraft(t *testing.T) {
	t.Parallel()
	var stored []config.Settings
	a, f, _ := ngTest(t, func(c *AppConfig) {
		c.Settings = options.Hooks{
			Load: func() (config.Settings, error) { return config.Defaults(), nil },
			Save: func(s config.Settings) error { stored = append(stored, s); return nil },
		}
	})
	click(t, a, "menu.newgame")
	click(t, a, "wizard.next")
	f.Frame(t, a)
	ng := a.ng
	if ng.SaveMode != difficulty.ReloadAnytime {
		t.Fatalf("default save mode = %v", ng.SaveMode)
	}
	click(t, a, "save.commit")
	click(t, a, "story.pick.gambler")
	click(t, a, "diff.merciless")
	click(t, a, "wizard.next") // writes the choices back to the settings
	if a.nav.screen() != ports.ScreenWorldGen {
		t.Fatalf("screen = %s", a.nav.screen())
	}
	if len(stored) != 1 {
		t.Fatalf("settings saved %d times, want once", len(stored))
	}
	if g := stored[0].Gameplay; g.LastStoryteller != "gambler" || g.LastDifficulty != "merciless" || g.LastSaveMode != "commitment" {
		t.Fatalf("stored = %+v", g)
	}
	// Back keeps the draft's choices.
	click(t, a, "wizard.back")
	f.Frame(t, a)
	if a.nav.screen() != ports.ScreenStoryteller || ng.SaveMode != difficulty.Commitment || ng.Storyteller != "gambler" || ng.Preset != difficulty.Merciless {
		t.Fatalf("after Back: %s %v %s %v", a.nav.screen(), ng.SaveMode, ng.Storyteller, ng.Preset)
	}
	// The next New Colony remembers them.
	a.popScreen()
	a.popScreen()
	click(t, a, "menu.newgame")
	if a.ng.Storyteller != "gambler" || a.ng.Preset != difficulty.Merciless || a.ng.SaveMode != difficulty.Commitment {
		t.Fatalf("fresh draft = %s %v %v", a.ng.Storyteller, a.ng.Preset, a.ng.SaveMode)
	}
}

func TestPacingSparklineDeterministic(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	for _, p := range storyteller.Profiles() {
		a, b := wizardpkg.PacingOf(p), wizardpkg.PacingOf(p)
		if len(a) != wizardpkg.SparkDays || fmt.Sprint(a) != fmt.Sprint(b) {
			t.Fatalf("%s: pacing differs between calls or has %d days", p.Key, len(a))
		}
		total := 0
		for _, v := range a {
			total += v
		}
		if total == 0 {
			t.Fatalf("%s: no threats in sixty days", p.Key)
		}
		seen[fmt.Sprint(a)] = true
	}
	if len(seen) != 3 {
		t.Fatalf("the three storytellers must pace differently, got %d patterns", len(seen))
	}
}

// ---- drawing and sizes ---------------------------------------------------------

func TestSetupScreensDrawAtBothSizesWithoutArt(t *testing.T) {
	t.Parallel()
	for _, size := range [][2]int{{1280, 720}, {1920, 1080}} {
		a, f, st := ngTest(t, nil)
		d, _ := scenario.Builtin("ember_clan")
		d.Builtin, d.Key, d.Name = false, "", "Drawn"
		st.putDef(t, "drawn", d)
		sizeTo(t, a, f, size[0], size[1])
		img := ebiten.NewImage(size[0], size[1])
		a.Draw(img) // the menu
		click(t, a, "menu.newgame")
		sizeTo(t, a, f, size[0], size[1])
		a.Draw(img)
		click(t, a, "scen.edit")
		f.Frame(t, a)
		a.Draw(img)
		click(t, a, "wizard.back")
		f.Frame(t, a)
		click(t, a, "wizard.next")
		f.Frame(t, a)
		a.Draw(img)
		click(t, a, "diff.custom")
		f.Frame(t, a)
		a.Draw(img)
		if a.nav.screen() != ports.ScreenStoryteller || a.uiModalTop() != "diff.custom" {
			t.Fatalf("%v: screen=%s modal=%q", size, a.nav.screen(), a.uiModalTop())
		}
	}
}

func TestNewGameScreensNeedMinWindow(t *testing.T) {
	t.Parallel()
	a, f, _ := ngTest(t, nil)
	click(t, a, "menu.newgame")
	sizeTo(t, a, f, 1000, 600)
	if hasWidget(a, "wizard.next") || hasWidget(a, "scen.row.broken_hull") {
		t.Fatal("below 1280x720 the setup screen declares no widgets")
	}
	a.Draw(ebiten.NewImage(1000, 600))
	sizeTo(t, a, f, 1280, 720)
	if !hasWidget(a, "wizard.next") {
		t.Fatal("at the minimum size the widgets are back")
	}
}

func TestCapturesOpenTheirScreens(t *testing.T) {
	t.Parallel()
	cases := []struct {
		ui     string
		screen ports.Screen
		modal  string
	}{
		{"scenario", ports.ScreenScenario, ""},
		{"scenario-editor", ports.ScreenScenarioEditor, ""},
		{"scenario-editor-addpart", ports.ScreenScenarioEditor, "sced.addpart"},
		{"storyteller", ports.ScreenStoryteller, ""},
		{"difficulty-custom", ports.ScreenStoryteller, "diff.custom"},
	}
	for _, c := range cases {
		a, f, _ := ngTest(t, nil)
		a.cfg.Capture = &Capture{Path: t.TempDir() + "/x.png", UI: c.ui}
		for i := 0; i < captureFrames+10; i++ {
			f.Frame(t, a)
		}
		if a.nav.screen() != c.screen || a.uiModalTop() != c.modal || a.renderer != nil {
			t.Errorf("%s: screen=%s modal=%q renderer=%v", c.ui, a.nav.screen(), a.uiModalTop(), a.renderer != nil)
		}
	}
}

func TestQuickStartRunsTheWholeFlowAndDropsTheDraft(t *testing.T) {
	t.Parallel()
	a, _, _ := ngTest(t, nil)
	quickStart(t, a)
	if a.ng != nil || a.renderer == nil || a.simsStarted != 1 {
		t.Fatalf("draft=%v renderer=%v sims=%d: starting ends the wizard", a.ng, a.renderer != nil, a.simsStarted)
	}
}
