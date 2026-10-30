package weapon

import (
	"testing"

	"rimworld-go/internal/domain/combat/attack"
	"rimworld-go/internal/domain/combat/damage"
	"rimworld-go/internal/domain/combat/explosion"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/quality"
)

// TestCBT3Defs pins the new gear and the mini-turret's gun to the wiki.
// wiki: Doomsday rocket launcher, Triple rocket launcher, Smoke launcher,
// Charge rifle, Charge lance, Ikwa, Mini-turret
func TestCBT3Defs(t *testing.T) {
	t.Parallel()
	rl, trl := DefOf(RocketLauncher), DefOf(TripleRocketLauncher)
	if rl.Name != "rocket launcher" || rl.Blast != (explosion.Def{Kind: explosion.Bomb, Radius: 7800, Damage: 50, Pen: 100}) ||
		rl.Range != 36 || rl.Warmup != 270 || rl.ForcedMiss != 1900 || rl.Burst != 1 || rl.Speed != 833 || !SingleUse(RocketLauncher) {
		t.Fatalf("rocket launcher %+v", rl)
	}
	if trl.Name != "triple rocket launcher" || trl.Blast.Radius != 3900 || trl.ForcedMiss != 2900 || trl.Burst != 3 ||
		trl.BurstTicks != 20 || !SingleUse(TripleRocketLauncher) || SingleUse(SmokeLauncher) {
		t.Fatalf("triple rocket launcher %+v", trl)
	}
	if sm := DefOf(SmokeLauncher); sm.Name != "smoke launcher" || sm.Blast.Kind != explosion.Smoke || sm.Blast.Radius != 2400 || sm.Range != 23 {
		t.Fatalf("smoke launcher %+v", sm)
	}
	if cr := DefOf(ChargeRifle); cr.Name != "charge rifle" || cr.Damage != 16 || cr.ArmorPen != 350 || cr.Burst != 3 ||
		cr.BurstTicks != 12 || cr.Range != 27 || cr.Bands != attack.DefaultBands(550, 640, 550, 450) {
		t.Fatalf("charge rifle %+v", cr)
	}
	if cl := DefOf(ChargeLance); cl.Name != "charge lance" || cl.Damage != 30 || cl.ArmorPen != 450 || cl.Speed != 2000 ||
		HitOf(ChargeLance, cl).Stopping != 1500 {
		t.Fatalf("charge lance %+v", cl)
	}
	if ik := DefOf(Ikwa); ik.Ranged || ik.Damage != 15 || len(ToolsOf(Ikwa)) != 4 || HitOf(Ikwa, ik).Form != damage.FormStab {
		t.Fatalf("ikwa %+v", ik)
	}
	sg := DefOf(SentryGun)
	if sg.Damage != 12 || sg.ArmorPen != 180 || sg.Cooldown != 288 || sg.Range != 28 || sg.Burst != 2 || sg.BurstTicks != 8 ||
		sg.Speed != 1166 || sg.Bands != attack.DefaultBands(770, 700, 450, 240) {
		t.Fatalf("sentry gun %+v", sg)
	}
	for k := item.RocketLauncher; k <= item.Ikwa; k++ {
		if !k.IsWeapon() || ForItem(k) == Fists {
			t.Errorf("%v is no weapon", k)
		}
	}
	if !Stock(ChargeRifle) || !Stock(RocketLauncher) || Stock(Ikwa) {
		t.Fatal("the new guns club with the stock, the ikwa is a blade")
	}
	if v := Stats(TripleRocketLauncher); !v.SingleUse || v.Burst != 3 || v.Blast != "bomb" {
		t.Fatalf("triple view %+v", v)
	}
}

// TestHeldTuning: quality scales a gun's accuracy (capped) and damage and
// a blade's damage; the material scales melee damage by the blow's type.
// wiki: Quality, Mace, Wood
func TestHeldTuning(t *testing.T) {
	t.Parallel()
	if (Held{}).Tuned(DefOf(Mace)) != DefOf(Mace) || (Held{}).Tool(maceTools[0]) != maceTools[0] {
		t.Fatal("the zero make changes nothing")
	}
	normal := Held{Quality: quality.Stored(quality.Normal), Stuff: item.Steel}
	if normal.Tuned(DefOf(Mace)) != DefOf(Mace) {
		t.Fatal("normal steel is the def")
	}
	wood := Held{Stuff: item.Wood}
	if got := wood.Tuned(DefOf(Mace)).Damage; got != 14 { // 16 x 0.9
		t.Fatalf("wooden mace head %d", got)
	}
	if got := wood.Tool(knifeTools[0]).Damage; got != 4 { // 12 x 0.4 (blade: sharp)
		t.Fatalf("wooden knife blade %d", got)
	}
	if got := wood.Tool(knifeTools[1]).Damage; got != 8 { // 9 x 0.9 (handle: blunt)
		t.Fatalf("wooden knife handle %d", got)
	}
	master := Held{Quality: quality.Stored(quality.Masterwork)}
	if got := master.Tool(maceTools[0]); got.Damage != 23 || got.Pen != 348 { // 16 x 1.45, 24 % x 1.45
		t.Fatalf("masterwork mace %+v", got)
	}
	leg := Held{Quality: quality.Stored(quality.Legendary)}
	sn := leg.Tuned(DefOf(SniperRifle))
	if sn.Bands.Long.Acc != 1000 || sn.Bands.Touch.Acc != 750 || sn.Damage != 37 || sn.ArmorPen != 570 {
		t.Fatalf("legendary sniper %+v", sn)
	}
	awful := Held{Quality: quality.Stored(quality.Awful)}
	if d := awful.Tuned(DefOf(Fists)); d.Damage != 3 {
		t.Fatalf("awful melee %+v", d)
	}
	if d := awful.Tuned(DefOf(SentryGun)); d.Bands.Touch.Acc != 616 || d.Damage != 10 {
		t.Fatalf("awful gun %+v", d)
	}
	if d := (Held{Quality: quality.Stored(quality.Good)}).Tuned(Def{Ranged: true, Accuracy: 95, Damage: 10}); d.Accuracy != 100 {
		t.Fatalf("legacy accuracy capped %+v", d)
	}
}
