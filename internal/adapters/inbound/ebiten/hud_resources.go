package ebiten

import (
	"image/color"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/economy/item"
)

// The resource list (PLAY-018: HUD-TOP owns it): what the colony holds in
// its stockpiles as rows of icon + count, top-left, in the layout's
// Resources rect. With Gameplay.CategorizedResources the rows sit under
// collapsible category headers. The list scrolls (mouse wheel, scrollbar)
// when it is taller than its rect, and the layout hides it when less than
// resourcesMin px are left.

const (
	resIconSize  = 16
	resScrollID  = "res.scroll"
	resCatPrefix = "res.cat." // a category header: toggles its group
	resRowPrefix = "res.row." // an item kind's row: hover shows its label
	resPad       = 4
)

// resRow is one line of the list: a category header or an item row.
type resRow struct {
	header bool
	cat    string // the category (a header's own name)
	kind   string // item kind key (rows)
	label  string
	qty    int // a header's qty is its group's total
}

// resKey is what the rows were built from: the same key means the same rows.
type resKey struct {
	first                                  *game.StoredResourceView
	n, sum                                 int
	cat                                    bool
	ver                                    int
	food, wood, stone, steel, silver, gold int
}

// resourcesState is this region's presentation state (App.hst).
type resourcesState struct {
	collapsed  map[string]bool // folded category headers
	ver        int             // bumped when collapsed changes
	key        resKey
	rows       []resRow
	built      bool
	widths     map[string]float32 // count text widths (the plates), at widthScale
	widthScale float64
}

// ---- rows ------------------------------------------------------------------

// legacyStored are the readouts the HUD showed before the read model had
// stockpile counts: the loose-stack totals (food, wood, stone, steel, and
// silver and gold once mined). They stand in while Stored is empty, so the
// list never goes blank.
func legacyStored(w game.WorldView) []game.StoredResourceView {
	out := []game.StoredResourceView{
		{Kind: "food", Label: "Food", Category: "Food", Qty: w.Food},
		{Kind: "wood", Label: "Wood", Category: "Materials", Qty: w.Wood},
		{Kind: "stone", Label: "Stone", Category: "Materials", Qty: w.Stone},
		{Kind: "steel", Label: "Steel", Category: "Materials", Qty: w.Steel},
	}
	if w.Silver > 0 {
		out = append(out, game.StoredResourceView{Kind: "silver", Label: "Silver", Category: "Materials", Qty: w.Silver})
	}
	if w.Gold > 0 {
		out = append(out, game.StoredResourceView{Kind: "gold", Label: "Gold", Category: "Materials", Qty: w.Gold})
	}
	return out
}

// buildResourceRows is the list for w: stored kinds with stock (the totals
// when there are none), flat, or grouped under headers in category order.
// A group in collapsed shows only its header.
func buildResourceRows(w game.WorldView, categorised bool, collapsed map[string]bool) []resRow {
	src := make([]game.StoredResourceView, 0, len(w.Stored))
	for _, s := range w.Stored {
		if s.Qty > 0 {
			src = append(src, s)
		}
	}
	if len(src) == 0 {
		src = legacyStored(w)
	}
	rows := make([]resRow, 0, len(src)+8)
	if !categorised {
		for _, s := range src {
			rows = append(rows, resRow{cat: s.Category, kind: s.Kind, label: s.Label, qty: s.Qty})
		}
		return rows
	}
	slices.SortStableFunc(src, func(a, b game.StoredResourceView) int {
		if d := item.CategoryRank(a.Category) - item.CategoryRank(b.Category); d != 0 {
			return d
		}
		return strings.Compare(a.Category, b.Category) // unknown categories, by name
	})
	for i := 0; i < len(src); {
		j := i
		total := 0
		for ; j < len(src) && src[j].Category == src[i].Category; j++ {
			total += src[j].Qty
		}
		cat := src[i].Category
		rows = append(rows, resRow{header: true, cat: cat, label: cat, qty: total})
		if !collapsed[cat] {
			for _, s := range src[i:j] {
				rows = append(rows, resRow{cat: cat, kind: s.Kind, label: s.Label, qty: s.Qty})
			}
		}
		i = j
	}
	return rows
}

// resourceRows are the list's rows now, rebuilt only when what they come
// from changes (the HUD asks for them many times a frame).
func (a *App) resourceRows() []resRow {
	if a.renderer == nil {
		return nil
	}
	w := a.shell.World()
	st := &a.hst.res
	k := resKey{
		n: len(w.Stored), cat: a.settings.Gameplay.CategorizedResources, ver: st.ver,
		food: w.Food, wood: w.Wood, stone: w.Stone, steel: w.Steel, silver: w.Silver, gold: w.Gold,
	}
	if len(w.Stored) > 0 {
		k.first = &w.Stored[0]
		for _, s := range w.Stored {
			k.sum += s.Qty
		}
	}
	if st.built && st.key == k {
		return st.rows
	}
	st.key, st.built = k, true
	st.rows = buildResourceRows(w, k.cat, st.collapsed)
	return st.rows
}

// resourcesHeight is the list's content height the layout reserves. It
// feeds the layout, so it never calls a.hud().
func (a *App) resourcesHeight() float32 {
	return float32(len(a.resourceRows())) * resourceRowH
}

// ---- viewport --------------------------------------------------------------

// resourceViewport is where the rows go inside r: the whole rect when the
// content fits, else the rect less the scrollbar; and the scroll offset.
func (a *App) resourceViewport(r rect, contentH float32) (view rect, off float32) {
	if contentH <= r.H {
		return r, 0
	}
	view = rect{X: r.X, Y: r.Y, W: max(0, r.W-scrollBarW), H: r.H}
	off = max(0, min(contentH-r.H, a.uiSt().scroll[resScrollID]))
	return view, off
}

// resourcesUI declares the scroll view when the list is taller than its
// rect: it owns the mouse wheel over the rect (the camera does not zoom).
func (a *App) resourcesUI(sw, sh int) {
	if a.renderer == nil {
		return
	}
	r := a.hud().Resources
	if ch := a.resourcesHeight(); !r.Empty() && ch > r.H {
		a.uiScrollArea(resScrollID, r, ch, scrollOpts{Step: resourceRowH * 3})
		return
	}
	delete(a.uiSt().scroll, resScrollID)
}

// ---- buttons ---------------------------------------------------------------

// resourceButtons are the visible rows as hover areas: a row's tip is its
// label; a header toggles its group.
func (a *App) resourceButtons() []button {
	if a.renderer == nil {
		return nil
	}
	r := a.hud().Resources
	if r.Empty() {
		return nil
	}
	rows := a.resourceRows()
	view, off := a.resourceViewport(r, float32(len(rows))*resourceRowH)
	out := make([]button, 0, len(rows))
	for i, row := range rows {
		rr := rect{X: view.X, Y: view.Y + float32(i)*resourceRowH - off, W: view.W, H: resourceRowH}.Intersect(view)
		if rr.Empty() {
			continue
		}
		b := button{x: rr.X, y: rr.Y, w: rr.W, h: rr.H}
		if row.header {
			b.id = resCatPrefix + row.cat
			b.tip = row.cat + " (click to fold)"
			if a.hst.res.collapsed[row.cat] {
				b.tip = row.cat + " (click to unfold)"
			}
		} else {
			b.id, b.tip = resRowPrefix+row.kind, row.label
		}
		out = append(out, b)
	}
	return out
}

// applyResourceClick handles a "res." click: a header folds or unfolds
// its group; a row has no click action.
func (a *App) applyResourceClick(id string) {
	cat, ok := strings.CutPrefix(id, resCatPrefix)
	if !ok {
		return
	}
	st := &a.hst.res
	if st.collapsed == nil {
		st.collapsed = map[string]bool{}
	}
	st.collapsed[cat] = !st.collapsed[cat]
	st.ver++
}

// rightClickResources answers a right-click over the list: it is the HUD's,
// not an order on the map under it.
func (a *App) rightClickResources(mx, my int) bool {
	if a.renderer == nil {
		return false
	}
	r := a.hud().Resources
	return !r.Empty() && r.Hit(mx, my)
}

// ---- drawing ---------------------------------------------------------------

// resourceIconKey is the art key of kind's HUD icon ("granite blocks" ->
// "ui_icon/granite_blocks").
func resourceIconKey(kind string) string {
	return "ui_icon/" + strings.ReplaceAll(kind, " ", "_")
}

// resourceIcon is kind's icon: its ui_icon, else its item sprite (the
// item inspector's fallback). nil when the art has neither.
func (a *App) resourceIcon(kind string) (*ebiten.Image, colorScale) {
	art := a.renderer.art
	if art == nil {
		return nil, noCS
	}
	key, cs := resourceIconKey(kind), noCS
	if !art.Has(key) {
		key, cs = a.renderer.stoneItem(itemSpriteSpecies(kind, "", 0, 0), kind, noCS)
	}
	return art.Image(key, color.NRGBA{}), cs
}

// resWidth is the logical width of count text s (cached: the same few
// strings are drawn every frame).
func (a *App) resWidth(s string) float32 {
	st := &a.hst.res
	if sc := hudScale(); sc != st.widthScale || st.widths == nil || len(st.widths) > 512 {
		st.widths, st.widthScale = map[string]float32{}, sc // text metrics follow the HUD scale (world pass or not)
	}
	if w, ok := st.widths[s]; ok {
		return w
	}
	w := textW(s, tsBody)
	st.widths[s] = w
	return w
}

// drawResources draws the resource list in the layout's rect.
func (a *App) drawResources(screen *ebiten.Image, sw, sh int) {
	if a.renderer == nil {
		return
	}
	r := a.hud().Resources
	if r.Empty() {
		return
	}
	rows := a.resourceRows()
	contentH := float32(len(rows)) * resourceRowH
	view, off := a.resourceViewport(r, contentH)
	dst := screen
	if contentH > r.H {
		dst = clipTo(screen, view)
	}
	mx, my := a.currentCursor()
	for i, row := range rows {
		line := rect{X: view.X, Y: view.Y + float32(i)*resourceRowH - off, W: view.W, H: resourceRowH}
		if line.Bottom() <= view.Y || line.Y >= view.Bottom() {
			continue
		}
		hover := view.Hit(mx, my) && line.Hit(mx, my)
		if row.header {
			a.drawResourceHeader(dst, line, row, hover)
		} else {
			a.drawResourceRow(dst, line, row, hover)
		}
	}
}

// resPlate is the translucent plate behind a readout, so it reads over any
// terrain.
func resPlate(hover bool) color.RGBA {
	if hover {
		return fade(uiBtnHover, 0.9)
	}
	return fade(uiSpace, 0.62)
}

// drawResourceRow draws an item row: icon, then the count, on a plate that
// is as wide as they need.
func (a *App) drawResourceRow(dst *ebiten.Image, line rect, row resRow, hover bool) {
	count := strconv.Itoa(row.qty)
	plateW := min(line.W, resPad+resIconSize+resPad+a.resWidth(count)+resPad+2)
	uiFillRect(dst, line.X, line.Y+1, plateW, line.H-2, resPlate(hover), false)
	ix, iy := line.X+resPad, line.Y+(line.H-resIconSize)/2
	if img, cs := a.resourceIcon(row.kind); img != nil {
		drawIconCS(dst, img, float64(ix), float64(iy), resIconSize, cs)
	} else { // no art for it: a letter tile
		uiFillRect(dst, ix, iy, resIconSize, resIconSize, uiBtnDisabled, false)
		uiStrokeRect(dst, ix, iy, resIconSize, resIconSize, 1, uiBtnBorder, false)
		if l := firstLetter(row.label); l != "" {
			drawCentered(dst, l, float64(ix+resIconSize/2), float64(iy+resIconSize/2), withColor(tsSmall, uiText))
		}
	}
	drawText(dst, count, float64(ix+resIconSize+resPad), float64(line.Y)+(float64(line.H)-lineH(tsBody))/2, tsBody)
}

// drawResourceHeader draws a category header: a chevron, the name and the
// group's total.
func (a *App) drawResourceHeader(dst *ebiten.Image, line rect, row resRow, hover bool) {
	uiFillRect(dst, line.X, line.Y+1, line.W, line.H-2, resPlate(hover), false)
	cx, cy := line.X+resPad+4, line.Y+line.H/2
	chev := [][2]float32{{cx - 4, cy - 3}, {cx + 4, cy - 3}, {cx, cy + 3}} // open: points down
	if a.hst.res.collapsed[row.cat] {
		chev = [][2]float32{{cx - 3, cy - 4}, {cx + 3, cy}, {cx - 3, cy + 4}} // folded: points right
	}
	uiFillPoly(dst, chev, uiAccent, false)
	st := withColor(tsLabel, uiHeading)
	drawText(dst, ellipsize(row.label, st, float64(line.W-2*resPad-48)), float64(line.X+resPad+14), float64(line.Y)+(float64(line.H)-lineH(st))/2, st)
	tot := withColor(tsSmall, uiTextDim)
	tot.Align = alignRight
	drawText(dst, strconv.Itoa(row.qty), float64(line.Right()-resPad-2), float64(line.Y)+(float64(line.H)-lineH(tot))/2, tot)
}

// firstLetter is the first rune of s, upper case ("" for none).
func firstLetter(s string) string {
	r, n := utf8.DecodeRuneInString(s)
	if n == 0 {
		return ""
	}
	return strings.ToUpper(string(r))
}
