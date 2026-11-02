package game

import (
	"testing"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/combat/projectile"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/world/fire"
	"rimworld-go/internal/domain/world/stain"
)

// AX-5 whole-game tests: fire foam and the new security buildings, on the
// powered turret rig (turret_test.go).

// ax5View is building id's view.
func ax5View(t *testing.T, g *Game, id uint64) BuildingView {
	t.Helper()
	for _, b := range g.World().Buildings {
		if uint64(b.ID) == id {
			return b
		}
	}
	t.Fatalf("building %d not in the view", id)
	return BuildingView{}
}

func TestAX5KindsAreLive(t *testing.T) {
	t.Parallel()
	for _, k := range []construction.Kind{construction.FoamPopper, construction.IEDFoam, construction.FoamTurret,
		construction.IEDWarhead, construction.RocketBattery} {
		if k.Planned() {
			t.Fatalf("%v is still planned", k)
		}
	}
}

// TestAX5FoamTurretPutsOutFire: a powered foam turret sprays a fire it
// sees, the fire goes out, foam lies where it landed and nothing burns
// under it; the view shows the charge spent.
func TestAX5FoamTurretPutsOutFire(t *testing.T) {
	t.Parallel()
	out := 1000
	g, _ := turretRig(t, 3, 40, 30, &out)
	tid := buildNow(t, g, construction.FoamTurret, item.Steel, construction.South, 10, 5)
	g.Step(powerInterval)
	g.startFire(10, 14, 60)
	g.startFire(11, 14, 60)
	for i := 0; i < 600 && (g.fireAt(10, 14) > 0 || g.fireAt(11, 14) > 0); i++ {
		g.Step(1)
	}
	if g.fireAt(10, 14) > 0 || g.fireAt(11, 14) > 0 {
		t.Fatal("the foam turret left the fire burning")
	}
	if s, ok := g.stains.tiles[[2]int{10, 14}]; !ok || s.Kind != stain.Foam {
		t.Fatalf("no foam under the fire: %+v", s)
	}
	if g.cellFuel(10, 14, nil).Burns() || g.ignite(10, 14, fire.IgniteSize) {
		t.Fatal("something burns under fire foam")
	}
	v := ax5View(t, g, uint64(tid))
	if v.AX.ChargeCap != 10 || v.AX.Charges != 9 {
		t.Fatalf("charges %d / %d", v.AX.Charges, v.AX.ChargeCap)
	}
}

// TestAX5PopperBurstsAndIsRebuilt: a fire beside a foam popper sets it
// off; its foam puts the fire out and a blueprint takes its place.
func TestAX5PopperBurstsAndIsRebuilt(t *testing.T) {
	t.Parallel()
	out := 0
	g, _ := turretRig(t, 4, 40, 30, &out)
	pid := buildNow(t, g, construction.FoamPopper, item.Steel, construction.South, 20, 15)
	if v := ax5View(t, g, uint64(pid)); !v.AX.Armed {
		t.Fatal("a fresh popper is armed")
	}
	g.startFire(22, 16, 60)
	for i := 0; i < 400 && g.fireAt(22, 16) > 0; i++ {
		g.Step(1)
	}
	if g.fireAt(22, 16) > 0 {
		t.Fatal("the popper left the fire burning")
	}
	if _, ok := g.bldgs.Get(pid); ok || g.layerFrameAt(20, 15, construction.LayerObject) == 0 {
		t.Fatal("the spent popper is not replaced by a blueprint")
	}
	foam := false
	for _, b := range g.World().Bursts {
		foam = foam || b.Blast == "foam" && b.Radius == 99
	}
	if !foam {
		t.Fatalf("no foam burst in the view: %+v", g.World().Bursts)
	}
}

// TestAX5WarheadCharge: a raider on a warhead charge sets off the
// antimatter warhead: a wide bomb that hurts a raider ten tiles off.
func TestAX5WarheadCharge(t *testing.T) {
	t.Parallel()
	out := 0
	g, _ := turretRig(t, 5, 40, 30, &out)
	buildNow(t, g, construction.IEDWarhead, item.AntimatterWarhead, construction.South, 20, 15)
	far := turretRaider(t, g, 30, 15)
	turretRaider(t, g, 20, 15)
	g.combat.Traps.Underfoot(g.raiderIDs)
	g.Step(30)
	if v, _ := g.vital.Get(far); v.Injury == 0 && v.Status == 0 {
		t.Fatalf("a raider ten tiles off was untouched: %+v", v)
	}
	bomb := false
	for _, b := range g.World().Bursts {
		bomb = bomb || b.Blast == "bomb" && b.Radius == 149
	}
	if !bomb {
		t.Fatal("no warhead blast in the view")
	}
}

// TestAX5RocketBatteryVolley: a powered rocket battery fires its volley
// at a raider and then shows an empty load.
func TestAX5RocketBatteryVolley(t *testing.T) {
	t.Parallel()
	out := 1000
	g, _ := turretRig(t, 6, 50, 40, &out)
	rid := buildNow(t, g, construction.RocketBattery, item.Steel, construction.South, 10, 5)
	g.Step(powerInterval)
	turretRaider(t, g, 10, 25)
	fired := false
	for i := 0; i < 400; i++ {
		g.Step(1)
		for _, tr := range g.World().Traces {
			fired = fired || tr.Weapon == "rocket battery" && tr.Stage == projectile.Fired
		}
	}
	v := ax5View(t, g, uint64(rid))
	if v.AX.ChargeCap != 12 || v.AX.Charges != 0 || !fired {
		t.Fatalf("rockets %d / %d, fired %v", v.AX.Charges, v.AX.ChargeCap, fired)
	}
}
