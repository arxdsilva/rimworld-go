package inspector

import (
	"strings"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
)

// benchFixture selects a stove at (3,4) holding three bills.
func benchFixture(t *testing.T) (*fakeShell, Building) {
	t.Helper()
	f := newFake()
	bills := threeBills()
	for i := range bills {
		bills[i].Radius, bills[i].SkillMax = 10, 20
	}
	f.world.Buildings = []game.BuildingView{{ID: 7, Kind: "stove", X: 3, Y: 4, W: 1, H: 1, Bench: true, Bills: bills}}
	f.world.Zones = []game.ZoneView{{ID: 2, Kind: "stockpile"}, {ID: 3, Kind: "stockpile", Name: "Cold"}, {ID: 4, Kind: "grow"}}
	f.pawns = []game.PawnView{{ID: 11, Name: "Ada", Status: "alive"}, {ID: 12, Name: "Bo", Status: "alive"}, {ID: 13, Name: "Dead", Status: "dead"}}
	f.sel.HasBuilding, f.sel.SelBuilding = true, 7
	return f, New(f).Building()
}

// testIngredients is meat (allowed) and plant (forbidden); a BillView's
// own list can only be built by the sim, so the tests inject it.
func testIngredients() []BillIngredient {
	return []BillIngredient{{Item: 5, Label: "meat", Allowed: true}, {Item: 6, Label: "plant", Allowed: false}}
}

// openCfg opens bill i's dialog over its settings plus the test ingredients.
func openCfg(a Building, i int) {
	b, _ := a.SelectedBuilding()
	cfg := billSettings(b.Bills[i])
	cfg.Ingredients = testIngredients()
	a.Bills().openBillConfigWith(b, i, cfg)
}

func lastCmd(t *testing.T, f *fakeShell) command.Command {
	t.Helper()
	if len(f.cmds) == 0 {
		t.Fatal("no command sent")
	}
	return f.cmds[len(f.cmds)-1]
}

func TestBillRowHasReorderConfigAndCopyButtons(t *testing.T) {
	t.Parallel()
	b := game.BuildingView{ID: 7, Kind: "stove", Bench: true, Bills: threeBills()}
	body := BillsBody(b, 600)
	if len(body.Buttons) != 9*len(b.Bills) {
		t.Fatalf("buttons = %d, want %d", len(body.Buttons), 9*len(b.Bills))
	}
	byID := map[string]bool{}
	for _, bt := range body.Buttons {
		byID[bt.ID] = bt.Disabled
		if bt.X+bt.W > 600 {
			t.Errorf("%s overflows the row", bt.ID)
		}
	}
	for _, id := range []string{"bill.cfg.1", "bill.copy.1", "bill.up.1", "bill.dn.1"} {
		if dis, ok := byID[id]; !ok || dis {
			t.Errorf("%s present=%v disabled=%v", id, ok, dis)
		}
	}
	if !byID["bill.up.0"] || byID["bill.dn.0"] || !byID["bill.dn.2"] || byID["bill.up.2"] {
		t.Error("the first bill cannot go up and the last cannot go down")
	}
}

func TestBillsBodyShowsWorkerProgressAndReason(t *testing.T) {
	t.Parallel()
	b := game.BuildingView{ID: 7, Kind: "stove", Bench: true, Bills: threeBills()}
	extra := func(i int) BillExtra {
		switch i {
		case 0:
			return BillExtra{Worker: "Ada", ProgressPct: 40}
		case 1:
			return BillExtra{NoWorker: "nobody skilled enough"}
		}
		return BillExtra{}
	}
	body := BillsBodyWith(b, 600, extra)
	if !strings.Contains(body.Labels[0].Text, "[Ada 40%]") {
		t.Errorf("row 0 = %q", body.Labels[0].Text)
	}
	if !strings.Contains(body.Labels[1].Text, "(no worker: nobody skilled enough)") {
		t.Errorf("row 1 = %q", body.Labels[1].Text)
	}
	if strings.Contains(body.Labels[2].Text, "worker") {
		t.Errorf("row 2 = %q", body.Labels[2].Text)
	}
}

func TestReorderButtonsSendReorderBill(t *testing.T) {
	t.Parallel()
	f, a := benchFixture(t)
	a.ApplyBuildingClick("bill.up.2")
	if c := lastCmd(t, f); c.Kind != command.ReorderBill || c.X != 3 || c.Y != 4 || c.Index != 2 || c.Count != -1 {
		t.Fatalf("up = %+v", c)
	}
	a.ApplyBuildingClick("bill.dn.0")
	if c := lastCmd(t, f); c.Kind != command.ReorderBill || c.Index != 0 || c.Count != 1 {
		t.Fatalf("down = %+v", c)
	}
}

func TestBillCopyPasteSendsAddConfigureAndIngredients(t *testing.T) {
	t.Parallel()
	f, a := benchFixture(t)
	f.world.Buildings[0].Bills[0].Recipe = "cook_meal"
	if r := a.Bills().pasteBlock(f.world.Buildings[0]); r == "" {
		t.Fatal("pasting an empty clipboard is blocked")
	}
	a.ApplyBuildingClick(BillPasteBtn)
	if len(f.cmds) != 0 {
		t.Fatalf("empty paste sent %+v", f.cmds)
	}
	recs := a.RecipesFor("stove")
	if len(recs) == 0 {
		t.Skip("no stove recipes")
	}
	f.world.Buildings[0].Bills[0].Recipe = recs[0].Name
	a.ApplyBuildingClick("bill.copy.0")
	if a.billClip.Clip == nil {
		t.Fatal("copy left the clipboard empty")
	}
	a.billClip.Clip.Settings.Ingredients = testIngredients()
	a.ApplyBuildingClick(BillPasteBtn)
	if len(f.cmds) != 3 {
		t.Fatalf("paste sent %d commands: %+v", len(f.cmds), f.cmds)
	}
	add, cfg, ing := f.cmds[0], f.cmds[1], f.cmds[2]
	if add.Kind != command.AddBill || add.Recipe != recs[0].ID || add.Count != 5 || add.Mode != BillModeDo {
		t.Errorf("add = %+v", add)
	}
	if cfg.Kind != command.ConfigureBill || cfg.Index != 3 || cfg.Radius != 10 || cfg.SkillMax != 20 {
		t.Errorf("configure = %+v", cfg)
	}
	if ing.Kind != command.SetBillIngredient || ing.Index != 3 || ing.Stuff != 6 || ing.Enabled {
		t.Errorf("ingredient = %+v", ing)
	}
	a.billClip.Clip.Recipe = "no such recipe"
	if a.Bills().pasteBlock(f.world.Buildings[0]) == "" {
		t.Error("a recipe the bench lacks cannot be pasted")
	}
}

func TestBillConfigDialogControlsSendConfigureBill(t *testing.T) {
	t.Parallel()
	f, a := benchFixture(t)
	openCfg(a, 1)
	if a.modalTop() != BillCfgModalID {
		t.Fatalf("modal = %q", a.modalTop())
	}
	f.frameOnce()
	cases := []struct {
		id    string
		check func(c command.Command) bool
	}{
		{"bill.cfg.store.1", func(c command.Command) bool { return c.Store == StoreFloor }},
		{"bill.cfg.store.2", func(c command.Command) bool { return c.Store == StoreSpecific }},
		{"bill.cfg.zone.3", func(c command.Command) bool { return c.Store == StoreSpecific && c.Zone == 3 }},
		{"bill.cfg.radius.plus", func(c command.Command) bool { return c.Radius == 11 }},
		{"bill.cfg.radius.minus", func(c command.Command) bool { return c.Radius == 10 }},
		{"bill.cfg.radius.unlimited", func(c command.Command) bool { return c.Radius == 0 }},
		{"bill.cfg.smin.plus", func(c command.Command) bool { return c.SkillMin == 1 }},
		{"bill.cfg.smax.minus", func(c command.Command) bool { return c.SkillMax == 19 && c.SkillMin == 1 }},
		{"bill.cfg.worker.12", func(c command.Command) bool { return c.Worker == 12 }},
		{"bill.cfg.worker.0", func(c command.Command) bool { return c.Worker == 0 }},
	}
	for _, tc := range cases {
		before := len(f.cmds)
		if !f.clickWidget(tc.id) {
			t.Fatalf("%s is not on the dialog", tc.id)
		}
		if len(f.cmds) != before+1 {
			t.Fatalf("%s sent %d commands", tc.id, len(f.cmds)-before)
		}
		c := lastCmd(t, f)
		if c.Kind != command.ConfigureBill || c.X != 3 || c.Y != 4 || c.Index != 1 || !tc.check(c) {
			t.Errorf("%s -> %+v", tc.id, c)
		}
	}
	// the colonists list leaves the dead out; stockpiles leave the grow zone out
	for _, id := range []string{"bill.cfg.worker.13", "bill.cfg.zone.4"} {
		if _, ok := f.Kit().Find(id); ok {
			t.Errorf("%s should not be offered", id)
		}
	}
}

func TestBillConfigIngredientTogglesSendSetBillIngredient(t *testing.T) {
	t.Parallel()
	f, a := benchFixture(t)
	openCfg(a, 0)
	f.frameOnce()
	f.clickWidget("bill.cfg.ing.5") // meat: allowed -> forbidden
	if c := lastCmd(t, f); c.Kind != command.SetBillIngredient || c.Index != 0 || c.Stuff != 5 || c.Enabled {
		t.Fatalf("forbid meat = %+v", c)
	}
	f.clickWidget("bill.cfg.ing.6") // plant: forbidden -> allowed
	if c := lastCmd(t, f); c.Stuff != 6 || !c.Enabled {
		t.Fatalf("allow plant = %+v", c)
	}
	f.clickWidget("bill.cfg.ing.5") // meat again: back to allowed
	if c := lastCmd(t, f); c.Stuff != 5 || !c.Enabled {
		t.Fatalf("allow meat = %+v", c)
	}
}

func TestBillConfigUnpauseOnlyForUntilBills(t *testing.T) {
	t.Parallel()
	f, a := benchFixture(t)
	openCfg(a, 0) // a "do N" bill
	f.frameOnce()
	if _, ok := f.Kit().Find("bill.cfg.unpause.plus"); ok {
		t.Fatal("a do-N bill has no unpause control")
	}
	f.Kit().CloseTop()
	openCfg(a, 2) // "until you have"
	f.frameOnce()
	if !f.clickWidget("bill.cfg.unpause.plus") {
		t.Fatal("until bill lacks the unpause control")
	}
	if c := lastCmd(t, f); c.Kind != command.ConfigureBill || c.UnpauseAt != 1 || c.Index != 2 {
		t.Fatalf("unpause = %+v", c)
	}
	f.clickWidget("bill.cfg.unpause.minus")
	if c := lastCmd(t, f); c.UnpauseAt != 0 {
		t.Fatalf("unpause floor = %+v", c)
	}
}

func TestBedStatsLinesAndForbidToggle(t *testing.T) {
	t.Parallel()
	f, a := benchFixture(t)
	bed := game.BuildingView{ID: 9, Kind: "bed", X: 1, Y: 1, W: 1, H: 2, Comfort: 700, RestEffect: 1050, Beauty: -2000,
		Facilities: game.FacilityEndTable}
	f.world.Buildings = append(f.world.Buildings, bed)
	got := strings.Join(bedStatLines(bed), "|")
	for _, want := range []string{"Comfort: 70%", "Rest effectiveness: 105%", "Beauty: -2.0", "Linked: End table"} {
		if !strings.Contains(got, want) {
			t.Errorf("bed lines %q lack %q", got, want)
		}
	}
	if strings.Contains(got, "Dresser") {
		t.Errorf("no dresser is linked: %q", got)
	}
	if bedStatLines(game.BuildingView{Kind: "wall"}) != nil {
		t.Error("a wall has no bed stats")
	}
	fb := forbidCommand(bed)
	if fb.On || len(fb.Cmds) != 1 {
		t.Fatalf("forbid button = %+v", fb)
	}
	if c := fb.Cmds[0]; c.Kind != command.SetForbidden || c.Target != 9 || !c.Enabled {
		t.Errorf("forbid command = %+v", c)
	}
	bed.Forbidden = true
	if fb = forbidCommand(bed); !fb.On || fb.Cmds[0].Enabled {
		t.Errorf("a forbidden building's toggle allows it: %+v", fb)
	}
	found := false
	for _, bt := range a.BuildingCommands(9) {
		found = found || bt.ID == BuildingForbidID
	}
	if !found {
		t.Error("BuildingCommands lacks Forbid")
	}
}
