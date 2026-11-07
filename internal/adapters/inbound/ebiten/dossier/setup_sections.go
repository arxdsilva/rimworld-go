package dossier

import (
	"errors"
	"image/color"
	"reflect"
	"strconv"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/dossier/sheet"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/lang"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/platform/config"
)

// The Sections tab (RHUD-330...353): the tree of the current mode's elements
// on the left, the selected element's settings and the add menus on the right.
// The editor works on a copy of the tree; every edit is applied to a clone,
// checked with sheet.Validate, written through Setup.Edit (which clears the
// current preset) and read back by the card at once.

const (
	secRowH   = 22
	secIndent = 14
	secBtnH   = 26
)

// secRow is one visible line of the tree.
type secRow struct {
	path  []int
	depth int
	n     config.HUDNode // a copy; its Kids are not used
	kids  int
}

// Sections is the Sections tab.
type Sections struct {
	s *Setup

	cat map[string]sheet.Entry

	work      config.HUDNode
	seen      *config.HUDNode // the settings' tree this copy came from
	loaded    bool
	docked    bool
	sel       []int
	collapsed map[string]bool
	rows      []secRow
	dirty     bool
	mine      map[string]bool // kind|def pairs the built-in tree has
	pal       map[string]color.RGBA
	contentH  float32
}

func newSections(s *Setup) *Sections {
	t := &Sections{s: s, cat: map[string]sheet.Entry{}, collapsed: map[string]bool{}, pal: map[string]color.RGBA{}}
	t.loadCat()
	return t
}

// loadCat reads the element catalog in the current language.
func (t *Sections) loadCat() {
	for _, e := range sheet.Catalog(t.s.a) {
		t.cat[e.Key] = e
	}
}

// Draw does nothing: Declare queues every paint through the kit.
func (t *Sections) Draw(dst *ebiten.Image, r gfx.Rect) {}

// ptrOf is the settings' tree for a mode.
func ptrOf(h config.HUD, docked bool) *config.HUDNode {
	if docked {
		return h.Layouts.Docked
	}
	return h.Layouts.Floating
}

func defaultOf(docked bool) config.HUDNode {
	if docked {
		return sheet.DefaultDocked()
	}
	return sheet.DefaultFloating()
}

// cloneNode copies a tree; an empty child list is nil, as in a file.
func cloneNode(n config.HUDNode) config.HUDNode {
	if len(n.Kids) == 0 {
		n.Kids = nil
		return n
	}
	kids := make([]config.HUDNode, len(n.Kids))
	for i, k := range n.Kids {
		kids[i] = cloneNode(k)
	}
	n.Kids = kids
	return n
}

// sync reloads the working copy when the mode changed or something else (a
// preset, the defaults) replaced the settings' tree.
func (t *Sections) sync(h config.HUD) {
	cur := ptrOf(h, h.Docked)
	if t.loaded && t.docked == h.Docked && cur == t.seen {
		return
	}
	def := defaultOf(h.Docked)
	base := def
	if cur != nil {
		base = *cur
	}
	t.work, t.seen, t.docked, t.loaded = cloneNode(base), cur, h.Docked, true
	t.sel, t.collapsed, t.dirty = nil, map[string]bool{}, true
	t.mine = map[string]bool{}
	var walk func(n config.HUDNode)
	walk = func(n config.HUDNode) {
		t.mine[n.Kind+"|"+n.Def] = true
		for _, k := range n.Kids {
			walk(k)
		}
	}
	walk(def)
}

func pathKey(p []int) string {
	var b strings.Builder
	for _, i := range p {
		b.WriteString(strconv.Itoa(i))
		b.WriteByte('.')
	}
	return b.String()
}

// build flattens the tree, skipping the children of folded elements.
func (t *Sections) build() {
	t.rows = t.rows[:0]
	var walk func(n config.HUDNode, path []int)
	walk = func(n config.HUDNode, path []int) {
		t.rows = append(t.rows, secRow{path: path, depth: len(path), n: n, kids: len(n.Kids)})
		if t.collapsed[pathKey(path)] {
			return
		}
		for i, k := range n.Kids {
			walk(k, append(append([]int(nil), path...), i))
		}
	}
	walk(t.work, nil)
	t.dirty = false
}

func (t *Sections) selIndex() int {
	key := pathKey(t.sel)
	for i, r := range t.rows {
		if pathKey(r.path) == key {
			return i
		}
	}
	return 0
}

// colours resolves the tree_* keys: the player's colour, else the theme's.
func (t *Sections) colours(h config.HUD) {
	for _, k := range []string{"tree_stack", "tree_block", "tree_line", "tree_widget", "tree_bad", "tree_core", "tree_mine"} {
		c, _ := uikit.DossierColor(k)
		if s, ok := h.Colors[k]; ok {
			if pc, ok := sheet.ParseColor(s); ok {
				c = pc
			}
		}
		t.pal[k] = c
	}
}

// nameOf is an element's words: its label and, for a generic one, what it shows.
func (t *Sections) nameOf(n config.HUDNode) string {
	e, ok := t.cat[n.Kind]
	label := e.Label
	if !ok {
		label = t.s.a.Tf("Unknown: %s", n.Kind)
	}
	if n.Def != "" {
		if d, ok := sheet.LookupDef(t.s.a, n.Kind, n.Def); ok {
			label += ": " + d.Label
		} else {
			label += ": " + n.Def
		}
	}
	return label
}

// tint is a row's colour key by the kind of element (RHUD-333).
func (t *Sections) tint(n config.HUDNode) color.RGBA {
	e, ok := t.cat[n.Kind]
	switch {
	case !ok || e.Reserved:
		return t.pal["tree_bad"]
	case n.Kind == sheet.KindColumn || n.Kind == sheet.KindStrip:
		return t.pal["tree_stack"]
	case n.Kind == sheet.KindBlock:
		return t.pal["tree_block"]
	case n.Kind == sheet.KindLine:
		return t.pal["tree_line"]
	case n.Kind == "gap" || n.Kind == "rule":
		return t.pal["tree_core"]
	case !t.mine[n.Kind+"|"+n.Def]:
		return t.pal["tree_mine"]
	}
	return t.pal["tree_widget"]
}

// attrs are the words at a row's right edge (RHUD-334).
func attrs(tr lang.Tr, n config.HUDNode) string {
	var parts []string
	if n.BarStyle != "" {
		parts = append(parts, sheet.BarStyleLabel(tr, n.BarStyle))
	}
	if n.Fill {
		parts = append(parts, tr.T("fills"))
	}
	if n.Who != "" {
		parts = append(parts, strings.Join(strings.Split(sheet.WhoSet(n), ""), " "))
	}
	return strings.Join(parts, " | ")
}

func (t *Sections) tip(n config.HUDNode) string {
	title := t.nameOf(n)
	key := n.Kind
	if n.Def != "" {
		key += " / " + n.Def
	}
	body := t.s.a.Tf("Key: %s. Built in.", key)
	if e, ok := t.cat[n.Kind]; ok && e.Tip != "" {
		body = e.Tip + "\n" + body
	}
	return title + "\n" + body
}

// Declare lays the tab out in r and declares its widgets.
func (t *Sections) Declare(r gfx.Rect) {
	h := t.s.HUD()
	t.loadCat()
	t.sync(h)
	t.colours(h)
	if t.dirty {
		t.build()
	}
	k := t.s.k
	left, right := r.SplitH(r.W * 0.5)
	left.W -= 8
	right.X += 4
	right.W -= 4

	hs := gfx.Label
	hs.Color = uikit.Heading
	k.Label(left.X, left.Y, t.s.a.T("Elements"), hs)
	list := gfx.Rect{X: left.X, Y: left.Y + 24, W: left.W, H: max(40, left.H-24)}
	sel := t.selIndex()
	if k.List("sec.tree", list, len(t.rows), &sel, uikit.ListSpec{
		RowH: secRowH,
		Tip:  func(i int) string { return t.tip(t.rows[i].n) },
		Draw: func(dst *ebiten.Image, i int, row gfx.Rect, hover, selected bool) {
			t.drawRow(dst, i, row, hover, selected)
		},
	}) && sel >= 0 && sel < len(t.rows) {
		t.sel = t.rows[sel].path
	}
	k.Queue(func(dst *ebiten.Image) { uikit.StrokeRect(dst, list, uikit.PanelBorder) })
	t.declareDetail(right)
}

func (t *Sections) drawRow(dst *ebiten.Image, i int, row gfx.Rect, hover, selected bool) {
	if i >= len(t.rows) {
		return
	}
	rw := t.rows[i]
	switch {
	case selected:
		uikit.FillRect(dst, row, uikit.BtnSelected)
	case hover:
		uikit.FillRect(dst, row, uikit.BtnHover)
	}
	st := gfx.Body
	st.Color = t.tint(rw.n)
	x := float64(row.X) + 6 + float64(rw.depth*secIndent)
	y := gfx.CenterTop(float64(row.Y), float64(row.H), st)
	if rw.kids > 0 {
		mark := "-"
		if t.collapsed[pathKey(rw.path)] {
			mark = "+"
		}
		gfx.Text(dst, mark, x, y, st)
	}
	x += 12
	at := attrs(t.s.a, rw.n)
	dim := gfx.Small
	dim.Color = uikit.TextDim
	aw := float64(0)
	if at != "" {
		aw = float64(gfx.TextW(at, dim)) + 8
		dim.Align = gfx.AlignRight
		gfx.Text(dst, at, float64(row.Right())-6, gfx.CenterTop(float64(row.Y), float64(row.H), dim), dim)
	}
	label := t.nameOf(rw.n)
	if len(rw.path) == 0 {
		label = t.s.a.T("Whole card")
	}
	gfx.Text(dst, uikit.FitText(label, st, float32(float64(row.Right())-x-aw-6)), x, y, st)
}

// ownerOf lets the form helpers reach the kit.
type kitOwner struct{ k *uikit.Kit }

func (o kitOwner) Kit() *uikit.Kit { return o.k }

// selected is the selected node of the working copy, or nil.
func (t *Sections) selected() *config.HUDNode { return sheet.At(&t.work, t.sel) }

func (t *Sections) declareDetail(r gfx.Rect) {
	k := t.s.k
	area := k.ScrollArea("sec.detail", r, t.contentH, uikit.ScrollOpts{})
	f := uikit.NewForm(area)
	o := kitOwner{k}
	title := t.s.a.T("The card layout")
	if t.docked {
		title = t.s.a.T("The pane layout")
	}
	f.SectionTitle(o, title)
	t.textRow(f, t.s.a.T("Edits show on the card at once. The Look tab switches between the pane and the floating card."))

	n := t.selected()
	if n == nil {
		t.sel = nil
		n = &t.work
	}
	name := t.nameOf(*n)
	if len(t.sel) == 0 {
		name = t.s.a.T("Whole card")
	}
	f.SectionTitle(o, t.s.a.T("Selected"))
	if row, ok := f.Row(20); ok {
		st := gfx.Label
		k.Label(row.X, row.Y, uikit.FitText(name, st, row.W), st)
	}
	t.orderRow(f, *n)
	t.addRow(f)
	t.fillRow(f, *n)
	t.targetRows(f, o, *n)
	t.defRows(f, *n)
	f.Gap(6)
	f.SectionTitle(o, t.s.a.T("Back to the built-in layout"))
	if row, ok := f.Row(secBtnH + 4); ok {
		same := reflect.DeepEqual(t.work, defaultOf(t.docked)) && t.seen == nil
		uikit.StyledButton(k, "sec.default", row.InsetXY(0, 1), t.s.a.T("Use the built-in layout"), uikit.BtnOpts{
			Disabled: same, Reason: t.s.a.T("The built-in layout is already in use."),
			Tip: t.s.a.T("Built-in layout\nThrows away your changes to this layout. Presets are kept."),
		}, t.useDefault)
	}
	t.contentH = f.Content()
}

// textRow draws wrapped dim text as one form row.
func (t *Sections) textRow(f *uikit.Form, s string) {
	st := uikit.DimText()
	lines := gfx.WrapPx(s, st, float64(f.W))
	if row, ok := f.Row(float32(len(lines))*float32(gfx.LineH(st)) + 4); ok {
		uikit.TextLines(t.s.k, row.X, row.Y, row.W, s, st)
	}
}

func (t *Sections) fillRow(f *uikit.Form, n config.HUDNode) {
	if n.Kind != sheet.KindColumn && n.Kind != sheet.KindStrip && n.Kind != sheet.KindBlock {
		return
	}
	v := n.Fill
	f.Check(kitOwner{t.s.k}, "sec.fill", t.s.a.T("Fill the space left over"), &v, "",
		t.s.a.T("Fill\nA stack or panel that fills takes a share of the room the card has left."))
	if v != n.Fill {
		t.edit(func(root *config.HUDNode) ([]int, error) {
			sheet.At(root, t.sel).Fill = v
			return t.sel, nil
		})
	}
}

// whoNames are the four targets' words, in sheet.WhoLetters order (c s p w).
func whoNames(tr lang.Tr) [4]struct {
	who       sheet.Who
	label, id string
} {
	return [4]struct {
		who       sheet.Who
		label, id string
	}{
		{sheet.WhoColonist, tr.T("Colonists"), "c"},
		{sheet.WhoStranger, tr.T("Prisoners, guests and raiders"), "s"},
		{sheet.WhoPrisoner, tr.T("Your animals"), "p"},
		{sheet.WhoWild, tr.T("Wild creatures"), "w"},
	}
}

func (t *Sections) targetRows(f *uikit.Form, o kitOwner, n config.HUDNode) {
	set := sheet.WhoSet(n)
	for _, w := range whoNames(t.s.a) {
		on := strings.Contains(set, w.id)
		v := on
		reason := ""
		if on && len(set) == 1 {
			reason = t.s.a.T("At least one kind of target must stay on.")
		}
		f.Check(o, "sec.who."+w.id, t.s.a.Tf("Shows for: %s", w.label), &v, reason, "")
		if v != on {
			who := w.who
			t.edit(func(root *config.HUDNode) ([]int, error) {
				return t.sel, sheet.SetWho(sheet.At(root, t.sel), who, v)
			})
		}
	}
}

func (t *Sections) defRows(f *uikit.Form, n config.HUDNode) {
	k := t.s.k
	if sheet.NeedsDef(n.Kind) {
		if row, ok := f.Row(secBtnH + 4); ok {
			cur := n.Def
			if d, ok := sheet.LookupDef(t.s.a, n.Kind, n.Def); ok {
				cur = d.Label
			}
			uikit.StyledButton(k, "sec.def", row.InsetXY(0, 1), t.s.a.Tf("Shows: %s", cur), uikit.BtnOpts{Tip: t.s.a.T("Which one\nChoose what this element shows.")}, func() {
				t.defMenu(n.Kind, func(key string) {
					t.edit(func(root *config.HUDNode) ([]int, error) {
						sheet.At(root, t.sel).Def = key
						return t.sel, nil
					})
				})
			})
		}
	}
	if sheet.IsBar(n.Kind) {
		if row, ok := f.Row(secBtnH + 4); ok {
			cur := t.s.a.T("Fades up")
			if n.BarStyle != "" {
				cur = sheet.BarStyleLabel(t.s.a, n.BarStyle)
			}
			uikit.StyledButton(k, "sec.style", row.InsetXY(0, 1), t.s.a.Tf("Colours: %s", cur), uikit.BtnOpts{Tip: t.s.a.T("Bar colours\nHow the bar changes colour as it fills.")}, func() {
				t.styleMenu(func(s string) {
					t.edit(func(root *config.HUDNode) ([]int, error) {
						sheet.At(root, t.sel).BarStyle = s
						return t.sel, nil
					})
				})
			})
		}
	}
}

// orderRow is fold, up, down and remove (RHUD-332, 335-337).
func (t *Sections) orderRow(f *uikit.Form, n config.HUDNode) {
	k := t.s.k
	root := len(t.sel) == 0
	last := len(t.sel) - 1
	var first, end bool
	if !root {
		parent := sheet.At(&t.work, t.sel[:last])
		first = t.sel[last] == 0
		end = parent == nil || t.sel[last] >= len(parent.Kids)-1
	}
	if len(n.Kids) > 0 {
		if row, ok := f.Row(secBtnH + 4); ok {
			label := t.s.a.T("Fold away what it holds")
			if t.collapsed[pathKey(t.sel)] {
				label = t.s.a.T("Show what it holds")
			}
			uikit.StyledButton(k, "sec.fold", row.InsetXY(0, 1), label, uikit.BtnOpts{}, func() {
				key := pathKey(t.sel)
				t.collapsed[key] = !t.collapsed[key]
				t.dirty = true
			})
		}
	}
	row, ok := f.Row(secBtnH + 4)
	if !ok {
		return
	}
	cols := row.InsetXY(0, 1).Columns(3, 4)
	top := t.s.a.T("The whole card cannot be moved or removed.")
	uikit.StyledButton(k, "sec.up", cols[0], t.s.a.T("Up"), uikit.BtnOpts{Disabled: root || first, Reason: ifElse(root, top, t.s.a.T("It is already first."))}, func() {
		t.edit(func(rt *config.HUDNode) ([]int, error) { p, _ := sheet.Move(rt, t.sel, -1); return p, nil })
	})
	uikit.StyledButton(k, "sec.down", cols[1], t.s.a.T("Down"), uikit.BtnOpts{Disabled: root || end, Reason: ifElse(root, top, t.s.a.T("It is already last."))}, func() {
		t.edit(func(rt *config.HUDNode) ([]int, error) { p, _ := sheet.Move(rt, t.sel, 1); return p, nil })
	})
	uikit.StyledButton(k, "sec.remove", cols[2], t.s.a.T("Remove"), uikit.BtnOpts{Style: uikit.BtnDanger, Disabled: root, Reason: top}, func() {
		up := append([]int(nil), t.sel[:last]...)
		t.edit(func(rt *config.HUDNode) ([]int, error) {
			if !sheet.Remove(rt, t.sel) {
				return t.sel, sheet.ErrNoSuchNode
			}
			return up, nil
		})
	})
}

func ifElse(c bool, a, b string) string {
	if c {
		return a
	}
	return b
}

func (t *Sections) addRow(f *uikit.Form) {
	k := t.s.k
	row, ok := f.Row(secBtnH + 4)
	if !ok {
		return
	}
	cols := row.InsetXY(0, 1).Columns(3, 4)
	uikit.StyledButton(k, "sec.add.layout", cols[0], t.s.a.T("Add layout..."), uikit.BtnOpts{Tip: t.s.a.T("Add layout\nStacks, panels, rows, blank space and dividers.")}, func() {
		t.addMenu(t.s.a.T("Add a layout element"), "layout")
	})
	uikit.StyledButton(k, "sec.add.value", cols[1], t.s.a.T("Add value..."), uikit.BtnOpts{Tip: t.s.a.T("Add a value\nA line of words, a menu, a skill, a training, a stat or a record.")}, func() {
		t.addMenu(t.s.a.T("Add a value"), "values", "pickers", "generic")
	})
	uikit.StyledButton(k, "sec.add.bar", cols[2], t.s.a.T("Add bar..."), uikit.BtnOpts{Tip: t.s.a.T("Add a bar\nA need or health bar, then how it is coloured.")}, func() {
		t.addMenu(t.s.a.T("Add a bar"), "bars")
	})
}

// useDefault drops this mode's tree so the built-in one is used.
func (t *Sections) useDefault() {
	docked := t.docked
	t.s.Edit(func(h *config.HUD) {
		if docked {
			h.Layouts.Docked = nil
		} else {
			h.Layouts.Floating = nil
		}
	})
	t.s.a.Hint(t.s.a.T("The built-in layout is back."))
}

// edit applies f to a clone of the tree; if the result is no worse than it was
// it becomes the working copy and goes into the settings, else a reason shows.
func (t *Sections) edit(f func(root *config.HUDNode) ([]int, error)) {
	w := cloneNode(t.work)
	before := len(sheet.Validate(t.s.a, w))
	np, err := f(&w)
	if err == nil {
		if ps := sheet.Validate(t.s.a, w); len(ps) > before {
			err = errors.New(ps[before].Text)
		}
	}
	if err != nil {
		if errors.Is(err, errNoHolder) {
			t.s.a.Hint(t.s.a.T("Nothing selected can hold that. Select a stack or panel first."))
			return
		}
		t.s.a.Hint(err.Error())
		return
	}
	t.work, t.sel, t.collapsed, t.dirty = w, np, map[string]bool{}, true
	t.commit()
}

// commit writes the working copy into the settings (a copy equal to the
// built-in tree is stored as nil).
func (t *Sections) commit() {
	docked := t.docked
	out := cloneNode(t.work)
	var stored *config.HUDNode
	if !reflect.DeepEqual(out, defaultOf(docked)) {
		stored = &out
	}
	t.s.Edit(func(h *config.HUD) {
		if docked {
			h.Layouts.Docked = stored
		} else {
			h.Layouts.Floating = stored
		}
	})
	t.seen = stored
}

// place puts n where the selection allows: inside it, else after it in the
// nearest ancestor that can hold it.
func (t *Sections) place(root *config.HUDNode, n config.HUDNode) ([]int, error) {
	if at := sheet.At(root, t.sel); at != nil && sheet.CanHold(at.Kind, n.Kind) {
		return sheet.Add(root, t.sel, len(at.Kids), n)
	}
	for i := len(t.sel) - 1; i >= 0; i-- {
		parent := t.sel[:i]
		if p := sheet.At(root, parent); p != nil && sheet.CanHold(p.Kind, n.Kind) {
			return sheet.Add(root, parent, t.sel[i]+1, n)
		}
	}
	return nil, errNoHolder
}

// errNoHolder is an addition with nowhere to go; the player reads a hint instead.
var errNoHolder = errors.New("no selected node can hold the addition")

type menuItem struct {
	label, tip, off string
}

// menu opens a list of choices with a hover card on each (RHUD-341, 348).
func (t *Sections) menu(id, title string, items []menuItem, pick func(i int)) {
	k := t.s.k
	h := float32(uikit.ModalTitleH+2*uikit.ModalPad) + min(float32(len(items)), 12)*uikit.ListRowH + 4
	mid := "pick." + id
	k.Push(uikit.Modal{
		ID: mid, Title: title, W: 380, H: h,
		Draw: func(body gfx.Rect) {
			choice := -1
			changed := k.List("modal."+mid+".list", body, len(items), &choice, uikit.ListSpec{
				Label:    func(i int) string { return items[i].label },
				Tip:      func(i int) string { return items[i].label + "\n" + items[i].tip },
				Disabled: func(i int) string { return items[i].off },
			})
			if changed && choice >= 0 {
				k.CloseModal(mid)
				pick(choice)
			}
		},
	})
}

// addMenu lists the catalog entries of the groups and adds the chosen one.
func (t *Sections) addMenu(title string, groups ...string) {
	var keys []string
	var items []menuItem
	for _, g := range groups {
		for _, e := range sheet.Catalog(t.s.a) {
			if e.Group != g {
				continue
			}
			it := menuItem{label: e.Label, tip: e.Tip}
			if e.Reserved {
				it.off = "That shows nothing until the game has clothing policies."
			}
			keys = append(keys, e.Key)
			items = append(items, it)
		}
	}
	t.menu("sec.add", title, items, func(i int) { t.addKey(keys[i]) })
}

// addKey adds an element, asking for its definition and bar colours first.
func (t *Sections) addKey(key string) {
	n := sheet.NewNode(key)
	finish := func() {
		t.edit(func(root *config.HUDNode) ([]int, error) { return t.place(root, n) })
	}
	style := func() {
		if !sheet.IsBar(key) {
			finish()
			return
		}
		t.styleMenu(func(s string) { n.BarStyle = s; finish() })
	}
	if !sheet.NeedsDef(key) {
		style()
		return
	}
	t.defMenu(key, func(d string) { n.Def = d; style() })
}

func (t *Sections) defMenu(key string, pick func(def string)) {
	defs := sheet.Defs(t.s.a, key)
	items := make([]menuItem, len(defs))
	for i, d := range defs {
		items[i] = menuItem{label: d.Label, tip: t.s.a.Tf("Key: %s. Built in.", key+" / "+d.Key)}
	}
	t.menu("sec.def", t.s.a.T("Which one?"), items, func(i int) { pick(defs[i].Key) })
}

func (t *Sections) styleMenu(pick func(style string)) {
	styles := sheet.BarStyles()
	items := make([]menuItem, len(styles))
	for i, s := range styles {
		items[i] = menuItem{label: sheet.BarStyleLabel(t.s.a, s), tip: t.s.a.Tf("Key: %s.", s)}
	}
	t.menu("sec.style", t.s.a.T("Bar colours"), items, func(i int) { pick(styles[i]) })
}
