package ebiten

import (
	"bytes"
	"strings"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/scenario"
	"rimworld-go/internal/platform/input"
)

// editorApp opens the Scenario Editor on a copy of the selected built-in
// (New scenario when fromNew), on a large window so every row is declared.
func editorApp(t *testing.T, fromNew bool) (*App, *fakeInput, *memStore) {
	t.Helper()
	a, f, st := ngTest(t, nil)
	click(t, a, "menu.newgame")
	sizeTo(t, a, f, 1920, 1080)
	if fromNew {
		click(t, a, "scen.new")
	} else {
		click(t, a, "scen.edit")
	}
	f.frame(t, a)
	if a.nav.screen() != screenScenarioEditor || a.ng.ed == nil {
		t.Fatalf("screen = %s, editor = %v", a.nav.screen(), a.ng.ed)
	}
	return a, f, st
}

func kinds(d scenario.Def) []scenario.PartKind {
	var out []scenario.PartKind
	for _, p := range d.Parts {
		out = append(out, p.Kind)
	}
	return out
}

func TestEditorOpensOnACopy(t *testing.T) {
	t.Parallel()
	a, f, _ := editorApp(t, false)
	ed := a.ng.ed
	if ed.def.Builtin || ed.def.Key != "" || ed.def.Name != "Broken Hull copy" {
		t.Fatalf("copy = builtin %v key %q name %q", ed.def.Builtin, ed.def.Key, ed.def.Name)
	}
	ed.def.Parts[0].Qty = 99
	orig, _ := scenario.Builtin("broken_hull")
	if orig.Parts[0].Qty == 99 || a.ng.entries[0].def.Parts[0].Qty == 99 {
		t.Fatal("editing the copy changed the original")
	}
	f.frame(t, a)

	a2, _, _ := editorApp(t, true)
	if d := a2.ng.ed.def; d.Name != "New scenario" || len(d.Parts) != 1 || d.Parts[0].Kind != scenario.PartStartItem || d.Builtin {
		t.Fatalf("new scenario = %+v", d)
	}
	if errs := scenario.HasErrors(a2.ng.ed.def.Validate()); errs {
		t.Fatalf("a new scenario must be valid: %v", a2.ng.ed.def.Validate())
	}
}

func TestEditorAddMoveRemoveParts(t *testing.T) {
	t.Parallel()
	a, f, _ := editorApp(t, true)
	ed := a.ng.ed

	press(t, a, f, "sced.add")
	if a.uiModalTop() != "sced.addpart" {
		t.Fatalf("modal = %q", a.uiModalTop())
	}
	f.frame(t, a)
	for _, k := range []string{"forced_trait", "age_range"} {
		press(t, a, f, "sced.addkind."+k)
		f.frame(t, a)
		if a.uiModalOpen() {
			t.Fatalf("choosing %s should close the dialog", k)
		}
		if k == "forced_trait" {
			press(t, a, f, "sced.add")
		}
	}
	want := []scenario.PartKind{scenario.PartStartItem, scenario.PartForcedTrait, scenario.PartAgeRange}
	if got := kinds(ed.def); len(got) != 3 || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("parts = %v, want %v", got, want)
	}
	if !ed.open[1] || !ed.open[2] {
		t.Fatalf("added parts open expanded: %v", ed.open)
	}

	// Move the last part to the top, one step at a time.
	press(t, a, f, "sced.up.2")
	f.frame(t, a)
	press(t, a, f, "sced.up.1")
	f.frame(t, a)
	if got := kinds(ed.def); got[0] != scenario.PartAgeRange || got[1] != scenario.PartStartItem || got[2] != scenario.PartForcedTrait {
		t.Fatalf("after moving up twice: %v", got)
	}
	if !ed.open[0] || ed.open[1] || !ed.open[2] {
		t.Fatalf("the open flags follow their parts: %v", ed.open)
	}
	press(t, a, f, "sced.down.0")
	f.frame(t, a)
	if got := kinds(ed.def); got[0] != scenario.PartStartItem || got[1] != scenario.PartAgeRange {
		t.Fatalf("after moving down: %v", got)
	}
	// The ends can't move further.
	if w, _ := a.uiSt().find("sced.up.0"); !w.disabled {
		t.Error("the first part can't move up")
	}
	if w, _ := a.uiSt().find("sced.down.2"); !w.disabled {
		t.Error("the last part can't move down")
	}
	// Remove the middle one.
	press(t, a, f, "sced.del.1")
	f.frame(t, a)
	if got := kinds(ed.def); len(got) != 2 || got[0] != scenario.PartStartItem || got[1] != scenario.PartForcedTrait {
		t.Fatalf("after removing: %v", got)
	}
	if !ed.open[1] || ed.open[2] {
		t.Fatalf("open flags after removal: %v", ed.open)
	}
}

func TestEditorCardsToggleAndEditValues(t *testing.T) {
	t.Parallel()
	a, f, _ := editorApp(t, true)
	ed := a.ng.ed
	press(t, a, f, "sced.card.0") // expand the starting item
	f.frame(t, a)
	if !ed.open[0] || !hasWidget(a, "sced.p.0.item") {
		t.Fatalf("expanded card shows its controls: open=%v", ed.open)
	}
	// Quantity: type digits into the field; committing clamps.
	press(t, a, f, "sced.p.0.qty")
	f.key(t, a, "A", input.ModCtrl) // select all
	f.typeText(t, a, "7")
	f.frame(t, a)
	if ed.def.Parts[0].Qty != 7 {
		t.Fatalf("typed quantity = %d, want 7", ed.def.Parts[0].Qty)
	}
	f.key(t, a, "Enter", 0)
	f.frame(t, a)
	press(t, a, f, "sced.p.0.qty")
	f.key(t, a, "A", input.ModCtrl)
	f.typeText(t, a, "99999")
	f.frame(t, a)
	f.key(t, a, "Enter", 0)
	f.frame(t, a)
	if got := ed.def.Parts[0].Qty; got != scenario.MaxQty {
		t.Fatalf("an oversized quantity is clamped on commit: %d, want %d", got, scenario.MaxQty)
	}
	ed.def.Parts[0].Qty = 0
	f.frame(t, a)
	if got := scenario.HasErrors(ed.def.Validate()); !got {
		t.Fatal("a zero quantity is an error")
	}

	// Item dropdown: pick another item.
	press(t, a, f, "sced.p.0.item")
	if a.uiModalTop() != "pick.sced.p.0.item" {
		t.Fatalf("modal = %q", a.uiModalTop())
	}
	f.frame(t, a)
	idx := indexWhere(len(edItems), func(i int) bool { return edItems[i] == item.Steel })
	// Scroll the list so the row is declared, then click it.
	a.uiSt().scroll["modal.pick.sced.p.0.item.list.scroll"] = 0
	id := "modal.pick.sced.p.0.item.list." + uiNum(idx)
	for i := 0; i < 60 && !hasWidget(a, id); i++ {
		a.uiSt().scroll["modal.pick.sced.p.0.item.list"] += 3 * listRowH
		f.frame(t, a)
	}
	press(t, a, f, id)
	f.frame(t, a)
	if ed.def.Parts[0].Item != item.Steel || a.uiModalOpen() {
		t.Fatalf("item = %v modal=%v", ed.def.Parts[0].Item, a.uiModalOpen())
	}
	press(t, a, f, "sced.card.0") // collapse again
	f.frame(t, a)
	if ed.open[0] || hasWidget(a, "sced.p.0.item") {
		t.Fatal("collapsed card hides its controls")
	}
}

func TestEditorMandatoryFields(t *testing.T) {
	t.Parallel()
	a, f, _ := editorApp(t, true)
	d := &a.ng.ed.def
	press(t, a, f, "sced.faction.1")
	press(t, a, f, "sced.arrival.2")
	press(t, a, f, "sced.tech.0")
	if d.Faction != faction.Clans || d.Arrival != scenario.WalkIn || d.Tech != faction.Neolithic {
		t.Fatalf("segments: faction=%v arrival=%v tech=%v", d.Faction, d.Arrival, d.Tech)
	}
	// People and the pool size move together and stay within +8.
	d.People, d.ChooseFrom = 3, 8
	f.frame(t, a)
	press(t, a, f, "sced.people.inc")
	press(t, a, f, "sced.people.inc")
	if d.People != 5 || d.ChooseFrom != 10 {
		t.Fatalf("people=%d chooseFrom=%d, want 5 and 10", d.People, d.ChooseFrom)
	}
	press(t, a, f, "sced.choose.inc")
	press(t, a, f, "sced.choose.inc")
	press(t, a, f, "sced.choose.inc")
	if d.ChooseFrom != 13 {
		t.Fatalf("chooseFrom = %d, want the max 13", d.ChooseFrom)
	}
	for i := 0; i < 12; i++ {
		press(t, a, f, "sced.people.dec")
	}
	if d.People != 1 || d.ChooseFrom < 1 || d.ChooseFrom > 9 {
		t.Fatalf("people=%d chooseFrom=%d", d.People, d.ChooseFrom)
	}
}

func TestEditorValidationLinksToPart(t *testing.T) {
	t.Parallel()
	a, f, _ := editorApp(t, true)
	a.sw, a.sh = 1280, 720 // a small window: the bad part is below the fold
	ed := a.ng.ed
	for i := 0; i < 11; i++ {
		ed.def.Parts = append(ed.def.Parts, scenario.NewPart(scenario.PartNoClothes))
	}
	bad := scenario.NewPart(scenario.PartForcedTrait)
	bad.Chance = 0 // below the 5% minimum
	ed.def.Parts = append(ed.def.Parts, bad)
	last := len(ed.def.Parts) - 1
	f.frame(t, a)
	f.frame(t, a)

	probs := ed.def.Validate()
	if len(probs) != 1 || probs[0].Part != last || probs[0].Severity != scenario.Error {
		t.Fatalf("problems = %+v, want one error on part %d", probs, last)
	}
	if hasWidget(a, "sced.card."+uiNum(last)) {
		t.Fatal("the bad part should start below the visible form")
	}
	if ed.open[last] {
		t.Fatal("the part starts collapsed")
	}
	// Until it is fixed, Done is blocked by the error and says how many.
	w, _ := a.uiSt().find("sced.done")
	if !w.disabled || w.reason != "Fix 1 problem" {
		t.Fatalf("Done: disabled=%v reason=%q", w.disabled, w.reason)
	}
	press(t, a, f, "sced.prob.0")
	f.frame(t, a)
	f.frame(t, a)
	if !ed.open[last] {
		t.Fatal("clicking a problem expands its part")
	}
	if a.uiSt().scroll["sced.form"] <= 0 {
		t.Fatal("clicking a problem scrolls to its part")
	}
	if !hasWidget(a, "sced.card."+uiNum(last)) || !hasWidget(a, "sced.p."+uiNum(last)+".chance") {
		t.Fatal("the part and its controls are in view")
	}
}

func TestEditorDoneBlockedByErrorsNotWarnings(t *testing.T) {
	t.Parallel()
	a, f, _ := editorApp(t, true)
	ed := a.ng.ed
	// Remove the only food item: a warning (the kit has no food), not an error.
	press(t, a, f, "sced.del.0")
	f.frame(t, a)
	probs := ed.def.Validate()
	if len(probs) != 1 || probs[0].Severity != scenario.Warning {
		t.Fatalf("problems = %+v, want one warning", probs)
	}
	w, _ := a.uiSt().find("sced.done")
	if w.disabled {
		t.Fatalf("warnings must not block Done: %q", w.reason)
	}
	// An error blocks it.
	ed.def.Name = ""
	f.frame(t, a)
	w, _ = a.uiSt().find("sced.done")
	if !w.disabled || !strings.HasPrefix(w.reason, "Fix ") {
		t.Fatalf("an error must block Done: disabled=%v %q", w.disabled, w.reason)
	}
	click(t, a, "sced.done")
	if a.nav.screen() != screenScenarioEditor {
		t.Fatalf("Done with errors left the editor: %s", a.nav.screen())
	}
	ed.def.Name = "Fresh start"
	f.frame(t, a)
	click(t, a, "sced.done")
	if a.nav.screen() != screenScenario || a.ng.ed != nil {
		t.Fatalf("Done = %s, editor %v", a.nav.screen(), a.ng.ed)
	}
	if len(a.ngUnsaved) != 1 || a.ngUnsaved[0].Name != "Fresh start" {
		t.Fatalf("unsaved = %+v", a.ngUnsaved)
	}
	sel := a.ng.selected()
	if sel == nil || !sel.unsaved || sel.name != "Fresh start (unsaved)" {
		t.Fatalf("selected = %+v, want the edited scenario, marked unsaved", sel)
	}
	// Editing the unsaved entry replaces it instead of adding another.
	f.frame(t, a)
	click(t, a, "scen.edit")
	f.frame(t, a)
	a.ng.ed.def.Summary = "changed"
	f.frame(t, a)
	click(t, a, "sced.done")
	if len(a.ngUnsaved) != 1 || a.ngUnsaved[0].Summary != "changed" {
		t.Fatalf("unsaved = %+v", a.ngUnsaved)
	}
	// And it goes forward from Choose Scenario.
	f.frame(t, a)
	click(t, a, "wizard.next")
	if a.nav.screen() != screenStoryteller || a.ng.scenario.Name != "Fresh start" {
		t.Fatalf("screen=%s scenario=%q", a.nav.screen(), a.ng.scenario.Name)
	}
}

func TestEditorSaveLoadRoundTrip(t *testing.T) {
	t.Parallel()
	a, f, st := editorApp(t, true)
	ed := a.ng.ed
	ed.def.Name = "My Test: Camp!"
	ed.def.Summary = "round trip"
	ed.def.Parts = append(ed.def.Parts, scenario.NewPart(scenario.PartForcedTrait), scenario.NewPart(scenario.PartScheduledIncident))
	ed.def.Desc = "line one\nline two"
	f.frame(t, a)

	press(t, a, f, "sced.save")
	f.frame(t, a)
	data, ok := st.files["my-test-camp"]
	if !ok || ed.savedAs != "my-test-camp" || !bytes.Equal(ed.saved, data) {
		t.Fatalf("saved files = %v, savedAs = %q", fileNames(st.files), ed.savedAs)
	}
	got, err := scenario.Decode(data)
	if err != nil || got.Name != "My Test: Camp!" || len(got.Parts) != 3 || got.Parts[1].Kind != scenario.PartForcedTrait {
		t.Fatalf("decoded = %+v, %v", got, err)
	}
	if a.hint != "saved my-test-camp" {
		t.Errorf("hint = %q", a.hint)
	}
	if ed.dirty() {
		t.Fatal("a saved scenario is not dirty")
	}

	// Edit, then Load brings the saved one back.
	ed.def.Summary = "scribbled"
	f.frame(t, a)
	press(t, a, f, "sced.load")
	if a.uiModalTop() != "sced.loadfile" {
		t.Fatalf("modal = %q", a.uiModalTop())
	}
	f.frame(t, a)
	press(t, a, f, "modal.sced.load.table.body.0")
	f.frame(t, a)
	if a.uiModalOpen() || ed.def.Summary != "round trip" || ed.def.Desc != "line one\nline two" {
		t.Fatalf("modal=%v summary=%q", a.uiModalOpen(), ed.def.Summary)
	}
	// Saving the same name again from this editor needs no question.
	press(t, a, f, "sced.save")
	f.frame(t, a)
	if a.uiModalOpen() {
		t.Fatal("re-saving a file this editor wrote asks nothing")
	}

	// A different editor with the same name asks before it overwrites.
	f.frame(t, a)
	click(t, a, "wizard.back")
	f.frame(t, a)
	click(t, a, "scen.new")
	f.frame(t, a)
	b := a.ng.ed
	b.def.Name = "my test camp"
	f.frame(t, a)
	press(t, a, f, "sced.save")
	if a.uiModalTop() != "sced.overwrite" {
		t.Fatalf("modal = %q, want the overwrite question", a.uiModalTop())
	}
	before := append([]byte(nil), st.files["my-test-camp"]...)
	press(t, a, f, "modal.sced.overwrite.cancel")
	f.frame(t, a)
	if !bytes.Equal(st.files["my-test-camp"], before) {
		t.Fatal("Cancel must leave the file alone")
	}
	press(t, a, f, "sced.save")
	press(t, a, f, "modal.sced.overwrite.ok")
	f.frame(t, a)
	if bytes.Equal(st.files["my-test-camp"], before) {
		t.Fatal("OK must overwrite")
	}
	// Done after saving selects the file in the list.
	click(t, a, "sced.done")
	if sel := a.ng.selected(); a.nav.screen() != screenScenario || sel == nil || sel.key != "file.my-test-camp" || sel.unsaved {
		t.Fatalf("screen=%s selected=%+v", a.nav.screen(), sel)
	}
	if len(a.ngUnsaved) != 0 {
		t.Fatalf("a saved scenario is not an unsaved one: %v", a.ngUnsaved)
	}
}

func fileNames(m map[string][]byte) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestEditorSaveNeedsAStoreAndAValidScenario(t *testing.T) {
	t.Parallel()
	a, f, st := editorApp(t, true)
	ed := a.ng.ed
	ed.def.Name = ""
	f.frame(t, a)
	w, _ := a.uiSt().find("sced.save")
	if !w.disabled || !strings.Contains(w.reason, "Fix 1 problem") {
		t.Fatalf("save with errors: %v %q", w.disabled, w.reason)
	}
	ed.def.Name = "Disk test"
	st.saveErr = errTest
	f.frame(t, a)
	click(t, a, "sced.save")
	if !strings.Contains(a.hint, "save failed") || ed.saved != nil {
		t.Fatalf("hint = %q saved=%v", a.hint, ed.saved != nil)
	}
	a.cfg.Scenarios = nil
	f.frame(t, a)
	for _, id := range []string{"sced.save", "sced.load"} {
		if w, _ := a.uiSt().find(id); !w.disabled {
			t.Errorf("%s needs a scenario store", id)
		}
	}
}

var errTest = &testErr{}

type testErr struct{}

func (*testErr) Error() string { return "disk full" }

func TestEditorLoadShowsErrorsOfBrokenFiles(t *testing.T) {
	t.Parallel()
	a, f, st := editorApp(t, true)
	st.files["broken"] = []byte(`{"version":9}`)
	press(t, a, f, "sced.load")
	f.frame(t, a)
	press(t, a, f, "modal.sced.load.table.body.0")
	f.frame(t, a)
	if !a.uiModalOpen() || !strings.Contains(a.ng.ed.loadErr, "broken") {
		t.Fatalf("a file that can't be read keeps the dialog open with the reason: modal=%v err=%q", a.uiModalOpen(), a.ng.ed.loadErr)
	}
	a.Draw(ebiten.NewImage(a.sw, a.sh))
}

func TestEditorResetAndSeed(t *testing.T) {
	t.Parallel()
	a, f, _ := editorApp(t, false)
	a.wiz.seed, a.wiz.seedText = 777, "777"
	worldBefore := a.ng.world
	ed := a.ng.ed
	seed := ed.def.Seed
	press(t, a, f, "sced.seed")
	if ed.def.Seed == seed {
		t.Fatal("Randomize seed must change the scenario's seed")
	}
	if a.wiz.seed != 777 || a.wiz.seedText != "777" || a.ng.world.SeedText != worldBefore.SeedText {
		t.Fatalf("the planet seed changed: wiz=%d world=%q", a.wiz.seed, a.ng.world.SeedText)
	}
	ed.def.Name = "Scribble"
	ed.def.Parts = nil
	f.frame(t, a)
	press(t, a, f, "sced.reset")
	if ed.def.Name != "Broken Hull copy" || len(ed.def.Parts) == 0 || ed.def.Seed != seed {
		t.Fatalf("reset = %q, %d parts, seed %d", ed.def.Name, len(ed.def.Parts), ed.def.Seed)
	}
}

func TestEditorBackAsksOnlyWhenDirty(t *testing.T) {
	t.Parallel()
	a, f, _ := editorApp(t, false)
	click(t, a, "wizard.back")
	if a.nav.screen() != screenScenario || a.uiModalOpen() {
		t.Fatalf("an untouched editor leaves at once: %s", a.nav.screen())
	}
	f.frame(t, a)
	click(t, a, "scen.edit")
	f.frame(t, a)
	a.ng.ed.def.Name = "Edited"
	f.frame(t, a)
	click(t, a, "wizard.back")
	f.frame(t, a)
	if a.nav.screen() != screenScenarioEditor || a.uiModalTop() != "sced.discard" {
		t.Fatalf("screen=%s modal=%q, want Discard changes?", a.nav.screen(), a.uiModalTop())
	}
	press(t, a, f, "modal.sced.discard.cancel")
	f.frame(t, a)
	if a.nav.screen() != screenScenarioEditor {
		t.Fatal("Cancel stays in the editor")
	}
	f.key(t, a, "Escape", 0) // Esc asks too
	if a.uiModalTop() != "sced.discard" {
		t.Fatalf("Esc: modal=%q", a.uiModalTop())
	}
	press(t, a, f, "modal.sced.discard.ok")
	f.frame(t, a)
	if a.nav.screen() != screenScenario || a.ng.ed != nil || len(a.ngUnsaved) != 0 {
		t.Fatalf("Discard: screen=%s ed=%v unsaved=%d", a.nav.screen(), a.ng.ed, len(a.ngUnsaved))
	}
}

// Every part kind lays out, expands and draws at both window sizes.
func TestEditorEveryPartKindDraws(t *testing.T) {
	t.Parallel()
	a, f, _ := editorApp(t, true)
	ed := a.ng.ed
	ed.def.Parts = nil
	for _, c := range scenario.Catalog() {
		for _, k := range c.Kinds {
			p := scenario.NewPart(k.Kind)
			ed.def.Parts = append(ed.def.Parts, p)
			ed.open[len(ed.def.Parts)-1] = true
		}
	}
	for _, size := range [][2]int{{1920, 1080}, {1280, 720}} {
		sizeTo(t, a, f, size[0], size[1])
		for off := float32(0); off < 3000; off += 400 {
			a.uiSt().scroll["sced.form"] = off
			f.frame(t, a)
			a.Draw(ebiten.NewImage(size[0], size[1]))
		}
	}
	// At 1080p, with the form scrolled to the top, the first controls exist.
	sizeTo(t, a, f, 1920, 1080)
	a.uiSt().scroll["sced.form"] = 0
	f.frame(t, a)
	if !hasWidget(a, "sced.p.0.trait") || !hasWidget(a, "sced.p.0.chance") {
		t.Fatal("the forced trait controls are declared")
	}
	// A condition part with a missing kind is reported, not drawn as garbage.
	ed.def.Parts = append(ed.def.Parts, scenario.Part{Kind: scenario.PartNone})
	f.frame(t, a)
	if len(ed.def.Validate()) == 0 {
		t.Fatal("an unknown part kind is a validation problem")
	}
}

func TestEditorHelpers(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"Broken Hull":           "broken-hull",
		"  --Ünï  cødé!! ":      "n-c-d",
		"":                      "scenario",
		"???":                   "scenario",
		strings.Repeat("a", 90): strings.Repeat("a", 40),
	} {
		if got := slugOf(in); got != want {
			t.Errorf("slugOf(%q) = %q, want %q", in, got, want)
		}
	}
	d, _ := scenario.Builtin("gilded_expedition")
	c := copyForEditing(d)
	if c.Name != "Gilded Expedition copy" || c.Builtin || c.Key != "" {
		t.Fatalf("copy = %+v", c)
	}
	long := cloneDef(d)
	long.Name = strings.Repeat("x", scenario.MaxName)
	long.Builtin = true
	if n := len([]rune(copyForEditing(long).Name)); n != scenario.MaxName {
		t.Fatalf("a long copied name is %d runes, want the limit %d", n, scenario.MaxName)
	}
	custom := cloneDef(d)
	custom.Builtin, custom.Name = false, "Mine"
	if copyForEditing(custom).Name != "Mine" {
		t.Fatal("a custom scenario keeps its name when copied")
	}
	if prettyKey("industrial_medicine") != "Industrial medicine" || prettyKey("") != "" {
		t.Fatal("prettyKey")
	}
}
