package ebiten

import (
	"fmt"
	"strings"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/platform/config"
	"rimworld-go/internal/platform/sysinfo"
)

// PLAY-019 WARNINGS: the RAM model, the low-RAM notice on the main menu and
// the map-size warnings. The game warns only; it never blocks a size.

const gib = sysinfo.GiB

// hwOf is a Hardware hook reporting total bytes of memory.
func hwOf(total uint64) func() sysinfo.Info {
	return func() sysinfo.Info { return sysinfo.Info{Total: total, Available: total / 2, Cores: 4} }
}

// withHardware configures an App's machine.
func withHardware(total uint64) func(*AppConfig) {
	return func(c *AppConfig) { c.Hardware = hwOf(total) }
}

// textEvents are the text runs of a trace.
func textEvents(ev []traceEvent) []traceEvent {
	var out []traceEvent
	for _, e := range ev {
		if e.Kind == "text" {
			out = append(out, e)
		}
	}
	return out
}

// drawnText draws the App once at its logical size x scale and returns the
// text runs it drew, in physical pixels.
func drawnText(t *testing.T, a *App, pw, ph int) []traceEvent {
	t.Helper()
	got := collectTrace(t)
	a.Draw(ebiten.NewImage(pw, ph))
	return textEvents(*got)
}

// ---- the model (pure) ------------------------------------------------------

func TestEstimatedRAMGrowsWithTheMap(t *testing.T) {
	t.Parallel()
	if got := estimatedRAM(250) - estimatedRAM(0); got != 250*250*ramPerTile {
		t.Fatalf("250 adds %d bytes over the base, want side^2 x %d", got, ramPerTile)
	}
	if estimatedRAM(0) != ramBase {
		t.Fatalf("estimatedRAM(0) = %d, want the base %d", estimatedRAM(0), ramBase)
	}
	prev := estimatedRAM(0)
	for _, s := range []int{120, 200, 250, 300, 400, 500} {
		if e := estimatedRAM(s); e <= prev {
			t.Errorf("estimatedRAM(%d) = %d is not above the smaller map's %d", s, e, prev)
		} else {
			prev = e
		}
	}
	if estimatedRAM(-5) != ramBase {
		t.Error("a negative side counts as no tiles")
	}
	// The measured footprint fit: 1850 MiB at no tiles, and the 250 ->
	// grown 500 difference (1180 MiB over 187 500 tiles) per tile.
	if ramBase != 1850<<20 || ramPerTile != 6600 {
		t.Errorf("base %d, per tile %d; want 1850 MiB and 6600 B (measured peak footprint)", ramBase, ramPerTile)
	}
	if got := estimatedRAM(500) >> 20; got < 3400 || got > 3450 {
		t.Errorf("grown 500 estimate %d MiB, want the measured 3422 MiB", got)
	}
	if ramPerTile < 256 {
		t.Errorf("per-tile cost %d is below the 256-byte floor", ramPerTile)
	}
}

func TestMinimumRAMIsOneAndAHalfPlusTwoGiBRoundedUp(t *testing.T) {
	t.Parallel()
	need := estimatedRAM(defaultMapSide)*3/2 + 2*gib
	got := minimumRAM()
	if got%gib != 0 {
		t.Errorf("minimum %d is not a whole number of GiB", got)
	}
	if got < need || got >= need+gib {
		t.Errorf("minimum %d is not %d rounded up to a GiB", got, need)
	}
}

func TestGBRounding(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		b           uint64
		ceil, round int
	}{
		{0, 0, 0},
		{1, 1, 0},
		{gib, 1, 1},
		{gib + 1, 2, 1},
		{gib * 3 / 2, 2, 2},        // 1.5 GiB rounds half up
		{gib * 37 / 10, 4, 4},      // 3.7
		{gib * 79 / 10, 8, 8},      // a "8 GB" PC that reports 7.9 GiB
		{16 * gib, 16, 16},         // exact
		{16*gib - 100<<20, 16, 16}, // 15.9 GiB
	} {
		if got := gbCeil(c.b); got != c.ceil {
			t.Errorf("gbCeil(%d) = %d, want %d", c.b, got, c.ceil)
		}
		if got := gbRound(c.b); got != c.round {
			t.Errorf("gbRound(%d) = %d, want %d", c.b, got, c.round)
		}
	}
}

func TestLowRAMTextNamesBothAmounts(t *testing.T) {
	t.Parallel()
	want := fmt.Sprintf("This computer has 4 GB of memory; the game needs about %d GB to run smoothly. It will use low graphics settings.",
		minimumRAM()/gib)
	if got := lowRAMText(4 * gib); got != want {
		t.Errorf("text = %q, want %q", got, want)
	}
	// Just under the minimum must never read "has 6 GB; needs about 6 GB".
	just := minimumRAM() - 1
	if !strings.Contains(lowRAMText(just), fmt.Sprintf("has %d GB", minimumRAM()/gib-1)) {
		t.Errorf("a machine just under the minimum reads %q", lowRAMText(just))
	}
}

func TestMapSizeNoteOnlyOverSeventyPercentOfRAM(t *testing.T) {
	t.Parallel()
	// A machine whose 70% line falls between a 350 and a 400 map.
	total := estimatedRAM(375) * 10 / 7
	for _, s := range []int{120, 200, 250, 300, 350} {
		if n := mapSizeNote(s, total); n != "" {
			t.Errorf("%d fits (%d of %d bytes) but warns %q", s, estimatedRAM(s), total, n)
		}
	}
	for _, s := range []int{400, 450, 500} {
		want := fmt.Sprintf("Needs about %d GB; this computer has %d GB.", gbCeil(estimatedRAM(s)), gbRound(total))
		if n := mapSizeNote(s, total); n != want {
			t.Errorf("%d: note %q, want %q", s, n, want)
		}
	}
	// The line itself: exactly 70% does not warn, a byte less memory does.
	est := estimatedRAM(300)
	if mapSizeNote(300, (est*10+6)/7) != "" {
		t.Error("an estimate at or under 70% of RAM must not warn")
	}
	if mapSizeNote(300, est*10/7-1) == "" {
		t.Error("an estimate over 70% of RAM must warn")
	}
}

func TestMapSizeNoteWithUnknownHardware(t *testing.T) {
	t.Parallel()
	for _, s := range []int{120, 200, 250, 300, 349} {
		if n := mapSizeNote(s, 0); n != "" {
			t.Errorf("%d with unknown hardware: %q, want nothing", s, n)
		}
	}
	for _, s := range []int{350, 400, 500} {
		if n := mapSizeNote(s, 0); n != "Large map: needs a capable computer" {
			t.Errorf("%d with unknown hardware: %q, want the gentle large-map note", s, n)
		}
	}
	// The fallback line is the application's "Large" flag.
	for _, o := range game.MapSizes() {
		if o.Large != (o.Side >= largeMapSide) {
			t.Errorf("%d: game says Large=%v but the UI fallback starts at %d", o.Side, o.Large, largeMapSide)
		}
	}
}

func TestMapSizeLabelNoLongerClaimsAComputer(t *testing.T) {
	t.Parallel()
	for _, o := range game.MapSizes() {
		if l := mapSizeLabel(o); strings.Contains(l, "capable computer") {
			t.Errorf("%d: label %q keeps the old fixed text", o.Side, l)
		}
	}
	if got := mapSizeLabel(game.MapSizeView{Side: 400, Large: true}); got != "400 x 400" {
		t.Errorf("label = %q, want the plain size", got)
	}
}

// ---- the main-menu notice ----------------------------------------------------

func TestLowRAMNoticeShowsOn4GiBAndHidesOn16GiBAndUnknown(t *testing.T) {
	for _, c := range []struct {
		name  string
		mod   func(*AppConfig)
		shown bool
	}{
		{"4 GiB", withHardware(4 * gib), true},
		{"just under the minimum", withHardware(minimumRAM() - 1), true},
		{"the minimum", withHardware(minimumRAM()), false},
		{"16 GiB", withHardware(16 * gib), false},
		{"hardware unknown (nil)", nil, false},
		{"hardware unreadable (Total 0)", withHardware(0), false},
	} {
		t.Run(c.name, func(t *testing.T) {
			a, f, _ := ngTest(t, c.mod)
			f.Frame(t, a)
			if got := hasWidget(a, ramDismissID); got != c.shown {
				t.Fatalf("Got it button present = %v, want %v", got, c.shown)
			}
			joined := ""
			for _, e := range drawnText(t, a, a.sw, a.sh) {
				joined += e.Text + " "
			}
			if got := strings.Contains(joined, "run smoothly"); got != c.shown {
				t.Errorf("notice text drawn = %v, want %v", got, c.shown)
			}
		})
	}
}

func TestLowRAMNoticeIsOnlyOnTheMainMenu(t *testing.T) {
	a, f, _ := ngTest(t, withHardware(4*gib))
	f.Frame(t, a)
	if !hasWidget(a, ramDismissID) {
		t.Fatal("the main menu shows the notice")
	}
	click(t, a, "menu.options")
	f.Frame(t, a)
	if hasWidget(a, ramDismissID) {
		t.Error("the Options screen must not carry the main-menu notice")
	}
}

func TestLowRAMNoticeDrawsItsTextInTheCard(t *testing.T) {
	a, f, _ := ngTest(t, withHardware(4*gib))
	f.Frame(t, a)
	text := lowRAMText(4 * gib)
	card := a.ramNoticeCard(a.sw, a.sh, text)
	if card.W <= 0 || card.H <= 0 {
		t.Fatalf("card %+v", card)
	}
	var lines []string
	for _, e := range drawnText(t, a, a.sw, a.sh) {
		if float32(e.X) >= card.X && float32(e.Y) >= card.Y && float32(e.X+e.W) <= card.Right()+0.5 && float32(e.Y+e.H) <= card.Bottom()+0.5 {
			lines = append(lines, e.Text)
		}
	}
	if got := strings.Join(lines, " "); !strings.Contains(got, strings.Join(strings.Fields(text), " ")) {
		t.Errorf("text inside the card = %q, want it to hold %q", got, text)
	}
}

func TestLowRAMNoticeStyledWithTheWarningToken(t *testing.T) {
	t.Parallel()
	if got := ramNoticeStyle().Color; got != uiWarning {
		t.Errorf("notice text colour = %v, want uiWarning %v", got, uiWarning)
	}
}

func TestLowRAMNoticeDoesNotCoverTheMenuButtons(t *testing.T) {
	a, f, _ := ngTest(t, withHardware(4*gib))
	f.Frame(t, a)
	card := a.ramNoticeCard(a.sw, a.sh, lowRAMText(4*gib))
	btns := a.mainMenuButtons(a.sw, a.sh)
	if len(btns) < 7 {
		t.Fatalf("%d menu buttons", len(btns))
	}
	for _, b := range btns {
		if rectsOverlap(card, rect{X: b.X, Y: b.Y, W: b.W, H: b.H}) {
			t.Errorf("the notice %+v covers the %q button", card, b.ID)
		}
	}
	if card.X < 0 || card.Y < 0 || card.Right() > float32(a.sw) || card.Bottom() > float32(a.sh) {
		t.Errorf("the notice %+v leaves the %dx%d screen", card, a.sw, a.sh)
	}
	w, ok := a.uiSt().Find(ramDismissID)
	if !ok {
		t.Fatal("no Got it button")
	}
	if w.R.X < card.X || w.R.Y < card.Y || w.R.Right() > card.Right() || w.R.Bottom() > card.Bottom() {
		t.Errorf("Got it %+v is outside its card %+v", w.R, card)
	}
}

func rectsOverlap(a, b rect) bool {
	return a.X < b.Right() && b.X < a.Right() && a.Y < b.Bottom() && b.Y < a.Bottom()
}

func TestLowRAMNoticeDismissalIsRememberedPerAmountOfRAM(t *testing.T) {
	var saved []config.Settings
	hooks := func(start config.Settings) func(*AppConfig) {
		return func(c *AppConfig) {
			c.Hardware = hwOf(4 * gib)
			c.Settings = SettingsHooks{
				Load: func() (config.Settings, error) { return start, nil },
				Save: func(s config.Settings) error { saved = append(saved, s); return nil },
			}
		}
	}
	a, f, _ := ngTest(t, hooks(config.Defaults()))
	f.Frame(t, a)
	if !hasWidget(a, ramDismissID) {
		t.Fatal("a fresh settings file shows the notice")
	}
	press(t, a, f, ramDismissID)
	f.Frame(t, a)
	if hasWidget(a, ramDismissID) {
		t.Fatal("Got it hides the notice")
	}
	if a.settings.Gameplay.RAMNoticeSeen != 4*gib {
		t.Fatalf("RAMNoticeSeen = %d, want the dismissed amount %d", a.settings.Gameplay.RAMNoticeSeen, 4*gib)
	}
	if len(saved) == 0 || saved[len(saved)-1].Gameplay.RAMNoticeSeen != 4*gib {
		t.Fatalf("the dismissal was not saved: %d saves", len(saved))
	}
	for _, e := range drawnText(t, a, a.sw, a.sh) {
		if strings.Contains(e.Text, "run smoothly") {
			t.Fatalf("the dismissed notice is still drawn: %q", e.Text)
		}
	}

	// A later start with the same amount of RAM: still dismissed.
	seen := saved[len(saved)-1]
	b, g, _ := ngTest(t, hooks(seen))
	g.Frame(t, b)
	if hasWidget(b, ramDismissID) {
		t.Error("the same amount of RAM must stay dismissed on the next start")
	}
	// A different amount (a memory change, another machine) shows it again.
	c, h, _ := ngTest(t, func(cfg *AppConfig) {
		hooks(seen)(cfg)
		cfg.Hardware = hwOf(3 * gib)
	})
	h.Frame(t, c)
	if !hasWidget(c, ramDismissID) {
		t.Error("a different amount of RAM brings the notice back")
	}
}

func TestLowRAMNoticeAtUIScales(t *testing.T) {
	for _, s := range []float64{1, 1.5, 2} {
		t.Run(fmt.Sprintf("x%v", s), func(t *testing.T) {
			a, f, _ := ngTest(t, withHardware(4*gib))
			f.Frame(t, a)
			pw, ph := atLogical1280(t, a, f, s)
			text := lowRAMText(4 * gib)
			card := a.ramNoticeCard(a.sw, a.sh, text)
			ev := drawnText(t, a, pw, ph)
			n := 0
			for _, e := range ev {
				if e.X < 0 || e.Y < 0 || e.X+e.W > float32(pw)+0.5 || e.Y+e.H > float32(ph)+0.5 {
					t.Errorf("text %q at %g,%g %gx%g leaves the %dx%d screen", e.Text, e.X, e.Y, e.W, e.H, pw, ph)
				}
				if rectsOverlap(rect{X: e.X / float32(s), Y: e.Y / float32(s), W: e.W / float32(s), H: e.H / float32(s)}, card) {
					n++
					if e.X/float32(s) < card.X-0.5 || (e.X+e.W)/float32(s) > card.Right()+0.5 {
						t.Errorf("notice line %q runs out of its card sideways", e.Text)
					}
				}
			}
			if n == 0 {
				t.Error("no notice text found in the card")
			}
			for _, h := range a.layoutHits(a.sw, a.sh) {
				if h.ID == ramDismissID && (h.X < 0 || h.Y < 0 || h.X+h.W > float32(a.sw) || h.Y+h.H > float32(a.sh)) {
					t.Errorf("Got it %+v is outside the logical screen", h)
				}
			}
			// The hit is reachable: the cursor over its centre makes it the hot widget.
			var btn button
			for _, h := range a.layoutHits(a.sw, a.sh) {
				if h.ID == ramDismissID {
					btn = h
				}
			}
			f.Cur.Mx, f.Cur.My = int((btn.X+btn.W/2)*float32(s)), int((btn.Y+btn.H/2)*float32(s))
			f.Frame(t, a)
			f.Frame(t, a)
			if a.uiSt().Hot != ramDismissID {
				t.Errorf("the cursor over Got it hits %q", a.uiSt().Hot)
			}
		})
	}
}

// ---- the map-size warnings ---------------------------------------------------

// sizeNoteTotal is a machine whose 70% line falls between 350 and 400.
func sizeNoteTotal() uint64 { return estimatedRAM(375) * 10 / 7 }

// sizeRows is, per map size option, the radio row's rectangle and the text
// runs drawn on it (physical pixels, scale 1).
func sizeRows(t *testing.T, a *App) (opts []game.MapSizeView, rows []rect, drawn [][]traceEvent) {
	t.Helper()
	opts = mapSizeOptions()
	ev := drawnText(t, a, a.sw, a.sh)
	for i := range opts {
		w, ok := a.uiSt().Find(optID("land.mapsize", i))
		if !ok {
			t.Fatalf("no size row %d", i)
		}
		rows = append(rows, w.R)
		var on []traceEvent
		for _, e := range ev {
			if e.Y >= w.R.Y && e.Y+e.H <= w.R.Bottom()+0.5 && e.X >= w.R.X && e.X+e.W <= w.R.Right()+0.5 {
				on = append(on, e)
			}
		}
		drawn = append(drawn, on)
	}
	return opts, rows, drawn
}

func openAdvanced(t *testing.T, a *App, f *fakeInput) {
	t.Helper()
	click(t, a, "land.advanced")
	f.Frame(t, a)
	f.Frame(t, a)
}

func TestSizePickerWarnsOnlyForSizesOverSeventyPercent(t *testing.T) {
	total := sizeNoteTotal()
	a, f, _ := ngTest(t, withHardware(total))
	toLanding(t, a)
	f.Frame(t, a)
	openAdvanced(t, a, f)
	opts, _, drawn := sizeRows(t, a)
	warned := 0
	for i, o := range opts {
		want := mapSizeNote(o.Side, total)
		got := ""
		for _, e := range drawn[i] {
			if strings.HasPrefix(e.Text, "Needs about") || strings.HasPrefix(e.Text, "Large map") {
				got = e.Text
			}
		}
		if got != want {
			t.Errorf("%d: warning drawn %q, want %q", o.Side, got, want)
		}
		if want != "" {
			warned++
		}
	}
	if warned != 3 {
		t.Errorf("%d sizes warn, want 3 (400, 450 and 500)", warned)
	}
}

func TestSizeWarningsNameTheRightNumbers(t *testing.T) {
	total := sizeNoteTotal()
	a, f, _ := ngTest(t, withHardware(total))
	toLanding(t, a)
	f.Frame(t, a)
	openAdvanced(t, a, f)
	opts, _, drawn := sizeRows(t, a)
	for i, o := range opts {
		if o.Side != 500 {
			continue
		}
		want := fmt.Sprintf("Needs about %d GB; this computer has %d GB.", gbCeil(estimatedRAM(500)), gbRound(total))
		found := false
		for _, e := range drawn[i] {
			found = found || e.Text == want
		}
		if !found {
			t.Errorf("500 x 500 row lacks %q: %s", want, visSummary(drawn[i], 6))
		}
	}
}

func TestWarnedSizesStaySelectable(t *testing.T) {
	total := sizeNoteTotal()
	a, f, _ := ngTest(t, withHardware(total))
	toLanding(t, a)
	f.Frame(t, a)
	openAdvanced(t, a, f)
	for i, o := range mapSizeOptions() {
		if mapSizeNote(o.Side, total) == "" || !o.Enabled {
			continue
		}
		w, ok := a.uiSt().Find(optID("land.mapsize", i))
		if !ok || w.Disabled {
			t.Fatalf("%d: warned size row present=%v disabled=%v", o.Side, ok, w.Disabled)
		}
		press(t, a, f, optID("land.mapsize", i))
		if a.ng.mapSize != o.Side {
			t.Errorf("clicking %d x %d left the map at %d", o.Side, o.Side, a.ng.mapSize)
		}
	}
}

func TestSizePickerNeverWarnsOn16GiB(t *testing.T) {
	a, f, _ := ngTest(t, withHardware(16*gib))
	toLanding(t, a)
	f.Frame(t, a)
	openAdvanced(t, a, f)
	for _, e := range drawnText(t, a, a.sw, a.sh) {
		if strings.HasPrefix(e.Text, "Needs about") || strings.HasPrefix(e.Text, "Large map") {
			t.Errorf("16 GiB shows %q", e.Text)
		}
	}
}

func TestSizePickerUnknownHardwareNotesLargeMapsOnly(t *testing.T) {
	a, f, _ := ngTest(t, nil)
	toLanding(t, a)
	f.Frame(t, a)
	openAdvanced(t, a, f)
	opts, _, drawn := sizeRows(t, a)
	for i, o := range opts {
		got := false
		for _, e := range drawn[i] {
			if e.Text == "Large map: needs a capable computer" {
				got = true
			}
			if strings.HasPrefix(e.Text, "Needs about") {
				t.Errorf("%d: a RAM figure with unknown hardware: %q", o.Side, e.Text)
			}
		}
		if got != (o.Side >= 350) {
			t.Errorf("%d: large-map note drawn = %v", o.Side, got)
		}
	}
}

func TestSizeWarningsAtUIScales(t *testing.T) {
	total := sizeNoteTotal()
	for _, s := range []float64{1, 1.5, 2} {
		t.Run(fmt.Sprintf("x%v", s), func(t *testing.T) {
			a, f, _ := ngTest(t, withHardware(total))
			toLanding(t, a)
			f.Frame(t, a)
			openAdvanced(t, a, f)
			pw, ph := atLogical1280(t, a, f, s)
			f.Frame(t, a)
			ev := drawnText(t, a, pw, ph)
			found := 0
			for _, e := range ev {
				if !strings.HasPrefix(e.Text, "Needs about") {
					continue
				}
				found++
				var row rect
				for i := range mapSizeOptions() {
					w, _ := a.uiSt().Find(optID("land.mapsize", i))
					if e.Y >= w.R.Y*float32(s)-0.5 && e.Y+e.H <= w.R.Bottom()*float32(s)+0.5 {
						row = w.R
					}
				}
				if row.Empty() {
					t.Errorf("warning %q at y=%g is on no size row", e.Text, e.Y)
					continue
				}
				if e.X < row.X*float32(s)-0.5 || e.X+e.W > row.Right()*float32(s)+0.5 {
					t.Errorf("warning %q (%g..%g) leaves its row %+v", e.Text, e.X, e.X+e.W, row)
				}
				// It must not run into the size label at the row's left.
				me := rect{X: e.X, Y: e.Y, W: e.W, H: e.H}
				for _, l := range ev {
					if strings.Contains(l.Text, " x ") && rectsOverlap(me, rect{X: l.X, Y: l.Y, W: l.W, H: l.H}) {
						t.Errorf("warning %q overlaps the label %q", e.Text, l.Text)
					}
				}
			}
			if found != 3 {
				t.Errorf("%d warnings drawn, want 3", found)
			}
		})
	}
}

// ---- the site panel ------------------------------------------------------------

func TestTileInfoUsesTheSameRule(t *testing.T) {
	a, _ := landingApp(t)
	p := a.ng.planet
	land := -1
	for i := 0; i < p.TileCount() && land < 0; i++ {
		if ok, _ := p.ValidLanding(i); ok {
			land = i
		}
	}
	total := sizeNoteTotal()
	for _, c := range []struct {
		side  int
		total uint64
	}{{300, total}, {400, total}, {500, total}, {300, 0}, {349, 0}, {350, 0}, {500, 0}, {500, 16 * gib}} {
		want := mapSizeNote(c.side, c.total)
		warns := buildTileInfo(p, land, c.side, c.total).warnings
		for _, w := range warns {
			if strings.HasPrefix(w, "Needs about") || strings.HasPrefix(w, "Large map") {
				if w != want {
					t.Errorf("side %d total %d: panel says %q, picker says %q", c.side, c.total, w, want)
				}
			}
		}
		if got := containsStr(warns, want); (want != "") != got {
			t.Errorf("side %d total %d: panel warnings %v, want %q", c.side, c.total, warns, want)
		}
	}
	// The old fixed 400 line is gone: a 400 map on a big machine is quiet.
	if w := buildTileInfo(p, land, 400, 16*gib).warnings; containsStr(w, "Large map: needs a capable computer") {
		t.Errorf("400 on 16 GiB still warns: %v", w)
	}
	// Unknown hardware: the gentle note at 350 and up, same as the picker.
	if w := buildTileInfo(p, land, 350, 0).warnings; !containsStr(w, "Large map: needs a capable computer") {
		t.Errorf("350 with unknown hardware: %v", w)
	}
}

func TestSitePanelShowsTheRAMWarning(t *testing.T) {
	total := sizeNoteTotal()
	a, f, _ := ngTest(t, withHardware(total))
	toLanding(t, a)
	click(t, a, "land.random")
	a.ng.mapSize = 500
	f.Frame(t, a)
	f.Frame(t, a)
	want := mapSizeNote(500, total)
	found := false
	for _, e := range drawnText(t, a, a.sw, a.sh) {
		found = found || e.Text == want
	}
	if !found {
		t.Errorf("the site panel does not show %q", want)
	}
	a.ng.mapSize = 300
	f.Frame(t, a)
	f.Frame(t, a)
	for _, e := range drawnText(t, a, a.sw, a.sh) {
		if strings.HasPrefix(e.Text, "Needs about") {
			t.Errorf("a 300 map on this machine fits but the panel says %q", e.Text)
		}
	}
}
