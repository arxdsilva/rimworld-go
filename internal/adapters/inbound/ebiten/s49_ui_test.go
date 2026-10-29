package ebiten

import (
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/devtools"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/inspector"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

func TestHealthPickerUsesViewOptionsAndShowsReasons(t *testing.T) {
	t.Parallel()
	a, f, s := healthApp(t)
	h := s.health[1]
	h.Options = []game.OperationOptionView{
		{Label: "Install glass eye (left eye)", Kind: uint8(body.InstallImplant), Part: uint8(body.EyeL), Item: uint8(item.GlassEye), Disabled: true, Reason: "no glass eye in stock"},
		{Label: "Amputate right arm", Kind: uint8(body.Amputate), Part: uint8(body.ArmR)},
	}
	h.Operations = []game.OperationView{{Label: "Install peg leg (left leg)", Slot: 0, Status: "no peg leg in stock"}}
	s.health[1] = h
	if got := pickerOptions(h); len(got) != 2 || got[0].Reason != "no glass eye in stock" || got[1].Reason != "" {
		t.Fatalf("pickerOptions = %+v", got)
	}
	drawFrame(t, a, f) // status line under the queued row paints
	press(t, a, f, inspector.HealthAddID)
	f.Frame(t, a)
	if !hasWidget(a, uikit.OptID(inspector.HealthPickList, 1)) {
		t.Fatal("both view options are listed")
	}
	// the disabled one still queues
	press(t, a, f, uikit.OptID(inspector.HealthPickList, 0))
	press(t, a, f, "modal."+inspector.HealthPickID+".health.addop.ok")
	var got *command.Command
	for i := range s.cmds {
		if s.cmds[i].Kind == command.AddOperation {
			got = &s.cmds[i]
		}
	}
	if got == nil || got.Index != int(body.InstallImplant) || got.Build != uint8(body.EyeL) || got.Stuff != uint8(item.GlassEye) {
		t.Fatalf("AddOperation = %+v", got)
	}
}

func TestOperationStatusColour(t *testing.T) {
	t.Parallel()
	if inspector.OpStatusColor(game.OperationView{Status: "no peg leg in stock", Blocked: true}) != uikit.Warning ||
		inspector.OpStatusColor(game.OperationView{Status: "a doctor is on the way"}) != uikit.TextDim {
		t.Fatal("blocked operations warn, progress stays dim")
	}
}

func benchWithMostRecipes() (string, int) {
	count := map[string]int{}
	best := ""
	for _, r := range game.Recipes() {
		count[r.Bench]++
		if count[r.Bench] > count[best] {
			best = r.Bench
		}
	}
	return best, count[best]
}

func TestWorkbenchBillPickerReachesEveryRecipe(t *testing.T) {
	t.Parallel()
	kind, n := benchWithMostRecipes()
	s := bioSim()
	s.world.Buildings = []game.BuildingView{{ID: 77, Kind: kind, X: 8, Y: 8, W: 2, H: 1, Bench: true,
		Bills: []game.BillView{{Label: "a"}, {Label: "b"}, {Label: "c"}}}}
	a, f := colonyApp(t, s)
	if !a.insp.Building().SelectBuildingAt(8, 8) {
		t.Fatal("select bench")
	}
	// the bills live in the pane's Bills tab (PLAY-018), declared into the
	// expanded panel
	tab, ok := bldTab(a.insp.Building().BuildingPane(77), "inspector.bills")
	if !ok {
		t.Fatal("a bench has a Bills tab")
	}
	r := gfx.Rect{X: 0, Y: 180, W: tab.WantW, H: tab.WantH}.Inset(inspector.PaneInset) // what the pane hands Declare
	setHook(a.uiSt(), func(a *App) { tab.Declare(r) })
	f.Frame(t, a)
	for _, w := range a.uiSt().Cur {
		if strings.HasPrefix(w.ID, "bill.add.") {
			t.Errorf("per-recipe button %s would overflow the panel", w.ID)
		}
		if w.R.X < r.X || w.R.Y < r.Y || w.R.Right() > r.Right() || w.R.Bottom() > r.Bottom() {
			t.Errorf("%s outside the panel", w.ID)
		}
	}
	cur := a.uiSt().Cur
	for i, p := range cur {
		for _, q := range cur[i+1:] {
			if p.ID == "bill.rows" || q.ID == "bill.rows" { // the scroll view holds the rows
				continue
			}
			if p.R.X < q.R.Right() && q.R.X < p.R.Right() && p.R.Y < q.R.Bottom() && q.R.Y < p.R.Bottom() {
				t.Errorf("%s overlaps %s", p.ID, q.ID)
			}
		}
	}
	if !hasWidget(a, inspector.BillPickBtn) {
		t.Fatal("no Add bill button")
	}
	press(t, a, f, inspector.BillPickBtn)
	f.Frame(t, a)
	if a.uiModalTop() != inspector.BillPickID {
		t.Fatal("picker not open")
	}
	if n < 1 || !hasWidget(a, uikit.OptID(inspector.BillPickList, 0)) {
		t.Fatal("recipes listed")
	}
	if got := len(a.insp.Building().RecipesFor(kind)); got != n {
		t.Fatalf("recipes %d != %d", got, n)
	}
	press(t, a, f, uikit.OptID(inspector.BillPickList, 0))
	press(t, a, f, "modal."+inspector.BillPickID+".bill.picker.ok")
	if len(cmdsOf(s, command.AddBill)) != 1 {
		t.Fatalf("AddBill not sent: %+v", s.cmds)
	}
}

// devSim is a stub that also offers the developer hooks.
type devSim struct {
	*stubSim
	lost    []string
	implant bool
}

func (d *devSim) SpawnAnimal(string, int, int) (entity.EntityID, error) { return 0, nil }
func (d *devSim) KillAnimal(entity.EntityID)                            {}
func (d *devSim) FireIncident(string) bool                              { return true }
func (d *devSim) Raid(int)                                              {}
func (d *devSim) SpawnStack(item.Kind, int, int, int) (entity.EntityID, error) {
	return 0, nil
}
func (d *devSim) SetNeeds(entity.EntityID, int, int) {}
func (d *devSim) DevLosePart(_ entity.EntityID, part string) bool {
	d.lost = append(d.lost, part)
	return true
}
func (d *devSim) DevSpawnImplants(entity.EntityID) bool { d.implant = true; return true }

func TestDevMenuLosePartAndSpawnImplants(t *testing.T) {
	t.Parallel()
	d := &devSim{stubSim: bioSim()}
	a, f := colonyApp(t, d.stubSim)
	set := a.settings
	set.Gameplay.DevMode = true
	a.setSettings(set)
	a.renderer.Select(1)
	f.Frame(t, a)
	acts := a.tools.Actions(3, 3)
	byLabel := map[string]devtools.Action{}
	for _, x := range acts {
		byLabel[x.Label] = x
	}
	imp, ok := byLabel["Spawn implants"]
	if !ok || imp.Reason != "" {
		t.Fatalf("Spawn implants action: %+v", imp)
	}
	imp.Run(d)
	if !d.implant {
		t.Fatal("DevSpawnImplants not called")
	}
	lose, ok := byLabel["Lose a part..."]
	if !ok || lose.Reason != "" {
		t.Fatalf("Lose a part action: %+v", lose)
	}
	lose.Run(d)
	f.Frame(t, a)
	if a.uiModalTop() != devtools.LosePickID {
		t.Fatal("part picker not open")
	}
	press(t, a, f, uikit.OptID(devtools.LosePickList, 2))
	press(t, a, f, "modal."+devtools.LosePickID+".dev.lose.ok")
	if len(d.lost) != 1 || d.lost[0] != devtools.LoseParts()[2] {
		t.Fatalf("lost = %v", d.lost)
	}
}

func TestDevPartActionsNeedAColonist(t *testing.T) {
	t.Parallel()
	d := &devSim{stubSim: bioSim()}
	a, _ := colonyApp(t, d.stubSim)
	set := a.settings
	set.Gameplay.DevMode = true
	a.setSettings(set)
	for _, x := range a.tools.Actions(1, 1) {
		if (x.Label == "Spawn implants" || x.Label == "Lose a part...") && x.Reason == "" {
			t.Fatalf("%s should need a selected colonist", x.Label)
		}
	}
}
