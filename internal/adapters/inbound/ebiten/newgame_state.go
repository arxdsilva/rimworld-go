package ebiten

import (
	"fmt"
	"image/color"
	"sort"
	"strconv"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/difficulty"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/scenario"
	"rimworld-go/internal/domain/storyteller"
	"rimworld-go/internal/domain/world/planet"
	"rimworld-go/internal/platform/input"
)

// The New Colony wizard's draft (NG-1 §1.6) and the pieces the first
// setup screens share: the draft itself, the scenario list, the footer
// buttons and a few form helpers. Each screen's Next writes its part into
// the draft; the draft is only turned into a game when Start is pressed.

// Defaults of a fresh draft (NG-1 §3.10).
const (
	defaultStoryteller = "steady"
	defaultScenarioKey = "broken_hull"
	// defaultMapSide is the map size a fresh draft asks for, capped to what
	// the engine gate allows today.
	defaultMapSide = 250
)

const defaultPreset = difficulty.Settled

// defaultCoverage is the globe coverage of a fresh draft (percent).
const defaultCoverage = 30

// scenEntry is one row source of the Choose Scenario list: a built-in, a
// custom file or an unsaved scenario from this session.
type scenEntry struct {
	key, name, summary string
	def                scenario.Def
	builtin, unsaved   bool
	file               string // the store file name of a saved custom scenario
	err                string // why a file can't be read
	modified           string
	problems           []scenario.Problem
	hardness           int
	lines              []game.DetailLine
}

// deletable entries are custom ones (a file or an unsaved copy).
func (e scenEntry) deletable() bool { return !e.builtin }

// errCount is the number of error-severity problems.
func (e scenEntry) errCount() int {
	n := 0
	for _, p := range e.problems {
		if p.Severity == scenario.Error {
			n++
		}
	}
	return n
}

// newGameState is the wizard draft: everything chosen so far.
type newGameState struct {
	entries     []scenEntry
	sel         int          // index into entries
	scenario    scenario.Def // working copy, stamped when Next is pressed on Choose Scenario
	search      string
	storyteller string // profile key
	preset      difficulty.Preset
	custom      difficulty.Settings // the Custom preset's settings
	customOpen  bool                // the Custom Difficulty dialog has been shown once
	saveMode    difficulty.SaveMode

	// Create World, Landing and Characters (screen_worldgen.go,
	// screen_landing.go, screen_pawns.go).
	world   planet.Params
	tile    int // selected landing tile, -1 none
	season  int // -1 Auto, else a season index (0 Spring .. 3 Winter)
	mapSize int
	err     string

	planet    *planet.Planet  // the generated world, kept across Back
	planetKey string          // Params.Key() the planet was built from
	builder   *planet.Builder // in-flight generation
	genStep   int             // work units stepped per frame
	genPhase  planet.Phase
	genFrac   float32
	genErr    string  // why the last generation failed
	genHold   float32 // captures: stop stepping at this progress
	globe     *globeView
	globeOf   *planet.Planet
	landRolls int  // presses of "Select random site"
	outside   bool // the last click was on the uncovered part of the globe
	legend    bool // the biome legend is open
	facSel    int  // highlighted row of the Factions table
	info      infoCache

	pool    candidatePool // the character candidates
	poolKey string        // what the pool was made from
	pawnSel int           // highlighted roster position
	names   [3]string     // first, nick, last drafts of the highlighted candidate
	namesOf int           // the roster position the drafts belong to (-1 none)
	nameErr string
	drag    rosterDrag
	load    *loadState // the colony is being built
	// buildHook replaces the one-step builder (tests).
	buildHook func(NewGame) (SimBuilder, error)

	// initial is the signature of the defaults, to tell whether a later step
	// was changed.
	initial string

	// ed is the open Scenario Editor.
	ed *editorState
	// cd is the Custom Difficulty dialog's working copy.
	cd customDiff
	// bufs holds the text of numeric fields; the text field edits it in
	// place between frames, so it lives here and not on the stack.
	bufs map[string]*string
	// formH remembers each scroll form's content height of the last frame.
	formH map[string]float32
}

// signature is a comparable summary of the steps after Choose Scenario.
func (ng *newGameState) signature() string {
	return fmt.Sprint(ng.storyteller, ng.preset, ng.custom, ng.saveMode, ng.world, ng.tile, ng.season, ng.mapSize)
}

// changedLater reports a step after Choose Scenario differs from its default.
func (ng *newGameState) changedLater() bool { return ng.signature() != ng.initial }

// selected is the highlighted scenario entry.
func (ng *newGameState) selected() *scenEntry {
	if ng.sel < 0 || ng.sel >= len(ng.entries) {
		return nil
	}
	return &ng.entries[ng.sel]
}

// selectKey highlights the entry with key (the first one when it is gone).
func (ng *newGameState) selectKey(key string) {
	for i, e := range ng.entries {
		if e.key == key {
			ng.sel = i
			return
		}
	}
	ng.sel = 0
}

func (ng *newGameState) selectedKey() string {
	if e := ng.selected(); e != nil {
		return e.key
	}
	return defaultScenarioKey
}

// draft is the wizard's state, created on first use.
func (a *App) draft() *newGameState {
	if a.ng == nil {
		a.ng = a.newDraft()
	}
	return a.ng
}

// newDraft is a fresh draft with the defaults, or the last choices stored
// in the settings.
func (a *App) newDraft() *newGameState {
	g := a.settings.Gameplay
	seedText := randomWorldPhrase()
	if a.cfg.DefaultSeed != 0 {
		seedText = strconv.FormatUint(a.cfg.DefaultSeed, 10)
	}
	ng := &newGameState{
		storyteller: defaultStoryteller,
		preset:      defaultPreset,
		saveMode:    difficulty.ReloadAnytime,
		tile:        -1,
		season:      -1,
		mapSize:     defaultMapSideEnabled(),
		namesOf:     -1,
		world: planet.Params{SeedText: seedText, Coverage: defaultCoverage, Rain: 2, Temp: 2, Pop: 1,
			Factions: faction.DefaultSlots()},
	}
	if p, ok := storyteller.ByKey(g.LastStoryteller); ok && !p.Hidden {
		ng.storyteller = p.Key
	}
	if p, ok := difficulty.Parse(g.LastDifficulty); ok && p != difficulty.Custom {
		ng.preset = p
	}
	if g.LastSaveMode == difficulty.Commitment.String() {
		ng.saveMode = difficulty.Commitment
	}
	ng.custom = difficulty.Of(ng.preset)
	ng.refreshScenarios(a)
	ng.selectKey(defaultScenarioKey)
	ng.initial = ng.signature()
	return ng
}

// newEntry is a list entry for a readable definition.
func newEntry(d scenario.Def) scenEntry {
	return scenEntry{
		key: d.Key, name: d.Name, summary: d.Summary, def: d, builtin: d.Builtin,
		problems: d.Validate(), hardness: d.HardnessPips(), lines: scenarioLines(d),
	}
}

// refreshScenarios rebuilds the list: the built-ins, the files of the
// store and this session's unsaved scenarios. It keeps the selection.
func (ng *newGameState) refreshScenarios(a *App) {
	prev := ng.selectedKey()
	entries := make([]scenEntry, 0, 8)
	for _, d := range scenario.Builtins() {
		entries = append(entries, newEntry(d))
	}
	var custom []scenEntry
	if st := a.cfg.Scenarios; st != nil {
		files, err := st.List()
		if err != nil {
			ng.err = "scenarios: " + err.Error()
		}
		for _, f := range files {
			e := scenEntry{key: "file." + f.Name, name: f.Name, file: f.Name, modified: f.Modified}
			data, err := st.Load(f.Name)
			var d scenario.Def
			if err == nil {
				d, err = scenario.Decode(data)
			}
			if err != nil {
				e.err = err.Error()
				e.summary = "Can't read this file"
				custom = append(custom, e)
				continue
			}
			d.Builtin = false
			ne := newEntry(d)
			ne.key, ne.file, ne.modified = e.key, f.Name, f.Modified
			if strings.TrimSpace(ne.name) == "" {
				ne.name = f.Name
			}
			custom = append(custom, ne)
		}
	}
	for i, d := range a.ngUnsaved {
		d.Builtin = false
		e := newEntry(d)
		e.key = "unsaved." + strconv.Itoa(i)
		e.unsaved = true
		e.name = d.Name + " (unsaved)"
		custom = append(custom, e)
	}
	sort.SliceStable(custom, func(i, j int) bool {
		return strings.ToLower(custom[i].name) < strings.ToLower(custom[j].name)
	})
	ng.entries = append(entries, custom...)
	ng.selectKey(prev)
}

// ---- navigation ------------------------------------------------------

// goBack is the Back button and Escape on a setup screen: it pops one
// screen, asking first when that would throw work away (NG-1 §1.5).
func (a *App) goBack() {
	if ng := a.ng; ng != nil && ng.load != nil {
		return // the colony is being built: nothing to go back to
	}
	switch a.nav.screen() {
	case screenScenarioEditor:
		if ng := a.ng; ng != nil && ng.ed != nil && ng.ed.dirty() {
			a.uiConfirm("sced.discard", "Discard changes?", "The scenario has changes that were not saved or kept. Leave the editor and lose them?", "Discard", true, a.popScreen)
			return
		}
	case screenScenario:
		if ng := a.ng; ng != nil && ng.changedLater() {
			a.uiConfirm("scen.discard", "Discard this new colony setup?", "You changed later steps of this setup. Going back to the main menu throws those choices away.", "Discard", true, a.popScreen)
			return
		}
	}
	a.popScreen()
}

// popScreen pops one screen and tidies the draft: the editor's state goes
// with the editor, and the main menu forgets the whole draft.
func (a *App) popScreen() {
	a.uiSt().modals = nil
	was := a.nav.screen()
	a.nav.pop()
	switch {
	case was == screenScenarioEditor && a.ng != nil:
		a.ng.ed = nil
	case a.nav.screen() == screenMainMenu:
		a.ng = nil
	}
}

// forward pushes the next wizard screen.
func (a *App) forward() { a.apply(intent{kind: intentPush, screen: wizardNext(a.nav.screen())}) }

// ngClick runs a new-game widget id from the legacy click path (captures
// and tests that click by id); widgets normally handle their own clicks.
// It reports the id was one of these screens'.
func (a *App) ngClick(id string) bool {
	switch {
	case id == "menu.newgame":
		a.ng = nil
		a.draft()
		a.apply(intent{kind: intentPush, screen: screenScenario})
	case strings.HasPrefix(id, "scen."), strings.HasPrefix(id, "sced."),
		strings.HasPrefix(id, "story."), strings.HasPrefix(id, "diff."), strings.HasPrefix(id, "save."),
		strings.HasPrefix(id, "world."), strings.HasPrefix(id, "land."), strings.HasPrefix(id, "pawns."),
		strings.HasPrefix(id, "fac."), strings.HasPrefix(id, "wizard."):
		// Kit widgets answer these through uiClick; a stale id does nothing.
	default:
		return false
	}
	return true
}

// ---- shared widgets ----------------------------------------------------

// ngButton is a kit button whose click runs fn at once (also when the
// click comes from the id-driven click path of tests and captures), not on
// the next frame.
func (a *App) ngButton(id string, r rect, label string, o btnOpts, fn func()) {
	u := a.uiSt()
	pressed := a.uiButton(id, r, label, o)
	if n := len(u.cur); n > 0 && u.cur[n-1].id == id {
		u.cur[n-1].click = func() {
			if o.Disabled {
				a.uiDeny(o.Reason)
				return
			}
			a.uiOK()
			fn()
		}
	}
	if pressed {
		fn()
	}
}

// ngFooter declares the Back button and the forward button of a setup
// screen. Enter presses the forward button when it is enabled.
func (a *App) ngFooter(footer rect, label, nextID string, o btnOpts, onNext func()) {
	a.ngButton("wizard.back", footerBack(footer), "Back", btnOpts{}, a.goBack)
	if o.Style == btnNormal {
		o.Style = btnPrimary
	}
	a.ngButton(nextID, footerNext(footer), label, o, onNext)
	if a.in.fired(input.Accept) && !o.Disabled && !a.uiModalOpen() && !a.uiTextEditing() {
		onNext()
	}
	if o.Disabled && o.Reason != "" {
		a.uiLabel(footerBack(footer).right()+16, footer.Y+(footer.H-float32(lineH(tsBody)))/2, o.Reason, dimText())
	}
}

// dimText is the secondary body style.
func dimText() textStyle {
	st := tsBody
	st.Color = uiTextDim
	return st
}

// drawNGFrame paints the backdrop, the dialog frame and its title; the
// widgets of the screen's hook go on top. A window below the minimum gets
// the "too small" note instead.
func (a *App) drawNGFrame(screen *ebiten.Image, sw, sh int, title string) {
	var p *planet.Planet
	if a.ng != nil {
		p = a.ng.planet
	}
	a.drawSetupBackdrop(screen, sw, sh, p)
	if tooSmall(sw, sh) {
		drawTooSmall(screen, sw, sh)
		return
	}
	frame, titleR, _, footer := dialogFrame(sw, sh)
	drawPanel(screen, frame.X, frame.Y, frame.W, frame.H)
	st := tsTitle
	st.Color = uiHeading
	drawText(screen, title, float64(titleR.X+dialogPad), float64(titleR.Y)+(float64(titleR.H)-lineH(st))/2, st)
	line := uiPanelBorder
	fillRect(screen, rect{frame.X, titleR.bottom() - 1, frame.W, 1}, line)
	fillRect(screen, rect{frame.X, footer.Y, frame.W, 1}, line)
}

// formCtx lays a vertical form out inside a scroll view: row hands out
// the next row of the content, shifted by the scroll offset; a row that is
// not fully inside the view reports false and the caller declares no
// widget for it.
type formCtx struct {
	view rect
	x, w float32
	y    float32 // content height so far
	off  float32
}

// newForm starts a form in the view of a scroll area (padding 4 px).
func newForm(area scrollArea) *formCtx {
	return &formCtx{view: area.View, x: area.View.X + 4, w: max(0, area.View.W-8), off: area.Off}
}

// row is the next row of height h.
func (f *formCtx) row(h float32) (rect, bool) {
	r := rect{f.x, f.view.Y + f.y - f.off, f.w, h}
	f.y += h
	return r, r.Y >= f.view.Y-0.5 && r.bottom() <= f.view.bottom()+0.5
}

// gap leaves h px empty.
func (f *formCtx) gap(h float32) { f.y += h }

// top is the screen y of the next row.
func (f *formCtx) top() float32 { return f.view.Y + f.y - f.off }

// content is the content height so far (for the scroll area next frame).
func (f *formCtx) content() float32 { return f.y + 4 }

// formHeight is the content height remembered for the form id.
func (ng *newGameState) formHeight(id string) float32 { return ng.formH[id] }

func (ng *newGameState) setFormHeight(id string, h float32) {
	if ng.formH == nil {
		ng.formH = map[string]float32{}
	}
	ng.formH[id] = h
}

// rowLabel puts a dim label at the left of a form row and returns the
// rectangle left for the control.
func (a *App) rowLabel(r rect, label string, labelW float32) rect {
	st := dimText()
	a.uiLabel(r.X, r.Y+(r.H-float32(lineH(st)))/2, fitText(label, st, labelW-8), st)
	return rect{r.X + labelW, r.Y, max(0, r.W-labelW), r.H}
}

// ngChoice is a dropdown: a button showing the current option that opens
// a modal list. pick receives the chosen option. A reason disables it.
func (a *App) ngChoice(id string, r rect, title string, opts []string, cur int, disabled string, pick func(i int)) {
	label := "-"
	if cur >= 0 && cur < len(opts) {
		label = opts[cur]
	}
	a.ngButton(id, r, label+"  v", btnOpts{Disabled: disabled != "", Reason: disabled}, func() {
		a.pushPicker(id, title, opts, cur, pick)
	})
}

// pushPicker opens the option list of a dropdown.
func (a *App) pushPicker(id, title string, opts []string, cur int, pick func(i int)) {
	h := float32(modalTitleH+2*modalPad) + min(float32(len(opts)), 12)*listRowH + 4
	mid := "pick." + id
	a.uiPushModal(modal{
		id: mid, title: title, w: 380, h: h,
		draw: func(a *App, body rect) {
			choice := -1
			changed := a.uiList("modal."+mid+".list", body, len(opts), &choice, listSpec{
				Draw: func(dst *ebiten.Image, i int, row rect, hover, selected bool) {
					drawListRow(dst, row, opts[i], hover, i == cur, false)
				},
			})
			if changed && choice >= 0 {
				a.uiCloseModal(mid)
				pick(choice)
			}
		},
	})
}

// ngStepper is "- value +" in r over *v, within [lo, hi].
func (a *App) ngStepper(id string, r rect, v *int, lo, hi int) {
	*v = max(lo, min(hi, *v))
	bw := float32(28)
	a.ngButton(id+".dec", rect{r.X, r.Y, bw, r.H}, "-", btnOpts{Disabled: *v <= lo, Reason: fmt.Sprintf("The minimum is %d.", lo)}, func() { *v = max(lo, *v-1) })
	a.ngButton(id+".inc", rect{r.X + bw + 56, r.Y, bw, r.H}, "+", btnOpts{Disabled: *v >= hi, Reason: fmt.Sprintf("The maximum is %d.", hi)}, func() { *v = min(hi, *v+1) })
	val := *v
	a.uiSt().queue(func(dst *ebiten.Image) {
		st := tsLabel
		st.Align = alignCenter
		drawText(dst, strconv.Itoa(val), float64(r.X+bw+28), float64(r.Y)+(float64(r.H)-lineH(st))/2, st)
	})
}

// ngNum is a digits-only text field over *v. The value follows what is
// typed (validation shows out-of-range numbers); committing clamps it to
// [lo, hi].
func (a *App) ngNum(id string, r rect, v *int, lo, hi int) {
	ng := a.draft()
	if ng.bufs == nil {
		ng.bufs = map[string]*string{}
	}
	buf := ng.bufs[id]
	if buf == nil {
		buf = new(string)
		ng.bufs[id] = buf
	}
	if a.uiSt().edit.id != id { // not being typed into: follow the model
		*buf = strconv.Itoa(*v)
	}
	changed, committed := a.uiTextField(id, r, buf, textFieldSpec{MaxLen: 5, Valid: digitsOnly, Tip: fmt.Sprintf("%d to %d", lo, hi)})
	if changed || committed {
		if n, err := strconv.Atoi(*buf); err == nil {
			*v = n
		}
	}
	if committed {
		*v = max(lo, min(hi, *v))
		*buf = strconv.Itoa(*v)
	}
}

// segmented is a row of mutually exclusive buttons over *sel.
func (a *App) segmented(id string, r rect, labels []string, sel *int, tips []string) {
	n := len(labels)
	if n == 0 {
		return
	}
	w := (r.W - float32(n-1)*4) / float32(n)
	for i, l := range labels {
		i := i
		o := btnOpts{Selected: *sel == i}
		if i < len(tips) {
			o.Tip = tips[i]
		}
		a.ngButton(optID(id, i), rect{r.X + float32(i)*(w+4), r.Y, w, r.H}, l, o, func() { *sel = i })
	}
}

// textLines draws s wrapped to w px from (x, y) in st and returns the
// height used.
func (a *App) textLines(x, y, w float32, s string, st textStyle) float32 {
	h := float32(0)
	for _, l := range wrapPx(s, st, float64(w)) {
		a.uiLabel(x, y+h, l, st)
		h += float32(lineH(st))
	}
	return h
}

// wrapHeight is the height wrapPx would use for s.
func wrapHeight(s string, st textStyle, w float32) float32 {
	return float32(len(wrapPx(s, st, float64(w)))) * float32(lineH(st))
}

// unsavedDefs are this session's custom scenarios that were never saved
// to a file; they stay in the list until the game quits.
type unsavedDefs []scenario.Def

// noTint draws a sprite untinted.
var noTint color.NRGBA

// declareOnScreenLayer makes the widgets a screen hook declares belong to
// the screen's own layer (0), so they stay dead and painted under an open
// modal. uiBegin hands hooks the top layer, which would keep them live and
// draw them over the dialog.
func (a *App) declareOnScreenLayer() { a.uiSt().layer = 0 }
