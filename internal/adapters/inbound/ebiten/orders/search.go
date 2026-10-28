package orders

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/adapters/inbound/ebiten/selection"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
)

// Search (UI-016, Z; MAIN_UI §55): a text box over the map. Typing is the
// text context, so no hotkey fires while it is open. Results come from
// the read models (colonists, animals by name or species, items by kind,
// buildings, plants, blueprints), best match first; Up/Down and Enter or
// a click select the thing and centre the camera on it; Esc closes.

// Search is the search box's state.
type Search struct {
	Open bool
	Text string
	Idx  int
	// cache of the last result list
	hits    []Hit
	hitText string
	hitTick clock.Tick
	hitSet  bool
}

// Hit is one result.
type Hit struct {
	Label string
	Kind  string
	C     selection.Cand
	X, Y  int
	Rank  int
}

const (
	searchW       = 440
	searchMinW    = 200 // the box never gets narrower, however small the window
	searchMaxHits = 8
	searchRowH    = 22
	searchMaxLen  = 32
)

var searchKindOrder = map[selection.Kind]int{selection.Pawn: 0, selection.Animal: 1, selection.Item: 2, selection.Building: 3, selection.Frame: 4, selection.Plant: 5}

// OpenSearch opens the box.
func (c *Controller) OpenSearch() {
	c.Search = Search{Open: true}
}

// CloseSearch closes the box.
func (c *Controller) CloseSearch() { c.Search = Search{} }

// matchRank is how well query q matches label: 0 prefix, 1 a word's
// prefix, 2 anywhere, -1 not at all.
func matchRank(label, q string) int {
	l := strings.ToLower(label)
	switch {
	case strings.HasPrefix(l, q):
		return 0
	case strings.Contains(l, " "+q), strings.Contains(l, "_"+q):
		return 1
	case strings.Contains(l, q):
		return 2
	}
	return -1
}

// searchResults ranks what matches q in the read models.
func (c *Controller) searchResults(w game.WorldView, pawns []game.PawnView, q string) []Hit {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return nil
	}
	var out []Hit
	add := func(label, kind string, c selection.Cand, x, y int, names ...string) {
		best := -1
		for _, n := range append([]string{label}, names...) {
			if r := matchRank(n, q); r >= 0 && (best < 0 || r < best) {
				best = r
			}
		}
		if best >= 0 {
			out = append(out, Hit{Label: label, Kind: kind, C: c, X: x, Y: y, Rank: best})
		}
	}
	for _, p := range pawns {
		if !p.Embodied {
			add(pawnLabel(p), "colonist", selection.Cand{Kind: selection.Pawn, ID: p.ID}, p.X, p.Y)
		}
	}
	for _, an := range w.Animals {
		label := titleCase(an.Label)
		if an.Name != "" {
			label = an.Name + " (" + an.Label + ")"
		}
		add(label, "animal", selection.Cand{Kind: selection.Animal, ID: an.ID}, an.X, an.Y, an.Label, an.Species)
	}
	for _, it := range w.Items {
		if !it.Carried {
			add(fmt.Sprintf("%s x %d", titleCase(it.Kind), it.Qty), "item", selection.Cand{Kind: selection.Item, ID: it.ID}, it.X, it.Y)
		}
	}
	for _, b := range w.Buildings {
		add(c.sh.BuildingTitle(b), "building", selection.Cand{Kind: selection.Building, ID: b.ID}, b.X, b.Y, b.Kind)
	}
	for _, f := range w.Frames {
		add(titleCase(f.Kind)+" blueprint", "blueprint", selection.Cand{Kind: selection.Frame, ID: f.ID}, f.X, f.Y)
	}
	for _, p := range w.Plants {
		add(titleCase(strings.ReplaceAll(p.Species, "_", " ")), "plant", selection.Cand{Kind: selection.Plant, ID: p.ID}, p.X, p.Y)
	}
	sort.SliceStable(out, func(i, j int) bool {
		hi, hj := out[i], out[j]
		if hi.Rank != hj.Rank {
			return hi.Rank < hj.Rank
		}
		if ki, kj := searchKindOrder[hi.C.Kind], searchKindOrder[hj.C.Kind]; ki != kj {
			return ki < kj
		}
		if hi.Label != hj.Label {
			return hi.Label < hj.Label
		}
		return hi.C.ID < hj.C.ID
	})
	if len(out) > searchMaxHits {
		out = out[:searchMaxHits]
	}
	return out
}

// searchHits is the cached result list for the current text.
// SearchHits is the cached result list for the current text.
func (c *Controller) SearchHits() []Hit {
	s := &c.Search
	tick := c.sh.Tick()
	if !s.hitSet || s.hitText != s.Text || s.hitTick != tick {
		s.hits = c.searchResults(c.sh.World(), c.sh.Pawns(), s.Text)
		s.hitText, s.hitTick, s.hitSet = s.Text, tick, true
	}
	return s.hits
}

// searchTyping is the search box's text context: characters, Backspace,
// Up/Down, Enter and Esc.
func (c *Controller) SearchTyping() {
	f, _ := c.sh.Frame()
	s := &c.Search
	for _, r := range f.Raw.Chars {
		if unicode.IsPrint(r) && utf8.RuneCountInString(s.Text) < searchMaxLen {
			s.Text += string(r)
			s.Idx = 0
		}
	}
	if f.Key("Backspace") && s.Text != "" {
		_, n := utf8.DecodeLastRuneInString(s.Text)
		s.Text = s.Text[:len(s.Text)-n]
		s.Idx = 0
	}
	hits := c.SearchHits()
	switch {
	case f.Key("Escape"):
		c.CloseSearch()
	case f.Key("ArrowDown") && len(hits) > 0:
		s.Idx = (s.Idx + 1) % len(hits)
	case f.Key("ArrowUp") && len(hits) > 0:
		s.Idx = (s.Idx - 1 + len(hits)) % len(hits)
	case f.Key("Enter") || f.Key("NumpadEnter"):
		if s.Idx < len(hits) {
			c.pickSearch(hits[s.Idx])
		}
	}
}

// pickSearch selects a result and centres the camera on it.
func (c *Controller) pickSearch(h Hit) {
	c.CloseSearch()
	c.sh.Selection().Select(h.C)
	c.sh.CenterOnMap(float64(h.X)+0.5, float64(h.Y)+0.5)
}

// searchRect is the search box (PLAY-018): centred under the colonist bar
// (the top when the bar is off), kept between the resource list and the
// right column, so it covers neither them nor the bar.
// SearchRect is the search box.
func (c *Controller) SearchRect(sw int) uikit.Button {
	n := len(c.SearchHits())
	l := c.sh.Layout()
	y := float32(hudlayout.HudMargin)
	if !l.Bar.Empty() {
		y = l.Bar.Bottom()
	}
	side := float32(hudlayout.RightW + 2*hudlayout.HudMargin) // the bar keeps this much clear on both sides; so does the box
	w := max(min(searchW, float32(sw)-2*side), searchMinW)
	x := (float32(sw) - w) / 2
	x = max(min(x, float32(sw)-side-w), hudlayout.HudMargin+hudlayout.ResourcesW+hudlayout.HudMargin) // a narrow window pushes it left of the right column
	return uikit.Button{X: float32(int(x)), Y: y + hudlayout.HudMargin, W: w, H: float32(46 + max(n, 1)*searchRowH)}
}

// searchButtons are the result rows ("search.pick.N").
// SearchButtons are the result rows ("search.pick.N").
func (c *Controller) SearchButtons(sw int) []uikit.Button {
	if !c.Search.Open {
		return nil
	}
	r := c.SearchRect(sw)
	out := []uikit.Button{}
	for i, h := range c.SearchHits() {
		out = append(out, uikit.Button{ID: "search.pick." + strconv.Itoa(i), Label: h.Label,
			X: r.X + 8, Y: r.Y + 40 + float32(i*searchRowH), W: r.W - 16, H: searchRowH - 2, Selected: i == c.Search.Idx})
	}
	return append(out, uikit.Button{ID: "search.box", X: r.X, Y: r.Y, W: r.W, H: 38})
}

// applySearchClick picks a clicked result.
// ApplySearchClick picks a clicked result.
func (c *Controller) ApplySearchClick(id string) {
	i, err := strconv.Atoi(strings.TrimPrefix(id, "search.pick."))
	if hits := c.SearchHits(); err == nil && i >= 0 && i < len(hits) {
		c.pickSearch(hits[i])
	}
}

// DrawSearch draws the box.
func (c *Controller) DrawSearch(screen *ebiten.Image, sw int) {
	if !c.Search.Open {
		return
	}
	r := c.SearchRect(sw)
	uikit.DrawPanel(screen, r.X, r.Y, r.W, r.H)
	gfx.FillRect(screen, r.X+8, r.Y+8, r.W-16, 24, uikit.Space, false)
	gfx.StrokeRect(screen, r.X+8, r.Y+8, r.W-16, 24, 1, uikit.Accent, false)
	_, updates := c.sh.Frame()
	cursor := ""
	if (updates/30)%2 == 0 {
		cursor = "_"
	}
	query := []rune(c.Search.Text)
	for len(query) > 1 && gfx.TextW("Search: "+string(query)+cursor, gfx.Body) > r.W-28 {
		query = query[1:] // keep the end of a long query, where the player is typing
	}
	gfx.Text(screen, "Search: "+string(query)+cursor, float64(r.X)+14, float64(r.Y)+8+(24-gfx.LineH(gfx.Body))/2, gfx.Body)
	hits := c.SearchHits()
	if len(hits) == 0 {
		msg := "Type a name, species, item or building"
		if c.Search.Text != "" {
			msg = "Nothing matches"
		}
		gfx.Text(screen, msg, float64(r.X)+16, float64(r.Y)+42, gfx.WithColor(gfx.Body, uikit.TextDim))
	}
	mx, my := c.sh.Cursor()
	for i, b := range c.SearchButtons(sw) {
		if i >= len(hits) {
			break
		}
		fill := uikit.Panel
		switch {
		case b.Selected:
			fill = uikit.BtnSelected
		case b.Contains(mx, my):
			fill = uikit.BtnHover
		}
		gfx.FillRect(screen, b.X, b.Y, b.W, b.H, fill, false)
		kind := hits[i].Kind
		ty := float64(b.Y) + (float64(b.H)-gfx.LineH(gfx.Body))/2
		kw := float64(gfx.TextW(kind, gfx.Small))
		gfx.Text(screen, gfx.Ellipsize(hits[i].Label, gfx.Body, float64(b.W)-kw-26), float64(b.X)+6, ty, gfx.Body)
		gfx.Text(screen, kind, float64(b.X+b.W)-8, float64(b.Y)+(float64(b.H)-gfx.LineH(gfx.Small))/2, gfx.TextStyle{Size: gfx.Small.Size, Color: uikit.TextDim, Align: gfx.AlignRight})
	}
}

// pawnLabel is a colonist's name in the results.
func pawnLabel(p game.PawnView) string {
	if p.Name != "" {
		return p.Name
	}
	return fmt.Sprintf("Colonist %d", p.ID)
}

func titleCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
