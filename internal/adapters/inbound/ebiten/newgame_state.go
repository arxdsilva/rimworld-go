package ebiten

import (
	"fmt"
	"image/color"
	"sort"
	"strconv"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/ids"
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
	Key, Name, Summary string
	Def                scenario.Def
	builtin, Unsaved   bool
	File               string // the store file name of a saved custom scenario
	Err                string // why a file can't be read
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
	Entries     []scenEntry
	Sel         int          // index into entries
	Scenario    scenario.Def // working copy, stamped when Next is pressed on Choose Scenario
	Search      string
	Storyteller string // profile key
	Preset      difficulty.Preset
	Custom      difficulty.Settings // the Custom preset's settings
	CustomOpen  bool                // the Custom Difficulty dialog has been shown once
	SaveMode    difficulty.SaveMode

	// Create World, Landing and Characters (screen_worldgen.go,
	// screen_landing.go, screen_pawns.go).
	World   planet.Params
	Tile    int // selected landing tile, -1 none
	Season  int // -1 Auto, else a season index (0 Spring .. 3 Winter)
	MapSize int
	err     string

	Planet    *planet.Planet  // the generated world, kept across Back
	PlanetKey string          // Params.Key() the planet was built from
	Builder   *planet.Builder // in-flight generation
	GenStep   int             // work units stepped per frame
	GenPhase  planet.Phase
	GenFrac   float32
	genErr    string  // why the last generation failed
	GenHold   float32 // captures: stop stepping at this progress
	Globe     *globeView
	globeOf   *planet.Planet
	LandRolls int  // presses of "Select random site"
	Outside   bool // the last click was on the uncovered part of the globe
	Legend    bool // the biome legend is open
	facSel    int  // highlighted row of the Factions table
	Info      infoCache

	Pool    candidatePool // the character candidates
	PoolKey string        // what the pool was made from
	PawnSel int           // highlighted roster position
	Names   [3]string     // first, nick, last drafts of the highlighted candidate
	NamesOf int           // the roster position the drafts belong to (-1 none)
	NameErr string
	drag    rosterDrag
	Load    *loadState // the colony is being built
	// BuildHook replaces the one-step builder (tests).
	BuildHook func(NewGame) (SimBuilder, error)

	// initial is the signature of the defaults, to tell whether a later step
	// was changed.
	initial string

	// Ed is the open Scenario Editor.
	Ed *editorState
	// Cd is the Custom Difficulty dialog's working copy.
	Cd customDiff
	// bufs holds the text of numeric fields; the text field edits it in
	// place between frames, so it lives here and not on the stack.
	bufs map[string]*string
	// formH remembers each scroll form's content height of the last frame.
	formH map[string]float32
}

// signature is a comparable summary of the steps after Choose Scenario.
func (ng *newGameState) signature() string {
	return fmt.Sprint(ng.Storyteller, ng.Preset, ng.Custom, ng.SaveMode, ng.World, ng.Tile, ng.Season, ng.MapSize)
}

// changedLater reports a step after Choose Scenario differs from its default.
func (ng *newGameState) changedLater() bool { return ng.signature() != ng.initial }

// Selected is the highlighted scenario entry.
func (ng *newGameState) Selected() *scenEntry {
	if ng.Sel < 0 || ng.Sel >= len(ng.Entries) {
		return nil
	}
	return &ng.Entries[ng.Sel]
}

// selectKey highlights the entry with key (the first one when it is gone).
func (ng *newGameState) selectKey(key string) {
	for i, e := range ng.Entries {
		if e.Key == key {
			ng.Sel = i
			return
		}
	}
	ng.Sel = 0
}

func (ng *newGameState) selectedKey() string {
	if e := ng.Selected(); e != nil {
		return e.Key
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
		Storyteller: defaultStoryteller,
		Preset:      defaultPreset,
		SaveMode:    difficulty.ReloadAnytime,
		Tile:        -1,
		Season:      -1,
		MapSize:     defaultMapSideEnabled(),
		NamesOf:     -1,
		World: planet.Params{SeedText: seedText, Coverage: defaultCoverage, Rain: 2, Temp: 2, Pop: 1,
			Factions: faction.DefaultSlots()},
	}
	if p, ok := storyteller.ByKey(g.LastStoryteller); ok && !p.Hidden {
		ng.Storyteller = p.Key
	}
	if p, ok := difficulty.Parse(g.LastDifficulty); ok && p != difficulty.Custom {
		ng.Preset = p
	}
	if g.LastSaveMode == difficulty.Commitment.String() {
		ng.SaveMode = difficulty.Commitment
	}
	ng.Custom = difficulty.Of(ng.Preset)
	ng.RefreshScenarios(a)
	ng.selectKey(defaultScenarioKey)
	ng.initial = ng.signature()
	return ng
}

// newEntry is a list entry for a readable definition.
func newEntry(d scenario.Def) scenEntry {
	return scenEntry{
		Key: d.Key, Name: d.Name, Summary: d.Summary, Def: d, builtin: d.Builtin,
		problems: d.Validate(), hardness: d.HardnessPips(), lines: scenarioLines(d),
	}
}

// RefreshScenarios rebuilds the list: the built-ins, the files of the
// store and this session's unsaved scenarios. It keeps the selection.
func (ng *newGameState) RefreshScenarios(a *App) {
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
			e := scenEntry{Key: "file." + f.Name, Name: f.Name, File: f.Name, modified: f.Modified}
			data, err := st.Load(f.Name)
			var d scenario.Def
			if err == nil {
				d, err = scenario.Decode(data)
			}
			if err != nil {
				e.Err = err.Error()
				e.Summary = "Can't read this file"
				custom = append(custom, e)
				continue
			}
			d.Builtin = false
			ne := newEntry(d)
			ne.Key, ne.File, ne.modified = e.Key, f.Name, f.Modified
			if strings.TrimSpace(ne.Name) == "" {
				ne.Name = f.Name
			}
			custom = append(custom, ne)
		}
	}
	for i, d := range a.ngUnsaved {
		d.Builtin = false
		e := newEntry(d)
		e.Key = "unsaved." + strconv.Itoa(i)
		e.Unsaved = true
		e.Name = d.Name + " (unsaved)"
		custom = append(custom, e)
	}
	sort.SliceStable(custom, func(i, j int) bool {
		return strings.ToLower(custom[i].Name) < strings.ToLower(custom[j].Name)
	})
	ng.Entries = append(entries, custom...)
	ng.selectKey(prev)
}

// ---- navigation ------------------------------------------------------

// goBack is the Back button and Escape on a setup screen: it pops one
// screen, asking first when that would throw work away (NG-1 §1.5).
func (a *App) goBack() {
	if ng := a.ng; ng != nil && ng.Load != nil {
		return // the colony is being built: nothing to go back to
	}
	switch a.nav.screen() {
	case screenScenarioEditor:
		if ng := a.ng; ng != nil && ng.Ed != nil && ng.Ed.Dirty() {
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
	a.uiSt().Modals = nil
	was := a.nav.screen()
	a.nav.pop()
	switch {
	case was == screenScenarioEditor && a.ng != nil:
		a.ng.Ed = nil
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
	case id == ids.MenuNewGame:
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
	if n := len(u.Cur); n > 0 && u.Cur[n-1].ID == id {
		u.Cur[n-1].Click = func() {
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
	back := func() { a.pageSound(); a.goBack() }
	a.ngButton(ids.WizardBack, footerBack(footer), "Back", btnOpts{}, back)
	if o.Style == btnNormal {
		o.Style = btnPrimary
	}
	if nextID == ids.WizardNext {
		inner := onNext
		onNext = func() { a.pageSound(); inner() }
	}
	a.ngButton(nextID, footerNext(footer), label, o, onNext)
	if a.in.Fired(input.Accept) && !o.Disabled && !a.uiModalOpen() && !a.uiTextEditing() {
		onNext()
	}
	if o.Disabled && o.Reason != "" {
		a.uiLabel(footerBack(footer).Right()+16, footer.Y+(footer.H-float32(lineH(tsBody)))/2, o.Reason, dimText())
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
		p = a.ng.Planet
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
	fillRect(screen, rect{X: frame.X, Y: titleR.Bottom() - 1, W: frame.W, H: 1}, line)
	fillRect(screen, rect{X: frame.X, Y: footer.Y, W: frame.W, H: 1}, line)
}

// formCtx lays a vertical form out inside a scroll view: row hands out
// the next row of the content, shifted by the scroll offset; a row that is
// not fully inside the view reports false and the caller declares no
// widget for it.
type formCtx struct {
	View rect
	X, W float32
	y    float32 // content height so far
	Off  float32
}

// newForm starts a form in the view of a scroll area (padding 4 px).
func newForm(area scrollArea) *formCtx {
	return &formCtx{View: area.View, X: area.View.X + 4, W: max(0, area.View.W-8), Off: area.Off}
}

// Row is the next Row of height h.
func (f *formCtx) Row(h float32) (rect, bool) {
	r := rect{X: f.X, Y: f.View.Y + f.y - f.Off, W: f.W, H: h}
	f.y += h
	return r, r.Y >= f.View.Y-0.5 && r.Bottom() <= f.View.Bottom()+0.5
}

// Gap leaves h px empty.
func (f *formCtx) Gap(h float32) { f.y += h }

// Content is the Content height so far (for the scroll area next frame).
func (f *formCtx) Content() float32 { return f.y + 4 }

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
	return rect{X: r.X + labelW, Y: r.Y, W: max(0, r.W-labelW), H: r.H}
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
		ID: mid, Title: title, W: 380, H: h,
		Draw: func(a *App, body rect) {
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
	a.ngButton(id+".dec", rect{X: r.X, Y: r.Y, W: bw, H: r.H}, "-", btnOpts{Disabled: *v <= lo, Reason: fmt.Sprintf("The minimum is %d.", lo)}, func() { *v = max(lo, *v-1) })
	a.ngButton(id+".inc", rect{X: r.X + bw + 56, Y: r.Y, W: bw, H: r.H}, "+", btnOpts{Disabled: *v >= hi, Reason: fmt.Sprintf("The maximum is %d.", hi)}, func() { *v = min(hi, *v+1) })
	val := *v
	a.uiSt().Queue(func(dst *ebiten.Image) {
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
	if a.uiSt().Edit.ID != id { // not being typed into: follow the model
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
		a.ngButton(optID(id, i), rect{X: r.X + float32(i)*(w+4), Y: r.Y, W: w, H: r.H}, l, o, func() { *sel = i })
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
func (a *App) declareOnScreenLayer() { a.uiSt().Layer = 0 }
