package wizard

import (
	"fmt"
	"image/color"
	"sort"
	"strconv"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/globe"
	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
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
	DefaultStoryteller = "steady"
	defaultScenarioKey = "broken_hull"
	// DefaultMapSide is the map size a fresh draft asks for, capped to what
	// the engine gate allows today.
	DefaultMapSide = 250
)

const defaultPreset = difficulty.Settled

// DefaultCoverage is the globe coverage of a fresh draft (percent).
const DefaultCoverage = 30

// ScenEntry is one row source of the Choose Scenario list: a built-in, a
// custom file or an unsaved scenario from this session.
type ScenEntry struct {
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
func (e ScenEntry) deletable() bool { return !e.builtin }

// errCount is the number of error-severity problems.
func (e ScenEntry) errCount() int {
	n := 0
	for _, p := range e.problems {
		if p.Severity == scenario.Error {
			n++
		}
	}
	return n
}

// Draft is the wizard draft: everything chosen so far.
type Draft struct {
	Entries     []ScenEntry
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
	Globe     *globe.View
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

	// initial is the signature of the defaults, to tell whether a later step
	// was changed.
	initial string

	// Ed is the open Scenario Editor.
	Ed *editorState
	// Cd is the Custom Difficulty dialog's working copy.
	Cd CustomDiff
	// bufs holds the text of numeric fields; the text field edits it in
	// place between frames, so it lives here and not on the stack.
	bufs map[string]*string
	// formH remembers each scroll form's content height of the last frame.
	formH map[string]float32
}

// signature is a comparable summary of the steps after Choose Scenario.
func (ng *Draft) signature() string {
	return fmt.Sprint(ng.Storyteller, ng.Preset, ng.Custom, ng.SaveMode, ng.World, ng.Tile, ng.Season, ng.MapSize)
}

// changedLater reports a step after Choose Scenario differs from its default.
func (ng *Draft) changedLater() bool { return ng.signature() != ng.initial }

// Selected is the highlighted scenario entry.
func (ng *Draft) Selected() *ScenEntry {
	if ng.Sel < 0 || ng.Sel >= len(ng.Entries) {
		return nil
	}
	return &ng.Entries[ng.Sel]
}

// selectKey highlights the entry with key (the first one when it is gone).
func (ng *Draft) selectKey(key string) {
	for i, e := range ng.Entries {
		if e.Key == key {
			ng.Sel = i
			return
		}
	}
	ng.Sel = 0
}

func (ng *Draft) selectedKey() string {
	if e := ng.Selected(); e != nil {
		return e.Key
	}
	return defaultScenarioKey
}

// draft is the wizard's state, created on first use.
func (w *Flow) draft() *Draft {
	if w.d == nil {
		w.setDraft(w.newDraft())
	}
	return w.d
}

// newDraft is a fresh draft with the defaults, or the last choices stored
// in the settings.
func (w *Flow) newDraft() *Draft {
	g := w.sh.Settings().Gameplay
	seedText := RandomWorldPhrase()
	if w.sh.Env().DefaultSeed != 0 {
		seedText = strconv.FormatUint(w.sh.Env().DefaultSeed, 10)
	}
	ng := &Draft{
		Storyteller: DefaultStoryteller,
		Preset:      defaultPreset,
		SaveMode:    difficulty.ReloadAnytime,
		Tile:        -1,
		Season:      -1,
		MapSize:     DefaultMapSideEnabled(),
		NamesOf:     -1,
		World: planet.Params{SeedText: seedText, Coverage: DefaultCoverage, Rain: 2, Temp: 2, Pop: 1,
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
	ng.RefreshScenarios(w.store(), w.Unsaved)
	ng.selectKey(defaultScenarioKey)
	ng.initial = ng.signature()
	return ng
}

// newEntry is a list entry for a readable definition.
func newEntry(d scenario.Def) ScenEntry {
	return ScenEntry{
		Key: d.Key, Name: d.Name, Summary: d.Summary, Def: d, builtin: d.Builtin,
		problems: d.Validate(), hardness: d.HardnessPips(), lines: ScenarioLines(d),
	}
}

// RefreshScenarios rebuilds the list: the built-ins, the files of the
// store (nil: none) and this session's unsaved scenarios. It keeps the
// selection.
func (ng *Draft) RefreshScenarios(st ScenarioStore, unsaved []scenario.Def) {
	prev := ng.selectedKey()
	entries := make([]ScenEntry, 0, 8)
	for _, d := range scenario.Builtins() {
		entries = append(entries, newEntry(d))
	}
	var custom []ScenEntry
	if st != nil {
		files, err := st.List()
		if err != nil {
			ng.err = "scenarios: " + err.Error()
		}
		for _, f := range files {
			e := ScenEntry{Key: "file." + f.Name, Name: f.Name, File: f.Name, modified: f.Modified}
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
	for i, d := range unsaved {
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

// GoBack answers the shell's Back (the Back button, Escape) on a setup
// screen: true lets the shell pop the screen; false keeps it, when the
// colony is being built or when popping would throw work away and the
// player is asked first (NG-1 §1.5). A confirmed discard asks the shell
// again (pop), and that Back goes through.
func (w *Flow) GoBack() bool {
	if w.confirmed {
		w.confirmed = false
		return true
	}
	if ng := w.d; ng != nil && ng.Load != nil {
		return false // the colony is being built: nothing to go back to
	}
	switch w.screen() {
	case ports.ScreenScenarioEditor:
		if ng := w.d; ng != nil && ng.Ed != nil && ng.Ed.Dirty() {
			w.kit().Confirm("sced.discard", "Discard changes?", "The scenario has changes that were not saved or kept. Leave the editor and lose them?", "Discard", true, w.pop)
			return false
		}
	case ports.ScreenScenario:
		if ng := w.d; ng != nil && ng.changedLater() {
			w.kit().Confirm("scen.discard", "Discard this new colony setup?", "You changed later steps of this setup. Going back to the main menu throws those choices away.", "Discard", true, w.pop)
			return false
		}
	}
	return true
}

// back asks the shell to go back a screen (GoBack decides).
func (w *Flow) back() { w.sh.Do(ports.Back{}) }

// pop leaves the screen without asking: the shell's Back, already agreed.
func (w *Flow) pop() {
	w.confirmed = true
	w.sh.Do(ports.Back{})
}

// Popped tidies the draft after the shell popped screen was and shows
// now: the editor's state goes with the editor, and the main menu forgets
// the whole draft.
func (w *Flow) Popped(was, now ports.Screen) {
	switch {
	case was == ports.ScreenScenarioEditor && w.d != nil:
		w.d.Ed = nil
	case now == ports.ScreenMainMenu:
		w.setDraft(nil)
	}
}

// forward pushes the next wizard screen.
func (w *Flow) forward() { w.sh.Do(ports.Push{Screen: Next(w.screen())}) }

// Click runs a new-game widget id from the legacy click path (captures
// and tests that click by id); widgets normally handle their own clicks.
// It reports the id was one of these screens'.
func (w *Flow) Click(id string) bool {
	switch {
	case id == ids.MenuNewGame:
		w.setDraft(nil)
		w.draft()
		w.sh.Do(ports.Push{Screen: ports.ScreenScenario})
	case strings.HasPrefix(id, "scen."), strings.HasPrefix(id, "sced."),
		strings.HasPrefix(id, "story."), strings.HasPrefix(id, "diff."), strings.HasPrefix(id, "save."),
		strings.HasPrefix(id, "world."), strings.HasPrefix(id, "land."), strings.HasPrefix(id, "pawns."),
		strings.HasPrefix(id, "fac."), strings.HasPrefix(id, "wizard."):
		// Kit widgets answer these through Kit.Click; a stale id does nothing.
	default:
		return false
	}
	return true
}

// ---- shared widgets ----------------------------------------------------

// ngFooter declares the Back button and the forward button of a setup
// screen. Enter presses the forward button when it is enabled.
func (w *Flow) ngFooter(footer gfx.Rect, label, nextID string, o uikit.BtnOpts, onNext func()) {
	back := func() { w.kit().Host().Cue("ui.page"); w.back() }
	uikit.StyledButton(w.kit(), ids.WizardBack, uikit.FooterBack(footer), "Back", uikit.BtnOpts{}, back)
	if o.Style == uikit.BtnNormal {
		o.Style = uikit.BtnPrimary
	}
	if nextID == ids.WizardNext {
		inner := onNext
		onNext = func() { w.kit().Host().Cue("ui.page"); inner() }
	}
	uikit.StyledButton(w.kit(), nextID, uikit.FooterNext(footer), label, o, onNext)
	if w.in().Fired(input.Accept) && !o.Disabled && !w.kit().ModalOpen() && !w.kit().Editing() {
		onNext()
	}
	if o.Disabled && o.Reason != "" {
		w.kit().Label(uikit.FooterBack(footer).Right()+16, footer.Y+(footer.H-float32(gfx.LineH(gfx.Body)))/2, o.Reason, uikit.DimText())
	}
}

// drawNGFrame paints the backdrop, the dialog frame and its title; the
// widgets of the screen's hook go on top. A window below the minimum gets
// the "too small" note instead.
func (w *Flow) drawNGFrame(screen *ebiten.Image, sw, sh int, title string) {
	var p *planet.Planet
	if w.d != nil {
		p = w.d.Planet
	}
	w.DrawBackdrop(screen, sw, sh, p)
	if uikit.TooSmall(sw, sh) {
		uikit.DrawTooSmall(screen, sw, sh)
		return
	}
	frame, titleR, _, footer := uikit.DialogFrame(sw, sh)
	uikit.DrawPanel(screen, frame.X, frame.Y, frame.W, frame.H)
	st := gfx.Title
	st.Color = uikit.Heading
	gfx.Text(screen, title, float64(titleR.X+uikit.DialogPad), float64(titleR.Y)+(float64(titleR.H)-gfx.LineH(st))/2, st)
	line := uikit.PanelBorder
	uikit.FillRect(screen, gfx.Rect{X: frame.X, Y: titleR.Bottom() - 1, W: frame.W, H: 1}, line)
	uikit.FillRect(screen, gfx.Rect{X: frame.X, Y: footer.Y, W: frame.W, H: 1}, line)
}

// formHeight is the content height remembered for the form id.
func (ng *Draft) formHeight(id string) float32 { return ng.formH[id] }

func (ng *Draft) setFormHeight(id string, h float32) {
	if ng.formH == nil {
		ng.formH = map[string]float32{}
	}
	ng.formH[id] = h
}

// ngNum is a digits-only text field over *v. The value follows what is
// typed (validation shows out-of-range numbers); committing clamps it to
// [lo, hi].
func (w editorPage) ngNum(id string, r gfx.Rect, v *int, lo, hi int) {
	ng := w.draft()
	if ng.bufs == nil {
		ng.bufs = map[string]*string{}
	}
	buf := ng.bufs[id]
	if buf == nil {
		buf = new(string)
		ng.bufs[id] = buf
	}
	if w.kit().Edit.ID != id { // not being typed into: follow the model
		*buf = strconv.Itoa(*v)
	}
	changed, committed := w.kit().TextField(id, r, buf, uikit.TextFieldSpec{MaxLen: 5, Valid: uikit.DigitsOnly, Tip: fmt.Sprintf("%d to %d", lo, hi)})
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

// wrapHeight is the height wrapPx would use for s.
func wrapHeight(s string, st gfx.TextStyle, w float32) float32 {
	return float32(len(gfx.WrapPx(s, st, float64(w)))) * float32(gfx.LineH(st))
}

// noTint draws a sprite untinted.
var noTint color.NRGBA
