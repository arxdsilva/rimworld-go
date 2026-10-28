package ebiten

import (
	"strings"
	"testing"

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
	if got := pickerOptions(h); len(got) != 2 || got[0].reason != "no glass eye in stock" || got[1].reason != "" {
		t.Fatalf("pickerOptions = %+v", got)
	}
	drawFrame(t, a, f) // status line under the queued row paints
	press(t, a, f, healthAddID)
	f.Frame(t, a)
	if !hasWidget(a, optID(healthPickList, 1)) {
		t.Fatal("both view options are listed")
	}
	// the disabled one still queues
	press(t, a, f, optID(healthPickList, 0))
	press(t, a, f, "modal."+healthPickID+".health.addop.ok")
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
	if opStatusColor(game.OperationView{Status: "no peg leg in stock", Blocked: true}) != uiWarning ||
		opStatusColor(game.OperationView{Status: "a doctor is on the way"}) != uiTextDim {
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
	if !a.selectBuildingAt(8, 8) {
		t.Fatal("select bench")
	}
	// the bills live in the pane's Bills tab (PLAY-018), declared into the
	// expanded panel
	tab, ok := bldTab(a.buildingPane(77), "inspector.bills")
	if !ok {
		t.Fatal("a bench has a Bills tab")
	}
	r := rect{X: 0, Y: 180, W: tab.WantW, H: tab.WantH}.Inset(paneInset) // what the pane hands Declare
	setHook(a.uiSt(), func(a *App) { tab.Declare(r) })
	f.Frame(t, a)
	for _, w := range a.uiSt().cur {
		if strings.HasPrefix(w.id, "bill.add.") {
			t.Errorf("per-recipe button %s would overflow the panel", w.id)
		}
		if w.r.X < r.X || w.r.Y < r.Y || w.r.Right() > r.Right() || w.r.Bottom() > r.Bottom() {
			t.Errorf("%s outside the panel", w.id)
		}
	}
	cur := a.uiSt().cur
	for i, p := range cur {
		for _, q := range cur[i+1:] {
			if p.id == "bill.rows" || q.id == "bill.rows" { // the scroll view holds the rows
				continue
			}
			if p.r.X < q.r.Right() && q.r.X < p.r.Right() && p.r.Y < q.r.Bottom() && q.r.Y < p.r.Bottom() {
				t.Errorf("%s overlaps %s", p.id, q.id)
			}
		}
	}
	if !hasWidget(a, billPickBtn) {
		t.Fatal("no Add bill button")
	}
	press(t, a, f, billPickBtn)
	f.Frame(t, a)
	if a.uiModalTop() != billPickID {
		t.Fatal("picker not open")
	}
	if n < 1 || !hasWidget(a, optID(billPickList, 0)) {
		t.Fatal("recipes listed")
	}
	if got := len(a.recipesFor(kind)); got != n {
		t.Fatalf("recipes %d != %d", got, n)
	}
	press(t, a, f, optID(billPickList, 0))
	press(t, a, f, "modal."+billPickID+".bill.picker.ok")
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
	acts := a.devActions(3, 3)
	byLabel := map[string]devAction{}
	for _, x := range acts {
		byLabel[x.label] = x
	}
	imp, ok := byLabel["Spawn implants"]
	if !ok || imp.reason != "" {
		t.Fatalf("Spawn implants action: %+v", imp)
	}
	imp.run(d)
	if !d.implant {
		t.Fatal("DevSpawnImplants not called")
	}
	lose, ok := byLabel["Lose a part..."]
	if !ok || lose.reason != "" {
		t.Fatalf("Lose a part action: %+v", lose)
	}
	lose.run(d)
	f.Frame(t, a)
	if a.uiModalTop() != devLosePickID {
		t.Fatal("part picker not open")
	}
	press(t, a, f, optID(devLosePickList, 2))
	press(t, a, f, "modal."+devLosePickID+".dev.lose.ok")
	if len(d.lost) != 1 || d.lost[0] != devLoseParts()[2] {
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
	for _, x := range a.devActions(1, 1) {
		if (x.label == "Spawn implants" || x.label == "Lose a part...") && x.reason == "" {
			t.Fatalf("%s should need a selected colonist", x.label)
		}
	}
}
