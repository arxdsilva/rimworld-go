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
	k.Label(left.X, left.Y, "Looks for the card", hs)
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
	uikit.StyledButton(k, "pre.load", btns[0], "Use this look", uikit.BtnOpts{Style: uikit.BtnPrimary, Disabled: !has, Reason: "Choose a look from the list first."}, func() { t.load(e) })
	reason := "Choose one of your own looks from the list first."
	if has && e.builtIn {
		reason = "A built-in look cannot be deleted."
	}
	uikit.StyledButton(k, "pre.delete", btns[1], "Delete", uikit.BtnOpts{Style: uikit.BtnDanger, Disabled: !has || e.builtIn || t.s.store == nil, Reason: reason}, func() { t.confirmDelete(e) })

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
		return e.label + "\nThis file cannot be used: " + e.err.Error()
	case e.builtIn:
		return e.label + "\nA look that comes with the game."
	}
	return e.label + "\nOne of your own looks, kept in " + e.stem + ".json."
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
	src := "yours"
	if e.builtIn {
		src = "built in"
	}
	st := gfx.Body
	switch {
	case e.err != nil:
		st.Color, src = uikit.Danger, "cannot be read"
	case cur != "" && e.label == cur:
		st.Color = uikit.Accent
		src += ", in use"
	}
	dim := gfx.Small
	dim.Color, dim.Align = uikit.TextDim, gfx.AlignRight
	gfx.Text(dst, src, float64(row.Right())-6, float64(row.Y)+(float64(row.H)-gfx.LineH(dim))/2, dim)
	w := float32(gfx.TextW(src, gfx.Small)) + 18
	gfx.Text(dst, uikit.FitText(e.label, st, row.W-w-8), float64(row.X)+8, float64(row.Y)+(float64(row.H)-gfx.LineH(st))/2, st)
}

func (t *Presets) declareRight(r gfx.Rect, cur string) {
	k := t.s.k
	area := k.ScrollArea("pre.detail", r, t.contentH, uikit.ScrollOpts{})
	f := uikit.NewForm(area)
	o := kitOwner{k}

	f.SectionTitle(o, "In use")
	words := "The card is set up by hand: no look is in use."
	if cur != "" {
		words = "The card uses the look \"" + cur + "\". Any change to the card's sections puts it back to a hand-made one."
	}
	t.sectionText(f, words)

	f.SectionTitle(o, "Keep the current look")
	if row, ok := f.Row(30); ok {
		k.TextField("pre.name", row.InsetXY(0, 2), &t.name, uikit.TextFieldSpec{
			MaxLen: sheet.MaxPresetName, Placeholder: "A name for this look",
			Valid: func(r rune) bool {
				return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == ' '
			},
			Tip: "Name\nLetters, digits, spaces, - and _. The built-in names are taken.",
		})
	}
	flags := []struct {
		id, label, tip string
		v              *bool
	}{
		{"docked", "Pane sections", "The layout of the pane beside the tabs.", &t.flags.Docked},
		{"floating", "Card sections", "The layout of the floating card.", &t.flags.Floating},
		{"width", "How wide", "The pane's tab width and the card's width.", &t.flags.Width},
		{"height", "How tall", "The pane's and the card's height.", &t.flags.Height},
		{"tabs", "Tab count", "How many tabs the pane has room for.", &t.flags.Tabs},
		{"text", "Type sizes", "The text sizes and spacing.", &t.flags.Text},
	}
	for i := 0; i < len(flags); i += 2 {
		row, ok := f.Row(28)
		if !ok {
			continue
		}
		for j, c := range flags[i : i+2] {
			k.Check("pre.keep."+c.id, row.Columns(2, 4)[j], c.label, c.v, uikit.CheckOpts{Tip: "Keep: " + c.label + "\n" + c.tip})
		}
	}
	if row, ok := f.Row(secBtnH + 6); ok {
		uikit.StyledButton(k, "pre.save", row.InsetXY(0, 2), "Keep as a look", uikit.BtnOpts{Style: uikit.BtnPrimary, Disabled: t.saveBlock() != "", Reason: t.saveBlock()}, t.save)
	}

	f.Gap(6)
	f.SectionTitle(o, "Files")
	where := t.s.folder
	if where == "" {
		where = "(no folder in this run)"
	}
	t.sectionText(f, "Looks are kept in "+where+". Drop a file there and it shows here the next time this tab opens.")
	if t.listErr != "" {
		t.sectionText(f, "The folder could not be read: "+t.listErr)
	}
	if row, ok := f.Row(secBtnH + 6); ok {
		reason := t.openErr
		if t.s.open == nil || t.s.folder == "" {
			reason = "This run has no folder to show."
		}
		uikit.StyledButton(k, "pre.folder", row.InsetXY(0, 2), "Show card files", uikit.BtnOpts{Disabled: reason != "", Reason: reason}, t.showFolder)
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
		return "This run has nowhere to keep looks."
	case sheet.ValidName(t.name) != nil:
		return sheet.ValidName(t.name).Error() + "."
	case t.flags == (sheet.SaveFlags{}):
		return "Choose at least one part to keep."
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
		return "There is room for 100 looks; delete one first."
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
		t.s.a.Hint("The look was not kept: " + err.Error())
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
	t.s.a.Hint("Kept the look \"" + name + "\".")
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
		t.s.a.Hint("\"" + e.label + "\" cannot be used (" + e.err.Error() + "). The built-in card is back.")
		return
	}
	sheet.Apply(e.p, &st.HUD)
	t.s.pr.SetSettings(st)
	t.s.p.Invalidate()
	msg := "Now using the look \"" + e.label + "\"."
	if e.builtIn && e.label == "Standard" {
		msg = "The standard card is back."
	}
	if sheet.IsOlder(e.p) {
		msg += " It was made by an older game; check it over."
	}
	t.s.a.Hint(msg)
}

func (t *Presets) confirmDelete(e presetEntry) {
	t.s.k.Confirm("pre.delete", "Delete this look?", "\""+e.label+"\" will be removed from the folder. The card keeps looking as it does now.", "Delete", true, func() {
		if err := t.s.store.Delete(e.stem); err != nil {
			t.s.a.Hint("Could not delete: " + err.Error())
			return
		}
		t.sel = -1
		t.refresh()
		t.s.a.Hint("Deleted the look \"" + e.label + "\".")
	})
}

func (t *Presets) showFolder() {
	if err := t.s.open(t.s.folder); err != nil {
		t.openErr = "The folder could not be shown: " + err.Error()
		t.s.a.Hint(t.openErr)
	}
}
