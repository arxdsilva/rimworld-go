package ebiten

import (
	"strings"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/adapters/inbound/ebiten/wizard"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/world/planet"
	"rimworld-go/internal/platform/input"
)

// worldApp is a menu App on Create World with a tiny world.
func worldApp(t *testing.T) (*App, *fakeInput) {
	t.Helper()
	a, f, _ := ngTest(t, nil)
	toWorld(t, a)
	f.Frame(t, a)
	return a, f
}

func TestSeedFieldFreeTextAndDice(t *testing.T) {
	t.Parallel()
	a, f := worldApp(t)
	ng := a.ng
	press(t, a, f, "world.seed")
	f.Key(t, a, "A", input.ModCtrl)
	f.TypeText(t, a, "My World-7_x!?")
	if got := ng.World.SeedText; got != "My World-7_x" {
		t.Fatalf("seed = %q, want letters, digits, space, - and _ only", got)
	}
	f.Key(t, a, "A", input.ModCtrl)
	f.TypeText(t, a, strings.Repeat("a", 40))
	if n := len([]rune(ng.World.SeedText)); n != wizard.MaxSeedText {
		t.Fatalf("seed is %d characters, want at most %d", n, wizard.MaxSeedText)
	}
	// Clearing the field and leaving it never leaves an empty seed.
	f.Key(t, a, "A", input.ModCtrl)
	f.Key(t, a, "Backspace", 0)
	f.Key(t, a, "Enter", 0)
	if strings.TrimSpace(ng.World.SeedText) == "" {
		t.Fatal("an empty seed must be replaced by a phrase")
	}
	// Dice replaces the text with a two-word phrase and does not generate.
	before := ng.World.SeedText
	press(t, a, f, "world.dice")
	if ng.World.SeedText == before || !strings.Contains(ng.World.SeedText, "-") {
		t.Fatalf("dice gave %q (was %q)", ng.World.SeedText, before)
	}
	if ng.Builder != nil || ng.Planet != nil || a.uiSt().ModalOpen() {
		t.Fatal("the dice only replaces the text")
	}
}

func TestWorldWordsMakeValidTwoWordSeeds(t *testing.T) {
	t.Parallel()
	for i := 0; i < 200; i++ {
		p := wizard.RandomWorldPhrase()
		if len(p) < 3 || len([]rune(p)) > wizard.MaxSeedText || strings.Count(p, "-") != 1 {
			t.Fatalf("phrase %q", p)
		}
		for _, r := range p {
			if !wizard.SeedTextRune(r) {
				t.Fatalf("phrase %q has %q", p, r)
			}
		}
	}
	if len(wizard.WorldAdjectives) < 40 || len(wizard.WorldNouns) < 40 {
		t.Fatal("the word lists are too short for variety")
	}
}

func TestGenerateRunsBuilderToDone(t *testing.T) {
	t.Parallel()
	a, f := worldApp(t)
	ng := a.ng
	phases := map[string]bool{}
	var last float32
	click(t, a, "world.generate")
	if !a.uiSt().ModalOpen() || a.uiModalTop() != wizard.WorldProgress || ng.Builder == nil {
		t.Fatalf("modal=%q builder=%v", a.uiModalTop(), ng.Builder != nil)
	}
	for i := 0; i < 4000 && a.nav.screen() == ports.ScreenWorldGen; i++ {
		ng.GenStep = 100 // small steps, so the phases show one by one
		f.Frame(t, a)
		if ng.Builder != nil {
			phases[wizard.WorldPhaseName(ng.GenPhase)] = true
			if ng.GenFrac < last {
				t.Fatalf("progress went back: %v after %v", ng.GenFrac, last)
			}
			last = ng.GenFrac
		}
	}
	if a.nav.screen() != ports.ScreenLanding || ng.Planet == nil || ng.Builder != nil || a.uiSt().ModalOpen() {
		t.Fatalf("screen=%s planet=%v builder=%v modal=%v", a.nav.screen(), ng.Planet != nil, ng.Builder != nil, a.uiSt().ModalOpen())
	}
	if ng.Planet.Params.SeedText != tinySeed || ng.Planet.TileCount() < 500 || ng.PlanetKey != ng.World.Key() {
		t.Fatalf("planet seed=%q tiles=%d", ng.Planet.Params.SeedText, ng.Planet.TileCount())
	}
	if len(phases) < 3 {
		t.Fatalf("progress showed phases %v, want several", phases)
	}
}

func TestGenerateSameParamsSkips(t *testing.T) {
	t.Parallel()
	a, f := worldApp(t)
	ng := a.ng
	click(t, a, "world.generate")
	runUntil(t, a, func() bool { return a.nav.screen() == ports.ScreenLanding })
	p := ng.Planet
	click(t, a, "wizard.back")
	f.Frame(t, a)
	if a.nav.screen() != ports.ScreenWorldGen || ng.Planet != p {
		t.Fatalf("Back keeps the planet: screen=%s", a.nav.screen())
	}
	w, ok := a.uiSt().Find("world.generate")
	if !ok || w.Label != "Next" {
		t.Fatalf("the forward button says %q, want Next for a matching world", w.Label)
	}
	click(t, a, "world.generate")
	if a.nav.screen() != ports.ScreenLanding || ng.Planet != p || ng.Builder != nil || a.uiSt().ModalOpen() {
		t.Fatalf("Next must reuse the planet: screen=%s builder=%v", a.nav.screen(), ng.Builder != nil)
	}
}

func TestParamsChangeMarksPlanetStale(t *testing.T) {
	t.Parallel()
	a, f := worldApp(t)
	ng := a.ng
	click(t, a, "world.generate")
	runUntil(t, a, func() bool { return a.nav.screen() == ports.ScreenLanding })
	click(t, a, "land.random")
	if ng.Tile < 0 {
		t.Fatal("no tile chosen")
	}
	click(t, a, "wizard.back")
	f.Frame(t, a)
	ng.World.Rain = 4
	f.Frame(t, a)
	if ng.WorldReady() || ng.Tile != -1 || ng.Planet == nil {
		t.Fatalf("ready=%v tile=%d planet=%v: a changed setting makes the world stale and clears the tile", ng.WorldReady(), ng.Tile, ng.Planet != nil)
	}
	if w, _ := a.uiSt().Find("world.generate"); w.Label != "Generate" {
		t.Fatalf("label = %q, want Generate", w.Label)
	}
	ng.World.Rain = 2 // back to what the planet was made from
	f.Frame(t, a)
	if !ng.WorldReady() {
		t.Fatal("the original settings match the planet again")
	}
	// A slider drag changes a setting through the real widget too.
	ng.World.Rain = 2
	a.wiz.Draft().World.Temp = 1
	f.Frame(t, a)
	if ng.WorldReady() {
		t.Fatal("temperature is part of the world's key")
	}
}

func TestFactionTableAddRemoveReset(t *testing.T) {
	t.Parallel()
	a, f := worldApp(t)
	ng := a.ng
	count := func(ty faction.Type) int {
		if i := wizard.SlotIndex(ng.World.Factions, ty); i >= 0 {
			return ng.World.Factions[i].Count
		}
		return -1
	}
	if count(faction.Settlers) != 2 || count(faction.Freebooters) != 1 {
		t.Fatalf("defaults = %+v", ng.World.Factions)
	}
	click(t, a, "world.fac.settlers.inc")
	click(t, a, "world.fac.settlers.inc")
	click(t, a, "world.fac.settlers.inc") // the maximum is 4
	if got := count(faction.Settlers); got != faction.DefOf(faction.Settlers).MaxCount {
		t.Fatalf("settlers = %d, want clamped to the maximum", got)
	}
	click(t, a, "world.fac.clans.dec")
	click(t, a, "world.fac.clans.dec")
	click(t, a, "world.fac.clans.dec") // the minimum is 0
	if got := count(faction.Clans); got != 0 {
		t.Fatalf("clans = %d, want 0", got)
	}
	click(t, a, "world.fac.drifters.rm")
	if count(faction.Drifters) != -1 {
		t.Fatal("remove takes the row away")
	}
	// Add: a removed type comes back from the list with a count of 1.
	click(t, a, "world.facAdd")
	if !a.uiSt().ModalOpen() {
		t.Fatal("Add faction opens a list")
	}
	choices := wizard.AddableTypes(ng.World.Factions)
	if len(choices) == 0 || wizard.SlotIndex(ng.World.Factions, faction.Drifters) >= 0 && count(faction.Drifters) >= faction.DefOf(faction.Drifters).MaxCount {
		t.Fatalf("addable = %v", choices)
	}
	f.Frame(t, a)
	press(t, a, f, "modal.pick.world.facAdd.pick.list.0")
	f.Frame(t, a)
	if a.uiSt().ModalOpen() {
		t.Fatal("picking closes the list")
	}
	if count(choices[0]) < 1 {
		t.Fatalf("%v was not added: %+v", choices[0], ng.World.Factions)
	}
	click(t, a, "world.facReset")
	want := faction.DefaultSlots()
	if len(ng.World.Factions) != len(want) {
		t.Fatalf("reset = %+v, want %+v", ng.World.Factions, want)
	}
	for i, s := range want {
		if ng.World.Factions[i] != s {
			t.Fatalf("reset = %+v, want %+v", ng.World.Factions, want)
		}
	}
	// The Swarm is locked: it has no controls and no slot.
	if hasWidget(a, "world.fac.swarm.inc") || wizard.SlotIndex(ng.World.Factions, faction.Swarm) >= 0 {
		t.Fatal("the Swarm is always present and can't be edited")
	}
	for _, d := range wizard.AddableTypes(nil) {
		if d == faction.Swarm {
			t.Fatal("the Swarm is never addable")
		}
	}
}

func TestNoHostileWarning(t *testing.T) {
	t.Parallel()
	if !wizard.HostileCapable(faction.DefaultSlots()) {
		t.Fatal("the defaults include Freebooters and Drifters")
	}
	peaceful := []faction.Slot{{Type: faction.Settlers, Count: 2}, {Type: faction.Clans, Count: 2}}
	if wizard.HostileCapable(peaceful) {
		t.Fatal("settlers and clans alone can't raid")
	}
	if wizard.HostileCapable([]faction.Slot{{Type: faction.Freebooters, Count: 0}}) {
		t.Fatal("a zero count is nobody")
	}
	if !wizard.HostileCapable(append(peaceful, faction.Slot{Type: faction.Freebooters, Count: 1})) {
		t.Fatal("freebooters make raids possible")
	}
	// The screen draws the warning without a problem in both states.
	a, f := worldApp(t)
	a.ng.World.Factions = peaceful
	f.Frame(t, a)
	a.Draw(ebiten.NewImage(a.sw, a.sh))
	if !strings.Contains(wizard.NoHostileWarning, "No faction can raid you") {
		t.Fatal("warning text")
	}
}

func TestCancelDiscards(t *testing.T) {
	t.Parallel()
	a, f := worldApp(t)
	ng := a.ng
	ng.GenStep = 50 // slow, so it is still running
	click(t, a, "world.generate")
	ng.GenStep = 50
	f.Frame(t, a)
	if ng.Builder == nil {
		t.Fatal("generation should be running")
	}
	press(t, a, f, "modal.world.progress.cancel")
	f.Frame(t, a)
	if ng.Builder != nil || ng.Planet != nil || a.uiSt().ModalOpen() || a.nav.screen() != ports.ScreenWorldGen {
		t.Fatalf("builder=%v planet=%v modal=%v screen=%s", ng.Builder != nil, ng.Planet != nil, a.uiSt().ModalOpen(), a.nav.screen())
	}
	// Esc cancels as well.
	click(t, a, "world.generate")
	ng.GenStep = 50
	f.Frame(t, a)
	f.Key(t, a, "Escape", 0)
	if ng.Builder != nil || a.uiSt().ModalOpen() || a.nav.screen() != ports.ScreenWorldGen {
		t.Fatal("Esc on the progress modal cancels the generation")
	}
}

func TestStepBudgetDoesNotChangeTheWorld(t *testing.T) {
	t.Parallel()
	p := defaultTestParams()
	whole := planet.Build(p)
	b := planet.NewBuilder(p)
	for {
		if _, _, done := b.Step(137); done {
			break
		}
	}
	if b.Planet().Params.Key() != whole.Params.Key() || b.Planet().TileCount() != whole.TileCount() || b.Planet().Seed != whole.Seed {
		t.Fatal("stepping must give the same planet")
	}
}

func TestResetAllConfirmsThenRestoresDefaults(t *testing.T) {
	t.Parallel()
	a, f := worldApp(t)
	ng := a.ng
	ng.World.Rain, ng.World.Coverage = 0, 50
	press(t, a, f, "world.resetAll")
	if a.uiModalTop() != "world.resetAll.confirm" {
		t.Fatalf("modal = %q, want a confirmation", a.uiModalTop())
	}
	if ng.World.Rain != 0 {
		t.Fatal("nothing changes before the player confirms")
	}
	f.Frame(t, a)
	press(t, a, f, "modal.world.resetAll.confirm.ok")
	f.Frame(t, a)
	if ng.World.Rain != 2 || ng.World.Coverage != wizard.DefaultCoverage || ng.World.SeedText == tinySeed || ng.World.SeedText == "" {
		t.Fatalf("world = %+v", ng.World)
	}
}

func TestCoverageSliderOffersFiveOnlyInDevMode(t *testing.T) {
	t.Parallel()
	a, f := worldApp(t)
	a.ng.World.Coverage = 30
	if got := a.wiz.CoverageOptions(); len(got) != 3 || got[0] != 30 {
		t.Fatalf("options = %v", got)
	}
	a.settings.Gameplay.DevMode = true
	if got := a.wiz.CoverageOptions(); len(got) != 4 || got[0] != 5 {
		t.Fatalf("dev options = %v", got)
	}
	f.Frame(t, a)
	if !hasWidget(a, "world.coverage") {
		t.Fatal("coverage slider")
	}
}

func TestWorldScreenDrawsAtBothSizes(t *testing.T) {
	t.Parallel()
	for _, size := range [][2]int{{1280, 720}, {1920, 1080}} {
		a, f := worldApp(t)
		sizeTo(t, a, f, size[0], size[1])
		img := ebiten.NewImage(size[0], size[1])
		a.Draw(img)
		click(t, a, "world.generate")
		f.Frame(t, a)
		a.Draw(img) // the progress modal
	}
}
