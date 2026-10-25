package ebiten

import (
	"errors"
	"strings"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
)

// fieldsSim is a stub that also offers the map field layers.
type fieldsSim struct {
	*stubSim
	layers []game.MapFieldLayer
	err    error
	calls  int
}

func (f *fieldsSim) MapFieldLayers() ([]game.MapFieldLayer, error) { f.calls++; return f.layers, f.err }

func testFieldLayers() []game.MapFieldLayer {
	return []game.MapFieldLayer{
		{Name: "Elevation", W: 2, H: 2, Scalar: []float32{0, 0.5, 1, 0.25}},
		{Name: "Caves", W: 2, H: 2, Classes: []uint8{0, 1, 2, 1}, Legend: []game.LegendEntry{
			{Label: "open", RGBA: [4]uint8{10, 20, 30, 255}}, {Label: "rock", RGBA: [4]uint8{200, 0, 0, 255}}, {Label: "cave", RGBA: [4]uint8{0, 200, 0, 255}}}},
		{Name: "Start score", W: 2, H: 2, Scalar: []float32{0, 0, 0, 0}, Marks: [][2]int{{1, 1}}},
	}
}

func fieldsApp(t *testing.T, fs *fieldsSim) *App {
	t.Helper()
	if fs.stubSim == nil {
		fs.stubSim = &stubSim{}
	}
	a, _ := colonyApp(t, fs.stubSim)
	a.renderer.game = fs
	set := a.settings
	set.Gameplay.DevMode = true
	a.setSettings(set)
	return a
}

func mapFieldsAction(t *testing.T, a *App) devAction {
	t.Helper()
	for _, x := range a.devActions(1, 1) {
		if strings.HasPrefix(x.label, "Map fields") {
			return x
		}
	}
	t.Fatal("dev menu has no Map fields entry")
	return devAction{}
}

func TestDevMenuMapFieldsCyclesThroughEveryLayerBackToOff(t *testing.T) {
	t.Parallel()
	fs := &fieldsSim{layers: testFieldLayers()}
	a := fieldsApp(t, fs)
	if !strings.HasSuffix(mapFieldsAction(t, a).label, "off") {
		t.Fatalf("starts off: %q", mapFieldsAction(t, a).label)
	}
	for _, want := range []string{"Elevation", "Caves", "Start score"} {
		mapFieldsAction(t, a).run(nil)
		if got := mapFieldsAction(t, a).label; !strings.HasSuffix(got, want) {
			t.Fatalf("label %q, want ...%s", got, want)
		}
	}
	mapFieldsAction(t, a).run(nil)
	if !strings.HasSuffix(mapFieldsAction(t, a).label, "off") || a.dev.fields.on() {
		t.Fatal("cycle must return to off")
	}
}

func TestDevMenuMapFieldsReportsAnError(t *testing.T) {
	t.Parallel()
	a := fieldsApp(t, &fieldsSim{err: errors.New("map made by an older generator")})
	if msg := mapFieldsAction(t, a).run(nil); !strings.Contains(msg, "older generator") || a.dev.fields.on() {
		t.Fatalf("msg %q on=%v", msg, a.dev.fields.on())
	}
}

func TestMapFieldsActionNeedsASource(t *testing.T) {
	t.Parallel()
	a, _ := colonyApp(t, &stubSim{})
	set := a.settings
	set.Gameplay.DevMode = true
	a.setSettings(set)
	if x := mapFieldsAction(t, a); x.reason == "" {
		t.Fatal("a game without map fields disables the entry")
	}
}

func TestMapFieldsImageBuiltOncePerSelection(t *testing.T) {
	t.Parallel()
	fs := &fieldsSim{layers: testFieldLayers()}
	a := fieldsApp(t, fs)
	mapFieldsAction(t, a).run(nil)
	o := &a.dev.fields
	img := o.image()
	if img == nil || o.image() != img || o.builds != 1 {
		t.Fatalf("built %d times", o.builds)
	}
	mapFieldsAction(t, a).run(nil)
	if o.image() == nil || o.builds != 2 {
		t.Fatalf("a new selection builds once more, got %d", o.builds)
	}
}

func TestMapFieldPixels(t *testing.T) {
	t.Parallel()
	l := testFieldLayers()
	px := fieldPixels(l[1]) // classes
	if len(px) != 16 || px[3] != fieldAlpha {
		t.Fatalf("pixels %v", px)
	}
	if px[4] != 200*fieldAlpha/255 { // rock red, premultiplied
		t.Fatalf("premultiplied red = %d", px[4])
	}
	s := fieldPixels(l[0])
	if s[3] != fieldAlpha || s[0] == s[8] && s[1] == s[9] && s[2] == s[10] {
		t.Fatalf("scalar ramp should differ between 0 and 1: %v", s)
	}
	m := fieldPixels(l[2]) // mark at (1,1)
	if m[12] == m[0] && m[13] == m[1] {
		t.Fatal("mark not painted")
	}
}

func TestMapFieldLegendListsClasses(t *testing.T) {
	t.Parallel()
	l := testFieldLayers()
	got := fieldLegendLines(l[1])
	if len(got) != 3 || got[0] != "open" || got[2] != "cave" {
		t.Fatalf("legend %v", got)
	}
	if s := fieldLegendLines(l[0]); len(s) != 2 {
		t.Fatalf("scalar legend is low/high: %v", s)
	}
}

func TestMapFieldsOffWhenLeavingDevMode(t *testing.T) {
	t.Parallel()
	a := fieldsApp(t, &fieldsSim{layers: testFieldLayers()})
	mapFieldsAction(t, a).run(nil)
	set := a.settings
	set.Gameplay.DevMode = false
	a.setSettings(set)
	if a.dev.fields.on() {
		t.Fatal("leaving developer mode clears the overlay")
	}
}

func TestMapFieldsDrawMapAndLegend(t *testing.T) {
	t.Parallel()
	a := fieldsApp(t, &fieldsSim{layers: testFieldLayers()})
	screen := ebiten.NewImage(320, 240)
	for range 4 { // every layer, then off
		a.drawMapFields(screen)
		a.drawMapFieldLegend(screen)
		mapFieldsAction(t, a).run(nil)
	}
	if a.dev.fields.on() {
		t.Fatal("cycled back to off")
	}
}
