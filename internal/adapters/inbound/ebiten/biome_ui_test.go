package ebiten

import (
	artpkg "rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/inspector"
	"rimworld-go/internal/adapters/inbound/ebiten/mainmenu"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/render/creature"
	motionpkg "rimworld-go/internal/adapters/inbound/ebiten/render/motion"
	spritepkg "rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/adapters/inbound/ebiten/render/world"
	"rimworld-go/internal/adapters/inbound/ebiten/windows"
	"rimworld-go/internal/adapters/inbound/ebiten/wizard"
	"slices"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/biome"
)

// worldSim is a stub simulation with a canned world.
type worldSim struct {
	*stubSim
	w game.WorldView
}

func (s *worldSim) World() game.WorldView { return s.w }

func appWith(sim Simulation, mod func(*AppConfig), got *[]NewGame) *App {
	cfg := AppConfig{
		Version:       "test",
		LoadArt:       func() (*artpkg.Catalog, error) { return nil, nil },
		LoadSplash:    func() (*ebiten.Image, error) { return nil, nil },
		LoadMenuScene: func() (*mainmenu.Diorama, error) { return nil, nil },
		NewSim: func(ng NewGame) (Simulation, error) {
			if got != nil {
				*got = append(*got, ng)
			}
			return sim, nil
		},
	}
	if mod != nil {
		mod(&cfg)
	}
	return NewApp(cfg)
}

func TestPickerListsEveryBiomeAndStartPassesTheChoice(t *testing.T) {
	t.Parallel()
	var got []NewGame
	a := appWith(&stubSim{w: 4, h: 4}, func(c *AppConfig) { c.DefaultSeed = 4242 }, &got)
	a.finishBoot()
	toLanding(t, a)
	// The old biome list is gone: the globe's tile decides the biome.
	if hasWidget(a, "landing.biome.0") {
		t.Fatal("the landing screen no longer lists biomes")
	}
	click(t, a, "land.random")
	tile := a.ng.Tile
	if tile < 0 {
		t.Fatal("Select random site must choose a tile")
	}
	want := a.ng.Planet.Biome[tile]
	click(t, a, "wizard.next")
	click(t, a, "wizard.start")
	if len(got) != 1 || got[0].Biome != want || got[0].Setup == nil || got[0].Setup.Tile != tile || got[0].Seed != got[0].Setup.Seed {
		t.Fatalf("NewSim got %+v, want the tile's biome %v and its setup", got, want)
	}
}

func TestLandingDetailShowsClimateGroundAndWildlife(t *testing.T) {
	t.Parallel()
	id, _ := biome.Parse("boreal_forest")
	lines := strings.Join(wizard.LandingDetail(game.BiomeInfoOf(id), 60), "\n")
	for _, want := range []string{"Boreal forest", "winter low", "summer high", "Growing", "Ground", "Plants", "Wildlife"} {
		if !strings.Contains(lines, want) {
			t.Errorf("detail lacks %q:\n%s", want, lines)
		}
	}
	for _, b := range game.Biomes() {
		if len(b.Wildlife) > 5 {
			t.Errorf("%s lists %d wildlife, want at most 5", b.Key, len(b.Wildlife))
		}
	}
}

func TestWorldStepSeedFieldAndDice(t *testing.T) {
	t.Parallel()
	a := appWith(&stubSim{w: 4, h: 4}, func(c *AppConfig) { c.DefaultSeed = 99 }, nil)
	a.finishBoot()
	click(t, a, "menu.newgame")
	click(t, a, "wizard.next")
	click(t, a, "wizard.next")
	ng := a.wiz.Draft()
	if ng.World.SeedText != "99" {
		t.Fatalf("seed = %q, want the configured 99", ng.World.SeedText)
	}
	click(t, a, "world.dice")
	if ng.World.SeedText == "99" || ng.World.SeedText == "" {
		t.Fatalf("dice kept the seed: %q", ng.World.SeedText)
	}
	ng.World.SeedText, ng.World.Coverage = tinySeed, 5
	click(t, a, "world.generate")
	runUntil(t, a, func() bool { return a.nav.screen() == ports.ScreenLanding })
	if ng.World.SeedText != tinySeed || ng.Planet == nil {
		t.Fatalf("generate: seed=%q planet=%v", ng.World.SeedText, ng.Planet != nil)
	}
}

func TestAnimalInspectorLines(t *testing.T) {
	t.Parallel()
	a := testApp(nil) // the features hang off NewApp (ARCH-005)
	doe := game.AnimalView{Species: "goat", Tame: true, Female: true, State: "feeding", Food: 78, Rest: 61,
		AgeDays: 34, Adult: true, Stage: "adult", Diet: "plants", Pregnant: true, PregPct: 40, Product: "milk", ProductPct: 70, Scale: 1}
	if got := a.insp.Animal().AnimalStateLine(doe); got != "Feeding" {
		t.Errorf("state = %q", got)
	}
	if got := inspector.AnimalLifeLine(doe); got != "Female | adult | 34 days old" {
		t.Errorf("life = %q", got)
	}
	if got := inspector.AnimalPregLine(doe); got != "Pregnant 40%" {
		t.Errorf("preg = %q", got)
	}
	if got := inspector.AnimalDietLine(doe); got != "Diet: plants" {
		t.Errorf("diet = %q", got)
	}
	// Food + Rest, the product, the pregnancy and the diet (the pane's lines)
	ls := a.insp.Animal().AnimalPaneLines(doe)
	for _, want := range []string{"Pregnant 40%", "Diet: plants", "Milk: 70%"} {
		if !slices.Contains(ls, want) {
			t.Errorf("pane lines %q lack %q", ls, want)
		}
	}
	if !inspector.AnimalHasNeeds(doe) {
		t.Error("a goat has Food and Rest")
	}
	kid := game.AnimalView{State: "resting", Female: false, AgeDays: 6, Stage: "young", Scale: 0.6}
	if got := inspector.AnimalLifeLine(kid); got != "Male | young | 6 days old" {
		t.Errorf("juvenile = %q", got)
	}
	wild := game.AnimalView{State: "wandering"}
	if got := a.insp.Animal().AnimalStateLine(wild); got != "Wild | Wandering" {
		t.Errorf("wild state = %q", got)
	}
	if inspector.AnimalHasNeeds(game.AnimalView{}) {
		t.Error("an animal view without needs has no Food and Rest")
	}
}

func TestJuvenilePoseShrinks(t *testing.T) {
	t.Parallel()
	ps := creature.YoungPose(motionpkg.Pose{Px: 0.5, Py: 0.55, Sy: 1.01}, 0.6)
	if ps.Sx != 0.6 || ps.Sy < 0.6 || ps.Sy > 0.61 {
		t.Fatalf("pose = %+v", ps)
	}
	if creature.YoungPose(motionpkg.Pose{}, 1).Sx != 0 || creature.YoungPose(motionpkg.Pose{}, 0).Sx != 0 {
		t.Fatal("an adult keeps its pose")
	}
}

func wildWorld() game.WorldView {
	return game.WorldView{Animals: []game.AnimalView{
		{ID: 1, Species: "deer", Label: "deer", X: 3, Y: 4},
		{ID: 2, Species: "deer", Label: "deer", X: 8, Y: 9, Designated: true},
		{ID: 3, Species: "goat", Label: "goat", Tame: true, Name: "Pip", X: 1, Y: 1},
	}}
}

func TestWildTabRowsAndButtonsEmitCommands(t *testing.T) {
	t.Parallel()
	sim := &worldSim{stubSim: &stubSim{w: 16, h: 16, wild: []game.WildlifeRow{
		{Species: "deer", Label: "deer", Count: 2, Diet: "plants", TameChance: 35, Meat: 40, Hunted: 1},
	}}, w: wildWorld()}
	a := appWith(sim, nil, nil)
	a.finishBoot()
	a.startGame()
	click(t, a, "tab.animals")
	click(t, a, "animals.view.wild")
	if !a.win.Wild {
		t.Fatal("the Wild toggle shows the wild list")
	}
	click(t, a, "animals.wild.hunt.deer")
	click(t, a, "animals.wild.tame.deer")
	click(t, a, "animals.wild.cancel.deer")
	var hunt, tame, undo int
	for _, c := range sim.cmds {
		switch c.Kind {
		case command.DesignateHunt:
			hunt++
			if c.X != 3 || c.Y != 4 {
				t.Errorf("hunt all marks the unmarked deer, got %+v", c)
			}
		case command.DesignateTame:
			tame++
		case command.Undesignate:
			undo++
			if c.Order != command.OrderHunt || c.X != 8 {
				t.Errorf("cancel clears the marked deer: %+v", c)
			}
		}
	}
	if hunt != 1 || tame != 1 || undo != 1 {
		t.Fatalf("hunt/tame/cancel = %d/%d/%d, want 1/1/1: %+v", hunt, tame, undo, sim.cmds)
	}
	click(t, a, "animals.focus.deer")
	click(t, a, "animals.focus.deer") // cycles to the second deer without panicking
	click(t, a, "animals.view.colony")
	if a.win.Wild {
		t.Fatal("the Colony toggle shows the colony list")
	}
}

func TestColonyListColumns(t *testing.T) {
	t.Parallel()
	an := game.AnimalView{Female: true, AgeDays: 20, Adult: true, State: "feeding", Product: "wool", ProductPct: 55, Pregnant: true, PregPct: 40}
	for got, want := range map[string]string{
		windows.AnimalSexLetter(an):                  "F",
		windows.AnimalAgeText(an):                    "20d",
		windows.AnimalProductText(an):                "wool 55%",
		windows.AnimalPregCell(an):                   "40%",
		windows.AnimalDoing(an):                      "feeding",
		windows.AnimalProductText(game.AnimalView{}): "-",
	} {
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}
	if got := windows.AnimalAgeText(game.AnimalView{AgeDays: 5, State: "resting"}); !strings.Contains(got, "young") {
		t.Errorf("juvenile age = %q", got)
	}
}

func TestTroughFilterEmitsCommand(t *testing.T) {
	t.Parallel()
	b := game.BuildingView{ID: 77, Kind: "feeding trough", X: 4, Y: 4, W: 1, H: 2, TroughFill: 12, TroughCap: 60, TroughFilter: "plants"}
	sim := &worldSim{stubSim: &stubSim{w: 16, h: 16}, w: game.WorldView{Buildings: []game.BuildingView{b}}}
	a := appWith(sim, nil, nil)
	a.finishBoot()
	a.startGame()
	a.insp.Building().SelectBuildingAt(4, 4)
	if _, ok := a.insp.SelectedBuilding(); !ok {
		t.Fatal("the trough should be selected")
	}
	// the filter is one command button with the three filters as options
	// (PLAY-018); picking an option enqueues its command
	filter, ok := bldCmd(a.insp.Building().BuildingCommands(77), "cmd.bldg.trough")
	if !ok || filter.Options == nil {
		t.Fatal("the trough should offer its filter options")
	}
	opts := filter.Options()
	if len(opts) != 3 || opts[0].Label != "Plants" || opts[1].Label != "Meat" || opts[2].Label != "Any" {
		t.Fatalf("options = %+v", opts)
	}
	if !opts[0].Disabled {
		t.Fatal("the current filter (plants) is not offered again")
	}
	a.renderer.enqueue(*opts[1].Cmd) // Meat
	a.renderer.enqueue(*opts[2].Cmd) // Any
	var modes []uint8
	for _, c := range sim.cmds {
		if c.Kind == command.SetTroughFilter {
			if c.Target != 77 {
				t.Errorf("target = %d", c.Target)
			}
			modes = append(modes, c.Mode)
		}
	}
	if len(modes) != 2 || modes[0] != 1 || modes[1] != 2 {
		t.Fatalf("modes = %v, want [1 2] (meat, any)", modes)
	}
}

// PLAY-015: the trough's herd is the tame animals in the marker pen next to
// it (the trough's own tile blocks movement, so it is not a pen cell).
func TestPenAnimalsListsTheTroughsHerd(t *testing.T) {
	t.Parallel()
	cell := func(x, y int) uint64 { return 1 << (y*10 + x) }
	w := game.WorldView{
		TerrainW: 10,
		Pens: []game.PenView{
			{Enclosed: true, Bits: []uint64{cell(1, 1) | cell(2, 1) | cell(1, 2)}},
			{Enclosed: false, X: 2, Y: 3}, // an open pen holds nothing
		},
		Animals: []game.AnimalView{
			{ID: 1, Tame: true, X: 1, Y: 1}, {ID: 2, Tame: true, X: 9, Y: 9}, {ID: 3, Tame: false, X: 2, Y: 1},
		},
	}
	got := inspector.PenAnimals(w, 2, 2)
	if len(got) != 1 || got[0].ID != 1 {
		t.Fatalf("pen animals = %+v", got)
	}
	if inspector.PenAnimals(w, 8, 8) != nil {
		t.Fatal("a trough by no pen has no herd")
	}
}

func TestMessageFocusJumpsToTheCreature(t *testing.T) {
	t.Parallel()
	sim := &worldSim{stubSim: &stubSim{w: 40, h: 40}, w: game.WorldView{
		Animals:  []game.AnimalView{{ID: entity.EntityID(5), Species: "deer", X: 30, Y: 31}},
		Messages: []game.MessageView{{Tick: 1, Text: "A deer gave birth", Focus: 5}},
	}}
	a := appWith(sim, nil, nil)
	a.finishBoot()
	a.startGame()
	a.sw, a.sh = 1280, 720
	found := false
	for _, b := range a.chrome.Right().MessageButtons(a.sw) {
		found = found || b.ID == "msg.0"
	}
	if !found {
		t.Fatal("a message with a Focus is clickable")
	}
	a.applyClick("msg.0")
	if !a.sel.HasAnimal || a.sel.SelAnimal != 5 {
		t.Fatalf("selected animal = %v/%d", a.sel.HasAnimal, a.sel.SelAnimal)
	}
}

func TestHandleWorkColumn(t *testing.T) {
	t.Parallel()
	if windows.WorkColumns[11] != "Hndl" {
		t.Fatalf("column 11 = %q", windows.WorkColumns[11])
	}
	if c, ok := windows.WorkTypeForCol(11); !ok || c != command.WorkHandle {
		t.Fatalf("column 11 maps to %v, %v", c, ok)
	}
	p := game.PawnView{Priorities: make([]int, 12)}
	p.Priorities[11] = 2
	if windows.WorkPriority(p, 11) != 2 {
		t.Fatal("the Handle priority reads from the pawn's priorities")
	}
}

func TestNewBiomeKeysFallBackWithoutArt(t *testing.T) {
	t.Parallel()
	r := &Renderer{}
	for _, terr := range []string{"sand", "marsh", "ice", "soil", "unheard_of"} {
		if k, _ := r.pipe().TerrainTileKey(terr, 1); k == "" {
			t.Errorf("terrain %q has no key", terr)
		}
	}
	// no art loaded: every new plant still resolves to a key
	for _, sp := range []string{"pine", "jungle_tree", "cactus", "shrub", "reeds"} {
		k, sc, ok := world.NewPlantKey(game.PlantView{Species: sp, X: 1, Y: 2, Growth: 50}, 7)
		if !ok || k == "" || sc <= 0 {
			t.Errorf("plant %q: %q %v %v", sp, k, sc, ok)
		}
		if got := r.seasonKey(k, game.WorldView{Season: "Winter"}); got == "" {
			t.Errorf("plant %q has no season key", sp)
		}
	}
	for _, kind := range []string{"milk", "wool", "egg", "fertile egg", "ember oil", "wool parka", "wool carpet"} {
		if world.ItemSprite(kind) == "" {
			t.Errorf("item %q has no sprite key", kind)
		}
	}
	if k := world.TroughKey(game.BuildingView{TroughFill: 3, Facing: "east"}); k != "furniture/feeding_trough_full_east" {
		t.Errorf("trough key = %q", k)
	}
	if world.BuildSprite("feeding trough", "", 0, false, false) == "" {
		t.Error("the trough ghost has a sprite")
	}
	for _, d := range game.Biomes() {
		_ = spritepkg.GrassTint(d.Key)
	}
	if spritepkg.GrassTint("desert") == spritepkg.NoCS || spritepkg.GrassTint("temperate_forest") != spritepkg.NoCS {
		t.Error("desert grass is tinted dry; temperate stays as drawn")
	}
}
