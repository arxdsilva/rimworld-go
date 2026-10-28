package windows

import (
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/ids"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/ai/work"
	"rimworld-go/internal/domain/pawn/drug"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/planet"
)

// fakeReader is a colony with no wildlife, history or world map.
type fakeReader struct{}

func (fakeReader) Wildlife() []game.WildlifeRow                      { return nil }
func (fakeReader) History() []game.HistoryEntryView                  { return nil }
func (fakeReader) WorldMap() game.WorldMapView                       { return game.WorldMapView{Tile: -1} }
func (fakeReader) IncapableReason(entity.EntityID, work.Type) string { return "" }

// The lists keep every column when the window has room and leave the
// low-priority ones out, never the buttons, when it does not.
func TestAnimalColumnsDropToFit(t *testing.T) {
	t.Parallel()
	if c := animalColsFor(colonyFullW, false); len(c) != 0 {
		t.Fatalf("a full-width window drops %v", c)
	}
	if c := animalColsFor(wildFullW, true); len(c) != 0 {
		t.Fatalf("a full-width wild list drops %v", c)
	}
	for _, w := range []float32{1672, 1248, 1032, 900, 865, 830} {
		c := animalColsFor(w, false)
		if end := float32(c.at(colBtn) + 196 + 12); end > w {
			t.Errorf("colony list at %v px ends at %v with %d dropped", w, end, len(c))
		}
		if c.at(colBtn) < colMaster || !c.shown(colName) || !c.shown(colMaster) || !c.shown(colArea) || !c.shown(colFollow) {
			t.Errorf("colony list at %v px lost a needed column: %v", w, c)
		}
	}
	for _, w := range []float32{984, 865, 700} {
		c := animalColsFor(w, true)
		if end := float32(c.at(wildBtn) + 272 + 12); end > w {
			t.Errorf("wild list at %v px ends at %v", w, end)
		}
	}
	// Doing and Training go first, then Preg., and the columns before a
	// dropped one stay put
	c := animalColsFor(1100, false)
	if c.shown(colDoing) || c.shown(colTrain) || !c.shown(colPreg) || c.at(colBtn) != colBtn-108-104 || c.at(colMaster) != colMaster {
		t.Errorf("1100 px drops %v, buttons at %d, Master at %d", c, c.at(colBtn), c.at(colMaster))
	}
	c = animalColsFor(1032, false)
	if c.shown(colPreg) || c.at(colBtn) != colBtn-108-104-44 || c.at(colMaster) != colMaster-44 || c.at(colHealth) != colHealth-44 || c.at(colProduct) != colProduct {
		t.Errorf("1032 px drops %v, buttons at %d, Master at %d", c, c.at(colBtn), c.at(colMaster))
	}
}

func TestNextPresetCycles(t *testing.T) {
	t.Parallel()
	p := drug.NoDrugs
	seen := map[drug.Preset]bool{}
	for i := 0; i < len(AssignPresets); i++ {
		seen[p] = true
		p = NextPreset(p)
	}
	if p != drug.NoDrugs || len(seen) != 3 {
		t.Fatalf("cycle ends at %v after %v", p, seen)
	}
	if PresetLabel(drug.Unrestricted) != "Unrestricted" {
		t.Fatal(PresetLabel(drug.Unrestricted))
	}
}

// The World tab charts the colony's planet a slice per frame, once: a
// reopened tab finds it in the cache.
func TestWorldTabChartsThePlanetOnce(t *testing.T) {
	t.Parallel()
	_, w := newFake()
	st := &worldUI{picked: -1, hasMap: true,
		params: planet.Params{SeedText: "world-tab", Coverage: 5, Rain: 2, Temp: 2, Pop: 1}}
	var p *planet.Planet
	for i := 0; i < 400 && p == nil; i++ {
		p = w.World().planetFor(st)
	}
	if p == nil {
		t.Fatal("the world was never charted")
	}
	if q := w.World().planetFor(&worldUI{params: st.params, hasMap: true}); q != p {
		t.Fatal("a reopened tab charts the world again")
	}
}

// A kit window opens in the Window rect replacing the open window (the
// Architect and the order menu give way), a second open of the same id
// does nothing, its tab toggles it shut, and a quiet close skips OnClose.
func TestKitWindowOpensTogglesAndCloses(t *testing.T) {
	t.Parallel()
	sh, w := newFake()
	sh.env.Game = fakeReader{}
	w.Show("work")
	w.Assign().Launch()
	if !w.KitOpen() || w.KitID() != AssignModalID || w.Mode != AssignModalID {
		t.Fatalf("Assign not open: mode %q kit %q", w.Mode, w.KitID())
	}
	if sw, shh := w.Want(); sw != assignW || shh != assignH || w.Anchor() != sh.WindowRect() {
		t.Fatalf("want %vx%v anchor %v", sw, shh, w.Anchor())
	}
	if len(sh.intents) != 2 || sh.intents[1] != (ports.CloseArchitect{}) || sh.floats != 2 {
		t.Fatalf("intents %v floats %d", sh.intents, sh.floats)
	}
	w.Assign().Launch() // already open: nothing
	if len(sh.intents) != 2 {
		t.Fatalf("a second open showed the window again: %v", sh.intents)
	}
	w.Assign().Toggle()
	if w.Open || w.KitID() != "" {
		t.Fatal("the tab did not close Assign")
	}
	closed := 0
	w.OpenKit(uikit.Modal{ID: "x", W: 10, H: 10, OnClose: func() { closed++ }})
	w.CloseModal("x")
	if w.Open || closed != 0 {
		t.Fatalf("quiet close: open %v closed %d", w.Open, closed)
	}
	w.OpenKit(uikit.Modal{ID: "x", OnClose: func() { closed++ }})
	w.Click("win.close")
	if w.Open || closed != 1 {
		t.Fatalf("the X: open %v closed %d", w.Open, closed)
	}
}

// The hand-drawn windows lay their buttons out in the Window rect, and
// their clicks become commands.
func TestHandDrawnWindowsButtonsAndClicks(t *testing.T) {
	t.Parallel()
	sh, w := newFake()
	sh.env.Game = fakeReader{}
	sh.pawns = []game.PawnView{{ID: 1, Name: "Ada", Status: "alive"}, {ID: 2, Name: "Bo", Status: "alive"}}
	sh.world.Research = []game.ResearchView{{ID: 3, Label: "Stonecutting", Available: true}, {ID: 4, Label: "Done", Done: true}}
	sh.world.Animals = []game.AnimalView{{ID: 50, Species: "dustbuck", Tame: true}, {ID: 51, Species: "dustbuck"}}
	if len(w.Buttons()) != 0 {
		t.Fatal("buttons with no window open")
	}
	w.Show("work")
	if got := len(w.Buttons()); got != 2*len(WorkLive) {
		t.Fatalf("work cells %d, want %d", got, 2*len(WorkLive))
	}
	if ww, wh := w.Want(); ww <= 0 || wh <= 0 {
		t.Fatalf("work wants %vx%v", ww, wh)
	}
	w.Show("schedule")
	if got := len(w.Buttons()); got != len(schedBrushes)+3*24 {
		t.Fatalf("schedule buttons %d", got)
	}
	w.Schedule().Paint(ids.PrefixSchedCell + "2.5")
	w.Show("research")
	bs := w.Buttons()
	if len(bs) != 2 || bs[0].Disabled || !bs[1].Disabled {
		t.Fatalf("research buttons %+v", bs)
	}
	w.Research().Pick(bs[0].ID)
	w.Show("animals")
	if got := len(w.Buttons()); got != 2+3 { // Colony / Wild, then Master, Slaughter, Release
		t.Fatalf("animals buttons %d", got)
	}
	w.Animals().Click("animals.release.50")
	want := []command.Kind{command.SetSchedule, command.SetResearch, command.ReleaseAnimal}
	if len(sh.cmds) != len(want) {
		t.Fatalf("commands %+v", sh.cmds)
	}
	for i, k := range want {
		if sh.cmds[i].Kind != k {
			t.Fatalf("command %d is %v, want %v", i, sh.cmds[i].Kind, k)
		}
	}
	if sh.cmds[0].Pawn != 2 || sh.cmds[0].Index != 5 || sh.cmds[1].Build != 3 || sh.cmds[2].Pawn != 50 {
		t.Fatalf("command fields %+v", sh.cmds)
	}
}
