package ebiten

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
)

// Search (UI-016, Z; MAIN_UI §55): a text box over the map. Typing is the
// text context, so no hotkey fires while it is open. Results come from
// the read models (colonists, animals by name or species, items by kind,
// buildings, plants, blueprints), best match first; Up/Down and Enter or
// a click select the thing and centre the camera on it; Esc closes.

type searchState struct {
	open bool
	text string
	idx  int
	// cache of the last result list
	hits    []searchHit
	hitText string
	hitTick clock.Tick
	hitSet  bool
}

// searchHit is one result.
type searchHit struct {
	label string
	kind  string
	c     selCand
	x, y  int
	rank  int
}

const (
	searchW       = 440
	searchMinW    = 200 // the box never gets narrower, however small the window
	searchMaxHits = 8
	searchRowH    = 22
	searchMaxLen  = 32
)

var searchKindOrder = map[selKind]int{selPawn: 0, selAnimal: 1, selItem: 2, selBuilding: 3, selFrame: 4, selPlant: 5}

func (a *App) openSearch() {
	a.search = searchState{open: true}
}

func (a *App) closeSearch() { a.search = searchState{} }

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
func searchResults(w game.WorldView, pawns []game.PawnView, q string) []searchHit {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return nil
	}
	var out []searchHit
	add := func(label, kind string, c selCand, x, y int, names ...string) {
		best := -1
		for _, n := range append([]string{label}, names...) {
			if r := matchRank(n, q); r >= 0 && (best < 0 || r < best) {
				best = r
			}
		}
		if best >= 0 {
			out = append(out, searchHit{label: label, kind: kind, c: c, x: x, y: y, rank: best})
		}
	}
	for _, p := range pawns {
		if !p.Embodied {
			add(pawnLabel(p), "colonist", selCand{selPawn, p.ID}, p.X, p.Y)
		}
	}
	for _, an := range w.Animals {
		label := titleCase(an.Label)
		if an.Name != "" {
			label = an.Name + " (" + an.Label + ")"
		}
		add(label, "animal", selCand{selAnimal, an.ID}, an.X, an.Y, an.Label, an.Species)
	}
	for _, it := range w.Items {
		if !it.Carried {
			add(fmt.Sprintf("%s x %d", titleCase(it.Kind), it.Qty), "item", selCand{selItem, it.ID}, it.X, it.Y)
		}
	}
	for _, b := range w.Buildings {
		add(buildingTitle(b), "building", selCand{selBuilding, b.ID}, b.X, b.Y, b.Kind)
	}
	for _, f := range w.Frames {
		add(titleCase(f.Kind)+" blueprint", "blueprint", selCand{selFrame, f.ID}, f.X, f.Y)
	}
	for _, p := range w.Plants {
		add(titleCase(strings.ReplaceAll(p.Species, "_", " ")), "plant", selCand{selPlant, p.ID}, p.X, p.Y)
	}
	sort.SliceStable(out, func(i, j int) bool {
		hi, hj := out[i], out[j]
		if hi.rank != hj.rank {
			return hi.rank < hj.rank
		}
		if ki, kj := searchKindOrder[hi.c.kind], searchKindOrder[hj.c.kind]; ki != kj {
			return ki < kj
		}
		if hi.label != hj.label {
			return hi.label < hj.label
		}
		return hi.c.id < hj.c.id
	})
	if len(out) > searchMaxHits {
		out = out[:searchMaxHits]
	}
	return out
}

// searchHits is the cached result list for the current text.
func (a *App) searchHits() []searchHit {
	s := &a.search
	if a.renderer == nil {
		return nil
	}
	tick := a.shell.Tick()
	if !s.hitSet || s.hitText != s.text || s.hitTick != tick {
		s.hits = searchResults(a.shell.World(), a.shell.Pawns(), s.text)
		s.hitText, s.hitTick, s.hitSet = s.text, tick, true
	}
	return s.hits
}

// searchTyping is the search box's text context: characters, Backspace,
// Up/Down, Enter and Esc.
func (a *App) searchTyping() {
	f := &a.in
	s := &a.search
	for _, r := range f.Raw.Chars {
		if unicode.IsPrint(r) && utf8.RuneCountInString(s.text) < searchMaxLen {
			s.text += string(r)
			s.idx = 0
		}
	}
	if f.Key("Backspace") && s.text != "" {
		_, n := utf8.DecodeLastRuneInString(s.text)
		s.text = s.text[:len(s.text)-n]
		s.idx = 0
	}
	hits := a.searchHits()
	switch {
	case f.Key("Escape"):
		a.closeSearch()
	case f.Key("ArrowDown") && len(hits) > 0:
		s.idx = (s.idx + 1) % len(hits)
	case f.Key("ArrowUp") && len(hits) > 0:
		s.idx = (s.idx - 1 + len(hits)) % len(hits)
	case f.Key("Enter") || f.Key("NumpadEnter"):
		if s.idx < len(hits) {
			a.pickSearch(hits[s.idx])
		}
	}
}

// pickSearch selects a result and centres the camera on it.
func (a *App) pickSearch(h searchHit) {
	a.closeSearch()
	a.selectCand(h.c)
	a.centerOnMap(float64(h.x)+0.5, float64(h.y)+0.5)
}

// searchRect is the search box (PLAY-018): centred under the colonist bar
// (the top when the bar is off), kept between the resource list and the
// right column, so it covers neither them nor the bar.
func (a *App) searchRect(sw int) button {
	n := len(a.searchHits())
	l := a.hud()
	y := float32(hudMargin)
	if !l.Bar.Empty() {
		y = l.Bar.Bottom()
	}
	side := float32(rightW + 2*hudMargin) // the bar keeps this much clear on both sides; so does the box
	w := max(min(searchW, float32(sw)-2*side), searchMinW)
	x := (float32(sw) - w) / 2
	x = max(min(x, float32(sw)-side-w), hudMargin+resourcesW+hudMargin) // a narrow window pushes it left of the right column
	return button{x: float32(int(x)), y: y + hudMargin, w: w, h: float32(46 + max(n, 1)*searchRowH)}
}

// searchButtons are the result rows ("search.pick.N").
func (a *App) searchButtons(sw int) []button {
	if !a.search.open || a.renderer == nil {
		return nil
	}
	r := a.searchRect(sw)
	out := []button{}
	for i, h := range a.searchHits() {
		out = append(out, button{id: "search.pick." + strconv.Itoa(i), label: h.label,
			x: r.x + 8, y: r.y + 40 + float32(i*searchRowH), w: r.w - 16, h: searchRowH - 2, selected: i == a.search.idx})
	}
	return append(out, button{id: "search.box", x: r.x, y: r.y, w: r.w, h: 38})
}

// applySearchClick picks a clicked result.
func (a *App) applySearchClick(id string) {
	i, err := strconv.Atoi(strings.TrimPrefix(id, "search.pick."))
	if hits := a.searchHits(); err == nil && i >= 0 && i < len(hits) {
		a.pickSearch(hits[i])
	}
}

func (a *App) drawSearch(screen *ebiten.Image, sw int) {
	if !a.search.open || a.renderer == nil {
		return
	}
	r := a.searchRect(sw)
	drawPanel(screen, r.x, r.y, r.w, r.h)
	uiFillRect(screen, r.x+8, r.y+8, r.w-16, 24, uiSpace, false)
	uiStrokeRect(screen, r.x+8, r.y+8, r.w-16, 24, 1, uiAccent, false)
	cursor := ""
	if (a.updates/30)%2 == 0 {
		cursor = "_"
	}
	query := []rune(a.search.text)
	for len(query) > 1 && textW("Search: "+string(query)+cursor, tsBody) > r.w-28 {
		query = query[1:] // keep the end of a long query, where the player is typing
	}
	drawText(screen, "Search: "+string(query)+cursor, float64(r.x)+14, float64(r.y)+8+(24-lineH(tsBody))/2, tsBody)
	hits := a.searchHits()
	if len(hits) == 0 {
		msg := "Type a name, species, item or building"
		if a.search.text != "" {
			msg = "Nothing matches"
		}
		drawText(screen, msg, float64(r.x)+16, float64(r.y)+42, withColor(tsBody, uiTextDim))
	}
	mx, my := a.currentCursor()
	for i, b := range a.searchButtons(sw) {
		if i >= len(hits) {
			break
		}
		fill := uiPanel
		switch {
		case b.selected:
			fill = uiBtnSelected
		case b.contains(mx, my):
			fill = uiBtnHover
		}
		uiFillRect(screen, b.x, b.y, b.w, b.h, fill, false)
		kind := hits[i].kind
		ty := float64(b.y) + (float64(b.h)-lineH(tsBody))/2
		kw := float64(textW(kind, tsSmall))
		drawText(screen, ellipsize(hits[i].label, tsBody, float64(b.w)-kw-26), float64(b.x)+6, ty, tsBody)
		drawText(screen, kind, float64(b.x+b.w)-8, float64(b.y)+(float64(b.h)-lineH(tsSmall))/2, textStyle{Size: tsSmall.Size, Color: uiTextDim, Align: alignRight})
	}
}
