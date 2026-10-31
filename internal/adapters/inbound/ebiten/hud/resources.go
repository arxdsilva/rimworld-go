package hud

import (
	"image/color"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/render/creature"
	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/adapters/inbound/ebiten/render/world"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/economy/item"

	ebiten "github.com/hajimehoshi/ebiten/v2"
)

// The resource list (PLAY-018: HUD-TOP owns it): what the colony holds in
// its stockpiles as rows of icon + count, top-left, in the layout's
// Resources rect. With Gameplay.CategorizedResources the rows sit under
// collapsible category headers. The list scrolls (mouse wheel, scrollbar)
// when it is taller than its rect, and the layout hides it when less than
// resourcesMin px are left.

const (
	resIconSize  = 16
	ResScrollID  = "res.scroll"
	resCatPrefix = "res.cat." // a category header: toggles its group
	resRowPrefix = "res.row." // an item kind's row: hover shows its label
	resPad       = 4
)

// ResRow is one line of the list: a category header or an item row.
type ResRow struct {
	Header bool
	Cat    string // the category (a header's own name)
	Kind   string // item kind key (rows)
	Label  string
	Qty    int    // a header's qty is its group's total
	Text   string // when set, the count text instead of Qty (the days-of-food row)
}

// resKey is what the rows were built from: the same key means the same rows.
type resKey struct {
	first                                  *game.StoredResourceView
	n, sum                                 int
	cat                                    bool
	ver                                    int
	food, wood, stone, steel, silver, gold int
	foodDays                               int
}

// resourcesState is this region's presentation state (Chrome.res).
type resourcesState struct {
	collapsed  map[string]bool // folded category headers
	ver        int             // bumped when collapsed changes
	key        resKey
	rows       []ResRow
	built      bool
	widths     map[string]float32 // count text widths (the plates), at widthScale
	widthScale float64
}

// FoodDaysKind is the days-of-food row's kind (its icon is the food one).
const FoodDaysKind = "food_days"

// FoodDaysRow is the readout row of days (tenths): "3.4 d" with the hover
// label "Days of food: 3.4"; ok is false while the colony has no food
// reading (FoodDays 0 before the simulation fills it).
func FoodDaysRow(tenths int) (ResRow, bool) {
	if tenths <= 0 {
		return ResRow{}, false
	}
	days := strconv.Itoa(tenths/10) + "." + strconv.Itoa(tenths%10)
	return ResRow{Kind: FoodDaysKind, Label: "Days of food: " + days, Text: days + " d"}, true
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

// BuildResourceRows is the list for w: stored kinds with stock (the totals
// when there are none), flat, or grouped under headers in category order.
// A group in collapsed shows only its header.
func BuildResourceRows(w game.WorldView, categorised bool, collapsed map[string]bool) []ResRow {
	src := make([]game.StoredResourceView, 0, len(w.Stored))
	for _, s := range w.Stored {
		if s.Qty > 0 {
			src = append(src, s)
		}
	}
	if len(src) == 0 {
		src = legacyStored(w)
	}
	rows := make([]ResRow, 0, len(src)+8)
	if !categorised {
		for _, s := range src {
			rows = append(rows, ResRow{Cat: s.Category, Kind: s.Kind, Label: s.Label, Qty: s.Qty})
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
		rows = append(rows, ResRow{Header: true, Cat: cat, Label: cat, Qty: total})
		if !collapsed[cat] {
			for _, s := range src[i:j] {
				rows = append(rows, ResRow{Cat: cat, Kind: s.Kind, Label: s.Label, Qty: s.Qty})
			}
		}
		i = j
	}
	return rows
}

// Rows are the list's rows now, rebuilt only when what they come
// from changes (the HUD asks for them many times a frame).
func (c Resources) Rows() []ResRow {
	if !c.live() {
		return nil
	}
	w := c.sh.World()
	st := &c.res
	k := resKey{
		n: len(w.Stored), cat: c.sh.Settings().Gameplay.CategorizedResources, ver: st.ver,
		food: w.Food, wood: w.Wood, stone: w.Stone, steel: w.Steel, silver: w.Silver, gold: w.Gold, foodDays: w.FoodDays,
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
	st.rows = BuildResourceRows(w, k.cat, st.collapsed)
	if r, ok := FoodDaysRow(w.FoodDays); ok {
		st.rows = append([]ResRow{r}, st.rows...)
	}
	return st.rows
}

// Height is the list's content height the layout reserves. It feeds the
// layout, so it never asks for it.
func (c Resources) Height() float32 {
	return float32(len(c.Rows())) * hudlayout.ResourceRowH
}

// ---- viewport --------------------------------------------------------------

// viewport is where the rows go inside r: the whole rect when the
// content fits, else the rect less the scrollbar; and the scroll offset.
func (c Resources) viewport(r gfx.Rect, contentH float32) (view gfx.Rect, off float32) {
	if contentH <= r.H {
		return r, 0
	}
	view = gfx.Rect{X: r.X, Y: r.Y, W: max(0, r.W-uikit.ScrollBarW), H: r.H}
	off = max(0, min(contentH-r.H, c.kit().Scroll[ResScrollID]))
	return view, off
}

// Declare declares the scroll view when the list is taller than its
// rect: it owns the mouse wheel over the rect (the camera does not zoom).
func (c Resources) Declare(sw, sh int) {
	if !c.live() {
		return
	}
	r := c.sh.Layout().Resources
	if ch := c.Height(); !r.Empty() && ch > r.H {
		c.kit().ScrollArea(ResScrollID, r, ch, uikit.ScrollOpts{Step: hudlayout.ResourceRowH * 3})
		return
	}
	delete(c.kit().Scroll, ResScrollID)
}

// ---- buttons ---------------------------------------------------------------

// Buttons are the visible rows as hover areas: a row's tip is its
// label; a header toggles its group.
func (c Resources) Buttons() []uikit.Button {
	if !c.live() {
		return nil
	}
	r := c.sh.Layout().Resources
	if r.Empty() {
		return nil
	}
	rows := c.Rows()
	view, off := c.viewport(r, float32(len(rows))*hudlayout.ResourceRowH)
	out := make([]uikit.Button, 0, len(rows))
	for i, row := range rows {
		rr := gfx.Rect{X: view.X, Y: view.Y + float32(i)*hudlayout.ResourceRowH - off, W: view.W, H: hudlayout.ResourceRowH}.Intersect(view)
		if rr.Empty() {
			continue
		}
		b := uikit.Button{X: rr.X, Y: rr.Y, W: rr.W, H: rr.H}
		if row.Header {
			b.ID = resCatPrefix + row.Cat
			b.Tip = row.Cat + " (click to fold)"
			if c.res.collapsed[row.Cat] {
				b.Tip = row.Cat + " (click to unfold)"
			}
		} else {
			b.ID, b.Tip = resRowPrefix+row.Kind, row.Label
		}
		out = append(out, b)
	}
	return out
}

// Click handles a "res." click: a header folds or unfolds
// its group; a row has no click action.
func (c Resources) Click(id string) {
	cat, ok := strings.CutPrefix(id, resCatPrefix)
	if !ok {
		return
	}
	st := &c.res
	st.collapsed[cat] = !st.collapsed[cat]
	st.ver++
}

// RightClick answers a right-click over the list: it is the HUD's,
// not an order on the map under it.
func (c Resources) RightClick(mx, my int) bool {
	if !c.live() {
		return false
	}
	r := c.sh.Layout().Resources
	return !r.Empty() && r.Hit(mx, my)
}

// ---- drawing ---------------------------------------------------------------

// ResourceIconKey is the art key of kind's HUD icon ("granite blocks" ->
// "ui_icon/granite_blocks").
func ResourceIconKey(kind string) string {
	return "ui_icon/" + strings.ReplaceAll(kind, " ", "_")
}

// Icon is kind's icon: its ui_icon, else its item sprite (the
// item inspector's fallback). nil when the art has neither.
func (c Resources) Icon(kind string) (*ebiten.Image, sprite.ColorScale) {
	cat := c.sh.Env().Art
	if cat == nil {
		return nil, sprite.NoCS
	}
	if kind == FoodDaysKind {
		kind = "food"
	}
	key, cs := ResourceIconKey(kind), sprite.NoCS
	if !cat.Has(key) {
		key, cs = c.sh.StoneItem(world.ItemSpriteSpecies(kind, "", 0, 0), kind, sprite.NoCS)
	}
	return cat.Image(key, color.NRGBA{}), cs
}

// Collapsed is the folded category headers (the map the region reads and
// toggles; the root's journey test peeks at it).
func (c Resources) Collapsed() map[string]bool { return c.res.collapsed }

// width is the logical width of count text s (cached: the same few
// strings are drawn every frame).
func (c Resources) width(s string) float32 {
	st := &c.res
	if sc := gfx.HUDScale(); sc != st.widthScale || st.widths == nil || len(st.widths) > 512 {
		st.widths, st.widthScale = map[string]float32{}, sc // text metrics follow the HUD scale (world pass or not)
	}
	if w, ok := st.widths[s]; ok {
		return w
	}
	w := gfx.TextW(s, gfx.Body)
	st.widths[s] = w
	return w
}

// Draw draws the resource list in the layout's rect.
func (c Resources) Draw(screen *ebiten.Image, sw, sh int) {
	if !c.live() {
		return
	}
	r := c.sh.Layout().Resources
	if r.Empty() {
		return
	}
	rows := c.Rows()
	contentH := float32(len(rows)) * hudlayout.ResourceRowH
	view, off := c.viewport(r, contentH)
	dst := screen
	if contentH > r.H {
		dst = gfx.ClipTo(screen, view)
	}
	mx, my := c.sh.Cursor()
	for i, row := range rows {
		line := gfx.Rect{X: view.X, Y: view.Y + float32(i)*hudlayout.ResourceRowH - off, W: view.W, H: hudlayout.ResourceRowH}
		if line.Bottom() <= view.Y || line.Y >= view.Bottom() {
			continue
		}
		hover := view.Hit(mx, my) && line.Hit(mx, my)
		if row.Header {
			c.drawHeader(dst, line, row, hover)
		} else {
			c.drawRow(dst, line, row, hover)
		}
	}
}

// resPlate is the translucent plate behind a readout, so it reads over any
// terrain.
func resPlate(hover bool) color.RGBA {
	if hover {
		return creature.Fade(uikit.BtnHover, 0.9)
	}
	return creature.Fade(uikit.Space, 0.62)
}

// drawRow draws an item row: icon, then the count, on a plate that
// is as wide as they need.
func (c Resources) drawRow(dst *ebiten.Image, line gfx.Rect, row ResRow, hover bool) {
	count := strconv.Itoa(row.Qty)
	if row.Text != "" {
		count = row.Text
	}
	plateW := min(line.W, resPad+resIconSize+resPad+c.width(count)+resPad+2)
	gfx.FillRect(dst, line.X, line.Y+1, plateW, line.H-2, resPlate(hover), false)
	ix, iy := line.X+resPad, line.Y+(line.H-resIconSize)/2
	if img, cs := c.Icon(row.Kind); img != nil {
		drawIconCS(dst, img, float64(ix), float64(iy), resIconSize, cs)
	} else { // no art for it: a letter tile
		gfx.FillRect(dst, ix, iy, resIconSize, resIconSize, uikit.BtnDisabled, false)
		gfx.StrokeRect(dst, ix, iy, resIconSize, resIconSize, 1, uikit.BtnBorder, false)
		if l := firstLetter(row.Label); l != "" {
			uikit.DrawCentered(dst, l, float64(ix+resIconSize/2), float64(iy+resIconSize/2), gfx.WithColor(gfx.Small, uikit.Text))
		}
	}
	gfx.Text(dst, count, float64(ix+resIconSize+resPad), float64(line.Y)+(float64(line.H)-gfx.LineH(gfx.Body))/2, gfx.Body)
}

// drawHeader draws a category header: a chevron, the name and the
// group's total.
func (c Resources) drawHeader(dst *ebiten.Image, line gfx.Rect, row ResRow, hover bool) {
	gfx.FillRect(dst, line.X, line.Y+1, line.W, line.H-2, resPlate(hover), false)
	cx, cy := line.X+resPad+4, line.Y+line.H/2
	chev := [][2]float32{{cx - 4, cy - 3}, {cx + 4, cy - 3}, {cx, cy + 3}} // open: points down
	if c.res.collapsed[row.Cat] {
		chev = [][2]float32{{cx - 3, cy - 4}, {cx + 3, cy}, {cx - 3, cy + 4}} // folded: points right
	}
	gfx.FillPoly(dst, chev, uikit.Accent, false)
	st := gfx.WithColor(gfx.Label, uikit.Heading)
	gfx.Text(dst, gfx.Ellipsize(row.Label, st, float64(line.W-2*resPad-48)), float64(line.X+resPad+14), float64(line.Y)+(float64(line.H)-gfx.LineH(st))/2, st)
	tot := gfx.WithColor(gfx.Small, uikit.TextDim)
	tot.Align = gfx.AlignRight
	gfx.Text(dst, strconv.Itoa(row.Qty), float64(line.Right()-resPad-2), float64(line.Y)+(float64(line.H)-gfx.LineH(tot))/2, tot)
}

// firstLetter is the first rune of s, upper case ("" for none).
func firstLetter(s string) string {
	r, n := utf8.DecodeRuneInString(s)
	if n == 0 {
		return ""
	}
	return strings.ToUpper(string(r))
}
