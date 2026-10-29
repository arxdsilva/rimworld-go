package ebiten

// journey_wave_d_test.go: the wave D journeys through the real UI loop:
// PRD-3 a fueled smithy burns wood while it crafts, BED-3 sitting raises
// comfort, DEF-3 the mortar's forced target, the high-explosive IED, the
// door and turret holds, and the "colonists need beds" alert.

import (
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

var _ = owe("journey_wave_d_test.go", []string{}, []string{})

// TestJourneySmithyBillWithFuel (PRD-3): a wood-fired forge takes wood
// from a colonist, the colonist crafts a smithy recipe, and the forge's
// fuel level drops while it works.
func TestJourneySmithyBillWithFuel(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}))
	wdBuild(j, "wood-fired forge", 10, 5)
	defs := recipe.For(construction.FueledSmithy)
	if len(defs) == 0 {
		t.Fatal("no recipe for the wood-fired forge yet (PRD-3)")
	}
	def := defs[0]
	j.g.Enqueue(command.Command{Kind: command.AddBill, X: 10, Y: 5, Recipe: uint8(def.ID), Mode: 0, Count: 1})
	j.g.ApplyPending()
	if _, err := j.g.SpawnStack(item.Wood, 60, 12, 5); err != nil {
		t.Fatal(err)
	}
	for _, k := range def.InputKinds() {
		if _, err := j.g.SpawnStack(k, 40, 13, 6); err != nil {
			t.Fatal(err)
		}
	}
	id := wdBuilding(j, 10, 5)
	bv := func() (fuel int, has bool) {
		for _, b := range j.world().Buildings {
			if b.ID == id {
				return b.FuelPct, b.Fuel
			}
		}
		t.Fatal("the forge vanished")
		return 0, false
	}
	if _, has := bv(); !has {
		t.Fatal("BuildingView.Fuel is false for the wood-fired forge: it carries no fuel")
	}
	peak, drop, made := 0, false, false
	j.key("Digit3", 0)
	for i := 0; i < 15000 && !(drop && made); i++ {
		careFeed(j)
		j.update(1)
		f, _ := bv()
		if f > peak {
			peak = f
		}
		if peak > 0 && f < peak {
			drop = true
		}
		for _, it := range j.world().Items {
			if def.Output != 0 && it.Kind == def.Output.String() && !it.Carried {
				made = true
			}
		}
	}
	j.key("Space", 0)
	if peak == 0 {
		t.Fatal("nobody hauled wood into the forge: its fuel never rose above 0")
	}
	if !drop {
		t.Fatalf("the forge's fuel never dropped below its peak %d%% while the bill ran", peak)
	}
	if !made {
		t.Fatalf("the %q bill produced no %s", def.Label, def.Output)
	}
}

// TestJourneySittingRaisesComfort (BED-3): a colonist sits on a seat
// and the comfort need on its view climbs.
func TestJourneySittingRaisesComfort(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}))
	// BuildNow cannot place a fabric piece (its stuff lookup knows only
	// wood, stone and steel), so the seat is the wood stool.
	wdBuild(j, "stool", 9, 5)
	pawn := ordColonists(j)[0]
	start := wcNum(j, j.pawn(pawn), "Comfort")
	best, lowest := start, start
	j.key("Digit3", 0)
	for i := 0; i < 12000; i++ {
		careFeed(j)
		j.update(1)
		c := wcNum(j, j.pawn(pawn), "Comfort")
		lowest = min(lowest, c)
		if c > best && c > lowest {
			best = c
			break
		}
	}
	j.key("Space", 0)
	if !(best > lowest) {
		t.Fatalf("PawnView.Comfort never rose: started %v, lowest %v, best %v (no one sat down)", start, lowest, best)
	}
}

// wdMortar builds a mortar at (10, 6) and a colonist's loading stack.
func wdMortarSetup(j *journey) (entity.EntityID, uint64) {
	j.t.Helper()
	wdBuild(j, "mortar", 10, 6)
	shell := wdKindAny(j, "mortar shell", "mortar shells", "shell", "shells")
	if _, err := j.g.SpawnStack(shell, 6, 12, 6); err != nil {
		j.t.Fatal(err)
	}
	return wdBuilding(j, 10, 6), ordColonists(j)[0]
}

// TestJourneyMortarForcedTarget (DEF-3): the player drafts a colonist to
// man the mortar, presses the inspector's Set target button, clicks a
// map cell, and a shell bursts near it.
func TestJourneyMortarForcedTarget(t *testing.T) {
	t.Parallel()
	// a 48-tile map: the mortar's minimum range is 29.9 tiles (wiki)
	j := newJourney(t, jSize(48, 48), jPawns([2]int{10, 8}))
	id, pawn := wdMortarSetup(j)
	// load: let the colonist haul the shells in
	j.key("Digit3", 0)
	for i := 0; i < 6000; i++ {
		careFeed(j)
		j.update(1)
		if wcNum(j, wdView(j, id), "Shells") > 0 {
			break
		}
	}
	j.key("Space", 0)
	if wcNum(j, wdView(j, id), "Shells") <= 0 {
		t.Fatal("the mortar never got shells (BuildingView.Shells stayed 0): no one rearmed it")
	}
	ordPick(j, 0)
	j.key("R", 0)
	if !j.pawn(pawn).Drafted {
		t.Fatal("R did not draft the colonist")
	}
	// man the mortar: right-click it and take the menu row if one exists
	j.clickTile(10, 6, inputframe.Right, 0)
	if rows := j.a.ord.FloatButtons(); len(rows) > 0 {
		picked := false
		for _, b := range rows {
			if strings.Contains(strings.ToLower(b.Label), "mortar") || strings.HasPrefix(strings.ToLower(b.Label), "man") {
				j.pick(b.Label)
				picked = true
				break
			}
		}
		if !picked {
			j.key("Escape", 0)
		}
	}
	wdSelect(j, 10, 6)
	j.clickID(wdButton(j, "Set target button", "target"))
	const tx, ty = 42, 38 // 32 tiles from the mortar, past its minimum range
	j.lookAt(tx, ty)
	j.update(1)
	j.settle()
	j.clickTile(tx, ty, 0, 0)
	c, ok := j.lastCmd(command.SetMortarTarget)
	if !ok || c.X != 10 || c.Y != 6 || c.X2 != tx || c.Y2 != ty {
		t.Fatalf("SetMortarTarget %+v (sent %v), want mortar (10,6) target (%d,%d)", c, ok, tx, ty)
	}
	if v := wdView(j, id); !wcField(j, v, "HasTarget").Bool() || wcNum(j, v, "TargetX") != tx || wcNum(j, v, "TargetY") != ty {
		t.Fatalf("mortar view %+v does not show the forced target", v)
	}
	j.key("Digit3", 0)
	near := false
	for i := 0; i < 6000 && !near; i++ {
		careFeed(j)
		j.update(1)
		for _, b := range j.world().Bursts {
			if abs(b.X-tx) <= 10 && abs(b.Y-ty) <= 10 { // forced miss radius 9 (wiki: Mortar)
				near = true
			}
		}
	}
	j.key("Space", 0)
	if !near {
		t.Fatalf("no burst near (%d, %d): bursts %+v, mortar %+v", tx, ty, j.world().Bursts, wdView(j, id))
	}
	// clearing the order sends ClearMortarTarget
	wdSelect(j, 10, 6)
	j.clickID(wdButton(j, "Clear target button", "clear"))
	if c, ok := j.lastCmd(command.ClearMortarTarget); !ok || c.X != 10 || c.Y != 6 {
		t.Fatalf("ClearMortarTarget %+v (sent %v), want mortar (10,6)", c, ok)
	}
}

// TestJourneyIEDTrigger (DEF-3): a raider steps on a hidden blast charge
// ring around the colony; it explodes and hurts the raider.
func TestJourneyIEDTrigger(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{12, 12}))
	for y := 8; y <= 16; y++ {
		for x := 8; x <= 16; x++ {
			if max(abs(x-12), abs(y-12)) == 4 {
				wdBuild(j, "blast charge", x, y)
			}
		}
	}
	j.g.RaidArmed(item.Club, item.Club, item.Club)
	j.update(1) // the raid alert pauses the game (pause on threat); unpause after it
	hurt, burst := false, false
	j.key("Digit3", 0)
	for i := 0; i < 8000 && !(hurt && burst); i++ {
		careFeed(j)
		j.update(1)
		for _, p := range j.g.Snapshot() {
			if p.Hostile && p.Health < 100 {
				hurt = true
			}
		}
		for _, b := range j.world().Bursts {
			if wcStr(j, b, "Blast") == "bomb" {
				burst = true
			}
		}
	}
	j.key("Space", 0)
	if !burst {
		t.Fatalf("no bomb burst on WorldView.Bursts: %+v", j.world().Bursts)
	}
	if !hurt {
		t.Fatal("the charge went off but no raider was hurt")
	}
}

// wdToggle selects the building, presses the button whose id holds subs,
// and checks the command and the view flag.
func wdToggle(t *testing.T, kind string, x, y int, subs []string, cmd command.Kind, field string) {
	t.Helper()
	j := newJourney(t, jPawns([2]int{8, 4}))
	wdBuild(j, kind, x, y)
	id := wdSelect(j, x, y)
	j.clickID(wdButton(j, kind+" hold button", subs...))
	c, ok := j.lastCmd(cmd)
	if !ok || c.X != x || c.Y != y || !c.Enabled {
		t.Fatalf("command %+v (sent %v), want %v at (%d,%d) Enabled", c, ok, cmd, x, y)
	}
	j.update(2)
	if !wcField(j, wdView(j, id), field).Bool() {
		t.Fatalf("BuildingView.%s is false after the button", field)
	}
	j.clickID(wdButton(j, kind+" hold button (off)", subs...))
	j.update(2)
	if wcField(j, wdView(j, id), field).Bool() {
		t.Fatalf("BuildingView.%s still true after pressing the button again", field)
	}
}

// TestJourneyDoorHoldOpen (DEF-3): the door's inspector toggles hold open.
func TestJourneyDoorHoldOpen(t *testing.T) {
	t.Parallel()
	wdToggle(t, "powered door", 10, 5, []string{"hold"}, command.SetDoorHoldOpen, "HoldOpen")
}

// TestJourneyTurretHoldFire (DEF-3): the turret's inspector toggles hold fire.
func TestJourneyTurretHoldFire(t *testing.T) {
	t.Parallel()
	wdToggle(t, "sentry turret", 10, 5, []string{"hold"}, command.SetTurretHold, "HoldFire")
}

// TestJourneyColonistsNeedBedsAlert (BED-3): a colony without beds raises
// the alert, and building a bed clears it.
func TestJourneyColonistsNeedBedsAlert(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}))
	has := func() bool {
		for _, a := range j.a.chrome.Alerts().Current() {
			if strings.Contains(strings.ToLower(a.Label), "bed") {
				return true
			}
		}
		return false
	}
	j.update(3)
	j.draw(2)
	if !has() {
		var labels []string
		for _, a := range j.a.chrome.Alerts().Current() {
			labels = append(labels, a.Label)
		}
		t.Fatalf("no bed alert for a colony with no beds; alerts %q", labels)
	}
	if !j.g.BuildNow("bed", "wood", "south", 10, 4) {
		t.Fatal("no bed")
	}
	j.update(3)
	j.draw(2)
	if has() {
		t.Fatal("the bed alert stays up with a bed built")
	}
}
