package ebiten

import (
	"errors"
	"strings"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/devtools"
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

func mapFieldsAction(t *testing.T, a *App) devtools.Action {
	t.Helper()
	for _, x := range a.tools.Actions(1, 1) {
		if strings.HasPrefix(x.Label, "Map fields") {
			return x
		}
	}
	t.Fatal("dev menu has no Map fields entry")
	return devtools.Action{}
}

func TestDevMenuMapFieldsCyclesThroughEveryLayerBackToOff(t *testing.T) {
	t.Parallel()
	fs := &fieldsSim{layers: testFieldLayers()}
	a := fieldsApp(t, fs)
	if !strings.HasSuffix(mapFieldsAction(t, a).Label, "off") {
		t.Fatalf("starts off: %q", mapFieldsAction(t, a).Label)
	}
	for _, want := range []string{"Elevation", "Caves", "Start score"} {
		mapFieldsAction(t, a).Run(nil)
		if got := mapFieldsAction(t, a).Label; !strings.HasSuffix(got, want) {
			t.Fatalf("label %q, want ...%s", got, want)
		}
	}
	mapFieldsAction(t, a).Run(nil)
	if !strings.HasSuffix(mapFieldsAction(t, a).Label, "off") || a.tools.State.Fields.On() {
		t.Fatal("cycle must return to off")
	}
}

func TestDevMenuMapFieldsReportsAnError(t *testing.T) {
	t.Parallel()
	a := fieldsApp(t, &fieldsSim{err: errors.New("map made by an older generator")})
	if msg := mapFieldsAction(t, a).Run(nil); !strings.Contains(msg, "older generator") || a.tools.State.Fields.On() {
		t.Fatalf("msg %q on=%v", msg, a.tools.State.Fields.On())
	}
}

func TestMapFieldsActionNeedsASource(t *testing.T) {
	t.Parallel()
	a, _ := colonyApp(t, &stubSim{})
	set := a.settings
	set.Gameplay.DevMode = true
	a.setSettings(set)
	if x := mapFieldsAction(t, a); x.Reason == "" {
		t.Fatal("a game without map fields disables the entry")
	}
}

func TestMapFieldsImageBuiltOncePerSelection(t *testing.T) {
	t.Parallel()
	fs := &fieldsSim{layers: testFieldLayers()}
	a := fieldsApp(t, fs)
	mapFieldsAction(t, a).Run(nil)
	o := &a.tools.State.Fields
	img := o.Image()
	if img == nil || o.Image() != img || o.Builds != 1 {
		t.Fatalf("built %d times", o.Builds)
	}
	mapFieldsAction(t, a).Run(nil)
	if o.Image() == nil || o.Builds != 2 {
		t.Fatalf("a new selection builds once more, got %d", o.Builds)
	}
}

func TestMapFieldsOffWhenLeavingDevMode(t *testing.T) {
	t.Parallel()
	a := fieldsApp(t, &fieldsSim{layers: testFieldLayers()})
	mapFieldsAction(t, a).Run(nil)
	set := a.settings
	set.Gameplay.DevMode = false
	a.setSettings(set)
	if a.tools.State.Fields.On() {
		t.Fatal("leaving developer mode clears the overlay")
	}
}

func TestMapFieldsDrawMapAndLegend(t *testing.T) {
	t.Parallel()
	a := fieldsApp(t, &fieldsSim{layers: testFieldLayers()})
	screen := ebiten.NewImage(320, 240)
	for range 4 { // every layer, then off
		a.tools.DrawMapFields(screen)
		a.tools.DrawMapFieldLegend(screen)
		mapFieldsAction(t, a).Run(nil)
	}
	if a.tools.State.Fields.On() {
		t.Fatal("cycled back to off")
	}
}
