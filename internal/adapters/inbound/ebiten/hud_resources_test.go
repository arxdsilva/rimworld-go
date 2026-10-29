package ebiten

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/hud"
	"rimworld-go/internal/adapters/inbound/ebiten/hudlayout"
	"rimworld-go/internal/application/game"
)

// PLAY-018 HUD-TOP: the resource list, top-left, in the layout's rect.

func htStored(kind, label, cat string, qty int) game.StoredResourceView {
	return game.StoredResourceView{Kind: kind, Label: label, Category: cat, Qty: qty}
}

// htManyStored is n distinct kinds with stock.
func htManyStored(n int) []game.StoredResourceView {
	out := make([]game.StoredResourceView, n)
	for i := range out {
		out[i] = htStored(fmt.Sprintf("kind%02d", i), fmt.Sprintf("Kind %02d", i), "Materials", 10+i)
	}
	return out
}

func htResApp(t *testing.T, w game.WorldView) (*App, *fakeInput) {
	t.Helper()
	return htApp(t, &stubSim{pawns: htPawns(2), world: w}, 1280, 720)
}

func htRowKinds(rows []resRow) []string {
	var out []string
	for _, r := range rows {
		if r.header {
			out = append(out, "#"+r.cat)
		} else {
			out = append(out, r.kind)
		}
	}
	return out
}

func TestResourceRowsComeFromStoredGoods(t *testing.T) {
	t.Parallel()
	a, _ := htResApp(t, game.WorldView{
		Food: 99, Wood: 99, // loose stacks: not what the list shows
		Stored: []game.StoredResourceView{
			htStored("wood", "Wood", "Materials", 40),
			htStored("stone", "Stone", "Materials", 0), // none left: omitted
			htStored("steel", "Steel", "Materials", 7),
			htStored("meal", "Meal", "Food", 3),
		},
	})
	if got, want := htRowKinds(a.resourceRows()), []string{"wood", "steel", "meal"}; !slices.Equal(got, want) {
		t.Fatalf("rows = %v, want %v", got, want)
	}
	if h := a.chrome.Res().Height(); h != 3*hudlayout.ResourceRowH {
		t.Fatalf("resourcesHeight = %v, want %v", h, 3*hudlayout.ResourceRowH)
	}
	l := a.hud()
	want := gfx.Rect{X: hudlayout.HudMargin, Y: hudlayout.HudMargin, W: hudlayout.ResourcesW, H: 3 * hudlayout.ResourceRowH}
	if l.Resources != want {
		t.Fatalf("Resources rect = %+v, want %+v", l.Resources, want)
	}
	bs := a.chrome.Res().Buttons()
	if len(bs) != 3 {
		t.Fatalf("%d resource buttons, want 3", len(bs))
	}
	for i, b := range bs {
		kind := []string{"wood", "steel", "meal"}[i]
		label := []string{"Wood", "Steel", "Meal"}[i]
		if b.ID != "res.row."+kind || b.Tip != label {
			t.Errorf("button %d = %q tip %q, want res.row.%s tip %q", i, b.ID, b.Tip, kind, label)
		}
		if r := (gfx.Rect{X: b.X, Y: b.Y, W: b.W, H: b.H}); r.X < l.Resources.X || r.Right() > l.Resources.Right() || r.Y < l.Resources.Y || r.Bottom() > l.Resources.Bottom() {
			t.Errorf("button %s %+v leaves the list %+v", b.ID, r, l.Resources)
		}
	}
	// the legacy chrome is gone: the layout covers the list
	assertChromeIsTheLayout(t, a)
}

func TestResourceRowsFallBackToTheTotals(t *testing.T) {
	t.Parallel()
	a, _ := htResApp(t, game.WorldView{Food: 3, Wood: 40, Stone: 12})
	got := a.resourceRows()
	want := []string{"food", "wood", "stone", "steel"}
	if !slices.Equal(htRowKinds(got), want) {
		t.Fatalf("fallback rows = %v, want %v", htRowKinds(got), want)
	}
	if got[1].qty != 40 || got[3].qty != 0 {
		t.Fatalf("fallback quantities = %+v", got)
	}
	// precious metals join once the colony has some
	a, _ = htResApp(t, game.WorldView{Silver: 25, Gold: 8})
	if kinds := htRowKinds(a.resourceRows()); !slices.Contains(kinds, "silver") || !slices.Contains(kinds, "gold") {
		t.Fatalf("fallback rows = %v, want silver and gold", kinds)
	}
	if a.chrome.Res().Height() == 0 || a.hud().Resources.Empty() {
		t.Fatal("the list is blank while the read model is empty")
	}
}

func TestResourceRowsNeverGoBlank(t *testing.T) {
	t.Parallel()
	// every stored row is empty: the totals fallback keeps the list from going blank
	a, _ := htResApp(t, game.WorldView{Stored: []game.StoredResourceView{htStored("wood", "Wood", "Materials", 0)}})
	if len(a.resourceRows()) == 0 {
		t.Fatal("no rows at all")
	}
}

func TestResourcesCategorisedWithCollapsibleHeaders(t *testing.T) {
	a, _ := htResApp(t, game.WorldView{Stored: []game.StoredResourceView{
		htStored("wood", "Wood", "Materials", 40),
		htStored("meal", "Meal", "Food", 3),
		htStored("meat", "Meat", "Food", 12),
		htStored("steel", "Steel", "Materials", 7),
		htStored("odd", "Oddity", "Zebra things", 1),
	}})
	a.settings.Gameplay.CategorizedResources = true
	want := []string{"#Food", "meal", "meat", "#Materials", "wood", "steel", "#Zebra things", "odd"}
	if got := htRowKinds(a.resourceRows()); !slices.Equal(got, want) {
		t.Fatalf("categorised rows = %v, want %v", got, want)
	}
	if h := a.chrome.Res().Height(); h != float32(len(want))*hudlayout.ResourceRowH {
		t.Fatalf("height = %v, want %v", h, float32(len(want))*hudlayout.ResourceRowH)
	}
	var ids []string
	for _, b := range a.chrome.Res().Buttons() {
		ids = append(ids, b.ID)
	}
	wantIDs := []string{"res.cat.Food", "res.row.meal", "res.row.meat", "res.cat.Materials", "res.row.wood", "res.row.steel", "res.cat.Zebra things", "res.row.odd"}
	if !slices.Equal(ids, wantIDs) {
		t.Fatalf("button ids = %v, want %v", ids, wantIDs)
	}

	// a click on a header folds its group; another opens it again
	click(t, a, "res.cat.Food")
	want = []string{"#Food", "#Materials", "wood", "steel", "#Zebra things", "odd"}
	if got := htRowKinds(a.resourceRows()); !slices.Equal(got, want) {
		t.Fatalf("after folding Food: %v, want %v", got, want)
	}
	if h := a.chrome.Res().Height(); h != float32(len(want))*hudlayout.ResourceRowH {
		t.Fatalf("height after folding = %v, want %v", h, float32(len(want))*hudlayout.ResourceRowH)
	}
	if l := a.hud(); l.Resources.H != float32(len(want))*hudlayout.ResourceRowH {
		t.Fatalf("the layout rect did not shrink: %+v", l.Resources)
	}
	click(t, a, "res.cat.Food")
	if got := htRowKinds(a.resourceRows()); len(got) != 8 {
		t.Fatalf("after opening Food: %v", got)
	}

	// turning the setting off flattens the list
	a.settings.Gameplay.CategorizedResources = false
	if got, want := htRowKinds(a.resourceRows()), []string{"wood", "meal", "meat", "steel", "odd"}; !slices.Equal(got, want) {
		t.Fatalf("flat rows = %v, want %v", got, want)
	}
}

func TestResourcesHeaderTotalsShowWhenFolded(t *testing.T) {
	a, _ := htResApp(t, game.WorldView{Stored: []game.StoredResourceView{
		htStored("meal", "Meal", "Food", 3), htStored("meat", "Meat", "Food", 12),
	}})
	a.settings.Gameplay.CategorizedResources = true
	click(t, a, "res.cat.Food")
	rows := a.resourceRows()
	if len(rows) != 1 || !rows[0].header || rows[0].qty != 15 {
		t.Fatalf("folded rows = %+v, want one Food header totalling 15", rows)
	}
}

func TestResourcesScrollWithTheWheel(t *testing.T) {
	a, f := htResApp(t, game.WorldView{Stored: htManyStored(40)})
	r := a.hud().Resources
	content := a.chrome.Res().Height()
	if content != 40*hudlayout.ResourceRowH || r.H >= content {
		t.Fatalf("setup: content %v in a %v px list, want it to overflow", content, r.H)
	}
	maxOff := content - r.H
	first := func() string {
		bs := a.chrome.Res().Buttons()
		if len(bs) == 0 {
			return ""
		}
		return bs[0].ID
	}
	if got := first(); got != "res.row.kind00" {
		t.Fatalf("first visible row = %q before scrolling", got)
	}
	// the buttons are the visible rows only, clipped to the list
	for _, b := range a.chrome.Res().Buttons() {
		if b.Y < r.Y-0.01 || b.Y+b.H > r.Bottom()+0.01 {
			t.Fatalf("button %s %v..%v leaves the list %v..%v", b.ID, b.Y, b.Y+b.H, r.Y, r.Bottom())
		}
	}
	zoom := a.renderer.smooth
	f.Cur.Mx, f.Cur.My = int(r.X+20), int(r.Y+20)
	f.Cur.WheelY = -1
	f.Frame(t, a)
	off := a.uiSt().Scroll[hud.ResScrollID]
	if off <= 0 {
		t.Fatalf("the wheel over the list left the offset at %v", off)
	}
	if got := first(); got == "res.row.kind00" {
		t.Fatalf("the first visible row did not change after scrolling %v px", off)
	}
	if a.renderer.smooth.PendingScale != zoom.PendingScale || a.renderer.smooth.Zooming != zoom.Zooming {
		t.Fatalf("the camera zoomed on a wheel the list took: %+v -> %+v", zoom, a.renderer.smooth)
	}
	// scrolling stops at the end of the content
	for i := 0; i < 40; i++ {
		f.Cur.WheelY = -1
		f.Frame(t, a)
	}
	if off = a.uiSt().Scroll[hud.ResScrollID]; off != maxOff {
		t.Fatalf("offset = %v after scrolling to the end, want %v", off, maxOff)
	}
	bs := a.chrome.Res().Buttons()
	if last := bs[len(bs)-1]; last.ID != "res.row.kind39" || last.Y+last.H > r.Bottom()+0.01 {
		t.Fatalf("last row at the end = %+v", last)
	}
	// and back to the top
	for i := 0; i < 40; i++ {
		f.Cur.WheelY = 1
		f.Frame(t, a)
	}
	if off = a.uiSt().Scroll[hud.ResScrollID]; off != 0 {
		t.Fatalf("offset = %v after scrolling back, want 0", off)
	}
	// a wheel away from the list belongs to the camera
	before := a.renderer.smooth
	f.Cur.Mx, f.Cur.My = 640, 400
	f.Cur.WheelY = -1
	f.Frame(t, a)
	if off = a.uiSt().Scroll[hud.ResScrollID]; off != 0 {
		t.Fatalf("a wheel over the map scrolled the list to %v", off)
	}
	if a.renderer.smooth == before {
		t.Fatal("a wheel over the map did not reach the camera (so the check that the list keeps the wheel from it proves nothing)")
	}
}

func TestResourcesWheelIsNotTakenWhenTheListFits(t *testing.T) {
	a, f := htResApp(t, game.WorldView{Stored: htManyStored(3)})
	r := a.hud().Resources
	f.Cur.Mx, f.Cur.My = int(r.X+20), int(r.Y+10)
	if a.uiSt().WheelOver(f.Cur.Mx, f.Cur.My) {
		t.Fatal("a list that fits takes the wheel")
	}
}

func TestResourcesHiddenWithoutRoom(t *testing.T) {
	// a 90 px high window leaves 38 px above the tabs: under the 40 px minimum
	a, _ := htResApp(t, game.WorldView{Stored: htManyStored(5)})
	a.sh = 90
	if l := a.hud(); !l.Resources.Empty() {
		t.Fatalf("Resources = %+v with no room", l.Resources)
	}
	if bs := a.chrome.Res().Buttons(); len(bs) != 0 {
		t.Fatalf("%d resource buttons with no room", len(bs))
	}
	ev := htTrace(t, func() { a.chrome.Res().Draw(ebiten.NewImage(1280, 90), 1280, 90) })
	if len(ev) != 0 {
		t.Fatalf("the hidden list drew %d things", len(ev))
	}
	if a.chrome.Res().RightClick(20, 20) {
		t.Fatal("a hidden list takes right clicks")
	}
}

func TestResourcesDrawIconAndCountPerRow(t *testing.T) {
	a, _ := htResApp(t, game.WorldView{Stored: []game.StoredResourceView{
		htStored("wood", "Wood", "Materials", 40), htStored("steel", "Steel", "Materials", 1234),
	}})
	r := a.hud().Resources
	ev := htTrace(t, func() { a.chrome.Res().Draw(ebiten.NewImage(1280, 720), 1280, 720) })
	texts := map[string]gfx.TraceEvent{}
	fills := 0
	for _, e := range ev {
		if !htInside(e, r) {
			t.Errorf("draw %+v outside the list %+v", e, r)
		}
		switch e.Kind {
		case "text":
			texts[e.Text] = e
		case "fill":
			fills++
		}
	}
	for _, c := range []string{"40", "1234"} {
		if _, ok := texts[c]; !ok {
			t.Errorf("no count %q drawn (texts: %v)", c, texts)
		}
	}
	if fills == 0 {
		t.Error("no backing plate behind the readouts")
	}
	// the count sits to the right of its icon, on its own row
	if texts["40"].Y >= texts["1234"].Y {
		t.Errorf("rows out of order: 40 at y %v, 1234 at y %v", texts["40"].Y, texts["1234"].Y)
	}
}

func TestResourcesHeadersDrawTheirNames(t *testing.T) {
	a, _ := htResApp(t, game.WorldView{Stored: []game.StoredResourceView{
		htStored("meal", "Meal", "Food", 3), htStored("wood", "Wood", "Materials", 40),
	}})
	a.settings.Gameplay.CategorizedResources = true
	ev := htTrace(t, func() { a.chrome.Res().Draw(ebiten.NewImage(1280, 720), 1280, 720) })
	var texts []string
	for _, e := range ev {
		if e.Kind == "text" {
			texts = append(texts, e.Text)
		}
	}
	for _, want := range []string{"Food", "Materials", "3", "40"} {
		if !slices.Contains(texts, want) {
			t.Errorf("text %q not drawn (drew %v)", want, texts)
		}
	}
}

func TestResourcesScaleWithTheUI(t *testing.T) {
	for _, s := range []float64{1, 1.5, 2} {
		sw, sh := int(1280/s), int(720/s)
		a, _ := htResApp(t, game.WorldView{Stored: htManyStored(12)})
		a.sw, a.sh = sw, sh
		withUIScale(t, s) // after the frames: Update re-derives the scale from the settings
		r := htPhysRect(a.hud().Resources, s)
		screen := ebiten.NewImage(1280, 720)
		ev := htTrace(t, func() { a.chrome.Res().Draw(screen, sw, sh) })
		if len(ev) == 0 {
			t.Fatalf("s=%v: nothing drawn", s)
		}
		for _, e := range ev {
			if !htInside(e, r) {
				t.Errorf("s=%v: draw %+v outside the list %+v", s, e, r)
			}
		}
	}
}

func TestRightClickOverTheResourcesIsTaken(t *testing.T) {
	t.Parallel()
	a, _ := htResApp(t, game.WorldView{Stored: htManyStored(5)})
	r := a.hud().Resources
	if !a.chrome.Res().RightClick(int(r.X)+10, int(r.Y)+10) {
		t.Error("a right click on the list reached the map")
	}
	if a.chrome.Res().RightClick(int(r.Right())+40, int(r.Y)+10) {
		t.Error("a right click beside the list was taken")
	}
	if a.chrome.Res().RightClick(int(r.X)+10, int(r.Bottom())+40) {
		t.Error("a right click under the list was taken")
	}
}

func TestResourceRowsAreCachedPerView(t *testing.T) {
	w := game.WorldView{Stored: htManyStored(30)}
	a, _ := htResApp(t, w)
	a.chrome.Res().Rows()
	if n := testing.AllocsPerRun(50, func() { a.chrome.Res().Rows() }); n != 0 {
		t.Fatalf("resourceRows allocates %v per call for an unchanged view", n)
	}
	// a changed view is rebuilt
	st := a.renderer.game.(*stubSim)
	st.world = game.WorldView{Stored: htManyStored(5)}
	if n := len(a.resourceRows()); n != 5 {
		t.Fatalf("%d rows after the view changed, want 5", n)
	}
}

func TestResourceIconFallsBackToTheItemSprite(t *testing.T) {
	art := testArt(t)
	a, _ := htResApp(t, game.WorldView{})
	a.renderer.art = art
	for _, kind := range []string{"wood", "steel", "silver", "granite blocks"} {
		img, _ := a.chrome.Res().Icon(kind)
		if img == nil {
			t.Errorf("no icon for %q: neither a ui_icon nor an item sprite", kind)
		}
	}
	if img, _ := a.chrome.Res().Icon("no such kind"); img != nil {
		t.Error("an unknown kind found an icon")
	}
}

func TestResourcesDrawWithoutArt(t *testing.T) {
	t.Parallel()
	a, _ := htResApp(t, game.WorldView{Stored: htManyStored(4)})
	if a.renderer.art != nil {
		t.Fatal("setup: the unit test has art")
	}
	screen := ebiten.NewImage(1280, 720)
	a.chrome.Res().Draw(screen, 1280, 720) // missing icons degrade to a letter tile: no panic
}

func TestResourceRowButtonsDoNotCollideWithEachOther(t *testing.T) {
	t.Parallel()
	a, _ := htResApp(t, game.WorldView{Stored: htManyStored(12)})
	bs := a.chrome.Res().Buttons()
	for i := 1; i < len(bs); i++ {
		if bs[i].Y < bs[i-1].Y+bs[i-1].H-0.01 {
			t.Fatalf("buttons %s and %s overlap", bs[i-1].ID, bs[i].ID)
		}
	}
	if !strings.HasPrefix(bs[0].ID, "res.row.") || strconv.Itoa(len(bs)) != "12" {
		t.Fatalf("buttons = %d, first %q", len(bs), bs[0].ID)
	}
}
