package ebiten

// journey_ax5_test.go (AX-5): fire foam and the new security buildings
// through the real UI loop: the foam popper and the two new charges laid
// from the Architect, a popper smothering a fire, the charges' and the
// foam turret's inspector lines, and hold fire on the foam turret and the
// rocket battery.

import (
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/inputframe"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/research"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/pkg/mathx"
)

// ax5Lay arms tool in cat from the Architect, clicks (x, y) and checks the
// blueprint of kind it lays there.
func ax5Lay(t *testing.T, j *journey, cat, tool, kind string, x, y int) {
	t.Helper()
	j.lookAt(x, y)
	j.update(1)
	j.openArch(cat, tool)
	j.clickTile(x, y, inputframe.Left, 0)
	if c, ok := j.lastCmd(command.DesignateBuild); !ok || c.X != x || c.Y != y {
		t.Fatalf("%s: DesignateBuild %+v ok=%v, want (%d, %d)", tool, c, ok, x, y)
	}
	j.update(1)
	for _, f := range j.world().Frames {
		if f.X == x && f.Y == y && f.Kind == kind {
			j.leaveArch()
			return
		}
	}
	t.Fatalf("%s: no %s blueprint at (%d, %d): %+v", tool, kind, x, y, j.world().Frames)
}

// TestJourneyAX5FoamPopper: the foam popper is laid from Misc once Fire
// foam is researched; a built one shows Armed, a fire started beside it
// sets it off, its foam puts the fire out and leaves foam on the ground,
// and a blueprint takes its place.
func TestJourneyAX5FoamPopper(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}))
	rsFinish(j, int(research.FoamSuppression))
	ax5Lay(t, j, "misc", "foam_popper", "foam popper", 14, 8)
	wdBuild(j, "foam popper", 20, 8)
	id := wdSelect(j, 20, 8)
	if lines := wbPaneLines(j, id); !strings.Contains(lines, "armed") {
		t.Fatalf("popper pane %q, want Armed", lines)
	}
	burning := func() bool {
		for _, f := range j.world().Fires {
			if f.X == 22 && f.Y == 9 {
				return true
			}
		}
		return false
	}
	burst := func() bool {
		for _, b := range j.world().Bursts {
			if wcStr(j, b, "Blast") == "foam" {
				return true
			}
		}
		return false
	}
	// a fire on bare ground soon dies: it is lit again until the popper
	// (which looks every 30 ticks) has sensed it and gone off
	j.key("Digit3", 0)
	for i := 0; i < 1200 && !burst(); i++ {
		if !burning() {
			j.g.Enqueue(command.Command{Kind: command.DevStartFire, X: 22, Y: 9})
			j.g.ApplyPending()
		}
		j.update(1)
	}
	j.key("Space", 0)
	if !burst() || burning() {
		t.Fatalf("foam burst %v, fire still burning %v", burst(), burning())
	}
	foam := false
	for _, s := range j.world().Stains {
		foam = foam || s.Kind == "foam" && s.X == 22 && s.Y == 9
	}
	if !foam {
		t.Fatal("no foam on the ground where the fire was")
	}
	relaid := false
	for _, f := range j.world().Frames {
		relaid = relaid || f.Kind == "foam popper" && f.X == 20 && f.Y == 8
	}
	if !relaid {
		t.Fatal("the spent popper left no blueprint")
	}
}

// TestJourneyAX5Charges: the foam and warhead charges are laid from
// Security once Buried charges is researched; a colonist builds the foam
// charge from two foam shells, and it shows Armed in its pane.
func TestJourneyAX5Charges(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}))
	rsFinish(j, int(research.Charges))
	ax5Lay(t, j, "security", "ied_foam", "foam charge", 14, 8)
	ax5Lay(t, j, "security", "ied_warhead", "warhead charge", 16, 8)
	// the foam charge's one material is foam shells: a colonist hauls two
	// to its blueprint and builds it
	if _, err := j.g.SpawnStack(item.FoamShells, 2, 10, 6); err != nil {
		t.Fatal(err)
	}
	built := func() bool {
		for _, b := range j.world().Buildings {
			if b.Kind == "foam charge" && b.X == 14 && b.Y == 8 {
				return true
			}
		}
		return false
	}
	j.key("Digit3", 0)
	for i := 0; i < 8000 && !built(); i++ {
		careFeedHostileFree(j)
		j.update(1)
	}
	// the builder stands beside it (its figure covers the tile): let it walk off
	clear := func() bool {
		for _, p := range j.g.Snapshot() {
			if mathx.Abs(p.X-14) <= 1 && mathx.Abs(p.Y-8) <= 2 {
				return false
			}
		}
		return true
	}
	for i := 0; i < 4000 && !clear(); i++ {
		careFeedHostileFree(j)
		j.update(1)
	}
	j.key("Space", 0)
	if !built() {
		t.Fatal("the foam charge was never built")
	}
	id := wdSelect(j, 14, 8)
	if lines := wbPaneLines(j, id); !strings.Contains(lines, "armed") {
		t.Fatalf("charge pane %q, want Armed", lines)
	}
}

// TestJourneyAX5FoamTurret: a foam turret's pane shows its ten foam
// charges, and its Hold fire button holds it.
func TestJourneyAX5FoamTurret(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jPawns([2]int{8, 4}))
	rsFinish(j, int(research.FoamTurrets))
	ax5Lay(t, j, "security", "foam_turret", "foam turret", 14, 8)
	wdBuild(j, "foam turret", 20, 8)
	id := wdSelect(j, 20, 8)
	if lines := wbPaneLines(j, id); !strings.Contains(lines, "foam charges  10 / 10") {
		t.Fatalf("foam turret pane %q, want its charges", lines)
	}
	wdToggle(t, "foam turret", 10, 5, []string{"hold"}, command.SetTurretHold, "HoldFire")
}

// TestJourneyAX5RocketBattery: a rocket battery is laid from Security
// once researched. A powered one holds its rockets until the player aims
// it (combat's TestRocketBatteryVolley: even with a raider in range): its inspector's Set target button and a map
// click send SetMortarTarget, the drafted colonist mans it, and its
// rockets burst around the target, each scattered on its own. The volley
// empties it and clears the target, and the pane asks for chemfuel.
func TestJourneyAX5RocketBattery(t *testing.T) {
	t.Parallel()
	j := newJourney(t, jSize(48, 48), jPawns([2]int{10, 9}))
	rsFinish(j, int(research.RocketBatteries))
	ax5Lay(t, j, "security", "rocket_battery", "rocket battery", 30, 8)
	if !j.g.BuildNow("wood-fired generator", "steel", "south", 2, 2) {
		t.Fatal("no generator")
	}
	j.g.FuelCampfire(2, 2)
	for x := 2; x <= 10; x++ {
		j.g.BuildNow("power conduit", "steel", "south", x, 4)
	}
	wdBuild(j, "rocket battery", 10, 5)
	id := wdSelect(j, 10, 5)
	if lines := wbPaneLines(j, id); !strings.Contains(lines, "rockets  12 / 12") || !strings.Contains(lines, "set a target") {
		t.Fatalf("rocket battery pane %q, want a full volley and no target", lines)
	}
	const tx, ty = 10, 30
	// draft the colonist beside the battery: even drafted and idle there,
	// it fires nothing without a target
	j.lookAt(10, 9)
	j.update(1)
	j.settle()
	ordPick(j, 0)
	j.key("R", 0)
	if !j.pawn(ordColonists(j)[0]).Drafted {
		t.Fatal("R did not draft the colonist")
	}
	rockets := func() int {
		n := 0
		for _, b := range j.world().Bursts {
			if wcStr(j, b, "Blast") == "bomb" && b.Radius == 29 {
				n++
			}
		}
		return n
	}
	careFeed(j)
	j.ff(3600, ffStride, func() bool { careFeed(j); return false }) // 600 Updates at Very fast
	if rockets() != 0 {
		t.Fatal("the rocket battery fired on its own")
	}
	wdSelect(j, 10, 5)
	j.clickID(wdButton(j, "Set target button", "target"))
	j.lookAt(tx, ty)
	j.update(1)
	j.settle()
	j.clickTile(tx, ty, 0, 0)
	if c, ok := j.lastCmd(command.SetMortarTarget); !ok || c.X != 10 || c.Y != 5 || c.X2 != tx || c.Y2 != ty {
		t.Fatalf("SetMortarTarget %+v (sent %v), want battery (10,5) target (%d,%d)", c, ok, tx, ty)
	}
	if v := wdView(j, id); !wcField(j, v, "HasTarget").Bool() {
		t.Fatalf("battery view %+v does not show the target", v)
	}
	careFeed(j)
	// 3000 Updates at Very fast, the bursts counted after every tick
	j.ffWatch(18000, ffStride, func() bool { return rockets() >= 12 }, func() bool { careFeed(j); return false })
	near, spread := 0, false
	for _, b := range j.world().Bursts {
		if wcStr(j, b, "Blast") == "bomb" && b.Radius == 29 && mathx.Abs(b.X-tx) <= 6 && mathx.Abs(b.Y-ty) <= 6 {
			near++
			spread = spread || b.X != tx || b.Y != ty
		}
	}
	if near != 12 || !spread {
		t.Fatalf("%d rockets burst within 6 tiles of the target (spread %v), want 12 scattered", near, spread)
	}
	v := wdView(j, id)
	if wcField(j, v, "HasTarget").Bool() {
		t.Fatal("the volley left the target set")
	}
	wdSelect(j, 10, 5)
	if lines := wbPaneLines(j, id); !strings.Contains(lines, "rockets  0 / 12") || !strings.Contains(lines, "chemfuel") {
		t.Fatalf("pane after the volley %q", lines)
	}
}
