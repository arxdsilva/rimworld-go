package ebiten

import (
	"strings"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

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
		LoadArt:       func() (*artCatalog, error) { return nil, nil },
		LoadSplash:    func() (*ebiten.Image, error) { return nil, nil },
		LoadMenuScene: func() (*menuDiorama, error) { return nil, nil },
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
	tile := a.ng.tile
	if tile < 0 {
		t.Fatal("Select random site must choose a tile")
	}
	want := a.ng.planet.Biome[tile]
	click(t, a, "wizard.next")
	click(t, a, "wizard.start")
	if len(got) != 1 || got[0].Biome != want || got[0].Setup == nil || got[0].Setup.Tile != tile || got[0].Seed != got[0].Setup.Seed {
		t.Fatalf("NewSim got %+v, want the tile's biome %v and its setup", got, want)
	}
}

func TestLandingDetailShowsClimateGroundAndWildlife(t *testing.T) {
	t.Parallel()
	id, _ := biome.Parse("boreal_forest")
	lines := strings.Join(landingDetail(game.BiomeInfoOf(id), 60), "\n")
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
	ng := a.draft()
	if ng.world.SeedText != "99" {
		t.Fatalf("seed = %q, want the configured 99", ng.world.SeedText)
	}
	click(t, a, "world.dice")
	if ng.world.SeedText == "99" || ng.world.SeedText == "" {
		t.Fatalf("dice kept the seed: %q", ng.world.SeedText)
	}
	ng.world.SeedText, ng.world.Coverage = tinySeed, 5
	click(t, a, "world.generate")
	runUntil(t, a, func() bool { return a.nav.screen() == screenLanding })
	if ng.world.SeedText != tinySeed || ng.planet == nil {
		t.Fatalf("generate: seed=%q planet=%v", ng.world.SeedText, ng.planet != nil)
	}
}

func TestTopBarNamesTheBiome(t *testing.T) {
	t.Parallel()
	got := seasonText(game.WorldView{Biome: "Boreal forest", BiomeKey: "boreal_forest", Season: "Spring", OutdoorTemp: -20})
	if got != "Boreal forest | Spring  -2C outside" {
		t.Fatalf("season text = %q", got)
	}
	if tip := strings.Join(biomeTooltip(game.WorldView{BiomeKey: "boreal_forest"}), "\n"); !strings.Contains(tip, "Growing days") || !strings.Contains(tip, "winter low") {
		t.Fatalf("tooltip = %q", tip)
	}
	if biomeTooltip(game.WorldView{}) != nil {
		t.Fatal("no biome key, no tooltip")
	}
}

func TestAnimalInspectorLines(t *testing.T) {
	t.Parallel()
	a := &App{}
	doe := game.AnimalView{Species: "goat", Tame: true, Female: true, State: "feeding", Food: 78, Rest: 61,
		AgeDays: 34, Adult: true, Stage: "adult", Diet: "plants", Pregnant: true, PregPct: 40, Product: "milk", ProductPct: 70, Scale: 1}
	if got := a.animalStateLine(doe); got != "Feeding" {
		t.Errorf("state = %q", got)
	}
	if got := animalLifeLine(doe); got != "Female | adult | 34 days old" {
		t.Errorf("life = %q", got)
	}
	if got := animalPregLine(doe); got != "Pregnant 40%" {
		t.Errorf("preg = %q", got)
	}
	if got := animalDietLine(doe); got != "Diet: plants" {
		t.Errorf("diet = %q", got)
	}
	// Food + Rest, product bar, pregnancy line, diet line
	if h := animalExtraH(doe); h != 40+20+16+16 {
		t.Errorf("extra height = %d", h)
	}
	kid := game.AnimalView{State: "resting", Female: false, AgeDays: 6, Stage: "young", Scale: 0.6}
	if got := animalLifeLine(kid); got != "Male | young | 6 days old" {
		t.Errorf("juvenile = %q", got)
	}
	wild := game.AnimalView{State: "wandering"}
	if got := a.animalStateLine(wild); got != "Wild | Wandering" {
		t.Errorf("wild state = %q", got)
	}
	if animalExtraH(game.AnimalView{}) != 0 {
		t.Error("an animal view without needs adds no height")
	}
}

func TestJuvenilePoseShrinks(t *testing.T) {
	t.Parallel()
	ps := youngPose(pose{px: 0.5, py: 0.55, sy: 1.01}, 0.6)
	if ps.sx != 0.6 || ps.sy < 0.6 || ps.sy > 0.61 {
		t.Fatalf("pose = %+v", ps)
	}
	if youngPose(pose{}, 1).sx != 0 || youngPose(pose{}, 0).sx != 0 {
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
	if !a.animalsWild {
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
	if a.animalsWild {
		t.Fatal("the Colony toggle shows the colony list")
	}
}

func TestColonyListColumns(t *testing.T) {
	t.Parallel()
	an := game.AnimalView{Female: true, AgeDays: 20, Adult: true, State: "feeding", Product: "wool", ProductPct: 55, Pregnant: true, PregPct: 40}
	for got, want := range map[string]string{
		animalSexLetter(an):                  "F",
		animalAgeText(an):                    "20d",
		animalProductText(an):                "wool 55%",
		animalPregCell(an):                   "40%",
		animalDoing(an):                      "feeding",
		animalProductText(game.AnimalView{}): "-",
	} {
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}
	if got := animalAgeText(game.AnimalView{AgeDays: 5, State: "resting"}); !strings.Contains(got, "young") {
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
	a.selectBuildingAt(4, 4)
	if _, ok := a.selectedBuilding(); !ok {
		t.Fatal("the trough should be selected")
	}
	click(t, a, "bldg.trough.meat")
	click(t, a, "bldg.trough.any")
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
	got := penAnimals(w, 2, 2)
	if len(got) != 1 || got[0].ID != 1 {
		t.Fatalf("pen animals = %+v", got)
	}
	if penAnimals(w, 8, 8) != nil {
		t.Fatal("a trough by no pen has no herd")
	}
}

func TestAnimalAlerts(t *testing.T) {
	t.Parallel()
	// PLAY-015: a 2x2 marker pen (cells 0, 1, 10, 11 of a 10-wide map), an
	// open pen and a roaming cow
	pen := game.PenView{ID: 3, Name: "Pen 1", Enclosed: true, Bits: []uint64{1<<0 | 1<<1 | 1<<10 | 1<<11}}
	open := game.PenView{ID: 7, Name: "Pen 2", X: 6, Y: 6}
	grow := game.ZoneView{ID: 4, Kind: game.ZoneGrow, Cells: [][2]int{{5, 5}}}
	w := game.WorldView{
		TerrainW: 10,
		Zones:    []game.ZoneView{grow},
		Pens:     []game.PenView{pen, open},
		Animals: []game.AnimalView{
			{ID: 1, Tame: true, State: "wandering", Food: 10, X: 0, Y: 0},
			{ID: 2, Tame: true, State: "wandering", Food: 90, X: 1, Y: 0},
			{ID: 3, Species: "deer", State: "feeding", X: 5, Y: 5},
			{ID: 4, Species: "timbergnaw", X: 9, Y: 9},
			{ID: 5, Species: "cow", Tame: true, Penned: true, Pen: "Roaming", Food: 90, X: 8, Y: 2},
		},
	}
	keys := map[string]bool{}
	for _, al := range animalAlerts(w) {
		keys[al.key] = true
	}
	for _, want := range []string{"animalhungry", "cropraid", "timbergnaw", "penfull.3", "penopen.7", "unpenned"} {
		if !keys[want] {
			t.Errorf("missing alert %q in %v", want, keys)
		}
	}
	if len(animalAlerts(game.WorldView{})) != 0 {
		t.Error("no animals, no alerts")
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
	for _, b := range a.messageButtons(a.sw) {
		found = found || b.id == "msg.0"
	}
	if !found {
		t.Fatal("a message with a Focus is clickable")
	}
	a.applyClick("msg.0")
	if !a.hasAnimal || a.selAnimal != 5 {
		t.Fatalf("selected animal = %v/%d", a.hasAnimal, a.selAnimal)
	}
}

func TestHandleWorkColumn(t *testing.T) {
	t.Parallel()
	if workColumns[11] != "Hndl" {
		t.Fatalf("column 11 = %q", workColumns[11])
	}
	if c, ok := workTypeForCol(11); !ok || c != command.WorkHandle {
		t.Fatalf("column 11 maps to %v, %v", c, ok)
	}
	p := game.PawnView{Priorities: make([]int, 12)}
	p.Priorities[11] = 2
	if workPriority(p, 11) != 2 {
		t.Fatal("the Handle priority reads from the pawn's priorities")
	}
}

func TestNewBiomeKeysFallBackWithoutArt(t *testing.T) {
	t.Parallel()
	r := &Renderer{}
	for _, terr := range []string{"sand", "marsh", "ice", "soil", "unheard_of"} {
		if k, _ := r.terrainTileKey(terr, 1); k == "" {
			t.Errorf("terrain %q has no key", terr)
		}
	}
	// no art loaded: every new plant still resolves to a key
	for _, sp := range []string{"pine", "jungle_tree", "cactus", "shrub", "reeds"} {
		k, sc, ok := newPlantKey(game.PlantView{Species: sp, X: 1, Y: 2, Growth: 50}, 7)
		if !ok || k == "" || sc <= 0 {
			t.Errorf("plant %q: %q %v %v", sp, k, sc, ok)
		}
		if got := r.seasonKey(k, game.WorldView{Season: "Winter"}); got == "" {
			t.Errorf("plant %q has no season key", sp)
		}
	}
	for _, kind := range []string{"milk", "wool", "egg", "fertile egg", "ember oil", "wool parka", "wool carpet"} {
		if itemSprite(kind) == "" {
			t.Errorf("item %q has no sprite key", kind)
		}
	}
	if k := troughKey(game.BuildingView{TroughFill: 3, Facing: "east"}); k != "furniture/feeding_trough_full_east" {
		t.Errorf("trough key = %q", k)
	}
	if buildSprite("feeding trough", "", 0, false, false) == "" {
		t.Error("the trough ghost has a sprite")
	}
	for _, d := range game.Biomes() {
		_ = grassTint(d.Key)
	}
	if grassTint("desert") == noCS || grassTint("temperate_forest") != noCS {
		t.Error("desert grass is tinted dry; temperate stays as drawn")
	}
}
