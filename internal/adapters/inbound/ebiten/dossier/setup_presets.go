package dossier

import (
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/dossier/sheet"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/platform/config"
)

// The Presets tab (RHUD-058, 358-369, 371-378, 380): the built-in and the
// player's looks in one list, keeping the current look under a name, deleting
// a kept one, and showing the folder the files live in. The files are read when
// the tab opens, so one dropped into the folder shows without a restart.

// maxKept is how many of the player's presets the list reads: a folder with
// more is cut off here, so a hostile or huge folder cannot grow the list or
// the work done when the tab opens.
const maxKept = 100

// presetEntry is one row of the list.
type presetEntry struct {
	stem    string // the file name, "" for a built-in
	label   string
	p       sheet.Preset
	builtIn bool
	err     error // the file cannot be used
}

// Presets is the Presets tab.
type Presets struct {
	s *Setup

	entries  []presetEntry
	sel      int
	have     bool
	seenAt   uint64
	name     string
	flags    sheet.SaveFlags
	openErr  string
	listErr  string
	contentH float32
}

func newPresets(s *Setup) *Presets {
	return &Presets{s: s, sel: -1, flags: sheet.SaveFlags{Docked: true, Floating: true, Width: true, Height: true, Tabs: true, Text: true}}
}

// Draw does nothing: Declare queues every paint through the kit.
func (t *Presets) Draw(dst *ebiten.Image, r gfx.Rect) {}

// refresh reads the built-in presets and the files in the folder.
func (t *Presets) refresh() {
	keep := ""
	if t.sel >= 0 && t.sel < len(t.entries) {
		keep = t.entries[t.sel].label
	}
	t.entries, t.listErr, t.openErr = t.entries[:0], "", ""
	for _, p := range sheet.BuiltIns() {
		t.entries = append(t.entries, presetEntry{label: p.Name, p: p, builtIn: true})
	}
	if t.s.store != nil {
		stems, err := t.s.store.List()
		if err != nil {
			t.listErr = err.Error()
		}
		if len(stems) > maxKept {
			stems = stems[:maxKept]
		}
		for _, stem := range stems {
			t.entries = append(t.entries, t.read(stem))
		}
	}
	t.sel = -1
	for i, e := range t.entries {
		if e.label == keep && keep != "" {
			t.sel = i
		}
	}
	t.have, t.seenAt = true, t.s.k.Host().Updates()
}

// read decodes one file; a file that cannot be used stays in the list with its
// reason.
func (t *Presets) read(stem string) presetEntry {
	e := presetEntry{stem: stem, label: stem}
	data, err := t.s.store.Load(stem)
	if err == nil {
		e.p, err = sheet.DecodePreset(data)
	}
	if err != nil {
		e.err = err
		return e
	}
	if n := strings.TrimSpace(e.p.Name); n != "" {
		e.label = n
	}
	return e
}

// Declare lays the tab out in r and declares its widgets.
func (t *Presets) Declare(r gfx.Rect) {
	k := t.s.k
	if now := k.Host().Updates(); !t.have || now > t.seenAt+3 { // the tab was not shown a moment ago: it just opened
		t.refresh()
	}
	t.seenAt = k.Host().Updates()
	cur := t.s.HUD().Preset

	left, right := r.SplitH(r.W * 0.5)
	left.W -= 8
	right.X += 4
	right.W -= 4

	hs := gfx.Label
	hs.Color = uikit.Heading
	k.Label(left.X, left.Y, t.s.a.T("Looks for the card"), hs)
	btnY := left.Bottom() - secBtnH
	list := gfx.Rect{X: left.X, Y: left.Y + 24, W: left.W, H: max(40, btnY-left.Y-24-6)}
	k.List("pre.list", list, len(t.entries), &t.sel, uikit.ListSpec{
		RowH: secRowH,
		Tip:  func(i int) string { return t.entryTip(t.entries[i]) },
		Draw: func(dst *ebiten.Image, i int, row gfx.Rect, hover, selected bool) {
			t.drawEntry(dst, i, row, hover, selected, cur)
		},
	})
	k.Queue(func(dst *ebiten.Image) { uikit.StrokeRect(dst, list, uikit.PanelBorder) })

	btns := gfx.Rect{X: left.X, Y: btnY, W: left.W, H: secBtnH}.Columns(2, 4)
	e, has := t.chosen()
	uikit.StyledButton(k, "pre.load", btns[0], t.s.a.T("Use this look"), uikit.BtnOpts{Style: uikit.BtnPrimary, Disabled: !has, Reason: t.s.a.T("Choose a look from the list first.")}, func() { t.load(e) })
	reason := t.s.a.T("Choose one of your own looks from the list first.")
	if has && e.builtIn {
		reason = t.s.a.T("A built-in look cannot be deleted.")
	}
	uikit.StyledButton(k, "pre.delete", btns[1], t.s.a.T("Delete"), uikit.BtnOpts{Style: uikit.BtnDanger, Disabled: !has || e.builtIn || t.s.store == nil, Reason: reason}, func() { t.confirmDelete(e) })

	t.declareRight(right, cur)
}

func (t *Presets) chosen() (presetEntry, bool) {
	if t.sel < 0 || t.sel >= len(t.entries) {
		return presetEntry{}, false
	}
	return t.entries[t.sel], true
}

func (t *Presets) entryTip(e presetEntry) string {
	switch {
	case e.err != nil:
		return t.s.a.Tf("%s\nThis file cannot be used: %s", e.label, e.err.Error())
	case e.builtIn:
		return t.s.a.Tf("%s\nA look that comes with the game.", e.label)
	}
	return t.s.a.Tf("%s\nOne of your own looks, kept in %s.json.", e.label, e.stem)
}

func (t *Presets) drawEntry(dst *ebiten.Image, i int, row gfx.Rect, hover, selected bool, cur string) {
	if i >= len(t.entries) {
		return
	}
	e := t.entries[i]
	switch {
	case selected:
		uikit.FillRect(dst, row, uikit.BtnSelected)
	case hover:
		uikit.FillRect(dst, row, uikit.BtnHover)
	}
	src := t.s.a.T("yours")
	if e.builtIn {
		src = t.s.a.T("built in")
	}
	st := gfx.Body
	switch {
	case e.err != nil:
		st.Color, src = uikit.Danger, t.s.a.T("cannot be read")
	case cur != "" && e.label == cur:
		st.Color = uikit.Accent
		src = t.s.a.Tf("%s, in use", src)
	}
	dim := gfx.Small
	dim.Color, dim.Align = uikit.TextDim, gfx.AlignRight
	gfx.Text(dst, src, float64(row.Right())-6, gfx.CenterTop(float64(row.Y), float64(row.H), dim), dim)
	w := float32(gfx.TextW(src, gfx.Small)) + 18
	gfx.Text(dst, uikit.FitText(e.label, st, row.W-w-8), float64(row.X)+8, gfx.CenterTop(float64(row.Y), float64(row.H), st), st)
}

func (t *Presets) declareRight(r gfx.Rect, cur string) {
	k := t.s.k
	area := k.ScrollArea("pre.detail", r, t.contentH, uikit.ScrollOpts{})
	f := uikit.NewForm(area)
	o := kitOwner{k}

	f.SectionTitle(o, t.s.a.T("In use"))
	words := t.s.a.T("The card is set up by hand: no look is in use.")
	if cur != "" {
		words = t.s.a.Tf("The card uses the look \"%s\". Any change to the card's sections puts it back to a hand-made one.", cur)
	}
	t.sectionText(f, words)

	f.SectionTitle(o, t.s.a.T("Keep the current look"))
	if row, ok := f.Row(30); ok {
		k.TextField("pre.name", row.InsetXY(0, 2), &t.name, uikit.TextFieldSpec{
			MaxLen: sheet.MaxPresetName, Placeholder: t.s.a.T("A name for this look"),
			Valid: func(r rune) bool {
				return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == ' '
			},
			Tip: t.s.a.T("Name\nLetters, digits, spaces, - and _. The built-in names are taken."),
		})
	}
	flags := []struct {
		id, label, tip string
		v              *bool
	}{
		{"docked", t.s.a.T("Pane sections"), t.s.a.T("The layout of the pane beside the tabs."), &t.flags.Docked},
		{"floating", t.s.a.T("Card sections"), t.s.a.T("The layout of the floating card."), &t.flags.Floating},
		{"width", t.s.a.T("How wide"), t.s.a.T("The pane's tab width and the card's width."), &t.flags.Width},
		{"height", t.s.a.T("How tall"), t.s.a.T("The pane's and the card's height."), &t.flags.Height},
		{"tabs", t.s.a.T("Tab count"), t.s.a.T("How many tabs the pane has room for."), &t.flags.Tabs},
		{"text", t.s.a.T("Type sizes"), t.s.a.T("The text sizes and spacing."), &t.flags.Text},
	}
	for i := 0; i < len(flags); i += 2 {
		row, ok := f.Row(28)
		if !ok {
			continue
		}
		for j, c := range flags[i : i+2] {
			k.Check("pre.keep."+c.id, row.Columns(2, 4)[j], c.label, c.v, uikit.CheckOpts{Tip: t.s.a.Tf("Keep: %s\n%s", c.label, c.tip)})
		}
	}
	if row, ok := f.Row(secBtnH + 6); ok {
		uikit.StyledButton(k, "pre.save", row.InsetXY(0, 2), t.s.a.T("Keep as a look"), uikit.BtnOpts{Style: uikit.BtnPrimary, Disabled: t.saveBlock() != "", Reason: t.saveBlock()}, t.save)
	}

	f.Gap(6)
	f.SectionTitle(o, t.s.a.T("Files"))
	where := t.s.folder
	if where == "" {
		where = t.s.a.T("(no folder in this run)")
	}
	t.sectionText(f, t.s.a.Tf("Looks are kept in %s. Drop a file there and it shows here the next time this tab opens.", where))
	if t.listErr != "" {
		t.sectionText(f, t.s.a.Tf("The folder could not be read: %s", t.listErr))
	}
	if row, ok := f.Row(secBtnH + 6); ok {
		reason := t.openErr
		if t.s.open == nil || t.s.folder == "" {
			reason = t.s.a.T("This run has no folder to show.")
		}
		uikit.StyledButton(k, "pre.folder", row.InsetXY(0, 2), t.s.a.T("Show card files"), uikit.BtnOpts{Disabled: reason != "", Reason: reason}, t.showFolder)
	}
	t.contentH = f.Content()
}

func (t *Presets) sectionText(f *uikit.Form, s string) {
	st := uikit.DimText()
	lines := gfx.WrapPx(s, st, float64(f.W))
	if row, ok := f.Row(float32(len(lines))*float32(gfx.LineH(st)) + 4); ok {
		uikit.TextLines(t.s.k, row.X, row.Y, row.W, s, st)
	}
}

// saveBlock is why the current look cannot be kept now, or "".
func (t *Presets) saveBlock() string {
	switch {
	case t.s.store == nil:
		return t.s.a.T("This run has nowhere to keep looks.")
	case sheet.ValidName(t.name) != nil:
		return sheet.ValidName(t.name).Error() + "."
	case t.flags == (sheet.SaveFlags{}):
		return t.s.a.T("Choose at least one part to keep.")
	}
	stem, kept := sheet.Slug(t.name), 0
	for _, e := range t.entries {
		if e.builtIn {
			continue
		}
		kept++
		if e.stem == stem {
			return ""
		}
	}
	if kept >= maxKept {
		return t.s.a.T("There is room for 100 looks; delete one first.")
	}
	return ""
}

// save keeps the current look under the typed name.
func (t *Presets) save() {
	name := strings.TrimSpace(t.name)
	p := sheet.Capture(t.s.HUD(), name, t.flags)
	data, err := sheet.EncodePreset(p)
	if err == nil && len(data) > sheet.MaxPresetBytes {
		err = sheet.ErrPresetTooBig
	}
	if err == nil {
		err = t.s.store.Save(sheet.Slug(name), data)
	}
	if err != nil {
		t.s.a.Hint(t.s.a.Tf("The look was not kept: %s", err.Error()))
		return
	}
	t.setUsing(name)
	t.sel = -1
	t.entries = nil
	t.have = false
	t.refresh()
	for i, e := range t.entries {
		if e.stem == sheet.Slug(name) {
			t.sel = i
		}
	}
	t.s.a.Hint(t.s.a.Tf("Kept the look \"%s\".", name))
}

// setUsing names the look in use without touching the layouts (an Edit would
// clear the name).
func (t *Presets) setUsing(name string) {
	st := t.s.pr.Settings()
	st.HUD.Preset = name
	t.s.pr.SetSettings(st)
}

// load puts a look on the card (RHUD-359, 371-373, 375-376). A file that cannot
// be used sends the card back to the built-in layouts.
func (t *Presets) load(e presetEntry) {
	st := t.s.pr.Settings()
	if e.err != nil {
		st.HUD.Layouts, st.HUD.Preset = config.HUDLayouts{}, ""
		t.s.pr.SetSettings(st)
		t.s.p.Invalidate()
		t.s.a.Hint(t.s.a.Tf("\"%s\" cannot be used (%s). The built-in card is back.", e.label, e.err.Error()))
		return
	}
	sheet.Apply(e.p, &st.HUD)
	t.s.pr.SetSettings(st)
	t.s.p.Invalidate()
	msg := t.s.a.Tf("Now using the look \"%s\".", e.label)
	if e.builtIn && e.label == "Standard" {
		msg = t.s.a.T("The standard card is back.")
	}
	if sheet.IsOlder(e.p) {
		msg = t.s.a.Tf("%s It was made by an older game; check it over.", msg)
	}
	t.s.a.Hint(msg)
}

func (t *Presets) confirmDelete(e presetEntry) {
	t.s.k.Confirm("pre.delete", t.s.a.T("Delete this look?"), t.s.a.Tf("\"%s\" will be removed from the folder. The card keeps looking as it does now.", e.label), t.s.a.T("Delete"), true, func() {
		if err := t.s.store.Delete(e.stem); err != nil {
			t.s.a.Hint(t.s.a.Tf("Could not delete: %s", err.Error()))
			return
		}
		t.sel = -1
		t.refresh()
		t.s.a.Hint(t.s.a.Tf("Deleted the look \"%s\".", e.label))
	})
}

func (t *Presets) showFolder() {
	if err := t.s.open(t.s.folder); err != nil {
		t.openErr = t.s.a.Tf("The folder could not be shown: %s", err.Error())
		t.s.a.Hint(t.openErr)
	}
}
