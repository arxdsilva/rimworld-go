package ebiten

import (
	"strconv"
	"testing"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/application/scenario"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/prisoner"
	"rimworld-go/internal/domain/sim/command"
)

// journey_dialogs_test.go: dialog and inspector pane journeys (QA-001). The debts below are what this
// file still owes the guard (TestEveryInputActionHasAJourney); delete a
// name here once a journey in this file covers it.
var _ = owe("journey_dialogs_test.go",
	// OpenCasket: only a vault from the landing site's map generation holds
	// a sealed casket; no public game hook or scenario stages one.
	[]string{"OpenCasket"},
	[]string{})

// dlgBuildingAt is the colony's building standing on (x, y).
func dlgBuildingAt(j *journey, x, y int) game.BuildingView {
	j.t.Helper()
	b, ok := buildingAt(j.world(), x, y)
	if !ok {
		j.t.Fatalf("no building at (%d, %d)", x, y)
	}
	return b
}

// dlgBuilding is a building's current view.
func dlgBuilding(j *journey, id uint64) game.BuildingView {
	j.t.Helper()
	for _, b := range j.world().Buildings {
		if uint64(b.ID) == id {
			return b
		}
	}
	j.t.Fatalf("no building %d", id)
	return game.BuildingView{}
}

// TestJourneyDialogsBenchBills: a workbench's Bills tab adds a bill from
// the recipe picker, raises its count and deletes it.
func TestJourneyDialogsBenchBills(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	if !j.g.BuildNow("workbench", "wood", "south", 10, 5) {
		t.Fatal("no workbench")
	}
	bench := dlgBuildingAt(j, 10, 5)
	j.clickTile(10, 5, mbLeft, 0)
	if !j.a.sel.HasBuilding || uint64(j.a.sel.SelBuilding) != uint64(bench.ID) {
		t.Fatalf("the click selected %v (has %v), want the workbench", j.a.sel.SelBuilding, j.a.sel.HasBuilding)
	}
	j.clickID("inspector.bills")
	if j.a.paneOpenTab() != "inspector.bills" {
		t.Fatalf("open tab %q, want Bills", j.a.paneOpenTab())
	}
	j.clickID("bill.pick")
	j.update(1)
	if j.a.uiModalTop() != billPickID {
		t.Fatalf("top modal %q, want the recipe picker", j.a.uiModalTop())
	}
	j.clickID("bill.picker.list.0")
	j.clickID("modal.bill.picker.bill.picker.ok")
	add, ok := j.lastCmd(command.AddBill)
	if !ok || add.X != 10 || add.Y != 5 {
		t.Fatalf("AddBill %+v (sent %v), want one for the workbench", add, ok)
	}
	bills := dlgBuilding(j, uint64(bench.ID)).Bills
	if len(bills) != 1 {
		t.Fatalf("%d bills after Add, want 1", len(bills))
	}
	before := bills[0].Target
	j.clickID("bill.plus.0")
	if _, ok := j.lastCmd(command.SetBill); !ok {
		t.Fatal("+ sent no SetBill")
	}
	if got := dlgBuilding(j, uint64(bench.ID)).Bills[0].Target; got != before+1 {
		t.Fatalf("count %d after +, want %d", got, before+1)
	}
	j.clickID("bill.del.0")
	if _, ok := j.lastCmd(command.RemoveBill); !ok {
		t.Fatal("x sent no RemoveBill")
	}
	if n := len(dlgBuilding(j, uint64(bench.ID)).Bills); n != 0 {
		t.Fatalf("%d bills after x, want none", n)
	}
}

// TestJourneyDialogsBedOwnerMedicalPrisoners: a bed's commands assign an
// owner through the picker, and flip the medical and prisoner marks.
func TestJourneyDialogsBedOwnerMedicalPrisoners(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	if !j.g.BuildNow("bed", "wood", "south", 12, 5) {
		t.Fatal("no bed")
	}
	bed := dlgBuildingAt(j, 12, 5)
	colonist := j.g.Snapshot()[0].ID
	j.clickTile(12, 5, mbLeft, 0)
	if !j.a.sel.HasBuilding || j.a.sel.SelBuilding != bed.ID {
		t.Fatal("the click did not select the bed")
	}

	j.clickID(bedOwnerID(0))
	j.update(1)
	if j.a.uiModalTop() != bedOwnerModalID {
		t.Fatalf("top modal %q, want the owner picker", j.a.uiModalTop())
	}
	j.clickID(bedOwnerPickID(colonist))
	if c, ok := j.lastCmd(command.AssignBedOwner); !ok || c.Target != uint64(bed.ID) || c.Pawn != uint64(colonist) {
		t.Fatalf("AssignBedOwner %+v (sent %v), want the colonist in the bed", c, ok)
	}
	if got := dlgBuilding(j, uint64(bed.ID)).Claimant; got != colonist {
		t.Fatalf("the bed's owner is %v, want %v", got, colonist)
	}
	if j.a.uiModalOpen() {
		t.Fatal("picking an owner left the picker open")
	}

	j.clickID(bedMedicalID)
	if c, ok := j.lastCmd(command.SetBedMedical); !ok || !c.Enabled {
		t.Fatalf("SetBedMedical %+v (sent %v), want it on", c, ok)
	}
	if !dlgBuilding(j, uint64(bed.ID)).Medical {
		t.Fatal("the bed is not medical after the toggle")
	}
	j.clickID(bedMedicalID) // back off, so the prisoner mark is not refused
	j.clickID("bldg.bed.prisoners")
	if c, ok := j.lastCmd(command.SetBedForPrisoners); !ok || !c.Enabled {
		t.Fatalf("SetBedForPrisoners %+v (sent %v), want it on", c, ok)
	}
	if !j.g.BedForPrisoners(bed.ID) {
		t.Fatal("the bed is not for prisoners after the toggle")
	}
}

// TestJourneyDialogsHealthOperations: the Health tab's self-tend box,
// the Rest when hurt toggle, and the operation queue: add two through the
// picker, move the first down, cancel one.
func TestJourneyDialogsHealthOperations(t *testing.T) {
	t.Parallel()
	j := newJourney(t)
	p := j.g.Snapshot()[0]
	j.g.Injure(p.ID, 4)
	q := j.pawn(uint64(p.ID))
	j.clickTile(q.X, q.Y, mbLeft, 0)
	if sel, ok := j.a.selectedPawn(); !ok || sel.ID != p.ID {
		t.Fatal("the click did not select the colonist")
	}

	rest := p.RestWhenHurt
	j.clickID("cmd.pawn.bedrest")
	if c, ok := j.lastCmd(command.SetPawnBedRest); !ok || c.Enabled == rest {
		t.Fatalf("SetPawnBedRest %+v (sent %v), want it flipped from %v", c, ok, rest)
	}
	if j.pawn(uint64(p.ID)).RestWhenHurt == rest {
		t.Fatal("Rest when hurt did not flip")
	}

	j.clickID("inspector.health")
	if j.a.paneOpenTab() != "inspector.health" {
		t.Fatalf("open tab %q, want Health", j.a.paneOpenTab())
	}
	h0, _ := j.g.Health(p.ID)
	j.clickID(healthSelfID)
	if c, ok := j.lastCmd(command.SetSelfTend); !ok || c.Enabled == h0.SelfTend {
		t.Fatalf("SetSelfTend %+v (sent %v), want it flipped from %v", c, ok, h0.SelfTend)
	}
	if h, _ := j.g.Health(p.ID); h.SelfTend == h0.SelfTend {
		t.Fatal("self-tend did not flip")
	}

	ops := func() []game.OperationView { h, _ := j.g.Health(p.ID); return h.Operations }
	// two rows for different parts (the queue refuses a second operation
	// of the same kind on the same part)
	h1, _ := j.g.Health(p.ID)
	opts := pickerOptions(h1)
	rows := []int{0}
	for i, o := range opts {
		if o.op != opts[0].op || o.part != opts[0].part {
			rows = append(rows, i)
			break
		}
	}
	if len(rows) < 2 {
		t.Fatalf("the picker offers %d operations, all on one part", len(opts))
	}
	for _, i := range rows {
		if !j.clickID(healthAddID) {
			t.Fatal("Add operation is disabled")
		}
		j.update(1)
		if j.a.uiModalTop() != healthPickID {
			t.Fatalf("top modal %q, want the operation picker", j.a.uiModalTop())
		}
		j.clickID(optID(healthPickList, i))
		j.clickID("modal." + healthPickID + ".health.addop.ok")
	}
	// The picker and the Cancel button enqueue on the simulation directly,
	// not through the renderer (health_ui.go): nothing records them and,
	// paused, nothing applies them until the colony runs. One tick does.
	j.tick(1)
	appliedBy(j, command.AddOperation, func() bool { return len(ops()) == 2 })
	first, second := ops()[0].Label, ops()[1].Label
	j.clickID(healthDownID(0))
	if c, ok := j.lastCmd(command.ReorderOperation); !ok || c.Count != 1 || c.Index != 0 {
		t.Fatalf("ReorderOperation %+v (sent %v), want slot 0 down", c, ok)
	}
	if got := ops(); got[0].Label != second || got[1].Label != first {
		t.Fatalf("queue %q, %q after moving the first down, want %q, %q", got[0].Label, got[1].Label, second, first)
	}
	j.clickID(healthRemoveID(0))
	j.tick(1)
	appliedBy(j, command.RemoveOperation, func() bool { got := ops(); return len(got) == 1 && got[0].Label == first })
}

// radioColony is a colony with a powered radio console, a trade beacon
// with silver and goods in reach and the default factions, at day 6: the
// tradeScene of cmd/game (the console at (10, 5), the colonist starting at
// (7, 7)).
func radioColony(t *testing.T) *journey {
	t.Helper()
	// seed 6: the default factions include a friendly one, so aid can be asked
	g := game.New(game.Config{Seed: 6, Width: 30, Height: 30})
	if _, err := g.SpawnPawn(7, 7); err != nil {
		t.Fatal(err)
	}
	j := newJourney(t, jGame(g))
	g.DevAddFactions()
	g.SkipTo(6, 8)
	if !g.BuildNow("wood-fired generator", "steel", "south", 15, 2) {
		t.Fatal("no generator")
	}
	g.FuelCampfire(15, 2)
	for x := 10; x <= 15; x++ {
		g.BuildNow("power conduit", "steel", "south", x, 4)
	}
	if !g.BuildNow("radio console", "steel", "south", 10, 5) || !g.BuildNow("trade beacon", "steel", "south", 14, 5) {
		t.Fatal("no console or beacon")
	}
	for _, s := range []struct {
		k    item.Kind
		n, x int
		y    int
	}{{item.Silver, 400, 15, 5}, {item.Leather, 30, 13, 5}, {item.Cloth, 40, 15, 6}} {
		if _, err := g.SpawnStack(s.k, s.n, s.x, s.y); err != nil {
			t.Fatal(err)
		}
	}
	j.tick(240) // the generator lights and the console comes online
	return j
}

// useRadio selects the colonist, orders it onto the console with a right
// click and runs the colony until the comms dialog opens.
func useRadio(t *testing.T, j *journey) {
	t.Helper()
	p := j.g.Snapshot()[0]
	j.lookAt(p.X, p.Y) // the colonist wandered while the generator lit
	j.clickTile(p.X, p.Y, mbLeft, 0)
	if sel, ok := j.a.selectedPawn(); !ok || sel.ID != p.ID {
		t.Fatal("the click did not select the colonist")
	}
	j.settle()
	j.lookAt(10, 5)
	j.clickTile(10, 5, mbRight, 0)
	j.pick("Use radio")
	if c, ok := j.lastCmd(command.Prioritize); !ok || c.Order != command.OrderUseComms {
		t.Fatalf("the menu sent %+v (%v), want Prioritize UseComms", c, ok)
	}
	j.key("Digit1", 0)
	j.until(func() bool { return j.a.uiModalTop() == commsModalID }, 3000)
}

// TestJourneyDialogsCommsConsole: a colonist ordered onto a powered radio
// console opens the comms dialog: gift silver, request a caravan and aid
// from a friendly faction, close it. Back on the radio, call the trade
// ship: the trade dialog trades one item (Accept) and Cancel ends the deal.
func TestJourneyDialogsCommsConsole(t *testing.T) {
	t.Parallel()
	j := radioColony(t)
	if !j.g.FireIncident("orbital_trader") {
		t.Skip("the orbital trader incident refused this colony")
	}
	useRadio(t, j)
	v, ok := j.g.CommsView()
	if !ok {
		t.Fatal("the comms dialog is open with no comms session")
	}
	friend := -1
	for _, f := range v.Factions {
		if f.Aid.Allowed && f.Caravan.Allowed && f.Gift.Allowed {
			friend = f.ID
			break
		}
	}
	if friend < 0 {
		t.Fatalf("no faction takes gifts, caravan and aid requests: %+v", v.Factions)
	}
	goodwill := func() int {
		for _, f := range j.g.Factions() {
			if f.ID == friend {
				return f.Goodwill
			}
		}
		t.Fatalf("faction %d is gone", friend)
		return 0
	}
	id := func(act string) string { return "comms." + strconv.Itoa(friend) + "." + act }

	gw := goodwill()
	j.clickID(id("giftmore"))
	j.clickID(id("gift"))
	if c, ok := j.lastCmd(command.CommsGift); !ok || c.Index != friend || c.Count != commsGiftStart+commsGiftStep {
		t.Fatalf("CommsGift %+v (sent %v), want %d silver to faction %d", c, ok, commsGiftStart+commsGiftStep, friend)
	}
	if goodwill() <= gw {
		t.Fatalf("goodwill %d after the gift, want above %d", goodwill(), gw)
	}
	gw = goodwill()
	j.clickID(id("caravan"))
	if c, ok := j.lastCmd(command.CommsRequestCaravan); !ok || c.Index != friend {
		t.Fatalf("CommsRequestCaravan %+v (sent %v), want faction %d", c, ok, friend)
	}
	if goodwill() >= gw {
		t.Fatal("the caravan request cost no goodwill")
	}
	if !j.clickID(id("aid")) {
		t.Fatal("Request aid is disabled after the caravan request")
	}
	if c, ok := j.lastCmd(command.CommsRequestAid); !ok || c.Index != friend {
		t.Fatalf("CommsRequestAid %+v (sent %v), want faction %d", c, ok, friend)
	}
	j.clickID("modal." + commsModalID + ".close")
	if _, ok := j.lastCmd(command.CloseComms); !ok {
		t.Fatal("Close sent no CloseComms")
	}
	if _, open := j.g.CommsView(); open || j.a.uiModalOpen() {
		t.Fatalf("comms still open after Close (session %v, modal %q)", open, j.a.uiModalTop())
	}

	// back on the radio: call the ship
	j.key("Space", 0)
	useRadio(t, j)
	j.clickID("comms.call")
	if _, ok := j.lastCmd(command.CommsCallTrader); !ok {
		t.Fatal("Call trade ship sent no CommsCallTrader")
	}
	j.update(2)
	if j.a.uiModalTop() != tradeModalID {
		t.Fatalf("top modal %q after the call, want the trade dialog", j.a.uiModalTop())
	}
	tradeOne(t, j)
	j.clickID("modal." + tradeModalID + ".cancel")
	if _, ok := j.lastCmd(command.CloseTrade); !ok {
		t.Fatal("Cancel sent no CloseTrade")
	}
	if _, open := j.g.TradeView(); open {
		t.Fatal("the trade session survived Cancel")
	}
}

// tradeOne puts one item on the open trade dialog's table (sells one of
// the first goods the trader buys, else buys the cheapest it sells) and
// accepts: ProposeTrade leaves, and the trader's stock moves by one.
func tradeOne(t *testing.T, j *journey) {
	t.Helper()
	v, ok := j.g.TradeView()
	if !ok {
		t.Fatal("the trade dialog is open with no session")
	}
	var row game.TradeRowView
	btn, delta := "", 0
	for _, ln := range tradeLines(v.Rows) {
		r := ln.row
		switch {
		case ln.head != "":
		case r.WillSell && r.ColonyQty > 0 && delta != -1:
			row, btn, delta = r, "give", -1
		case r.WillBuy && r.TraderQty > 0 && delta == 0 && (row.Kind == "" || r.BuyC < row.BuyC):
			row, btn = r, "take"
		}
	}
	if btn == "take" {
		delta = 1
	}
	if row.Kind == "" {
		t.Fatalf("nothing to trade: %+v", v.Rows)
	}
	if !j.clickID("trade." + row.Kind + "." + btn + ".1") {
		t.Fatalf("%s 1 %s is disabled", btn, row.Kind)
	}
	if !j.clickID("modal." + tradeModalID + ".accept") {
		t.Fatalf("Accept is disabled with one %s on the table", row.Kind)
	}
	if c, ok := j.lastCmd(command.ProposeTrade); !ok || c.Name == "" {
		t.Fatalf("ProposeTrade %+v (sent %v), want an offer", c, ok)
	}
	after, ok := j.g.TradeView()
	if !ok {
		t.Fatal("the session ended on Accept")
	}
	for _, r := range after.Rows {
		if r.Kind == row.Kind && r.TraderQty != row.TraderQty-delta { // a bought item comes later, by pod
			t.Fatalf("the trader has %d %s after the colony traded %+d of its %d", r.TraderQty, row.Kind, delta, row.TraderQty)
		}
	}
}

// TestJourneyDialogsTradeCaravanAccept: a trader caravan walks in; a
// right click on the trader with a colonist selected orders the trade
// (Prioritize OrderTrade); the colonist walks up, the trade dialog opens
// and pauses the colony, one item changes hands (Accept) and Cancel ends
// the deal.
func TestJourneyDialogsTradeCaravanAccept(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jSize(60, 60), jPawns([2]int{30, 30}))
	g := j.g
	g.DevAddFactions()
	g.SkipTo(4, 8)
	if _, err := g.SpawnStack(item.Silver, 300, 32, 31); err != nil {
		t.Fatal(err)
	}
	// a caravan trades only stockpiled goods: lay a stockpile under the
	// silver with the Architect, as a player would
	j.lookAt(32, 31)
	j.key("Tab", 0)
	j.clickID("arch.cat.zones")
	j.clickID("arch.tool.stockpile")
	j.dragTiles(mbLeft, 31, 30, 33, 32, 0)
	for i := 0; i < 2 && j.a.archOpen; i++ {
		j.key("Escape", 0)
	}
	if _, ok := zoneCellAt(j.world(), 32, 31); !ok {
		t.Fatal("no stockpile under the silver")
	}
	if !g.FireIncident("trader_caravan") {
		t.Skip("the trader caravan incident refused this colony")
	}
	colonist := g.Snapshot()[0].ID
	trader := func() (game.PawnView, bool) {
		for _, p := range g.Snapshot() {
			if !p.Visitor {
				continue
			}
			for _, o := range g.OrdersAt(colonist, p.X, p.Y) {
				if o.Cmd.Order == command.OrderTrade && !o.Disabled {
					return p, true
				}
			}
		}
		return game.PawnView{}, false
	}
	j.key("Digit3", 0)
	j.until(func() bool { _, ok := trader(); return ok }, 3000)
	j.key("Space", 0)
	tp, ok := trader()
	if !ok {
		t.Fatal("the trader left between the Updates")
	}
	c := j.pawn(uint64(colonist))
	j.lookAt(c.X, c.Y)
	j.clickTile(c.X, c.Y, mbLeft, 0)
	j.settle()
	j.lookAt(tp.X, tp.Y)
	j.clickTile(tp.X, tp.Y, mbRight, 0)
	j.pick("Trade with")
	if cmd, ok := j.lastCmd(command.Prioritize); !ok || cmd.Order != command.OrderTrade {
		t.Fatalf("the menu sent %+v (%v), want Prioritize Trade", cmd, ok)
	}
	j.key("Digit1", 0)
	j.until(func() bool { return j.a.uiModalTop() == tradeModalID }, 3000)
	if j.a.renderer.speed != speedPaused {
		t.Fatal("the trade dialog did not pause the colony")
	}
	tradeOne(t, j)
	j.clickID("modal." + tradeModalID + ".cancel")
	if _, ok := j.lastCmd(command.CloseTrade); !ok {
		t.Fatal("Cancel sent no CloseTrade")
	}
	if _, open := g.TradeView(); open || j.a.uiModalOpen() {
		t.Fatalf("trade still open after Cancel (modal %q)", j.a.uiModalTop())
	}
}

// TestJourneyDialogsPrisonerTab: the prisoner's Prisoner tab sets the
// interaction mode and releases the prisoner.
func TestJourneyDialogsPrisonerTab(t *testing.T) {
	t.Parallel()
	g, cast, err := scenario.Social(1, scenario.SocialSide, scenario.SocialSide)
	if err != nil {
		t.Fatal(err)
	}
	j := newJourney(t, jGame(g))
	if j.a.uiModalOpen() {
		j.key("Escape", 0)
	}
	if _, ok := g.Prisoner(cast.Prisoner); !ok { // the warden walks over and arrests
		j.key("Digit3", 0)
		j.until(func() bool { _, ok := g.Prisoner(cast.Prisoner); return ok }, 3000)
		j.key("Space", 0)
	}
	p := j.pawn(uint64(cast.Prisoner))
	j.lookAt(p.X, p.Y)
	j.clickTile(p.X, p.Y, mbLeft, 0)
	if sel, ok := j.a.selectedPawn(); !ok || sel.ID != cast.Prisoner {
		t.Fatalf("the click selected %v (%v), want the prisoner", sel.ID, ok)
	}
	j.clickID(prisonerTabID)
	if j.a.paneOpenTab() != prisonerTabID {
		t.Fatalf("open tab %q, want Prisoner", j.a.paneOpenTab())
	}
	j.clickID(prisonerModeID(prisoner.Reduce))
	if c, ok := j.lastCmd(command.SetPrisonerMode); !ok || c.Target != uint64(cast.Prisoner) || c.Mode != uint8(prisoner.Reduce) {
		t.Fatalf("SetPrisonerMode %+v (sent %v), want Reduce for the prisoner", c, ok)
	}
	if v, _ := g.Prisoner(cast.Prisoner); v.Mode != uint8(prisoner.Reduce) {
		t.Fatalf("mode %d after Reduce, want %d", v.Mode, prisoner.Reduce)
	}
	if !j.clickID(prisonerRelease) {
		t.Fatal("Release now is disabled for a healthy prisoner")
	}
	if c, ok := j.lastCmd(command.ReleasePrisoner); !ok || c.Target != uint64(cast.Prisoner) {
		t.Fatalf("ReleasePrisoner %+v (sent %v), want the prisoner", c, ok)
	}
	if _, still := g.Prisoner(cast.Prisoner); still {
		t.Fatal("still a prisoner after Release now")
	}
}

// TestJourneyDialogsCaravanFormAndReturn: on a wizard colony (it has a
// world), F8 opens the World tab; Form caravan picks a colonist and a
// settlement and sends them (FormCaravan); the caravan's row calls it
// back (CaravanReturn).
func TestJourneyDialogsCaravanFormAndReturn(t *testing.T) {
	t.Parallel()
	j := newMenuJourney(t)
	wizardToColony(t, j)
	if j.a.renderer.speed != speedPaused {
		j.key("Space", 0)
	}
	j.key("F8", 0)
	if j.a.uiModalTop() != worldModalID {
		t.Fatalf("F8 opened %q, want the World tab", j.a.uiModalTop())
	}
	if !j.clickID("modal." + worldModalID + ".world.form") {
		t.Fatal("Form caravan is disabled on a colony with a world")
	}
	j.update(1)
	if j.a.uiModalTop() != formCaravanID {
		t.Fatalf("top modal %q, want Form caravan", j.a.uiModalTop())
	}
	member := j.g.Snapshot()[0].ID
	j.clickID("form.member." + strconv.FormatUint(uint64(member), 10))
	dest := -1
	for _, s := range j.g.WorldSites() {
		if s.Visitable {
			dest = s.Tile
			break
		}
	}
	if dest < 0 {
		t.Fatalf("no settlement takes a caravan: %+v", j.g.WorldSites())
	}
	j.clickID("form.dest." + strconv.Itoa(dest))
	if !j.clickID("modal." + formCaravanID + ".form.send") {
		t.Fatal("Send is disabled with a colonist and a destination picked")
	}
	c, ok := j.lastCmd(command.FormCaravan)
	if !ok || c.Count != dest || c.Name != strconv.FormatUint(uint64(member), 10) {
		t.Fatalf("FormCaravan %+v (sent %v), want colonist %d to tile %d", c, ok, member, dest)
	}
	cs := j.g.WorldMap().Caravans
	if len(cs) != 1 {
		t.Fatalf("%d caravans after Send, want 1", len(cs))
	}
	if j.a.uiModalTop() != worldModalID {
		t.Fatalf("top modal %q after Send, want the World tab back", j.a.uiModalTop())
	}
	row := "world.caravan." + strconv.FormatUint(cs[0].ID, 10) + ".return"
	if !j.clickID(row) {
		t.Fatalf("the caravan's %q button is disabled (state %q)", row, cs[0].State)
	}
	if c, ok := j.lastCmd(command.CaravanReturn); !ok || c.Target != cs[0].ID {
		t.Fatalf("CaravanReturn %+v (sent %v), want caravan %d", c, ok, cs[0].ID)
	}
	if after := j.g.WorldMap().Caravans; len(after) == 1 && after[0].State == cs[0].State {
		t.Fatalf("the caravan is still %q after %s", after[0].State, func() string { l, _ := caravanReturnLabel(cs[0]); return l }())
	}
}
